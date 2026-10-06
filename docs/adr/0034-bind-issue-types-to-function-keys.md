---
type: ADR
id: "0034"
title: "Bind issue types to function keys F1-F4"
status: active
date: 2026-10-06
---

## Context

ADR 0033 put a type quick filter on `Y`: `Y` switched the header panel to the five types and a digit toggled a `type:<name>` term. Showing epics took two keys and a look at the panel to find the digit, and the digits meant something else again in the label, assignee and project panels.

The owner wants the k9s shape instead, where each resource kind (pods, deployments, routes) has its own function key and one press switches to it. On GitHub issue #14 the reporter agreed. `F1` was a second help key next to `?`, and nothing else used `F2`-`F4`.

The keybindings half of ADR 0033 (the `keybindings:` list in `config.yaml`, its query and action bindings and its conflict rules) is unchanged and stays in force through this ADR.

## Decision

**`F1` shows only epics, `F2` features, `F3` tasks and `F4` bugs. The key replaces every `type:` term in the shared query with its own and keeps the other terms; when its type is already the only type term, the key removes it. Help is on `?` only. The `Y` panel is removed.**

- The state stays the query text (ADR 0001), so the filter shows in the query bar, works in list, tree and board, and `Esc` clears it.
- One type at a time, not a toggle set: a function key switches what is shown, as in k9s. Several types together are still possible by typing `type:epic type:bug` after `/`.
- Chore has no key. It is the least used type, and `F5` is already reload.
- The header's type legend shows the key beside each type, and the footer shows `F1-4 type`.
- The actions `type_epic`, `type_feature`, `type_task` and `type_bug` replace `type_picker`, so a user can move the types to other keys in `config.yaml`. `f2`-`f4` join the built-in keys, so binding one needs `override: true`.
- The footer guard (`TestDetailFooterKeysAllWork`) presses `F1` for the `F1-4` hint.

## Options considered

- **One function key per type, replace semantics** (chosen): one press, matches k9s, nothing to learn from a panel.
- **Function keys that toggle terms** (types combine): keeps ADR 0033's alternatives, but a second press of another key would add rather than switch, which is not what a resource key does in k9s.
- **Keep `Y` beside the function keys**: two ways to do one thing, and the panel steals the header from projects.
- **Keep `F1` as help and use `F2`-`F5`**: `F5` is reload, and the owner asked for `F1` = epic. Most terminal apps that use function keys for content (k9s, mc) do not reserve `F1`.

## Consequences

Switching types is one key in every main view. A user who pressed `F1` for help must press `?`. A config with `type_picker` fails at startup with an unknown action, which names the fix. Terminals that capture function keys (some macOS setups send `F1`-`F4` to the system) need `fn` or a config binding to another key.

Re-evaluate if users ask for a key for chore or for combined types on one key.
