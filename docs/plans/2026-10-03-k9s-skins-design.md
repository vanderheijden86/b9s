# k9s skins in b9s: design

Epic: bd-9zft. Decision record: [ADR 0033](../adr/0033-load-k9s-skins-into-theme-slots.md).

## Goal

A k9s user points b9s at the skin file they already use, and b9s's TUI and web
board take the same colours as k9s.

## Choices made with the user

| Question | Answer |
|----------|--------|
| Scope | TUI and web |
| How a skin is chosen | `ui.skin` in the b9s config: a built-in name or a file path. b9s never reads the k9s config |
| Built-in themes | Sepia and Dracula become built-in skins in k9s format |
| Mapping | Derive every b9s token from k9s slots, plus an optional `b9s:` block in the same file |

One change from the first draft: `Ctrl-T` keeps its three choices (auto, light,
dark) and a skin fills the slot that matches its background, instead of
`Ctrl-T` cycling through every skin. The ADR gives the reasons.

## Components

- `pkg/skin`: `Parse` lays a skin over the stock k9s skin, maps k9s slots to
  `Palette` tokens, applies the `b9s:` block, classifies light or dark, and
  derives tints. `Resolve` accepts a built-in name or a path. Built-ins are
  embedded from `pkg/skin/builtin/`.
- `pkg/ui/palette.go`: `applySkins(light, dark)` sets every `Color*` global and
  every style built from them. `UseSkin` replaces one slot.
- `cmd/b9s`: `applyConfiguredSkin` runs after `config.Load`, before the project
  picker renders. A failure keeps the built-ins and shows in the status bar
  (TUI) or on stderr (`b9s web`).
- `pkg/web/skin.go`: `SkinCSS` renders both slots as custom properties, served
  at `/skin.css`. The appearance sheet reads the skin names from the same file.

## Tests

- `pkg/skin`: upstream k9s skins (dracula, nord, gruvbox-light, solarized-light,
  transparent, stock) parse, classify and fill every token.
- `pkg/ui`: no hex `AdaptiveColor` literal outside `palette.go`; built-in
  contrast; slot replacement; footer names the skin.
- `pkg/web` and `web/tests/skin.spec.ts`: `/skin.css` content, a configured skin
  reaching the browser, and the chooser naming the skins.
- `tests/e2e`: a gruvbox skin in the config paints the terminal through OSC 11.
