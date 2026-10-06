---
type: ADR
id: "0035"
title: "Take hotkeys from a k9s-style hotkeys.yaml"
status: active
date: 2026-10-06
---

## Context

ADR 0033 let users bind keys in a `keybindings:` list inside `config.yaml`. Each entry named a `key` in b9s spelling (`ctrl+t`) and either a `query` or one of 26 built-in `action` names. ADR 0034 kept that list and added the `type_*` actions for the function keys.

b9s is modelled on k9s, and its users know k9s's config. The owner asked for the shortcut config to match k9s as closely as possible. k9s keeps user shortcuts in a separate `hotkeys.yaml`:

```yaml
hotKeys:
  shift-0:
    shortCut: Shift-0
    description: Viewing pods
    command: pods app=kindnet
    override: false
    keepHistory: false
```

A k9s hotkey runs a `:` command, and the command may carry a filter after the resource name. k9s has no action table: a hotkey is a saved command. Aliases (`aliases.yaml`), external plugins (`plugins.yaml`) and live reload are separate k9s features, filed as separate work (bd-w7ju) and left out here.

The `keybindings:` list was never in a release: no tag contains it. Nobody has a config to migrate.

## Decision

**User shortcuts live in `~/.config/b9s/hotkeys.yaml` in the k9s shape: a `hotKeys` map of name to `shortCut`, `description`, `command`, `override` and `keepHistory`. `shortCut` uses the k9s key syntax, and `command` is a `:` command. The `keybindings:` section of `config.yaml`, its `action` bindings and the action table are removed.**

- `shortCut` accepts `F1`-`F12`, `Ctrl-<letter>`, `Alt-<character>`, `Shift-<letter or digit>` and one character, with modifier names in any case. `Shift-<digit>` means the character a US keyboard sends (`Shift-0` is `)`), because a terminal reports the character and never the shift.
- A type command (`epic`, `feature`, `task`, `bug`, `chore`) and `issues` take query terms after the name, as `:pods app=kindnet` does in k9s. With terms, the command replaces the whole shared query (ADR 0001): `epic status:open` gives `status:open type:epic`. Without terms, it keeps its existing behaviour of swapping only the type term. The prompt accepts the same text, so a hotkey is exactly a saved `:` line.
- A hotkey does not toggle. Pressing it again sets the same query, as in k9s. `Esc` clears the query.
- `keepHistory` is accepted so a k9s entry loads unchanged. It has no effect, because the `:` prompt keeps no history.
- The key rules of ADR 0033 stay: reserved keys can never be bound, a built-in key (including `F1`-`F4` from ADR 0034) needs `override: true`, and one key has one hotkey. Errors name the entry as `hotKeys.<name>`.
- An unknown top-level key, an unknown field (`shortcut` for `shortCut`), an unknown command, an unknown query field or a broken rule stops startup with exit code 2 and lists every problem. A missing file defines no hotkeys.
- The file is read once at startup.

## Options considered

- **k9s `hotkeys.yaml`, commands only** (chosen): a k9s user's file works after changing the command, and there is one way to describe what a key does, the `:` command line.
- **k9s shape plus an `action` field**: keeps rebinding of `s`, `e`, `K` and the like, but invents a field k9s does not have, and keeps a 26-name table that must track every key handler.
- **Keep `keybindings:` in `config.yaml`, accept k9s key syntax**: smallest change, but the file, shape and field names still differ from k9s, which is what the owner asked to remove.
- **Load both files, migrate `keybindings:`**: there are no users of the old section, so migration code would protect nobody.

## Consequences

A k9s user can bind b9s keys from memory. Saved views such as "open bugs labelled ui" are one line, and the same text works at the `:` prompt.

Built-in actions such as sort, edit or the board can no longer be moved to another key. Views become bindable when `:` gets view commands (bd-w7ju.1.1). The `type_*` actions of ADR 0034 are gone with the table, and F1-F4 themselves are unchanged.

Re-evaluate when aliases, plugins or live reload land (bd-w7ju.2, bd-w7ju.3, bd-w7ju.4), or if users ask to rebind built-in actions.

This ADR supersedes the keybindings part of ADR 0033 and the `type_*` action bullet of ADR 0034. The function keys of ADR 0034 stay in force.
