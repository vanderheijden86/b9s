package datasource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestReadableProjectDatabasesRefreshesCatalog(t *testing.T) {
	names := []string{"alpha"}
	db := sql.OpenDB(catalogTestConnector{query: func(query string) (driver.Rows, error) {
		if query == "SHOW DATABASES" {
			return &catalogTestRows{names: append([]string(nil), names...)}, nil
		}
		return &catalogTestRows{}, nil
	}})
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, want := range [][]string{{"alpha"}, {"alpha", "beta"}, {"beta"}, {}} {
		names = want
		got, err := readableProjectDatabases(ctx, db)
		if err != nil || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("catalog = %v, %v, want %v", got, err, want)
		}
	}
}

func TestReadableProjectDatabasesExcludesNonBeads(t *testing.T) {
	var queries []string
	db := sql.OpenDB(catalogTestConnector{query: func(query string) (driver.Rows, error) {
		queries = append(queries, query)
		if query == "SHOW DATABASES" {
			return &catalogTestRows{names: []string{"mysql", "../escape", "bad`name", "plain", "denied", "gone", "beads"}}, nil
		}
		switch query {
		case "SELECT 1 FROM `plain`.issues LIMIT 0":
			return nil, &mysql.MySQLError{Number: 1146}
		case "SELECT 1 FROM `denied`.issues LIMIT 0":
			return nil, &mysql.MySQLError{Number: 1044}
		case "SELECT 1 FROM `gone`.issues LIMIT 0":
			return nil, &mysql.MySQLError{Number: 1049}
		case "SELECT 1 FROM `beads`.issues LIMIT 0":
			return &catalogTestRows{}, nil
		default:
			t.Fatalf("unsafe or system database probe: %s", query)
			return nil, errors.New("unexpected query")
		}
	}})
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := readableProjectDatabases(ctx, db)
	if err != nil || strings.Join(got, ",") != "beads" || len(queries) != 5 {
		t.Fatalf("catalog = %v, %v, queries=%v", got, err, queries)
	}
}

func TestReadableProjectDatabasesReportsTransientFailures(t *testing.T) {
	for _, failureAt := range []string{"SHOW DATABASES", "SELECT 1 FROM `beads`.issues LIMIT 0"} {
		t.Run(failureAt, func(t *testing.T) {
			db := sql.OpenDB(catalogTestConnector{query: func(query string) (driver.Rows, error) {
				if query == failureAt {
					return nil, context.DeadlineExceeded
				}
				return &catalogTestRows{names: []string{"beads"}}, nil
			}})
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := readableProjectDatabases(ctx, db); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("transient error = %v, want deadline error", err)
			}
		})
	}
}

type catalogTestConnector struct {
	query func(string) (driver.Rows, error)
}

func (c catalogTestConnector) Connect(context.Context) (driver.Conn, error) { return c, nil }
func (c catalogTestConnector) Driver() driver.Driver                        { return c }
func (c catalogTestConnector) Open(string) (driver.Conn, error)             { return c, nil }
func (c catalogTestConnector) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (c catalogTestConnector) Close() error              { return nil }
func (c catalogTestConnector) Begin() (driver.Tx, error) { return nil, errors.New("read only") }
func (c catalogTestConnector) QueryContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.query(query)
}

type catalogTestRows struct{ names []string }

func (r *catalogTestRows) Columns() []string { return []string{"Database"} }
func (r *catalogTestRows) Close() error      { return nil }
func (r *catalogTestRows) Next(dest []driver.Value) error {
	if len(r.names) == 0 {
		return io.EOF
	}
	dest[0], r.names = r.names[0], r.names[1:]
	return nil
}

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
		{
			name: "comments unavailable wrapping a no-such-table error",
			// The nested MySQL 1146 would classify as ReachNoIssuesTable on
			// its own, but the source itself is reachable: only its comments
			// table is missing. ErrCommentsUnavailable must be checked before
			// errors.As unwraps to the MySQL error underneath it.
			err:  fmt.Errorf("%w: query comments: %w", ErrCommentsUnavailable, &mysql.MySQLError{Number: 1146}),
			want: ReachReachable,
		},
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
