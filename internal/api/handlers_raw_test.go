package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleDraftFileRaw(t *testing.T) {
	store := NewStore(time.Minute)
	d := store.Create("main", map[string][]byte{"Equipment.yaml": []byte("model: Demo\nnamespace: urn:x\nversion: 1.0.0\npublicationDate: 2026-01-01\n")})
	srv := NewServer(nil, store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/drafts/"+d.ID+"/file?file=Equipment.yaml", nil)
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "model: Demo") {
		t.Fatalf("body missing yaml: %q", rec.Body.String())
	}
}
