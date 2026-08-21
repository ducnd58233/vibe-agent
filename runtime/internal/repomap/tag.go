package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func tagFiles(root string, relPaths []string) ([]fileTags, error) {
	taggers := map[string]*gotreesitter.Tagger{}
	out := make([]fileTags, 0, len(relPaths))
	for _, rel := range relPaths {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		entry := grammars.DetectLanguage(filepath.ToSlash(rel))
		if entry == nil {
			continue
		}
		query := strings.TrimSpace(grammars.ResolveTagsQuery(*entry))
		if query == "" {
			continue
		}
		langName := grammars.DisplayName(entry)
		tagger, ok := taggers[langName]
		if !ok {
			built, err := newTagger(entry, query)
			if err != nil {
				continue
			}
			taggers[langName] = built
			tagger = built
		}
		src, err := os.ReadFile(filepath.Clean(abs))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		var defs, refs []symbol
		for _, t := range tagger.Tag(src) {
			name := strings.TrimSpace(t.Name)
			if name == "" {
				continue
			}
			switch {
			case strings.HasPrefix(t.Kind, "definition."):
				defs = append(defs, symbol{Kind: t.Kind, Name: name})
			case strings.HasPrefix(t.Kind, "reference."):
				refs = append(refs, symbol{Kind: t.Kind, Name: name})
			}
		}
		if len(defs) == 0 && len(refs) == 0 {
			continue
		}
		out = append(out, fileTags{Path: rel, Defs: defs, Refs: refs})
	}
	return out, nil
}

func newTagger(entry *grammars.LangEntry, query string) (*gotreesitter.Tagger, error) {
	lang := entry.Language()
	var opts []gotreesitter.TaggerOption
	if entry.TokenSourceFactory != nil {
		opts = append(opts, gotreesitter.WithTaggerTokenSourceFactory(func(src []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(src, lang)
		}))
	}
	return gotreesitter.NewTagger(lang, query, opts...)
}
