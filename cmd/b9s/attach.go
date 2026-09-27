package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attach"
	"github.com/vanderheijden86/beadwork/internal/bdrun"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

const attachUsage = `usage: b9s attach <issue-id> <file>...
       b9s attach --detach <issue-id> <sha256>
       b9s attach list <issue-id> [--json]
       b9s attach get <issue-id> <sha256|name> [-o path] [--force]
       b9s attach url <issue-id> <sha256|name>

Uploads a file to the project's configured blob store and writes the Beads
reference comment that attaches it (ADR 0024). Needs an "attachments:"
section in b9s's config file; see README.md, section "Attachments".
`

// runAttach implements `b9s attach` and returns the process exit code.
func runAttach(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}

	switch args[0] {
	case "list":
		return runAttachList(args[1:], stdout, stderr)
	case "get":
		return runAttachGet(args[1:], stdout, stderr)
	case "url":
		return runAttachURL(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, attachUsage)
		return 0
	default:
		return runAttachAddOrDetach(args, stdout, stderr)
	}
}

// runAttachAddOrDetach handles the two forms that share a flag set: adding
// one or more files, and detaching a hash with --detach.
func runAttachAddOrDetach(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	detach := fs.Bool("detach", false, "detach an attachment instead of adding one")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	rest := fs.Args()

	if *detach {
		if len(rest) != 2 {
			fmt.Fprint(stderr, attachUsage)
			return 2
		}
		return runAttachDetach(rest[0], rest[1], stdout, stderr)
	}
	if len(rest) < 2 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	return runAttachAdd(rest[0], rest[1:], stdout, stderr)
}

// attachEnv is the project context every b9s attach subcommand needs: a blob
// store bound to the current project, a way to run bd, and a way to read an
// issue's comments. Building it once per invocation keeps every subcommand
// free of the datasource/config wiring.
type attachEnv struct {
	handle *blobstore.Handle
	bd     attach.BdRunner
	cl     attach.CommentLoader
}

func newAttachEnv(ctx context.Context) (*attachEnv, error) {
	// Checked first, and alone: a project with no attachments: section has
	// no need for bd on PATH, a .beads directory, or an open data source,
	// so none of the three is resolved before this fails.
	appCfg, cfgErr := config.Load()
	if cfgErr != nil {
		return nil, fmt.Errorf("loading b9s config: %w", cfgErr)
	}
	if appCfg.Attachments == nil {
		return nil, fmt.Errorf(
			"attachments are not configured for this project; add an attachments: section to %s (README.md, section \"Attachments\"): %w",
			config.ConfigPath(), blobstore.ErrNotConfigured)
	}

	bdPath, ok := bdrun.Resolve()
	if !ok {
		return nil, errors.New("bd is not installed or not on PATH")
	}

	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		return nil, fmt.Errorf("finding .beads directory: %w", err)
	}
	projectDir := filepath.Dir(beadsDir)
	projectName := filepath.Base(projectDir)

	opened, failure := datasource.OpenProject(datasource.OpenTarget{Name: projectName, Dir: projectDir})
	if failure != nil {
		return nil, failure
	}

	srcInfo, err := blobstore.SourceInfoFromDataSource(opened.Source, beadsDir, projectName)
	if err != nil {
		return nil, err
	}
	handle, err := blobstore.Open(ctx, appCfg.Attachments, srcInfo)
	if err != nil {
		if errors.Is(err, blobstore.ErrNotConfigured) {
			return nil, fmt.Errorf(
				"attachments are not configured for this project; add an attachments: section to %s (README.md, section \"Attachments\"): %w",
				config.ConfigPath(), err)
		}
		return nil, err
	}

	bd := attach.RunnerFunc(func(args ...string) (string, error) {
		return bdrun.Run(bdPath, projectDir, args...)
	})
	cl := attach.CommentLoaderFunc(func(issueID string) ([]*model.Comment, error) {
		issues, err := datasource.LoadFromSource(opened.Source)
		if err != nil {
			return nil, err
		}
		for i := range issues {
			if issues[i].ID == issueID {
				return issues[i].Comments, nil
			}
		}
		return nil, fmt.Errorf("issue %s not found", issueID)
	})

	return &attachEnv{handle: handle, bd: bd, cl: cl}, nil
}

// attachEnvExitCode reports the exit code for a newAttachEnv failure:
// missing configuration is a usage problem (2), everything else (bd
// missing, the project's data source unreachable) is a runtime failure (1).
func attachEnvExitCode(err error) int {
	if errors.Is(err, blobstore.ErrNotConfigured) {
		return 2
	}
	return 1
}

