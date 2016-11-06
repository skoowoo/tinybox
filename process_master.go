package tinybox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/skoo87/tinybox/pipe"
	"github.com/skoo87/tinybox/proto"
)

const (
	evStop  = "stop"
	evChild = "child"
	evExec  = "exec"
)

type masterProcess struct {
	base
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

func (p *masterProcess) Start(c *Container) error {
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

	// http sever base on unix domain socket.
	ls := make(chan struct{})
	go func() {
		if err := ListenAndServe(c.UnixFile(), p.ec, ls); err != nil {
			log.Printf("Http server error: %v \n", err)
			os.Exit(1)
		}
	}()

	<-ls

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

	// Save container pid.
	c.Pid = p.Pid()

	// Send info to container init process.
	c.writePipe()

	return nil
}

func (p *masterProcess) Wait(c *Container) error {
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

	if err := os.Remove(c.UnixFile()); err != nil {
		log.Printf("Remove unix socket %s error: %v \n", err)
	}
	if err := os.Remove(c.PipeFile()); err != nil {
		log.Printf("Remove pipe %s error: %v \n", err)
	}
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
			// Kill the init process, and all processes must be stopped.
			//if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			//	errorln("Kill init process error: %v", err)
			//}
			log.Printf("Kill init process: %d \n", c.Pid)
			syscall.Kill(c.Pid, syscall.SIGKILL)

		case evChild:

		case evExec:
			er := ev.data.(*proto.ExecRequest)
			command, err := json.Marshal(er)
			if err != nil {
				log.Printf("Encode exec error: %v \n", err)
				break
			}

			execFunc := func() error {
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

				var wg sync.WaitGroup
				wg.Add(2)

				ro, wo, err := os.Pipe()
				if err != nil {
					Funlock(lock)
					return err
				}
				outbuf := bytes.NewBuffer(make([]byte, 0, 1000))
				go func() {
					defer wg.Done()
					io.Copy(outbuf, ro)
					ro.Close()
				}()

				re, we, err := os.Pipe()
				if err != nil {
					Funlock(lock)
					return err
				}
				errbuf := bytes.NewBuffer(make([]byte, 0, 1000))
				go func() {
					defer wg.Done()
					io.Copy(errbuf, re)
					re.Close()
				}()

				cmd := &exec.Cmd{
					Dir:    "/tmp",
					Path:   "/proc/self/exe",
					Args:   []string{"setns", c.Name},
					Stdout: wo,
					Stderr: we,
				}
				if cmd.SysProcAttr == nil {
					cmd.SysProcAttr = &syscall.SysProcAttr{}
				}
				cmd.ExtraFiles = append(cmd.ExtraFiles, child)

				// 通过环境变量给 setns 进程传递数据.
				cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_INIT_PID__=%d", c.Pid))
				cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_PIPE__=%d", 2+len(cmd.ExtraFiles)))
				cmd.Env = append(cmd.Env, fmt.Sprintf("__TINYBOX_CMD__=%s", string(command)))

				if err := cmd.Start(); err != nil {
					Funlock(lock)
					return fmt.Errorf("Start setns process error: %v", err)
				}

				wo.Close()
				we.Close()

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

				wg.Wait()

				var resp proto.ExecResponse
				resp.Status = proto.Success
				resp.Stdout = outbuf.String()
				resp.Stderr = errbuf.String()

				ev.c <- resp

				return nil
			}

			if err := execFunc(); err != nil {
				log.Printf("%v \n", err)
			}

			close(ev.c)

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
