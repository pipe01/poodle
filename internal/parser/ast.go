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
	TextLines  []Value
	Nodes      []Node
}

type TagAttribute struct {
	pos

	Name  string
	Value Value
}

type Value interface {
	Node
}

type ValueLiteral struct {
	pos
	Contents string
}

type ValueGoExpr struct {
	pos
	Contents string
}

type ValueConcat struct {
	pos
	Values []Value
}
