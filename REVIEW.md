# Review Guidelines

How a pull request to OpenSysML is reviewed, by a person, by an agent before it pushes, and by
the automated review on GitHub. The same checklist serves all three, so a change that passes it
locally should arrive with nothing left to find. The procedure an agent follows to apply it is
`.agents/skills/reviewing-pull-requests/SKILL.md`; the coding rules it enforces are in `AGENTS.md`.

## What a finding is

A finding names an input the language permits, the function that mishandles it, and the wrong
result a user sees: "when a constraint redefines its owner's bound attribute, `fixedFeatures.add`
discards the constraint's binding, so `%check` reports satisfiable against the declared value".
A review that cannot fill in all three parts has an impression, not a finding; keep looking or let
it go. Style remarks are limited to the conventions below.

## Critical areas

Every layer has the same failure shape: the change handles the declaration it was written for and
not the other ways SysML v2 lets the same thing be said. For each changed function, walk these
axes and ask what the new branch does on each:

- **Inheritance paths.** Inherited, redefined, subsetted and conjugated features; a feature
  reached through a chain; a usage subsetting another usage; an exhibit or perform of an inherited
  behaviour; a redefinition that supplies no new ends. The declared element is the easy case.
- **Nameless and nested forms.** Anonymous connectors, interfaces and bindings; unnamed ends;
  prefixed (`#Tag`) declarations; quoted names with escapes (`#'Owner\'s Level'`); parenthesized
  and bare body arguments; a nested or cyclic declaration; a usage with both a written range and
  body members.
- **Same-name collisions.** A feature named like its owner's, like an inherited default, like a
  Python/Go attribute the client adds, or like a query keyword (`body`, `locale`, `language`);
  two structures that both allocate `sym:0`; two parameters that hash to one key.
- **Absent versus empty.** An empty doc body, an empty collection and an unanswered read are three
  different values; a count of zero and a withheld count are not the same. Do not report a
  default or a fallback as a stated value.
- **Fallbacks and fall-throughs.** A branch that falls back to the previous behaviour when the new
  condition is not met must do so only for the inputs it was written for; a check that accepts a
  nonzero exit, an incomplete set or a conflicting id is a bug. An error must not be swallowed
  where an outer binding or a later branch could still answer.
- **Limits and truncation.** A path, depth, object or step budget that resets between roots,
  treats zero as unlimited, or is applied after an unbounded walk; a `truncated` flag the client
  discards or that is set when nothing was omitted; a recursive model that exhausts the cache.
- **Ordering.** Owned memberships and relationships have a defined order (declaration order,
  with implied members at their clause); a reader that places a membership last, or two readers
  that order differently, is a finding.
- **Nil and panic.** A recorded or library-written entry can be empty; the parser never fails, so
  every AST shape reaches the semantic layer. A dereference without a guard is a bug, not a flag.

## Surfaces that must agree

The semantic engine has several readers of one model, and a change to one must reach, or be shown
not to affect, the others. Name the ones the diff touches and check each:

- Symbol-backed and graph-backed metamodel readers (`internal/semantic/semantics` versus
  `internal/semantic/metamodel`), including `ImpliedBaseFQNs`/`kindBaseFQN` and the two implied
  specialization paths.
- Reflection (`member`, `feature`, `ownedMember`, `usage`, `input`), RDF export and import
  round trip (`internal/translate/export`), API JSON, and the gRPC `GetSymbol` /
  `GetElementProperties` forms.
- The lowered IR (`internal/ir/lower`) and the runtime that consumes it; the view renderers for
  DOT, Mermaid, PlantUML, D2, draw.io and the editors' canvases, which each carry their own copy of
  a label, marker or compartment rule.
- Every client (Go, Python, Node, Rust, Java, Julia, MATLAB) for a gRPC change: the new field
  decoded, the capability gated before the call, the older-service path still correct, no new
  attribute shadowing a model feature.
- The REPL, LSP, MCP catalogue, man pages and `docs/reference/` for a flag, command or diagnostic;
  the agent skills repository for anything an agent reads (`AGENTS.md` §6.3).

## Conventions enforced

These are mechanical and are checked before a push, not discovered in review:

- A code comment is one or two lines (`AGENTS.md` §7). Run
  `git diff --merge-base origin/develop -- '*.go' | awk '/^\+\s*\/\//{n++; if(n>2) print; next} {n=0}'`
  and extend it to the other comment syntaxes the diff touches (`#` in Make, shell, YAML and
  Python; `;` in Emacs Lisp); it must print nothing.
- No dead code: a helper left without callers after a refactor fails `staticcheck` and the
  no-dead-code rule. `go vet ./...` and `make lint`.
- A change under `internal/translate/export` moves the writer digest the api-json provenance guard
  pins: run `go run -C tools ./cmd/pilot-apijson-diff -update` and `go test -C tools ./referee/apijson`
  before pushing (`AGENTS.md` §6.2). The same holds for the pilot-differential baseline and the
  embedded library snapshot when their inputs move.
- Tests are not reduced. Deleting or narrowing a test that still guards live code is a finding
  even when the replacement is tidier; each bug fixed gets a regression test named for the
  behaviour; a `_test.go` file over 2,000 lines or a fixture over 200 KiB fails
  `tests/hygiene/size_test.go`.
- No internal work-item labels, session references or process narration in anything a reader
  sees (`AGENTS.md` §7); `python3 scripts/check-doc-ids.py`.
- A changelog fragment under `changes/unreleased/` for a user-visible change, never an edit to
  `CHANGELOG.md`; `python3 scripts/changelog.py check`.
- A compliance row moved honestly: a rule listed twice, or marked faithful on an approximation,
  is a finding.

## Ignore

- Generated stubs and recorded fixtures listed in `.gitattributes` (protobuf and Connect stubs,
  goldens, `.expected.json`, exported Turtle and JSON, measured baselines): review the source
  change that regenerated them and the adjudication in `docs/project/`, not the regenerated bytes.
- `tests/corpus/testdata/training_examples_expected.txt` is never re-baselined; a diff there is
  itself the finding.
- Formatting that `gofmt` owns.
