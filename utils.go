package tinybox

import (
	"os"
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
