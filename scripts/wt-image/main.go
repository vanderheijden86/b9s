// wt-image builds a frozen worktree through the preview Dockerfile.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

type source struct {
	Commit   string `json:"commit"`
	Dirty    bool   `json:"dirty"`
	TreeHash string `json:"tree_hash"`
}

type snapshot struct {
	Dir      string
	Source   source
	Manifest []input
}

type input struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wt-image:", err)
		os.Exit(1)
	}
}

func execute() error {
	repo := flag.String("repo", ".", "worktree to build")
	builder := flag.String("builder", "docker", "docker or buildkit")
	delivery := flag.String("delivery", "registry", "registry or k3d")
	registry := flag.String("registry", "", "local registry host[:port][/prefix] used by cluster nodes")
	pushRegistry := flag.String("push-registry", "", "Docker push endpoint if different from the cluster registry mirror")
	cluster := flag.String("cluster", "", "existing local k3d cluster, required for k3d delivery")
	addr := flag.String("buildkit-addr", "", "BuildKit address, required for buildkit")
	insecure := flag.Bool("insecure-registry", false, "use HTTP for a local BuildKit registry")
	output := flag.String("output", "", "fresh evidence directory outside the worktree, retained after exit")
	limit := flag.Duration("timeout", 10*time.Minute, "total deadline, at most 10m")
	flag.Parse()
	if flag.NArg() != 0 || *limit <= 0 || *limit > 10*time.Minute {
		return errors.New("unexpected arguments or timeout outside (0, 10m]")
	}
	if *builder != "docker" && *builder != "buildkit" {
		return errors.New("--builder must be docker or buildkit")
	}
	switch *delivery {
	case "registry":
		registryPattern := regexp.MustCompile(`^[a-zA-Z0-9.-]+(:[0-9]+)?(/[a-z0-9._-]+)*$`)
		validRegistry := func(value string) bool {
			authority, _, _ := strings.Cut(value, "/")
			return registryPattern.MatchString(value) &&
				(authority == "localhost" || strings.ContainsAny(authority, ".:"))
		}
		if !validRegistry(*registry) || (*pushRegistry != "" && !validRegistry(*pushRegistry)) {
			return errors.New("--registry and --push-registry require an explicit registry authority: localhost, a dotted host or host:port, without a scheme")
		}
	case "k3d":
		if *builder != "docker" || !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`).MatchString(*cluster) {
			return errors.New("k3d delivery requires --builder docker and an explicit --cluster")
		}
	default:
		return errors.New("--delivery must be registry or k3d")
	}
	if *builder == "buildkit" && (*addr == "" || (*pushRegistry != "" && *pushRegistry != *registry)) {
		return errors.New("buildkit requires --buildkit-addr and the same registry for build and push")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *limit)
	defer cancel()
	root, err := git(ctx, *repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if *output == "" {
		*output, err = os.MkdirTemp("", "b9s-wt-image-")
	} else {
		*output, err = filepath.Abs(*output)
		if err == nil {
			err = os.Mkdir(*output, 0755)
		}
	}
	if err != nil {
		return fmt.Errorf("creating fresh output directory: %w", err)
	}
	evidence, err := filepath.EvalSymlinks(*output)
	if err != nil {
		return err
	}
	if within(root, evidence) {
		return errors.New("--output must be outside the worktree")
	}
	fmt.Fprintln(os.Stderr, "EVIDENCE_DIR="+evidence)
	frozen, err := capture(ctx, root, filepath.Join(evidence, "context"))
	if err != nil {
		return err
	}
	if err := saveJSON(filepath.Join(evidence, "source.json"), frozen.Source); err != nil {
		return err
	}
	if err := saveJSON(filepath.Join(evidence, "manifest.json"), frozen.Manifest); err != nil {
		return err
	}
	tag := fmt.Sprintf("wt-%s-%t-%s", frozen.Source.Commit[:12], frozen.Source.Dirty, frozen.Source.TreeHash)
	image := "b9s-worktree:" + tag
	pushImage := image
	if *delivery == "registry" {
		image = *registry + "/" + image
		pushImage = image
		if *pushRegistry != "" {
			pushImage = *pushRegistry + "/b9s-worktree:" + tag
		}
	}
	buildArgs := []string{
		"COMMIT_SHA=" + frozen.Source.Commit,
		fmt.Sprintf("WORKTREE_DIRTY=%t", frozen.Source.Dirty),
		"WORKTREE_HASH=" + frozen.Source.TreeHash,
	}
	dockerfile := filepath.Join(frozen.Dir, "scripts", "preview", "Dockerfile")
	if *builder == "docker" {
		args := []string{"docker", "build", "--file", dockerfile, "--target", "runtime", "--tag", image}
		for _, arg := range buildArgs {
			args = append(args, "--build-arg", arg)
		}
		args = append(args, frozen.Dir)
		if err := logged(ctx, evidence, "build", args...); err != nil {
			return err
		}
		baked, err := command(ctx, "", "docker", "run", "--rm", "--network", "none", "--entrypoint", "cat", image, "/app/.source.json").Output()
		if err != nil {
			return fmt.Errorf("reading baked source: %w", err)
		}
		if err := checkSource(baked, frozen.Source); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(evidence, "baked-source.json"), baked, 0644); err != nil {
			return err
		}
		if *delivery == "registry" {
			if pushImage != image {
				if err := logged(ctx, evidence, "tag", "docker", "tag", image, pushImage); err != nil {
					return err
				}
			}
			if err := logged(ctx, evidence, "delivery", "docker", "push", pushImage); err != nil {
				return err
			}
		} else if err := logged(ctx, evidence, "delivery", "k3d", "image", "import", "--cluster", *cluster, image); err != nil {
			return err
		}
	} else {
		args := []string{"buildctl", "--addr", *addr, "build",
			"--frontend", "dockerfile.v0", "--progress", "plain",
			"--local", "context=" + frozen.Dir,
			"--local", "dockerfile=" + filepath.Dir(dockerfile),
			"--opt", "filename=Dockerfile", "--opt", "target=runtime",
			"--metadata-file", filepath.Join(evidence, "image.json"),
			"--output", fmt.Sprintf("type=image,name=%s,push=true,registry.insecure=%t", image, *insecure),
			"--output", "type=local,dest=" + filepath.Join(evidence, "runtime")}
		for _, arg := range buildArgs {
			args = append(args, "--opt", "build-arg:"+arg)
		}
		if err := logged(ctx, evidence, "build-and-push", args...); err != nil {
			return err
		}
		baked, err := os.ReadFile(filepath.Join(evidence, "runtime", "app", ".source.json"))
		if err != nil {
			return fmt.Errorf("reading baked source: %w", err)
		}
		if err := checkSource(baked, frozen.Source); err != nil {
			return err
		}
	}
	result := struct {
		source
		Image    string `json:"image"`
		Delivery string `json:"delivery"`
		Evidence string `json:"evidence"`
	}{frozen.Source, image, *delivery, evidence}
	if err := saveJSON(filepath.Join(evidence, "result.json"), result); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func checkSource(body []byte, want source) error {
	var got source
	if err := json.Unmarshal(body, &got); err != nil {
		return fmt.Errorf("decoding baked source: %w", err)
	}
	if got != want {
		return fmt.Errorf("baked source mismatch: got %+v, want %+v", got, want)
	}
	return nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func capture(ctx context.Context, root, dest string) (snapshot, error) {
	result := snapshot{Dir: dest}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	status, err := git(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return result, err
	}
	result.Source = source{Commit: head, Dirty: status != ""}
	ignore, err := os.ReadFile(filepath.Join(root, "scripts", "preview", "Dockerfile.dockerignore"))
	if err != nil {
		return result, fmt.Errorf("reading preview context exclusions: %w", err)
	}
	excluded := []string{".git"}
	for _, line := range strings.Split(string(ignore), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// A closed syntax keeps the snapshot filter and Docker's filter identical.
		if strings.ContainsAny(line, "*?![\\") || filepath.ToSlash(filepath.Clean(line)) != line || strings.HasPrefix(line, "../") || filepath.IsAbs(line) || line == "." || line == ".." {
			return result, fmt.Errorf("unsupported preview exclusion %q: expected a literal relative path", line)
		}
		excluded = append(excluded, line)
	}
	files, err := command(ctx, root, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return result, fmt.Errorf("listing worktree source: %w", err)
	}
	names := strings.Split(strings.TrimSuffix(string(files), "\x00"), "\x00")
	sort.Strings(names)
	if err := os.Mkdir(dest, 0755); err != nil {
		return result, err
	}
	tree := sha256.New()
	last := ""
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if name == "" || name == last {
			continue
		}
		last = name
		skip := false
		for _, prefix := range excluded {
			if name == prefix || strings.HasPrefix(name, prefix+"/") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil || !within(root, parent) {
			return result, fmt.Errorf("source parent escapes the build context: %s", name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return result, err
		}
		mode := "100644"
		hash := sha256.New()
		switch {
		case info.Mode().IsRegular():
			perm := os.FileMode(0644)
			if info.Mode()&0111 != 0 {
				perm, mode = 0755, "100755"
			}
			in, err := os.Open(path)
			if err != nil {
				return result, err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
			if err != nil {
				in.Close()
				return result, err
			}
			if err := out.Chmod(perm); err != nil {
				in.Close()
				out.Close()
				return result, err
			}
			_, copyErr := io.Copy(io.MultiWriter(out, hash), in)
			inErr, outErr := in.Close(), out.Close()
			if err := errors.Join(copyErr, inErr, outErr); err != nil {
				return result, err
			}
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return result, err
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil || !within(root, resolved) || filepath.IsAbs(link) {
				return result, fmt.Errorf("source symlink escapes the build context: %s", name)
			}
			if err := os.Symlink(link, target); err != nil {
				return result, err
			}
			mode = "120000"
			hash.Write([]byte(link))
		default:
			return result, fmt.Errorf("unsupported source file type: %s", name)
		}
		fmt.Fprintf(tree, "%s\x00%s\x00%x\n", name, mode, hash.Sum(nil))
		result.Manifest = append(result.Manifest, input{name, mode, hex.EncodeToString(hash.Sum(nil))})
	}
	result.Source.TreeHash = hex.EncodeToString(tree.Sum(nil))
	afterHead, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	afterStatus, err := git(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return result, err
	}
	if afterHead != head || afterStatus != status {
		return result, errors.New("worktree changed while capturing source, use a fresh output directory and retry")
	}
	return result, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := command(ctx, dir, "git", args...)
	cmd.Stderr = &stderr
	body, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, &stderr)
	}
	return strings.TrimSpace(string(body)), nil
}

func command(ctx context.Context, dir, executable string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	return cmd
}

func logged(ctx context.Context, evidence, step string, argv ...string) error {
	log, err := os.OpenFile(filepath.Join(evidence, step+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer log.Close()
	if err := json.NewEncoder(log).Encode(argv); err != nil {
		return err
	}
	cmd := command(ctx, "", argv[0], argv[1:]...)
	cmd.Stdout = io.MultiWriter(os.Stderr, log)
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w (deadline: %v), see %s", step, err, ctx.Err(), log.Name())
	}
	return nil
}

func saveJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0644)
}
