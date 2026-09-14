package datasource

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestClassifyConnError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Reachability
	}{
		{name: "no error", err: nil, want: ReachReachable},
		{name: "access denied", err: &mysql.MySQLError{Number: 1045, Message: "Access denied for user 'bd_b9s'"}, want: ReachDenied},
		{name: "database access denied", err: &mysql.MySQLError{Number: 1044}, want: ReachDenied},
		{name: "wrapped access denied", err: fmt.Errorf("cannot connect: %w", &mysql.MySQLError{Number: 1045}), want: ReachDenied},
		{name: "missing issues table", err: &mysql.MySQLError{Number: 1146}, want: ReachNoIssuesTable},
		{name: "dial failure", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, want: ReachServerDown},
		{name: "deadline", err: fmt.Errorf("ping: %w", context.DeadlineExceeded), want: ReachServerDown},
		{name: "anything else", err: errors.New("boom"), want: ReachUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyConnError(tt.err); got != tt.want {
				t.Errorf("ClassifyConnError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestReachabilityReasonsAreDistinct(t *testing.T) {
	seen := map[string]Reachability{}
	for _, r := range []Reachability{ReachUnknown, ReachReachable, ReachDenied, ReachNoIssuesTable, ReachServerDown} {
		reason := r.String()
		if reason == "" {
			t.Errorf("reachability %d has no reason text", r)
		}
		if previous, ok := seen[reason]; ok {
			t.Errorf("reachability %d and %d share reason %q", previous, r, reason)
		}
		seen[reason] = r
	}
}

func TestServerDSN_KeepsSpecialCharactersInPassword(t *testing.T) {
	t.Setenv("BEADS_DOLT_PASSWORD", "p@ss/w:rd?")

	dsn := serverDSN(DataSource{Type: SourceTypeDolt, Path: "127.0.0.1:3306", User: "bd_b9s"})

	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q): %v", dsn, err)
	}
	if parsed.User != "bd_b9s" || parsed.Passwd != "p@ss/w:rd?" || parsed.Addr != "127.0.0.1:3306" || parsed.DBName != "" {
		t.Errorf("parsed DSN = user %q pass %q addr %q db %q", parsed.User, parsed.Passwd, parsed.Addr, parsed.DBName)
	}
}

func TestListProjectDatabases_ReportsServerDownForClosedPort(t *testing.T) {
	_, err := ListProjectDatabases(DataSource{Type: SourceTypeDolt, Path: "127.0.0.1:1", User: "nobody"})

	if err == nil {
		t.Fatal("expected an error for a closed port")
	}
	if got := ClassifyConnError(err); got != ReachServerDown {
		t.Errorf("ClassifyConnError(%v) = %v, want server down", err, got)
	}
}
