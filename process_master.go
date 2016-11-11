package tinybox

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/skoo87/tinybox/pipe"
)

const (
	evStop  = "stop"
	evChild = "child"
	evExec  = "exec"
	evInfo  = "info"
)

type masterProcess struct {
	cmd  *exec.Cmd
	opt  Options
	ec   chan event
	sigs map[os.Signal]func(os.Signal, chan event)
	stop chan struct{}
	wg   sync.WaitGroup
}

func master() *masterProcess {
	return &masterProcess{
		ec: make(chan event, 10),
		sigs: map[os.Signal]func(os.Signal, chan event){
			syscall.SIGINT:  stopHandle,
			syscall.SIGTERM: stopHandle,
		},
		stop: make(chan struct{}),
	}
}

func (p *masterProcess) eStart(c *Container) error {
	parent, child, err := pipe.New()
	if err != nil {
		return err
	}
	defer parent.Close()
	defer child.Close()

	// lock file
	lock, err := Flock(c.LockFile())
	if err != nil {
		return err
	}

	cmd := &exec.Cmd{
		Dir:    "/tmp",
		Path:   "/proc/self/exe",
		Args:   []string{"setns", c.Name},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, child)

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_INIT_PID__=%d", c.Pid))
	cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_PIPE__=%d", 2+len(cmd.ExtraFiles)))
	cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_CMD__=%s", c.Path))

	if err := cmd.Start(); err != nil {
		Funlock(lock)
		return fmt.Errorf("Start setns process error: %v", err)
	}

	pid := struct {
		Pid int
	}{}
	if err := json.NewDecoder(parent).Decode(&pid); err != nil {
		Funlock(lock)
		return err
	}

	if debug {
		log.Printf("Exec process pid: %d \n", pid.Pid)
	}

	if status, err := cmd.Process.Wait(); err != nil {
		Funlock(lock)
		return err
	} else {
		log.Printf("setns process: %d exit \n", status.Pid())
	}

	process, err := os.FindProcess(pid.Pid)
	if err != nil {
		Funlock(lock)
		return err
	}

	// unlock file
	Funlock(lock)

	if status, err := process.Wait(); err != nil {
		return err
	} else {
		log.Printf("Exec process: %d exit \n", status.Pid())
	}

	return nil
}

func (p *masterProcess) Start(c *Container) error {
	if c.isExec {
		return p.eStart(c)
	}

	go func() {
		p.wg.Add(1)
		defer p.wg.Done()

		p.signals()
		log.Println("Signal loop exited")
	}()

	go func() {
		p.wg.Add(1)
		defer p.wg.Done()

		p.events(c)
		log.Println("Event loop exited")
	}()

	p.cmd = &exec.Cmd{
		Dir:         c.Rootfs,
		Path:        "/proc/self/exe",
		Args:        []string{"init", c.Name},
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Stdin:       os.Stdin,
		SysProcAttr: &syscall.SysProcAttr{},
	}
	p.cmd.SysProcAttr.Cloneflags = c.nsop.Cloneflags(c)

	p.cmd.Env = append(p.cmd.Env, os.Environ()...)

	if err := p.cmd.Start(); err != nil {
		return err
	}

	// Save container pid.
	c.Pid = p.cmd.Process.Pid

	// Set cgroup before init process running.
	if err := p.cgroup(c); err != nil {
		log.Println(err)
		return p.failToWait(c)
	}

	// Send info to container init process.
	c.writePipe()

	// write container's info into disk
	info, err := json.Marshal(c)
	if err != nil {
		log.Println(err)
	} else {
		if err = ioutil.WriteFile(c.JsonFile(), info, 0644); err != nil {
			log.Println(err)
		}
	}

	return p.wait(c)
}

func (p *masterProcess) failToWait(c *Container) error {
	syscall.Kill(c.Pid, syscall.SIGKILL)
	return p.wait(c)
}

func (p *masterProcess) wait(c *Container) error {
	go func() {
		p.cmd.Wait()
		close(p.stop)
		log.Println("Stop master process")
	}()

	p.wg.Wait()
	p.cleanup(c)

	return nil
}

func (p *masterProcess) cleanup(c *Container) {
	c.fsop.Unmount(c)

	if err := os.Remove(c.PipeFile()); err != nil {
		log.Printf("Remove pipe %s error: %v \n", err)
	}

	for _, path := range c.cgop.Paths() {
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Remove %s error: %v \n", path, err)
			}
		}
	}
}

func (p *masterProcess) cgroup(c *Container) error {
	if err := c.cgop.Memory(c); err != nil {
		return err
	}
	if err := c.cgop.CpuSet(c); err != nil {
		return err
	}
	if err := c.cgop.CpuAcct(c); err != nil {
		return err
	}
	if err := c.cgop.CPU(c); err != nil {
		return err
	}
	return nil
}

type event struct {
	action string
	data   interface{}
	c      chan interface{}
}

// signal function.
func stopHandle(sig os.Signal, c chan event) {
	if debug {
		log.Printf("handle stop \n")
	}

	ev := event{
		action: evStop,
	}

	select {
	case c <- ev:
	case <-time.After(time.Second * 5):
		log.Printf("Send event timeout: %ss \n", 5)
	}
}

// signals register and handle os's signal, must run it with a goroutine.
func (p *masterProcess) signals() {
	var slice []os.Signal
	for sig, _ := range p.sigs {
		slice = append(slice, sig)
	}

	sc := make(chan os.Signal, 10)
	signal.Ignore(syscall.SIGHUP)
	signal.Notify(sc, slice...)

	for {
		select {
		case sig := <-sc:
			log.Printf("Trap signal: %s \n", sig)

			if handle, ok := p.sigs[sig]; ok {
				handle(sig, p.ec)
			}

		case <-p.stop:
			return
		}
	}
}

// events handle event, must run it with a goroutine.
func (p *masterProcess) events(c *Container) {
	var ev event
	for {
		select {
		case ev = <-p.ec:
		case <-p.stop:
			return
		}

		log.Printf("Receive event: %s \n", ev.action)

		switch ev.action {
		case evStop:
			log.Printf("Kill init process: %d \n", c.Pid)
			syscall.Kill(c.Pid, syscall.SIGKILL)

		case evChild:

		default:
			if ev.c != nil {
				select {
				case ev.c <- "hello, world":
				default:
				}
			}
		}
	}
}
