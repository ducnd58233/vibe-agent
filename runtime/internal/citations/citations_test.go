package citations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractSkipsFencedCodeAndTrimsPunctuation(t *testing.T) {
	doc := "See [paper](https://arxiv.org/abs/2604.03173), and https://example.org/a.\n" +
		"```text\nhttps://inside.example/fenced\n```\n" +
		"Again https://arxiv.org/abs/2604.03173 is cited twice.\n"
	got := Extract([]byte(doc))
	want := []string{"https://arxiv.org/abs/2604.03173", "https://example.org/a"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Extract = %v, want %v", got, want)
	}
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/no-head":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/bot-check":
			w.WriteHeader(http.StatusForbidden)
		case "/broken":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCheckReportsOnlyTheURLsThatDoNotResolve(t *testing.T) {
	server := testServer(t)
	checker := Checker{Client: server.Client()}
	failures := checker.Check(t.Context(), []string{
		server.URL + "/ok",
		server.URL + "/no-head",
		server.URL + "/bot-check",
		server.URL + "/gone",
		server.URL + "/broken",
	})
	var failed []string
	for _, failure := range failures {
		failed = append(failed, failure.URL[len(server.URL):])
	}
	if strings.Join(failed, " ") != "/gone /broken" {
		t.Errorf("failed = %v, want /gone and /broken only", failed)
	}
}

// A research file is written by an agent, and this check requests every URL in
// it on its own. An address inside the machine or its network is not a public
// citation, and requesting one is how a document becomes a probe.
func TestThePublicClientRefusesAPrivateAddress(t *testing.T) {
	server := testServer(t)
	checker := Checker{Client: PublicClient(2 * time.Second)}
	failures := checker.Check(t.Context(), []string{server.URL + "/ok"})
	if len(failures) != 1 || !strings.Contains(failures[0].Reason, "not a public address") {
		t.Errorf("failures = %v, want the loopback test server refused", failures)
	}
}
