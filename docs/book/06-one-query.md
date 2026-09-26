# One query

Press `/` and type. Whatever you type filters the tree. Press `b` and the board shows the same filtered set. Press `g` and the graph does too. There is one query in b9s, and every view reads it.

That sentence is decision record 0001, the first one written, and it was written because the alternative had already happened. The earlier program had a filter on the tree and a search on the board, with slightly different syntax and slightly different matching, and a user who switched views lost their place. The fix was structural: the root model owns one query string, parses it once into an immutable `IssueQuery`, and every view derives its rows from that.

## The language

The query language is small and closed. Its whole grammar fits in a paragraph.

Plain words match fuzzily against the issue ID, the title and the labels, and nothing else. `mob web` finds "Mobile web UI". `foit` finds `bd-foit` and its children, because the ID is a field. A word does not search the description, the comments or the assignee, and that restriction is decision record 0004: a hit must be explained by data visible in its row. A fuzzy match on a paragraph of description is a result you cannot see the reason for, and a result you cannot see the reason for feels like a bug.

Predicates target a field: `status:open`, `priority:1`, `type:epic`, `label:lane-stage`, `assignee:andre`, `project:b9s`, `id:foit`, `title:web`. Two values in one field combine with OR, so `status:open,in_progress` is either. Different fields combine with AND. A leading `!` negates: `!status:closed`. Priority is exact, so `p1` never matches `p10`, and an incomplete predicate such as `status:` adds no constraint rather than matching nothing, so the list does not empty out while you are still typing.

Type filters have `:` commands as shortcuts. `:epic` is `type:epic`, `:bug` is `type:bug`, and `:issues` clears the type. They set the same query.

## Parsing once

`ParseIssueQuery` in `pkg/ui/query_state.go` turns the raw string into a list of predicates. It is a pure function: same string in, same query out, no reference to any view. The result is immutable. A view cannot add a predicate of its own, and that is the point.

```
   "/" field ──▶ raw string ──▶ ParseIssueQuery ──▶ IssueQuery
                                                       │
                       ┌───────────────┬───────────────┼─────────────┐
                       ▼               ▼               ▼             ▼
                     tree            board           graph        b9s web
```

The last arrow is worth a second look. The browser app sends its query string to `GET /api/query`, and the server runs the same `ParseIssueQuery` on it. There is no second query language in TypeScript, no attempt to keep two parsers in step. When the grammar grows, the phone gets the new grammar on the next build, for free.

## Matching in memory

The query runs against the loaded issues in memory. There is no round trip to the store. That is what makes the field feel instant: every keystroke re-runs the match over a few thousand issues and re-renders, well inside one frame.

It is also what keeps the matching honest. A SQL `LIKE` and a Go fuzzy matcher would not agree on edge cases, and a user who saw different results on the tree and in the project table would be right to complain. One matcher, one place, one result.

## The focused field

The other half of decision record 0002 is about focus. While the query field is being edited, it takes every key. That was covered in the last chapter as a routing rule, but it is worth seeing why it is a query decision as much as a keys decision.

k9s filters the same way: `/` opens a field, typing filters, `Enter` keeps the filter and returns focus to the table, `Esc` clears it. Users who arrive from k9s expect exactly that, and users who do not arrive from k9s still expect that typing a word does not fire actions. The rule that makes both true is the simplest one: while the field has focus, the field has every key.

## What the query is not

The query is not the sort. The sort comes from the user's configuration, chapter thirteen, and never from project state. The query is not the fold state either. Folding an epic hides its children in the tree and only in the tree. And the query is not persisted: it lives in the model for the length of the session and starts empty next time, except when `--filter` sets one on the command line.
