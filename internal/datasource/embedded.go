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
	"sync"
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

// graphInventoryTimeout bounds the inventory read that supplies the Issues of
// a Memory preview workspace.
const graphInventoryTimeout = 2 * time.Minute

// memoryGraphs keeps, per graph workspace keyed by its .beads directory,
// whether the last load found a Memory workspace and the last graph read on
// request. Loading Issues reads only the inventory: the graph needs one
// traversal per Issue component, seconds on a real workspace, and only the
// Memory views and decision columns use it (ADR 0051).
var memoryGraphs = struct {
	sync.RWMutex
	byDir      map[string]MemoryGraph
	workspaces map[string]bool
}{byDir: map[string]MemoryGraph{}, workspaces: map[string]bool{}}

// MemoryGraphFor returns the graph last stored for the graph workspace at
// beadsDir. It reports false until a view has requested and read one.
func MemoryGraphFor(beadsDir string) (MemoryGraph, bool) {
	memoryGraphs.RLock()
	defer memoryGraphs.RUnlock()
	g, ok := memoryGraphs.byDir[beadsDir]
	return g, ok
}

// MemoryWorkspaceFor reports whether the last load of beadsDir found a graph
// workspace with Memory views turned on, so its graph can be requested.
func MemoryWorkspaceFor(beadsDir string) bool {
	memoryGraphs.RLock()
	defer memoryGraphs.RUnlock()
	return memoryGraphs.workspaces[beadsDir]
}

// StoreMemoryGraph keeps a graph read on request for the views of beadsDir.
// With Memory views turned off nothing is kept.
func StoreMemoryGraph(beadsDir string, graph MemoryGraph) {
	memoryGraphs.Lock()
	defer memoryGraphs.Unlock()
	if memoryTurnedOff() {
		delete(memoryGraphs.byDir, beadsDir)
		return
	}
	memoryGraphs.byDir[beadsDir] = graph
}

// loadEmbeddedIssues reads an embedded Dolt project through `bd export`, or
// through the graph inventory when the workspace and bd can read one, since
// a graph workspace refuses export. The inventory carries every Issue and
// the blocking Links the Issues own; the Memory Links wait for a view to
// request the graph. Without graph support the export runs, so bd itself
// names what it cannot read. Only stdout is parsed: bd writes hints and
// warnings to stderr.
func loadEmbeddedIssues(source DataSource) ([]model.Issue, error) {
	bd, ok := bdrun.Resolve()
	if !ok {
		return nil, ErrBDNotFound
	}
	beadsDir := EmbeddedBeadsDir(source)
	project := filepath.Dir(beadsDir)
	if capability := detectGraphReadable(context.Background(), project); capability.State == MemoryReady {
		client, err := newGraphPreviewClient(project, capability.BD)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), graphInventoryTimeout)
		defer cancel()
		inv, err := client.Inventory(ctx)
		if err != nil {
			return nil, fmt.Errorf("read graph workspace %s: %w", beadsDir, err)
		}
		memoryGraphs.Lock()
		if memoryTurnedOff() {
			delete(memoryGraphs.byDir, beadsDir)
			delete(memoryGraphs.workspaces, beadsDir)
		} else {
			memoryGraphs.workspaces[beadsDir] = true
		}
		memoryGraphs.Unlock()
		return buildMemoryGraph(inv.Beads, inv.Links).Issues(), nil
	}
	memoryGraphs.Lock()
	delete(memoryGraphs.byDir, beadsDir)
	delete(memoryGraphs.workspaces, beadsDir)
	memoryGraphs.Unlock()
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
