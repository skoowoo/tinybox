package tinybox

import (
	"log"
	"os"
	"syscall"
)

type initProcess struct {
	base
}

func (p *initProcess) Exec(c *Container) error {
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

	if debug {
		log.Printf("Container info: %+v \n", c)
		log.Printf("Container init process: pid=%d \n", os.Getpid())
		if h, err := os.Hostname(); err == nil {
			log.Printf("Container hostname: %s \n", h)
		}
	}

	return syscall.Exec(c.Path, c.Argv, nil)
}
