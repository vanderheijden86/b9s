package datasource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests that create, fill or drop databases run only against a disposable
// local Dolt server named in B9S_TEST_DOLT_SCRATCH_ADDR. Every project's Beads
// data lives on the shared server, which is reached through the local tunnel
// on port 3306, so neither that port nor a remote host is ever accepted.
const scratchAddrEnv = "B9S_TEST_DOLT_SCRATCH_ADDR"

var errNoScratchServer = errors.New(scratchAddrEnv + " is not set")

// sharedServerTunnelPort is where the local SSH tunnel exposes the shared
// Dolt server.
const sharedServerTunnelPort = "3306"

// checkScratchAddr refuses any address that could reach shared Beads data.
func checkScratchAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("want host:port: %w", err)
	}
	if port == sharedServerTunnelPort {
		return fmt.Errorf("port %s is the tunnel to the shared Dolt server", port)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("%s is not a local address", host)
}

func scratchServerUser() string {
	if user := os.Getenv("B9S_TEST_DOLT_SCRATCH_USER"); user != "" {
		return user
	}
	return "root"
}

// requireScratchServer returns the disposable server address, skipping the
// test when none is configured and failing it when the address is unsafe.
func requireScratchServer(t *testing.T) string {
	t.Helper()
	addr := os.Getenv(scratchAddrEnv)
	if addr == "" {
		t.Skipf("%s: set it to a disposable local Dolt server to run tests that write", errNoScratchServer)
	}
	if err := checkScratchAddr(addr); err != nil {
		t.Fatalf("refusing to write to %s: %v", addr, err)
	}
	return addr
}

func TestCheckScratchAddrRefusesSharedOrRemoteServers(t *testing.T) {
	for _, addr := range []string{
		"127.0.0.1:3306",
		"localhost:3306",
		"db.example.com:3307",
		"[2a01:4f8:13a:643::2]:3307",
		"195.201.110.43:3307",
		"127.0.0.1",
	} {
		if err := checkScratchAddr(addr); err == nil {
			t.Errorf("checkScratchAddr(%q) accepted an address that may reach shared data", addr)
		}
	}
}

func TestCheckScratchAddrAcceptsLocalDisposableServer(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:13306", "localhost:3310", "[::1]:3307"} {
		if err := checkScratchAddr(addr); err != nil {
			t.Errorf("checkScratchAddr(%q) = %v, want accepted", addr, err)
		}
	}
}

// TestDatabaseCreatingTestsRequireScratchServer fails when a test function in
// this package or the E2E suite creates a database, with SQL or through
// `bd init --server`, without first calling requireScratchServer. Each package
// defines its own requireScratchServer with the same refusals.
func TestDatabaseCreatingTestsRequireScratchServer(t *testing.T) {
	var files []string
	for _, pattern := range []string{"*_test.go", "../../tests/e2e/*_test.go"} {
		matched, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matched...)
	}
	if len(files) == 0 {
		t.Fatal("no test files found to check")
	}
	fset := token.NewFileSet()
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "requireScratchServer" {
				continue
			}
			body := string(src[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
			createsDatabase := strings.Contains(body, "CREATE DATABASE") || strings.Contains(body, `"--server"`)
			if createsDatabase && !strings.Contains(body, "requireScratchServer(t)") {
				t.Errorf("%s: %s creates a database without requireScratchServer(t)", name, fn.Name.Name)
			}
		}
	}
}
