package parser

import (
	"github.com/pipe01/poodle/internal/lexer"
)

type pos lexer.Location

func (p pos) Position() lexer.Location {
	return lexer.Location(p)
}

type File struct {
	Name  string
	Nodes []Node
}

type Node interface {
	Position() lexer.Location
}

type NodeText struct {
	pos

	Text Value
}

type NodeTag struct {
	pos

	Name       string
	Attributes []TagAttribute
	Nodes      []Node
}

type TagAttribute struct {
	pos

	Name  string
	Value Value
}

type Value interface {
	Node
	value()
}

type ValueLiteral struct {
	pos
	Contents string
}

func (ValueLiteral) value() {}

type ValueGoExpr struct {
	pos
	Contents string
}

func (ValueGoExpr) value() {}

type ValueConcat struct {
	pos
	A, B Value
}

func (ValueConcat) value() {}
