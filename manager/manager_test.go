package manager_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmizerany/assert"

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

	path := filepath.Join(wd, "..", ".dockpit", "examples")
	mc, err := m.Start(path)
	if err != nil {
		t.Fatal(err)
	}

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
