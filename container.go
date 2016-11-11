package tinybox

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

type namespaceOper interface {
	Cloneflags(*Container) uintptr
	Setup(*Container) error
}

type cgroupOper interface {
	Paths() map[string]string
	Memory(*Container) error
	CPU(*Container) error
	CpuAcct(*Container) error
	CpuSet(*Container) error
}

type rootfsOper interface {
	Chroot(*Container) error
	Mount(*Container) error
	Unmount(*Container) error
}

type Container struct {
	Name string `json:"name"` // container's name
	Dir  string `json:"dir"`

	Rootfs   string   `json:"rootfs"`
	Path     string   `json:"path"` // the binary path of the first process.
	Argv     []string `json:"argv"`
	Hostname string   `json:"hostname"`
	CgPrefix string

	Pid int `json:"pid"` // process id of the init process

	nsop   namespaceOper `json:"-"`
	cgop   cgroupOper    `json:"-"`
	fsop   rootfsOper    `json:"-"`
	init   process       `json:"-"`
	master process       `json:"-"`
	setns  process       `json:"-"`
	isExec bool          `json:"-"`
}

func NewContainer() (*Container, error) {
	var opt Options
	if err := opt.Parse(); err != nil {
		return nil, err
	}

	c := new(Container)
	c.Name = opt.name
	c.Dir = filepath.Join("/var/run/tinybox", c.Name)
	c.isExec = opt.IsExec()
	c.CgPrefix = "tinybox"

	var err error
	if c.cgop, err = newCGroup(); err != nil {
		return nil, err
	}
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

	if opt.IsExec() {
		info, err := ioutil.ReadFile(c.JsonFile())
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(info, c); err != nil {
			return nil, err
		}

		c.Path = opt.argv
		c.Argv = nil
		c.Hostname = ""
		c.Rootfs = ""

		return c, nil
	}

	c.Rootfs = opt.root
	c.Path = opt.argv
	c.Argv = opt.args
	c.Hostname = opt.hostname

	return c, nil
}

func (c *Container) WaitJson() error {
	return c.readPipe()
}

func (c *Container) MasterStart() error {
	return c.master.Start(c)
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

func (c *Container) PipeFile() string {
	return filepath.Join(c.Dir, "pipe")
}

func (c *Container) LockFile() string {
	return filepath.Join(c.Dir, "lock")
}

func (c *Container) JsonFile() string {
	return filepath.Join(c.Dir, "container.json")
}
