package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the filesystem for changes.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	root      string
	onUpdate  func(path string)
	onDelete  func(path string)
}

// NewWatcher creates a new watcher.
func NewWatcher(root string, onUpdate, onDelete func(path string)) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsWatcher: fsWatcher,
		root:      root,
		onUpdate:  onUpdate,
		onDelete:  onDelete,
	}, nil
}

// Close closes the watcher.
func (w *Watcher) Close() error {
	return w.fsWatcher.Close()
}

// Start begins watching the directory tree.
func (w *Watcher) Start() error {
	// Add all directories to watcher
	err := filepath.WalkDir(w.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	go w.loop()
	return nil
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			
			// Handle Directory Create (add to watcher)
			if event.Has(fsnotify.Create) {
				fi, err := os.Stat(event.Name)
				if err == nil && fi.IsDir() {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						w.fsWatcher.Add(event.Name)
					}
				}
			}

			// Filter for .md files
			if filepath.Ext(event.Name) != ".md" {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				w.onUpdate(event.Name)
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				w.onDelete(event.Name)
				// Note: Rename is often Rename (old) then Create (new) or just Rename.
				// fsnotify behavior varies by platform. 
				// If it's a rename, we might want to handle the "new" name if it's provided in a separate event?
				// Usually Rename event gives the OLD name. The NEW name comes as a Create event?
				// If so, onDelete(old) then onUpdate(new) covers it.
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Println("watcher error:", err)
		}
	}
}
