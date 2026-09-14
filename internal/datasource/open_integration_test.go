package datasource

import (
	"errors"
	"os"
	"testing"
)

// A scoped project user can read only its own databases, so asking it for
// another database must be reported as refused access, never as a server that
// cannot be reached. Set B9S_TEST_DOLT_OTHER_DB to a database that user may not
// read.
func TestDoltIntegration_OpenProjectReportsDeniedDatabase(t *testing.T) {
	skipIfNoDoltIntegration(t)
	other := os.Getenv("B9S_TEST_DOLT_OTHER_DB")
	if other == "" {
		t.Skip("set B9S_TEST_DOLT_OTHER_DB to a database the test user may not read")
	}
	source := integrationSource()
	source.Database = other
	source.Priority = PriorityDolt

	_, failure := OpenProject(OpenTarget{Name: other, Dolt: &source})

	if failure == nil {
		t.Fatalf("opening %s succeeded, want access denied", other)
	}
	if failure.Reason != OpenDenied {
		cause := errors.Unwrap(failure.Err)
		for next := errors.Unwrap(cause); next != nil; next = errors.Unwrap(cause) {
			cause = next
		}
		t.Errorf("reason = %d (%s), want denied; error %v (innermost %T: %v)", failure.Reason, failure.Message(), failure.Err, cause, cause)
	}
}
