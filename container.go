package tinybox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type process interface {
	Start(*Container) error
	Wait(*Container) error
	Exec(*Container) error
	Pid() int
}

type namespaceOper interface {
	Cloneflags(*Container) uintptr
	Setup(*Container) error
}

type cgroupOper interface {
}

type rootfsOper interface {
	Chroot(*Container) error
	Mount(*Container) error
	Unmount(*Container) error
}

type Container struct {
	Rootfs   string        `json:"rootfs"`
	Path     string        `json:"path"` // the binary path of the first process.
	Argv     []string      `json:"argv"`
	Pid      int           `json:"pid"`  // process id of the init process
	Name     string        `json:"name"` // container's name
	Hostname string        `json:"hostname"`
	Running  bool          `json:"running"`
	Uptime   time.Time     `json:"uptime"`
	Dir      string        `json:"dir"`
	nsop     namespaceOper `json:"-"`
	cgop     cgroupOper    `json:"-"`
	fsop     rootfsOper    `json:"-"`
	init     process       `json:"-"`
	master   process       `json:"-"`
	setns    process       `json:"-"`
}

func NewContainer() (*Container, error) {
	var opt Options
	if err := opt.Parse(); err != nil {
		return nil, err
	}

	c := new(Container)
	c.Name = opt.name
	c.Dir = filepath.Join("/var/run/tinybox", c.Name)
	c.Path = opt.argv
	c.Argv = opt.args
	c.Rootfs = opt.root

	c.nsop = newNamespace()
	c.fsop = &rootFs{}
	c.master = master()
	c.setns = setns()
	c.init = &initProcess{}

	if err := MkdirIfNotExist(c.Dir); err != nil {
		return nil, err
	}

	// Create named pipe.
	if _, err := os.Lstat(c.PipeFile()); err != nil {
		if os.IsNotExist(err) {
			if err := syscall.Mkfifo(c.PipeFile(), 0); err != nil {
				return nil, err
			}
		}
	}

	if _, err := os.Lstat(c.LockFile()); err != nil {
		if os.IsNotExist(err) {
			if _, err := os.Create(c.LockFile()); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}

func (c *Container) MasterStart() error {
	return c.master.Start(c)
}

func (c *Container) MasterWait() error {
	return c.master.Wait(c)
}

func (c *Container) LoadJson() error {
	return c.readPipe()
}

func (c *Container) InitExec() error {
	return c.init.Exec(c)
}

func (c *Container) SetnsStart() error {
	return c.setns.Start(c)
}

// writePipe write the json of Container into pipe
func (c *Container) writePipe() error {
	pipe, err := os.OpenFile(c.PipeFile(), os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Write container pipe: %v", err)
	}
	defer pipe.Close()

	return json.NewEncoder(pipe).Encode(c)
}

// readPipe read the json of Container from pipe
func (c *Container) readPipe() error {
	pipe, err := os.OpenFile(c.PipeFile(), os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("Read container pipe: %v", err)
	}
	defer pipe.Close()

	return json.NewDecoder(pipe).Decode(c)
}

// PipeFile return container's pipe file path.
func (c *Container) PipeFile() string {
	return filepath.Join(c.Dir, "pipe")
}

func (c *Container) UnixFile() string {
	return filepath.Join(c.Dir, "unix.sock")
}

func (c *Container) LockFile() string {
	return filepath.Join(c.Dir, "lock")
}

// Sethostname set container's hostname.
func (c *Container) sethostname() error {
	return syscall.Sethostname([]byte(c.Hostname))
}
