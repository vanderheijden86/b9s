package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("B9S_WT_IMAGE_TEST_HELPER") == "1" {
		if err := execute(); err != nil {
			println(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "scripts/preview/Dockerfile.dockerignore", ".git\n.beads\ndocs\ndist\n")
	write(t, root, ".gitignore", "ignored\n")
	write(t, root, "cmd/b9s/main.go", "committed\n")
	write(t, root, "deleted.go", "deleted\n")
	write(t, root, "docs/private", "excluded\n")
	write(t, root, "pkg/web/dist/app.js", "embedded\n")
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "fixture")
	return root
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func captureTest(t *testing.T, root string) snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := capture(ctx, root, filepath.Join(t.TempDir(), "context"))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCaptureIncludesUncommittedBytes(t *testing.T) {
	root := fixture(t)
	write(t, root, "cmd/b9s/main.go", "staged\n")
	gitTest(t, root, "add", "cmd/b9s/main.go")
	write(t, root, "cmd/b9s/main.go", "working tree\n")
	write(t, root, "new.go", "untracked\n")
	got := captureTest(t, root)
	for name, want := range map[string]string{
		"cmd/b9s/main.go": "working tree\n",
		"new.go":          "untracked\n",
	} {
		body, err := os.ReadFile(filepath.Join(got.Dir, name))
		if err != nil || string(body) != want {
			t.Fatalf("%s = %q, %v, want %q", name, body, err, want)
		}
	}
	if !got.Source.Dirty || got.Source.Commit != gitTest(t, root, "rev-parse", "HEAD") {
		t.Fatalf("source = %+v", got.Source)
	}
}

func TestCaptureExcludesBuildIgnoredFiles(t *testing.T) {
	root := fixture(t)
	write(t, root, ".beads/credential", "secret\n")
	write(t, root, "ignored", "ignored\n")
	got := captureTest(t, root)
	for _, name := range []string{".git", ".beads", "docs", "ignored"} {
		if _, err := os.Lstat(filepath.Join(got.Dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s must be absent: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(got.Dir, "pkg/web/dist/app.js")); err != nil {
		t.Fatal("root dist exclusion must not exclude embedded web assets:", err)
	}
}

func TestCaptureOmitsDeletedTrackedFiles(t *testing.T) {
	root := fixture(t)
	if err := os.Rename(filepath.Join(root, "deleted.go"), filepath.Join(root, "ignored")); err != nil {
		t.Fatal(err)
	}
	got := captureTest(t, root)
	if _, err := os.Stat(filepath.Join(got.Dir, "deleted.go")); !os.IsNotExist(err) {
		t.Fatalf("deleted file included: %v", err)
	}
}

func TestCaptureHashIgnoresModificationTime(t *testing.T) {
	root := fixture(t)
	first := captureTest(t, root)
	now := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "cmd/b9s/main.go"), now, now); err != nil {
		t.Fatal(err)
	}
	second := captureTest(t, root)
	if first.Source.TreeHash != second.Source.TreeHash || first.Source.Dirty || second.Source.Dirty {
		t.Fatalf("mtime changed identity: %+v, %+v", first.Source, second.Source)
	}
}

func TestCaptureHashIdentifiesBytes(t *testing.T) {
	root := fixture(t)
	first := captureTest(t, root)
	write(t, root, "cmd/b9s/main.go", "different\n")
	second := captureTest(t, root)
	if first.Source.TreeHash == second.Source.TreeHash {
		t.Fatal("different source bytes share a tree hash")
	}
	body, err := os.ReadFile(filepath.Join(first.Dir, "cmd/b9s/main.go"))
	if err != nil || string(body) != "committed\n" {
		t.Fatal("source edit changed frozen build context")
	}
}

func TestCaptureHashIdentifiesExecutableMode(t *testing.T) {
	root := fixture(t)
	first := captureTest(t, root)
	if err := os.Chmod(filepath.Join(root, "cmd/b9s/main.go"), 0755); err != nil {
		t.Fatal(err)
	}
	second := captureTest(t, root)
	if first.Source.TreeHash == second.Source.TreeHash {
		t.Fatal("executable mode must change tree hash")
	}
}

func TestCaptureRejectsUnsupportedIgnorePatterns(t *testing.T) {
	root := fixture(t)
	write(t, root, "scripts/preview/Dockerfile.dockerignore", "*.secret\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := capture(ctx, root, filepath.Join(t.TempDir(), "context")); err == nil {
		t.Fatal("unsupported ignore syntax must fail before building an inaccurately hashed context")
	}
}

func TestCaptureRejectsEscapingSymlink(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := capture(ctx, root, filepath.Join(t.TempDir(), "context")); err == nil {
		t.Fatal("external symlink accepted")
	}
}

func TestCaptureRejectsSymlinkParent(t *testing.T) {
	root := fixture(t)
	write(t, root, ".gitignore", "ignored\ncmd\n")
	outside := t.TempDir()
	write(t, outside, "b9s/main.go", "outside source\n")
	if err := os.Rename(filepath.Join(root, "cmd"), filepath.Join(root, "ignored")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "cmd")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := capture(ctx, root, filepath.Join(t.TempDir(), "context")); err == nil {
		t.Fatal("tracked file copied through an escaping symlink parent")
	}
}

func imageCommand(t *testing.T, delivery string, failPush bool, registryArgs ...string) (string, string, error) {
	t.Helper()
	root := fixture(t)
	write(t, root, "cmd/b9s/main.go", "dirty build input\n")
	write(t, root, "scripts/preview/Dockerfile", "fixture Dockerfile\n")
	tools := t.TempDir()
	evidence := filepath.Join(t.TempDir(), "evidence")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$WT_TOOL_LOG"
case "$1" in
  build)
    for arg do context="$arg"; done
    cat "$context/cmd/b9s/main.go" > "$WT_BUILD_INPUT"
    ;;
  run) cat "$WT_EVIDENCE/source.json" ;;
  push) test "$WT_FAIL_PUSH" != 1 ;;
