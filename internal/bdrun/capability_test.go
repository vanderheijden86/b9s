package bdrun

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stockBdScript answers like a released bd: it has no versions or links
// command, and its memories help lists only the key/value memories.
const stockBdScript = `#!/bin/sh
case "$1" in
versions|links) echo "Error: unknown command \"$1\" for \"bd\"" >&2; exit 1 ;;
memories) echo "List all memories, or search by keyword."; exit 0 ;;
esac
exit 0
`

// previewBdScript answers like the Memory Beads preview build.
const previewBdScript = `#!/bin/sh
case "$1" in
versions) echo "Use bd versions ID to list one Memory's retained versions"; exit 0 ;;
links) echo "List current informational Links"; exit 0 ;;
memories) echo "      --format string   Graph Memory summaries: table or records-json (preview only)"; exit 0 ;;
esac
exit 1
`

// historyOnlyBdScript has retained versions and Links but no Memory records,
// the shape upstream may ship when Beads History lands before Memory Beads.
const historyOnlyBdScript = `#!/bin/sh
case "$1" in
versions|links) exit 0 ;;
memories) echo "List all memories, or search by keyword."; exit 0 ;;
esac
exit 1
`

func installFakeBd(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "bd")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProbeMemory_StockBdHasNoDialect(t *testing.T) {
	bd := installFakeBd(t, stockBdScript)

	got := ProbeMemory(context.Background(), bd)

	if got != MemoryDialectNone {
		t.Errorf("ProbeMemory = %q, want %q", got, MemoryDialectNone)
	}
}

func TestProbeMemory_PreviewBdHasPreviewDialect(t *testing.T) {
	bd := installFakeBd(t, previewBdScript)

	got := ProbeMemory(context.Background(), bd)

	if got != MemoryDialectPreviewV2 {
		t.Errorf("ProbeMemory = %q, want %q", got, MemoryDialectPreviewV2)
	}
}

func TestProbeMemory_HistoryWithoutMemoryRecordsHasNoDialect(t *testing.T) {
	bd := installFakeBd(t, historyOnlyBdScript)

	got := ProbeMemory(context.Background(), bd)

	if got != MemoryDialectNone {
		t.Errorf("ProbeMemory = %q, want %q: versions and links alone are not Memory support", got, MemoryDialectNone)
	}
}

func TestProbeMemory_HungBdHasNoDialectWithinBound(t *testing.T) {
	bd := installFakeBd(t, "#!/bin/sh\nsleep 30\n")

	start := time.Now()
	got := ProbeMemory(context.Background(), bd)
	elapsed := time.Since(start)

	if got != MemoryDialectNone {
		t.Errorf("ProbeMemory = %q, want %q", got, MemoryDialectNone)
	}
	if elapsed > 3*probeTimeout+3*waitDelay {
		t.Fatalf("ProbeMemory took %v, want it bounded by the per-call probe timeout", elapsed)
	}
}

// A probe must never reach a workspace database: a stray BEADS_DIR or a
// checkout in the working directory could make bd open, lock or migrate one.
func TestProbeMemory_RunsOutsideAnyWorkspace(t *testing.T) {
	t.Setenv("BEADS_DIR", t.TempDir())
	t.Setenv("BEADS_DB", filepath.Join(t.TempDir(), "beads.db"))
	guarded := strings.Replace(previewBdScript, "#!/bin/sh\n",
		"#!/bin/sh\n[ -n \"$BEADS_DIR$BEADS_DB\" ] && exit 1\n[ -n \"$(ls -A)\" ] && exit 1\n", 1)
	bd := installFakeBd(t, guarded)

	got := ProbeMemory(context.Background(), bd)

	if got != MemoryDialectPreviewV2 {
		t.Errorf("ProbeMemory = %q, want %q: the probe leaked workspace env or ran in a non-empty dir", got, MemoryDialectPreviewV2)
	}
}

// countingScript wraps script so every invocation appends a line to counter.
func countingScript(script, counter string) string {
	return strings.Replace(script, "#!/bin/sh\n", "#!/bin/sh\necho x >> '"+counter+"'\n", 1)
}

