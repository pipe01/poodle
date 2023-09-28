package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/pipe01/poodle/internal/generator"
	"github.com/pipe01/poodle/internal/workspace"
)

var (
	outDir      = kingpin.Flag("out-dir", "Folder to put generated files on").Short('o').Default(".").String()
	runImports  = kingpin.Flag("goimports", "Run goimports on each file after it's generated").Default("true").Bool()
	packageName = kingpin.Flag("pkg", "Package name to set on generated files").Default("main").String()
	forceExport = kingpin.Flag("export", "Make the first letter of all template names uppercase").Default("true").Bool()
	watch       = kingpin.Flag("watch", "Watch files for changes and recompile automatically").Short('w').Bool()
	files       = kingpin.Arg("files", "List of files to compile").Required().ExistingFiles()

	genOpts generator.Options
)

func main() {
	kingpin.Parse()

	*outDir, _ = filepath.Abs(*outDir)

	genOpts = generator.Options{
		Package:     *packageName,
		ForceExport: *forceExport,
	}

	if *watch {
		err := watchFiles()
		if err != nil {
			kingpin.Fatalf("failed to watch files: %w", err)
		}
	} else {
		err := generateAll()
		if err != nil {
			kingpin.Fatalf("failed to generate files: %s", err)
		}
	}
}

func generateAll() error {
	wd, _ := os.Getwd()
	ws := workspace.New(wd)

	for _, fname := range *files {
		_, err := generateFile(ws, fname, genOpts)
		if err != nil {
			return fmt.Errorf("load file %q: %s", fname, err)
		}
	}

	return nil
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

	if *runImports {
		cmd := exec.Command("goimports", "-w", outPath)
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return "", fmt.Errorf("run goimports on %q: %s", outPath, err)
		}
	}

	return outPath, nil
}

func watchFiles() error {
	watcher, err := NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	for _, f := range *files {
		err = watcher.WatchFile(f)
		if err != nil {
			return fmt.Errorf("watch file %q: %w", f, err)
		}
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	log.Println("watching files for changes...")

	<-ch
	return nil
}
