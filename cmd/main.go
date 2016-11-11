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

	typ := os.Args[0]
	if err := c.SetByType(typ); err != nil {
		log.Fatalln(err)
	}

	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	log.SetPrefix(typ + ": ")

	if err := c.P.Start(c); err != nil {
		log.Fatalln(err)
	}
}