func runAttachAdd(issueID string, paths []string, stdout, stderr io.Writer) int {
	ctx := context.Background()
	env, err := newAttachEnv(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return attachEnvExitCode(err)
	}

	results := attach.Add(ctx, env.handle, env.bd, issueID, paths)
	exit := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(stderr, "b9s attach: %v\n", r.Err)
			exit = 1
			continue
		}
		status := "attached"
		if r.Skipped {
			status = "attached (blob already present)"
		}
		fmt.Fprintf(stdout, "%s: %s %s (%s)\n", r.Path, status, r.Ref.SHA256, r.Ref.Type)
	}
	return exit
}

func runAttachDetach(issueID, sha256Hash string, stdout, stderr io.Writer) int {
	env, err := newAttachEnv(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return attachEnvExitCode(err)
	}

	if err := attach.Detach(env.bd, env.cl, issueID, sha256Hash); err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "detached %s from %s\n", sha256Hash, issueID)
	return 0
}

func runAttachList(args []string, stdout, stderr io.Writer) int {
	_, bools, rest, err := parseFlags(args, []string{"--json"}, nil)
	if err != nil || len(rest) != 1 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	jsonOut := bools["--json"]
	issueID := rest[0]

	env, err := newAttachEnv(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return attachEnvExitCode(err)
	}

	attachments, err := attach.List(env.cl, issueID)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}

	if jsonOut {
		return printAttachListJSON(attachments, stdout, stderr)
	}
	if len(attachments) == 0 {
		fmt.Fprintf(stdout, "%s has no attachments\n", issueID)
		return 0
	}
	for _, a := range attachments {
		fmt.Fprintf(stdout, "%s  %8d  %-30s  %s\n", a.SHA256, a.Size, a.Type, a.Name)
	}
	return 0
}

// jsonAttachment is the `attach list --json` element shape: every field
// attach.List exposes, spelled the way a script consuming this output
// expects (snake_case, no embedded struct).
type jsonAttachment struct {
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	AddedBy   string    `json:"added_by"`
	AddedAt   time.Time `json:"added_at"`
	CommentID string    `json:"comment_id"`
}

func printAttachListJSON(attachments []attach.Attachment, stdout, stderr io.Writer) int {
	out := make([]jsonAttachment, 0, len(attachments))
	for _, a := range attachments {
		out = append(out, jsonAttachment{
			SHA256: a.SHA256, Size: a.Size, Type: a.Type, Name: a.Name,
			AddedBy: a.AddedBy, AddedAt: a.AddedAt, CommentID: a.CommentID,
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	return 0
}

func runAttachGet(args []string, stdout, stderr io.Writer) int {
	values, bools, rest, err := parseFlags(args, []string{"--force"}, []string{"-o"})
	if err != nil || len(rest) != 2 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	issueID, query := rest[0], rest[1]
	out := values["-o"]
	force := bools["--force"]

	ctx := context.Background()
	env, err := newAttachEnv(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return attachEnvExitCode(err)
	}

	found, err := resolveOneAttachment(env, issueID, query)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}

	if out == "-" {
		tempPath, err := attach.Download(ctx, env.handle, found, os.TempDir())
		if err != nil {
			fmt.Fprintf(stderr, "b9s attach: %v\n", err)
			return 1
		}
		defer os.Remove(tempPath)
		f, err := os.Open(tempPath)
		if err != nil {
			fmt.Fprintf(stderr, "b9s attach: %v\n", err)
			return 1
		}
		defer f.Close()
		if _, err := io.Copy(stdout, f); err != nil {
			fmt.Fprintf(stderr, "b9s attach: %v\n", err)
			return 1
		}
		return 0
	}

	destPath, err := resolveGetDestPath(out, found.Name)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}

	// The temp file must land in destPath's directory: placeDownloadedFile's
	// os.Link and os.Rename do not cross filesystems.
	tempPath, err := attach.Download(ctx, env.handle, found, filepath.Dir(destPath))
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	defer os.Remove(tempPath)

	if err := placeDownloadedFile(tempPath, destPath, force); err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, destPath)
	return 0
}

// resolveGetDestPath turns -o's raw value into the file path get writes to.
// An empty value defaults to the attachment's name in the current
// directory; a value naming an existing directory writes the name inside
// it, rather than attempting to write over the directory itself.
func resolveGetDestPath(out, name string) (string, error) {
	if out == "" {
		return attach.SafeJoin(".", name)
	}
	if info, err := os.Stat(out); err == nil && info.IsDir() {
		return attach.SafeJoin(out, name)
	}
	return out, nil
}

// placeDownloadedFile moves tempPath (created by attach.Download, mode 0600
// like every os.CreateTemp file) to destPath with the permissions a normal
// file create would produce, either refusing to replace an existing file at
// destPath or, with force, replacing it outright.
func placeDownloadedFile(tempPath, destPath string, force bool) error {
	if err := os.Chmod(tempPath, normalFileMode()); err != nil {
		return err
	}
	if force {
		return os.Rename(tempPath, destPath)
	}
	return placeByExclusiveCreate(tempPath, destPath)
}

