package tinybox

import (
	"os/exec"
)

type base struct {
	cmd *exec.Cmd
}

func (p *base) Start(c *Container) error {
	return nil
}

func (p *base) Wait(c *Container) error {
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
