// Package identity resolves the names that appear on issues (creator, owner
// email, assignee) to the person, agent or pool behind them. The alias list
// lives in the bd config key b9s.identities; see docs/adr/0014.
package identity

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ConfigKey is the bd config key that holds the alias list.
const ConfigKey = "b9s.identities"

// PoolsConfigKey is bd's own list of claimable group assignees.
const PoolsConfigKey = "claim.pools"

// Kind classifies an identity.
type Kind string

const (
	KindUnknown Kind = ""
	KindHuman   Kind = "human"
	KindAgent   Kind = "agent"
	KindPool    Kind = "pool"
)

func (k Kind) valid() bool {
	return k == KindHuman || k == KindAgent || k == KindPool
}

// Identity is one entry of the alias list.
type Identity struct {
	Name    string   `json:"name"`
	Kind    Kind     `json:"kind"`
	Aliases []string `json:"aliases,omitempty"`
}

// Known reports whether the identity comes from configuration rather than
// from an unlisted raw name.
func (i Identity) Known() bool {
	return i.Kind != KindUnknown
}

// Registry maps aliases to identities. A nil Registry resolves every name to
// itself, which is what a JSONL-only project gets.
type Registry struct {
	identities []Identity
	byCanon    map[string]int
	conflicts  []string
}

// Parse builds a Registry from the raw b9s.identities JSON and the raw
// claim.pools value (comma-separated, as bd parses it). It always returns a
// usable Registry: a broken identity list still leaves the pools in place,
// and the error says what was wrong.
func Parse(identitiesJSON, claimPools string) (*Registry, error) {
	r := &Registry{byCanon: make(map[string]int)}

	var err error
	if strings.TrimSpace(identitiesJSON) != "" {
		var entries []Identity
		if jsonErr := json.Unmarshal([]byte(identitiesJSON), &entries); jsonErr != nil {
			err = fmt.Errorf("%s is not a valid JSON list: %w", ConfigKey, jsonErr)
		} else {
			for i, entry := range entries {
				if entry.Name == "" {
					err = fmt.Errorf("%s entry %d has no name", ConfigKey, i)
					continue
				}
				if !entry.Kind.valid() {
					err = fmt.Errorf("%s entry %q has kind %q; use human, agent or pool", ConfigKey, entry.Name, entry.Kind)
					continue
				}
				r.add(entry)
			}
		}
	}

	for _, pool := range strings.Split(claimPools, ",") {
		pool = strings.TrimSpace(pool)
		if pool == "" {
			continue
		}
		if _, listed := r.byCanon[CanonicalActor(pool)]; listed {
			// An explicit b9s.identities entry already describes this name.
			continue
		}
		r.add(Identity{Name: pool, Kind: KindPool})
	}
	return r, err
}

func (r *Registry) add(entry Identity) {
	idx := len(r.identities)
	r.identities = append(r.identities, entry)
	for _, alias := range append([]string{entry.Name}, entry.Aliases...) {
		canon := CanonicalActor(alias)
		if canon == "" {
			continue
		}
		if prev, taken := r.byCanon[canon]; taken {
			if prev != idx {
				r.conflicts = append(r.conflicts, fmt.Sprintf(
					"alias %q is listed under %q and %q; %q wins",
					alias, r.identities[prev].Name, entry.Name, r.identities[prev].Name))
			}
			continue
		}
		r.byCanon[canon] = idx
	}
}

// Resolve returns the identity behind a name. A name without an entry
// resolves to itself with KindUnknown, and the empty name stays empty.
func (r *Registry) Resolve(alias string) Identity {
	if alias == "" {
		return Identity{}
	}
	if r != nil {
		if idx, ok := r.byCanon[CanonicalActor(alias)]; ok {
			return r.identities[idx]
		}
	}
	return Identity{Name: alias}
}

// DisplayName is Resolve(alias).Name.
func (r *Registry) DisplayName(alias string) string {
	return r.Resolve(alias).Name
}

// Matches reports whether two names denote the same identity.
func (r *Registry) Matches(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return r.Resolve(a).Name == r.Resolve(b).Name
}

// Identities returns the configured identities in configuration order, with
// claim.pools entries last.
func (r *Registry) Identities() []Identity {
	if r == nil {
		return nil
	}
	out := make([]Identity, len(r.identities))
	copy(out, r.identities)
	return out
}

// Conflicts describes aliases listed under more than one identity.
func (r *Registry) Conflicts() []string {
	if r == nil {
		return nil
	}
	return append([]string(nil), r.conflicts...)
}

// CanonicalActor is a port of upstream beads' canonicalActor
// (internal/storage/issueops/identity.go). A run of '.', '_' or '-' is one
// separator and becomes '_', except that an exact "--" encodes '/'. Empty
// stays empty so an unassigned value never matches a person. Keep it in step
// with upstream: b9s must group actors exactly as bd's claim path compares
// them.
func CanonicalActor(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '.' || c == '_' || c == '-' {
			j := i
			for j < len(s) && (s[j] == '.' || s[j] == '_' || s[j] == '-') {
				j++
			}
			if s[i:j] == "--" {
				b.WriteByte('/')
			} else {
				b.WriteByte('_')
			}
			i = j
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}
