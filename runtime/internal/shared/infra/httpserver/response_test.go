package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONSetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusOK, map[string]string{"ok": "yes"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != contentTypeJSON {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestRespondErrorJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json")
	RespondError(rec, req, http.StatusBadRequest, "bad input")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "bad input" {
		t.Fatalf("message = %q", body.Message)
	}
}

func TestRespondErrorPlainForHTMX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Accept", "application/json")
	RespondError(rec, req, http.StatusBadRequest, "bad input")
	if rec.Header().Get("Content-Type") == contentTypeJSON {
		t.Fatal("htmx request should not get JSON error body")
	}
}
