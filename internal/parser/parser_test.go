package parser

import (
	"reflect"
	"testing"

	"github.com/pipe01/poodle/internal/lexer"
)

type TestFile struct {
	*File
	T *testing.T
}

func (t *TestFile) OnlyNode() TestNode {
	if len(t.File.Nodes) != 1 {
		t.T.Fatalf("expected 1 node, got %d", len(t.Nodes))
	}

	return TestNode{
		Node: t.Nodes[0],
		T:    t.T,
	}
}

func (t *TestFile) NodeAt(idx int) TestNode {
	if len(t.File.Nodes) != 1 {
		t.T.Fatalf("expected at least %d nodes, got %d", idx+1, len(t.Nodes))
	}

	return TestNode{
		Node: t.Nodes[idx],
		T:    t.T,
	}
}

type TestNode struct {
	Node
	T *testing.T
}

func (t TestNode) Run(fn interface{}) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func || fnType.NumIn() != 1 {
		panic("invalid function")
	}

	wantNodeType := fnType.In(0)
	actualNodeType := reflect.TypeOf(t.Node)

	if !actualNodeType.AssignableTo(wantNodeType) {
		t.T.Fatalf("expected node type %q, found %q", wantNodeType, actualNodeType)
	}

	reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(t.Node)})
}

func assert[T comparable](t *testing.T, expected, got T, msg string) {
	if got != expected {
		t.Fatalf("%s: expected %v, got %v", msg, expected, got)
	}
}

func TestParser(t *testing.T) {
	type testCase struct {
		name      string
		expectErr error
		tks       []lexer.Token
		verify    func(f *TestFile) error
	}

	cases := []testCase{
		{
			name: "simple tag",
			tks: []lexer.Token{
				{Type: lexer.TokenTagName, Contents: "input"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert(f.T, "input", n.Name, "tag name")
				})
				return nil
			},
		},
		{
			name: "simple tag with class",
			tks: []lexer.Token{
				{Type: lexer.TokenTagName, Contents: "input"},
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "foo"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert(f.T, "input", n.Name, "tag name")
					assert(f.T, "foo", n.Attributes[0].Value.Contents, "class name")
				})
				return nil
			},
		},
		{
			name: "shortcut div with class",
			tks: []lexer.Token{
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "foo"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert(f.T, "div", n.Name, "tag name")
					assert(f.T, "foo", n.Attributes[0].Value.Contents, "class name")
				})
				return nil
			},
		},
		{
			name: "shortcut div with multiple classes",
			tks: []lexer.Token{
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "foo"},
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "bar"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert(f.T, "div", n.Name, "tag name")
					assert(f.T, "foo bar", n.Attributes[0].Value.Contents, "class name")
				})
				return nil
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			f, err := Parse(c.tks)
			if err != nil {
				t.Fatalf("failed to parse tokens: %s", err)
			}

			tf := TestFile{
				File: f,
				T:    t,
			}

			err = c.verify(&tf)
			if err != c.expectErr {
				t.Fatalf("failed to verify result: %s", err)
			}
		})
	}
}
