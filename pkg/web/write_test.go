package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// fakeBd puts a bd on PATH that logs one line per call ("dir|args") and
// fails when an argument is "fail". It returns the log path.
func fakeBd(t *testing.T) (*ui.IssueWriter, string) {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := "#!/bin/sh\n" +
		"printf '%s|%s\\n' \"$PWD\" \"$*\" >> '" + log + "'\n" +
		"for a in \"$@\"; do [ \"$a\" = fail ] && { echo 'bd: refused'; exit 1; }; done\n" +
		"[ \"$1\" = create ] && echo 'Created issue: t-9'\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := ui.NewIssueWriter()
	if !w.IsAvailable() {
		t.Fatal("fake bd not found on PATH")
	}
	return w, log
}

func bdCalls(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func postWrite(t *testing.T, ts *testServer, c *http.Client, body string) (int, WriteResult) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/write", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ts.auth != nil {
		req.Header.Set(csrfHeader, ts.auth.CSRF())
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res WriteResult
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func TestWritesRunBdInTheCheckout(t *testing.T) {
	writer, log := fakeBd(t)
	ts := newTestServer(t, testAuth(t), writer)
	c := ts.pairedClient(t)
	dir, _ := filepath.EvalSymlinks(ts.dir)

	cases := []struct {
		body string
		args string
	}{
		{`{"op":"status","ids":["t-1"],"status":"in_progress"}`, "update t-1 --status=in_progress"},
		{`{"op":"status","ids":["t-1","t-2"],"status":"open"}`, "update t-1 t-2 --status=open"},
		{`{"op":"status","ids":["t-3"],"status":"closed","reason":"done"}`, "close t-3 --reason=done"},
		{`{"op":"close","ids":["t-1","t-3"]}`, "close t-1 t-3"},
		{`{"op":"delete","ids":["t-5"]}`, "delete t-5 --force"},
		{`{"op":"update","ids":["t-2"],"fields":{"priority":"P1"}}`, "update t-2 --priority=1"},
		{`{"op":"update","ids":["t-2"],"fields":{"title":"--status=closed"}}`, "update t-2 --title=--status=closed"},
		{`{"op":"comment","ids":["t-5"],"text":"-x looks like a flag"}`, "comments add t-5 -- -x looks like a flag"},
		{`{"op":"defer","ids":["t-3"],"until":"2026-10-01"}`, "defer t-3 --until=2026-10-01"},
	}
	for i, tc := range cases {
		code, res := postWrite(t, ts, c, tc.body)
		if code != http.StatusOK || !res.OK {
			t.Fatalf("%s: %d %+v", tc.body, code, res)
		}
		calls := bdCalls(t, log)
		if len(calls) != i+1 {
			t.Fatalf("%s: %d bd calls, want %d", tc.body, len(calls), i+1)
		}
		gotDir, args, _ := strings.Cut(calls[i], "|")
		gotDir, _ = filepath.EvalSymlinks(gotDir)
		if gotDir != dir || args != tc.args {
			t.Errorf("%s: bd ran %q in %q, want %q in %q", tc.body, args, gotDir, tc.args, dir)
		}
	}
}

func TestCreateReportsTheNewID(t *testing.T) {
	writer, _ := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	code, res := postWrite(t, ts, ts.Client(), `{"op":"create","fields":{"title":"New","type":"task","priority":"2"}}`)
	if code != http.StatusOK || res.Created != "t-9" {
		t.Fatalf("create: %d %+v", code, res)
	}
}

func TestInvalidWritesNeverReachBd(t *testing.T) {
	writer, log := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	bodies := []string{
		`{"op":"close","ids":["--all"]}`,
		`{"op":"close","ids":["t-1; rm -rf /"]}`,
		`{"op":"close","ids":[]}`,
		`{"op":"status","ids":["t-1"],"status":"tombstone"}`,
		`{"op":"update","ids":["t-1"],"fields":{"status":"tombstone"}}`,
		`{"op":"update","ids":["t-1"],"fields":{"db":"/tmp/x"}}`,
		`{"op":"update","ids":["t-1"],"fields":{"priority":"9"}}`,
		`{"op":"update","ids":["t-1"],"fields":{"parent":"--x"}}`,
		`{"op":"update","ids":["t-1","t-2"],"fields":{"title":"x"}}`,
		`{"op":"update","ids":["t-1"],"fields":{}}`,
		`{"op":"create","fields":{"type":"task"}}`,
		`{"op":"comment","ids":["t-1"],"text":"  "}`,
		`{"op":"defer","ids":["t-1"],"until":"--x"}`,
		`{"op":"sync"}`,
		`{"op":"close","ids":["t-1"],"extra":1}`,
		`not json`,
	}
	for _, body := range bodies {
		if code, _ := postWrite(t, ts, ts.Client(), body); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", body, code)
		}
	}
	if calls := bdCalls(t, log); len(calls) != 0 {
		t.Fatalf("bd ran for invalid writes: %v", calls)
	}
}

