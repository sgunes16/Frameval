package sandbox

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestTarRoundTripPreservesExecutableBit guards the regression where
// extractTarToDir wrote every file with os.Create (0644), silently stripping
// the executable bit off scripts a container hands back (e.g. spec-kit's
// .specify/scripts/*.sh). After that, `./script.sh` in the next phase failed
// with "Permission denied" (exit 126) and the harness ran degraded.
func TestTarRoundTripPreservesExecutableBit(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "data.txt"), []byte("plain\n"), 0o644); err != nil {
		t.Fatalf("write regular: %v", err)
	}

	archive, err := tarDirectory(src)
	if err != nil {
		t.Fatalf("tarDirectory: %v", err)
	}

	dst := t.TempDir()
	if err := extractTarToDir(bytes.NewReader(archive), dst, ""); err != nil {
		t.Fatalf("extractTarToDir: %v", err)
	}

	exec, err := os.Stat(filepath.Join(dst, "run.sh"))
	if err != nil {
		t.Fatalf("stat run.sh: %v", err)
	}
	if exec.Mode().Perm()&0o111 == 0 {
		t.Errorf("run.sh lost its executable bit: got %v, want an +x mode", exec.Mode().Perm())
	}

	plain, err := os.Stat(filepath.Join(dst, "data.txt"))
	if err != nil {
		t.Fatalf("stat data.txt: %v", err)
	}
	if plain.Mode().Perm()&0o111 != 0 {
		t.Errorf("data.txt should not be executable: got %v", plain.Mode().Perm())
	}
}
