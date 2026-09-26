# People

An issue has two people on it. The creator is fixed at creation: `created_by` holds a name and `owner` holds the git email behind it. The assignee changes, and it is the field most often edited.

Both fields hold names, and the names are not tidy. The same person appears as `andre`, `vanderheijden86`, their email and the name of an agent acting for them. A pool such as `agents` stands for several. b9s has to show one person per person, offer one entry per person in the assignee picker, and know who *you* are when you press `Ctrl-N` to create an issue as yourself.

## The registry

`pkg/identity` is the answer. It reads two things from the `bd` config table: `b9s.identities`, an alias list that b9s owns, and `claim.pools`, which `bd` already uses for its own claim logic. It parses them into a `Registry` that maps every alias to one identity, and it compares names the same way `bd`'s claim path does, so a name that `bd` treats as yours is one b9s treats as yours.

```
   bd config table ──▶ LoadIdentityConfig ──▶ identity.Parse ──▶ Registry
                                                                    │
        assignee picker, filters, suggestions ◀─────────────────────┤
        Ctrl-N actor (BEADS_ACTOR > BD_ACTOR > git > USER) ◀────────┤
        health popup: SQL login, You:, conflicts ◀──────────────────┘
```

Decision record 0014 put the alias list in `bd` config rather than in `~/.config/b9s/config.yaml`, and the reason is scope. Identities belong to a project and to everyone who works on it. A mapping kept in one person's config would have to be copied to every laptop and every agent container, and would drift. A mapping in the project's own config table travels with the project and is read by every b9s that opens it.

## Loading off the loop

The registry needs a query against the config table, so it loads as a `tea.Cmd`, off the update loop, when a Dolt project opens and again on every reload. `pkg/ui/identities.go` drops a load whose project no longer matches the current one, using the same generation idea as the project switch. A registry from project `2` must not describe the names in project `3`.

## Who you are

`Ctrl-N` opens the create form with you as the assignee, and the issue is created as you. Which name is "you" follows a fixed order: `BEADS_ACTOR`, then `BD_ACTOR`, then the git user, then `USER`. The first one set wins, and the registry resolves it to an identity so the assignee picker highlights your row.

The Dolt SQL login is deliberately not on that list. It is the workspace credential every agent and every human shares to reach the server, so it names a team, not a person. The health popup shows it as the login, shows the resolved `You:` beside it, and lists any alias that resolves to two identities as a conflict to fix in config.

## What this buys

The assignee picker shows people, not strings. Filters such as `assignee:andre` match every alias of that person. Suggestions in the edit form come from the registry rather than from whatever spellings happen to be in the current issues. And the same registry serves the browser app, because `b9s web` loads it into its store the same way the TUI loads it into the model.
