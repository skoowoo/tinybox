package tinyjail

import (
	"path"
	"syscall"
)

type rootFs struct{}

func (fs *rootFs) Mount(c *Container) error {
	return syscall.Mount("proc", path.Join(c.Rootfs, "proc"), "proc", 0, "")
}

func (fs *rootFs) Chroot(c *Container) error {
	return syscall.Chroot(c.Rootfs)
}
