package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/skoo87/tinybox"
	_ "github.com/skoo87/tinybox/nsenter"
)

func main() {
	c, err := tinybox.NewContainer()
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.OpenFile(filepath.Join(c.Dir, "log"), syscall.O_RDWR|syscall.O_CREAT|syscall.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Open log file error: %v \n", err)
	}
	log.SetOutput(f)

	log.Printf(">>>>> %v \n", os.Args)

	// init child process
	if os.Args[0] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		log.SetPrefix("init: ")

		if err := c.LoadJson(); err != nil {
			log.Fatalf("Init process load container error: %v", err)
		}

		if err := c.InitExec(); err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	}

	// setns child process
	if os.Args[0] == "setns" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		log.SetPrefix("setns: ")

		if err := c.SetnsStart(); err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	}

	log.SetPrefix("master: ")

	// master parent process
	if err := c.MasterStart(); err != nil {
		log.Fatalln(err)
	}
	c.MasterWait()
}
