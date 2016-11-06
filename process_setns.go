package tinybox

import (
	"encoding/json"
	"log"
	"os"
	"syscall"

	"github.com/skoo87/tinybox/proto"
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

	var er proto.ExecRequest
	if err := json.Unmarshal([]byte(cmd), &er); err != nil {
		return err
	}

	lock, err := Flock(c.LockFile())
	if err != nil {
		return err
	}
	Funlock(lock)

	return syscall.Exec(er.Path, er.Argv, os.Environ())
}
