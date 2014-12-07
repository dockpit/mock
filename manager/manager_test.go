package manager_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmizerany/assert"
	"github.com/dockpit/go-dockerclient"

	"github.com/dockpit/mock/manager"
)

func gethostcert(t *testing.T) (string, string) {
	h := os.Getenv("DOCKER_HOST")
	if h == "" {
		t.Skip("No DOCKER_HOST env variable setup")
		return "", ""
	}

	cert := os.Getenv("DOCKER_CERT_PATH")
	if cert == "" {
		t.Skip("No DOCKER_CERT_PATH env variable setup")
		return "", ""
	}

	return h, cert
}

func TestStart(t *testing.T) {
	host, cert := gethostcert(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	m, err := manager.NewManager(host, cert)
	if err != nil {
		t.Fatal(err)
	}

	//creat protbinding
	portb := map[docker.Port][]docker.PortBinding{
		docker.Port("8000/tcp"): []docker.PortBinding{docker.PortBinding{HostPort: "11000"}},
	}

	path := filepath.Join(wd, "..", ".dockpit", "examples")

	m.Stop(path)

	mc, err := m.Start(path, portb)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, true, strings.HasSuffix(mc.Endpoint, ":11000"), "mocked service should have port configured endpoint")

	resp, err := http.Get(fmt.Sprintf("%s/users", mc.Endpoint))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, resp.StatusCode)

	err = m.Stop(path)
	if err != nil {
		t.Fatal(err)
	}

	//this should fail since the container is now down
	resp, err = http.Get(fmt.Sprintf("%s/users", mc.Endpoint))
	assert.NotEqual(t, nil, err)
}