func invocations(t *testing.T, counter string) int {
	t.Helper()
	data, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

func TestProbeMemoryCached_SecondCallReusesResult(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "calls")
	bd := installFakeBd(t, countingScript(previewBdScript, counter))
	cache := filepath.Join(t.TempDir(), "bd-capabilities.json")

	first, hit1 := ProbeMemoryCached(context.Background(), bd, cache)
	callsAfterFirst := invocations(t, counter)
	second, hit2 := ProbeMemoryCached(context.Background(), bd, cache)

	if first != MemoryDialectPreviewV2 || second != MemoryDialectPreviewV2 {
		t.Fatalf("dialects = %q, %q, want both %q", first, second, MemoryDialectPreviewV2)
	}
	if hit1 || !hit2 {
		t.Errorf("cache hits = %v, %v, want false, true", hit1, hit2)
	}
	if got := invocations(t, counter); got != callsAfterFirst {
		t.Errorf("bd ran %d more times on the cached call, want 0", got-callsAfterFirst)
	}
}

func TestProbeMemoryCached_ReplacedBinaryIsProbedAgain(t *testing.T) {
	bd := installFakeBd(t, stockBdScript)
	cache := filepath.Join(t.TempDir(), "bd-capabilities.json")
	if got, _ := ProbeMemoryCached(context.Background(), bd, cache); got != MemoryDialectNone {
		t.Fatalf("stock probe = %q, want none", got)
	}

	if err := os.WriteFile(bd, []byte(previewBdScript), 0o755); err != nil {
		t.Fatal(err)
	}
	got, hit := ProbeMemoryCached(context.Background(), bd, cache)

	if hit {
		t.Error("cache hit after the binary changed, want a fresh probe")
	}
	if got != MemoryDialectPreviewV2 {
		t.Errorf("ProbeMemoryCached = %q, want %q", got, MemoryDialectPreviewV2)
	}
}

func TestProbeMemoryCached_CorruptCacheIsProbedAgain(t *testing.T) {
	bd := installFakeBd(t, previewBdScript)
	cache := filepath.Join(t.TempDir(), "bd-capabilities.json")
	if err := os.WriteFile(cache, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, hit := ProbeMemoryCached(context.Background(), bd, cache)

	if hit || got != MemoryDialectPreviewV2 {
		t.Errorf("ProbeMemoryCached = %q (hit %v), want %q from a fresh probe", got, hit, MemoryDialectPreviewV2)
	}
}

// A hung probe is not knowledge about the binary: caching it would hide
// Memory support for as long as that bd stays installed.
func TestProbeMemoryCached_TimeoutIsNotCached(t *testing.T) {
	bd := installFakeBd(t, "#!/bin/sh\nsleep 30\n")
	cache := filepath.Join(t.TempDir(), "bd-capabilities.json")

	ProbeMemoryCached(context.Background(), bd, cache)

	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("cache file written after a timed-out probe (stat err %v), want none", err)
	}
}

// TestProbeMemory_RealBinaries checks the probe against real bd builds. It is
// opt-in because neither binary is present on CI: set B9S_TEST_STOCK_BD to a
// released bd and B9S_TEST_PREVIEW_BD to a Memory preview bd.
func TestProbeMemory_RealBinaries(t *testing.T) {
	cases := []struct {
		env  string
		want MemoryDialect
	}{
		{"B9S_TEST_STOCK_BD", MemoryDialectNone},
		{"B9S_TEST_PREVIEW_BD", MemoryDialectPreviewV2},
	}
	ran := false
	for _, c := range cases {
		bd := os.Getenv(c.env)
		if bd == "" {
			continue
		}
		ran = true
		if got := ProbeMemory(context.Background(), bd); got != c.want {
			t.Errorf("%s=%s: ProbeMemory = %q, want %q", c.env, bd, got, c.want)
		}
	}
	if !ran {
		t.Skip("set B9S_TEST_STOCK_BD and/or B9S_TEST_PREVIEW_BD to probe real binaries")
	}
}

func TestForgetProbe_NextCallProbesAgain(t *testing.T) {
	bd := installFakeBd(t, previewBdScript)
	cache := filepath.Join(t.TempDir(), "bd-capabilities.json")
	ProbeMemoryCached(context.Background(), bd, cache)

	if err := ForgetProbe(bd, cache); err != nil {
		t.Fatalf("ForgetProbe: %v", err)
	}
	got, hit := ProbeMemoryCached(context.Background(), bd, cache)

	if hit || got != MemoryDialectPreviewV2 {
		t.Errorf("after ForgetProbe = %q (hit %v), want a fresh %q", got, hit, MemoryDialectPreviewV2)
	}
}

func TestForgetProbe_MissingCacheIsNotAnError(t *testing.T) {
	bd := installFakeBd(t, previewBdScript)

	if err := ForgetProbe(bd, filepath.Join(t.TempDir(), "absent.json")); err != nil {
		t.Errorf("ForgetProbe on a missing cache = %v, want nil", err)
	}
}
