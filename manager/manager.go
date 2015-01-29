package manager

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/samalba/dockerclient"

	"github.com/dockpit/dirtar"
	"github.com/dockpit/iowait"
)

type Manager struct {
	client *dockerclient.DockerClient
	host   string
}

// the Docker image that is used for mocks
var ImageName = "dockpit/mock:latest"
var MockPrivatePort string = "8000"
var ReadyExp = regexp.MustCompile(".*serving on.*")
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

	//use tlsc?
	var tlsc tls.Config
	if cert != "" {
		c, err := tls.LoadX509KeyPair(filepath.Join(cert, "cert.pem"), filepath.Join(cert, "key.pem"))
		if err != nil {
			return nil, err
		}

		tlsc.Certificates = append(tlsc.Certificates, c)
		tlsc.InsecureSkipVerify = true //@todo switch to secure with docker ca.pem
	}

	//create docker client
	m.client, err = dockerclient.NewDockerClient(hurl.String(), &tlsc)
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

// convert to an url
func (m *Manager) toPortUrl(dhost string, port string) (*url.URL, error) {
	hurl, err := url.Parse(m.host)
	if err != nil {
		return nil, err
	}

	hurl.Scheme = "http"
	hurl.Host = strings.Replace(hurl.Host, ":2376", fmt.Sprintf(":%s", port), 1)

	return hurl, nil
}

// Instruct a mock server to expect a certain request
func (m *Manager) Expect(casename string, port string) error {

	//use docker host location to form url
	hurl, err := m.toPortUrl(m.host, port)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("case", casename)

	//send switch request
	resp, err := http.Get(fmt.Sprintf("%s/_expect?%s", hurl.String(), q.Encode()))
	if err != nil {
		return err
	}

	//should return 200
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to set mock's expectation for case '%s': %s", casename, resp.Status)
	}

	return nil
}

// start a mock container by using examples from the given directory
//  @todo this method makes an awfull lot assumptions about the mocked service
//  	- only one prot to expose
//  	- it being http&tcp
//  	- host is exposed on port 2376
func (m *Manager) Start(dir string, port string) (*MockContainer, error) {

	//create name for container
	cname, err := containerName(dir)
	if err != nil {
		return nil, err
	}

	//expose private port to given host port
	pb := map[string][]dockerclient.PortBinding{}
	pb[MockPrivatePort+"/tcp"] = []dockerclient.PortBinding{
		dockerclient.PortBinding{"0.0.0.0", port},
	}

	//create the container
	id, err := m.client.CreateContainer(&dockerclient.ContainerConfig{Image: ImageName}, cname)
	if err != nil {
		return nil, err
	}

	err = m.client.StartContainer(id, &dockerclient.HostConfig{PortBindings: pb})
	if err != nil {
		return nil, err
	}

	rc, err := m.client.ContainerLogs(id, &dockerclient.LogOptions{Follow: true, Stdout: true, Stderr: true})
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// scan for ready line
	err = iowait.WaitForRegexp(rc, ReadyExp, ReadyTimeout)
	if err != nil {
		return nil, err
	}

	//use docker host location to form url
	hurl, err := m.toPortUrl(m.host, port)
	if err != nil {
		return nil, err
	}

	//tar examples into memory
	tar := bytes.NewBuffer(nil)
	err = dirtar.Tar(dir, tar)
	if err != nil {
		return nil, err
	}

	//send the upload request
	resp, err := http.Post(fmt.Sprintf("%s/_examples", hurl.String()), "application/x-tar", tar)
	if err != nil {
		return nil, err
	}

	//check if we uploaded correctly
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("Failed to upload examples to container '%s' %s", id, cname)
	}

	// send HUP signal to 'root' process (dockpit/mock) to reload examples
	err = m.client.KillContainer(id, "SIGHUP")
	if err != nil {
		return nil, err
	}

	return &MockContainer{id, hurl.String(), dir}, nil
}

// stop a mock container that was started from the given directory
func (m *Manager) Stop(dir string) error {

	//create name for container
	cname, err := containerName(dir)
	if err != nil {
		return err
	}

	//get all containers
	cs, err := m.client.ListContainers(true, false, "")
	if err != nil {
		return err
	}

	//get container that matches the name
	// var container *docker.APIContainers
	var container dockerclient.Container
	for _, c := range cs {
		for _, n := range c.Names {
			if n[1:] == cname {
				container = c
			}
		}
	}

	return m.client.RemoveContainer(container.Id, true)
}
