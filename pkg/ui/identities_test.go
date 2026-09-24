package ui

import (
	"testing"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/identity"
)

func doltModel(db string) Model {
	return NewModel(nil, "").
		WithSourceType(datasource.SourceTypeDolt).
		WithDoltSource(datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", Database: db})
}

func TestIdentitiesLoadedMsg_AppliesRegistry(t *testing.T) {
	m := doltModel("b9s")
	reg, _ := identity.Parse(`[{"name":"andre","kind":"human","aliases":["vanderheijden86"]}]`, "")
	next, _ := m.Update(identitiesLoadedMsg{source: m.doltSource, registry: reg})
	got := next.(Model).Identities().DisplayName("vanderheijden86")
	if got != "andre" {
		t.Fatalf("DisplayName = %q, want andre", got)
	}
}

func TestIdentitiesLoadedMsg_IgnoresOtherProject(t *testing.T) {
	m := doltModel("b9s")
	reg, _ := identity.Parse(`[{"name":"andre","kind":"human","aliases":["vanderheijden86"]}]`, "")
	stale := datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", Database: "other"}
	next, _ := m.Update(identitiesLoadedMsg{source: stale, registry: reg})
	if next.(Model).Identities() != nil {
		t.Fatal("a registry loaded for another project must not apply")
	}
}

func TestIdentitySource_NoneForJSONL(t *testing.T) {
	m := NewModel(nil, "")
	if _, ok := m.identitySource(); ok {
		t.Fatal("a JSONL project has no identity source")
	}
}

func TestLoadIdentitiesCmd_NilWithoutSource(t *testing.T) {
	if cmd := NewModel(nil, "").loadIdentitiesCmd(); cmd != nil {
		t.Fatal("no command expected without a database source")
	}
}
