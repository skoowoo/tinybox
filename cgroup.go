package tinybox

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	subsysMEM = "memory"
	subsysCPU = "cpu"
	subsysCA  = "cpuacct"
	subsysCS  = "cpuset"
	subsysDEV = "devices"
	subsysFZ  = "freezer"
	subsysBIO = "blkio"
	subsysHT  = "hugetlb"
)

var subs = []string{
	subsysMEM,
	subsysCPU,
	subsysCA,
	subsysCS,
	subsysDEV,
	subsysFZ,
	subsysBIO,
	subsysHT,
}

type CGroupOptions struct {
}

type CGroupSetter interface {
	IsSubsys(typ string) bool
	Validate(*CGroupOptions) error
	Write(*CGroupOptions, string) error
}

type SetterSlice []CGroupSetter

var setters SetterSlice

func registerSetter(s CGroupSetter) {
	setters = append(setters, s)
}

func (ss SetterSlice) Write(typ, dir string, opt *CGroupOptions) error {
	for _, setter := range ss {
		if !setter.IsSubsys(typ) {
			continue
		}
		if err := setter.Write(opt, dir); err != nil {
			return err
		}
	}
	return nil
}

type CGroup struct {
	mounts map[string]string
	roots  map[string]string
	paths  map[string]string
}

func newCGroup() (*CGroup, error) {
	check := func(str string, path string, tab map[string]string) {
		for _, name := range subs {
			ix := strings.Index(str, name)
			if ix >= 0 {
				last := ix + len(name)

				if last == len(str) {
					tab[name] = path
					continue
				}
				if str[last] == ',' {
					tab[name] = path
					continue
				}
			}
		}
	}

	file, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	roots := make(map[string]string, len(subs))
	br := bufio.NewReader(file)
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		fields := strings.Split(string(line), ":")
		if len(fields) != 3 {
			continue
		}

		check(fields[1], fields[2], roots)
	}

	file2, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file2.Close()

	mounts := make(map[string]string, len(subs))
	br = bufio.NewReader(file2)
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		fields := strings.Fields(string(line))
		if len(fields) != 6 {
			continue
		}
		if typ := fields[2]; typ != "cgroup" {
			continue
		}

		check(filepath.Base(fields[1]), fields[1], mounts)
	}

	cg := new(CGroup)
	cg.roots = roots
	cg.mounts = mounts
	cg.paths = make(map[string]string, len(subs))

	return cg, nil
}

func (cg *CGroup) Paths() map[string]string {
	return cg.paths
}

func (cg *CGroup) Validate(c *Container) error {
	for _, setter := range setters {
		if err := setter.Validate(&c.CgOpts); err != nil {
			return err
		}
	}
	return nil
}

func (cg *CGroup) Memory(c *Container) error {
	group, err := cg.cgroupPath(subsysMEM, c)
	if err != nil {
		return err
	}

	if err := WriteFileInt(filepath.Join(group, "cgroup.procs"), c.Pid); err != nil {
		return err
	}

	cg.paths[subsysMEM] = group
	return setters.Write(subsysMEM, group, &c.CgOpts)
}

func (cg *CGroup) CPU(c *Container) error {
	group, err := cg.cgroupPath(subsysCPU, c)
	if err != nil {
		return err
	}

	if err := WriteFileInt(filepath.Join(group, "cgroup.procs"), c.Pid); err != nil {
		return err
	}

	cg.paths[subsysCPU] = group
	return setters.Write(subsysCPU, group, &c.CgOpts)
}

func (cg *CGroup) CpuAcct(c *Container) error {
	group, err := cg.cgroupPath(subsysCA, c)
	if err != nil {
		return err
	}

	if err := WriteFileInt(filepath.Join(group, "cgroup.procs"), c.Pid); err != nil {
		return err
	}

	cg.paths[subsysCA] = group
	return setters.Write(subsysCA, group, &c.CgOpts)
}

func (cg *CGroup) CpuSet(c *Container) error {
	group, err := cg.cgroupPath(subsysCS, c)
	if err != nil {
		return err
	}

	isEmpty := func(file string) ([]byte, bool, error) {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			if err != io.EOF {
				return nil, false, err
			}
		}
		if len(bytes.Trim(b, " \n\r\t")) == 0 {
			return nil, true, nil
		}
		return b, false, nil
	}

	var tmpFiles []string
	dir := group
	for {
		filename := filepath.Join(dir, "cpuset.cpus")
		bs, empty, err := isEmpty(filename)
		if err != nil {
			return err
		}

		if !empty {
			for i := len(tmpFiles); i > 0; i-- {
				if err := ioutil.WriteFile(tmpFiles[i-1], bs, 0); err != nil {
					return err
				}
			}
			break
		}

		tmpFiles = append(tmpFiles, filename)
		dir = filepath.Dir(dir)
	}

	tmpFiles = tmpFiles[0:0]
	dir = group
	for {
		filename := filepath.Join(dir, "cpuset.mems")
		bs, empty, err := isEmpty(filename)
		if err != nil {
			return err
		}

		if !empty {
			for i := len(tmpFiles); i > 0; i-- {
				if err := ioutil.WriteFile(tmpFiles[i-1], bs, 0); err != nil {
					return err
				}
			}
			break
		}

		tmpFiles = append(tmpFiles, filename)
		dir = filepath.Dir(dir)
	}

	if err := WriteFileInt(filepath.Join(group, "cgroup.procs"), c.Pid); err != nil {
		return err
	}

	cg.paths[subsysCS] = group
	return setters.Write(subsysCS, group, &c.CgOpts)
}

func (cg *CGroup) cgroupPath(name string, c *Container) (string, error) {
	mount := cg.mounts[name]
	root := cg.roots[name]

	if mount == "" || root == "" {
		return "", fmt.Errorf("Not found %s mount or root path", name)
	}

	path := path.Join(mount, root, c.CgPrefix, c.Name)

	if debug {
		log.Printf("mount: %s, root: %s, prefix: %s, name: %s \n", mount, root, c.CgPrefix, c.Name)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}

	return path, nil
}
