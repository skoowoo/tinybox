package tinybox

import (
	"os/exec"
)

type process interface {
	Start(*Container) error
	Exec(*Container) error
	Pid() int
}

type base struct {
	cmd *exec.Cmd
}

func (p *base) Start(c *Container) error {
	return nil
}

func (p *base) Exec(c *Container) error {
	return nil
}

func (p *base) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}
