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

var selfClosingTags = map[string]struct{}{
	"area":   {},
	"base":   {},
	"br":     {},
	"col":    {},
	"embed":  {},
	"hr":     {},
	"img":    {},
	"input":  {},
	"link":   {},
	"meta":   {},
	"param":  {},
	"source": {},
	"track":  {},
	"wbr":    {},
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

func (p *parser) peek() *lexer.Token {
	if p.index >= len(p.tokens) {
		return &p.tokens[len(p.tokens)-1] // Last token should be EOF
	}

	return &p.tokens[p.index]
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
		Name:  strings.TrimSuffix(fname, filepath.Ext(fname)),
		Nodes: p.parseNodesBlock(0),
	}

	return &f
}

func (p *parser) parseNodesBlock(depth int) (nodes []Node) {
	var lastIf *NodeGoStatement

	for {
		tk := p.take()
		if tk.Type == lexer.TokenEOF {
			break
		}
		if tk.Type == lexer.TokenNewLine {
			continue
		}

		if tk.Depth > depth {
			p.addErrorAt(errors.New("unexpected indentation"), tk.Start)
			break
		}
		if tk.Depth < depth {
			p.rewind()
			break
		}

		p.rewind()
		node := p.parseNode(lastIf != nil)

		if st, ok := node.(*NodeGoStatement); ok {
			switch st.Keyword {
			case lexer.TokenStartIf:
				lastIf = st
			case lexer.TokenStartElse:
				lastIf.HasElse = true
			}
		} else {
			lastIf = nil
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func (p *parser) parseNode(hasSeenIf bool) Node {
	tk := p.take()

	switch tk.Type {
	case lexer.TokenKeyword:
		p.rewind()
		return p.parseKeyword()

	case lexer.TokenTagName:
		return p.parseTag(tk.Depth, tk.Start, tk.Contents)

	case lexer.TokenDot, lexer.TokenHashtag:
		p.rewind()
		return p.parseTag(tk.Depth, tk.Start, "div")

	case lexer.TokenInterpolationStart:
		tkKeyword := p.take()
		stmt := NodeGoStatement{
			pos:     pos(tkKeyword.Start),
			Keyword: tkKeyword.Type,
		}

		switch tkKeyword.Type {
		case lexer.TokenStartIf, lexer.TokenStartFor:
			tk, ok := p.mustTake(lexer.TokenGoExpr)
			if !ok {
				return nil
			}
			stmt.Argument = tk.Contents

		case lexer.TokenStartElse:
			if !hasSeenIf {
				p.addErrorAt(errors.New(`found "else" without matching "if"`), tkKeyword.Start)
			}

		default:
			p.addErrorAt(&UnexpectedTokenError{
				Got:      tkKeyword.Contents,
				Expected: "a valid statement keyword",
			}, tkKeyword.Start)
			return nil
		}

		stmt.Nodes = p.parseNodesBlock(tk.Depth + 1)

		return &stmt

	case lexer.TokenPipe:
		var val Value

		if p.peek().Type == lexer.TokenNewLine {
			val = ValueLiteral{
				Contents: "\n",
			}
		} else {
			val = p.parseInlineValue()
		}

		return &NodeText{
			pos:  pos(tk.Start),
			Text: val,
		}
	}

	p.addErrorAt(&UnexpectedTokenError{
		Got:      tk.Contents,
		Expected: "a valid node",
	}, tk.Start)
	return nil
}

func (p *parser) parseTag(depth int, start lexer.Location, name string) Node {
	tagNode := NodeTag{
		pos:  pos(start),
		Name: name,
	}
	_, tagNode.IsSelfClosing = selfClosingTags[name]

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
				tagNode.Nodes = append(tagNode.Nodes, &NodeText{
					pos:  pos(v.Position()),
					Text: v,
				})
			}
		}
	}

	tagNode.Nodes = append(tagNode.Nodes, p.parseNodesBlock(depth+1)...)

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

func (p *parser) parseKeyword() Node {
	tk, ok := p.mustTake(lexer.TokenKeyword)
	if !ok {
		return nil
	}

	switch tk.Contents {
	case "arg":
		tkArg, ok := p.mustTake(lexer.TokenTagInlineText)
		if !ok {
			return nil
		}

		return &NodeArg{
			pos: pos(tk.Start),
			Arg: tkArg.Contents,
		}
	}

	p.addErrorAt(&UnexpectedTokenError{
		Got:      tk.Contents,
		Expected: "a known keyword",
	}, tk.Start)
	return nil
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
