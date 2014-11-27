package manager

type MockContainer struct {
	ID       string `json:"id"`
	Endpoint string `json:"endpoint"`
	Dir      string `json:"dir"`
}
