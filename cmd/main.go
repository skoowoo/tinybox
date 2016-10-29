package main

import (
	"log"
	"os"

	"github.com/tinyjail"
)

func main() {
	c := tinyjail.NewContainer()

	if os.Args[0] == "init" {
		c.Name = os.Args[1]
		if err := c.InitExec(); err != nil {
			log.Fatalln(err)
		}
		os.Exit(1)
	}

	if err := c.MasterStart(); err != nil {
		log.Fatalf("%v \n", err)
		return
	}
	c.MasterWait()
}
