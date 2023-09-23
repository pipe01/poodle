package generator

import (
	"fmt"
	"io"
	"reflect"

	"github.com/pipe01/poodle/internal/parser"
)

func Visit(w io.Writer, f *parser.File) {
	ctx := context{
		w: &outputWriter{
			w: w,
		},
	}

	ctx.visitFile(f)
}

type context struct {
	w *outputWriter
}

func (c *context) visitFile(f *parser.File) {
	c.w.WriteFileHeader("main")
	c.w.WriteFuncHeader(f.Name)

	c.visitNodes(f.Nodes)

	c.w.WriteFuncFooter()
}

func (c *context) visitNodes(nodes []parser.Node) {
	for _, n := range nodes {
		c.visitNode(n)
	}
}

func (c *context) visitNode(n parser.Node) {
	switch n := n.(type) {
	case *parser.NodeTag:
		c.visitNodeTag(n)

	case *parser.NodeText:
		c.visitValue(n.Text)

	case *parser.NodeGoStatement:
		c.visitNodeGoStatement(n)

	default:
		panic(fmt.Errorf("unknown node type %s", reflect.ValueOf(n).String()))
	}
}

func (c *context) visitNodeTag(n *parser.NodeTag) {
	c.w.WriteLiteralUnescapedf("<%s", n.Name)

	for _, attr := range n.Attributes {
		c.w.WriteLiteralUnescapedf(` %s="`, attr.Name)
		c.visitValue(attr.Value)
		c.w.WriteLiteralUnescaped(`"`)
	}

	if n.IsSelfClosing {
		c.w.WriteLiteralUnescaped("/>")
	} else {
		c.w.WriteLiteralUnescaped(">")

		for _, n := range n.Nodes {
			c.visitNode(n)
		}

		c.w.WriteLiteralUnescapedf("</%s>", n.Name)
	}
}

func (c *context) visitNodeGoStatement(n *parser.NodeGoStatement) {
	c.w.WriteStatementStart(!n.HasElse, n.Keyword, n.Argument)
	c.visitNodes(n.Nodes)
	c.w.WriteStatementEnd(!n.HasElse)
}

func (c *context) visitValue(v parser.Value) {
	switch v := v.(type) {
	case parser.ValueLiteral:
		c.w.WriteLiteralUnescapedf(`%s`, v.Contents)

	case parser.ValueGoExpr:
		c.w.WriteGoEscaped(v.Contents)

	case parser.ValueConcat:
		c.visitValue(v.A)
		c.visitValue(v.B)
	}
}
