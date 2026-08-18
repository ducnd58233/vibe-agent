package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogCommandsBuild(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/commands?q=build", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="composer-catalog"`) {
		t.Fatalf("missing catalog: %s", body)
	}
	if !strings.Contains(body, `data-testid="catalog-item"`) {
		t.Fatalf("missing item: %s", body)
	}
	if !strings.Contains(body, "/build") {
		t.Fatalf("missing /build: %s", body)
	}
}

func TestCatalogUnknownQueryEmpty(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/commands?q=zzzz-not-a-command-xyzzy", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `data-testid="catalog-item"`) {
		t.Fatalf("expected no items: %s", rec.Body.String())
	}
}
