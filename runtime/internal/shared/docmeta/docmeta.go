// Package docmeta validates slug/date/version metadata on goal deliverables.
//
// SPEC, PLAN, and TASKS prose carry YAML front matter. tasks.json carries the
// same three fields as JSON. Doctor and the layout checker (T6) call into this
// package instead of re-parsing the rules in two places.
package docmeta

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
)

// Meta is the required identity of one revision's deliverables.
type Meta struct {
	Slug    string `yaml:"slug" json:"slug"`
	Date    string `yaml:"date" json:"date"`
	Version int    `yaml:"version" json:"version"`
}

// Validate reports whether meta is internally consistent.
func Validate(meta Meta) error {
	if !validate.Slug(meta.Slug) {
		return fmt.Errorf("slug %q must be kebab-case", meta.Slug)
	}
	if !validate.Date(meta.Date) {
		return fmt.Errorf("date %q is not YYYY-MM-DD", meta.Date)
	}
	if meta.Version < 1 {
		return fmt.Errorf("version must be >= 1, got %d", meta.Version)
	}
	return nil
}

// ParseFrontMatter reads the leading YAML block from a markdown document.
//
// The block must start at the first byte, use --- fences, and contain slug,
// date, and version. Extra keys are ignored so a document can carry status or
// other fields without this package needing to know them.
func ParseFrontMatter(raw []byte) (Meta, error) {
	const fence = "---"
	text := string(raw)
	if !strings.HasPrefix(text, fence+"\n") && !strings.HasPrefix(text, fence+"\r\n") {
		return Meta{}, fmt.Errorf("document has no YAML front matter")
	}
	rest := text[len(fence):]
	rest = strings.TrimPrefix(rest, "\r")
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		return Meta{}, fmt.Errorf("front matter is not closed")
	}
	body := rest[:end]
	var meta Meta
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.KnownFields(false)
	if err := decoder.Decode(&meta); err != nil {
		return Meta{}, fmt.Errorf("parse front matter: %w", err)
	}
	if err := Validate(meta); err != nil {
		return Meta{}, err
	}
	return meta, nil
}
