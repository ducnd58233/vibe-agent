package observability

import (
	"errors"
	"strings"
	"testing"
)

func TestLogErrorRedactsCredentialInMessage(t *testing.T) {
	dir := t.TempDir()
	var console strings.Builder
	log, closer, err := NewLogger(Options{
		Service: "test",
		Level:   "error",
		Stdout:  &console,
		Dir:     dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	secret := "ghp_" + "abcdefghijklmnopqrst"
	LogError(log, "tool failed", errors.New("token="+secret))
	if strings.Contains(console.String(), secret) {
		t.Fatalf("secret leaked to console: %s", console.String())
	}
}
