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

	Args []string
}

type Node interface {
	Position() lexer.Location
}

type NodeArg struct {
	pos

	Arg string
}

type NodeMixinDef struct {
	pos

	Name  string
	Args  []MixinArg
	Nodes []Node
}

type MixinArg struct {
	Name string
	Type string
}

type NodeImport struct {
	pos

	Path string
}

type NodeMixinCall struct {
	pos

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
	pos

	Keyword StatementKeyword
	Nodes   []Node

	Argument string
	HasElse  bool
}

type NodeGoBlock struct {
	pos

	Contents string
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

	IsSelfClosing bool
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
	Contents   string
	EscapeHTML bool
}

func (ValueGoExpr) value() {}

type ValueConcat struct {
	pos
	A, B Value
}

func (ValueConcat) value() {}
