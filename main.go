package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/pipe01/poodle/internal/generator"
	"github.com/pipe01/poodle/internal/workspace"
)

func main() {
	outDir := kingpin.Flag("out-dir", "Folder to put generated files on").Default(".").String()
	files := kingpin.Arg("files", "List of files to compile").Required().Strings()

	kingpin.Parse()

	wd, _ := os.Getwd()
	ws := workspace.New(wd)

	if *outDir == "" {
		*outDir = wd
	}

	for _, fname := range *files {
		err := generateFile(ws, fname, *outDir)
		if err != nil {
			log.Fatalf("failed to load file %q: %s", fname, err)
		}
	}
}

func generateFile(ws *workspace.Workspace, fname string, outDir string) error {
	f, err := ws.Load(fname)
	if err != nil {
		return err
	}

	outName := fname + ".go"
	outPath := filepath.Join(outDir, outName)

	outf, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outf.Close()

	err = generator.Visit(outf, f)
	if err != nil {
		return fmt.Errorf("generate output: %w", err)
	}

	return nil
}
