package redact

import (
	"strings"
	"testing"
)

func TestContainsCredentialRegexShape(t *testing.T) {
	secret := "api_key = sk-" + strings.Repeat("0", 32)
	if !ContainsCredential(secret) {
		t.Fatalf("expected credential in %q", secret)
	}
}

func TestContainsCredentialGitleaksGitHubPAT(t *testing.T) {
	secret := "gh" + "p_" + strings.Repeat("12", 20)
	if !ContainsCredential("token=" + secret) {
		t.Fatalf("expected gitleaks-detected pat in haystack")
	}
}

func TestContainsCredentialLeavesPlaceholder(t *testing.T) {
	if ContainsCredential("API_KEY=xxx") {
		t.Fatal("short placeholder value should not match contextual pattern")
	}
}
