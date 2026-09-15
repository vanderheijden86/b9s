package datasource

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
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
		{name: "dolt database access denied", err: &mysql.MySQLError{Number: 1105, Message: "Access denied for user 'bd_b9s'@'%' to database 'LP_Team'"}, want: ReachDenied},
		{name: "other dolt generic error", err: &mysql.MySQLError{Number: 1105, Message: "branch not found"}, want: ReachUnknown},
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
	t.Setenv("B9S_TRUSTED_DOLT_ENDPOINTS", "")

	dsn, err := serverDSN(DataSource{Type: SourceTypeDolt, Path: "127.0.0.1:3306", User: "bd_b9s"})
	if err != nil {
		t.Fatalf("serverDSN: %v", err)
	}

	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q): %v", dsn, err)
	}
	if parsed.User != "bd_b9s" || parsed.Passwd != "p@ss/w:rd?" || parsed.Addr != "127.0.0.1:3306" || parsed.DBName != "" {
		t.Errorf("parsed DSN = user %q pass %q addr %q db %q", parsed.User, parsed.Passwd, parsed.Addr, parsed.DBName)
	}
}

func TestServerDSN_RejectsPasswordForUntrustedEndpoint(t *testing.T) {
	t.Setenv("BEADS_DOLT_PASSWORD", "project-secret")
	t.Setenv("B9S_TRUSTED_DOLT_ENDPOINTS", "")

	_, err := serverDSN(DataSource{Type: SourceTypeDolt, Path: "attacker.example:3306", User: "reader"})
	if err == nil {
		t.Fatal("expected untrusted endpoint to be rejected before authentication")
	}
	if !strings.Contains(err.Error(), "B9S_TRUSTED_DOLT_ENDPOINTS") {
		t.Fatalf("error should explain the explicit trust control, got %v", err)
	}
}

func TestServerDSN_AllowsExplicitlyTrustedEndpoint(t *testing.T) {
	t.Setenv("BEADS_DOLT_PASSWORD", "project-secret")
	t.Setenv("B9S_TRUSTED_DOLT_ENDPOINTS", "db.internal:3306, other.internal:3307")

	dsn, err := serverDSN(DataSource{Type: SourceTypeDolt, Path: "db.internal:3306", User: "reader"})
	if err != nil {
		t.Fatalf("serverDSN: %v", err)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q): %v", dsn, err)
	}
	if parsed.Passwd != "project-secret" || parsed.Addr != "db.internal:3306" {
		t.Fatalf("parsed DSN = pass %q addr %q", parsed.Passwd, parsed.Addr)
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
