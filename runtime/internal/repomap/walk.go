package repomap

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/sourcefiles"
)

func listSourceFiles(ctx context.Context, root string) ([]string, error) {
	files, err := sourcefiles.List(ctx, root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Rel)
	}
	return out, nil
}
