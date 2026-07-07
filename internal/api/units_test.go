package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleUnits(t *testing.T) {
	srv := NewServer(nil, NewStore(time.Minute))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/api/units", nil))
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Units []struct {
			Symbol string `json:"symbol"`
		} `json:"units"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Units) == 0 {
		t.Fatal("no units returned")
	}
}
