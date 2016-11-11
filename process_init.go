package tinybox

import (
	"fmt"
	"log"
	"os"
	"syscall"
)

type initProcess struct {
}

func (p *initProcess) Start(c *Container) error {
	if err := c.WaitJson(); err != nil {
		return fmt.Errorf("Init process load container error: %v", err)
	}

	if debug {
		log.Printf("Container info: %+v \n", c)
	}

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

	log.Printf("Run init process: %s, %v", c.Path, c.Argv)

	return syscall.Exec(c.Path, c.Argv, os.Environ())
}
