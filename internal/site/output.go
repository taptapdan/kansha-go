package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// markerFile is written into every built site so kansha knows it may safely
// clear the directory on the next build.
const markerFile = ".kansha"

// DirOutput writes files under Dir.
type DirOutput struct {
	Dir string
}

// Prepare empties Dir so files from an earlier build don't linger. To avoid
// deleting someone's work, it refuses to clear a non-empty directory that
// kansha didn't create.
func (o DirOutput) Prepare() error {
	entries, err := os.ReadDir(o.Dir)
	switch {
	case os.IsNotExist(err):
		// Nothing to clear.
	case err != nil:
		return err
	case len(entries) > 0:
		if _, err := os.Stat(filepath.Join(o.Dir, markerFile)); err != nil {
			return fmt.Errorf("%s is not empty and wasn't created by kansha; choose another output folder or delete it first", o.Dir)
		}
		if err := os.RemoveAll(o.Dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(o.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(o.Dir, markerFile), []byte("Built by kansha. This directory is replaced on every build.\n"), 0o644)
}

// Write writes one file, creating parent directories as needed.
func (o DirOutput) Write(name string, data []byte) error {
	p := filepath.Join(o.Dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// MemOutput keeps files in memory. It is safe for concurrent use.
type MemOutput struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemOutput returns an empty MemOutput.
func NewMemOutput() *MemOutput { return &MemOutput{files: map[string][]byte{}} }

// Write stores one file.
func (o *MemOutput) Write(name string, data []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.files[name] = data
	return nil
}

// Get returns a stored file.
func (o *MemOutput) Get(name string) ([]byte, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	b, ok := o.files[name]
	return b, ok
}
