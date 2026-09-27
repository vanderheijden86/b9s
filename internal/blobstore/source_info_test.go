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
		DoltHost:    "127.0.0.1",
		Database:    "b9s",
		BeadsDir:    "/repo/.beads",
		ProjectName: "b9s",
	}
	if info != want {
		t.Fatalf("SourceInfoFromDataSource() = %+v, want %+v", info, want)
	}
}

func TestSourceInfoFromDataSource_DoltDefaultsMissingDatabase(t *testing.T) {
	source := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "203.0.113.5:3306"}
	info, err := SourceInfoFromDataSource(source, "/repo/.beads", "b9s")
	if err != nil {
		t.Fatalf("SourceInfoFromDataSource: %v", err)
	}
	if info.Database != "beads" {
		t.Errorf("Database = %q, want the metadata.json default %q", info.Database, "beads")
	}
	if info.DoltHost != "203.0.113.5" {
		t.Errorf("DoltHost = %q, want the host without its port", info.DoltHost)
	}
}

func TestSourceInfoFromDataSource_DoltRejectsUnparsableAddress(t *testing.T) {
	source := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "not-a-host-port"}
	if _, err := SourceInfoFromDataSource(source, "/repo/.beads", "b9s"); err == nil {
		t.Fatal("SourceInfoFromDataSource succeeded on an unparsable address")
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
			if info.DoltHost != "" || info.Database != "" {
				t.Errorf("info = %+v, want no Dolt fields for a non-Dolt source", info)
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
