package tinyjail

import (
	"path"
	"syscall"
)

type rootFs struct{}

func (fs *rootFs) Mount(c *Container) error {
	flag := syscall.MS_SLAVE | syscall.MS_REC

	if err := syscall.Mount("", "/", "", uintptr(flag), ""); err != nil {
		return err
	}

	if err := syscall.Mount(c.Rootfs, c.Rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}

	if err := syscall.Mount("proc", path.Join(c.Rootfs, "proc"), "proc", 0, ""); err != nil {
		return err
	}

	return nil
}

func (fs *rootFs) Chroot(c *Container) error {
	if err := syscall.Chdir(c.Rootfs); err != nil {
		return err
	}

	if err := syscall.Mount(c.Rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return err
	}

	if err := syscall.Chroot("."); err != nil {
		return err
	}
	return syscall.Chdir("/")
}
