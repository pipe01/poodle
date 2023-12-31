package ast

import (
	"github.com/pipe01/poodle/internal/lexer"
)

type Pos lexer.Location

func (p Pos) Position() lexer.Location {
	return lexer.Location(p)
}

type File struct {
	Name  string
	Nodes []Node

	Args    []string
	Imports []string
}

type Node interface {
	Position() lexer.Location
}

type NodeComment struct {
	Pos

	Text string
}

type NodeDoctype struct {
	Pos

	Value string
}

type NodeMixinDef struct {
	Pos

	Name  string
	Args  []MixinArg
	Nodes []Node
}

type MixinArg struct {
	Name string
	Type string
}

type NodeInclude struct {
	Pos

	File *File
	Path string
}

type NodeMixinCall struct {
	Pos

	Name string
	Args []string
}

type StatementKeyword string

const (
	KeywordIf   StatementKeyword = "if"
	KeywordElse StatementKeyword = "else"
	KeywordFor  StatementKeyword = "for"
)

type NodeGoStatement struct {
	Pos

	Keyword StatementKeyword
	Nodes   []Node

	Argument string
	HasElse  bool
}

type NodeGoBlock struct {
	Pos

	Contents string
}

type NodeText struct {
	Pos

	Text Value
}

type NodeTag struct {
	Pos

	Name       string
	Attributes []TagAttribute
	Nodes      []Node

	IsSelfClosing bool
}

type TagAttribute struct {
	Pos

	Name  string
	Value Value

	// Only add this attribute if this Go expression evaluates to true
	Condition string
}

type Value interface {
	Node
	value()
}

type ValueLiteral struct {
	Pos
	Contents string
}

func (ValueLiteral) value() {}

type ValueGoExpr struct {
	Pos
	Contents   string
	EscapeHTML bool
}

func (ValueGoExpr) value() {}

type ValueConcat struct {
	Pos
	A, B Value
}

func (ValueConcat) value() {}
