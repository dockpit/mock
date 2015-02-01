package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/zenazn/goji/bind"

	"github.com/dockpit/lang"
	"github.com/dockpit/lang/manifest"
	"github.com/dockpit/lang/parser"
)

type Server struct {
	Reload chan os.Signal
	Stop   chan os.Signal
	Errors chan error

	dir      string
	listener net.Listener
	parser   parser.Parser
}

// @todo, lang.NewParser changed to markdown/file version
func NewServer(b, dir string) *Server {

	return &Server{
		Reload: make(chan os.Signal),
		Stop:   make(chan os.Signal),
		Errors: make(chan error),

		listener: bind.Socket(b),
		dir:      dir,
	}
}

// read examples from the filesystem and (re)create the
// the server mux
func (s *Server) loadExamples() error {

	//determine parser type again after upload
	//@todo duplicate with pit/command/types.go
	//get files in dir
	fis, err := ioutil.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Failed to open examples in '%s', is this a Dockpit project?", s.dir)
		}

		return err
	}

	//any markdown files in path
	isMarkdown := false
	for _, fi := range fis {
		if filepath.Ext(fi.Name()) == ".md" {
			isMarkdown = true
		}
	}

	//if so use markdown parser
	if isMarkdown {
		s.parser = lang.MarkdownParser(s.dir)
	} else {
		s.parser = lang.FileParser(s.dir)
	}

	//create manifest data using the parser
	//@todo, these errors are not reported in tests
	md, err := s.parser.Parse()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Failed to open .example/examples in '%s', is this a Dockpit project?", s.dir)
		}

		return fmt.Errorf("Parsing error: %s", err)
	}

	//create manifest from data
	m, err := manifest.NewManifest(md)
	if err != nil {
		return fmt.Errorf("Failed to create manifest from parsed data: %s", err)
	}

	//create mux from manifest
	mock := NewMock(m, s.dir)
	mux, err := mock.Mux()
	if err != nil {
		return err
	}

	//create and replace default mux
	h := http.NewServeMux()
	h.Handle("/", mux)
	http.DefaultServeMux = h

	return nil
}

// Handles signals on the reload and stop channels
func (s *Server) handleSignals() {
	for {
		select {

		//incase of a KILL/INT signal: stop listening
		case <-s.Stop:

			//close the current listener which
			//unblocks the http.Serve()
			s.listener.Close()

		//in case of a HUP signal: reload the mux
		case <-s.Reload:

			//reload the default mux
			err := s.loadExamples()
			if err != nil {
				s.Errors <- err
			}

		}
	}
}

// start serving on the listener, blocks until the listener is closed
func (s *Server) Serve() error {

	//start handling signals
	go s.handleSignals()

	//load examples into the default http mux
	err := s.loadExamples()
	if err != nil {
		return err
	}

	//start serving using default http mux
	err = http.Serve(s.listener, nil)
	if err != nil {

		//@todo do something like this http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
		if strings.HasSuffix(err.Error(), "use of closed network connection") {
			return nil
		}

		return err
	}

	return nil
}
