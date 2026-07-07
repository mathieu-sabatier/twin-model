package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPreviewModelDesign(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/preview/modeldesign?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Errorf("content-type = %q, want xml", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ModelDesign") {
		t.Errorf("body is not ModelDesign XML: %.80s", body)
	}
}

func TestDiffAgainstBase(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	// Insert BrandNewType into the object_types block so it is parsed as a type,
	// not an instance. Appending to the end would land it under instances:.
	base := readExample(t)
	edited := strings.Replace(base, "object_types:\n", "object_types:\n\n  BrandNewType:\n    base: OpcUa:BaseObjectType\n", 1)
	putFiles(t, ts, id, map[string]string{"equipment.yaml": edited}).Body.Close()

	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/diff?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var dr struct {
		Changes []struct {
			Kind string `json:"kind"`
			Type string `json:"type"`
		} `json:"changes"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range dr.Changes {
		if c.Kind == "TypeAdded" && c.Type == "BrandNewType" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected TypeAdded BrandNewType, got %+v", dr.Changes)
	}
}

func TestResolvedMembers(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	// FurnaceType exists in the example and extends EquipmentType; resolved members
	// must include inherited ones. Assert the endpoint returns a non-empty set with
	// declaredIn tags.
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/types/FurnaceType/resolved?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var rr struct {
		Type    string `json:"type"`
		Members []struct {
			Name       string `json:"name"`
			DeclaredIn string `json:"declaredIn"`
		} `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		t.Fatal(err)
	}
	if rr.Type != "FurnaceType" || len(rr.Members) == 0 {
		t.Fatalf("resolved = %+v", rr)
	}
	for _, m := range rr.Members {
		if m.DeclaredIn == "" {
			t.Errorf("member %q missing declaredIn", m.Name)
		}
	}
}

func TestResolvedUnknownType(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/types/NopeType/resolved?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPreviewDiagramTypes(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/preview/diagram?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "classDiagram") {
		t.Errorf("default view should be a classDiagram, got:\n%.60s", body)
	}
}

func TestPreviewDiagramInstances(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/preview/diagram?file=equipment.yaml&view=instances")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "flowchart TD") {
		t.Errorf("view=instances should be a flowchart, got:\n%.60s", body)
	}
}

func readExample(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
