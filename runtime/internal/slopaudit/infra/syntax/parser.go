package syntax

import (
	"fmt"

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

func (Parser) Parse(path string, source []byte) Result {
	entry := grammars.DetectLanguage(path)
	if entry == nil {
		return Result{}
	}
	tree, err := grammars.ParseFilePooled(path, source)
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
	return Result{Parsed: true, Line: line, Error: fmt.Sprintf("tree-sitter parse error in %s", grammars.DisplayName(entry))}
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
