# Deciding in writing

Every chapter so far has pointed at a decision record. There are twenty-one of them in `docs/adr/`, numbered `0001` to `0021`, and this chapter is about the practice rather than any one decision, because the practice is the part of the design that made the rest possible.

## What a record is

A record is one file, one decision. It has a status, a date, a context section that says what situation forced the choice, a decision in bold, the options that were considered with their trade-offs, and the consequences. Where the decision is about interaction, who calls whom and what happens on failure, it carries a small sequence diagram, because a reviewer spots a wrong arrow faster than a wrong sentence.

The full list reads as a history of the program:

| Record | Decision |
|--------|----------|
| 0001 | one query state across the TUI views |
| 0002 | global fuzzy matching with a focused search field |
| 0003 | capture the mouse wheel, leave selection to the terminal |
| 0004 | plain search matches ID, title and labels only |
| 0005, 0006 | the first mobile bridge, through tmux and Tailscale paths |
| 0007 | one SELECT-only account reads the shared catalog |
| 0008 | recent projects in the header, not discovered folders |
| 0009 | entity views from a `:` prompt with a closed alias table |
| 0010 | the tree sort comes from user config, never project state |
| 0011 | bulk actions act on marks, else on the cursor row |
| 0012 | open a project before replacing the visible one |
| 0013 | trust Dolt endpoints before sending the password |
| 0014 | map actors to identities in `bd` config |
| 0015 | hide recent projects the startup user cannot read |
| 0016, 0017, 0018 | the board: two layouts, then one, then the epic rail |
| 0019 | a mobile web UI from `b9s web` with an embedded app |
| 0020 | pair every browser with a persistent token |
| 0021 | let a login proxy's email header pair the owner |

## Immutable once active

An active record is never edited. When a decision changes, a new record with the next number says which one it supersedes, and the old one gets exactly one edit: its status becomes `superseded` and a field names the successor. Records 0016 and 0017 are superseded by 0018, and both still say what they said when they were written.

The reason is the audit trail. A record that is rewritten to match the present tells you what is true now, which the code already tells you. A record that is frozen at its date tells you what was believed then, which nothing else does. When someone proposes epic chips for the board, the value is not in knowing that the board has a rail. It is in reading that chips were built, used for two weeks and removed, and reading why in the words of the person who removed them.

## Records versus the architecture document

`docs/ARCHITECTURE.md` describes the current state and links to the records behind each part. It is edited freely and is always current. The records are immutable and each covers one moment. The two are different documents with different jobs, and a repository that has only one of them is missing half of its memory.

This book is a third kind of document. It is a narrative, it will go stale, and it says so in its preface.

## Configuration is a decision too

Record 0010 is the smallest decision in the set and a good example of why small ones get written down. **The tree sort comes from the user's config, never from project state.** The alternative, remembering a sort per project, seemed friendly and turned out to be confusing: the same key sorted two projects differently and nobody could remember having asked for that.

`pkg/config` reads `~/.config/b9s/config.yaml`: sort defaults, the poll interval, the recent projects, `lock_recent`. The sort field names in `config.go` must stay in step with `ui.SortField`, and a test in `pkg/ui` checks that they do. An unknown field or direction makes the whole file fail to load, and a file that fails to load is ignored as a whole and never overwritten, so a typo costs you your settings for a session and never costs you the file.

## Closed sets, everywhere

If there is one habit that runs through every record, it is the closed set. The `:` prompt has a closed alias table. Reachability is a closed enum from MySQL error numbers. `OpenReason` is a closed set shared by the CLI and the TUI. The project switch has two states. The web write endpoint accepts a closed set of operations. The board's `v` cycles two designs.

A closed set is one you can test in full, render in a table, and reason about without reading the code that consumes it. The alternative, a string that means something to the function that made it, is the shape of most bugs the records were written to prevent. Where b9s has been extended, it has been extended by adding a value to a set, and the compiler or a table-driven test says where else that value needs handling.

## Writing one

A new record is due when a change embodies a choice with real trade-offs: a new abstraction, a reversal, a load-bearing decision whose reasoning would otherwise live only in a code comment. Routine changes do not get one. The test is whether a contributor in a year would want to know why, and whether the code alone would tell them. If the answer is yes and no, write it down.
