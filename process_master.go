package tinybox

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/skoo87/tinybox/proto"
)

const (
	evStop  = "stop"
	evChild = "child"
	evExec  = "exec"
)

type masterProcess struct {
	base
	opt    Options
	ec     chan event
	sigs   map[os.Signal]func(os.Signal, chan event)
	stop   chan struct{}
	childs int32
	wg     sync.WaitGroup
}

func master() *masterProcess {
	return &masterProcess{
		ec: make(chan event, 10),
		sigs: map[os.Signal]func(os.Signal, chan event){
			syscall.SIGINT:  stopHandle,
			syscall.SIGTERM: stopHandle,
			syscall.SIGCHLD: childHandle,
		},
		stop:   make(chan struct{}),
		childs: 0,
	}
}

func (p *masterProcess) Start(c *Container) error {
	go func() {
		p.wg.Add(1)
		defer p.wg.Done()

		p.signals()
	}()

	go func() {
		p.wg.Add(1)
		defer p.wg.Done()

		p.events(c)
	}()

	// http sever base on unix domain socket.
	go func() {
		if err := ListenAndServe(c.UnixFile(), p.ec); err != nil {
			log.Printf("Http server error: %v \n", err)
			os.Exit(1)
		}
	}()

	// Create named pipe.
	if _, err := os.Lstat(c.PipeFile()); err != nil {
		if os.IsNotExist(err) {
			if err := syscall.Mkfifo(c.PipeFile(), 0); err != nil {
				return err
			}
		}
	}

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

	if err := p.cmd.Start(); err != nil {
		return err
	}
	p.incrChild()

	// Save container pid.
	c.Pid = p.Pid()

	// Send info to container init process.
	c.writePipe()
	return nil
}

func (p *masterProcess) Wait(c *Container) error {
	p.wg.Wait()
	p.cleanup(c)

	return nil
}

func (p *masterProcess) cleanup(c *Container) {
	c.fsop.Unmount(c)

	if err := os.Remove(c.UnixFile()); err != nil {
		log.Printf("Remove unix socket %s error: %v \n", err)
	}
	if err := os.Remove(c.PipeFile()); err != nil {
		log.Printf("Remove pipe %s error: %v \n", err)
	}
}

func (p *masterProcess) incrChild() {
	atomic.AddInt32(&p.childs, 1)
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

func childHandle(sig os.Signal, c chan event) {
	if debug {
		log.Printf("handle child \n")
	}

	ev := event{
		action: evChild,
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
			// Kill the init process, and all processes must be stopped.
			//if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			//	errorln("Kill init process error: %v", err)
			//}
			syscall.Kill(c.Pid, syscall.SIGKILL)
			p.cmd.Wait()

		case evChild:
			if atomic.AddInt32(&p.childs, -1) == 0 {
				close(p.stop)
			}

		case evExec:
			er := ev.data.(*proto.ExecRequest)
			command, err := json.Marshal(er)
			if err != nil {
				log.Printf("Encode exec error: %v \n", err)
			}

			execFn := func() error {
				cmd := &exec.Cmd{
					Dir:    "/tmp",
					Path:   "/proc/self/exe",
					Args:   []string{"setns", c.Name},
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				}
				// 通过环境变量给 setns 进程传递数据.
				cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_INIT_PID__=%d", c.Pid))
				cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_CMD__=%s", string(command)))

				if err := cmd.Start(); err != nil {
					return fmt.Errorf("Start setns process error: %v", err)
				}
				p.incrChild()
				p.incrChild()

				status, err := cmd.Process.Wait()
				if err != nil {
					cmd.Wait()
					return fmt.Errorf("Process wait error: %v", err)
				}
				ws := status.Sys().(syscall.WaitStatus)
				if ws.ExitStatus() == 1 {
					cmd.Wait()
					return fmt.Errorf("Setns process failed")
				}

				// pid 通过 exit code 返回.
				pid := ws.ExitStatus()

				process, err := os.FindProcess(pid)
				if err != nil {
					return fmt.Errorf("Find process error: %v, pid = %d", err, pid)
				}
				cmd.Process = process
				cmd.Wait()

				ev.c <- "test"
				return nil
			}

			if err := execFn(); err != nil {
				log.Printf("%v \n", err)
			}

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
