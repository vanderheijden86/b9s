package identity

import (
	"strings"
	"testing"
)

// These cases follow upstream beads internal/storage/issueops/identity.go so
// b9s groups actors exactly as bd's claim path compares them.
func TestCanonicalActor_MatchesUpstream(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"alice":           "alice",
		"gastown.mayor":   "gastown_mayor",
		"gastown_mayor":   "gastown_mayor",
		"gastown-mayor":   "gastown_mayor",
		"gastown__mayor":  "gastown_mayor",
		"gastown--mayor":  "gastown/mayor",
		"gastown/mayor":   "gastown/mayor",
		"a.-_b":           "a_b",
		"---":             "_",
		"Alice":           "Alice",
		"rig.dog-3":       "rig_dog_3",
		"x---y":           "x_y",
		"ünï.cöde":        "ünï_cöde",
		"trailing.":       "trailing_",
		"--lead":          "/lead",
		"mixed.-name--ok": "mixed_name/ok",
		// Inputs from upstream identity_parity_test.go.
		"gastown.dog-3":   "gastown_dog_3",
		".mayor":          "_mayor",
		"mayor.":          "mayor_",
		"gastown---mayor": "gastown_mayor",
	}
	for in, want := range cases {
		if got := CanonicalActor(in); got != want {
			t.Errorf("CanonicalActor(%q) = %q, want %q", in, got, want)
		}
	}
}

const sampleIdentities = `[
  {"name": "vanderheijden86", "kind": "human", "aliases": ["vanderheijden86@gmail.com", "andre"]},
  {"name": "lane-agents", "kind": "agent", "aliases": ["ubuntu", "BrownBear"]}
]`

func TestResolve_AliasMapsToName(t *testing.T) {
	r, err := Parse(sampleIdentities, "")
	if err != nil {
		t.Fatal(err)
	}
	got := r.Resolve("vanderheijden86@gmail.com")
	if got.Name != "vanderheijden86" || got.Kind != KindHuman {
		t.Fatalf("Resolve(email) = %+v", got)
	}
}

func TestResolve_NameMapsToItself(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	if got := r.Resolve("lane-agents"); got.Name != "lane-agents" || got.Kind != KindAgent {
		t.Fatalf("Resolve(name) = %+v", got)
	}
}

func TestResolve_MatchesCanonicalSpelling(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	if got := r.Resolve("lane.agents"); got.Name != "lane-agents" {
		t.Fatalf("Resolve(lane.agents) = %+v", got)
	}
}

func TestResolve_UnknownAliasResolvesToItself(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	got := r.Resolve("jemanuel")
	if got.Name != "jemanuel" || got.Kind != KindUnknown || got.Known() {
		t.Fatalf("Resolve(unknown) = %+v", got)
	}
}

func TestResolve_EmptyStaysEmpty(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	if got := r.Resolve(""); got.Name != "" {
		t.Fatalf("Resolve(\"\") = %+v", got)
	}
}

func TestResolve_NilRegistryReturnsRawName(t *testing.T) {
	var r *Registry
	if got := r.Resolve("alice"); got.Name != "alice" {
		t.Fatalf("nil registry Resolve = %+v", got)
	}
}

func TestParse_ClaimPoolsBecomePools(t *testing.T) {
	r, err := Parse("", " fable-crew, night-crew ,")
	if err != nil {
		t.Fatal(err)
	}
	for _, pool := range []string{"fable-crew", "night-crew"} {
		if got := r.Resolve(pool); got.Kind != KindPool || got.Name != pool {
			t.Fatalf("Resolve(%q) = %+v", pool, got)
		}
	}
}

func TestParse_ListedPoolIsNoConflict(t *testing.T) {
	r, _ := Parse(`[{"name": "fable-crew", "kind": "pool", "aliases": ["fable"]}]`, "fable-crew")
	if len(r.Conflicts()) != 0 {
		t.Fatalf("a pool listed in both keys is not a conflict: %v", r.Conflicts())
	}
}

func TestParse_DuplicateAliasKeepsFirst(t *testing.T) {
	r, _ := Parse(`[
	  {"name": "alice", "kind": "human", "aliases": ["shared"]},
	  {"name": "bob", "kind": "human", "aliases": ["shared"]}
	]`, "")
	if got := r.Resolve("shared"); got.Name != "alice" {
		t.Fatalf("first entry should win, got %+v", got)
	}
}

func TestParse_DuplicateAliasIsReported(t *testing.T) {
	r, _ := Parse(`[
	  {"name": "alice", "kind": "human", "aliases": ["shared"]},
	  {"name": "bob", "kind": "human", "aliases": ["shared"]}
	]`, "")
	conflicts := r.Conflicts()
	if len(conflicts) != 1 || !strings.Contains(conflicts[0], "shared") ||
		!strings.Contains(conflicts[0], "alice") || !strings.Contains(conflicts[0], "bob") {
		t.Fatalf("conflicts = %v", conflicts)
	}
}

func TestParse_InvalidJSONKeepsPools(t *testing.T) {
	r, err := Parse(`[{"name":`, "fable-crew")
	if err == nil {
		t.Fatal("invalid JSON must return an error")
	}
	if got := r.Resolve("fable-crew"); got.Kind != KindPool {
		t.Fatalf("pools must survive a broken identity list, got %+v", got)
	}
}

func TestParse_UnknownKindIsError(t *testing.T) {
	_, err := Parse(`[{"name": "alice", "kind": "robot"}]`, "")
	if err == nil || !strings.Contains(err.Error(), "robot") {
		t.Fatalf("err = %v", err)
	}
}

func TestParse_MissingNameIsError(t *testing.T) {
	_, err := Parse(`[{"kind": "human", "aliases": ["x"]}]`, "")
	if err == nil {
		t.Fatal("an entry without a name must be an error")
	}
}

func TestIdentities_ListsConfiguredInOrder(t *testing.T) {
	r, _ := Parse(sampleIdentities, "fable-crew")
	var names []string
	for _, id := range r.Identities() {
		names = append(names, id.Name)
	}
	if strings.Join(names, ",") != "vanderheijden86,lane-agents,fable-crew" {
		t.Fatalf("Identities() = %v", names)
	}
}

func TestMatches_AliasOfSameIdentity(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	if !r.Matches("andre", "vanderheijden86") {
		t.Fatal("an alias should match its identity name")
	}
	if r.Matches("andre", "ubuntu") {
		t.Fatal("aliases of different identities must not match")
	}
}

func TestMatches_EmptyNeverMatchesNonEmpty(t *testing.T) {
	r, _ := Parse(sampleIdentities, "")
	if r.Matches("", "andre") {
		t.Fatal("an unassigned value must not match a person")
	}
}
