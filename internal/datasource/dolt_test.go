package datasource

import (
	"os"
	"strings"
	"testing"
)

// doltTestDSN returns the DSN for the Dolt test server from an environment variable.
// When the variable is not set, Dolt integration tests are skipped.
func doltTestDSN() string {
	return os.Getenv("B9S_TEST_DOLT_DSN")
}

// skipIfNoDolt skips the current test when no Dolt server address is configured.
func skipIfNoDolt(t *testing.T) {
	t.Helper()
	if doltTestDSN() == "" {
		t.Skip("B9S_TEST_DOLT_DSN not set, skipping Dolt integration test")
	}
}

// doltSource builds a DataSource from the B9S_TEST_DOLT_DSN environment variable.
// The variable may be a bare host:port address or a full DSN; for the bare address
// form, the addr field is used directly as DataSource.Path.
func doltSource() DataSource {
	dsn := doltTestDSN()
	// If the caller supplied a full DSN (contains "@"), extract the address part.
	// Otherwise treat the whole value as a host:port address.
	addr := dsn
	if idx := strings.Index(dsn, "@tcp("); idx != -1 {
		rest := dsn[idx+len("@tcp("):]
		if end := strings.Index(rest, ")"); end != -1 {
			addr = rest[:end]
		}
	}
	return DataSource{
		Type: SourceTypeDolt,
		Path: addr,
	}
}

// TestDoltReader_NewAndClose verifies that a DoltReader can be opened and closed
// when a live Dolt server is available.
func TestDoltReader_NewAndClose(t *testing.T) {
	skipIfNoDolt(t)

	r, err := NewDoltReader(doltSource())
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// TestDoltReader_LoadIssues verifies that LoadIssues returns without error and
// that each returned issue has a non-empty ID and Title.
func TestDoltReader_LoadIssues(t *testing.T) {
	skipIfNoDolt(t)

	r, err := NewDoltReader(doltSource())
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	defer r.Close()

	issues, err := r.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	for i, issue := range issues {
		if issue.ID == "" {
			t.Errorf("issue[%d] has empty ID", i)
		}
		if issue.Title == "" {
			t.Errorf("issue[%d] (ID=%s) has empty title", i, issue.ID)
		}
	}
	t.Logf("Loaded %d issues from Dolt", len(issues))
}

// TestDoltReader_GetHeadHash verifies that GetHeadHash returns a non-empty
// commit hash string from the Dolt server.
func TestDoltReader_GetHeadHash(t *testing.T) {
	skipIfNoDolt(t)

	r, err := NewDoltReader(doltSource())
	if err != nil {
		t.Fatalf("NewDoltReader failed: %v", err)
	}
	defer r.Close()

	hash, err := r.GetHeadHash()
	if err != nil {
		t.Fatalf("GetHeadHash failed: %v", err)
	}
	if hash == "" {
		t.Error("GetHeadHash returned empty string")
	}
	t.Logf("HEAD hash: %s", hash)
}

// TestDoltReader_ConnectFailure verifies that attempting to connect to a port
// with no Dolt server returns a descriptive error. This test does NOT skip when
// B9S_TEST_DOLT_DSN is unset because it tests failure behaviour, not a live
// server.
func TestDoltReader_ConnectFailure(t *testing.T) {
	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:19999",
	}

	_, err := NewDoltReader(source)
	if err == nil {
		t.Fatal("expected connection error for unreachable port, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "127.0.0.1:19999") {
		t.Errorf("error message should reference the target address; got: %s", errMsg)
	}
	t.Logf("Got expected error: %v", err)
}
