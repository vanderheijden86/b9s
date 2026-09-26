package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// webSourceHash mirrors sourceHash in web/build.mjs: every file under
// web/src and web/public plus build.mjs and package-lock.json, sorted by
// slash path, each hashed as "path\nlength\n" followed by its bytes.
func webSourceHash(t *testing.T, dir string) string {
	t.Helper()
	var rel []string
	for _, sub := range []string{"src", "public"} {
		err := filepath.WalkDir(filepath.Join(dir, sub), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			r, err := filepath.Rel(dir, p)
			rel = append(rel, filepath.ToSlash(r))
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	rel = append(rel, "build.mjs", "package-lock.json")
	sort.Strings(rel)
	h := sha256.New()
	for _, r := range rel {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r)))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\n%d\n", r, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The bundle is committed so `go build` needs no Node toolchain. This test
// is what keeps it honest: a change under web/ without `make web` fails here.
func TestEmbeddedBundleIsCurrent(t *testing.T) {
	want := webSourceHash(t, filepath.Join("..", "..", "web"))
	got, err := os.ReadFile(filepath.Join("dist", "source.sha256"))
	if err != nil {
		t.Fatalf("dist/source.sha256: %v (run make web)", err)
	}
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("pkg/web/dist was built from other sources than web/ holds now: run make web and commit pkg/web/dist")
	}
}
