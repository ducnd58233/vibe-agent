package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteHTMXFragmentEscapesMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeHTMXFragment(rec, `<script>alert(1)</script>`)
	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatalf("fragment must escape HTML, got %q", body)
	}
	if !strings.Contains(body, htmxEmptyErrorClass) {
		t.Fatalf("fragment should use empty class, got %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestWriteHTMXOrErrorUsesFragmentForHTMX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	writeHTMXOrError(rec, req, http.StatusBadRequest, "bad path")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bad path") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWriteHTMXOrErrorUsesHTTPErrorWithoutHTMX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	writeHTMXOrError(rec, req, http.StatusBadRequest, "bad path")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "bad path\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
