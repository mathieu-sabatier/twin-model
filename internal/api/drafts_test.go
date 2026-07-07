package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// createDraft seeds the host at "main" and POSTs a draft, returning its id.
func createDraft(t *testing.T, ts *httptest.Server, host *fakeHost) string {
	t.Helper()
	seedEquipment(t, host, "main")
	body, _ := json.Marshal(map[string]string{"baseRef": "main"})
	resp, err := http.Post(ts.URL+"/api/drafts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create draft status = %d, want 201", resp.StatusCode)
	}
	var out struct {
		ID    string   `json:"id"`
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID == "" || len(out.Files) != 1 || out.Files[0] != "equipment.yaml" {
		t.Fatalf("unexpected create response: %+v", out)
	}
	return out.ID
}

func TestCreateAndReadDraftModel(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)

	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/model")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var mr struct {
		Model *struct {
			Name string `json:"name"`
		} `json:"model"`
		ParseError string `json:"parseError"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		t.Fatal(err)
	}
	if mr.ParseError != "" || mr.Model == nil || mr.Model.Name != "AcmeEquipment" {
		t.Fatalf("unexpected draft model: %+v err=%q", mr.Model, mr.ParseError)
	}
}

func TestDraftModelUnknownID(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/drafts/deadbeef/model")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCreateDraftMissingBaseRef(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/drafts", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func putFiles(t *testing.T, ts *httptest.Server, id string, files map[string]string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"files": files})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/drafts/"+id+"/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPutFilesCanonicalizes(t *testing.T) {
	ts, host, store := newTestServer(t)
	id := createDraft(t, ts, host)

	// A parseable file with a default kind/rule the formatter must drop.
	src := "model:\n  name: M\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-07-02\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n      X: { kind: variable, type: Double, rule: mandatory }\n"
	resp := putFiles(t, ts, id, map[string]string{"equipment.yaml": src})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	d, _ := store.Get(id)
	got := string(d.Files["equipment.yaml"])
	if bytes.Contains([]byte(got), []byte("kind: variable")) {
		t.Errorf("stored file was not canonicalized (kept default kind):\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte("type: Double")) {
		t.Errorf("canonical output lost the member:\n%s", got)
	}
}

func TestPutFilesStoresUnparseableRaw(t *testing.T) {
	ts, host, store := newTestServer(t)
	id := createDraft(t, ts, host)
	garbage := "model: [this is : not valid yaml"
	resp := putFiles(t, ts, id, map[string]string{"equipment.yaml": garbage})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (parse errors are surfaced on read, not on write)", resp.StatusCode)
	}
	d, _ := store.Get(id)
	if string(d.Files["equipment.yaml"]) != garbage {
		t.Errorf("unparseable content should be stored raw, got:\n%s", d.Files["equipment.yaml"])
	}
}

func TestGetDraftMetadata(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)

	resp, err := http.Get(ts.URL + "/api/drafts/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var dr struct {
		ID        string   `json:"id"`
		BaseRef   string   `json:"baseRef"`
		Files     []string `json:"files"`
		UpdatedAt string   `json:"updatedAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		t.Fatal(err)
	}
	if dr.ID != id || dr.BaseRef != "main" || len(dr.Files) != 1 || dr.Files[0] != "equipment.yaml" {
		t.Fatalf("draft metadata = %+v", dr)
	}
	if dr.UpdatedAt == "" {
		t.Error("updatedAt should be set")
	}
}

func TestGetDraftUnknownID(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/drafts/deadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestValidateReturnsDiagnostics(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	// Introduce an error: unknown member type.
	bad := "model:\n  name: M\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-07-02\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n      X: { type: Nope }\n"
	putFiles(t, ts, id, map[string]string{"equipment.yaml": bad}).Body.Close()

	resp, err := http.Post(ts.URL+"/api/drafts/"+id+"/validate?file=equipment.yaml", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var vr struct {
		Diagnostics []struct {
			Code string `json:"code"`
			Path string `json:"path"`
		} `json:"diagnostics"`
		ParseError string `json:"parseError"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range vr.Diagnostics {
		if d.Code == "unknown-type" && d.Path == "object_types/T/members/X/type" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown-type diagnostic with a Path, got %+v", vr.Diagnostics)
	}
}
