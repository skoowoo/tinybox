package tinyjail

import (
	"syscall"
)

func newNamespace() NamespaceManager {
	return NamespaceManager{
		"MNT":  &setNS{clone: syscall.CLONE_NEWNS},
		"UTS":  &setUTS{clone: syscall.CLONE_NEWUTS},
		"PID":  &setPID{clone: syscall.CLONE_NEWPID},
		"NET":  &setNET{clone: syscall.CLONE_NEWNET},
		"USER": &setUSER{clone: syscall.CLONE_NEWUSER},
		"IPC":  &setIPC{clone: syscall.CLONE_NEWIPC},
	}
}

type namespaceSetter interface {
	setup(*Container) error
	flag(*Container) uintptr
}

type NamespaceManager map[string]namespaceSetter

func (m NamespaceManager) Cloneflags(c *Container) uintptr {
	if c.Rootfs == "" {
		c.Hostname = "" // If not set rootfs, don't set namespace and hostname.
		return 0
	}

	var flag uintptr

	for _, set := range m {
		flag |= set.flag(c)
	}
	return flag
}

func (m NamespaceManager) Setup(c *Container) error {
	return nil
}

type baseN struct{}

func (b baseN) setup(c *Container) error {
	return nil
}

// Set mount namespace.
type setNS struct {
	clone int
	baseN
}

func (s setNS) flag(c *Container) uintptr {
	return uintptr(s.clone)
}

// Set uts namespace.
type setUTS struct {
	clone int
	baseN
}

func (s setUTS) flag(c *Container) uintptr {
	return uintptr(s.clone)
}

// Set pid namespace.
type setPID struct {
	clone int
	baseN
}

func (s setPID) flag(c *Container) uintptr {
	return uintptr(s.clone)
}

// Set net namespace.
type setNET struct {
	clone int
	baseN
}

func (s setNET) flag(c *Container) uintptr {
	return uintptr(0)
}

// Set user namespace.
type setUSER struct {
	clone int
	baseN
}

func (s setUSER) flag(c *Container) uintptr {
	return uintptr(0)
}

// Set ipc namespace.
type setIPC struct {
	clone int
	baseN
}

func (s setIPC) flag(c *Container) uintptr {
	return uintptr(s.clone)
}
