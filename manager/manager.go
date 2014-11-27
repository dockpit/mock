package manager

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsouza/go-dockerclient"

	"github.com/dockpit/dirtar"
)

type Manager struct {
	client *docker.Client
	host   string
}

// the Docker image that is used for mocks
var ImageName = "dockpit/mock:latest"
var MockPrivatePort int64 = 8000

// Manages state for microservice testing by creating
// docker images and starting containers when necessary
func NewManager(host, cert string) (*Manager, error) {
	m := &Manager{
		host: host,
	}

	//parse docker host addr as url
	hurl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	//change to http connection
	hurl.Scheme = "https"

	//create docker client
	m.client, err = docker.NewTLSClient(hurl.String(), filepath.Join(cert, "cert.pem"), filepath.Join(cert, "key.pem"), filepath.Join(cert, "ca.pem"))
	if err != nil {
		return nil, err
	}

	return m, nil
}

// start a mock container by using examples from the given directory
func (m *Manager) Start(dir string) (*MockContainer, error) {

	//create the container
	c, err := m.client.CreateContainer(docker.CreateContainerOptions{
		// Name: fmt.Sprintf("pitmock_%s", filepath.Base(dep)),
		Config: &docker.Config{Image: ImageName},
	})

	if err != nil {
		return nil, err
	}

	//start the container we created
	err = m.client.StartContainer(c.ID, &docker.HostConfig{PublishAllPorts: true})
	if err != nil {
		return nil, err
	}

	//tar examples into memory
	tar := bytes.NewBuffer(nil)
	err = dirtar.Tar(dir, tar)
	if err != nil {
		return nil, err
	}

	//get container port mapping
	ci, err := m.client.InspectContainer(c.ID)
	if err != nil {
		return nil, err
	}

	//use docker host location to form url
	hurl, err := url.Parse(m.host)
	if err != nil {
		return nil, err
	}

	//wait for container to settle?
	//@todo very dirty business here
	<-time.After(time.Millisecond * 200)

	//get the external port for 8000 and turn into an url we can send http requests to
	hurl.Scheme = "http"
	for _, pconfig := range ci.NetworkSettings.PortMappingAPI() {
		if pconfig.PrivatePort == MockPrivatePort {
			hurl.Host = strings.Replace(hurl.Host, ":2376", fmt.Sprintf(":%d", pconfig.PublicPort), 1)
		}
	}

	//send the upload request
	resp, err := http.Post(fmt.Sprintf("%s/_examples", hurl.String()), "application/x-tar", tar)
	if err != nil {
		return nil, err
	}

	//check if we uploaded correctly
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("Failed to upload examples to container '%s' %s", ci.ID, ci.Name)
	}

	//send HUP signal to 'root' process (dockpit/mock) to reload examples
	err = m.client.KillContainer(docker.KillContainerOptions{
		ID:     c.ID,
		Signal: docker.Signal(syscall.SIGHUP),
	})

	if err != nil {
		return nil, err
	}

	return &MockContainer{c.ID, hurl.String(), dir}, nil
}

// stop a mock container that was started from the given directory
func (m *Manager) Stop(dir string) error {
	//@todo implement
	return fmt.Errorf("Not yet implemented")
}
