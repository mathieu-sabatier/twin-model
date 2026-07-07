package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func seedEquipment(t *testing.T, host *fakeHost, ref string) {
	t.Helper()
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	host.trees[ref] = map[string][]byte{"equipment.yaml": data}
}

// dtoModel is a minimal decode target for assertions.
type dtoModel struct {
	Name string `json:"name"`
}

func TestGetModelFromRef(t *testing.T) {
	ts, host, _ := newTestServer(t)
	seedEquipment(t, host, "main")

	resp, err := http.Get(ts.URL + "/api/model?ref=main&file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var mr struct {
		File        string    `json:"file"`
		Model       *dtoModel `json:"model"`
		Diagnostics []any     `json:"diagnostics"`
		ParseError  string    `json:"parseError"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		t.Fatal(err)
	}
	if mr.ParseError != "" || mr.Model == nil {
		t.Fatalf("unexpected parseError=%q model=%v", mr.ParseError, mr.Model)
	}
	if mr.Model.Name != "AcmeEquipment" {
		t.Errorf("model.name = %q", mr.Model.Name)
	}
}

func TestGetModelMissingRef(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/model?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGetModelUnknownFile(t *testing.T) {
	ts, host, _ := newTestServer(t)
	seedEquipment(t, host, "main")
	resp, err := http.Get(ts.URL + "/api/model?ref=main&file=nope.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSchemaEndpoint(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/schema")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "$schema") && !strings.Contains(string(body), "object_types") {
		t.Errorf("schema body looks wrong: %.80s", body)
	}
}
