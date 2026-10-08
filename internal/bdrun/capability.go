package bdrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanderheijden86/b9s/pkg/debug"
)

// MemoryDialect names the Memory Beads read format a bd binary speaks.
// Released bd has none; the version string cannot tell them apart, because
// preview builds report the same semver as a release (ADR 0044, plan
// docs/plans/2026-10-07-memory-capability-gating.md).
type MemoryDialect string

const (
	MemoryDialectNone MemoryDialect = "none"
	// MemoryDialectPreviewV2 is the graph preview build: records-json
	// summaries and the /preview-memory-v2 and /preview-issue-v2 Types.
	MemoryDialectPreviewV2 MemoryDialect = "preview-v2"
)

// probeTimeout bounds each help call. A preview bd answers in about 0.5s;
// one that takes longer than this is treated as not offering Memory.
const probeTimeout = 2 * time.Second

// ProbeMemory reports which Memory dialect bdPath speaks, judged only from
// its help output so no database is ever opened.
func ProbeMemory(ctx context.Context, bdPath string) MemoryDialect {
	dialect, _ := probeMemory(ctx, bdPath)
	return dialect
}

// probeMemory also reports whether the answer is conclusive: a timed-out or
// cancelled call says nothing about the binary and must not be cached.
//
// The preview needs all three signals. versions and links alone are what
// upstream Beads History will ship, which has no Memory records; only the
// records-json Memory summaries mark the preview read format b9s parses.
func probeMemory(ctx context.Context, bdPath string) (MemoryDialect, bool) {
	dir, err := os.MkdirTemp("", "b9s-bd-probe-")
	if err != nil {
		debug.Log("bdrun: probe temp dir: %v", err)
		return MemoryDialectNone, false
	}
	defer os.RemoveAll(dir)

	for _, args := range [][]string{{"versions", "--help"}, {"links", "--help"}} {
		if _, ok, conclusive := probeHelp(ctx, bdPath, dir, args); !ok {
			return MemoryDialectNone, conclusive
		}
	}
	out, ok, conclusive := probeHelp(ctx, bdPath, dir, []string{"memories", "--help"})
	if !ok {
		return MemoryDialectNone, conclusive
	}
	if !strings.Contains(out, "records-json") {
		return MemoryDialectNone, true
	}
	return MemoryDialectPreviewV2, true
}

// probeHelp runs one help call in the empty dir with the workspace selectors
// stripped from the environment, so bd cannot find a database to open.
func probeHelp(ctx context.Context, bdPath, dir string, args []string) (stdout string, ok, conclusive bool) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := command(ctx, bdPath, dir, nil, args)
	cmd.Env = withoutWorkspaceEnv(os.Environ())
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	start := time.Now()
	err := classify(ctx, cmd, cmd.Run(), time.Since(start), dir, args)
	if errors.Is(err, ErrTimeout) || errors.Is(err, ErrCancelled) {
		debug.Log("bdrun: Memory probe %v inconclusive: %v", args, err)
		return "", false, false
	}
	return outBuf.String(), err == nil, true
}

func withoutWorkspaceEnv(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "BEADS_DIR=") || strings.HasPrefix(kv, "BEADS_DB=") {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}

// probeCache maps a resolved bd path to the dialect probed for the file as
// it was then. Size and modification time stand in for the binary's
// identity, so a rebuilt or replaced bd is probed again.
type probeCache struct {
	Binaries map[string]probeEntry `json:"binaries"`
}

type probeEntry struct {
	Size    int64         `json:"size"`
	ModTime int64         `json:"mod_time_ns"`
	Memory  MemoryDialect `json:"memory"`
}

// DefaultProbeCacheFile is where b9s keeps probe results: under
// XDG_CACHE_HOME when it is set, else the platform's user cache directory.
// An empty result means no cache directory is known, and every start probes.
func DefaultProbeCacheFile() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		var err error
		if dir, err = os.UserCacheDir(); err != nil {
			return ""
		}
	}
	return filepath.Join(dir, "b9s", "bd-capabilities.json")
}

// ProbeMemoryCached is ProbeMemory with the answer kept in cacheFile, so a
// normal start costs one stat of bdPath. hit reports whether the cache
// answered. A cache that cannot be read or written only costs a fresh probe.
func ProbeMemoryCached(ctx context.Context, bdPath, cacheFile string) (dialect MemoryDialect, hit bool) {
	info, err := os.Stat(bdPath)
	if err != nil || cacheFile == "" {
		return ProbeMemory(ctx, bdPath), false
	}
	key, err := filepath.Abs(bdPath)
	if err != nil {
		key = bdPath
	}

	cache := readProbeCache(cacheFile)
	if e, ok := cache.Binaries[key]; ok && e.Size == info.Size() && e.ModTime == info.ModTime().UnixNano() {
		return e.Memory, true
	}

	dialect, conclusive := probeMemory(ctx, bdPath)
	if conclusive {
		cache.Binaries[key] = probeEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano(), Memory: dialect}
		if err := writeProbeCache(cacheFile, cache); err != nil {
			debug.Log("bdrun: writing probe cache %s: %v", cacheFile, err)
		}
	}
	return dialect, false
}

// ForgetProbe drops the cached answer for bdPath, so the next
// ProbeMemoryCached asks the binary again. It is the escape hatch for a bd
// replaced in place without its size or modification time changing.
func ForgetProbe(bdPath, cacheFile string) error {
	if cacheFile == "" {
		return nil
	}
	key, err := filepath.Abs(bdPath)
	if err != nil {
		key = bdPath
	}
	cache := readProbeCache(cacheFile)
	if _, ok := cache.Binaries[key]; !ok {
		return nil
	}
	delete(cache.Binaries, key)
	return writeProbeCache(cacheFile, cache)
}

func readProbeCache(path string) probeCache {
	cache := probeCache{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cache); err != nil {
			debug.Log("bdrun: ignoring unreadable probe cache %s: %v", path, err)
			cache = probeCache{}
		}
	}
	if cache.Binaries == nil {
		cache.Binaries = map[string]probeEntry{}
	}
	return cache
}

// writeProbeCache replaces the file by rename, so two b9s processes probing
// at once each leave a whole file rather than an interleaved one.
func writeProbeCache(path string, cache probeCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bd-capabilities-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
