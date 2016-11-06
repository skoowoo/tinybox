package tinybox

import (
	"log"
	"syscall"
)

type initProcess struct {
	base
}

func (p *initProcess) Exec(c *Container) error {
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

	return syscall.Exec(c.Path, c.Argv, nil)
}
