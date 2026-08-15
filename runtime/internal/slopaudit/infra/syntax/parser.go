package syntax

import (
	"fmt"
	"path/filepath"
	"unicode"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type Result struct {
	Parsed bool
	Line   int
	Error  string
}

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (Parser) Parse(path string, source []byte, language string) Result {
	slashPath := filepath.ToSlash(path)
	entry := grammars.DetectLanguage(slashPath)
	if entry == nil {
		return Result{}
	}
	displayName := grammars.DisplayName(entry)
	if !matchesLanguage(displayName, language) {
		return Result{}
	}
	tree, err := grammars.ParseFilePooled(slashPath, source)
	if err != nil {
		return Result{Parsed: true, Line: 1, Error: err.Error()}
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return Result{Parsed: true}
	}
	if !root.HasError() {
		return Result{Parsed: true}
	}
	line := firstErrorLine(root)
	return Result{Parsed: true, Line: line, Error: fmt.Sprintf("tree-sitter parse error in %s", displayName)}
}

func matchesLanguage(grammarName, detectedLanguage string) bool {
	if detectedLanguage == "" {
		return true
	}
	grammar := normalizeLanguageName(grammarName)
	detected := normalizeLanguageName(detectedLanguage)
	return grammar == detected || languageHasPrefixOrSuffix(grammar, detected) || languageHasPrefixOrSuffix(detected, grammar)
}

func normalizeLanguageName(value string) string {
	out := make([]rune, 0, len(value))
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			out = append(out, unicode.ToLower(char))
		}
	}
	return string(out)
}

func languageHasPrefixOrSuffix(value, token string) bool {
	return len(token) > 2 && len(value) > len(token) &&
		(value[:len(token)] == token || value[len(value)-len(token):] == token)
}

func firstErrorLine(node *gotreesitter.Node) int {
	if node == nil {
		return 1
	}
	if node.IsError() || node.IsMissing() {
		return int(node.StartPoint().Row) + 1
	}
	for _, child := range node.Children() {
		if child != nil && child.HasError() {
			return firstErrorLine(child)
		}
	}
	return int(node.StartPoint().Row) + 1
}
