package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/zenazn/goji/bind"

	"github.com/dockpit/lang"
	"github.com/dockpit/lang/manifest"
)

type Server struct {
	Reload chan os.Signal
	Stop   chan os.Signal
	Errors chan error

	dir      string
	listener net.Listener
	parser   *lang.Parser
}

func NewServer(b, dir string) *Server {
	return &Server{
		Reload: make(chan os.Signal),
		Stop:   make(chan os.Signal),
		Errors: make(chan error),

		dir:      dir,
		listener: bind.Socket(b),
		parser:   lang.NewParser(dir),
	}
}

// read examples from the filesystem and (re)create the
// the server mux
func (s *Server) loadExamples() error {

	//create contract data using the parser
	cd, err := s.parser.Parse()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Failed to open .example/examples in '%s', is this a Dockpit project?", s.dir)
		}

		return fmt.Errorf("Parsing error: %s", err)
	}

	//create contract from data
	c, err := manifest.NewContract(cd)
	if err != nil {
		return fmt.Errorf("Failed to create contract from parsed data: %s", err)
	}

	//create mux from contract
	mock := NewMock(c, s.dir)
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
