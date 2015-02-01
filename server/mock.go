package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zenazn/goji/web"

	"github.com/dockpit/dirtar"
	"github.com/dockpit/lang/manifest"
)

type Recording struct {
	Count int `json:"count"`
}

type Expectation struct {
	Pair *manifest.Pair
}

//
//
// Represents a mocked manifest
type Mock struct {
	manifest manifest.M
	dir      string

	Expectations map[string]map[string][]*Expectation
	Recordings   map[string]*Recording
}

func NewMock(m manifest.M, dir string) *Mock {
	return &Mock{manifest: m, dir: dir,
		Expectations: make(map[string]map[string][]*Expectation),
		Recordings:   make(map[string]*Recording),
	}
}

//allow external programs to upload a new set of examples as a tar archive
func (m *Mock) UploadExamples(c web.C, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/x-tar" {
		http.Error(w, fmt.Sprintf("Expected Content-Type 'application/x-tar', received: '%s'", r.Header.Get("Content-Type")), http.StatusBadRequest)
		return
	}

	//empty old directory
	files, err := ioutil.ReadDir(m.dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		err := os.RemoveAll(filepath.Join(m.dir, f.Name()))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	//untar into empty dir
	err = dirtar.Untar(m.dir, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//all went well
	w.WriteHeader(201)
}

//instruct the mock to expect the given case to be requested
func (m *Mock) Expect(c web.C, w http.ResponseWriter, r *http.Request) {
	var resource manifest.R
	var action manifest.A
	var pair *manifest.Pair

	casename := r.URL.Query().Get("case")
	if casename == "" {
		http.Error(w, fmt.Sprintf("casename required as query parameter 'case', got %s", r.URL.Query()), http.StatusBadRequest)
		return
	}

	//attempt to find the pair that blongs to the casename
	res, err := m.manifest.Resources()
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not get resources: %s", err), http.StatusInternalServerError)
		return
	}

	for _, r := range res {
		acs, err := r.Actions()
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not get actions: %s", err), http.StatusInternalServerError)
			return
		}

		for _, a := range acs {
			for _, p := range a.Pairs() {
				if p.Name == casename {
					resource = r
					pair = p
					action = a
				}
			}
		}
	}

	if pair == nil || action == nil {
		http.Error(w, fmt.Sprintf("Could not find case with name '%s'", casename), http.StatusNotFound)
	}

	//lazily create expectation maps
	var ok bool
	var acts map[string][]*Expectation
	if acts, ok = m.Expectations[resource.Pattern()]; !ok {
		acts = make(map[string][]*Expectation)
		m.Expectations[resource.Pattern()] = acts
	}

	//set current pair for an action @note, can only be one
	var expects []*Expectation
	if expects, ok = acts[action.Method()]; !ok {
		expects = []*Expectation{}
		acts[action.Method()] = expects
	}

	//add to end of expected questions
	acts[action.Method()] = append(acts[action.Method()], &Expectation{pair})
}

//allows the mock to return current recording through a http response
func (m *Mock) ListRecordings(c web.C, w http.ResponseWriter, r *http.Request) {
	casename := r.URL.Query().Get("case")
	if casename == "" {
		http.Error(w, fmt.Sprintf("casename required as query parameter 'case', got %s", r.URL.Query()), http.StatusBadRequest)
		return
	}

	if rec, ok := m.Recordings[casename]; ok {

		//empty recording when it is retrieved
		defer func() {
			m.Recordings[casename] = &Recording{}
		}()

		encoder := json.NewEncoder(w)
		err := encoder.Encode(rec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else {
		http.Error(w, fmt.Sprintf("No recordings for case '%s'", casename), http.StatusNotFound)
		return
	}

}

// walk resources, actions and pairs to map all necessary states to
// create a router that mocks the manifest
func (m *Mock) Mux() (*web.Mux, error) {
	mux := web.New()

	res, err := m.manifest.Resources()
	if err != nil {
		return mux, err
	}

	//look at each resource
	for _, r := range res {

		//scope resource to each handler closure
		func(r manifest.R) {
			mux.Handle(r.Pattern(), func(ctx web.C, w http.ResponseWriter, req *http.Request) {

				if expectedActs, ok := m.Expectations[r.Pattern()]; ok {
					if expects, ok := expectedActs[req.Method]; ok && len(expects) > 0 {

						//unshift the first expectation
						expect := expects[0]
						m.Expectations[r.Pattern()][req.Method] = expects[1:]

						//let the pair handle it
						expect.Pair.GenerateHandler().ServeHTTPC(ctx, w, req)

						//Recordings met expecation
						var rec *Recording
						if rec, ok = m.Recordings[expect.Pair.Name]; !ok {
							rec = &Recording{}
							m.Recordings[expect.Pair.Name] = rec
						}

						rec.Count++
						return
					} else {
						http.Error(w, fmt.Sprintf("Didn't expect (any more) '%s' actions on resource '%s'", req.Method, r.Pattern()), http.StatusBadRequest)
						return
					}
				} else {
					http.Error(w, fmt.Sprintf("Didn't expect resource '%s' to be requested", r.Pattern()), http.StatusBadRequest)
					return
				}

			})

		}(r)

	}

	//allow the mock to report resource recordings
	mux.Get("/_expect", m.Expect)
	mux.Get("/_recordings", m.ListRecordings)
	mux.Post("/_examples", m.UploadExamples)

	return mux, nil
}
