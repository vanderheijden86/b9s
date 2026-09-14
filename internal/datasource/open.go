package datasource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-sql-driver/mysql"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// OpenReason classifies why a project could not be opened. It is a closed set:
// every reason has a readable message and next steps, shared by the CLI and
// the TUI.
type OpenReason uint8

const (
	OpenNotAProject OpenReason = iota + 1
	OpenServerDown
	OpenDenied
	OpenNoIssuesTable
	OpenUnreadable
	OpenTimedOut
	openReasonEnd
)

const mysqlErrUnknownDatabase = 1049

// OpenFailure describes a project that could not be opened.
type OpenFailure struct {
	Reason   OpenReason
	Project  string // display name
	Dir      string // project folder; empty for a database without a checkout
	Server   string // host:port of the Dolt server, when one was tried
	Database string
	User     string
	Err      error // the underlying error, for detail views
}

func (f *OpenFailure) Error() string {
	if f.Err == nil {
		return f.Message()
	}
	return f.Message() + ": " + f.Err.Error()
}

func (f *OpenFailure) Unwrap() error { return f.Err }

// Message is a one-line, human-readable reason. Reasons have no fallback text,
// so a reason added without a message fails TestEveryOpenReasonHasMessageAndNextSteps.
func (f *OpenFailure) Message() string {
	switch f.Reason {
	case OpenNotAProject:
		return fmt.Sprintf("%s is not a Beads project", f.location())
	case OpenServerDown:
		return fmt.Sprintf("Cannot reach the Dolt server at %s", f.Server)
	case OpenDenied:
		return fmt.Sprintf("User %s may not read database %s on %s", f.User, f.Database, f.Server)
	case OpenNoIssuesTable:
		return fmt.Sprintf("Database %s on %s is not a Beads database", f.Database, f.Server)
	case OpenUnreadable:
		return fmt.Sprintf("Cannot read the issues of %s", f.location())
	case OpenTimedOut:
		return fmt.Sprintf("%s took too long to open", f.Project)
	}
	return ""
}

// Try lists concrete next steps for the user.
func (f *OpenFailure) Try() []string {
	switch f.Reason {
	case OpenNotAProject:
		return []string{
			"Start b9s in a folder that contains a .beads directory",
			"Create a Beads project here: bd init",
		}
	case OpenServerDown:
		return []string{
			fmt.Sprintf("Check that the Dolt server or SSH tunnel for %s is running", f.Server),
			"Test the connection with bd: bd list",
		}
	case OpenDenied:
		return []string{
			fmt.Sprintf("Check BEADS_DOLT_PASSWORD for user %s", f.User),
			fmt.Sprintf("Ask the server admin for read access to %s", f.Database),
		}
	case OpenNoIssuesTable:
		return []string{
			"Check dolt_database in .beads/metadata.json",
			fmt.Sprintf("Create the database: bd init --server --database=%s", f.Database),
		}
	case OpenUnreadable:
		return []string{
			"Check .beads/metadata.json and the issues file",
			"Re-export the issues: bd export",
		}
	case OpenTimedOut:
		server := f.Server
		if server == "" {
			server = "this project"
		}
		return []string{
			"Retry: the server may be busy",
			fmt.Sprintf("Check that the Dolt server or SSH tunnel for %s is running", server),
		}
	}
	return nil
}

func (f *OpenFailure) location() string {
	if f.Dir != "" {
		return f.Dir
	}
	return f.Project
}

// OpenTarget names a project to open: a folder with a .beads directory, or a
// Dolt database without a local checkout.
type OpenTarget struct {
	Name string
	Dir  string
	Dolt *DataSource
}

// OpenedProject is a project whose issues loaded.
type OpenedProject struct {
	Issues []model.Issue
	Source DataSource
	// DoltFailure is set when the configured Dolt server failed and a local
	// export was read instead.
	DoltFailure *OpenFailure
}

// OpenProject loads a project's issues from its most authoritative readable
// source, or explains why it cannot. When the Dolt server fails, lower-priority
// local sources are still tried; if none of them opens, the Dolt failure is
// reported, because a missing export is a symptom rather than the cause.
func OpenProject(target OpenTarget) (OpenedProject, *OpenFailure) {
	sources, failure := openSources(target)
	if failure != nil {
		return OpenedProject{}, failure
	}

	var doltFailure, lastFailure *OpenFailure
	for _, source := range sources {
		issues, err := LoadFromSource(source)
		if err == nil {
			return OpenedProject{Issues: issues, Source: source, DoltFailure: doltFailure}, nil
		}
		f := &OpenFailure{Reason: OpenUnreadable, Project: target.Name, Dir: target.Dir, Err: err}
		if source.Type == SourceTypeDolt {
			f.Reason = classifyDoltFailure(err)
			f.Server = source.Path
			f.Database = source.Database
			f.User = source.User
			if doltFailure == nil {
				doltFailure = f
			}
		}
		lastFailure = f
	}
	if doltFailure != nil {
		return OpenedProject{}, doltFailure
	}
	return OpenedProject{}, lastFailure
}

func openSources(target OpenTarget) ([]DataSource, *OpenFailure) {
	if target.Dir == "" {
		if target.Dolt == nil {
			return nil, &OpenFailure{Reason: OpenNotAProject, Project: target.Name}
		}
		return []DataSource{*target.Dolt}, nil
	}

	beadsDir := filepath.Join(target.Dir, ".beads")
	if info, err := os.Stat(beadsDir); err != nil || !info.IsDir() {
		return nil, &OpenFailure{Reason: OpenNotAProject, Project: target.Name, Dir: target.Dir, Err: err}
	}
	sources, err := DiscoverSources(loadDiscoveryOptions(beadsDir, target.Dir))
	if err == nil && len(sources) == 0 {
		err = fmt.Errorf("no issues file or Dolt server configuration in %s", beadsDir)
	}
	if err != nil {
		return nil, &OpenFailure{Reason: OpenUnreadable, Project: target.Name, Dir: target.Dir, Err: err}
	}
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].Priority > sources[j].Priority })
	return sources, nil
}

// classifyDoltFailure maps a Dolt connection or query error to an OpenReason.
// An unknown database is reported like a missing issues table: in both cases
// the name does not point at a Beads database.
func classifyDoltFailure(err error) OpenReason {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrUnknownDatabase {
		return OpenNoIssuesTable
	}
	switch ClassifyConnError(err) {
	case ReachDenied:
		return OpenDenied
	case ReachNoIssuesTable:
		return OpenNoIssuesTable
	case ReachServerDown:
		return OpenServerDown
	case ReachReachable, ReachUnknown:
		return OpenUnreadable
	}
	return OpenUnreadable
}
