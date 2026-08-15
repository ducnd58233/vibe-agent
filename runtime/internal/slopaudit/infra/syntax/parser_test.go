package syntax

import "testing"

func TestParserParsesCodebaseLanguages(t *testing.T) {
	parser := NewParser()
	cases := []struct {
		name   string
		path   string
		source string
	}{
		{name: "go", path: "main.go", source: "package main\nfunc main() {}\n"},
		{name: "tsx", path: "Button.tsx", source: "export function Button() { return <button>Save</button> }\n"},
		{name: "yaml", path: "deploy.yaml", source: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.Parse(tc.path, []byte(tc.source))
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
	result := NewParser().Parse("bad.go", []byte("package main\nfunc main( {\n"))
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
	result := NewParser().Parse("component.unknownext", []byte("debug temporary\n"))
	if result.Parsed {
		t.Fatalf("Parsed = true for unknown file: %+v", result)
	}
}
