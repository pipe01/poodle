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
		mixins: make(map[string]*parser.NodeMixinDef),
	}

	ctx.visitFile(f)
}

type context struct {
	w *outputWriter

	mixins map[string]*parser.NodeMixinDef
}

func (c *context) visitFile(f *parser.File) error {
	args := []string{}
	for _, n := range f.Nodes {
		if n, ok := n.(*parser.NodeArg); ok {
			args = append(args, n.Arg)
		}
	}

	c.w.WriteFileHeader("main")
	c.w.WriteFuncHeader(f.Name, args)

	err := c.visitNodes(f.Nodes)
	if err != nil {
		return err
	}

	c.w.WriteBlockEnd(true)
	return nil
}

func (c *context) visitNodes(nodes []parser.Node) error {
	var err error

	for _, n := range nodes {
		err = c.visitNode(n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *context) visitNode(n parser.Node) error {
	switch n := n.(type) {
	case *parser.NodeTag:
		c.visitNodeTag(n)

	case *parser.NodeText:
		c.visitValue(n.Text)

	case *parser.NodeGoStatement:
		return c.visitNodeGoStatement(n)

	case *parser.NodeGoBlock:
		c.visitNodeGoBlock(n)

	case *parser.NodeArg:
		// Skip, already handled in visitFile

	case *parser.NodeMixinDef:
		return c.visitNodeMixinDef(n)

	case *parser.NodeMixinCall:
		return c.visitNodeMixinCall(n)

	default:
		return fmt.Errorf("unknown node type %s", reflect.ValueOf(n).String())
	}

	return nil
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

func (c *context) visitNodeGoStatement(n *parser.NodeGoStatement) error {
	c.w.WriteStatementStart(!n.HasElse, string(n.Keyword), n.Argument)
	if err := c.visitNodes(n.Nodes); err != nil {
		return err
	}
	c.w.WriteBlockEnd(!n.HasElse)

	return nil
}

func (c *context) visitNodeGoBlock(n *parser.NodeGoBlock) {
	c.w.WriteGoBlock(n.Contents)
}

func (c *context) visitNodeMixinDef(n *parser.NodeMixinDef) error {
	c.mixins[n.Name] = n

	args := make([]string, len(n.Args))
	for i, a := range n.Args {
		args[i] = fmt.Sprintf("%s %s", a.Name, a.Type)
	}

	c.w.WriteFuncVariableStart(mixinFuncName(n.Name), strings.Join(args, ", "))

	if err := c.visitNodes(n.Nodes); err != nil {
		return err
	}

	c.w.WriteBlockEnd(true)
	return nil
}

func (c *context) visitNodeMixinCall(n *parser.NodeMixinCall) error {
	mixinDef, ok := c.mixins[n.Name]
	if !ok {
		return fmt.Errorf("mixin %q not found", n.Name)
	}

	args := strings.Join(n.Args, ", ")
	c.w.WriteGoBlock(fmt.Sprintf("%s(%s)\n", mixinFuncName(mixinDef.Name), args))

	return nil
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

func mixinFuncName(mixinName string) string {
	return "_mixin_" + mixinName
}
