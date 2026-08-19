package catalog

import "github.com/ducnd58233/vibe-agent/runtime/internal/shared/markdown"

func tableRows(text string) []markdown.TableRow {
	return markdown.ParseFirstTable(text)
}

func linkTarget(cell string) string {
	return markdown.LinkTarget(cell)
}

func assetSlug(target string) string {
	return markdown.AssetSlug(target)
}
