package main

import (
	"log"
	"os"
	"runtime"

	"github.com/skoo87/tinybox"
	_ "github.com/skoo87/tinybox/nsenter"
)

func main() {
	c := tinybox.NewContainer()

	// init child process
	if os.Args[0] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()

		c.Dir = os.Args[1]
		if err := c.InitExec(); err != nil {
			log.Fatalln(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// setns child process
	if os.Args[0] == "setns" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()

		// c.Dir = os.Args[1]
		if err := c.SetnsStart(); err != nil {
			log.Fatalln(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// master parent process
	if err := c.MasterStart(); err != nil {
		log.Fatalf("%v \n", err)
		return
	}
	c.MasterWait()
}
