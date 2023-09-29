package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pipe01/poodle/internal/workspace"
)

type Watcher struct {
	watchingDirs, watchingFiles map[string]struct{}

	regenFiles []string
	watcher    *fsnotify.Watcher
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

	err := w.watchFile(fullPath)
	if err != nil {
		return err
	}

	w.regenFiles = append(w.regenFiles, fullPath)
	w.RegenFile(fullPath)
	return nil
}

func (w *Watcher) watchFile(path string) error {
	w.watchingFiles[path] = struct{}{}

	dir := filepath.Dir(path)
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
	lastModTime := map[string]time.Time{}

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

			// Prevent duplicate events in quick succession
			if lastTime, ok := lastModTime[event.Name]; ok {
				if time.Now().Sub(lastTime) < 100*time.Millisecond {
					continue
				}
			}
			lastModTime[event.Name] = time.Now()

			// Wait for the file to finish being written to
			time.Sleep(50 * time.Millisecond)

			log.Printf("file %q modified, recompiling...", event.Name)
			start := time.Now()

			for _, f := range w.regenFiles {
				w.RegenFile(f)
			}

			elapsed := time.Now().Sub(start)
			log.Printf("done in %s", elapsed)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func (w *Watcher) RegenFile(fullPath string) {
	name := filepath.Base(fullPath)

	ws := workspace.New(filepath.Dir(name))

	_, err := generateFile(ws, filepath.Base(fullPath), genOpts)
	if err != nil {
		printFileError(err)
	}

	for _, req := range ws.RequestedFiles() {
		reqPath, _ := filepath.Abs(req)

		w.watchFile(reqPath)
	}
}
