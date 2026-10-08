package datasource

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// This investigation reads explicitly prepared local copies. It never provisions
// a database or changes the application's reader. See docs/memory-read-research.md.
func researchClient(t *testing.T, mode string) (*GraphPreviewClient, string) {
	t.Helper()
	root := os.Getenv("B9S_MEMORY_RESEARCH_ROOT")
	if root == "" {
		t.Skip("set B9S_MEMORY_RESEARCH_ROOT to the isolated investigation directory")
	}
	if !strings.HasPrefix(root, "/private/tmp/b9s-mu5r-") {
		t.Fatal("research root must be an isolated /private/tmp/b9s-mu5r- directory")
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || real != root {
		t.Fatal("research root must be a canonical local directory")
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "BEADS_") || strings.HasPrefix(key, "BD_") {
			t.Setenv(key, "")
		}
	}
	t.Setenv("BEADS_DOLT_AUTO_START", "0")
	workspace := filepath.Join(root, mode)
	raw, err := os.ReadFile(filepath.Join(workspace, ".beads/metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	meta := researchJSON(t, raw)
	if meta["graph_workspace"] != filepath.Join(workspace, ".beads") || meta["dolt_mode"] != mode ||
		meta["graph_schema_version"] != float64(6) || meta["dolt_database"] != "beads_graph_91e5e718b4215551f61706a070e5c46b" {
		t.Fatal("workspace is not the prepared research snapshot")
	}
	if mode == "server" && (meta["dolt_server_host"] != "127.0.0.1" || meta["dolt_server_port"] != float64(58441) || meta["dolt_server_user"] != "root") {
		t.Fatal("server workspace must use the isolated loopback endpoint")
	}
	client, err := newGraphPreviewClient(workspace, filepath.Join(workspace, ".memory-preview/bin/bd"))
	if err != nil {
		t.Fatal(err)
	}
	return client, root
}

func TestMemoryReadResearchStages(t *testing.T) {
	c, root := researchClient(t, "embedded")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var inventory []byte
	for sample := 0; sample < 4; sample++ {
		for _, args := range [][]string{{"version"}, {"list", "--format", "records-json", "--all"}, {"memories", "--all", "--format", "records-json"}, {"links", "adr-0027", "--json"}} {
			start := time.Now()
			out, err := c.run(ctx, args...)
			if args[0] == "version" && err != nil && strings.Contains(err.Error(), "capability_unavailable") {
				t.Logf("sample=%d stage=startup_refusal wall=%s", sample, time.Since(start))
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("sample=%d stage=%s wall=%s bytes=%d", sample, args[0], time.Since(start), len(out))
			if args[0] == "list" {
				inventory = out
			}
		}
		start := time.Now()
		g, err := c.Graph(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("sample=%d stage=complete_graph wall=%s nodes=%d links=%d", sample, time.Since(start), len(g.Nodes), len(g.Edges))
	}
	if err := os.WriteFile(filepath.Join(root, "inventory.json"), inventory, 0600); err != nil {
		t.Fatal(err)
	}
	inv, err := ParseGraphPreview(bytes.NewReader(inventory))
	if err != nil {
		t.Fatal(err)
	}
	const iterations = 1000
	for _, stage := range []string{"decode", "native_graph", "adr_graph"} {
		beads := append([]GraphBead(nil), inv.Beads...)
		if stage == "native_graph" {
			for i := range beads {
				beads[i].Properties.Body = "Native Memory body without ADR frontmatter."
			}
		}
		start := time.Now()
		for i := 0; i < iterations; i++ {
			switch stage {
			case "decode":
				if _, err := ParseGraphPreview(bytes.NewReader(inventory)); err != nil {
					t.Fatal(err)
				}
			default:
				_ = buildMemoryGraph(beads, inv.Links)
			}
		}
		t.Logf("stage=%s iterations=%d per_snapshot=%s", stage, iterations, time.Since(start)/iterations)
	}
}

type researchRecord map[string]any

func researchJSON(t *testing.T, raw []byte) researchRecord {
	t.Helper()
	var v researchRecord
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// The private CLI and public BDP encode ownership and endpoints differently.
// Canonical IDs, Type URLs, properties and revision tokens remain comparable.
func researchProjection(r researchRecord) researchRecord {
	p := researchRecord{}
	for _, k := range []string{"id", "type", "properties", "source", "target"} {
		if v, ok := r[k]; ok {
			if ref, ok := v.(map[string]any); ok && (k == "source" || k == "target") {
				v = ref["uri"]
			}
			p[k] = v
		}
	}
	p["revision"] = r["revision"]
	if p["revision"] == nil {
		p["revision"] = r["version"]
	}
	var owned []researchRecord
	if links, ok := r["owned"].([]any); ok {
		for _, link := range links {
			owned = append(owned, researchProjection(researchRecord(link.(map[string]any))))
		}
	}
	if groups, ok := r["ownedLinks"].(map[string]any); ok {
		for _, group := range groups {
			for _, link := range group.([]any) {
				owned = append(owned, researchProjection(researchRecord(link.(map[string]any))))
			}
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i]["id"].(string) < owned[j]["id"].(string) })
	p["ownedRecords"] = owned
	return p
}

func TestMemoryReadResearchTransportParity(t *testing.T) {
	c, root := researchClient(t, "server")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	// Fixed loopback investigation endpoint, deliberately unrelated to the
	// shared-server environment or the application's connection discovery.
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:58441)/beads_graph_91e5e718b4215551f61706a070e5c46b?timeout=3s&readTimeout=10s&writeTimeout=10s")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	var schema int
	var workspace string
	if err := db.QueryRowContext(ctx, "SELECT schema_version,workspace FROM graph_preview_scope").Scan(&schema, &workspace); err != nil {
		t.Fatal(err)
	}
	if schema != 6 || workspace != filepath.Join(root, "server/.beads") {
		t.Fatal("SQL endpoint is not the prepared schema-6 investigation copy")
	}
	cli := map[string]researchRecord{}
	out, err := c.run(ctx, "list", "--format", "records-json", "--all")
	if err != nil {
		t.Fatal(err)
	}
	inv := researchJSON(t, out)["result"].(map[string]any)
	if inv["hasMore"] == true || inv["complete"] == false {
		t.Fatal("incomplete CLI inventory")
	}
	for _, item := range inv["items"].([]any) {
		r := researchRecord(item.(map[string]any))
		cli[r["id"].(string)] = r
	}
	beadCount := len(cli)
	// Incident reads supply unowned Links and their complete properties.
	for _, item := range inv["items"].([]any) {
		id := item.(map[string]any)["id"].(string)
		out, err := c.run(ctx, "links", shortBeadID(id), "--json")
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range researchJSON(t, out)["result"].([]any) {
			r := researchRecord(item.(map[string]any))
			cli[r["id"].(string)] = r
		}
	}
	t.Logf("CLI beads=%d links=%d", beadCount, len(cli)-beadCount)
	var sqlRecords map[string]researchRecord
	for sample := 0; sample < 4; sample++ {
		start := time.Now()
		sqlRecords = researchSQLSnapshot(t, ctx, db)
		t.Logf("sample=%d SQL full_snapshot=%s resources=%d", sample, time.Since(start), len(sqlRecords))
	}
	researchEqual(t, "SQL", cli, sqlRecords)
	client := &http.Client{Timeout: 10 * time.Second}
	get := func(path string) researchRecord {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:58442/b9s-memory-poc/"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("BDP %s status=%d err=%v body=%s", path, res.StatusCode, err, raw)
		}
		return researchJSON(t, raw)
	}
	for sample := 0; sample < 4; sample++ {
		start := time.Now()
		bdp := map[string]researchRecord{}
		pages := 0
		for _, collection := range []string{"beads/", "links/"} {
			next := collection + "?limit=10"
			for next != "" {
				if pages >= 100 {
					t.Fatal("BDP continuation did not terminate")
				}
				page := get(next)
				pages++
				for _, item := range page["items"].([]any) {
					r := researchRecord(item.(map[string]any))
					id := r["id"].(string)
					if _, exists := bdp[id]; exists {
						t.Fatalf("duplicate BDP resource %s", id)
					}
					bdp[id] = r
				}
				next = ""
				if v, ok := page["next"].(string); ok {
					const scope = "http://127.0.0.1:8765/b9s-memory-poc/"
					if !strings.HasPrefix(v, scope) {
						t.Fatal("BDP continuation left the snapshot scope")
					}
					next = strings.TrimPrefix(v, scope)
				}
			}
		}
		t.Logf("sample=%d BDP full_snapshot=%s resources=%d pages=%d", sample, time.Since(start), len(bdp), pages)
		researchEqual(t, "BDP", cli, bdp)
	}
	// Retained Memory and Link snapshots use the opaque version token, not
	// Dolt HEAD. Current BDP Read deliberately has no corresponding History API.
	rows, err := db.QueryContext(ctx, `SELECT v.path,v.version,v.snapshot FROM graph_preview_versions v
	 JOIN graph_preview_catalog c ON c.path=v.path
	 WHERE c.revision<>v.version ORDER BY v.path,v.ordinal LIMIT 1001`)
	if err != nil {
		t.Fatal(err)
	}
	type retained struct {
		path, version string
		raw           []byte
	}
	var versions []retained
	for rows.Next() {
		var v retained
		if err := rows.Scan(&v.path, &v.version, &v.raw); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(versions) == 0 {
		t.Fatal("snapshot has no retained historical versions to compare")
	}
	if len(versions) > 1000 {
		t.Fatal("retained version probe exceeds its bound")
	}
	for _, v := range versions {
		out, err := c.run(ctx, "show", v.path, "--version", v.version, "--json")
		if err != nil {
			t.Fatal(err)
		}
		cliRecord := researchRecord(researchJSON(t, out)["result"].(map[string]any))
		sqlRecord := researchJSON(t, v.raw)
		researchEqual(t, "retained SQL", map[string]researchRecord{v.path: cliRecord}, map[string]researchRecord{v.path: sqlRecord})
	}
	t.Logf("historical SQL/CLI parity retained_versions=%d", len(versions))
	encoded, _ := json.MarshalIndent(sqlRecords, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "sql-records.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
}

func researchEqual(t *testing.T, transport string, want, got map[string]researchRecord) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s count %d != %d", transport, len(got), len(want))
	}
	for id, r := range want {
		a, _ := json.Marshal(researchProjection(r))
		b, _ := json.Marshal(researchProjection(got[id]))
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs for %s\nCLI=%s\ngot=%s", transport, id, a, b)
		}
	}
}

// Retained snapshots are an experimental shortcut, not a replacement for the
// preview's current/retained consistency and descriptor validation.
func researchSQLSnapshot(t *testing.T, ctx context.Context, db *sql.DB) map[string]researchRecord {
	t.Helper()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	result := map[string]researchRecord{}
	rows, err := tx.QueryContext(ctx, `SELECT v.snapshot FROM graph_preview_catalog c
	 JOIN graph_preview_versions v ON v.path=c.path AND v.version=c.revision
	 WHERE c.allocation_state='live' AND c.backing<>'issue'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		r := researchJSON(t, raw)
		result[r["id"].(string)] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT c.path,c.type_url,c.revision,v.durable_state,m.owned
	 FROM graph_preview_catalog c JOIN graph_preview_issue_versions m ON m.path=c.path AND m.version=c.revision
	 JOIN issue_versions v ON v.issue_id=m.issue_id AND v.revision=m.issue_revision
	 WHERE c.allocation_state='live' AND c.backing='issue'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var path, typ, version string
		var raw, owned []byte
		if err := rows.Scan(&path, &typ, &version, &raw, &owned); err != nil {
			t.Fatal(err)
		}
		properties := researchJSON(t, raw)
		delete(properties, "dependencies")
		var links []any
		if err := json.Unmarshal(owned, &links); err != nil {
			t.Fatal(err)
		}
		id := "http://127.0.0.1:8765/b9s-memory-poc/" + path
		result[id] = researchRecord{"id": id, "type": typ, "revision": version, "properties": properties, "owned": links}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM graph_preview_catalog WHERE allocation_state='live'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if len(result) != count {
		t.Fatal(fmt.Sprintf("SQL missing retained records: %d/%d", len(result), count))
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMemoryReadResearchConcurrentWriter(t *testing.T) {
	if os.Getenv("B9S_MEMORY_RESEARCH_MUTATE") != "1" {
		t.Skip("set B9S_MEMORY_RESEARCH_MUTATE=1 to add one probe Memory and Link in the isolated copy")
	}
	c, root := researchClient(t, "server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:58441)/beads_graph_91e5e718b4215551f61706a070e5c46b?timeout=3s&readTimeout=10s&writeTimeout=10s")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var workspace string
	if err := db.QueryRowContext(ctx, "SELECT workspace FROM graph_preview_scope").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if workspace != filepath.Join(root, "server/.beads") {
		t.Fatal("refusing writer probe outside isolated copy")
	}
	get := func(path string) researchRecord {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:58442/b9s-memory-poc/"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("BDP read: %v status=%d %s", err, res.StatusCode, raw)
		}
		return researchJSON(t, raw)
	}
	id := fmt.Sprintf("mu5r-concurrent-%d", time.Now().UnixNano())
	if _, err := c.run(ctx, "remember", "Native probe body", "--id", id, "--title", "Concurrent reader probe"); err != nil {
		t.Fatal(err)
	}
	old := get("beads/" + id)
	page := get("beads/?limit=1")
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var before, inside, after string
	var hashBefore, hashAfter string
	if err := tx.QueryRowContext(ctx, "SELECT writer_token FROM graph_preview_scope").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT DOLT_HASHOF_DB()").Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.run(ctx, "link", id, "adr-0027", "--link-type", "types/example-cites", "--id", "links/"+id, "--properties", `{"note":"isolated concurrent writer probe"}`); err != nil {
		t.Fatal(err)
	}
	t.Logf("writer with open SQL read transaction completed=%s", time.Since(start))
	if err := tx.QueryRowContext(ctx, "SELECT writer_token FROM graph_preview_scope").Scan(&inside); err != nil {
		t.Fatal(err)
	}
	if before != inside {
		t.Fatal("read transaction changed snapshot during concurrent writer")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT writer_token FROM graph_preview_scope").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT DOLT_HASHOF_DB()").Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if after == before || hashAfter == hashBefore {
		t.Fatal("writer not detected by token/hash")
	}
	var retained researchRecord
	pages := 0
	for {
		pages++
		for _, item := range page["items"].([]any) {
			r := researchRecord(item.(map[string]any))
			if r["id"] == old["id"] {
				retained = r
			}
		}
		next, ok := page["next"].(string)
		if !ok {
			break
		}
		const scope = "http://127.0.0.1:8765/b9s-memory-poc/"
		if pages > 100 || !strings.HasPrefix(next, scope) {
			t.Fatal("invalid or unbounded continuation")
		}
		page = get(strings.TrimPrefix(next, scope))
	}
	if retained == nil || retained["revision"] != old["revision"] {
		t.Fatal("BDP continuation did not retain initial Memory revision")
	}
	fresh := get("beads/" + id)
	link := get("links/" + id)
	if fresh["revision"] == old["revision"] || researchProjection(link)["source"] != old["id"] {
		t.Fatal("fresh BDP reads did not see owned Link update")
	}
	t.Logf("SQL repeatable snapshot stable; token and working hash changed; BDP retained %d pages while fresh Link/Memory advanced", pages)
}
