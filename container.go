package tinyjail

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"
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
	Path     string        `json:"path"`
	Argv     []string      `json:"argv"`
	Pid      int           `json:"pid"`
	Name     string        `json:"name"`
	Hostname string        `json:"hostname"`
	nsop     namespaceOper `json:"-"`
	cgop     cgroupOper    `json:"-"`
	fsop     rootfsOper    `json:"-"`
	init     process       `json:"-"`
	master   process       `json:"-"`
}

func NewContainer() *Container {
	c := new(Container)
	c.nsop = newNamespace()
	c.fsop = &rootFs{}
	c.master = &masterProcess{}
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

// Dir return container's root dir, not rootfs.
func (c *Container) Dir() string {
	return path.Join(workRoot, c.Name)
}

// PipeFile return container's pipe file path.
func (c *Container) PipeFile() string {
	return filepath.Join(c.Dir(), "pipe")
}

// Sethostname set container's hostname.
func (c *Container) sethostname() error {
	return syscall.Sethostname([]byte(c.Hostname))
}
