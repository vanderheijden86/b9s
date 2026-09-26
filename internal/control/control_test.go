package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// shortDir returns a directory whose socket paths fit the 104-byte limit on
// macOS, which t.TempDir under /var/folders can exceed.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "b9sctl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestServeRegistersTheInstanceAndDeliversABranchRequest(t *testing.T) {
	dir := shortDir(t)
	got := make(chan Request, 1)
	srv, err := Serve(dir, Instance{PID: os.Getpid(), TmuxPane: "%7", Cwd: "/work"}, func(r Request) error {
		got <- r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("state dir mode = %v, %v; want 0700", info.Mode().Perm(), err)
	}
	instances, err := List(dir)
	if err != nil || len(instances) != 1 || instances[0].TmuxPane != "%7" {
		t.Fatalf("List = %+v, %v; want the one registered instance", instances, err)
	}

	if err := Send(instances[0], Request{Verb: VerbBranch, ID: "bd-abc"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case r := <-got:
		if r.Verb != VerbBranch || r.ID != "bd-abc" {
			t.Fatalf("handler got %+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler never received the request")
	}
}

func TestServeRejectsAnUnknownVerb(t *testing.T) {
	dir := shortDir(t)
	srv, err := Serve(dir, Instance{PID: os.Getpid()}, func(Request) error {
		t.Error("handler called for an unknown verb")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	instances, _ := List(dir)

	err = Send(instances[0], Request{Verb: "keys", ID: "q"})
	if err == nil || !strings.Contains(err.Error(), "unknown verb") {
		t.Fatalf("Send(unknown verb) = %v, want an unknown verb error", err)
	}
}

func TestCloseRemovesTheSocketAndRegistration(t *testing.T) {
	dir := shortDir(t)
	srv, err := Serve(dir, Instance{PID: os.Getpid()}, func(Request) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("Close left %d files behind", len(entries))
	}
}

func TestListDropsRegistrationsOfDeadProcesses(t *testing.T) {
	dir := shortDir(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "999999.json")
	if err := os.WriteFile(stale, []byte(`{"pid":999999,"socket":"/nowhere"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	instances, err := List(dir)
	if err != nil || len(instances) != 0 {
		t.Fatalf("List = %+v, %v; want no live instances", instances, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale registration was not removed")
	}
}

func TestPickChoosesTheInstanceInTheCallersTmuxWindow(t *testing.T) {
	instances := []Instance{
		{PID: 1, TmuxPane: "%10"},
		{PID: 2, TmuxPane: "%20"},
		{PID: 3},
	}
	windows := map[string]string{"%1": "@1", "%10": "@1", "%2": "@2", "%20": "@2", "%30": "@3"}

	tests := []struct {
		name    string
		sel     Selector
		wantPID int
		wantErr string
	}{
		{name: "explicit pane", sel: Selector{Pane: "%20", CallerPane: "%1"}, wantPID: 2},
		{name: "same window", sel: Selector{CallerPane: "%2"}, wantPID: 2},
		{name: "empty window", sel: Selector{CallerPane: "%30"}, wantErr: "no b9s in this tmux window"},
		{name: "unknown pane", sel: Selector{Pane: "%99"}, wantErr: "no b9s in pane %99"},
		{name: "outside tmux with several", sel: Selector{}, wantErr: "3 b9s instances"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Pick(instances, windows, tt.sel)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Pick err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got.PID != tt.wantPID {
				t.Fatalf("Pick = %+v, %v; want pid %d", got, err, tt.wantPID)
			}
		})
	}
}

func TestPickOutsideTmuxTakesTheOnlyInstance(t *testing.T) {
	got, err := Pick([]Instance{{PID: 5}}, nil, Selector{})
	if err != nil || got.PID != 5 {
		t.Fatalf("Pick = %+v, %v; want the only instance", got, err)
	}
}

func TestPickRejectsTwoInstancesInOneWindow(t *testing.T) {
	instances := []Instance{{PID: 1, TmuxPane: "%10"}, {PID: 2, TmuxPane: "%11"}}
	windows := map[string]string{"%1": "@1", "%10": "@1", "%11": "@1"}
	_, err := Pick(instances, windows, Selector{CallerPane: "%1"})
	if err == nil || !strings.Contains(err.Error(), "--pane") {
		t.Fatalf("Pick err = %v, want a hint to pass --pane", err)
	}
}

func TestParseTmuxPanesMapsPanesToWindows(t *testing.T) {
	got := parseTmuxPanes("%1 @3\n%12 @3\n\n%4 @9\n")
	if len(got) != 3 || got["%1"] != "@3" || got["%12"] != "@3" || got["%4"] != "@9" {
		t.Fatalf("parseTmuxPanes = %v", got)
	}
}
