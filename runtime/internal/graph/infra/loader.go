package infra

import (
	"bytes"
	"fmt"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph/domain"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and validates a graph file. The filename stem must equal
// metadata.id, so a graph cannot be referenced under two names.
func Load(path string) (*domain.Graph, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read graph: %w", err)
	}

	graph, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("graph %s: %w", path, err)
	}

	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if graph.Metadata.ID != stem {
		return nil, fmt.Errorf("graph %s: metadata.id %q does not match the filename", path, graph.Metadata.ID)
	}
	return graph, nil
}

// Parse decodes and validates graph bytes.
//
// Unknown fields are an error rather than ignored: a typo in a node key would
// otherwise silently drop the setting it was meant to configure.
func Parse(raw []byte) (*domain.Graph, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)

	var graph domain.Graph
	if err := decoder.Decode(&graph); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	graph.Index()

	if err := graph.Validate(); err != nil {
		return nil, err
	}
	return &graph, nil
}

// DefaultDir is where graphs live relative to a toolkit root.
func DefaultDir(toolkitRoot string) string {
	return filepath.Join(toolkitRoot, ".ai-agents", "graphs")
}

// LoadByID finds a graph by id in a directory.
func LoadByID(dir, id string) (*domain.Graph, error) {
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, id+ext)
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil, fmt.Errorf("no graph %q in %s", id, dir)
}

// LoadDir loads every graph in a directory, reporting all failures together.
func LoadDir(dir string) (map[string]*domain.Graph, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read graphs directory: %w", err)
	}

	graphs := map[string]*domain.Graph{}
	var problems domain.ProblemsError
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		loaded, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		graphs[loaded.Metadata.ID] = loaded
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return graphs, nil
}
