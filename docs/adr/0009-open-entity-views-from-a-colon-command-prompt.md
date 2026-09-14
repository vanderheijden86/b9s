---
type: ADR
id: "0009"
title: "Open entity views from a ':' command prompt backed by a closed alias table"
status: active
date: 2026-09-14
---

## Context

k9s opens any resource view by typing `:` and its name (`:pods`, `:deploy`,
`:ns`), with a grey completion of the typed prefix. b9s users wanted the same
for Beads entities (`:epic`, `:feature`, `:task`, `:chore`) and a way to reach
the project list (`:project`, ADR 0008).

b9s already has one query state shared by the tree, list and board views
(ADR 0001), whose `type:` predicate filters by issue type. A separate flat
view per entity would duplicate what that filter already does and fork the
query state the views agree on.

## Decision

**`:` opens a command prompt in every view unless a text field has focus.
Commands resolve through an alias table in code: entity commands (`epic`,
`feature`, `task`, `bug`, `chore`) replace every `type:` predicate in the
shared query and keep the current view, `issues` removes it, and `project`
opens the project table. Unknown input reports `unknown command`. A prefix of
a canonical name shows a grey suggestion that Tab or Right accepts, and an
accepted query stays visible as a compact chip.**

| Command | Aliases |
|---|---|
| `epic` | `epics`, `ep` |
| `feature` | `features`, `feat`, `ftr` |
| `task` | `tasks` |
| `bug` | `bugs` |
| `chore` | `chores` |
| `issues` | `all` |
| `project` | `projects`, `proj` |

## Options considered

- **Type filter in the shared query** (chosen): one query state, every view
  keeps working, and the filter is visible and editable with `/`. Other
  predicates such as `status:open` survive the command.
- **A dedicated flat list per entity**: closer to k9s, but it adds views that
  duplicate the list view and a second place where filtering happens.
- **Aliases in user config**: k9s allows this for custom resources, but the
  Beads issue types are a fixed set. A closed table lets a test check that
  every suggested name resolves, which a config file cannot guarantee.

## Consequences

- `:` typed into the `/` query field is text, not a command, because the query
  guard handles keys first.
- The prompt has no history. k9s binds history to `[` and `]`, which b9s uses
  for resizing the split pane.
- Adding a command means changing code and its tests; a user cannot add one.
- The project table does not follow terminal resizes while it is open.
- Re-evaluate if Beads gains user-defined issue types, which would need
  aliases generated from the schema rather than a fixed table.
