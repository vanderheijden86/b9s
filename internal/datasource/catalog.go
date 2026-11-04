package datasource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Reachability classifies whether a project database can be read. It is a
// closed set so the UI can never report a down tunnel as refused access, or
// the reverse.
type Reachability uint8

const (
	ReachUnknown Reachability = iota
	ReachReachable
	ReachDenied
	ReachNoIssuesTable
	ReachServerDown
)

// MySQL server error numbers that decide a Reachability.
const (
	mysqlErrDBAccessDenied = 1044
	mysqlErrAccessDenied   = 1045
	mysqlErrNoSuchTable    = 1146
	// Dolt reports a database the user may not read as this generic error,
	// with an "Access denied" message.
	mysqlErrUnknown = 1105
	// mysqlErrUnknownColumn is the only signal that legitimately means "try
	// the older, narrower issues schema" (see doltMissingColumnError).
	mysqlErrUnknownColumn = 1054
)

const catalogQueryTimeout = 10 * time.Second

const trustedDoltEndpointsEnv = "B9S_TRUSTED_DOLT_ENDPOINTS"

func readableProjectDatabases(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("listing Dolt databases: %w", err)
	}
	var candidates []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("reading Dolt database name: %w", err)
		}
		if IsProjectDatabaseName(name) {
			candidates = append(candidates, name)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("listing Dolt databases: %w", err)
	}

	var databases []string
	for _, name := range candidates {
		probe, err := db.QueryContext(ctx, "SELECT 1 FROM `"+name+"`.issues LIMIT 0")
		if err == nil {
			for probe.Next() {
			}
			err = probe.Err()
			probe.Close()
		}
		if err == nil {
			databases = append(databases, name)
			continue
		}
		var mysqlErr *mysql.MySQLError
		if ClassifyConnError(err) == ReachDenied || ClassifyConnError(err) == ReachNoIssuesTable ||
			(errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrUnknownDatabase) {
			continue
		}
		// An incomplete scan must not turn a network outage into project loss.
		return nil, fmt.Errorf("probing Dolt database %s: %w", name, err)
	}
	return databases, nil
}

// IsProjectDatabaseName restricts generated checkout names and SQL identifiers
// to the hosted image's database naming contract.
func IsProjectDatabaseName(name string) bool {
	switch name {
	case "", "information_schema", "mysql", "performance_schema", "sys", "dolt_cluster":
		return false
	}
	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

// ReadableProjectDatabases probes the startup user's current grants without
// selecting a database, so loss of the startup database cannot block discovery.
func ReadableProjectDatabases(ctx context.Context, source DataSource) ([]string, error) {
	dsn, err := serverDSN(source)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening Dolt discovery connection: %w", err)
	}
	defer db.Close()
	return readableProjectDatabases(ctx, db)
}

func (r Reachability) String() string {
	switch r {
	case ReachReachable:
		return "reachable"
	case ReachDenied:
		return "access denied"
	case ReachNoIssuesTable:
		return "no issues table"
	case ReachServerDown:
		return "server unreachable"
	default:
		return "unknown"
	}
}

// ClassifyConnError maps a connection or query error to a Reachability.
// ErrCommentsUnavailable is checked before the MySQL error is unwrapped: it
// can itself wrap an underlying MySQL error (e.g. "no such table: comments"
// -> 1146), which errors.As would otherwise find and misclassify as
// ReachNoIssuesTable, when the source itself is reachable and only its
// comments are unavailable. A server error number is checked next: a
// refused login arrives over a working connection and must not be mistaken
// for a network failure.
func ClassifyConnError(err error) Reachability {
	if err == nil {
		return ReachReachable
	}
	if errors.Is(err, ErrCommentsUnavailable) {
		return ReachReachable
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case mysqlErrDBAccessDenied, mysqlErrAccessDenied:
			return ReachDenied
		case mysqlErrNoSuchTable:
			return ReachNoIssuesTable
		case mysqlErrUnknown:
			if strings.HasPrefix(mysqlErr.Message, "Access denied") {
				return ReachDenied
			}
			return ReachUnknown
		default:
			return ReachUnknown
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, mysql.ErrInvalidConn) ||
		errors.Is(err, driver.ErrBadConn) {
		return ReachServerDown
	}
	return ReachUnknown
}

// doltConnectionConfig builds a driver configuration after checking whether an
// environment credential may be sent to the selected endpoint. Repository
// metadata is untrusted, so remote endpoints require an explicit operator
// allowlist before they can receive BEADS_DOLT_PASSWORD.
func doltConnectionConfig(source DataSource, database string) (*mysql.Config, error) {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = source.Path
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:3306"
	}
	cfg.User = source.User
	if cfg.User == "" {
		cfg.User = "root"
	}
	if !source.Passwordless {
		cfg.Passwd = os.Getenv("BEADS_DOLT_PASSWORD")
	}
	if cfg.Passwd != "" && !isTrustedDoltEndpoint(cfg.Addr) {
		return nil, fmt.Errorf("refusing to send BEADS_DOLT_PASSWORD to untrusted Dolt endpoint %q; add the exact host:port to %s", cfg.Addr, trustedDoltEndpointsEnv)
	}
	cfg.DBName = database
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = catalogQueryTimeout
	return cfg, nil
}

func isTrustedDoltEndpoint(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}

	want := net.JoinHostPort(strings.ToLower(host), port)
	for _, endpoint := range strings.Split(os.Getenv(trustedDoltEndpointsEnv), ",") {
		trustedHost, trustedPort, err := net.SplitHostPort(strings.TrimSpace(endpoint))
		if err != nil {
			continue
		}
		if net.JoinHostPort(strings.ToLower(trustedHost), trustedPort) == want {
			return true
		}
	}
	return false
}

// serverDSN connects to the server without selecting a database. The driver
// formats it, so passwords containing DSN separators survive intact.
func serverDSN(source DataSource) (string, error) {
	cfg, err := doltConnectionConfig(source, "")
	if err != nil {
		return "", err
	}
	return cfg.FormatDSN(), nil
}

// ListProjectDatabases returns the Beads databases the source's user can read,
// identified by having an issues table. System schemas never have one, and the
// server only lists schemas the user holds privileges on.
func ListProjectDatabases(source DataSource) ([]string, error) {
	dsn, err := serverDSN(source)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open Dolt connection: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), catalogQueryTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		"SELECT DISTINCT table_schema FROM information_schema.tables WHERE table_name = 'issues' ORDER BY table_schema")
	if err != nil {
		return nil, fmt.Errorf("cannot list databases on %s: %w", source.Path, err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("cannot read database name: %w", err)
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot list databases on %s: %w", source.Path, err)
	}
	return databases, nil
}
