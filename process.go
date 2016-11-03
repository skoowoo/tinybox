package tinybox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

func mkdirIfNotExist(name string) error {
	if _, err := os.Lstat(name); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(name, 0733); err != nil {
			return err
		}
	}
	return nil
}

type base struct {
	cmd *exec.Cmd
}

func (p *base) Start(c *Container) error {
	return nil
}

func (p *base) Wait(c *Container) error {
	return nil
}

func (p *base) Exec(c *Container) error {
	return nil
}

func (p *base) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// setns process
type setnsProcess struct {
	base
}

func setns() *setnsProcess {
	return new(setnsProcess)
}

func (p *setnsProcess) Start(c *Container) error {
	cmd := os.Getenv("__TINYBOX_CMD__")
	if cmd == "" {
		return nil
	}

	debugln("setns command: %s", cmd)

	var er proto.ExecRequest
	if err := json.Unmarshal([]byte(cmd), &er); err != nil {
		return err
	}

	return syscall.Exec(er.Path, er.Argv, nil)
}

// init process
type initProcess struct {
	base
}

func (p *initProcess) Exec(c *Container) error {
	// Recv info from socket pipe, the info come from runtime process.
	if err := c.readPipe(); err != nil {
		return err
	}

	logPrefix(c.Name)

	// Mount filesystem
	if err := c.fsop.Mount(c); err != nil {
		return err
	}

	// Chroot, if have root path.
	if c.Rootfs != "" {
		if err := c.fsop.Chroot(c); err != nil {
			return err
		}
	}

	// Do nothing after Chroot.

	debugln("Container info: %+v", c)
	debugln("Container init process: pid=%d", os.Getpid())
	if h, err := os.Hostname(); err == nil {
		debugln("Container hostname: %s", h)
	}

	return syscall.Exec(c.Path, c.Argv, nil)
}

// master process
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
	if err := p.opt.Parse(); err != nil {
		return err
	}

	logPrefix(p.opt.name)

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
			errorln("Http server error: %v", err)
			os.Exit(1)
		}
	}()

	// Mkdir /var/run/tinybox.
	if err := mkdirIfNotExist(p.opt.workDir); err != nil {
		return err
	}

	c.Name = p.opt.name
	c.Rootfs = p.opt.root
	c.Dir = filepath.Join(p.opt.workDir, p.opt.name)

	arg0, args, err := p.opt.ParseCmd(p.opt.runCmd)
	if err != nil {
		return err
	}
	c.Path = arg0
	c.Argv = args

	// Mkdir container's dir.
	if err := mkdirIfNotExist(c.Dir); err != nil {
		return err
	}

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
		Args:        []string{"init", c.Dir},
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
	if err := os.Remove(c.UnixFile()); err != nil {
		errorln("Remove unix socket %s error: %v", err)
	}
	if err := os.Remove(c.PipeFile()); err != nil {
		errorln("Remove pipe %s error: %v", err)
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
	debugln("handle stop")

	ev := event{
		action: evStop,
	}

	select {
	case c <- ev:
	case <-time.After(time.Second * 5):
		warnln("Send event timeout: %ss", 5)
	}
}

func childHandle(sig os.Signal, c chan event) {
	debugln("handle child")

	ev := event{
		action: evChild,
	}

	select {
	case c <- ev:
	case <-time.After(time.Second * 5):
		warnln("Send event timeout: %ss", 5)
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
			infoln("Trap signal: %s", sig)

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

		debugln("Receive event: %s", ev.action)

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
				errorln("Encode exec error: %v", err)
			}

			execFn := func() error {
				cmd := &exec.Cmd{
					Dir:    "/tmp",
					Path:   "/proc/self/exe",
					Args:   []string{"setns"},
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
				errorln("%v", err)
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
