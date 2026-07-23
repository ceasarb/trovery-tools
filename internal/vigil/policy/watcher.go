package policy

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
)

// Default directories to ignore during file watching.
var defaultIgnoreDirs = []string{
	".git",
	".demi/vigil",
	"node_modules",
	"__pycache__",
	".next",
	"dist",
	"build",
	".cache",
}

const debounceDuration = 100 * time.Millisecond

// FileWatcher monitors file changes in real-time using fsnotify.
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	scanner     *SecretsScanner
	projectRoot string

	mu         sync.Mutex
	changes    map[string]session.FileChange // deduped by path
	violations []session.PolicyViolation

	debounce map[string]*time.Timer
	done     chan struct{}
}

// NewFileWatcher creates a watcher that monitors the project root for changes.
func NewFileWatcher(projectRoot string, scanner *SecretsScanner) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		watcher:     w,
		scanner:     scanner,
		projectRoot: projectRoot,
		changes:     make(map[string]session.FileChange),
		debounce:    make(map[string]*time.Timer),
		done:        make(chan struct{}),
	}, nil
}

// Start begins watching for file changes. Call Stop() to end.
func (fw *FileWatcher) Start() error {
	if err := fw.addRecursive(fw.projectRoot); err != nil {
		return err
	}

	go fw.eventLoop()
	return nil
}

// Stop ends the watcher and returns accumulated changes and violations.
func (fw *FileWatcher) Stop() ([]session.FileChange, []session.PolicyViolation) {
	close(fw.done)
	fw.watcher.Close()

	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Flush pending debounce timers.
	for _, t := range fw.debounce {
		t.Stop()
	}

	changes := make([]session.FileChange, 0, len(fw.changes))
	for _, c := range fw.changes {
		changes = append(changes, c)
	}

	return changes, fw.violations
}

func (fw *FileWatcher) eventLoop() {
	for {
		select {
		case <-fw.done:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("file watcher error", "error", err)
		}
	}
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// Ignore events in excluded directories.
	if fw.shouldIgnore(path) {
		return
	}

	// Only care about writes, creates, and removes.
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) {
		return
	}

	// If a new directory is created, watch it too.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			fw.addRecursive(path)
			return
		}
	}

	// Debounce per file.
	fw.mu.Lock()
	if timer, exists := fw.debounce[path]; exists {
		timer.Stop()
	}
	fw.debounce[path] = time.AfterFunc(debounceDuration, func() {
		fw.processFile(path, event)
	})
	fw.mu.Unlock()
}

func (fw *FileWatcher) processFile(absPath string, event fsnotify.Event) {
	relPath, err := filepath.Rel(fw.projectRoot, absPath)
	if err != nil {
		relPath = absPath
	}

	changeType := "modified"
	if event.Has(fsnotify.Create) {
		changeType = "added"
	} else if event.Has(fsnotify.Remove) {
		changeType = "deleted"
	}

	fw.mu.Lock()
	fw.changes[relPath] = session.FileChange{
		Path:       relPath,
		ChangeType: changeType,
	}
	fw.mu.Unlock()

	// Scan for secrets on non-deleted files.
	if changeType != "deleted" && fw.scanner != nil {
		violations := fw.scanner.scanFile(absPath, relPath)
		if len(violations) > 0 {
			fw.mu.Lock()
			fw.violations = append(fw.violations, violations...)
			fw.mu.Unlock()

			for _, v := range violations {
				console.Warning("Real-time: " + v.Message)
			}
		}
	}
}

func (fw *FileWatcher) shouldIgnore(path string) bool {
	rel, err := filepath.Rel(fw.projectRoot, path)
	if err != nil {
		return false
	}

	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		for _, ignore := range defaultIgnoreDirs {
			if part == ignore {
				return true
			}
		}
	}
	return false
}

func (fw *FileWatcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths.
		}
		if !info.IsDir() {
			return nil
		}
		if fw.shouldIgnore(path) {
			return filepath.SkipDir
		}
		return fw.watcher.Add(path)
	})
}
