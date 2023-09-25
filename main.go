package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/pipe01/poodle/internal/generator"
	"github.com/pipe01/poodle/internal/workspace"
)

var (
	outDir      = kingpin.Flag("out-dir", "Folder to put generated files on").Default(".").String()
	runImports  = kingpin.Flag("goimports", "Run goimports on each file after it's generated").Default("true").Bool()
	packageName = kingpin.Flag("pkg", "Package name to set on generated files").Default("main").String()
	forceExport = kingpin.Flag("export", "Make the first letter of all template names uppercase").Default("true").Bool()
	files       = kingpin.Arg("files", "List of files to compile").Required().ExistingFiles()
)

func main() {
	kingpin.Parse()

	wd, _ := os.Getwd()
	ws := workspace.New(wd)

	if *outDir == "" {
		*outDir = wd
	}

	genOpts := generator.Options{
		Package:     *packageName,
		ForceExport: *forceExport,
	}

	for _, fname := range *files {
		outPath, err := generateFile(ws, fname, genOpts)
		if err != nil {
			kingpin.Fatalf("failed to load file %q: %s", fname, err)
		}

		if *runImports {
			cmd := exec.Command("goimports", "-w", outPath)
			if err = cmd.Run(); err != nil {
				kingpin.Fatalf("failed to run goimports on %q: %s", outPath, err)
			}
		}
	}
}

func generateFile(ws *workspace.Workspace, fname string, genOpts generator.Options) (outPath string, err error) {
	f, err := ws.Load(fname)
	if err != nil {
		return "", err
	}

	outName := fname + ".go"
	outPath = filepath.Join(*outDir, outName)

	outf, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	defer outf.Close()

	err = generator.Visit(outf, f, genOpts)
	if err != nil {
		return "", fmt.Errorf("generate output: %w", err)
	}

	return outPath, nil
}
