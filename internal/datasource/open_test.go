package datasource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// unreachableServer is a local port nothing listens on, so connecting fails
// fast with a refused connection.
const unreachableServer = "127.0.0.1:1"

func writeServerMetadata(t *testing.T, beadsDir, database string) {
	t.Helper()
	metadata, err := json.Marshal(map[string]any{
		"dolt_mode":        "server",
		"dolt_server_host": "127.0.0.1",
		"dolt_server_port": 1,
		"dolt_server_user": "nobody",
		"dolt_database":    database,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), metadata, 0o644); err != nil {
		t.Fatal(err)
	}
}

func beadsProject(t *testing.T) (dir, beadsDir string) {
	t.Helper()
	dir = t.TempDir()
	beadsDir = filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, beadsDir
}

func TestEveryOpenReasonHasMessageAndNextSteps(t *testing.T) {
	for reason := OpenNotAProject; reason < openReasonEnd; reason++ {
		f := &OpenFailure{Reason: reason, Project: "ghost", Dir: "/work/ghost", Server: "db.example:3306", Database: "ghost", User: "reader"}

		if msg := f.Message(); msg == "" || strings.Contains(msg, "%!") {
			t.Errorf("reason %d message = %q", reason, msg)
		}
		if try := f.Try(); len(try) == 0 {
			t.Errorf("reason %d has no next steps", reason)
		}
	}
}

func TestOpenProjectWithoutBeadsDirIsNotAProject(t *testing.T) {
	_, failure := OpenProject(OpenTarget{Name: "plain", Dir: t.TempDir()})

	if failure == nil || failure.Reason != OpenNotAProject {
		t.Fatalf("failure = %+v, want not a project", failure)
	}
}

func TestOpenProjectReportsUnreachableServerNotMissingExport(t *testing.T) {
	dir, beadsDir := beadsProject(t)
	writeServerMetadata(t, beadsDir, "ghost")

	_, failure := OpenProject(OpenTarget{Name: "ghost", Dir: dir})

	if failure == nil || failure.Reason != OpenServerDown {
		t.Fatalf("failure = %+v, want server down", failure)
	}
	if failure.Server != unreachableServer || failure.Database != "ghost" {
		t.Errorf("failure names server %q database %q, want %q ghost", failure.Server, failure.Database, unreachableServer)
	}
}

func TestOpenProjectReadsLocalExportWhenServerIsDown(t *testing.T) {
	dir, beadsDir := beadsProject(t)
	writeServerMetadata(t, beadsDir, "ghost")
	issue := `{"id":"g-1","title":"Exported","status":"open","priority":2,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(issue+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opened, failure := OpenProject(OpenTarget{Name: "ghost", Dir: dir})

	if failure != nil {
		t.Fatalf("failure = %+v, want the local export to open", failure)
	}
	if len(opened.Issues) != 1 {
		t.Errorf("opened %d issues, want 1", len(opened.Issues))
	}
	if opened.DoltFailure == nil || opened.DoltFailure.Reason != OpenServerDown {
		t.Errorf("dolt failure = %+v, want server down recorded", opened.DoltFailure)
	}
}

func TestOpenProjectOpensEmptyProject(t *testing.T) {
	dir, beadsDir := beadsProject(t)
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	opened, failure := OpenProject(OpenTarget{Name: "empty", Dir: dir})

	if failure != nil {
		t.Fatalf("failure = %+v, want an empty project to open", failure)
	}
	if len(opened.Issues) != 0 {
		t.Errorf("opened %d issues, want 0", len(opened.Issues))
	}
}

func TestOpenProjectWithoutReadableSourceIsUnreadable(t *testing.T) {
	dir, _ := beadsProject(t)

	_, failure := OpenProject(OpenTarget{Name: "bare", Dir: dir})

	if failure == nil || failure.Reason != OpenUnreadable {
		t.Fatalf("failure = %+v, want unreadable", failure)
	}
}

func TestOpenProjectDatabaseWithoutCheckoutReportsServerDown(t *testing.T) {
	source := DataSource{Type: SourceTypeDolt, Path: unreachableServer, Database: "ghost", User: "nobody", Priority: PriorityDolt}

	_, failure := OpenProject(OpenTarget{Name: "ghost", Dolt: &source})

	if failure == nil || failure.Reason != OpenServerDown {
		t.Fatalf("failure = %+v, want server down", failure)
	}
}

func TestClassifyDoltFailure(t *testing.T) {
	cases := []struct {
		err  error
		want OpenReason
	}{
		{&mysql.MySQLError{Number: 1045, Message: "Access denied"}, OpenDenied},
		{&mysql.MySQLError{Number: 1044, Message: "Access denied to database"}, OpenDenied},
		{&mysql.MySQLError{Number: 1105, Message: "Access denied for user 'reader'@'%' to database 'ghost'"}, OpenDenied},
		{&mysql.MySQLError{Number: 1146, Message: "Table 'ghost.issues' doesn't exist"}, OpenNoIssuesTable},
		{&mysql.MySQLError{Number: 1049, Message: "Unknown database 'ghost'"}, OpenNoIssuesTable},
		{fmt.Errorf("scan: %w", os.ErrClosed), OpenUnreadable},
	}
	for _, tc := range cases {
		if got := classifyDoltFailure(fmt.Errorf("cannot connect to Dolt: %w", tc.err)); got != tc.want {
			t.Errorf("classifyDoltFailure(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
