package generator

import (
	"fmt"
	"io"
	"reflect"
	"strings"

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
	args := []string{}
	for _, n := range f.Nodes {
		if n, ok := n.(*parser.NodeArg); ok {
			args = append(args, n.Arg)
		}
	}

	c.w.WriteFileHeader("main")
	c.w.WriteFuncHeader(f.Name, args)

	c.visitNodes(f.Nodes)

	c.w.WriteBlockEnd(true)
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

	case *parser.NodeGoBlock:
		c.visitNodeGoBlock(n)

	case *parser.NodeArg:
		// Skip, already handled in visitFile

	case *parser.NodeMixinDef:
		c.visitNodeMixinDef(n)

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
	c.w.WriteStatementStart(!n.HasElse, string(n.Keyword), n.Argument)
	c.visitNodes(n.Nodes)
	c.w.WriteBlockEnd(!n.HasElse)
}

func (c *context) visitNodeGoBlock(n *parser.NodeGoBlock) {
	c.w.WriteGoBlock(n.Contents)
}

func (c *context) visitNodeMixinDef(n *parser.NodeMixinDef) {
	args := make([]string, len(n.Args))
	for i, a := range n.Args {
		args[i] = fmt.Sprintf("%s %s", a.Name, a.Type)
	}

	c.w.WriteFuncVariableStart("_mixin_"+n.Name, strings.Join(args, ", "))

	c.visitNodes(n.Nodes)

	c.w.WriteBlockEnd(true)
}

func (c *context) visitValue(v parser.Value) {
	switch v := v.(type) {
	case parser.ValueLiteral:
		c.w.WriteLiteralUnescapedf(`%s`, v.Contents)

	case parser.ValueGoExpr:
		if v.EscapeHTML {
			c.w.WriteGoEscaped(v.Contents)
		} else {
			c.w.WriteGoUnescaped(v.Contents)
		}

	case parser.ValueConcat:
		c.visitValue(v.A)
		c.visitValue(v.B)
	}
}
