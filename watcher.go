package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/pipe01/poodle/internal/workspace"
)

type Watcher struct {
	watchingDirs, watchingFiles map[string]struct{}

	watcher *fsnotify.Watcher
}

func NewWatcher() (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		watchingDirs:  make(map[string]struct{}),
		watchingFiles: make(map[string]struct{}),
		watcher:       watcher,
	}
	go w.eventLoop()

	return w, nil
}

func (w *Watcher) WatchFile(path string) error {
	fullPath, _ := filepath.Abs(path)
	w.watchingFiles[fullPath] = struct{}{}

	dir := filepath.Dir(fullPath)
	if _, ok := w.watchingDirs[dir]; ok {
		return nil
	}

	err := w.watcher.Add(dir)
	if err != nil {
		return err
	}

	w.watchingDirs[dir] = struct{}{}

	return nil
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if !event.Has(fsnotify.Write) {
				continue
			}

			fname, _ := filepath.Abs(event.Name)

			if _, ok := w.watchingFiles[fname]; !ok {
				continue
			}

			w.fileModified(fname)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func (w *Watcher) fileModified(fullPath string) {
	name := filepath.Base(fullPath)

	log.Printf("file %q modified, recompiling...", name)

	ws := workspace.New(filepath.Dir(name))

	_, err := generateFile(ws, filepath.Base(fullPath), genOpts)
	if err != nil {
		log.Printf("failed to generate file %q: %s", fullPath, err)
	}

	for _, req := range ws.RequestedFiles() {
		w.WatchFile(req)
	}
}
