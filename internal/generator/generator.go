package generator

import (
	"io"

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

	for _, n := range f.Nodes {
		c.visitNode(n)
	}

	c.w.WriteFuncFooter()
}

func (c *context) visitNode(n parser.Node) {
	switch n := n.(type) {
	case *parser.NodeTag:
		c.w.WriteLiteralUnescapedf("<%s", n.Name)

		for _, attr := range n.Attributes {
			c.w.WriteLiteralUnescapedf(` %s="`, attr.Name)
			c.visitValue(attr.Value)
			c.w.WriteLiteralUnescaped(`"`)
		}

		c.w.WriteLiteralUnescaped(">")

		for _, n := range n.Nodes {
			c.visitNode(n)
		}

		c.w.WriteLiteralUnescapedf("</%s>", n.Name)
	}
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
