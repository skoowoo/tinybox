package tinybox

import (
	"fmt"
	"path/filepath"
	"strconv"
)

func init() {
	registerSetter(&defaultCpu{})
	registerSetter(&defaultCpuSet{})
}

type defaultCpu struct{}

func (d defaultCpu) IsSubsys(typ string) bool {
	return typ == subsysCPU
}

func (d defaultCpu) Validate(opt *CGroupOptions) error {
	if _, err := strconv.Atoi(opt.CpuShares); err != nil {
		return err
	}
	if _, err := strconv.Atoi(opt.CpuCfsPeriod); err != nil {
		return err
	}
	if _, err := strconv.Atoi(opt.CpuCfsquota); err != nil {
		return err
	}
	return nil
}

func (d defaultCpu) Write(opt *CGroupOptions, dir string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	if opt.CpuShares != "0" {
		WriteFileWithPanic(filepath.Join(dir, "cpu.shares"), opt.CpuShares)
	}
	if opt.CpuCfsPeriod != "0" {
		WriteFileWithPanic(filepath.Join(dir, "cpu.cfs_period_us"), opt.CpuCfsPeriod)
	}
	if opt.CpuCfsquota != "0" {
		WriteFileWithPanic(filepath.Join(dir, "cpu.cfs_quota_us"), opt.CpuCfsquota)
	}
	return
}

type defaultCpuSet struct{}

func (d defaultCpuSet) IsSubsys(typ string) bool {
	return typ == subsysCS
}

func (d defaultCpuSet) Validate(opt *CGroupOptions) error {
	return nil
}

func (d defaultCpuSet) Write(opt *CGroupOptions, dir string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	if opt.CpusetCpus != "" {
		WriteFileWithPanic(filepath.Join(dir, "cpuset.cpus"), opt.CpusetCpus)
	}
	if opt.CpusetMems != "" {
		WriteFileWithPanic(filepath.Join(dir, "cpuset.mems"), opt.CpusetMems)
	}
	return
}
