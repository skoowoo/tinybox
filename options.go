package tinybox

import (
	"flag"
	"fmt"
	"path"
	"strings"
)

// tinybox --run='' --name='' --root=''
// tinybox --exe='' --name=''

type Options struct {
	runCmd  string
	exeCmd  string
	name    string
	root    string
	workDir string
}

func (o *Options) register() {
	flag.StringVar(&o.runCmd, "run", "", "Container run command")
	flag.StringVar(&o.exeCmd, "exe", "", "Container exe command")
	flag.StringVar(&o.root, "root", "", "Container rootfs path")
	flag.StringVar(&o.name, "name", "", "Container name")
	flag.StringVar(&o.workDir, "work", "/var/run/tinybox", "")
}

func (o *Options) Parse() error {
	o.register()
	flag.Parse()

	if o.runCmd == "" && o.runCmd == "" {
		return fmt.Errorf("Not set command")
	}
	if o.root != "" && !path.IsAbs(o.root) {
		return fmt.Errorf("Root not absolute path: %s", o.root)
	}
	if o.name == "" {
		return fmt.Errorf("Not set container's name")
	}

	return nil
}

func (o *Options) ParseCmd(cmd string) (string, []string, error) {
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return "", nil, CmdError{cmd}
	}
	return args[0], args, nil
}

type CmdError struct {
	cmd string
}

func (e CmdError) Error() string {
	return fmt.Sprintf("options: Invalid command \"%s\"", e.cmd)
}
