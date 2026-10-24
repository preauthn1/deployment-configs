package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceSelf(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dbc")
	if err := os.WriteFile(target, []byte("old content"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	newBin := filepath.Join(dir, "new-dbc")
	if err := os.WriteFile(newBin, []byte("new content"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := replaceSelf(target, newBin); err != nil {
		t.Fatalf("replaceSelf: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading replaced target: %v", err)
	}
	if string(got) != "new content" {
		t.Fatalf("target content = %q, want %q", got, "new content")
	}

	// The pre-swap content is left behind under target+".old" — this
	// process (in the real dbc.exe case) is still running from it, so
	// replaceSelf can't delete it; cleanup happens on the next upgrade.
	old, err := os.ReadFile(target + ".old")
	if err != nil {
		t.Fatalf("reading %s: %v", target+".old", err)
	}
	if string(old) != "old content" {
		t.Fatalf(".old content = %q, want %q", old, "old content")
	}

	// No leftover staging file.
	if _, err := os.Stat(target + ".new"); !os.IsNotExist(err) {
		t.Fatalf("expected staged .new file to be gone, stat err = %v", err)
	}
}

func TestReplaceSelfLeavesNoStagingFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dbc")
	if err := os.WriteFile(target, []byte("old content"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := replaceSelf(target, filepath.Join(dir, "does-not-exist")); err == nil {
		t.Fatal("expected error when newBin doesn't exist")
	}

	if _, err := os.Stat(target + ".new"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover .new file, stat err = %v", err)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover .old file since the swap never started, stat err = %v", err)
	}
	// Original target must be untouched.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != "old content" {
		t.Fatalf("target content = %q, want unchanged %q", got, "old content")
	}
}
