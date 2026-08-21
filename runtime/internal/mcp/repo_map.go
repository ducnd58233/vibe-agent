package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/repomap"
)

func repoMap(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Budget int    `json:"budget"`
		Focus  string `json:"focus"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("vibe_repo_map: %w", err)
		}
	}
	budget := args.Budget
	if budget <= 0 {
		budget = DefaultFetchBudget
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := repomap.Build(ctx, deps.WorkspaceRoot, repomap.Options{
		Budget: budget,
		Focus:  args.Focus,
	})
	if err != nil {
		return nil, fmt.Errorf("vibe_repo_map: %w", err)
	}
	return map[string]any{
		"text":          result.Text,
		"tokens":        result.Tokens,
		"filesIncluded": result.FilesIncluded,
		"filesOmitted":  result.FilesOmitted,
	}, nil
}
