package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pipe01/poodle/internal/lexer"
	. "github.com/pipe01/poodle/internal/parser/ast"
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

func (e *ParserError) At() lexer.Location {
	return e.Location
}

type UnexpectedTokenError struct {
	Got      *lexer.Token
	Expected string
}

func (e *UnexpectedTokenError) Error() string {
	return fmt.Sprintf("expected %s, found %q (%s)", e.Expected, e.Got.Contents, e.Got.Type)
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
	tokens   []lexer.Token
	loadFile func(string) (*File, error)
	index    int

	errs    []*ParserError
	imports []string
	args    []string
}

func Parse(tokens []lexer.Token, loadFile func(string) (*File, error)) (*File, error) {
	p := parser{
		tokens:   tokens,
		loadFile: loadFile,
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
			Got:      tk,
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

func (p *parser) addErrorAt(err error, Pos lexer.Location) {
	p.errs = append(p.errs, &ParserError{
		Inner:    err,
		Location: Pos,
	})
}

func (p *parser) addError(err error) {
	if p.index == len(p.tokens) {

	}

	p.addErrorAt(err, p.tokens[p.index].Start)
}

func (p *parser) parseFile() *File {
	fname := filepath.Base(p.tokens[0].Start.File)

	nodes := p.parseNodesBlock(0)

	f := File{
		Name:    strings.TrimSuffix(fname, filepath.Ext(fname)),
		Nodes:   nodes,
		Args:    p.args,
		Imports: p.imports,
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

		if node == nil {
			continue
		}

		if st, ok := node.(*NodeGoStatement); ok {
			switch st.Keyword {
			case KeywordIf:
				lastIf = st
			case KeywordElse:
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
	case lexer.TokenCommentStart:
		for p.peek().Type == lexer.TokenCommentText {
			p.take()
		}
		return nil

	case lexer.TokenCommentStartBuffered:
		tkText, ok := p.mustTake(lexer.TokenCommentText)
		if !ok {
			return nil
		}

		return &NodeComment{
			Pos:  Pos(tk.Start),
			Text: tkText.Contents,
		}

	case lexer.TokenKeyword:
		p.rewind()
		return p.parseKeyword()

	case lexer.TokenIdentifier:
		return p.parseTag(tk.Depth, tk.Start, tk.Contents)

	case lexer.TokenDot, lexer.TokenHashtag:
		p.rewind()
		return p.parseTag(tk.Depth, tk.Start, "div")

	case lexer.TokenInterpolationStart:
		tkKeyword := p.take()

		if tkKeyword.Type == lexer.TokenGoBlock {
			return &NodeGoBlock{
				Pos:      Pos(tkKeyword.Start),
				Contents: tkKeyword.Contents,
			}
		}

		stmt := NodeGoStatement{
			Pos:     Pos(tkKeyword.Start),
			Keyword: StatementKeyword(tkKeyword.Contents),
		}

		switch stmt.Keyword {
		case KeywordIf, KeywordFor:
			tk, ok := p.mustTake(lexer.TokenGoExpr)
			if !ok {
				return nil
			}
			stmt.Argument = tk.Contents

		case KeywordElse:
			if !hasSeenIf {
				p.addErrorAt(errors.New(`found "else" without matching "if"`), tkKeyword.Start)
			}

		default:
			p.addErrorAt(&UnexpectedTokenError{
				Got:      tkKeyword,
				Expected: "a valid Go statement or block",
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
			Pos:  Pos(tk.Start),
			Text: val,
		}

	case lexer.TokenPlus:
		return p.parseMixinCall(tk.Start)
	}

	p.addErrorAt(&UnexpectedTokenError{
		Got:      tk,
		Expected: "a valid node",
	}, tk.Start)
	return nil
}

func (p *parser) parseTag(depth int, start lexer.Location, name string) Node {
	tagNode := NodeTag{
		Pos:  Pos(start),
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

		case lexer.TokenColon:
			var txt strings.Builder

			for {
				tkLine := p.peek()
				if tkLine.Depth == tk.Depth {
					break
				}

				if tkLine.Type != lexer.TokenInlineText {
					p.addErrorAt(&UnexpectedTokenError{
						Got:      tkLine,
						Expected: "some block text",
					}, tkLine.Start)
					break
				}

				p.take()
				txt.WriteString(tkLine.Contents)
				txt.WriteByte('\n')
			}

			tagNode.Nodes = append(tagNode.Nodes, &NodeText{
				Pos:  Pos(tk.Start),
				Text: ValueLiteral{Contents: txt.String()},
			})
			break loop

		case lexer.TokenNewLine, lexer.TokenEOF:
			break loop

		default:
			p.rewind()

			v := p.parseInlineValue()
			if v != nil {
				tagNode.Nodes = append(tagNode.Nodes, &NodeText{
					Pos:  Pos(v.Position()),
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
				Pos:  Pos(idTok.Start),
				Name: "id",
				Value: ValueLiteral{
					Pos:      Pos(idTok.Start),
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
			p.addErrorAt(&UnexpectedTokenError{
				Got:      tkName,
				Expected: "an attribute name",
			}, tkName.Start)
			break
		}

		var value Value

		tkEquals := p.peek()
		if tkEquals.Type == lexer.TokenEquals {
			p.take()

			value = p.parseAttributeValue()
			if value == nil {
				continue
			}
		} else {
			value = ValueLiteral{
				Contents: tkName.Contents,
			}
		}

		attrs = append(attrs, TagAttribute{
			Pos:   Pos(tkName.Start),
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
				Pos:      Pos(tk.Start),
				Contents: strings.TrimPrefix(strings.TrimSuffix(tk.Contents, `"`), `"`),
			})

		case lexer.TokenGoExpr:
			val = concatValues(val, ValueGoExpr{
				Pos:      Pos(tk.Start),
				Contents: tk.Contents,
			})

		default:
			if val == nil {
				p.addErrorAt(&UnexpectedTokenError{
					Got:      tk,
					Expected: "an attribute value",
				}, tk.Start)
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
		case lexer.TokenInlineText:
			val = concatValues(val, ValueLiteral{
				Pos:      Pos(tk.Start),
				Contents: tk.Contents,
			})

		case lexer.TokenInterpolationStart:
			escape := true

			tk := p.take()
			if tk.Type == lexer.TokenExclamationPoint {
				escape = false
			} else {
				p.rewind()
			}

			tk, ok := p.mustTake(lexer.TokenGoExpr)
			if !ok {
				continue
			}

			val = concatValues(val, ValueGoExpr{
				Pos:        Pos(tk.Start),
				Contents:   tk.Contents,
				EscapeHTML: escape,
			})

		case lexer.TokenEOF:
			break loop

		default:
			if val == nil {
				p.addError(&UnexpectedTokenError{
					Got:      tk,
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
		tkArg, ok := p.mustTake(lexer.TokenInlineText)
		if !ok {
			return nil
		}

		p.args = append(p.args, tkArg.Contents)
		return nil

	case "import":
		tkPath, ok := p.mustTake(lexer.TokenImportPath)
		if !ok {
			return nil
		}

		p.imports = append(p.imports, tkPath.Contents)
		return nil

	case "mixin":
		return p.parseMixinDef()

	case "include":
		return p.parseInclude(tk.Start)

	case "doctype":
		tkValue, ok := p.mustTake(lexer.TokenInlineText)
		if !ok {
			return nil
		}

		var value string

		switch tkValue.Contents {
		case "5":
			value = "html"
		default:
			p.addErrorAt(&UnexpectedTokenError{
				Got:      tkValue,
				Expected: "a known doctype shorthand value",
			}, tkValue.Start)
			return nil
		}

		return &NodeDoctype{
			Pos:   Pos(tk.Start),
			Value: value,
		}
	}

	p.addErrorAt(&UnexpectedTokenError{
		Got:      tk,
		Expected: "a known keyword",
	}, tk.Start)
	return nil
}

func (p *parser) parseMixinDef() Node {
	tkName, ok := p.mustTake(lexer.TokenIdentifier)
	if !ok {
		return nil
	}

	mixin := NodeMixinDef{
		Pos:  Pos(tkName.Start),
		Name: tkName.Contents,
	}

	tk := p.take()
	if tk.Type == lexer.TokenParenOpen {
		// Parse arguments
		for {
			tkName, ok := p.mustTake(lexer.TokenIdentifier)
			if !ok {
				break
			}

			tkType, ok := p.mustTake(lexer.TokenIdentifier)
			if !ok {
				break
			}

			mixin.Args = append(mixin.Args, MixinArg{
				Name: tkName.Contents,
				Type: tkType.Contents,
			})

			tk := p.take()
			if tk.Type == lexer.TokenParenClose {
				break
			} else if tk.Type != lexer.TokenComma {
				p.addErrorAt(&UnexpectedTokenError{
					Got:      tk,
					Expected: "comma or right parenthesis",
				}, tk.Start)
				break
			}
		}
	} else if tk.Type != lexer.TokenNewLine {
		p.addErrorAt(&UnexpectedTokenError{
			Got:      tk,
			Expected: "a newline or arguments",
		}, tk.Start)
		return nil
	}

	// Parse children
	mixin.Nodes = p.parseNodesBlock(tkName.Depth + 1)

	return &mixin
}

func (p *parser) parseMixinCall(start lexer.Location) Node {
	tkName, ok := p.mustTake(lexer.TokenIdentifier)
	if !ok {
		return nil
	}

	args := []string{}

	tk := p.take()
	if tk.Type == lexer.TokenParenOpen {
		// Parse arguments
		for {
			tk, ok := p.mustTake(lexer.TokenGoExpr)
			if !ok {
				break
			}

			args = append(args, tk.Contents)

			tk = p.take()
			if tk.Type == lexer.TokenParenClose {
				break
			} else if tk.Type != lexer.TokenComma {
				p.addErrorAt(&UnexpectedTokenError{
					Got:      tk,
					Expected: "a comma or a right parenthesis",
				}, tk.Start)
			}
		}
	} else if tk.Type != lexer.TokenNewLine && tk.Type != lexer.TokenEOF {
		p.addErrorAt(&UnexpectedTokenError{
			Got:      tk,
			Expected: "a newline or arguments",
		}, tk.Start)
		return nil
	}

	return &NodeMixinCall{
		Pos:  Pos(start),
		Name: tkName.Contents,
		Args: args,
	}
}

func (p *parser) parseInclude(start lexer.Location) Node {
	tkPath, ok := p.mustTake(lexer.TokenImportPath)
	if !ok {
		return nil
	}

	fname := tkPath.Contents
	if !strings.ContainsRune(fname, '.') {
		fname += ".poo"
	}

	file, err := p.loadFile(fname)
	if err != nil {
		p.addErrorAt(fmt.Errorf("load included file: %w", err), tkPath.Start)
		return nil
	}

	p.args = append(p.args, file.Args...)
	p.imports = append(p.imports, file.Imports...)

	return &NodeInclude{
		Pos:  Pos(start),
		File: file,
		Path: tkPath.Contents,
	}
}

func concatValues(a, b Value) Value {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	return ValueConcat{
		Pos: Pos(a.Position()),
		A:   a,
		B:   b,
	}
}
