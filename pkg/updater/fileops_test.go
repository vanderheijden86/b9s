package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPerformUpdateExecutableLocation(t *testing.T) {
	if expectedPath := os.Getenv("B9S_TEST_UPDATE_EXECUTABLE"); expectedPath != "" {
		resolved, err := GetCurrentBinaryPath()
		if err != nil {
			t.Fatal(err)
		}
		if resolved != expectedPath {
			t.Fatalf("resolved executable = %q, want %q", resolved, expectedPath)
		}
		_, err = PerformUpdate(&Release{TagName: "v99.0.0"}, true)
		if os.Getenv("B9S_TEST_UPDATE_BUNDLE") == "1" {
			if err == nil || !strings.Contains(err.Error(), "DMG") ||
				!strings.Contains(err.Error(), "https://github.com/vanderheijden86/b9s/releases/latest") {
				t.Fatalf("bundled updater must direct users to download the DMG, got %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "no binary available") {
			t.Fatalf("ordinary updater must continue to release asset checks, got %v", err)
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		path    string
		symlink bool
		bundled bool
	}{
		{"bundle", "B9s.app/Contents/Resources/bin/b9s", false, true},
		{"linked_bundle", "B9s.app/Contents/Resources/bin/b9s", true, true},
		{"uppercase_bundle", "B9s.APP/Contents/Resources/bin/b9s", false, true},
		{"ordinary", "bin/b9s", false, false},
		{"linked_ordinary", "bin/b9s", true, false},
		{"app_substring", "B9s.app-cache/bin/b9s", false, false},
		{"executable_suffix", "bin/b9s.app", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, filepath.FromSlash(tt.path))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := copyFile(executable, target); err != nil {
				t.Fatal(err)
			}
			expectedPath, err := filepath.EvalSymlinks(target)
			if err != nil {
				t.Fatal(err)
			}
			invokedPath := target
			if tt.symlink {
				invokedPath = filepath.Join(dir, "usr", "local", "bin", "b9s")
				if err := os.MkdirAll(filepath.Dir(invokedPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, invokedPath); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, invokedPath, "-test.run=^TestPerformUpdateExecutableLocation$", "-test.timeout=8s")
			cmd.Env = append(os.Environ(), "B9S_TEST_UPDATE_EXECUTABLE="+expectedPath, "B9S_TEST_UPDATE_BUNDLE=0")
			if tt.bundled {
				cmd.Env[len(cmd.Env)-1] = "B9S_TEST_UPDATE_BUNDLE=1"
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("updater subprocess: %v\n%s", err, output)
			}
			entries, err := os.ReadDir(filepath.Dir(target))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != filepath.Base(target) {
				t.Fatalf("updater changed executable directory: %v", entries)
			}
		})
	}
}

func TestCopyFile_PreservesContentAndMode(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.bin")
	dst := filepath.Join(tmpDir, "dst.bin")

	data := []byte("copy me")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.Chmod(src, 0o755); err != nil {
		t.Fatalf("chmod src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: got %q want %q", got, data)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat src: %v", err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if dstInfo.Mode() != srcInfo.Mode() {
		t.Fatalf("mode mismatch: dst=%v src=%v", dstInfo.Mode(), srcInfo.Mode())
	}
}

func TestExtractBinary_FromArchive(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "b9s.tar.gz")
	destPath := filepath.Join(tmpDir, "b9s")

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	payload := []byte("fake-binary")
	hdr := &tar.Header{
		Name: "b9s",
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("write tar payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if err := extractBinary(archivePath, destPath); err != nil {
		t.Fatalf("extractBinary failed: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
}

func TestRollback_NoBackup(t *testing.T) {
	err := Rollback()
	if err == nil {
		t.Fatalf("expected rollback to fail when no backup exists")
	}
	if !strings.Contains(err.Error(), "no backup found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