esac
`
	write(t, tools, "docker", script)
	write(t, tools, "k3d", "#!/bin/sh\nprintf 'k3d %s\\n' \"$*\" >> \"$WT_TOOL_LOG\"\n")
	for _, tool := range []string{"docker", "k3d"} {
		if err := os.Chmod(filepath.Join(tools, tool), 0755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := command(ctx, "", os.Args[0], "--repo", root, "--output", evidence, "--delivery", delivery)
	if delivery == "registry" {
		cmd.Args = append(cmd.Args, "--registry", "node-registry:5000", "--push-registry", "localhost:5001")
	} else {
		cmd.Args = append(cmd.Args, "--cluster", "this-worktree")
	}
	cmd.Args = append(cmd.Args, registryArgs...)
	pushValue := "0"
	if failPush {
		pushValue = "1"
	}
	cmd.Env = append(os.Environ(), "B9S_WT_IMAGE_TEST_HELPER=1",
		"PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WT_EVIDENCE="+evidence, "WT_TOOL_LOG="+filepath.Join(tools, "commands"),
		"WT_BUILD_INPUT="+filepath.Join(tools, "input"), "WT_FAIL_PUSH="+pushValue)
	body, err := cmd.CombinedOutput()
	log, readErr := os.ReadFile(filepath.Join(tools, "commands"))
	if readErr != nil {
		if err != nil && os.IsNotExist(readErr) {
			return evidence, string(body), err
		}
		t.Fatalf("command log: %v: %s", readErr, body)
	}
	input, readErr := os.ReadFile(filepath.Join(tools, "input"))
	if readErr != nil || string(input) != "dirty build input\n" {
		t.Fatalf("builder did not receive frozen dirty source: %q, %v", input, readErr)
	}
	return evidence, string(log), err
}

func TestRegistryRejectsAmbiguousAuthority(t *testing.T) {
	for _, value := range []string{"localregistry", "localregistry/prefix.with.dot"} {
		t.Run(value, func(t *testing.T) {
			evidence, output, err := imageCommand(t, "registry", false, "--registry", value)
			if err == nil || !strings.Contains(output, "explicit registry authority") {
				t.Fatalf("ambiguous --registry %q reached the builder: %v: %s", value, err, output)
			}
			if _, err := os.Stat(evidence); !os.IsNotExist(err) {
				t.Fatalf("registry rejection must precede source capture: %v", err)
			}
		})
	}
}

func TestPushRegistryRejectsAmbiguousAuthority(t *testing.T) {
	for _, value := range []string{"localregistry", "localregistry/prefix.with.dot"} {
		t.Run(value, func(t *testing.T) {
			evidence, output, err := imageCommand(t, "registry", false, "--push-registry", value)
			if err == nil || !strings.Contains(output, "explicit registry authority") {
				t.Fatalf("ambiguous --push-registry %q reached the builder: %v: %s", value, err, output)
			}
			if _, err := os.Stat(evidence); !os.IsNotExist(err) {
				t.Fatalf("push registry rejection must precede source capture: %v", err)
			}
		})
	}
}

func TestRegistryAcceptsExplicitAuthorities(t *testing.T) {
	for _, value := range []string{"localhost", "registry.internal", "localregistry:5000", "localhost/prefix.with.dot"} {
		t.Run(value, func(t *testing.T) {
			_, log, err := imageCommand(t, "registry", false, "--registry", value, "--push-registry", value)
			if err != nil || !strings.Contains(log, "push "+value+"/b9s-worktree:wt-") {
				t.Fatalf("explicit registry %q rejected or routed elsewhere: %v: %s", value, err, log)
			}
		})
	}
}

func TestDockerRegistryDeliveryUsesMirror(t *testing.T) {
	evidence, log, err := imageCommand(t, "registry", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--build-arg WORKTREE_DIRTY=true", "--build-arg WORKTREE_HASH=", "tag node-registry:5000/", "push localhost:5001/"} {
		if !strings.Contains(log, want) {
			t.Fatalf("missing %q in %s", want, log)
		}
	}
	if _, err := os.Stat(filepath.Join(evidence, "result.json")); err != nil {
		t.Fatal(err)
	}
}

func TestK3dDeliveryNamesOnlyAssignedCluster(t *testing.T) {
	_, log, err := imageCommand(t, "k3d", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "k3d image import --cluster this-worktree b9s-worktree:wt-") || strings.Contains(log, "push ") {
		t.Fatal(log)
	}
}

func TestFailedPushHasNoSuccessResult(t *testing.T) {
	evidence, _, err := imageCommand(t, "registry", true)
	if err == nil {
		t.Fatal("failed push reported success")
	}
	if _, err := os.Stat(filepath.Join(evidence, "result.json")); !os.IsNotExist(err) {
		t.Fatalf("failed delivery wrote success result: %v", err)
	}
}

func TestBakedSourceMismatchFails(t *testing.T) {
	body, err := json.Marshal(source{Commit: "one", Dirty: true, TreeHash: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if err := checkSource(body, source{Commit: "two", Dirty: true, TreeHash: "one"}); err == nil {
		t.Fatal("wrong image identity accepted")
	}
}
