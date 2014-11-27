package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dockpit/mock/server"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	//create the server
	s := server.NewServer(":8000", filepath.Join(wd, ".dockpit", "examples"))

	//send relevant signals to server
	signal.Notify(s.Reload, syscall.SIGHUP)
	signal.Notify(s.Stop, os.Kill, os.Interrupt)

	//handle errors
	go func() {
		for err := range s.Errors {
			log.Println(err)
		}
	}()

	log.Printf("Serving...\n")
	err = s.Serve()
	if err != nil {
		log.Fatal("after serve:", err)
	}

	log.Printf("Stopping...\n")
}
