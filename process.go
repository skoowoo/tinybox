package tinybox

import ()

type process interface {
	Start(*Container) error
}
