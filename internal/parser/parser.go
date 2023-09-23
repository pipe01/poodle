package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pipe01/poodle/internal/lexer"
	"golang.org/x/exp/slices"
)

var ErrLastTokenEOF = errors.New("last token must be EOF")

type ParserError struct {
	Inner    error
	Location lexer.Location
}

func (e *ParserError) Unwrap() error {
	return e.Inner
}

func (e *ParserError) Error() string {
	return fmt.Sprintf("%s at %s", e.Inner, &e.Location)
}

type UnexpectedTokenError struct {
	Got      string
	Expected string
}

func (e *UnexpectedTokenError) Error() string {
	return fmt.Sprintf("expected %s, found %q", e.Expected, e.Got)
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
		return nil, ErrLastTokenEOF
	}

	f := p.parseFile()
	if len(p.errs) > 0 {
		return nil, p.errs[0]
	}

	return f, nil
}

func (p *parser) take() (tk *lexer.Token) {
	if p.index >= len(p.tokens) {
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
	fname := p.tokens[0].Start.File

	f := File{
		Name: strings.TrimSuffix(fname, filepath.Ext(fname)),
	}

	for !p.isEOF() {
		node := p.parseTopLevelNode()

		if node != nil {
			f.Nodes = append(f.Nodes, node)
		}
	}

	return &f
}

func (p *parser) parseTopLevelNode() Node {
	tk := p.take()

	switch tk.Type {
	case lexer.TokenTagName:
		return p.parseTag(tk.Depth, tk.Start, tk.Contents)

	case lexer.TokenDot, lexer.TokenHashtag:
		p.rewind()
		return p.parseTag(tk.Depth, tk.Start, "div")
	}

	p.addErrorAt(&UnexpectedTokenError{
		Got:      tk.Contents,
		Expected: "a valid top-level node",
	}, tk.Start)
	return nil
}

func (p *parser) parseTag(depth int, start lexer.Location, name string) *NodeTag {
	tagNode := NodeTag{
		pos:  pos(start),
		Name: name,
	}

	var classes []string
	var idTok *lexer.Token

loop:
	for {
		tk := p.take()

		switch tk.Type {
		case lexer.TokenDot:
			tk, ok := p.mustTake(lexer.TokenClassName)
			if !ok {
				continue
			}

			classes = append(classes, tk.Contents)

		case lexer.TokenHashtag:
			tk, ok := p.mustTake(lexer.TokenID)
			if !ok {
				continue
			}

			idTok = tk

		case lexer.TokenParenOpen:
			tagNode.Attributes = p.parseTagAttributes()

		case lexer.TokenNewLine, lexer.TokenEOF:
			break loop

		default:
			p.rewind()

			v := p.parseInlineValue()
			if v != nil {
				tagNode.TextLines = []Value{v}
			}
		}
	}

	// Parse child nodes and text
	for {
		tk := p.take()
		if tk.Depth > depth+1 {
			p.addErrorAt(errors.New("unexpected indentation"), tk.Start)
			break
		}
		if tk.Depth <= depth {
			p.rewind()
			break
		}
		if tk.Type == lexer.TokenNewLine {
			continue
		}

		if tk.Type == lexer.TokenPipe {
			val := p.parseInlineValue()
			tagNode.TextLines = append(tagNode.TextLines, val)
		} else {
			name := tk.Contents
			if tk.Type != lexer.TokenTagName {
				p.rewind()
				name = "div"
			}

			tag := p.parseTag(tk.Depth, tk.Start, name)
			if tag != nil {
				tagNode.Nodes = append(tagNode.Nodes, tag)
			}
		}
	}

	if len(classes) > 0 {
		classAttrIdx := slices.IndexFunc(tagNode.Attributes, func(e TagAttribute) bool {
			return e.Name == "class"
		})
		joined := strings.Join(classes, " ")

		if classAttrIdx < 0 {
			tagNode.Attributes = append(tagNode.Attributes, TagAttribute{
				Name: "class",
				Value: ValueLiteral{
					Contents: joined,
				},
			})
		} else {
			attr := &tagNode.Attributes[classAttrIdx]
			attr.Value = concatValues(attr.Value, ValueLiteral{
				Contents: " " + joined,
			})
		}
	}

	if idTok != nil {
		hasIDAttr := slices.ContainsFunc(tagNode.Attributes, func(e TagAttribute) bool {
			return e.Name == "id"
		})

		if !hasIDAttr {
			tagNode.Attributes = append(tagNode.Attributes, TagAttribute{
				pos:  pos(idTok.Start),
				Name: "id",
				Value: ValueLiteral{
					pos:      pos(idTok.Start),
					Contents: idTok.Contents,
				},
			})
		}
	}

	return &tagNode
}

func (p *parser) parseTagAttributes() []TagAttribute {
	attrs := []TagAttribute{}

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

		value := p.parseAttributeValue()
		if value == nil {
			continue
		}

		attrs = append(attrs, TagAttribute{
			pos:   pos(tkName.Start),
			Name:  tkName.Contents,
			Value: value,
		})
	}

	return attrs
}

func (p *parser) parseAttributeValue() Value {
	var val Value

loop:
	for {
		tk := p.take()

		switch tk.Type {
		case lexer.TokenQuotedString:
			val = concatValues(val, ValueLiteral{
				pos:      pos(tk.Start),
				Contents: strings.TrimPrefix(strings.TrimSuffix(tk.Contents, `"`), `"`),
			})

		case lexer.TokenGoExpr:
			val = concatValues(val, ValueGoExpr{
				pos:      pos(tk.Start),
				Contents: tk.Contents,
			})

		default:
			if val == nil {
				p.addError(&UnexpectedTokenError{
					Got:      tk.Contents,
					Expected: "an attribute value",
				})
			} else {
				p.rewind()
			}
			break loop
		}
	}

	return val
}

func (p *parser) parseInlineValue() Value {
	var val Value

loop:
	for {
		tk := p.take()

		switch tk.Type {
		case lexer.TokenTagInlineText:
			val = concatValues(val, ValueLiteral{
				pos:      pos(tk.Start),
				Contents: tk.Contents,
			})

		case lexer.TokenInterpolationStart:
			tk, ok := p.mustTake(lexer.TokenGoExpr)
			if !ok {
				continue
			}

			val = concatValues(val, ValueGoExpr{
				pos:      pos(tk.Start),
				Contents: tk.Contents,
			})

		default:
			if val == nil {
				p.addError(&UnexpectedTokenError{
					Got:      tk.Contents,
					Expected: "an inline value",
				})
			} else {
				p.rewind()
			}
			break loop
		}
	}

	return val
}

func concatValues(a, b Value) Value {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	return ValueConcat{
		pos: pos(a.Position()),
		A:   a,
		B:   b,
	}
}
