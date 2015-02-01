package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dockpit/mock/server"
)

var Version = "0.0.0-DEV"
var Build = "unbuild"
var Bind = ":8000"

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	path := filepath.Join(wd, ".example", "examples")

	//create the server, default to fileparser
	s := server.NewServer(Bind, path)

	//send relevant signals to server
	signal.Notify(s.Reload, syscall.SIGHUP)
	signal.Notify(s.Stop, os.Kill, os.Interrupt)

	//handle errors
	go func() {
		for err := range s.Errors {
			log.Println(err)
		}
	}()

	log.Printf("Dockpit Mock %s (%s), serving on (%s)...\n", Version, Build, Bind)
	err = s.Serve()
	if err != nil {
		log.Fatal("after serve:", err)
	}

	log.Printf("Stopping...\n")
}