// placeByExclusiveCreate links tempPath to destPath, which succeeds only if
// destPath does not already exist as any file type, including a symlink:
// link(2) never follows a symlink already at destPath, so this can never be
// tricked into overwriting whatever file the symlink points to. When Link
// fails for a reason other than destPath already existing (crossing a
// filesystem boundary, or a filesystem with no hard links), the fallback is
// O_CREATE|O_EXCL, which gives the identical non-follow guarantee without
// needing tempPath and destPath to share a device.
func placeByExclusiveCreate(tempPath, destPath string) error {
	if err := os.Link(tempPath, destPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; use --force to overwrite", destPath)
		}
		if openErr := placeByExclusiveOpen(tempPath, destPath); openErr != nil {
			return openErr
		}
		return os.Remove(tempPath)
	}
	return os.Remove(tempPath)
}

// placeByExclusiveOpen copies tempPath's bytes into a file newly created at
// destPath with O_EXCL, the fallback placeByExclusiveCreate uses when Link
// cannot place the file directly.
func placeByExclusiveOpen(tempPath, destPath string) error {
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, normalFileMode())
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; use --force to overwrite", destPath)
		}
		return err
	}
	defer dst.Close()
	src, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(destPath)
		return err
	}
	return nil
}

func runAttachURL(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	issueID, query := args[0], args[1]

	ctx := context.Background()
	env, err := newAttachEnv(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return attachEnvExitCode(err)
	}

	found, err := resolveOneAttachment(env, issueID, query)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}

	url, err := attach.URL(ctx, env.handle, found)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, url)
	return 0
}

// parseFlags scans args for the flags named in boolFlags and valueFlags,
// wherever they appear: the documented usage puts a flag after its
// positional arguments (list <issue-id> [--json], get <issue-id>
// <sha256|name> [-o path]), a shape flag.FlagSet.Parse rejects since it
// stops scanning at the first non-flag argument.
//
// "--" ends flag scanning; every argument after it, including one that
// looks like a flag, is positional. A value flag accepts "name=value" as
// well as "name value". Every other token starting with "-" (other than a
// bare "-", reserved as a positional meaning stdin/stdout) is an unknown
// flag and an error, never silently treated as positional. A flag given
// more than once is an error, and so is a value that itself looks like a
// flag (starts with "-"), except a bare "-" as -o's value, which get's
// stdout convention requires.
func parseFlags(args []string, boolFlags, valueFlags []string) (values map[string]string, bools map[string]bool, rest []string, err error) {
	isBool := make(map[string]bool, len(boolFlags))
	for _, name := range boolFlags {
		isBool[name] = true
	}
	isValue := make(map[string]bool, len(valueFlags))
	for _, name := range valueFlags {
		isValue[name] = true
	}
	values = make(map[string]string)
	bools = make(map[string]bool)
	seen := make(map[string]bool)
	rest = make([]string, 0, len(args))

	setValue := func(name, value string) error {
		if seen[name] {
			return fmt.Errorf("flag %s given more than once", name)
		}
		if value != "-" || name != "-o" {
			if strings.HasPrefix(value, "-") {
				return fmt.Errorf("flag %s value %q looks like another flag", name, value)
			}
		}
		seen[name] = true
		values[name] = value
		return nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if isBool[a] {
			if seen[a] {
				return nil, nil, nil, fmt.Errorf("flag %s given more than once", a)
			}
			seen[a] = true
			bools[a] = true
			continue
		}
		if isValue[a] {
			if i+1 >= len(args) {
				return nil, nil, nil, fmt.Errorf("flag %s needs a value", a)
			}
			if err := setValue(a, args[i+1]); err != nil {
				return nil, nil, nil, err
			}
			i++
			continue
		}
		if name, value, ok := strings.Cut(a, "="); ok && isValue[name] {
			if err := setValue(name, value); err != nil {
				return nil, nil, nil, err
			}
			continue
		}
		if a != "-" && strings.HasPrefix(a, "-") {
			return nil, nil, nil, fmt.Errorf("unknown flag %q", a)
		}
		rest = append(rest, a)
	}
	return values, bools, rest, nil
}

// resolveOneAttachment lists issueID's attachments and resolves query
// against them, the lookup both `get` and `url` need before touching the
// blob store.
func resolveOneAttachment(env *attachEnv, issueID, query string) (attach.Attachment, error) {
	attachments, err := attach.List(env.cl, issueID)
	if err != nil {
		return attach.Attachment{}, err
	}
	return attach.Resolve(attachments, query)
}
