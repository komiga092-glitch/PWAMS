package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveFileAfterSoftDelete(t *testing.T) {
	t.Run("removes existing file without restoring metadata", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}

		restored := false
		if err := removeFileAfterSoftDelete(path, func() error {
			restored = true
			return nil
		}); err != nil {
			t.Fatalf("removeFileAfterSoftDelete() error = %v", err)
		}

		if restored {
			t.Fatal("metadata restore was called after successful file removal")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("file still exists or stat failed: %v", err)
		}
	})

	t.Run("treats missing file as already removed", func(t *testing.T) {
		restored := false
		path := filepath.Join(t.TempDir(), "missing.txt")
		if err := removeFileAfterSoftDelete(path, func() error {
			restored = true
			return nil
		}); err != nil {
			t.Fatalf("removeFileAfterSoftDelete() error = %v", err)
		}
		if restored {
			t.Fatal("metadata restore was called for a missing file")
		}
	})

	t.Run("restores metadata when file removal fails", func(t *testing.T) {
		restored := false
		path := filepath.Join(t.TempDir(), "non-empty")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child.txt"), []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
		err := removeFileAfterSoftDelete(path, func() error {
			restored = true
			return nil
		})
		if err == nil {
			t.Fatal("removeFileAfterSoftDelete() error = nil, want filesystem error")
		}
		if !restored {
			t.Fatal("metadata restore was not called")
		}
	})

	t.Run("reports metadata restoration failure", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "non-empty")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child.txt"), []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
		restoreErr := errors.New("restore failed")
		err := removeFileAfterSoftDelete(path, func() error {
			return restoreErr
		})
		if err == nil {
			t.Fatal("removeFileAfterSoftDelete() error = nil, want combined failure")
		}
		if !errors.Is(err, restoreErr) {
			t.Fatalf("removeFileAfterSoftDelete() error = %v, want restore error", err)
		}
	})
}
