package main

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/internal/datasource"
)

func TestAttachmentSourceInfo_Dolt(t *testing.T) {
	source := datasource.DataSource{
		Type:     datasource.SourceTypeDolt,
		Path:     "127.0.0.1:3306",
		Database: "b9s",
	}
	info, err := attachmentSourceInfo(source, "/repo/.beads", "b9s")
	if err != nil {
		t.Fatalf("attachmentSourceInfo: %v", err)
	}
	want := blobstore.SourceInfo{
		Kind:        blobstore.SourceDolt,
		DoltHost:    "127.0.0.1",
		Database:    "b9s",
		BeadsDir:    "/repo/.beads",
		ProjectName: "b9s",
	}
	if info != want {
		t.Fatalf("attachmentSourceInfo() = %+v, want %+v", info, want)
	}
}

func TestAttachmentSourceInfo_DoltDefaultsMissingDatabase(t *testing.T) {
	source := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "203.0.113.5:3306"}
	info, err := attachmentSourceInfo(source, "/repo/.beads", "b9s")
	if err != nil {
		t.Fatalf("attachmentSourceInfo: %v", err)
	}
	if info.Database != "beads" {
		t.Errorf("Database = %q, want the metadata.json default %q", info.Database, "beads")
	}
	if info.DoltHost != "203.0.113.5" {
		t.Errorf("DoltHost = %q, want the host without its port", info.DoltHost)
	}
}

func TestAttachmentSourceInfo_DoltRejectsUnparsableAddress(t *testing.T) {
	source := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "not-a-host-port"}
	if _, err := attachmentSourceInfo(source, "/repo/.beads", "b9s"); err == nil {
		t.Fatal("attachmentSourceInfo succeeded on an unparsable address")
	}
}

func TestAttachmentSourceInfo_SQLiteAndJSONL(t *testing.T) {
	cases := []struct {
		name string
		typ  datasource.SourceType
		want blobstore.SourceKind
	}{
		{"sqlite", datasource.SourceTypeSQLite, blobstore.SourceSQLite},
		{"jsonl local", datasource.SourceTypeJSONLLocal, blobstore.SourceJSONL},
		{"jsonl worktree", datasource.SourceTypeJSONLWorktree, blobstore.SourceJSONL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := attachmentSourceInfo(datasource.DataSource{Type: c.typ}, "/repo/.beads", "b9s")
			if err != nil {
				t.Fatalf("attachmentSourceInfo: %v", err)
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

func TestAttachmentSourceInfo_UnsupportedType(t *testing.T) {
	_, err := attachmentSourceInfo(datasource.DataSource{Type: "made-up"}, "/repo/.beads", "b9s")
	if err == nil || !strings.Contains(err.Error(), "made-up") {
		t.Fatalf("attachmentSourceInfo err = %v, want it to name the unsupported type", err)
	}
}
