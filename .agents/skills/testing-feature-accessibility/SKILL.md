---
name: testing-feature-accessibility
description: How to end-to-end test the constraint-tier feature-accessibility rule in internal/core/passes/w8c_feature_reference.go ("Must be an accessible feature") — building a non-vacuous old-vs-new differential, sweeping examples/ and testdata/ for false positives, refereeing both directions against the pinned pilot validator, and the traps that have reverted this rule before.
---

# Testing the `Must be an accessible feature` rule (W8C FeatureReferencePass)

`internal/core/passes/w8c_feature_reference.go` runs at `LevelConstraint` and emits
`Must be an accessible feature (use dot notation for nesting)`
(code `feature-reference-featuring-types`) and `Must be a valid feature`
(`feature-reference-referent`). This rule has been **written, reverted, and rewritten** because the
hard part is not making it fire — it is keeping it silent on valid models. Plan the effort
accordingly: budget most of it on false-positive hunting, not on the firing cases.

## Surface: the plain CLI already shows it

No flag is needed. `./bin/sysml <file.sysml>` runs the constraint tier, prints the diagnostic with
a caret under the offending path, prints `sysml: <file> did not analyse cleanly`, and exits **2**;
a clean file prints `✓ package <Name>` and exits 0. Build with `make build-sysml`.

Always redirect stdin (`< /dev/null`): with a file argument the binary still drops into the REPL
banner afterwards and will otherwise consume your heredoc/script text.

## Tier masking will fake a pass — make every "clean" fixture actually clean

Higher tiers are skipped when a lower tier errors (AGENTS.md §4). A fixture with a typo, an
unresolved `String`/`Anything` (they need `private import ScalarValues::*` / `Base::Anything`), or a
bad `port in p : T` / `flow a.x to b` spelling reports a *different* diagnostic and the constraint
pass never runs — which looks identical to "no false positive". After writing any negative
fixture, confirm the CLI prints `✓ package …`, not merely the absence of the accessibility message.
Grepping `-c 'Must be an accessible feature'` alone is **not** sufficient evidence.

## The differential is the only way to know a firing case is new

Much of this rule predates any given change (a usage's *value* expression has been checked for a
long time), so a case that fires today may not be attributable to the diff under test. Build the
parent build and compare:

```bash
cp internal/core/passes/w8c_feature_reference.go /tmp/w8c_new.go
git show HEAD~1:internal/core/passes/w8c_feature_reference.go > internal/core/passes/w8c_feature_reference.go
go build -o /tmp/sysml_old ./cmd/sysml
cp /tmp/w8c_new.go internal/core/passes/w8c_feature_reference.go   # restore, then verify git diff
```

Expect surprises: e.g. `calc def C { return x = P::Q::n; }` fires on *both* builds, because
`return x = …` is a usage with a value and was already covered.

## Sweep examples/ and testdata/ with both builds — this is where regressions surface

`go test ./...` does **not** protect this rule's corpora: the runtime conformance suite executes its
models without running the constraint tier, so a model can start erroring in the CLI while the whole
suite stays green. Diff the flagged-file sets:

```bash
sweep() { find examples testdata internal/core/parser/testdata internal/core/runtime/testdata \
    -type f \( -name '*.sysml' -o -name '*.kerml' \) -print0 |
  while IFS= read -r -d '' f; do
    n=$("$1" "$f" </dev/null 2>&1 | grep -c 'Must be an accessible feature')
    [ "$n" -gt 0 ] && echo -e "$n\t$f"
  done | sort; }
sweep /tmp/sysml_old > /tmp/old.txt; sweep ./bin/sysml > /tmp/new.txt
comm -13 /tmp/old.txt /tmp/new.txt   # newly flagged files
```

**Known sensitive cluster:** `internal/core/runtime/testdata/conformance/` models where a named
`action r accept msg : T;` node's parameter is read from a *sibling* node's body
(`action p { assign total := msg; }`). OpenSysML deliberately shares accept parameters with sibling
nodes (a divergence noted in the source near `w8cOwnedByImplicitNode`), but that escape hatch only
covers *unnamed* owners, so any widening of body-expression walking tends to light up ~11 of these
files. The pinned pilot *agrees* these are errors, so it is not a pilot false positive — but it does
newly reject the project's own valid-by-construction fixtures, which is worth reporting either way.

