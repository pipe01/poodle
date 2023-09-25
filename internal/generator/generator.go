package generator

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/pipe01/poodle/internal/parser/ast"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Options struct {
	Package     string
	ForceExport bool
}

type context struct {
	w    OutputWriter
	opts Options

	mixins map[string]*ast.NodeMixinDef

	callDepth int
}

func Visit(w io.Writer, f *ast.File, opts Options) error {
	ctx := context{
		w: &outputWriter{
			w: w,
		},
		opts:   opts,
		mixins: make(map[string]*ast.NodeMixinDef),
	}

	return ctx.visitFile(f)
}

func (c *context) visitFile(f *ast.File) error {
	for _, n := range f.Nodes {
		switch n := n.(type) {
		case *ast.NodeMixinDef:
			c.mixins[n.Name] = n
		}
	}

	importsMap := map[string]struct{}{
		`"bufio"`: {},
		`"html"`:  {},
	}
	for _, i := range f.Imports {
		importsMap[i] = struct{}{}
	}

	imports := maps.Keys(importsMap)
	slices.Sort(imports)

	c.w.WriteFileHeader(c.opts.Package, imports)

	name := f.Name
	if c.opts.ForceExport {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	c.w.WriteFuncHeader(name, f.Args)

	err := c.visitNodes(f.Nodes)
	if err != nil {
		return err
	}

	c.w.WriteBlockEnd(true)
	return nil
}

func (c *context) visitNodes(nodes []ast.Node) error {
	var err error

	for _, n := range nodes {
		err = c.visitNode(n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *context) visitNode(n ast.Node) error {
	switch n := n.(type) {
	case *ast.NodeTag:
		return c.visitNodeTag(n)

	case *ast.NodeText:
		c.visitValue(n.Text)

	case *ast.NodeGoStatement:
		return c.visitNodeGoStatement(n)

	case *ast.NodeGoBlock:
		c.visitNodeGoBlock(n)

	case *ast.NodeMixinCall:
		return c.visitNodeMixinCall(n)

	case *ast.NodeInclude:
		return c.visitNodes(n.File.Nodes)

	case *ast.NodeMixinDef:
		// Skip, already handled in visitFile

	default:
		return fmt.Errorf("unknown node type %s", reflect.ValueOf(n).String())
	}

	return nil
}

func (c *context) visitNodeTag(n *ast.NodeTag) error {
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
			err := c.visitNode(n)
			if err != nil {
				return err
			}
		}

		c.w.WriteLiteralUnescapedf("</%s>", n.Name)
	}

	return nil
}

func (c *context) visitNodeGoStatement(n *ast.NodeGoStatement) error {
	c.w.WriteStatementStart(!n.HasElse, string(n.Keyword), n.Argument)
	if err := c.visitNodes(n.Nodes); err != nil {
		return err
	}
	c.w.WriteBlockEnd(!n.HasElse)

	return nil
}

func (c *context) visitNodeGoBlock(n *ast.NodeGoBlock) {
	c.w.WriteGoBlock(n.Contents)
}

func (c *context) visitNodeMixinCall(n *ast.NodeMixinCall) error {
	if c.callDepth >= 100 {
		return errors.New("max call depth reached")
	}

	mixinDef, ok := c.mixins[n.Name]
	if !ok {
		return fmt.Errorf("mixin %q not found", n.Name)
	}

	if len(n.Args) != len(mixinDef.Args) {
		return fmt.Errorf("mixin %q needs %d argument but %d were passed", n.Name, len(mixinDef.Args), len(n.Args))
	}

	c.w.WriteBlockStart()

	for i, arg := range mixinDef.Args {
		c.w.WriteVariable(arg.Name, n.Args[i])
	}

	c.callDepth++
	if err := c.visitNodes(mixinDef.Nodes); err != nil {
		return err
	}
	c.callDepth--

	c.w.WriteBlockEnd(true)

	return nil
}

func (c *context) visitValue(v ast.Value) {
	switch v := v.(type) {
	case ast.ValueLiteral:
		c.w.WriteLiteralUnescapedf(`%s`, v.Contents)

	case ast.ValueGoExpr:
		if v.EscapeHTML {
			c.w.WriteGoEscaped(v.Contents)
		} else {
			c.w.WriteGoUnescaped(v.Contents)
		}

	case ast.ValueConcat:
		c.visitValue(v.A)
		c.visitValue(v.B)
	}
}

func mixinFuncName(mixinName string) string {
	return "_mixin_" + mixinName
}
