package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
)

// newEmbeddedWebProject lays out an embedded Dolt project and puts a bd on
// PATH that answers `bd export` for it.
func newEmbeddedWebProject(t *testing.T) (projectDir, noms string) {
	t.Helper()
	projectDir = t.TempDir()
	beadsDir := filepath.Join(projectDir, ".beads")
	noms = filepath.Join(beadsDir, "embeddeddolt", "emb", ".dolt", "noms")
	if err := os.MkdirAll(noms, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(beadsDir, "metadata.json"), `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb"}`, 0o644)
	write(filepath.Join(noms, "manifest"), "manifest-1", 0o644)
	write(filepath.Join(noms, "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"), "journal", 0o644)

	bin := t.TempDir()
	write(filepath.Join(bin, "bd"), `#!/bin/sh
[ "$1" = "export" ] || exit 2
echo '{"id":"emb-1","title":"Live","status":"open","priority":1,"issue_type":"task"}'
`, 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	return projectDir, noms
}

// TestStore_EmbeddedDoltPollsTheStoreFingerprint pins ADR 0025 for b9s web:
// the files bd rewrites while it only reads must not reload the project, and
// a write must.
func TestStore_EmbeddedDoltPollsTheStoreFingerprint(t *testing.T) {
	projectDir, noms := newEmbeddedWebProject(t)
	store := NewStore(20 * time.Millisecond)
	defer store.Close()

	if failure := store.Open(datasource.OpenTarget{Name: "emb", Dir: projectDir}, "emb"); failure != nil {
		t.Fatalf("Open: %v", failure)
	}
	h := store.Health(true)
	if h.Kind != string(datasource.SourceTypeDoltEmbedded) {
		t.Fatalf("kind = %q, want %q", h.Kind, datasource.SourceTypeDoltEmbedded)
	}
	if !strings.Contains(h.Watching, "polls") {
		t.Errorf("watching = %q, want the poll that ADR 0025 decides", h.Watching)
	}
	if h.Source != "dolt embedded/emb" {
		t.Errorf("source = %q, want dolt embedded/emb", h.Source)
	}

	opened := store.Version()
	if err := os.WriteFile(filepath.Join(noms, "journal.idx"), []byte("rebuilt by a read"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if v := store.Version(); v != opened {
		t.Fatalf("a file bd touches while reading reloaded the project (version %d -> %d)", opened, v)
	}

	if err := os.WriteFile(filepath.Join(noms, "manifest"), []byte("manifest-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for store.Version() == opened {
		if time.Now().After(deadline) {
			t.Fatal("no reload within 3s of a store write")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
