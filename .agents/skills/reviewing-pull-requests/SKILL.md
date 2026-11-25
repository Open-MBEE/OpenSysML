---
name: reviewing-pull-requests
description: How to review an OpenSysML branch before it is pushed, or a pull request after it is opened, to the standard of the automated review on GitHub — the mechanical gates to run first, how to enumerate the language's edge cases against every changed function, which sibling surfaces must be checked for parity, and the finding format that makes each result actionable.
---

# Reviewing a pull request

Review the branch the way the automated review will, before it does: a finding caught here costs
one commit; the same finding on GitHub costs a review round, a fix, a full CI tier and a second
review. The checklist of what counts is `REVIEW.md` at the repository root; this skill is the
procedure for applying it. Run it on the exact head you are about to push, after the full gate in
`AGENTS.md` §2, and fix everything it finds in that same push.

## 1. Mechanical gates

These produce findings on GitHub only when skipped locally. Every one must be clean:

```bash
git fetch -q origin develop                                 # a stale origin/develop puts merged work in every diff below
gofmt -l .                                                  # prints nothing
go vet ./... && make lint                                   # staticcheck + gosec; dead helpers fail here
git diff --merge-base origin/develop -U2 -- '*.go' | awk \
  '/^-/{next} /^[+ ][[:space:]]*\/\//{n++; a=a||/^\+/; if(n>2&&a) print; next} {n=0;a=0}'
                                                            # prints nothing: no added line leaves a comment over 2 lines
python3 scripts/check-doc-ids.py                            # no work-item labels in reader-facing text
python3 scripts/changelog.py check                          # fragments valid
go test -count=1 ./tests/hygiene                            # size caps, layering, contract tests
```

Repeat the comment check for the other comment syntaxes the diff touches: `#` in Makefiles,
shell, YAML and Python, `;` in Emacs Lisp, `//` in TypeScript and protobuf. A change under
`internal/translate/export` also needs `go run -C tools ./cmd/pilot-apijson-diff -check`, which
fails rather than skips when the pilot exporter is absent, and `go test -C tools ./referee/apijson`;
re-record with `-update` only after adjudicating a moved bucket (`AGENTS.md` §6.2). A library pin
bump needs `make extension-libraries-pin-check` (§6.1).

## 2. Build the change map

```bash
git diff --merge-base origin/develop --stat
git diff --merge-base origin/develop -U20
```

For every changed or added function write one line: its name, the input shape the change was
written for, and every caller (`grep -rn 'fnName(' internal tools client`). The callers are where
a changed contract shows up as a bug. Then list the deleted lines separately: a removed branch,
guard or test is a behaviour change until shown otherwise.

## 3. Enumerate the edge cases per function

For each function in the map, walk the axes in `REVIEW.md` § Critical areas and answer, for each,
what the new code does: inheritance paths (inherited, redefined, subsetted, conjugated, chained),
nameless and nested forms (anonymous, unnamed, prefixed, quoted, parenthesized, cyclic), same-name
collisions, absent versus empty, fallbacks that fall through, limits and truncation accounting,
ordering of memberships, and nil on recorded or library-written entries.

Decide each axis with a model, not a hunch. Write the smallest `.sysml` that puts the function on
that input and run it through the surface the change serves:

```bash
make build
./bin/sysml -validate probe.sysml                 # diagnostics
./bin/sysml -render 'P::v' probe.sysml            # a view change, with -render-form for each renderer
./bin/sysml -convert ttl probe.sysml              # export; then -from ttl -convert sysml for the round trip
printf '%%check P::c\n' | ./bin/sysml probe.sysml   # a REPL command
go test -run 'TestName' ./internal/...            # or a focused test with the probe as fixture
```

An axis that produces a wrong result is a finding. An axis that cannot occur for this function
(the parser rejects it, or a lower tier already errors) is noted as such and dropped. Any other
answer means the probe was not run.

## 4. Check the sibling surfaces

Read `REVIEW.md` § Surfaces that must agree and, for each surface that reads the data the diff
changes, either show the change reaches it or state why it is unaffected. The recurring misses:
a symbol-backed reader updated while the graph-backed one still answers the old way; reflection
changed while RDF export or API JSON still writes the old form; a view rule changed in one renderer
of DOT, Mermaid, PlantUML, D2, draw.io, VS Code and Emacs; a gRPC field added without the client
decoders, the capability preflight, or the older-service path; a flag or command changed without
the man page, `docs/reference/` and the agent skills.

## 5. Check the tests

- Every finding-shaped case from §3 that the change handles has a regression test named for the
  behaviour, as a conformance case, golden or unit test per `AGENTS.md` §5.
- No test was deleted or narrowed while the code it guarded is still live. A replacement that
  checks fewer kinds than the original is a reduction.
- Every golden, trace or baseline diff was read and is explained by the change; `-update` run
  blindly is a finding.
- The PR body's verification section names the gates that actually ran on this head.

## 6. Write the findings

One finding per defect, in the form the automated review uses so they can be compared:

```
**<Title: what is lost or wrong, under eight words>**
When <input the language permits>, `<function>` <does what>. <Why it matters: what a user sees.>

Example: <the smallest model or call that shows it>
Recommended fix: <where the fix belongs, by layer, and what test locks it>
Severity: bug (severe | non-severe) | flag (investigate)
```

A bug is a wrong result you reproduced; severe when it loses or corrupts data, panics, or hangs.
A flag is a path you could not reproduce but could not rule out; say what would settle it. Do not
file style remarks outside `REVIEW.md` § Conventions enforced, and do not file anything in the
files `REVIEW.md` § Ignore lists.

## 7. Fix, then push once

Fix every bug and settle every flag before the push; batch the fixes with the gate so the branch
moves once. A finding whose root cause is outside the PR's scope is still named in the PR body as
a known limitation with its reproducer, not left for the reviewer to discover.
