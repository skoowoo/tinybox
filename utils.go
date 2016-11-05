package tinybox

import (
	"os"
	"syscall"
)

func MkdirIfNotExist(name string) error {
	if _, err := os.Lstat(name); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(name, 0733); err != nil {
			return err
		}
	}
	return nil
}

func Flock(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)

	return file, err
}

func Funlock(file *os.File) error {
	defer file.Close()
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
