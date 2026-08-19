package redact

import (
	"strings"
	"testing"
)

func TestTextReplacesOpenAIKeyShape(t *testing.T) {
	const secret = "sk-0123456789abcdef0123456789ab"
	out := Text("token " + secret)
	if strings.Contains(out, secret) {
		t.Fatalf("secret still present: %q", out)
	}
	if !strings.Contains(out, Marker) || !strings.Contains(out, "credential") {
		t.Fatalf("expected markers in %q", out)
	}
}

func TestTextReplacesAnthropicKeyShape(t *testing.T) {
	secret := "sk-ant-api03-" + strings.Repeat("a", 24)
	out := Text("key=" + secret)
	if strings.Contains(out, secret) {
		t.Fatalf("secret still present: %q", out)
	}
}

func TestTextReplacesBearerToken(t *testing.T) {
	token := "Bearer " + strings.Repeat("x", 32)
	out := Text("Authorization: " + token)
	if strings.Contains(out, token) {
		t.Fatalf("bearer token still present: %q", out)
	}
}

func TestTextReplacesContextualAPIKey(t *testing.T) {
	out := Text(`config.api_key = "supersecretvalue123"`)
	if strings.Contains(out, "supersecretvalue123") {
		t.Fatalf("api key value still present: %q", out)
	}
}

func TestTextReplacesGoogleAPIKeyShape(t *testing.T) {
	secret := "AIza" + strings.Repeat("A", 35)
	out := Text(secret)
	if strings.Contains(out, secret) {
		t.Fatalf("google api key still present: %q", out)
	}
}

func TestTextLeavesShortSkWord(t *testing.T) {
	const body = `const risk = "sk-1";`
	if out := Text(body); out != body {
		t.Fatalf("short sk token should not redact, got %q", out)
	}
}

func TestLiteralPatternsMatchGateExpectations(t *testing.T) {
	if len(LiteralPatterns()) != len(literalSpecs) {
		t.Fatalf("literal pattern count = %d, want %d", len(LiteralPatterns()), len(literalSpecs))
	}
}
