package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dockpit/lang"
	"github.com/dockpit/lang/parser"
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

	//@todo duplicate with pit/command/types.go
	//get files in dir
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Failed to open examples in '%s', is this a Dockpit project?", path)
		}

		log.Fatal(err)
	}

	//any markdown files in path
	isMarkdown := false
	for _, fi := range fis {
		if filepath.Ext(fi.Name()) == ".md" {
			isMarkdown = true
		}
	}

	//if so use markdown parser
	var p parser.Parser
	if isMarkdown {
		p = lang.MarkdownParser(path)
	} else {
		p = lang.FileParser(path)
	}

	//create the server
	s := server.NewServer(Bind, path, p)

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
