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

func NewContainer() *Container {
	c := new(Container)
	c.nsop = newNamespace()
	c.fsop = &rootFs{}
	c.master = master()
	c.setns = setns()
	c.init = &initProcess{}

	return c
}

func (c *Container) MasterStart() error {
	return c.master.Start(c)
}

func (c *Container) MasterWait() error {
	return c.master.Wait(c)
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

// Sethostname set container's hostname.
func (c *Container) sethostname() error {
	return syscall.Sethostname([]byte(c.Hostname))
}
