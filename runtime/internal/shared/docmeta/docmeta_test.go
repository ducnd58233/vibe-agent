package docmeta_test

import (
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/docmeta"
)

func TestParseFrontMatterAcceptsRequiredFields(t *testing.T) {
	raw := []byte("---\nslug: demo-slug\ndate: 2026-08-21\nversion: 1\n---\n\n# Title\n")
	meta, err := docmeta.ParseFrontMatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Slug != "demo-slug" || meta.Date != "2026-08-21" || meta.Version != 1 {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestParseFrontMatterRejectsMissingDate(t *testing.T) {
	raw := []byte("---\nslug: demo-slug\nversion: 1\n---\n\n# Title\n")
	_, err := docmeta.ParseFrontMatter(raw)
	if err == nil {
		t.Fatal("missing date parsed")
	}
	if !strings.Contains(err.Error(), "date") {
		t.Errorf("error = %q, want it to name date", err)
	}
}

func TestParseFrontMatterRejectsMissingVersion(t *testing.T) {
	raw := []byte("---\nslug: demo-slug\ndate: 2026-08-21\n---\n\n# Title\n")
	_, err := docmeta.ParseFrontMatter(raw)
	if err == nil {
		t.Fatal("missing version parsed")
	}
}

func TestParseFrontMatterRejectsMissingSlug(t *testing.T) {
	raw := []byte("---\ndate: 2026-08-21\nversion: 1\n---\n\n# Title\n")
	_, err := docmeta.ParseFrontMatter(raw)
	if err == nil {
		t.Fatal("missing slug parsed")
	}
}

func TestParseFrontMatterRejectsNoFence(t *testing.T) {
	_, err := docmeta.ParseFrontMatter([]byte("# Title\n"))
	if err == nil {
		t.Fatal("bare markdown parsed as front matter")
	}
}

func TestValidateRejectsBadDate(t *testing.T) {
	err := docmeta.Validate(docmeta.Meta{Slug: "demo", Date: "21-08-2026", Version: 1})
	if err == nil {
		t.Fatal("bad date validated")
	}
}
