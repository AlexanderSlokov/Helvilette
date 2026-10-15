// Writing the agent's on-disk state so that a crash can never leave half a file.
//
// ADR-0008 D4 requires that `kill -9` or a power cut at any instruction is
// safe. A plain os.WriteFile truncates first and writes second, so dying in
// between leaves an empty or partial file, and the next start reads garbage as
// the last run. Issue #43.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with content, so readers see either the old
// file or the new one, never a mix. The temporary file sits in the same
// directory because rename is only atomic within one filesystem.
//
//	err := writeFileAtomic("/var/lib/helvilette/agent/last_run_summary.json", body, 0644)
func writeFileAtomic(path string, content []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("cannot create a temporary file next to %q: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // no-op once the rename has succeeded

	if err := fillAndSync(tmp, content, perm); err != nil {
		return fmt.Errorf("cannot write %q: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot move the new %q into place: %w", path, err)
	}
	return nil
}

// fillAndSync writes content, sets perm and flushes to disk before closing, so
// the rename that follows never publishes a file whose bytes are still in the
// page cache.
func fillAndSync(f *os.File, content []byte, perm os.FileMode) error {
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		return err
	}
	if err := f.Chmod(perm); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
