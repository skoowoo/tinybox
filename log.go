package tinyjail

import (
	"log"
	"os"
)

var debug = true

func init() {
	log.SetOutput(os.Stdout)
}

func infoln(format string, v ...interface{}) {
	format = "Info " + format + "\n"
	log.Printf(format, v)
}

func debugln(format string, v ...interface{}) {
	if !debug {
		return
	}

	format = "Debug " + format + "\n"
	log.Printf(format, v)
}

func warnln(format string, v ...interface{}) {
	format = "Warn " + format + "\n"
	log.Printf(format, v)
}

func errorln(format string, v ...interface{}) {
	format = "Error " + format + "\n"
	log.Printf(format, v)
}
