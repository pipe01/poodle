package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pipe01/poodle/internal/lexer"
)

type ParserError struct {
	Inner    error
	Location lexer.Location
}

func (e *ParserError) Error() string {
	return fmt.Sprintf("%s at %s", e.Inner, &e.Location)
}

type UnexpectedTokenError struct {
	Got      string
	Expected string
}

func (e *UnexpectedTokenError) Error() string {
	return fmt.Sprintf("expected %q, found %q", e.Expected, e.Got)
}

type parser struct {
	tokens []lexer.Token
	index  int

	errs []*ParserError
}

func Parse(tokens []lexer.Token) (*File, error) {
	p := parser{
		tokens: tokens,
	}

	if tokens[len(tokens)-1].Type != lexer.TokenEOF {
		return nil, errors.New("last token must be EOF")
	}

	return p.parseFile(), nil
}

func (p *parser) take() (tk *lexer.Token) {
	if p.index >= len(p.tokens)-1 {
		return &p.tokens[len(p.tokens)-1] // Last token should be EOF
	}

	tk = &p.tokens[p.index]
	p.index++

	return tk
}

func (p *parser) mustTake(typ lexer.TokenType) (tk *lexer.Token, found bool) {
	tk = p.take()
	if tk.Type != typ {
		p.addErrorAt(&UnexpectedTokenError{
			Got:      tk.Contents,
			Expected: typ.String(),
		}, tk.Start)
		return nil, false
	}

	return tk, true
}

func (p *parser) rewind() {
	if p.index == 0 {
		panic("cannot rewind any further")
	}

	p.index--
}

func (p *parser) isEOF() bool {
	return p.tokens[p.index].Type == lexer.TokenEOF
}

func (p *parser) addErrorAt(err error, pos lexer.Location) {
	p.errs = append(p.errs, &ParserError{
		Inner:    err,
		Location: pos,
	})
}

func (p *parser) addError(err error) {
	p.addErrorAt(err, p.tokens[p.index].Start)
}

func (p *parser) parseFile() *File {
	f := File{}

	for !p.isEOF() {
		node := p.parseTopLevelNode()
		f.Nodes = append(f.Nodes, node)
	}

	return &f
}

func (p *parser) parseTopLevelNode() Node {
	tk := p.take()

	switch tk.Type {
	case lexer.TokenTagName:
		return p.parseTag(tk.Start, tk.Contents)

	case lexer.TokenDot, lexer.TokenHashtag:
		p.rewind()
		return p.parseTag(tk.Start, "div")
	}

	panic("invalid")
}

func (p *parser) parseTag(start lexer.Location, name string) *NodeTag {
	tagNode := NodeTag{
		pos:  pos(start),
		Name: name,
	}

	var classes []string

loop:
	for {
		tk := p.take()

		switch tk.Type {
		case lexer.TokenDot:
			tk := p.take()

			if tk.Type != lexer.TokenClassName {
				panic("invalid")
			}

			classes = append(classes, tk.Contents)

		case lexer.TokenNewLine:
			break loop

		case lexer.TokenParenOpen:
			tagNode.Attributes = p.parseTagAttributes()
		}
	}

	if len(classes) > 0 {
		tagNode.Attributes = append(tagNode.Attributes, TagAttribute{
			Name: "class",
			Value: Value{
				Contents: strings.Join(classes, " "),
			},
		})
	}

	return &tagNode
}

func (p *parser) parseTagAttributes() []TagAttribute {
	attrs := []TagAttribute{}

loop:
	for {
		tkName := p.take()
		if tkName.Type == lexer.TokenParenClose {
			break
		}
		if tkName.Type != lexer.TokenAttributeName {
			p.addError(&UnexpectedTokenError{
				Got:      tkName.Contents,
				Expected: "an attribute name",
			})
			break
		}

		_, ok := p.mustTake(lexer.TokenEquals)
		if !ok {
			break
		}

		var value Value

		tkValue := p.take()

		switch tkValue.Type {
		case lexer.TokenQuotedString:
			value = Value{
				pos:      pos(tkValue.Start),
				Contents: tkValue.Contents,
			}

		case lexer.TokenGoExpr:
			value = Value{
				pos:            pos(tkValue.Start),
				Contents:       tkValue.Contents,
				IsGoExpression: true,
			}

		default:
			p.addError(&UnexpectedTokenError{
				Got:      tkValue.Contents,
				Expected: "an attribute value",
			})
			break loop
		}

		attrs = append(attrs, TagAttribute{
			pos:   pos(tkName.Start),
			Name:  tkName.Contents,
			Value: value,
		})
	}

	return attrs
}
