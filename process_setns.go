package tinybox

import (
	"log"
	"os"
	"strings"
	"syscall"
)

type setnsProcess struct {
	base
}

func setns() *setnsProcess {
	return new(setnsProcess)
}

func (p *setnsProcess) Start(c *Container) error {
	cmd := os.Getenv("__TINYBOX_CMD__")
	if cmd == "" {
		return nil
	}

	if debug {
		log.Printf("setns command: %s \n", cmd)
	}

	lock, err := Flock(c.LockFile())
	if err != nil {
		return err
	}
	Funlock(lock)

	argv := strings.Fields(cmd)
	if len(argv) == 0 {
		return nil
	}
	return syscall.Exec(argv[0], argv, os.Environ())
}
