package tinybox

import (
	"fmt"
)

type Errors struct {
	desc string
	err  error
}

func (e Errors) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s(%v)", e.desc, e.err)
	}
	return fmt.Sprintf("%s", e.desc)
}
