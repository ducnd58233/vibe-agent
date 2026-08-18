package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestValidateListenHostRejectsWildcard(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "[::]"} {
		if err := ValidateListenHost(host); err == nil {
			t.Fatalf("expected error for %q", host)
		}
	}
}

func TestValidateListenHostAllowsLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost"} {
		if err := ValidateListenHost(host); err != nil {
			t.Fatalf("%q: %v", host, err)
		}
	}
}

func TestEmptyShellRendersRequiredTestIDs(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, id := range []string{"app-shell", "rail", "trajectory-empty", "workspace-path"} {
		if !strings.Contains(text, `data-testid="`+id+`"`) {
			t.Fatalf("missing test id %q in %s", id, text)
		}
	}
	if strings.Contains(text, root) {
		// workspace path is expected
	} else if !strings.Contains(text, root) {
		t.Fatalf("workspace path missing from shell")
	}
}

func TestWriteStateCreatesWebJSON(t *testing.T) {
	root := t.TempDir()
	if err := WriteState(root, State{URL: "http://127.0.0.1:3080/", PID: 42}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "127.0.0.1:3080") {
		t.Fatalf("web.json = %s", raw)
	}
}
