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
)

const catalogQueryTimeout = 10 * time.Second

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
// A server error number is checked first: a refused login arrives over a
// working connection and must not be mistaken for a network failure.
func ClassifyConnError(err error) Reachability {
	if err == nil {
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

// serverDSN connects to the server without selecting a database. The driver
// formats it, so passwords containing DSN separators survive intact.
func serverDSN(source DataSource) string {
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
	cfg.Passwd = os.Getenv("BEADS_DOLT_PASSWORD")
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = catalogQueryTimeout
	return cfg.FormatDSN()
}

// ListProjectDatabases returns the Beads databases the source's user can read,
// identified by having an issues table. System schemas never have one, and the
// server only lists schemas the user holds privileges on.
func ListProjectDatabases(source DataSource) ([]string, error) {
	db, err := sql.Open("mysql", serverDSN(source))
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