func TestRetriedWriteRunsBdOnce(t *testing.T) {
	writer, log := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	body := `{"op":"close","ids":["t-1"],"key":"phone-42"}`
	for i := 0; i < 3; i++ {
		if code, res := postWrite(t, ts, ts.Client(), body); code != http.StatusOK || !res.OK {
			t.Fatalf("attempt %d: %d %+v", i, code, res)
		}
	}
	if calls := bdCalls(t, log); len(calls) != 1 {
		t.Fatalf("bd ran %d times for one idempotency key", len(calls))
	}
}

func TestFailedWriteReportsBdOutput(t *testing.T) {
	writer, _ := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	code, res := postWrite(t, ts, ts.Client(), `{"op":"close","ids":["t-1"],"reason":"fail"}`)
	if code != http.StatusOK && code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", code)
	}
	// "--reason=fail" is one argument, so the fake does not fail on it.
	if !res.OK {
		t.Fatalf("reason text must not reach bd as its own argument: %+v", res)
	}
	code, res = postWrite(t, ts, ts.Client(), `{"op":"update","ids":["fail"],"fields":{"title":"x"}}`)
	if code != http.StatusUnprocessableEntity || res.OK || !strings.Contains(res.Error, "bd: refused") {
		t.Fatalf("failing bd: %d %+v", code, res)
	}
}

func TestReadOnlyProjectRefusesWrites(t *testing.T) {
	writer, log := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	// A project read from outside its checkout: the fixture file is reused,
	// but the target has no directory to run bd in.
	src := datasource.DataSource{Type: datasource.SourceTypeJSONLLocal, Path: filepath.Join(ts.dir, ".beads", "issues.jsonl")}
	if failure := ts.store.Open(datasource.OpenTarget{Name: "remote", Dolt: &src}, "remote"); failure == nil {
		if !ts.store.Info().ReadOnly {
			t.Fatal("a project without a checkout must be read-only")
		}
		code, res := postWrite(t, ts, ts.Client(), `{"op":"close","ids":["t-1"]}`)
		if code != http.StatusUnprocessableEntity || !strings.Contains(res.Error, "read-only") {
			t.Fatalf("write on read-only project: %d %+v", code, res)
		}
	} else {
		// OpenProject refuses a non-Dolt source without a directory; the
		// checkout rule is still covered by the writer's own tests.
		t.Skipf("cannot open a checkout-less JSONL project: %s", failure.Message())
	}
	if calls := bdCalls(t, log); len(calls) != 0 {
		t.Fatalf("bd ran for a read-only project: %v", calls)
	}
}

func TestWriteReloadsBeforeAnswering(t *testing.T) {
	writer, _ := fakeBd(t)
	ts := newTestServer(t, nil, writer)
	before := ts.store.Version()
	_, res := postWrite(t, ts, ts.Client(), `{"op":"close","ids":["t-1"]}`)
	if res.Version <= before {
		t.Fatalf("result version %d, want above %d", res.Version, before)
	}
}

func TestIdempotencyEvictsOldest(t *testing.T) {
	c := newIdempotency(2, time.Minute)
	c.put("a", WriteResult{IDs: []string{"a"}})
	time.Sleep(time.Millisecond)
	c.put("b", WriteResult{IDs: []string{"b"}})
	time.Sleep(time.Millisecond)
	c.put("c", WriteResult{IDs: []string{"c"}})
	if _, ok := c.get("a"); ok {
		t.Error("oldest key kept past capacity")
	}
	if _, ok := c.get("c"); !ok {
		t.Error("newest key dropped")
	}
	expired := newIdempotency(2, -time.Second)
	expired.put("x", WriteResult{})
	if _, ok := expired.get("x"); ok {
		t.Error("expired key returned")
	}
}
