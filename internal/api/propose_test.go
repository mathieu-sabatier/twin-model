package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func propose(t *testing.T, ts *httptest.Server, id string, branch, title, msg string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"branch": branch, "title": title, "message": msg})
	resp, err := http.Post(ts.URL+"/api/drafts/"+id+"/propose", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestProposeOpensPR(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp := propose(t, ts, id, "model/change", "Update model", "body text")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.URL != host.prURL {
		t.Errorf("url = %q, want %q", out.URL, host.prURL)
	}
	if !host.openedPR {
		t.Fatal("OpenPR was not called")
	}
	if host.lastPR.Branch != "model/change" || host.lastPR.BaseRef != "main" {
		t.Errorf("ProposeParams = %+v", host.lastPR)
	}
	if _, ok := host.lastPR.Files["equipment.yaml"]; !ok {
		t.Errorf("proposed fileset missing equipment.yaml: %v", sortedKeys(host.lastPR.Files))
	}
}

func TestProposeBlockedOnLintError(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	// Introduce an error-severity diagnostic (unknown type).
	bad := "model:\n  name: M\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-07-02\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n      X: { type: Nope }\n"
	putFiles(t, ts, id, map[string]string{"equipment.yaml": bad}).Body.Close()

	resp := propose(t, ts, id, "model/change", "t", "m")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if host.openedPR {
		t.Error("OpenPR must not be called for a lint-red draft")
	}
	var out struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	var found bool
	for _, d := range out.Diagnostics {
		if d.Code == "unknown-type" {
			found = true
		}
	}
	if !found {
		t.Errorf("409 body should carry the blocking diagnostics, got %+v", out.Diagnostics)
	}
}

func TestProposeBlockedOnParseError(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	putFiles(t, ts, id, map[string]string{"equipment.yaml": "model: [not : valid"}).Body.Close()
	resp := propose(t, ts, id, "b", "t", "m")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if host.openedPR {
		t.Error("OpenPR must not be called when a file fails to parse")
	}
}

func TestProposeMissingBranch(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp := propose(t, ts, id, "", "Update", "body") // no branch
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if host.openedPR {
		t.Error("OpenPR must not be called when branch is missing")
	}
}

func TestProposeUnknownDraft(t *testing.T) {
	ts, host, _ := newTestServer(t)
	resp := propose(t, ts, "deadbeefdeadbeefdeadbeefdeadbeef", "model/x", "Update", "body")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if host.openedPR {
		t.Error("OpenPR must not be called for an unknown draft")
	}
}

func TestProposeMapsPRError(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	host.prErr = &PRError{Status: http.StatusNotFound, Body: `{"message":"Not Found"}`}
	resp := propose(t, ts, id, "model/x", "Update", "body")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	var out struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Error == "" || strings.Contains(out.Error, "Not Found") {
		t.Errorf("friendly error should not leak the raw payload: %q", out.Error)
	}
	if !strings.Contains(out.Detail, "Not Found") {
		t.Errorf("detail should carry the raw payload: %q", out.Detail)
	}
}
