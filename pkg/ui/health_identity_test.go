package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/identity"
)

func healthIdentityModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil, "")
	m.width, m.height = 120, 40
	m.activeProjectPath = "/projects/b9s"
	m.actorLookup = stubActorLookup(nil, "vanderheijden86")
	m.identityConfig = datasource.IdentityConfig{SQLUser: "bd_b9s@%"}
	reg, _ := identity.Parse(`[{"name":"andre","kind":"human","aliases":["vanderheijden86"]}]`, "")
	m.setIdentities(reg)
	return m
}

func TestIdentityHealth_SQLLoginDropsHost(t *testing.T) {
	var h DatabaseHealth
	healthIdentityModel(t).fillIdentityHealth(&h)
	if h.SQLLogin != "bd_b9s" {
		t.Fatalf("SQLLogin = %q", h.SQLLogin)
	}
}

func TestIdentityHealth_ResolvesActor(t *testing.T) {
	var h DatabaseHealth
	healthIdentityModel(t).fillIdentityHealth(&h)
	if h.Actor != "andre" || h.ActorKind != identity.KindHuman {
		t.Fatalf("actor = %q (%q)", h.Actor, h.ActorKind)
	}
}

func TestIdentityHealth_ReportsLoadErrorAndConflicts(t *testing.T) {
	m := healthIdentityModel(t)
	m.identityErr = errors.New("b9s.identities: bad kind")
	reg, _ := identity.Parse(`[{"name":"a","kind":"human","aliases":["x"]},{"name":"b","kind":"human","aliases":["x"]}]`, "")
	m.setIdentities(reg)
	var h DatabaseHealth
	m.fillIdentityHealth(&h)
	joined := strings.Join(h.IdentityProblems, "\n")
	if !strings.Contains(joined, "bad kind") || !strings.Contains(joined, `"x"`) {
		t.Fatalf("problems = %v", h.IdentityProblems)
	}
}

func TestHealthModal_ShowsSharedLoginAndIdentity(t *testing.T) {
	m := healthIdentityModel(t)
	m.dbHealth = DatabaseHealth{Backend: "Dolt (MySQL protocol)", Server: "127.0.0.1:3306", Database: "b9s", User: "bd_b9s", Connected: true}
	m.fillIdentityHealth(&m.dbHealth)
	view := stripANSI(m.renderDBHealthModal())
	for _, want := range []string{"SQL login:", "bd_b9s (shared credential)", "You:", "andre (human)"} {
		if !strings.Contains(view, want) {
			t.Errorf("health popup lacks %q:\n%s", want, view)
		}
	}
}

func TestHealthModal_UnmappedActorSaysSo(t *testing.T) {
	m := healthIdentityModel(t)
	m.setIdentities(nil)
	m.dbHealth = DatabaseHealth{Backend: "JSONL (flat file)", FilePath: "/p/issues.jsonl"}
	m.fillIdentityHealth(&m.dbHealth)
	view := stripANSI(m.renderDBHealthModal())
	if !strings.Contains(view, "vanderheijden86 (not in b9s.identities)") {
		t.Fatalf("health popup:\n%s", view)
	}
}
