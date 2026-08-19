package redact

import (
	"strings"
	"testing"
)

func BenchmarkText(b *testing.B) {
	// Warm gitleaks detector init outside timed section.
	_ = Text("warm")

	const clean = "GET /api/health status=200 latency=12ms"
	secret := "sk-" + strings.Repeat("a", 32)
	withSecret := "Authorization: Bearer " + secret
	toolCmd := "curl -H 'Authorization: Bearer " + strings.Repeat("x", 48) + "' https://api.example.com/v1/items"

	cases := map[string]string{
		"clean":      clean,
		"secret":     withSecret,
		"tool_cmd":   toolCmd,
		"contextual": `config.api_key = "supersecretvalue123"`,
	}

	for name, input := range cases {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = Text(input)
			}
		})
	}
}

func BenchmarkTruncateCommand(b *testing.B) {
	short := "git status"
	long := strings.Repeat("echo step ", 80)

	b.Run("short", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = TruncateCommand(short)
		}
	})
	b.Run("long", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = TruncateCommand(long)
		}
	})
}
