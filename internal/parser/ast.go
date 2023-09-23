package parser

import (
	"github.com/pipe01/poodle/internal/lexer"
)

type pos lexer.Location

func (p pos) Position() lexer.Location {
	return lexer.Location(p)
}

type File struct {
	Nodes []Node
}

type Node interface {
	Position() lexer.Location
}

type NodeTag struct {
	pos

	Name       string
	Attributes []TagAttribute
	Body       []Value
}

type TagAttribute struct {
	pos

	Name  string
	Value Value
}

type Value struct {
	pos

	Contents       string
	IsGoExpression bool
}
