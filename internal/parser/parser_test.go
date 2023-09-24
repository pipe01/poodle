package parser

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pipe01/poodle/internal/lexer"
	"github.com/stretchr/testify/assert"
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
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "input", n.Name, "tag name")
				})
				return nil
			},
		},
		{
			name: "simple tag with class",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "foo"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "input", n.Name, "tag name")
					assert.Equal(f.T, "class", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, "foo", n.Attributes[0].Value.Contents, "class name")
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
					assert.Equal(f.T, "div", n.Name, "tag name")
					assert.Equal(f.T, "class", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, "foo", n.Attributes[0].Value.Contents, "class name")
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
					assert.Equal(f.T, "div", n.Name, "tag name")
					assert.Equal(f.T, "class", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, "foo bar", n.Attributes[0].Value.Contents, "class name")
				})
				return nil
			},
		},

		{
			name: "shortcut div with id",
			tks: []lexer.Token{
				{Type: lexer.TokenHashtag},
				{Type: lexer.TokenID, Contents: "foo"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "div", n.Name, "tag name")
					assert.Equal(f.T, "id", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, "foo", n.Attributes[0].Value.Contents, "id name")
				})
				return nil
			},
		},
		{
			name: "shortcut div with id and class",
			tks: []lexer.Token{
				{Type: lexer.TokenHashtag},
				{Type: lexer.TokenID, Contents: "foo"},
				{Type: lexer.TokenDot},
				{Type: lexer.TokenClassName, Contents: "bar"},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "div", n.Name, "tag name")
					assert.Equal(f.T, "class", n.Attributes[0].Name, "class attr name")
					assert.Equal(f.T, "bar", n.Attributes[0].Value.Contents, "class name")
					assert.Equal(f.T, "id", n.Attributes[1].Name, "id attr name")
					assert.Equal(f.T, "foo", n.Attributes[1].Value.Contents, "id name")
				})
				return nil
			},
		},

		{
			name: "div with one attribute",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenParenOpen},
				{Type: lexer.TokenAttributeName, Contents: "foo"},
				{Type: lexer.TokenEquals},
				{Type: lexer.TokenQuotedString, Contents: `"bar"`},
				{Type: lexer.TokenParenClose},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "input", n.Name, "tag name")
					assert.Equal(f.T, "foo", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, `"bar"`, n.Attributes[0].Value.Contents, "name")
				})
				return nil
			},
		},
		{
			name: "div with two attributes",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenParenOpen},
				{Type: lexer.TokenAttributeName, Contents: "foo"},
				{Type: lexer.TokenEquals},
				{Type: lexer.TokenQuotedString, Contents: `"bar"`},
				{Type: lexer.TokenAttributeName, Contents: "foo2"},
				{Type: lexer.TokenEquals},
				{Type: lexer.TokenQuotedString, Contents: `"bar2"`},
				{Type: lexer.TokenParenClose},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "input", n.Name, "tag name")
					assert.Equal(f.T, "foo", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, `"bar"`, n.Attributes[0].Value.Contents, "name")
					assert.Equal(f.T, "foo2", n.Attributes[1].Name, "attr name")
					assert.Equal(f.T, `"bar2"`, n.Attributes[1].Value.Contents, "name")
				})
				return nil
			},
		},
		{
			name: "div with Go attribute",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenParenOpen},
				{Type: lexer.TokenAttributeName, Contents: "foo"},
				{Type: lexer.TokenEquals},
				{Type: lexer.TokenGoExpr, Contents: `(bar)`},
				{Type: lexer.TokenParenClose},
				{Type: lexer.TokenEOF},
			},
			verify: func(f *TestFile) error {
				f.OnlyNode().Run(func(n *NodeTag) {
					assert.Equal(f.T, "input", n.Name, "tag name")
					assert.Equal(f.T, "foo", n.Attributes[0].Name, "attr name")
					assert.Equal(f.T, `(bar)`, n.Attributes[0].Value.Contents, "name")
					assert.Equal(f.T, true, n.Attributes[0].Value.IsGoExpression, "name")
				})
				return nil
			},
		},

		{
			name: "error: no class after dot",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenDot},
				{Type: lexer.TokenQuotedString, Contents: `"hello"`},
				{Type: lexer.TokenEOF},
			},
			expectErr: &UnexpectedTokenError{
				Got:      `"hello"`,
				Expected: lexer.TokenClassName.String(),
			},
			verify: func(f *TestFile) error {
				return nil
			},
		},
		{
			name: "error: no id after hashtag",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenHashtag},
				{Type: lexer.TokenQuotedString, Contents: `"hello"`},
				{Type: lexer.TokenEOF},
			},
			expectErr: &UnexpectedTokenError{
				Got:      `"hello"`,
				Expected: lexer.TokenID.String(),
			},
			verify: func(f *TestFile) error {
				return nil
			},
		},
		{
			name: "error: invalid top level node",
			tks: []lexer.Token{
				{Type: lexer.TokenQuotedString, Contents: `"hello"`},
				{Type: lexer.TokenEOF},
			},
			expectErr: &UnexpectedTokenError{
				Got:      `"hello"`,
				Expected: "a valid top-level node",
			},
			verify: func(f *TestFile) error {
				return nil
			},
		},
		{
			name: "error: missing attribute name",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenParenOpen},
				{Type: lexer.TokenQuotedString, Contents: `"bar"`},
				{Type: lexer.TokenParenClose},
				{Type: lexer.TokenEOF},
			},
			expectErr: &UnexpectedTokenError{
				Got:      `"bar"`,
				Expected: "an attribute name",
			},
			verify: func(f *TestFile) error {
				return nil
			},
		},
		{
			name: "error: missing attribute value",
			tks: []lexer.Token{
				{Type: lexer.TokenIdentifier, Contents: "input"},
				{Type: lexer.TokenParenOpen},
				{Type: lexer.TokenAttributeName, Contents: `foo`},
				{Type: lexer.TokenEquals},
				{Type: lexer.TokenParenClose},
				{Type: lexer.TokenEOF},
			},
			expectErr: &UnexpectedTokenError{
				Got:      ``,
				Expected: "an attribute value",
			},
			verify: func(f *TestFile) error {
				return nil
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			f, err := Parse(c.tks)
			if err != nil && (c.expectErr == nil || errors.Unwrap(err).Error() != c.expectErr.Error()) {
				t.Fatalf("failed to parse tokens: %s", err)
			}

			tf := TestFile{
				File: f,
				T:    t,
			}

			err = c.verify(&tf)
			if err != nil {
				t.Fatalf("failed to verify result: %s", err)
			}
		})
	}
}
