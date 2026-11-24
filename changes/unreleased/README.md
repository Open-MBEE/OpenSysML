# Changelog fragments

A change describes itself here instead of editing `CHANGELOG.md`, so two branches that both
add an entry never touch the same lines. One file per change:

```
changes/unreleased/<slug>.<section>.md
```

- `<slug>` names the change — the branch or topic, e.g. `repl-send-signal`.
- `<section>` is the changelog heading, lower-cased: `added`, `changed`, `deprecated`,
  `removed`, `fixed`, `security` or `performance`.
- The body is the entry exactly as it will appear under that heading: one or more list items
  in the changelog's own style (`- **What changed, as a sentence.** Why, and what a reader
  will observe.`). No heading — that comes from the file name.

`python3 scripts/changelog.py check` validates every fragment and runs in CI. When a release is
prepared, `python3 scripts/changelog.py render` folds them into the `## Unreleased` section of
`CHANGELOG.md` and deletes them; see `docs/project/releasing.md`.
