package tinyjail

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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

	if c.Hostname != "" {
		if err := c.sethostname(); err != nil {
			return err
		}
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
	opt  Options
	ec   chan event
	sigs map[os.Signal]func(os.Signal, chan event)
	stop int32
}

func master() *masterProcess {
	return &masterProcess{
		ec: make(chan event, 10),
		sigs: map[os.Signal]func(os.Signal, chan event){
			syscall.SIGINT:  stopHandle,
			syscall.SIGTERM: stopHandle,
			syscall.SIGCHLD: childHandle,
		},
		stop: 0,
	}
}

func (p *masterProcess) Start(c *Container) error {
	if err := p.opt.Parse(); err != nil {
		return err
	}

	logPrefix(p.opt.name)

	// Mkdir /var/run/tinyjail.
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

	// Save container pid.
	c.Pid = p.Pid()

	// Send info to container init process.
	c.writePipe()
	return nil
}

func (p *masterProcess) Wait(c *Container) error {
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()

		p.signals()
	}()

	go func() {
		wg.Add(1)
		defer wg.Done()

		p.events(c)
	}()

	wg.Wait()
	p.cmd.Wait()

	return nil
}

type event struct {
	sig    os.Signal
	action string
}

// signal function.
func stopHandle(sig os.Signal, c chan event) {
	debugln("handle stop")

	ev := event{
		sig:    sig,
		action: "stop",
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
		sig:    sig,
		action: "child",
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

		case <-time.After(time.Second * 2):
		}

		if atomic.LoadInt32(&p.stop) != 0 {
			return
		}
	}
}

// events handle event, must run it with a goroutine.
func (p *masterProcess) events(c *Container) {
	for {
		ev := <-p.ec

		switch ev.action {
		case "stop":
			// Kill the init process, and all processes must be stopped.
			if err := p.cmd.Process.Kill(); err != nil {
				errorln("Kill init process error: %v", err)
				break
			}

		case "child":
			atomic.StoreInt32(&p.stop, 1)
			return

		default:
		}
	}
}
