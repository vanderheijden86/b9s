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
       b9s attach get <issue-id> <sha256|name> [-o path]
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

	appCfg, cfgErr := config.Load()
	if cfgErr != nil {
		return nil, fmt.Errorf("loading b9s config: %w", cfgErr)
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
	// flag.FlagSet.Parse stops at the first non-flag argument, which would
	// reject "list <issue-id> --json" (the order the usage string shows);
	// scanning manually accepts --json in either position.
	jsonOut, rest := extractBoolFlag(args, "--json")
	if len(rest) != 1 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
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
	// See runAttachList: "-o path" can follow the positionals, which
	// flag.FlagSet.Parse cannot handle, so this scans for it manually too.
	out, rest, ok := extractStringFlag(args, "-o")
	if !ok || len(rest) != 2 {
		fmt.Fprint(stderr, attachUsage)
		return 2
	}
	issueID, query := rest[0], rest[1]

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

	// The temp file must land in the same directory it is ultimately renamed
	// into, since os.Rename does not cross filesystems; stdout has no such
	// destination, so it gets the system temp dir instead.
	destDir := "."
	switch {
	case out == "-":
		destDir = os.TempDir()
	case out != "":
		destDir = filepath.Dir(out)
	}
	tempPath, err := attach.Download(ctx, env.handle, found, destDir)
	if err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	defer os.Remove(tempPath)

	if out == "-" {
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

	destPath := out
	if destPath == "" {
		destPath, err = attach.SafeJoin(".", found.Name)
		if err != nil {
			fmt.Fprintf(stderr, "b9s attach: %v\n", err)
			return 1
		}
	}
	if err := os.Rename(tempPath, destPath); err != nil {
		fmt.Fprintf(stderr, "b9s attach: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, destPath)
	return 0
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

// extractBoolFlag reports whether name is present anywhere in args, and
// returns the remaining arguments with it removed. It exists because the
// documented usage puts a boolean flag after its positional arguments
// (list <issue-id> [--json]), a shape flag.FlagSet.Parse rejects.
func extractBoolFlag(args []string, name string) (present bool, rest []string) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == name {
			present = true
			continue
		}
		rest = append(rest, a)
	}
	return present, rest
}

// extractStringFlag finds "name value" anywhere in args and returns value
// together with the remaining arguments with both removed. ok is false if
// name appears with no following value. Like extractBoolFlag, this exists
// because the documented usage (get <issue-id> <sha256|name> [-o path])
// allows the flag after its positionals, which flag.FlagSet.Parse rejects.
func extractStringFlag(args []string, name string) (value string, rest []string, ok bool) {
	rest = make([]string, 0, len(args))
	ok = true
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			if i+1 >= len(args) {
				ok = false
				continue
			}
			value = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	return value, rest, ok
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
