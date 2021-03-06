package manager_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestStartSwitch(t *testing.T) {
	host, cert := gethostcert(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	m, err := manager.NewManager(host, cert)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(wd, "..", ".example", "examples")

	//stop if we still got some mock servers running
	m.Stop(path)

	mc, err := m.Start(path, "11000")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, true, strings.HasSuffix(mc.Endpoint, ":11000"), "mocked service should have port configured endpoint")

	//unexpected case should return 400
	resp, err := http.Get(fmt.Sprintf("%s/users", mc.Endpoint))
	if err != nil {
		t.Fatal(err)
	}

	//mocks working?
	assert.Equal(t, 400, resp.StatusCode)

	//tell mock to expect the request
	err = m.Expect("list all users", "11000")
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.Get(fmt.Sprintf("%s/users", mc.Endpoint))
	if err != nil {
		t.Fatal(err)
	}

	//mocks working?
	assert.Equal(t, 200, resp.StatusCode)

	err = m.Stop(path)
	if err != nil {
		t.Fatal(err)
	}

	//this should fail since the container is now down
	resp, err = http.Get(fmt.Sprintf("%s/users", mc.Endpoint))
	assert.NotEqual(t, nil, err)
}
