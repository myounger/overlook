// Package watch turns filesystem events into debounced "something changed"
// signals.
package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	// Changes receives a value after a burst of events has gone quiet for
	// the debounce period. It never holds more than one pending signal.
	Changes chan struct{}

	fs       *fsnotify.Watcher
	debounce atomic.Int64 // a time.Duration; changes when the config is reloaded
	trees    []string
}

// New watches each dir in dirs (not their subfolders) and each dir in trees
// including all subfolders, picking up subfolders created later.
func New(dirs, trees []string, debounce time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{Changes: make(chan struct{}, 1), fs: fw, trees: trees}
	w.SetDebounce(debounce)
	for _, d := range dirs {
		if err := fw.Add(d); err != nil {
			fw.Close()
			return nil, err
		}
	}
	for _, t := range trees {
		w.addTree(t)
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Close() error { return w.fs.Close() }

// SetDebounce changes how long a burst of events must go quiet before a
// signal is sent.
func (w *Watcher) SetDebounce(d time.Duration) { w.debounce.Store(int64(d)) }

// addTree is best effort: folders that vanish or can't be read are skipped.
func (w *Watcher) addTree(root string) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			w.fs.Add(path)
		}
		return nil
	})
}

func (w *Watcher) inTree(path string) bool {
	for _, t := range w.trees {
		if rel, err := filepath.Rel(t, path); err == nil && filepath.IsLocal(rel) {
			return true
		}
	}
	return false
}

func (w *Watcher) loop() {
	timer := time.NewTimer(0)
	<-timer.C
	for {
		select {
		case ev, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if ev.Has(fsnotify.Create) && w.inTree(ev.Name) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					w.addTree(ev.Name)
				}
			}
			timer.Reset(time.Duration(w.debounce.Load()))
		case _, ok := <-w.fs.Errors:
			if !ok {
				return
			}
		case <-timer.C:
			select {
			case w.Changes <- struct{}{}:
			default:
			}
		}
	}
}
