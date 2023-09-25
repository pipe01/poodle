package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pipe01/poodle/internal/lexer"
	"github.com/pipe01/poodle/internal/parser"
	"github.com/pipe01/poodle/internal/parser/ast"
)

type Workspace struct {
	rootPath string

	parsedFiles map[string]*ast.File
}

func New(rootPath string) *Workspace {
	return &Workspace{
		rootPath:    rootPath,
		parsedFiles: make(map[string]*ast.File),
	}
}

func (w *Workspace) Load(relPath string) (*ast.File, error) {
	fullPath := filepath.Join(w.rootPath, relPath)

	if f, ok := w.parsedFiles[fullPath]; ok {
		return f, nil
	}

	bytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	l := lexer.New(bytes, relPath)
	tks, err := l.Collect()
	if err != nil {
		return nil, fmt.Errorf("lex file: %w", err)
	}

	file, err := parser.Parse(tks, w.Load)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	w.parsedFiles[fullPath] = file
	return file, nil
}
