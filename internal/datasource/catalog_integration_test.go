package datasource

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"
)

// integrationSource describes the configured test server without a database,
// which is how the :project table connects.
func integrationSource() DataSource {
	host := os.Getenv("B9S_TEST_DOLT_HOST")
	port := os.Getenv("B9S_TEST_DOLT_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("B9S_TEST_DOLT_USER")
	if user == "" {
		user = "root"
	}
	return DataSource{Type: SourceTypeDolt, Path: host + ":" + port, User: user}
}

func trustIntegrationSource(t *testing.T) {
	t.Helper()
	t.Setenv("B9S_TRUSTED_DOLT_ENDPOINTS", integrationSource().Path)
}

// Listing needs only read access, so it runs as a scoped project user: set
// B9S_TEST_DOLT_CATALOG_DB to a Beads database that user can read.
func TestDoltIntegration_ListProjectDatabasesIncludesReadableDatabase(t *testing.T) {
	skipIfNoDoltIntegration(t)
	trustIntegrationSource(t)
	expected := os.Getenv("B9S_TEST_DOLT_CATALOG_DB")
	if expected == "" {
		t.Skip("set B9S_TEST_DOLT_CATALOG_DB to a Beads database the test user can read")
	}

	databases, err := ListProjectDatabases(integrationSource())
	if err != nil {
		t.Fatalf("ListProjectDatabases: %v (reachability: %s)", err, ClassifyConnError(err))
	}

	if !slices.Contains(databases, expected) {
		t.Errorf("databases %v do not include %q", databases, expected)
	}
	for _, system := range []string{"information_schema", "mysql"} {
		if slices.Contains(databases, system) {
			t.Errorf("databases %v include system schema %q", databases, system)
		}
	}
}

func TestDoltIntegration_ListProjectDatabasesReportsDeniedForWrongPassword(t *testing.T) {
	skipIfNoDoltIntegration(t)
	trustIntegrationSource(t)
	t.Setenv("BEADS_DOLT_PASSWORD", fmt.Sprintf("wrong-%d", time.Now().UnixNano()))

	_, err := ListProjectDatabases(integrationSource())

	if got := ClassifyConnError(err); got != ReachDenied {
		t.Errorf("ClassifyConnError(%v) = %v, want denied", err, got)
	}
}
