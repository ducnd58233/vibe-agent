// Package citations checks that the URLs a research document cites resolve.
//
// A working link is necessary and not sufficient: a live page can still fail
// to support the claim beside it, and judging that needs a reader. What this
// catches is the cheaper, measured failure, a URL that never existed, which
// research agents produce at 3-13% (arXiv:2604.03173).
package citations

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/docmeta"
)

// DefaultTimeout bounds one URL check.
const DefaultTimeout = 15 * time.Second

var urlPattern = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `)\]]+`)

// Extract returns the distinct http(s) URLs in a markdown document, outside
// fenced code, in first-seen order. A fence is an example, not a citation.
func Extract(markdown []byte) []string {
	seen := map[string]bool{}
	var urls []string
	for _, match := range urlPattern.FindAllString(string(docmeta.StripFencedCode(markdown)), -1) {
		url := strings.TrimRight(match, ".,;:!?")
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}
	return urls
}

// Failure is a URL that did not resolve, and why.
type Failure struct {
	URL    string
	Reason string
}

func (f Failure) String() string { return f.URL + ": " + f.Reason }

// Checker requests each URL. The client decides which addresses may be dialled.
type Checker struct {
	Client *http.Client
}

// Check returns the URLs that do not resolve.
//
// A 401, 403, or 429 counts as resolving: the host answered for that path, and
// a bot check or a login wall is not a fabricated citation. Every other
// non-2xx/3xx status and every transport error is a failure, including a
// timeout, because this gate fails closed.
func (c Checker) Check(ctx context.Context, urls []string) []Failure {
	var failures []Failure
	for _, url := range urls {
		if reason := c.probe(ctx, url); reason != "" {
			failures = append(failures, Failure{URL: url, Reason: reason})
		}
	}
	return failures
}

func (c Checker) probe(ctx context.Context, url string) string {
	status, err := c.status(ctx, http.MethodHead, url)
	if err == nil && (status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented) {
		status, err = c.status(ctx, http.MethodGet, url)
	}
	switch {
	case err != nil:
		return err.Error()
	case status < 400, status == http.StatusUnauthorized, status == http.StatusForbidden,
		status == http.StatusTooManyRequests:
		return ""
	default:
		return fmt.Sprintf("%d %s", status, http.StatusText(status))
	}
}

func (c Checker) status(ctx context.Context, method, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, fmt.Errorf("bad URL: %w", err)
	}
	req.Header.Set("User-Agent", "vibe-agent-citation-check")
	resp, err := c.Client.Do(req)
	if err != nil {
		if errors.Is(err, errNotPublic) {
			return 0, errNotPublic
		}
		return 0, fmt.Errorf("unreachable: %w", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

var errNotPublic = errors.New("not a public address; a citation must be reachable on the public internet")

// PublicClient refuses to dial loopback, private, link-local, and unspecified
// addresses. The check runs on the resolved address at connect time, so a
// hostname that resolves inward is refused as well as a literal IP.
func PublicClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
				return errNotPublic
			}
			return nil
		},
	}
	// No Proxy: a proxy is dialled instead of the target, so the address check
	// above would inspect the proxy and let any target through.
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// Default checks against the public internet with the default timeout.
func Default() Checker { return Checker{Client: PublicClient(DefaultTimeout)} }
