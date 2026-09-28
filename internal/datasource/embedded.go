package datasource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/vanderheijden86/b9s/internal/bdrun"
	"github.com/vanderheijden86/b9s/pkg/loader"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// ErrBDNotFound is returned when an embedded Dolt project cannot be read
// because bd is not on PATH. b9s never opens an embedded store itself (ADR
// 0025), so without bd the project has no readable source.
var ErrBDNotFound = errors.New("bd is not installed or not on PATH")

// embeddedExportTimeout bounds one `bd export`. It is generous because the
// store may be briefly locked by a user's own bd write, which bd waits out.
const embeddedExportTimeout = 30 * time.Second

// EmbeddedBeadsDir returns the .beads directory of an embedded Dolt source,
// whose Path is the store at .beads/embeddeddolt/<database>.
func EmbeddedBeadsDir(source DataSource) string {
	return filepath.Dir(filepath.Dir(source.Path))
}

// loadEmbeddedIssues reads an embedded Dolt project through `bd export`.
// Only stdout is parsed: bd writes hints and warnings to stderr.
func loadEmbeddedIssues(source DataSource) ([]model.Issue, error) {
	bd, ok := bdrun.Resolve()
	if !ok {
		return nil, ErrBDNotFound
	}
	beadsDir := EmbeddedBeadsDir(source)
	ctx, cancel := context.WithTimeout(context.Background(), embeddedExportTimeout)
	defer cancel()
	out, stderr, err := bdrun.Output(ctx, bd, filepath.Dir(beadsDir), []string{"BEADS_DIR=" + beadsDir}, "export")
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("bd export in %s: %w: %s", beadsDir, err, stderr)
		}
		return nil, fmt.Errorf("bd export in %s: %w", beadsDir, err)
	}
	return loader.ParseIssues(bytes.NewReader(out))
}

// doltJournalFile is the name Dolt gives the chunk journal, the file every
// write appends to.
const doltJournalFile = "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"

// embeddedFingerprint summarises the state of an embedded store without
// opening it: the content of each manifest and the size of the journal. A bd
// write changes at least one of them. Every other file is left out, because
// a read touches them too: bd rebuilds journal.idx, takes LOCK and writes
// temporary nbs_manifest_* files while it reads, and a fingerprint that saw
// those would turn each reload by b9s into the trigger for the next one.
// Table files are immutable and listed in the manifest, so they need no
// entry of their own.
func embeddedFingerprint(storeDir string) (string, error) {
	noms := filepath.Join(storeDir, ".dolt", "noms")
	h := sha256.New()
	err := filepath.WalkDir(noms, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(noms, path)
		switch d.Name() {
		case "manifest":
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%x\x00", rel, content)
		case doltJournalFile:
			info, err := d.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%d\x00", rel, info.Size())
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint of %s: %w", storeDir, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
