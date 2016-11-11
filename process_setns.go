package tinybox

import (
	"log"
	"os"
	"os/exec"
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

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}

	return syscall.Exec(path, argv, os.Environ())
}
