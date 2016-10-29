package tinyjail

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

// tinyjail --run='' --name='' --root=''
// tinyjail --exe='' --name=''

var options Options

func init() {
	flag.StringVar(&options.runCmd, "run", "", "Container run command")
	flag.StringVar(&options.exeCmd, "exe", "", "Container exe command")
	flag.StringVar(&options.root, "root", "", "Container rootfs path")
	flag.StringVar(&options.name, "name", "", "Container name")
}

type Options struct {
	runCmd  string
	exeCmd  string
	name    string
	root    string
	working string
}

func (o *Options) Init() error {
	flag.Parse()

	if o.runCmd == "" && o.runCmd == "" {
		return fmt.Errorf("Not set command")
	}
	if wd, err := os.Getwd(); err != nil {
		return err
	} else {
		o.working = wd
	}
	if o.root != "" && !path.IsAbs(o.root) {
		return fmt.Errorf("Root not absolute path: %s", o.root)
	}
	if o.name == "" {
		return fmt.Errorf("Not set container's name")
	}
	return nil
}

func (o *Options) ParseRunCmd() (*exec.Cmd, error) {
	if o.runCmd == "" {
		return nil, CmdError{o.runCmd}
	}
	args := strings.Fields(o.runCmd)
	if len(args) == 0 {
		return nil, CmdError{o.runCmd}
	}
	return &exec.Cmd{
		Path: args[0],
		Args: args,
	}, nil
}

type CmdError struct {
	cmd string
}

func (e CmdError) Error() string {
	return fmt.Sprintf("options: Invalid command \"%s\"", e.cmd)
}
