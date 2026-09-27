package blobstore

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/datasource"
)

func TestSourceInfoFromDataSource_Dolt(t *testing.T) {
	source := datasource.DataSource{
		Type:     datasource.SourceTypeDolt,
		Path:     "127.0.0.1:3306",
		Database: "b9s",
	}
	info, err := SourceInfoFromDataSource(source, "/repo/.beads", "b9s")
	if err != nil {
		t.Fatalf("SourceInfoFromDataSource: %v", err)
	}
	want := SourceInfo{
		Kind:        SourceDolt,
		Database:    "b9s",
		BeadsDir:    "/repo/.beads",
		ProjectName: "b9s",
	}
	if info != want {
		t.Fatalf("SourceInfoFromDataSource() = %+v, want %+v", info, want)
	}
}

func TestSourceInfoFromDataSource_DoltDefaultsMissingDatabase(t *testing.T) {
	// Path is not parsed at all: a Dolt server reachable only as "::1" (no
	// port split needed here, but a real host can be) must not fail just
	// because SourceInfo no longer has anywhere to put a host.
	source := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "::1"}
	info, err := SourceInfoFromDataSource(source, "/repo/.beads", "b9s")
	if err != nil {
		t.Fatalf("SourceInfoFromDataSource: %v", err)
	}
	if info.Database != "beads" {
		t.Errorf("Database = %q, want the metadata.json default %q", info.Database, "beads")
	}
}

func TestSourceInfoFromDataSource_SQLiteAndJSONL(t *testing.T) {
	cases := []struct {
		name string
		typ  datasource.SourceType
		want SourceKind
	}{
		{"sqlite", datasource.SourceTypeSQLite, SourceSQLite},
		{"jsonl local", datasource.SourceTypeJSONLLocal, SourceJSONL},
		{"jsonl worktree", datasource.SourceTypeJSONLWorktree, SourceJSONL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := SourceInfoFromDataSource(datasource.DataSource{Type: c.typ}, "/repo/.beads", "b9s")
			if err != nil {
				t.Fatalf("SourceInfoFromDataSource: %v", err)
			}
			if info.Kind != c.want {
				t.Errorf("Kind = %q, want %q", info.Kind, c.want)
			}
			if info.BeadsDir != "/repo/.beads" || info.ProjectName != "b9s" {
				t.Errorf("info = %+v", info)
			}
			if info.Database != "" {
				t.Errorf("info = %+v, want no Database for a non-Dolt source", info)
			}
		})
	}
}

func TestSourceInfoFromDataSource_UnsupportedType(t *testing.T) {
	_, err := SourceInfoFromDataSource(datasource.DataSource{Type: "made-up"}, "/repo/.beads", "b9s")
	if err == nil || !strings.Contains(err.Error(), "made-up") {
		t.Fatalf("SourceInfoFromDataSource err = %v, want it to name the unsupported type", err)
	}
}