## Referee both directions against the pinned pilot

```bash
build/pilot-sysml-validator/validate-sysml-batch FILE...   # library default is baked in
```

Filter `grep -v log4j`. Two caveats:

- The pilot's parser rejects several OpenSysML extensions (`done end;`, `then <src> <tgt>;`,
  `transition … do assign …`) with `no viable alternative at input …`. It often still reports the
  semantic error on the lines it *did* parse, so read past the parse noise instead of discarding the
  file; better, reduce the model to standard spellings first.
- Where the pilot cannot resolve the name at all it says
  `Couldn't resolve reference to Element 'x'` rather than the accessibility message. Treat that as
  agreement-in-kind, not as silence.

**Gap class worth re-checking on any change here:** a body inside a *nested definition* that reads
the *enclosing definition's* feature — `part def P { attribute n = 1; calc def E { n + 1 } }`, and
the same shape with `constraint def` or a `state def` transition guard. The pilot reports; OpenSysML
has been silent on all three. Minimal probes reproduce it in three lines, so include them.

Element-filter expressions (`filter …;`, `import P::*[@T]`) are accessibility-checked with the
candidate element as the featuring context; the referent is accessible when its declaration comes
from library content. A filter fixture must use a metaclass or `metadata def` feature
(`metaclass M { var feature a : Boolean[1]; }`, `metadata def Safety { attribute isMandatory : Boolean; }`):
a plain `part def` attribute or any dot chain in a filter is masked by the type tier's
`Must be model-level evaluable` and can silently look like a pass.

## Fixture shapes worth keeping

Firing (one diagnostic each): a qualified path `Pkg::Def::attr` written in a `constraint def` body,
`assert constraint {}`, `require`/`assume constraint {}`, `calc def { return x = …; }`, a bare
implicit calc result `calc def C { Pkg::Def::attr }`, a transition guard, `assign v := …;`, an
`if <cond> { … }` / `while <cond> { … }` action body, a `state … entry action e { assign … }`, and a
`transition first a then b do assign …`.

Clean: a body naming its own / inherited / redefined feature; `s.mass` via a requirement `subject`;
`q.n` via a part; deep chains `a.w.radius`; connector and interface ends with `::>`; enum literals
`Color::red`; `ISQ`/`SI` quantity refs; a nested package with an `alias`; and — the risk area when
`walkMember` gains a `default:` fallthrough — bodies holding member kinds with no explicit case:
`bind`, `first`/`succession`, `flow … to …`, `message`, `send`/`accept`, `perform`/`exhibit`/
`include`, `doc`/`comment`, `@Metadata { … }`, and `view`/`render` members.

## Gates that must hold

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 \
  go test -count=1 ./internal/core/model -run 'TestTrainingExamples|TestPilotCorpora|TestCorpusGates'
go run ./cmd/pilot-diff    # summary also lands in build/pilot-diff/pilot-diff.txt lines 5-7
```

`pilot-diff` takes a couple of minutes and writes only under `build/` (gitignored) — confirm
`git status --porcelain` is clean rather than committing or regenerating any baseline JSON.

## Recording

This is CLI-only work, so there is nothing to record unless you deliberately drive `bin/sysml` in a
GUI terminal to give a reviewer visual proof. The image ships **konsole** (no `xfce4-terminal`,
no `xterm`), `DISPLAY=:0`:

```bash
DISPLAY=:0 nohup konsole --hide-menubar --hide-tabbar -e bash -c 'cd <repo>; export PS1="\$ "; clear; exec bash --norc' &
DISPLAY=:0 wmctrl -i -r <win-id> -b add,maximized_vert,maximized_horz
```

Drive it with small `/tmp/demo*.sh` scripts that `cat` the fixture and then run the CLI, so each
screen shows model and verdict together; `clear` between sections keeps output on one screen.

## Devin Secrets Needed

None. The corpora and the pinned validator are provisioned by the repo blueprint
(`scripts/download-training-examples.sh`, `download-pilot-corpora.sh`,
`download-pilot-sysml-validator.sh`).
