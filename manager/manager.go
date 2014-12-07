package manager

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dockpit/go-dockerclient"

	"github.com/dockpit/dirtar"
)

type Manager struct {
	client *docker.Client
	host   string
}

// the Docker image that is used for mocks
var ImageName = "dockpit/mock:latest"
var MockPrivatePort int64 = 8000
var ReadyExp = regexp.MustCompile(".*serving on.*")
var ReadyInterval = time.Millisecond * 50
var ReadyTimeout = time.Second * 1

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

// hash a path using md5
func containerName(path string) (string, error) {

	//create md5 of full path
	hash := md5.New()
	_, err := hash.Write([]byte(path))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("pitmock_%s", hex.EncodeToString(hash.Sum(nil))), nil
}

// start a mock container by using examples from the given directory
func (m *Manager) Start(dir string, portb map[docker.Port][]docker.PortBinding) (*MockContainer, error) {

	//create name for container
	cname, err := containerName(dir)
	if err != nil {
		return nil, err
	}

	//@todo, grab the first and set private port to expected mock service
	//to the mock porcess binding
	for _, pb := range portb {
		portb[docker.Port(strconv.FormatInt(MockPrivatePort, 10)+"/tcp")] = pb
		break
	}

	//create the container
	c, err := m.client.CreateContainer(docker.CreateContainerOptions{
		Name: cname,
		Config: &docker.Config{
			Image: ImageName,
		},
	})

	if err != nil {
		return nil, err
	}

	//start the container we created
	err = m.client.StartContainer(c.ID, &docker.HostConfig{PortBindings: portb})
	if err != nil {
		return nil, err
	}

	//wait for mock to be ready

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

	//tar examples into memory
	tar := bytes.NewBuffer(nil)
	err = dirtar.Tar(dir, tar)
	if err != nil {
		return nil, err
	}

	//'ping' logs until we got something that indicates
	// it started
	to := make(chan bool, 1)
	go func() {
		time.Sleep(ReadyTimeout)
		to <- true
	}()

	//start pinging logs
	var buf bytes.Buffer
	for {

		buf.Reset()
		err = m.client.Logs(docker.LogsOptions{
			Container:    c.ID,
			OutputStream: &buf,
			ErrorStream:  &buf,
			Stdout:       true,
			Stderr:       true,
			RawTerminal:  true,
		})
		if err != nil {
			return nil, err
		}

		//if it matches; break loop we can continue
		if ReadyExp.MatchString(buf.String()) {
			break
		}

		select {
		case <-to:
			return nil, fmt.Errorf("Mock server starting timed out")
			break
		case <-time.After(ReadyInterval):
			continue
		}

	}

	//get the external port for 8000 and turn into an url we can send http requests to
	//@todo, here we assume that the first configured port is the http interface, indicate explicetly
	//@todo, use the logic on line 77
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

	//create name for container
	cname, err := containerName(dir)
	if err != nil {
		return err
	}

	//get all containers
	cs, err := m.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return err
	}

	//get container that matches the name
	// var container *docker.APIContainers
	var container docker.APIContainers
	for _, c := range cs {
		for _, n := range c.Names {
			if n[1:] == cname {
				container = c
			}
		}
	}

	//remove hard since mocks are ephemeral
	return m.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            container.ID,
		RemoveVolumes: true,
		Force:         true,
	})
}
