// Package atomicfile writes complete files through a staged sibling and then
// replaces the destination. Callers only provide the content writer; staging,
// flush-to-disk, permissions, cleanup, and platform replacement live here.
package atomicfile

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

type Options struct {
	Pattern string
	Mode    fs.FileMode
}

// Write creates a temporary sibling, invokes write, syncs and closes the staged
// file, and replaces path only after every prior step succeeds.
func Write(path string, options Options, write func(io.Writer) error) (resultErr error) {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pattern := options.Pattern
	if pattern == "" {
		pattern = ".ayame-diff-*.tmp"
	}
	mode := options.Mode
	if mode == 0 {
		mode = 0o644
	}
	temp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	open := true
	defer func() {
		if open {
			if err := temp.Close(); resultErr == nil && err != nil {
				resultErr = err
			}
		}
		if resultErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := write(temp); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	open = false
	if err := os.Chmod(tempPath, mode); err != nil {
		return err
	}
	if err := Replace(tempPath, path); err != nil {
		return err
	}
	return nil
}

// Replace moves a fully written staged file into place. Unix rename replaces
// atomically. Windows requires a remove-then-rename fallback when a destination
// already exists.
func Replace(staged, destination string) error {
	if err := os.Rename(staged, destination); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(staged, destination)
}
