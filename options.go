package tinybox

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
)

var (
	ErrOptInvalid     = fmt.Errorf("Invalid options")
	ErrOptNoRun       = fmt.Errorf("Not set run command or invalid")
	ErrOptNoRoot      = fmt.Errorf("Not set root path or invalid")
	ErrOptInvalidName = fmt.Errorf("Invalid container's name")
)

// tinybox --run='' --name='' --root=''
// tinybox --exe='' --name=''

type Options struct {
	run      string
	exec     string
	argv     string
	args     []string
	name     string
	root     string
	wd       string
	hostname string
}

func (o *Options) register() {
	flag.StringVar(&o.run, "run", "", "Container run command")
	flag.StringVar(&o.exec, "exec", "", "")
	flag.StringVar(&o.root, "root", "", "Container rootfs path")
	flag.StringVar(&o.wd, "wd", "/", "Container working directory")
	flag.StringVar(&o.hostname, "hostname", "", "Container host name")
}

func (o *Options) Parse() error {
	if len(os.Args) < 2 {
		return ErrOptInvalid
	}

	if o.name = os.Args[1]; o.name == "" {
		return ErrOptInvalidName
	}

	if os.Args[0] == "init" || os.Args[0] == "setns" {
		return nil
	}

	var tmp []string
	tmp = append(tmp, os.Args[0])
	tmp = append(tmp, os.Args[2:]...)
	os.Args = tmp

	o.register()
	flag.Parse()

	o.argv = o.exec

	var err error

	if o.run != "" {
		if o.argv, o.args, err = parseRun(o.run); err != nil {
			return err
		}

		if o.root != "" && !path.IsAbs(o.root) {
			return ErrOptNoRoot
		}
	}

	return nil
}

func (o *Options) IsExec() bool {
	return o.run == "" && o.exec != ""
}

func parseRun(run string) (string, []string, error) {
	args := strings.Fields(run)
	if len(args) == 0 {
		return "", nil, ErrOptNoRun
	}
	return args[0], args, nil
}
