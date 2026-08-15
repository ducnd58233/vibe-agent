package syntax

import (
	"path/filepath"
	"testing"
)

func TestParserParsesCodebaseLanguages(t *testing.T) {
	parser := NewParser()
	cases := []struct {
		name   string
		path   string
		source string
		lang   string
	}{
		{name: "go", path: "main.go", source: "package main\nfunc main() {}\n", lang: "Go"},
		{name: "tsx", path: "Button.tsx", source: "export function Button() { return <button>Save</button> }\n", lang: "TSX"},
		{name: "yaml", path: "deploy.yaml", source: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n", lang: "YAML"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.path, []byte(tc.source), tc.lang)
			if !result.Parsed {
				t.Fatalf("Parsed = false for %s", tc.path)
			}
			if result.Error != "" {
				t.Fatalf("Error = %q", result.Error)
			}
		})
	}
}

func TestParserReportsSyntaxErrorLine(t *testing.T) {
	result := NewParser().Parse("bad.go", []byte("package main\nfunc main( {\n"), "Go")
	if !result.Parsed {
		t.Fatal("Parsed = false")
	}
	if result.Error == "" {
		t.Fatal("Error is empty")
	}
	if result.Line < 1 {
		t.Fatalf("Line = %d, want positive", result.Line)
	}
}

func TestParserSkipsUnknownFiles(t *testing.T) {
	result := NewParser().Parse("component.unknownext", []byte("debug temporary\n"), "Text")
	if result.Parsed {
		t.Fatalf("Parsed = true for unknown file: %+v", result)
	}
}

func TestParserNormalizesWindowsPathsBeforeGrammarDetection(t *testing.T) {
	result := NewParser().Parse(filepath.Join("runtime", "go.mod"), []byte("module example.com/app\n\ngo 1.26.5\n"), "Go Module")
	if !result.Parsed {
		t.Fatalf("Parsed = false for normalized path: %+v", result)
	}
	if result.Error != "" {
		t.Fatalf("Error = %q", result.Error)
	}
}
