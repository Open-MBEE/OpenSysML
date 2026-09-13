---
name: testing-sysml-repl
description: How to build, drive, and record end-to-end tests of the OpenSysML sysml REPL (bin/sysml) and the sysml-grpc service with its opensysml Python client — meta-command behavior, symbol lookup, action/state debugging, gRPC feature-value serialization, and GUI-terminal recording setup.
---

# Testing the `sysml` REPL end-to-end

## Action checker and witness replay

- Low-level runtime conformance fixtures may omit scalar imports because their
  harness builds a runtime directly. Before using one through the validating CLI
  or `%load`, make a scratch copy with `private import ScalarValues::*;` inside
  its package if needed. Do not edit the original fixture to make a demo pass.
- `action_fork_branches_write_one_feature.sysml` has action `test::clash`;
  `action_join_waits_for_slowest_branch.sysml` has `test::gather`. With imports
  complete, the former checks divergent `x=1,2` and the latter agrees on
  `arrived=3; seen=3`. Generate witnesses in a fresh scratch directory.
- A witness has choice lines, a blank line, then the trace. Compare the entire
  suffix to ordinary `-trace -schedule replay:<file>` output after removing
  `[trace] ` prefixes. Test both syntax-invalid witnesses and a syntactically
  valid header naming a token absent from the run.
- `%replay` does not switch the analysis engine: use `%engine auto` before
  `%action`/`%step`/`%continue` to step rather than search again.
  `-engine check file.sysml` alone preselects the engine in an interactive
  session; missing-action validation is reached when a check is requested.
- For a CLI reduction comparison, `por_independent_branches.sysml` needs
  `-schedule explore:runs=10000` to finish its 2520 linearizations; the default
  1024-run exploration stops incomplete. Compare outcome values as well as
  counts, and inspect the JSON `exploration.complete` field.
- State-only conformance cases under this action-only checker are refused at
  the engine/question boundary. That is not proof of reaching the lower-level
  paused-body snapshot refusal; distinguish those two kinds of coverage.

### Devin Secrets Needed

None for local checker and witness replay testing.

## Choice-pseudostate scheduling across driver surfaces

For a library-backed CLI fixture, import `ScalarValues::*` and `SI::*`; use
`accept after 1 [s]`, not the unitless trigger accepted by some isolated runtime
test fixtures. A timer lets identical models run in REPL and CLI without signal
injection flags. Put `do assign x := 8` on the incoming transition and two guards
`x > 5` and `x > 7` after `choice pick;` to distinguish dynamic choice guards
from pre-effect evaluation.

Use `%trace on`, `%state P::Machine`, `%advance 1`, `%current`. `%schedule
explore` may explicitly refuse at the prompt: use CLI `-schedule explore
-state P::Machine -advance 1 -trace model.sysml` for the outcome/witness table.
Copy `choice pick -> 2->right` into a witness file and select it via `%schedule
replay:<file>` before a fresh `%state`. Default policy is named `reverse`,
but a choice's default branch is still the first enabled declared branch.

Make replay rejection load-bearing: declare a third, disabled branch and replay
its exact label while two other branches stay enabled. Check both the absence of
successful branch execution and `is not enabled`; some driver paths may fail to
surface scheduler refusal. Repeat in a fresh CLI process, check its exit status,
and compare a signal-driven `%send Go` / `%step` to timer-driven `%advance`.
Also put a disabled branch *before* two enabled branches to verify witness
labels keep original declaration positions (`2->left`, `3->right`).
The text trace alone does not prove ChoicePoint File/Span or Go error identity.

### Devin Secrets Needed

None for local choice-pseudostate CLI/REPL testing.

## Model-level uncertainty versus object-level empty values

Use `cmd/pilot-exec-diff/testdata/models/undetermined_operands.sysml` to
contrast model and object evaluation without inventing a fixture. Before
instantiation, `%eval U::u` and
`%eval SequenceFunctions::size(T::rack.gear)` answer `<undetermined>`,
while `(U::u > 3) and false` answers `false` and the size of `T::rack.slots`
is `3`. Use fully qualified operands/functions when the file has multiple
packages.

After `%instantiate T::rack`, `%eval T::rack.loose` answers `[]`, not
`<unset>`: an optional multi-valued part is an empty sequence. Pin the
object explicitly with `%eval in T::rack : SequenceFunctions::size(loose)`
to observe `0`. `%features T::rack` prints many inherited nested features;
capture a short `%eval` result separately so it is not scrolled off-screen.

The Python `Model.eval("U::u")` result is `opensysml.Undetermined`, distinct
from `UNSET` and `None`; `bool()` must raise TypeError. Check the advertised
`undetermined_value` capability and raw `Value.undetermined` count bounds
for `U::u` (`1..1`), `T::rack.gear` (`1..*`), and `T::rack.loose` (`0..2`)
to distinguish typed transport from a string-only rendering.

## Error-model lookup order and bounded walking

Use real-service fixtures that distinguish an empty Query from an incomplete
Query. `package Demo { attribute :>> mass; part def Part { attribute mass; } }`
has an unresolved outer `Demo::mass` with SymbolInfo name `mass`, while the
effective-name Query sees only `Demo::Part::mass`. Lookup must agree with an
independent BFS and select the outer symbol despite the nonempty Query.

Also test the inclusive depth boundary: declare `part def First { attribute
:>> mass; }` before `part def Second { attribute mass; }` in Demo. The hidden
`Demo::First::mass` must win over the visible, same-depth `Demo::Second::mass`.
Inspect `model.root.id`: a real file root can be `""`, adding a BFS level not
represented by a nonempty owner ID. Do not infer tree depth from ID separators.

Count real GetSymbol requests with a transparent delegating observer on
`Connection._service`. Take counts before independent BFS warms child caches.
When comparing many lookups, create a fresh `Model(model._pb, connection)` for
each cold lookup. In a bounded-walk fixture, place hundreds of descendants
below the best candidate; assert that no descendant IDs are fetched, rather
than judging boundedness only from wall-clock timing. Clean shared names can
need owner Query requests; a unique name is the one-Query/one-GetSymbol control.

An equivalence corpus should separately report erroring files, not simply a
total lookup count. Files from a multi-file example loaded individually may
provide useful real unresolved-reference cases. Preserve their diagnostics
instead of silently skipping them.

## Unicode unit expressions in a GUI terminal

Synthetic keyboard typing can corrupt middle dots and superscript minus signs
in unit names before the REPL receives them. Use a UTF-8 clipboard instead:
`printf '%s\n' "<command>" | xclip -selection primary`, then middle-click the
xterm. Compare exact-input CLI results before treating a GUI parse failure as
a product bug. If xclip is absent and system installs are unavailable,
`apt-get download xclip` and `dpkg-deb -x <deb> <scratch-dir>` provide a local
binary without changing the system. Maximize the active window with
`wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz`; shell startup may
overwrite xterm's requested title, so title-based matching can fail.

Coherent-quantity probes should distinguish named-unit selection from fallback:
an untyped inverse second may remain `1/s` because Hz and Bq measure different
kinds; an unlisted dimension stays a base-unit product. Include both prefixed
and unprefixed inputs to check the magnitude, not just the displayed unit.

## Exploration scheduling

Use `internal/core/runtime/testdata/conformance/action_explore_three_writers.sysml`
with the qualified action `test::race`: `-schedule explore` should produce three
outcomes (`x=1,2,3`), two linearizations each, and `complete (6 runs)`. Compare
two outputs byte-for-byte, not just their counts. `explore:runs=2` exits 2;
the incomplete table and status may be written to stderr, while `-json` must
still produce parseable stdout with `checks[0].outcomes` and `.exploration`.
Check `-help` for the default policy before choosing a regression comparator:
at introduction of exploration it was `reverse`, not `declared`.

For a real state-conflict test, load `state_explore_transition_conflict.sysml`
through the Python client and call `explore_state("test::Switch",
events=["Go", "Go"])`. The CLI without events only checks initialization,
not competing transitions. Expect two paths, through `left` and `right`,
both ending in `settled`, with `side=1,2`. The event sequence is pinned by the
fixture's `.expected.json`.

At the interactive prompt, set `%schedule reverse`, try `%schedule explore`,
then query `%schedule` to prove the refusal preserved reverse. `%action`
starts the executor; `%continue` is needed to see its final result.

The REPL is the user-facing surface of `cmd/sysml`. Test it by actually running the binary, not
just via `go test ./internal/repl`.

## Build

```bash
export PATH=/usr/local/go/bin:$PATH   # the repo blueprint puts Go here
make build-sysml                      # -> ./bin/sysml
./bin/sysml --version                 # prints the commit it was built from — useful evidence on camera
```

Always rebuild after pulling a new commit; a stale `bin/sysml` silently tests the old revision.
Print `--version` at the start of a recording so the reviewer can see which commit ran.

### A contrast binary from the previous commit

For any fix whose point is "this used to fail", build the parent revision alongside the new one
and run the same input through both on camera. A worktree keeps the branch checkout untouched:

```bash
git worktree add /tmp/old<sha> <sha>            # e.g. the commit before the fix
(cd /tmp/old<sha> && make build-sysml && cp bin/sysml /tmp/old-sysml)
git worktree remove /tmp/old<sha>               # when finished
```

Then `/tmp/old-sysml` vs `./bin/sysml` on identical input is the strongest evidence available —
it rules out "the test would have passed anyway". Especially valuable for diagnostic wording and
line/column numbers, where a screenshot of the new behavior alone proves nothing. Building through
the Makefile preserves the version ldflags before the binary is copied out of the worktree; plain
`go build` would make `--version` report `dev` / `unknown`.

## Document rendering (`-render-document` / `%render-document`)

The document-query engine renders `part def`s specializing `DocumentQueries::Document` to Markdown:

```bash
./bin/sysml -render-document Observatory::MassReport internal/core/docrender/testdata/telescope_report.sysml
printf '%%load <file.sysml>\n%%render-document <Qualified::Name>\n%%quit\n' | timeout 30 ./bin/sysml
```

- The REPL output is byte-identical to the CLI output except for the banner line, the `✓ package …`
  load line, and a trailing `goodbye` — strip those before diffing REPL vs CLI.
- If the model has any validation error, the CLI exits 2 with
  `did not analyse cleanly; nothing was rendered` — diagnostics (including the typed Diagram
  diagnostics: bad direction, missing kind on a plain-element source, direction on a
  sequence/table kind) surface at analysis time, before any rendering.
- `Diagram` content blocks emit fenced ```mermaid blocks. Validate mermaid syntax offline with
  the pre-installed mermaid-cli instead of mermaid.live:
  `echo '{"args":["--no-sandbox"]}' > /tmp/pptr.json` then
  `/home/ubuntu/.local/mermaid/node_modules/.bin/mmdc -p /tmp/pptr.json -i block.mmd -o block.svg`
  (add `-o block.png -b white -s 2` for PNG evidence). If that path is missing, install with
  `npm install --prefix ~/.local/mermaid @mermaid-js/mermaid-cli`.
- Extract mermaid blocks from the rendered Markdown with awk:
  `awk '/```mermaid/{f=1;n++;next}/```/{f=0;next}f{print > "block"n".mmd"}' render.md`
- A failed `%render-document` leaves the REPL session alive and usable — but `%load`ing a second
  file that declares the same package makes its symbols ambiguous (`use a qualified name`).
  Restart the session, or use distinct package names between fixtures.

## Library-cache cold/warm testing (`XDG_CACHE_HOME`)

`bin/sysml` persists stdlib symbol indexes under `$XDG_CACHE_HOME/sysml-ls/libs/*-v<N>.idx`
(`internal/core/libs`, `formatVersion` in `record.go`). Cache-dependent bugs only show up on the
*second* run, so any change touching `symbols`/`libs`/`resolve` should be tested like this:

```bash
export C=/tmp/cachetest && rm -rf $C && mkdir $C
XDG_CACHE_HOME=$C ./bin/sysml -validate file.kerml > cold.out 2>&1; echo $?
XDG_CACHE_HOME=$C ./bin/sysml -validate file.kerml > warm.out 2>&1; echo $?
cmp cold.out warm.out    # cold and warm must be byte-identical
```

- A known reproducer class: implicit parameter redefinition against cached (Decl-less) library
  symbols, e.g. `package R { behavior b { step u : FeatureReferencingPerformances::FeatureWritePerformance { in onOccurrence { feature redefines startingAt; } } } }`
  in a `.kerml` file — cold-clean/warm-broken before cache format 21.
- Stale-format handling: warm the cache with a binary built from an older commit (worktree trick
  above), then run the new binary against the same `XDG_CACHE_HOME`. It must ignore the old
  `-v<N-1>.idx` files and write `-v<N>.idx` alongside them (`find $C -name '*.idx'` shows both);
  output must equal a fresh-cache run. Versions coexist — the old binary keeps using its own files.
- For corpus-level sweeps, run every file twice against one shared cache dir and `cmp` per-file
  outputs and exit codes; only the first file of pass 1 is truly cold, which is fine for
  warm-identity checks.

## Profiling flags: `-memstats`, `-cpuprofile`, `-memprofile` (PR #156)

`main()` is a one-liner over `runCLI()` returning an exit status, so every profile is flushed by a
`defer` before the process exits. That makes **exit codes the main regression risk** of any change
in `cmd/sysml/main.go`: walk every mode and assert the status, since a `return` mistranslated from
`os.Exit` is invisible in the output text. Values observed at bb0cfdc:

| command | exit |
|---|---|
| `-version`, `-eval '1+2'`, `-validate <clean>`, `<m> -convert sysml -o f`, `<m> -eval '1+1'` | 0 |
| `-validate <model with errors>` (reported as `did not analyse cleanly; no check was made`) | 2 |
| `<m> -convert sysml -validate` (refuses: "check it in its own run"; writes no file) | 2 |
| `-debug -quiet`, `OPENSYSML_MAX_STEPS=abc …`, `-cpuprofile`/`-memprofile` on an unwritable path | 2 |

- `-memstats` writes exactly one line to **stderr** (`sysml: 637ms wall, 242.6 MiB allocated in
  … allocations over … collections, … MiB taken from the OS`). Prove the split with
  `>out 2>err` and `cat` both — a memstats line on stdout would break `-convert -o /dev/stdout`
  and JSON reporting.
- It fires for the interactive REPL too, after the session ends, because it runs in the deferred
  stop. This is the cheapest check that the profile flush survives the REPL path.
  **Leave the REPL with Ctrl-D, not by typing `goodbye`** — at b3f16e4 `goodbye` at the `sysml>`
  prompt is parsed as model text and answers `1:1: error: expected a namespace member` (and, over a
  pipe, makes the whole run exit 2), so a transcript that types it looks like a failure.
- Heap profiles are written at end of run, when the model is already unreachable, so read them as
  `go tool pprof -top -sample_index=alloc_space <file>` (plain `-top` shows inuse_space and looks
  almost empty). Expect `parser.(*Parser).fill` / `parseUsage` and `symbols.*` at the top.
- A 0-byte profile, or `unrecognized profile format` from pprof, is the signature of the flush not
  happening — treat it as a failure of the runCLI change rather than a pprof problem.
- Unwritable-path errors must abort before any model work: assert no `✓` line is printed.
- Generate a load big enough for the numbers to be meaningful (a ~28k-line model with several
  hundred `part def`s allocates ~240 MiB, ~600 ms); `/tmp/m*.sysml` generators from earlier
  sessions may still be around.

## Diagnostics-unchanged checks for core refactors

For perf refactors in `internal/core` (scope indexes, resolve memoization, redefinition-owner
lookup) the only convincing evidence is a **byte-for-byte diff against a binary built from the
parent commit** (see the contrast-binary recipe above):

```bash
for f in inherit_ok noinherit imports_ok imports_bad; do
  diff <(./bin/sysml -validate /tmp/pt/$f.sysml 2>&1; echo $?) \
       <(/tmp/old-sysml -validate /tmp/pt/$f.sysml 2>&1; echo $?) >/dev/null \
    && echo "$f: IDENTICAL" || echo "$f: DIFFERS"
done
```

Fixtures that actually exercise the redefinition-owner path:

- **clean:** `part def Base { attribute speed : SpeedType; }`, `part def Car :> Base;`,
  `part def RaceCar :> Car { attribute speed : SpeedType :>> Base::speed; }` — inheritance through
  a chain must stay clean.
- **still an error:** the same `:>> Base::speed` inside a `part def Other` that does *not*
  specialize `Base` → `error: speed redefines speed, but speed is not an inherited member of Other`
  (code `redefinition-no-inherited`).
- Note an unqualified `attribute :>> weight : Real;` naming a member nothing declares reports
  `unresolved reference: weight` from name resolution instead, never `redefinition-no-inherited` —
  use the **qualified** `:>> Base::speed` form to reach the constraint pass.
- Imports: a wildcard `import Lib::*` plus a nested `import Lib::Inner::Wheel` for the positive
  case, and a usage of an undeclared type for the negative (`unresolved reference: Gearbox`).

## Per-snippet parse kind: .kerml vs .sysml gates (PR #360)

Since F51 the REPL analyzes each snippet under the parse kind of the file it came from, so the
parser's file-kind gates are testable through the binary:

- **Fixture pair that hits both gates:** `package F51Expr { feature at = 1; feature while = 2;
  feature merge = 3; feature decide = 4; feature total = at + while + merge + decide; }` (contextual
  keywords as names) and `package F51Type { class at; feature f : at; feature g = at::self; }`
  (keyword in type/qualified-name position). As `.kerml` both must `-validate` clean (exit 0) and
  `-convert sysml` successfully; byte-identical `.sysml` copies must produce four
  `"<kw>" is a reserved keyword` warnings (expr) and an `at::self` → `expected '{' or ';'` **error,
  exit 2** (type). The reference `./build/pilot-kerml-validator/validate-kerml` accepts both .kerml
  files (exit 0) — the agreement check.
- **The prompt is always SysML-kind:** `namespace N;` typed at `sysml>` must still get the
  `` `namespace` is KerML notation `` warning, and `package P { feature at = 1; }` the
  reserved-keyword warning, even right after clean `.kerml` %loads. If either warning disappears
  after a `%load *.kerml`, the snippet kind leaked into the prompt.
- **Mixed sessions:** `./bin/sysml k.kerml sn.sysml -validate` (or `%load` both) must attribute the
  kerml-notation warning **only** to the `.sysml` file, and a prompt snippet must resolve
  kerml-declared names across kinds (`package Q { attribute r = K::f; }` → `%eval Q::r`).
- Known pre-existing (same on main, not regressions): `%eval` of a reference whose value chain
  names a kerml feature spelled with a contextual keyword fails `unresolved reference: at`;
  compound `%eval` expressions reparse the buffer under an unknown-kind temp doc.

## Two ways to drive it

1. **Non-interactive (fast, for exploration and expected-value discovery).** The REPL reads a
   script from stdin fine:
   ```bash
   printf '%%load internal/repl/testdata/vehicle_package.sysml\n%%instantiate Vehicle\n%%features Demo::Vehicle\n%%quit\n' | timeout 30 ./bin/sysml
   ```
   Note `%%` in `printf` format strings. Always wrap in `timeout` so a hang shows up as a
   non-zero exit rather than stalling the session.
2. **Interactive in a GUI terminal (for the recording).** Because the app under test *is* a CLI,
   the recording should show a real terminal session. See the recording setup below.

Run shell commands such as `clear` only before launching the REPL. At the `sysml>` prompt they are
parsed as model source; rebuilding the model can restart a running exhibited machine and invalidate
the rest of a walkthrough. Loading a second model into the same session also restarts machines and
shifts instance IDs, so use a fresh REPL session for each model or contrast run.

**At the `sysml>` prompt an expression must go through `%eval`.** Bare text — even a fully
qualified `test::r.cost.v` — is submitted as *model source*, so it answers
`1:1: error: expected a namespace member` and leaves an unresolved buffer error that taints the
next submission with `note: deeper checks may not have run here…` (the same trap as typing
`clear`). On camera this looks like the feature is broken. Type `%eval test::r.cost.v`; if you have
already tainted the session, `%quit`/Ctrl-D and restart rather than continuing.

In Konsole, `xdotool key ctrl+shift+plus` may type a literal `+`; use
`xdotool key ctrl+shift+equal` to enlarge the font before recording.

### Driving a `type: "calc"` conformance fixture from the CLI

Fixtures under `internal/core/runtime/testdata/conformance/` whose `.expected.json` says
`{"type": "calc", "evaluate": "test::probe"}` are run non-interactively with `-calc` and an
explicit argument list — the parentheses are required even when the calc takes none:

```bash
./bin/sysml -calc "test::probe()" internal/core/runtime/testdata/conformance/<name>.sysml
./bin/sysml -validate internal/core/runtime/testdata/conformance/<name>.sysml   # cheap clean-model check
```

Reals print as the shortest decimal that reads back as the same value, a whole one keeping its
`.0` (`= [5.0, 2.0]`, `= 0.3333333333333333` for `1.0 / 3.0`). A failing evaluation exits 2 with
`sysml: calc invocation failed: calc test::probe: evaluating the returned expression: <error>`,
while a *static* rejection exits 2 with `did not analyse cleanly; no check was made` — worth
distinguishing in a report, since a negative case can be caught at either tier. Beware that a
`… | grep …` pipeline in a recorded one-liner makes `$?` the grep's status; capture sysml's own
exit code without a pipe when the exit code is part of the evidence.

## Saving and converting models (`%save`, `sysml -convert`)

The CLI is `sysml <model> -convert <format>`: the model is a positional argument and `-convert`
names the format to write, so the output path never decides it — `-o /dev/null`, `-o /dev/stdout`,
`-o /dev/fd/63` and a FIFO named without a suffix all work as they are. An *input* whose
extension names no format needs `-from`, otherwise you get
`cannot tell the format of "input.txt": expected .sysml, .kerml or .ttl, so pass -from`
and no write is attempted. The REPL takes the format from the file extension and has no such
flags, so `%save` says
`name the file with a .sysml, .kerml or .ttl extension` instead — check the two surfaces word
their advice differently and neither mentions the other's remedy.

Things worth setting up as fixtures before a save/write test, since each exercises a different
branch of `internal/core/export/write.go`:

- a FIFO (`mkfifo`) with a **background reader** — the write blocks until something reads; assert
  `ls -l` still shows `prw-` afterwards, i.e. the pipe was written through, not renamed over.
- `/dev/full` — the honest failure path; expect `cannot write /dev/full: no space left on device`
  and a non-zero exit rather than a success line.
- a symlink, a link **chain**, a link into another directory, and a **dangling** link — all must
  stay links, with the file they point at (created, if dangling) holding the new bytes.
- an existing 0600 file (permissions must survive) next to a brand-new one (0644).
- a directory chmod-ed 0500 containing a writable file, where the previous content is **longer**
  than the new model — proves the direct-write fallback truncates instead of leaving stale bytes.
  Remember to `chmod 700` it again or later cleanup fails.
- assert the negative too: no leftover `.name.sysml.<digits>` temp files, and failure messages
  must name the path the user typed rather than the temp file.

**`x.sysml -convert sysml` is a source-preserving formatter, not an AST printer.** It keeps the
original inline/multi-line layout and reproduces surface notation verbatim (a member-attached
`then part b;` comes back as `then part b;`, not as the desugared `then a b;`), so its output is
**not** evidence about what the parser built. To assert on AST/desugaring, use `-convert ttl` (generated
from the tree — e.g. `grep -c SuccessionAsUsage`) or a parser golden fixture. The `.ttl -> .sysml`
direction *is* a real AST print, so a round-trip through Turtle is the way to see canonical notation.
A useful corollary: to prove "no relationship was recorded", count the triples in the `.ttl`, never
grep the reformatted `.sysml`.

Models that convert to Turtle are a narrow set, but the set grows: **condition members
(constraint/requirement `assert`/`assume`/`require`, `subject`, `return`) map since PR #182**, while
state and action nodes still do not. Anything with a state substate, an initial node, `perform`,
`send`, an assignment or prefix metadata still fails with
`cannot convert the <thing> at <file>:<line>:<col>: save to .sysml or .kerml instead …` and exit 2.
How many of `examples/` convert is measured in `docs/project/roadmap.md` § Track D; most of the
training copies do, so a Turtle test can use real models — the Constraints/Requirements/
Analysis/Verification training packages are the richest. A useful sweep, which also proves "the message is clear and it never panics":

```bash
find examples -name '*.sysml' -print0 | while IFS= read -r -d '' f; do
  out=$(./bin/sysml -convert ttl "$f" -o /tmp/e.ttl 2>&1) \
    && echo "OK $f" || echo "FAIL $f :: $(echo "$out" | head -1)"
done | sort | uniq -c        # note -print0: many training paths contain spaces
```

A hand-written fixture is still the smallest positive case; this one round-trips and yields exactly
1180 bytes of Turtle:

```
package Demo {
    // a comment
    part def Vehicle {
        attribute mass;
    }
    part v : Vehicle;
}
```

Use it for byte-identity assertions across argument orders (`md5sum … | sort -u | wc -l` must print
`1`) — comparing hashes is far stronger evidence than eyeballing that each invocation "worked".
Beware: `-convert sysml` **does** normalize tab indentation to 4 spaces (comments and inline/
multi-line layout survive), so a tab-indented fixture like `vehicle_package.sysml` will never be
byte-equal to its own reformatted output. Assert idempotency (pass 2 == pass 1) rather than
fixture-equality (pass 1 == input).

For formatter changes, the cheap adversarial check is idempotency over real models:
convert every `examples/*.sysml` with `-convert sysml` twice and `diff` the two outputs — all eight
must be byte-identical, and stderr must stay empty for well-formed input. Pipe the whole loop
through `sort | uniq -c` so the evidence is one aggregate count instead of a screenful that scrolls
the failures off the top:

```bash
for f in examples/*.sysml internal/repl/testdata/*.sysml; do
  ./bin/sysml "$f" -convert sysml > /tmp/p1 2>/dev/null
  ./bin/sysml /tmp/p1 -convert sysml -from sysml > /tmp/p2 2>/dev/null
  cmp -s /tmp/p1 /tmp/p2 && echo idempotent || echo "NOT IDEMPOTENT: $f"
done | sort | uniq -c
```

### `bin/sysml-grpc` takes `-port`, not `-addr`

`-addr` prints "flag provided but not defined" with a usage dump, so drive a freshly built service
with `./bin/sysml-grpc -port 50123` and `opensysml.connect(port=50123, auto_start=False)` to keep
`~/.opensysml/bin` out of it.

## Proving a Turtle round trip loses nothing

`.sysml -> .ttl -> .sysml` is never byte-equal to the input (the back direction is a canonical AST
print: it spells out `in attribute x`, `return attribute`, adds the `;` after a bare condition and
re-indents). So assert **fixed points and AST identity**, never input-equality:

- `model.sysml -> a.ttl -> a.sysml -> b.ttl` with `diff a.ttl b.ttl` empty — this simultaneously
  proves the emitted Turtle is well-formed and reloadable and that nothing was lost.
- `b.ttl -> b.sysml` and `diff a.sysml b.sysml` empty (notation fixed point).
- Strongest: compare the **parsed AST** of the original and of the round trip. There is no CLI dump
  flag, so build a throwaway `main.go` that calls `parser.New(source.New(path, bytes)).ParseFile()`
  and prints `ast.Dump(f)`. It must live *inside* the repo module (`internal/…` blocks an outside
  module), so `mkdir tmp_dumpcmd && go build -o /tmp/dump ./tmp_dumpcmd && rm -rf tmp_dumpcmd`, then
  `diff <(/tmp/dump orig.sysml) <(/tmp/dump rt.sysml)`. Empty output is the no-silent-data-loss
  proof; `git status --porcelain` afterwards confirms the repo is untouched.

Two round-trip failure classes are **pre-existing** (reproduce them with a binary from the parent
commit before reporting them as regressions):

- **Quoted names are printed unquoted.** `package 'Package Example'` comes back as
  `package Package Example {` and `<'1'>` as `<1>`, so the re-parse fails with
  `1:17: expected '{' or ';'`. Most `examples/sysml-v2-training` files use quoted names, so sanitize
  them (`sed -E "s/package '([^']*)'/package P/"`) before a round-trip sweep, or the whole sweep is
  red for one unrelated printer bug.
- `ref` on an `end` attribute is dropped and `redefines`/`typing` relationship order is swapped in a
  couple of models.

### Behavioral bodies through RDF (PR #270)

Since PR #270 an action or state body converts instead of being refused, so a round-trip sweep now
reaches paths that used to stop at the first behavioral node. What to know:

- **`-o` decides nothing about the format, but the *extension* of an input does.** A sweep that
  writes intermediate files as `/tmp/x.rt` and feeds them back gets
  `cannot tell the format` and every model looks refused. Always name intermediates `.sysml` /
  `.ttl` (or pass `-from`).
- A good adversarial single fixture: one `action def` with
  `first/perform/assign/send … via/accept when/fork/join/merge/decision/while/loop-until/for/
  if-else/terminate/done/succession` plus one `state def` with a nested substate, a `region`,
  `entry`/`do`/`exit`, `defer`, choice/junction/shallow+deep history and a
  `transition first a accept s : S via p if g do action … then b`. At 77abc565 it round-trips with
  the *only* difference being that a state named `deep` comes back as `'deep'` (the writer quotes it
  because `deep` is a keyword) — cosmetic, and stable on the next hop.
- Corpus baseline at 77abc565: of the 110 `examples/**/*.sysml`, **95 convert to ttl and back, 15
  are refused**, and the only model whose notation drifts between hop 1 and hop 2 is
  `41. Language Extension/Model Library Example.sysml` (the documented `end [*] ref` parser gap).
  Treat any other drift as a real defect.
- Refusal probes that still work and print no file: two members of one namespace sharing a name, a
  bare `snapshot`, and a succession whose end name needs quotes (`then 'my a' b;`). Note
  `succession then b;` and `transition first a accept s : S;` (no `then`) are *parse* errors, so
  they cannot be used as export-refusal fixtures.
- Two silent-change classes were found in that sweep and fixed in the same PR: prefix metadata
  (`#Safety part def Car;`) is now carried as `sysx:prefixMetadata "#Safety"`, and a feature that
  wrote no kind keyword (`in x : Real`) is flagged `sysx:isKindImplicit` instead of gaining a kind
  on the way back. An `@` annotation ahead of a definition is refused, because the parser records
  it on the declaration before the one it prefixes — worth re-probing if the parser changes.
- `export.ExperimentalNotice` (internal/core/export/experimental.go) is printed verbatim by the CLI
  (stderr), `%save` and `ConvertResponse`, and the same wording is duplicated in
  `cmd/sysml/main.go`, `clients/python/opensysml/`, `api/proto/` and `docs/guide/`. Check every copy whenever
  the mapping's coverage changes.

### Checking the experimental notice's copies (PR #271)

When a PR claims one wording is stated from one place, check the *runtime* copies mechanically and
the *documented* claims by running them:

- Extract the Go literal and compare collapsed whitespace, rather than eyeballing:
  parse `internal/core/export/experimental.go` for the quoted pieces of `ExperimentalNotice`, join
  them, then compare `" ".join(x.split())` against the paragraph `./bin/sysml -help` prints between
  "Turtle is normalized." and "Every run that converts RDF". `cmd/sysml/main.go`'s `wrapped(…, 78)`
  wraps on `strings.Fields`, so also assert every line's **rune** count ≤ 78 — the notice contains
  `§` (2 bytes), so a byte-based wrapper would pass a naive byte check.
- `clients/python/opensysml/conversion.py:EXPERIMENTAL_NOTICE` should equal the same literal; compare it in
  Python against the Go file directly.
- The client fallback lives in `Connection.convert` (`connection.py`, `response.experimental_notice
  or EXPERIMENTAL_NOTICE`). To exercise it, wrap the stub: `Connection._stub` is a read-only
  property, so patch **`conn._service`** with an object that delegates via `__getattr__` and, in
  `Convert`, calls the real stub then `response.ClearField("experimental_notice")` /
  `ClearField("experimental")`. That is the only way to simulate an older service without touching
  the Go side.
- **Guide transcripts go stale silently.** `docs/guide/07-saving-and-rdf.md` and
  `docs/guide/09-clients.md` embed real byte counts and refusal messages. Re-run each fenced command:
  the `%save` pair (guide 2's `MyModel` file → `181 bytes of sysml` / `1872 bytes of ttl`) and
  `examples/rdf-interop-demo.sysml` (`7937` ttl / `877` sysml bytes) still hold at 493693a3, but the
  page's refusal example (`examples/state-machine-demo.sysml -convert ttl` → "cannot convert the
  substate member") and its prose "a model whose point is a behavior does not [convert]" are wrong
  since #270 — the model converts, and all ten `examples/parser_features_demo_*.kerml` convert too.
  Grep for the *old* wording (`model structure only`, `bodies state behavior`) across `docs/ cmd/
  clients/python/ api/proto/ internal/` to catch leftover copies, and check re-worded prose paragraphs did
  not leave one line far wider than its siblings (`awk '{print NR": "length($0)}'`).

### Round-trip fidelity of a declaration head (PR #272)

Two narrow export paths decide whether a declaration comes back spelled the way it was written:
`encodeSubaction`/`bareWord` (`internal/core/export/behavior.go`) for a combined state subaction, and
`wroteKindKeyword`/`withoutComments` (`rdf_out.go`) for a kind keyword ahead of a name. Testing them:

- **A fixture only exercises `wroteKindKeyword` when the commented word equals the keyword of the
  kind the declaration would get implicitly.** `in /* attribute */ x : Real;` inside an `action def`
  proves nothing: the inferred kind there is `part`, so the check looks for `part`, the ttl records
  `sysx:isKindImplicit true`, and old and new binaries agree. Put the same members inside a
  `part def` (where `in x : Real` infers *attribute*) and the pre-fix binary writes back
  `in attribute x : Real;` while the fixed one writes `in x : Real;`. Always diff against a contrast
  binary from the parent commit (§"A contrast binary from the previous commit") — an equal result
  means the fixture missed the path, not that the fix is a no-op.
- Useful fixture set for these paths, all inside one `part def`: `in /* attribute */ x`,
  `out // attribute\n y`, `in //* a note\n attribute */ w`, the control `in attribute z`,
  `in /* /* */ v`, `in //* same line note */ u`, plus quoted names that contain the markers
  (`attribute 'a /* b */ c'`, `in 'x{y'`, `attribute 'do'`) — the quoted ones must come back
  character-identical.
- For subactions: `entry do { … }`, `entry do{ … }`, `entry do{perform A;}` must all come back
  `entry do {` (the tight two were the bug), and `entry /* do */ { … }` must **not** gain a `do`.
  A comment adjacent to a real `do` (`entry /*c*/ do { … }`, `entry do/*c*/{ … }`) keeps it too since
  aa9fa5ab, which reads the subaction head through `withoutComments` as well; before that commit the
  comment token took `do`'s place and the `do` was dropped, so use a pre-aa9fa5ab binary as the
  contrast for these two.
- An unterminated comment in a head (`in /* attribute x : Real;`) is a *parse* error
  (`unterminated comment: missing */`), not an export refusal — so it cannot serve as a refusal
  fixture, only as a "writes no output" check.
- Corpus baseline at bf2f21e3: `converted=102 refused=18 total=120` over
  `examples/**/*.{sysml,kerml}`; 97 of the converting models are byte-stable across two RDF round
  trips, 0 drift. **5 models' RDF-written notation does not re-parse** (`parser_features_demo_
  declarations.kerml`, `27. Occurrences/Interaction {Example-2,Realization-1,Realization-2}`,
  `32. Requirements/Requirement Satisfaction`) — e.g. `event X;` comes back as
  `event references X;`, which the parser then rejects. Pre-existing (identical on the parent
  commit), so treat it as the baseline, not a regression.
- A cheap way to separate printer formatting from RDF-hop changes: convert the model both ways
  (`-convert ttl` → back, versus `-convert sysml` alone) and diff the two outputs token-per-line
  (`tr -s ' \t\n' '\n'`). At bf2f21e3 76 of 102 examples still differ that way (`:>` written
  `specializes`/`subsets`/`redefines`, `default 3` written `= 3`, `on;` written `'on';`, `doc`
  comments dropped), **identically on the parent commit** — so use the old-vs-new diff of that report
  as the regression gate rather than the raw count.

Also note the parser rejects `require constraint <name> { … }` (a *named* nested constraint after
`require`/`assume`): `expected '{' after 'require constraint'`. Only the anonymous form and the
`require R { … }` reference form parse, so an export test cannot cover the named variant.

## Argument order and the `--` marker

`cmd/sysml/args.go` permutes arguments before `flag.Parse`, so flags may be written **after** the
model (`sysml model.sysml -convert ttl`, `sysml model.sysml -trace`). This code is on the path for
*every* invocation, so any change to it needs the unrelated modes (`-e`, `-debug`, `-trace`,
`-quiet`, plain REPL load, `-version`, `-h`) re-checked, not just the conversion flow.

The orders worth covering, since each hits a different branch: model first, flags first, model
*between* two flags (`-o out.ttl model.sysml -convert ttl`), a joined value (`-convert=ttl`), an
`-o` value that itself starts with a dash (`-o -weird.ttl`), and a flag whose value could be
mistaken for a model (`-from sysml in.txt -convert ttl` — if `sysml` is read as the file you get a
bogus `open sysml: no such file`).

A **file name beginning with a dash needs the `--` marker**: `sysml -weird.sysml` is reported as
`flag provided but not defined: -weird.sysml` (correct — a genuine typo like `--badflag` must still
be caught), while `sysml -e "1+1" -- -weird.sysml` loads it. Remember `--` is POSIX end-of-options,
so **everything after it is positional**: `sysml -- -weird.sysml -e "1+1"` loads the model and then
tries to open `-e` as a second file (`load -e: open -e: no such file or directory`). Write the flags
*before* the marker. A regression here is easy to miss — assert on a dash-leading file name
explicitly, because a permutation bug that drops `--` still looks fine for ordinary file names.

## Fixtures worth knowing

- `internal/repl/testdata/vehicle_package.sysml` — everything nested in `package Demo`
  (`Engine`/`power`, `Vehicle`/`mass`+`engine`, `calc add`, a passing `withinMassLimit` and a
  failing `overMassLimit` constraint, `requirement SafeMass`). Ideal for package-scoped vs
  qualified lookup, and for pass *and* fail constraint paths in one file.
- `internal/repl/testdata/action_debug.sysml` — `Debug::tally` with named nodes
  `start, accumulate, end`, so `%break accumulate` has something to stop at; completes with
  `total = 5`.
- `internal/repl/testdata/state_debug.sysml` — `Debug::Cycle`, timed transitions at +10 and +5.
  Good for `%advance` accumulation: `%advance 1` then `%advance 9` reaches the event due at 10
  (`working`), and ten successive `%advance 1` calls process exactly two events (one at t=0, one at
  t=10) with zeros in between — a per-call deadline that did not accumulate would never reach t=10.
- `internal/core/runtime/testdata/conformance/state_orthogonal_regions.sysml` — `Test::TrafficLight`
  with two regions; `%current` should print one state per region joined by `|`
  (e.g. `start | start`, then `Walk | Green`), never `<unknown>`.
- Write your own for ambiguity: the same simple name (`part def Vehicle`) in two packages forces
  `error: symbol "Vehicle" is ambiguous: Alpha::Vehicle, Beta::Vehicle (use a qualified name)`.
- Write your own for parse errors (e.g. `attribute mass = ;` plus a missing `}`): the parser never
  panics, so the REPL should print diagnostics and keep accepting commands.
- `internal/core/runtime/testdata/conformance/state_body_state_local_member.sysml` — `test::monitor`,
  whose substate `working` declares `localGain` that its own entry action reads together with the
  package's `pkgBonus`; `%state monitor` + `%advance 1` reaches `done` with `result = 5.00`. The
  scope-regression canary for states declared directly in a machine body.

## Variations, variants and redefinition-inherited members

Fixtures live in `internal/core/runtime/testdata/conformance/`: `variation_attribute_selection.sysml`
(`test::idealDiamond`), `variation_part_selection.sysml` (`test::electricVehicle`),
`variation_interface_selection.sysml` (`test::nestedAssembly`), `variation_unselected.sysml`
(`test::unconfiguredDiamond`) and `ballandchain_variant_configuration.sysml`. Each `.expected.json`
is the cheapest source of the values `%features` should print.

- **Variant rendering.** A bound variation slot prints `name = variantName (Instance ID: n)` with the
  variant's nested values indented under it (`engine = electric (Instance ID: 2)` / `power = 150.00`).
  A plain `name = Instance(ID: n)` for a variation feature means nothing was bound.
- **Assert a computed attribute, not just the variant name.** `curbMass = 1200.00` (= `900 + mass`)
  distinguishes the `electric` variant (mass 300) from `petrol` (mass 200 → 1100); the variant label
  alone would look the same if the wrong nested values were materialized.
- **Always include a constraint that must be *violated*.** `variation_attribute_selection` asserts
  both `isIdeal` (satisfied) and `notShallow` (violated). An implementation where
  `x == x::variantName` returned true for any variant would still show `isIdeal: satisfied`, so the
  violated one is the only real discriminator. `%features` renders these inline as
  `name: <constraint: satisfied|violated>` — note `%satisfy` answers
  `no satisfaction assertion in the session` for `assert constraint` members, so use `%features`.
- **Error paths** (all are per-slot `<error: …>` lines, and the dependent computed slot repeats the
  cause): unselected → `variation has no variant selected: <usage>.<feature>`; a name that is not a
  variant → `not a variant of the variation: X is not a variant of Y (variants: a, b)`; two
  selections (`= (cut::cutIdeal, cut::cutShallow)`) → `more than one variant selected: variation cut
  selects 2 variants (cutIdeal, cutShallow)`. Assert the *variant list* is present — a message that
  stops at the offending name is the weaker pre-fix shape. Follow each with `%eval 1 + 1` → `= 2` to
  show the session survived, and wrap piped runs in `timeout` so a hang shows as non-zero exit.
- **Redefinition merge canary.** A three-line model is enough:
  `part base : Outer { part :>> inner { attribute :>> b = 7.0; } }` plus
  `part derived :> base { part :>> inner { attribute :>> a = 1.0; } }` where `Outer` computes
  `t = inner.b`. Fixed builds print `inner = {a = 1.00, b = 7.00}` and `t = 7.00`; a build that drops
  inherited nested members prints only `a = 1.00` and
  `t: <error: slot derived.t: member b not found in instance>` — a perfect A/B against the parent
  commit. Name the part something other than `derived` if you want to avoid the (harmless)
  `"derived" is a reserved keyword` warning.
- **A valueless feature of a value type (`attribute d : Real;`) renders as `= <unset>`** since the A3
  work (`runtime.Context.HoldsNoValue` + `runtime.UnsetText`); a collection of them reads
  `[<unset>, <unset>]` and no `(no features)` block is printed under it. On revisions *before* that
  change the same slots read `= Instance(ID: n)` / `(no features)` (also `ringCost`, `ringPort`), so
  a parent-commit binary is the ideal A/B. Things that must stay object-shaped: a class-typed part
  (`Instance(ID: n)` with its nested features, and a genuinely featureless `part def Empty` still
  legitimately shows `(no features)`) and a value type that *does* declare features
  (`attribute def Point { attribute x : Real = 1.0; }` still expands). Enums, strings, booleans,
  quantities, derived attributes and `null` are untouched.
  Caveat worth re-checking on any follow-up: an expression over an unset slot
  (`attribute n : Real = d + 1.0;`) still reports
  `<error: slot q.n: type mismatch: operator '+' is not defined for an instance and a Real>` — the
  diagnostic still says "an instance" rather than unset, i.e. the unset spelling has not reached the
  type-mismatch wording.
- **Surfaces to check together for any value-rendering change**, since they share `formatValue`:
  `%features` / `%eval` in the REPL, `sysml <model> -instantiate <fqn> -e <expr>` (same text), the same
  run with `-json` (the JSON encoder escapes it, so grep `\u003cunset\u003e`, not `<unset>`), and the
  gRPC/opensysml path. On the Python side `opensysml.UNSET` is a falsy singleton spelled `<unset>` and
  distinct from `None`: assert `inst.d is opensysml.UNSET`, `inst.d is not None`, `bool(inst.d) is
  False`, `inst.get_feature('d').value.WhichOneof('kind') == 'unset'` with `materialized=True`, and
  `'&lt;unset&gt;' in inst._repr_html_()`. `Value.unset` is send-only: the client cannot build it
  through `_python_to_value`, so to prove the server refuses it, hand-build the request and call the
  stub directly —
  `conn._stub.EvaluateCalc(sysml_pb2.EvaluateCalcRequest(model_hash=m.hash, symbol_id='P::Add',
  arguments=[sysml_pb2.Value(real_value=1.0), sysml_pb2.Value(unset=True)]))` answers
  `error = 'calc argument could not be read: unset is not a value a caller can supply'` (no result),
  and a follow-up `conn.calc('P::Add', m.hash, [1.0, 2.0])` → `3.0` proves the service survived.
  The refusal arrives **in band** — `resp.error` with `failure_reason=FAILURE_REASON_EVALUATION`, not
  an `INVALID_ARGUMENT` status exception — so assert on `resp.error`, and note the stub method is
  `EvaluateCalc` (there is no `Calc`).
- **Getting the CLI value surface wrong wastes a run:** `-instantiate <fqn>` *alone* prints only the
  creation lines and no slot values — the value surface is `-instantiate <fqn> -e <fqn>::d`, while
  `-e` *without* `-instantiate` answers `sysml: "…" has no value to evaluate` and exits 1 (the
  no-instance path, not unset). In `-json`, grep `u003cunset` — a pattern like `u003cunset.003e`
  misses, because `\u` is two characters. And write the null case `attribute nul : Real[0..1] = null;`:
  `Real = null` is itself a multiplicity violation and you end up debugging that instead.
- **Known limits, so don't plan around them:** `%features` takes only the instantiated usage's own name
  — `%features test::electricVehicle::engine` answers `no instance of …`, and
  `%eval test::electricVehicle.engine.power` answers `usage test::electricVehicle has no value`.
  Nested traversal is only observable through the indented nested rendering of the top-level `%features`.
- **Careful with `clear` while recording:** typed at the `sysml>` prompt it is parsed as a
  declaration (`1:1: error: expected a namespace member`) *and* drops previously created instances,
  so the next `%features` says `no instance of …`. Use `%clear` to reset the session, and clear the
  screen before entering the REPL.

### `variant` used outside a variation, and per-variation-point variant objects

This section describes behavior added by PR #128, so it applies only once that PR is on `main`; the
"old" shapes below are what current `main` prints, so they double as A/B canaries. Three easy
discriminators for changes in the variant/variation layer:

- **A `variant` member whose owner is not a `variation` must stay an ordinary feature.** Model:
  `part def P { attribute k : Real = 2.0; variant attribute x : Real = 1.0; }` +
  `part p : P { attribute total : Real = k + x; }`. Correct behavior: loading prints a *warning*
  (code `variant-outside-variation`) —
  ``variant x is declared in P, which is not a variation, so it offers no choice; declare its owner `variation` or drop `variant` `` —
  and `%features M::p` still shows `k = 2.00`, `x = 1.00`, `total = 3.00`, with `%eval M::p.x` → `1.00`.
  A build that skips every `DeclaresVariant` member instead prints no warning, omits `x`, reports
  `total: <error: slot p.total: type mismatch>` and `error: evaluation failed: member x not found in
  instance`. The same must hold for a package-level `variant` (warns, still readable through a
  computed attribute such as `h = loose + 1.0` → `6.00`) and for a `variant` with no value (warns,
  renders as an instance of its type). Genuine variants *inside* a `variation` must NOT warn —
  loading the `variation_*` conformance fixtures should stay diagnostic-free.
- **Two variation points selecting the same variant must not share one object.** Model: an
  `attribute def Shade { attribute lum : Real = 3.0; }`, a
  `variation attribute def ShadeChoice :> Shade { variant attribute bright :> Shade; variant attribute dark :> Shade; }`
  and a part with `attribute front : ShadeChoice = front::bright;` plus
  `attribute rear : ShadeChoice = rear::bright;`. Correct behavior prints two *different* instance
  IDs (`front = bright (Instance ID: 2)` / `rear = bright (Instance ID: 3)`); a build whose variant
  object cache is keyed only by `{owner, variant}` prints `Instance ID: 2` for both. The instance ID
  is the only visible signal here, so read it carefully.
- **A variation point may inherit its variation-ness, and its variants are still choices.** Model:
  `variation part def EngineChoice :> Engine;` with
  `part def Car { part engine : EngineChoice { variant part electric : Engine { attribute :>> power = 150.0; } } }`
  and `part ev : Car { part :>> engine = engine::electric; }`. Correct behavior: no warning and
  `engine = electric (Instance ID: 2)` with `power = 150.00`. A build testing the owner by keyword
  only either reports `not a variant of the variation: electric is not a variant of engine (variants:
  electric, …)` (self-contradictory) or `usage engine::electric has no value` plus a spurious
  `variant-outside-variation` warning. The same applies when the owner redefines a variation usage
  (`part :>> engine { variant part … }`) without restating `variation`. Drop the variants' own
  `: Engine` to test the second half: a variant specializes its variation point, so untyped
  `variant part petrol;` must still print `power = 100.00` (Engine's default) rather than
  `(no features)` — that shape is the sharper canary, since it needs the supertype edge, not just
  the selection.

## Testing lexical scope of behavior bodies (declaring-scope changes)

When a change claims "expressions in an action/state body now see their declaring scope", the same
model must be run on both binaries — but pick fixtures that can actually distinguish the layers,
because the obvious ones cannot:

- **A state fixture whose states share the machine's imports proves nothing about per-state scope.**
  If every name a state's behavior uses is also visible from the machine (or the package), a
  machine-scoped and a state-scoped lookup give identical output. To isolate the state's own scope
  the state must declare a member that *only* its own scope can answer
  (`state working { attribute localGain = 4.0; entry { assign result := localGain + pkgBonus; } }`),
  and for nesting a substate inside a `region` must declare one read by its own entry action.
  A broken build reports `error: event processing failed: enter state: entry action: eval assignment
  RHS: unresolved reference: localGain`.
- **Always include a shadowing case, because the failure mode is a wrong value, not an error.** Give
  a state (or an action, or a loop/if block) a member whose name also exists at package level with a
  *different* value, and assert the inner one wins. A build that hands out the enclosing scope
  completes happily with the package's number (e.g. `result = 100.00` instead of `4.00`), which no
  error-message check would ever catch. The three-layer action shape (package `h`, action `h`,
  block-local `h` → `seenAction = 2.00`, `seenBlock = 3.00`) is the equivalent for action bodies.
- Substates are **not** entered by nesting a succession inside a plain `state`; that shape runs
  the outer entry and completes without ever entering the inner state (`innerRan = 0.00` on every
  revision, pre-existing). Use the `region` form from `state_fork_join_pseudostate.sysml`, with a
  transition naming the substate (`transition first init then inner;`), or the machine never descends.
- Quantity values are the cheapest scope probe for imports, since a missing `SI::*` shows up as
  `not a quantity expression: not a measurement unit: unresolved unit s` rather than a wrong number.
  Pair each such model with a bare-`Real` rewrite and assert the same magnitudes, which catches a
  silently dropped unit factor.
- Comparing whole-session output across binaries with `diff` is a good regression sweep, but the
  `State data:` / `Results:` blocks are printed in **map iteration order**, so lines legitimately
  reorder run to run (`leftDone`/`rightDone`, a lone `status`). Diff the *values*, or treat a
  pure line-reordering diff as identical.

## Things to exercise (and their expected shapes)

- There is **no `%what` command** — check `%help` before believing a task description. The lookup
  surface for "does this name resolve?" is `%instantiate` / `%features` / `%eval` (a `part def` is
  easiest via `%instantiate`, an attribute via `%eval`), all funnelling through
  `internal/repl/lookup.go`. A request phrased as "`%what`/lookup" means those.
- Symbol-taking commands: `%instantiate %features %eval %calc %constraint %requirement %action %state`.
  All go through one helper (`internal/repl/lookup.go`), so test each with a **simple** name and a
  **qualified** one.
- `%slots`, the pre-0.1.0 spelling, is **removed**: it reads as `unknown command "%slots"` and is
  offered by neither `%help` nor tab completion. `%features` is the only listing command, so a
  script written against the old spelling has to be updated.
- A part whose type contains its own kind does **not** print a "materialization is bounded" note; the
  bounded walk renders the nested feature as `child : Node (not expanded: contains its own kind)`
  after expanding one level. Don't grep for wording the binary never emits — capture the real line
  over a pipe first. Follow it with `%eval 1 + 1` → `= 2` to show the session survived the walk.
- `%step` drives a state session: it polls change conditions, dispatches the next event or
  pending signal, or runs one do-behavior round. `%continue` remains action-only and answers
  `error: no active action session (use %action <name> first)` in a state-only session.
  Use repeated `%step` or `%advance <time>` and inspect `%current` for state-machine progress.
- When comparing two variants of the same model (e.g. a `private import` version against a
  `public import` or bare-`Real` rewrite), load each in a **separate REPL process**. Loading both
  into one session makes the shared simple name ambiguous —
  `error: symbol "propagate" is ambiguous: DescentQ::propagate, DescentR::propagate (use a qualified
  name)` — and qualifying it is not a reliable escape, since one bad submission in between can
  invalidate the other snippet. `./bin/sysml <file>` per variant keeps the runs independent.
- Meta-commands are only understood at the `sysml>` prompt: a shell habit like `clear; %action foo`
  is parsed as SysML, pollutes the session buffer, and can make an already-loaded package's symbols
  stop resolving (`unresolved reference: DescentR::propagate`). Type shell and REPL commands in
  separate turns, and `%clear` (or restart) if a stray line lands in the buffer.
- `%satisfy` takes no argument (every satisfaction assertion the model states) or the name of the
  element stating them, since `assert satisfy … by …` is anonymous.
- Instances are keyed by resolved FQN, so `%instantiate Vehicle` then `%features Demo::Vehicle` must
  hit the same `ID`, and the reverse spelling too. Differing IDs = broken keying.
- Qualified attribute access works with a full FQN (`%eval Demo::Engine::power` → `= 300.00`) but a
  **partial** qualification (`%eval Engine::power`) is `unresolved reference: …` — the qualified path
  goes through the index, which wants the whole FQN. Expect this, don't file it as a bug without
  checking intent.
- `%break <node>`: unknown node names are rejected *with the valid node list*; after a stop,
  `%tokens` should print the **node name** (`Token 1 @ accumulate`), not a Go type like `*ast.Usage`.
- A breakpoint on the node a token **already occupies** must fire: right after `%action Debug::tally`
  the token sits at `start`, so `%break start` + `%continue` has to pause at `start` rather than
  running to completion. Then check the mirror case — resuming with `%step` or `%continue` must not
  re-trigger on that same node (the executor records fired token/node visits). Test both directions;
  "fires" and "doesn't re-fire" are separate bugs.
- `%advance` also drives a state's **do behavior**: when the only queued event is past the deadline
  but the current state has do actions left, they run and the output gains a
  `Do behavior actions run: N` line (`internal/repl/testdata/state_do_far_event.sysml`, event at
  t=100 — `%advance 1` runs 2 do actions and `%current` shows `count = 2`). Assert the clock does
  **not** jump to the far event and the event stays queued, and that repeating small advances does
  not re-run the behavior (`count` stays 2, `0 event(s) processed`).
- `%advance` arguments: `abc` → invalid-time error, `-5` → must-not-be-negative, no arg → `usage:`,
  `1e400` → invalid (overflows a float64), a trailing `s` is allowed (`%advance 10s`), `0` still
  drains events already due, and a huge value → `No pending work - simulation time is now <clock>`
  (must not hang).
- `NaN`, `inf`, `-Inf` and `NaNs` (the `s` suffix is trimmed first, so `NaNs` parses as `NaN`) must be
  rejected with `expected a finite number of time units`. The important follow-up assertion is that
  the **clock survives** a rejected argument: run `%current` afterwards and check `Time:` is still a
  real number, then that normal advances still accumulate. A regression here poisons the clock to
  `Time: NaN` for the rest of the session, which no single error-message check would catch.
- **Two clocks, deliberately.** `stateSession.now` is the debugger clock (seeded from the executor,
  incremented by every requested duration, and it keeps moving even when nothing is queued);
  `exec.CurrentTime()` only moves when an event is processed. So `%advance` prints
  `✓ Advanced to <debugger clock>` plus `Last event at: <executor clock>`, and `%current` prints both
  `Time:` (debugger) and `Last event at:` (executor). `Advanced to 20.00` above `Last event at: 15.00`
  is correct, not a bug. When asserting on accumulation, use `%current`'s `Time:`.
- Error paths that must never panic: unloaded session (`no declarations loaded`), debugger commands
  with no active session, wrong-kind symbols (`%action` on a part def → `is not an action`),
  nonexistent names, and malformed qualified names (`Demo::`, `::Vehicle`, `Demo::::Vehicle`).

## Accepts in actions (accept/send semantics)

An action token that reaches an `accept` with no matching message **parks** instead of failing:
`%step` reports `State: Waiting` and keeps the token at the accept node. Two paths worth testing:

- Satisfiable: `internal/core/runtime/testdata/conformance/action_send_accept.sysml`
  (`%action communicator`) — `%break counter` pauses on the accept node, and resuming completes with
  `number = 50` / `n = 50` (the typed accept skips the String and takes the Integer). Loading these
  fixtures used to print a tier-2 `unresolved reference: n` diagnostic for the `assign` that reads
  the accepted parameter while the runtime bound it anyway; **PR #196 fixed that**, so on current
  revisions loading them must be clean. Seeing that diagnostic again is a regression of the
  accept-payload scope contribution, not harness noise.
- Unsatisfiable: write a one-accept action with nothing sending to it. `%step` → `State: Waiting`,
  and `%continue` must return a typed
  `accept deadlock in action <name>: nothing can post the awaited message (...)` — **never** hang.
  A breakpoint on the parked node still fires, and `%stop` then `%continue` gives
  `no active action session`.

Always run these under `timeout` when driving over a pipe; a hang is the failure mode to catch.

### An accept node's payload as a body-scoped name (PR #196)

`internal/core/resolve/accept_payload.go` contributes `action r accept msg : T;`'s payload to the
**body** the accept node is declared in, so sibling nodes read it by simple name. Test it on both
surfaces — `bin/sysml -validate f.sysml` for the diagnostic and
`printf '%%load f\n%%action <n>\n%%continue\n%%quit\n' | timeout 30 ./bin/sysml` for the value —
because check-clean alone never proves the runtime bound anything.

- The A/B against a parent-commit binary is what makes this convincing: the same model gives
  `error: unresolved reference: msg — did you mean test::communicator::receiver::msg?` (exit 2) on
  the old binary and `✓ … no errors` on the new one. Payload names must be **non-keyword** —
  `accept first : Integer` is parsed as an initial node and derails the whole graph with
  `action has multiple initial nodes`; `first`, `after`, `then` are all reserved.
- Positive shapes worth one run each, with the numbers a correct build gives (send value in
  parens): later sibling reads `msg + 1` (42 → `seen = 43`); nested `if msg > 5` + `while` in a
  sibling node (7 → `total = 21`); two accepts binding `alpha`/`beta` (11, 30 → `sum = 41`);
  reader declared textually **before** the accept but sequenced after it (4 → `seen = 5`).
- **Shadowing is the case only a value can catch.** A package-level `attribute msg : Integer = 1;`
  keeps the model check-clean on *both* binaries, so assert `seen = 9` (the sent payload) and not
  `1`. Note a *body*-level `attribute msg = 111` in the same action is a weak probe: the accept
  writes the same value slot, so `seen = 9` either way — prefer the package-level form.
- Negatives that must stay `unresolved reference: msg` at exit 2, and each exercises a different
  guard: a misspelling in the same body (now gains `— did you mean msg?`); a **sibling** action's
  node in the same package; a package-level `attribute` initializer; a wildcard
  `import src::communicator::*` into another package; and a `part def` body declaring an action
  node with the accept (`sharesBodyFeatureSpace` rejects a part — this one is easy to forget).
- A node that executes **before** the accept binds must fail, not read stale data:
  `error: execution failed: eval assignment RHS: unresolved reference: msg` in ~0.1 s, with
  `%tokens` afterwards still showing `Token 1 @ <node>` (session survives).
- A **qualified** `receiver::msg` reference resolves at check time and then fails the run with
  `usage receiver::msg has no value` — identical on both binaries, i.e. pre-existing rather than a
  regression of this change. Always A/B it before reporting it as a defect.
- Cheap corpus gate for a resolve change: loop `internal/core/runtime/testdata/conformance/*.sysml`
  and `examples/*.sysml` comparing `grep -c 'error:'` counts new vs old binary and print only files
  where new > old (~1 min).

## Addressed sends and per-object message identity (`send S() to t`, PR #267)

To observe *which object* consumed a message you must drive one performer at a time. `%state` and
`%action` take an object argument (`%state <machine> <object>`, `internal/repl/meta.go:320`) but the
object must already exist, so **`%instantiate <Pkg>::<part>` first** — otherwise every command
answers `error: no instance of "…" (use %instantiate first)` followed by `no active state machine
session`, which reads like a broken session.

The fixture shape that distinguishes confined from leaking delivery is two structurally identical,
unconnected parts of one type, each with a `via`-qualified accept **and** a catch-all:

```
transition first waiting accept Ping via inPort then received;   // the addressee
transition first waiting accept Ping             then strayed;   // the leak detector
```

Run the same model once per performer: the addressee must reach `received`, every other object must
stay in `waiting`. `strayed` is the pre-fix signature (a same-named port of another object consuming
the message) and it appears for *both* objects on a pre-#267 binary — always A/B against a
parent-commit binary, since "it ran and ended somewhere" proves nothing on its own.

Limits worth knowing before writing fixtures:

- **Nested performers cannot be driven from the prompt.** `%state <M> Pkg::one::mid::inner` fails
  with `no instance of "Pkg::Mid::inner"` even after `%instantiate Pkg::one` materialized it (visible
  in `%slots`). So a nested/composite target (`one.mid.inner.inPort`) is only observable *negatively*:
  put the catch-all accept on the **sending outer** object and assert it stays `waiting` with no
  error — the deep path resolved (else it would be a typed error) and did not leak outward.
- **`accept … via <dotted.path>` does not parse** in a state transition (`expected a body member` /
  `expected the target of the transition after 'then'`) nor in an action node (`expected ';' after
  accept action`). Only `send … via p.q` takes a nested path. Don't design an accept-side nested-port
  fixture; use `conformance/port_nested_port_path.sysml` (connect `p.q` to a simple `sink`) instead.
- Addressing needs **no connector**: `send Ping() to two.inPort` from `one` delivers to `two`, and the
  sender must not consume it. That cross-object case is the cheapest positive proof of identity.
- `send X to self` resolves to nothing and is a silent no-op on both old and new binaries (no
  receiver is named `self`) — expect `waiting`, not an error. A target naming a **part** rather than a
  port (`one.mid.inner`) is also delivered-but-unaccepted, not an error.
- The two unroutable wordings are distinct and both must stay reachable
  (`internal/core/runtime/routing.go`): an addressed target gives
  `send reaches no receiving port: "alpha.count" names no port of an object the sender can address`,
  while a `via` port gives `port "lonely" is joined to no port that can receive it` /
  `port "dst" is joined only to outbound ends (src)`. A pre-#267 binary **completes successfully**
  (`✓ Action completed`) for the addressed-target cases, so assert the error text, not just non-zero
  exit. From a state entry action it surfaces as
  `error: event processing failed: enter state: entry action: send reaches no receiving port: …` with
  `Current state: <none>`; the REPL session must survive it (`%tokens` still shows `Token 1 @ sender`,
  `%eval 1 + 2` still answers `3`).
- Regression set that must be byte-identical old vs new for any routing change:
  `port_nested_port_path` (`got = 7`), `port_interface_typed_connection` (conjugated `~Chan`,
  `got = 13`), `send_no_reachable_receiver`, `send_into_outbound_only_end`,
  `state_send_self_signal` (`send Ping to Driver` → `done`), `action_send_accept` (accept-node target
  → `n = 50`, `number = 50`).
- Loading these conformance fixtures in the REPL prints pre-existing
  `unresolved reference: Integer — did you mean ScalarValues::Integer?` noise; it reproduces on the
  parent binary and does not stop execution.

## Standard SysML v2 behavioral notation (flows, triggers, sends, transition effects)

Testing this family end-to-end has a few traps that cost a whole run if hit late:

- **Object-addressed signals can be injected with `%send <signal>[(p=expr)] to <object>`.**
  Start `%state <machine> <object>` after `%instantiate <object>` when the object does not
  already exhibit the machine. `%send` reports the transition it would enable; `%step` actually
  dispatches it. Omitting `to` addresses the debugged performer. Use `%events` to distinguish
  signals in flight from the timed-event queue. For a port-addressed signal, use a model sender:
  give the machine `port out : P; port in : P; connect out to in;` and a state whose
  `entry send Item(9) via out;` feeds the transition (the shape of
  `internal/core/runtime/testdata/conformance/state_transition_accept_via_port.sysml`). The shipped
  `state_transition_accept_payload.sysml` has **no** sender — its event comes from the
  `.expected.json` `events` array, so in the REPL it waits until a signal is injected.
  The harness event array is not automatically replayed by `%load`.
- **Always run a signal-driven state model on both the REPL and the gRPC/conformance path and
  diff them.** They disagreed until `ProcessNextEvent`/`%advance` learned to dispatch a pending
  context-bus signal: a port send produced by an entry action was delivered by `RunToCompletion`
  (gRPC `execute_state`, conformance) but not while stepping, so the machine parked with the
  attribute at its initial value. `TestAdvanceDeliversPendingPortSignal` locks the parity; a
  REPL-only result understates the feature and a gRPC-only result hides a debugger gap.
- **A transition effect is worth testing per effect form**, because they lower differently:
  `do assign x := <expr>` and a performed-action reference (`do perform Bump then s;`) reach the
  executor through different lowering paths, and the performed form used to abort with
  `transition effect: unsupported action type: *ast.Membership`. A parse-only check on such a form
  proves nothing — always drive the transition and assert the attribute changed.
- After any change to statement-termination in `parser/behavior.go`, re-run a **negative** case in
  the same breath: a genuinely missing `;` in an action body (`assign n := n + 1` followed by
  another statement) must still report `expected ';' after assignment`, and one in a state body
  must still report `expected '{' or ';' after declaration`.
- `accept at <t>` / `accept after <d>` in an **action** body are supposed to fail fast with
  `no clock to wait on: … a time event is only waited on by a state machine's transitions` — check
  they do not park forever. In a **state** transition, plain `accept after 5` works while
  `accept after 5 [s]` fails with `schedule events: time duration must be constant, got quantity`;
  prefer the unitless form unless quantities are the thing under test.
- Both spellings of a machine's start state work: an explicit `initial <s>;` and the standard
  `entry; then off;` succession out of the body's own entry subaction. If a bodied
  `exhibit state s { … }` reports `initialize state machine: no initial state found in state
  machine <name>`, neither reached lowering — check the state body actually parsed.
- Corpus regression sweep for parser work: `./bin/sysml <file>` over
  `/home/ubuntu/corpus/apps/*.sysml` and `/home/ubuntu/corpus/dragon/Dragon.sysml`, counting
  `error:` lines, is a cheap high-signal gate — read each remaining message and classify it as
  structural/unresolved-library vs behavioral rather than trusting the count alone.
- Don't name a opensysml probe script `grpc.py`: `sys.path[0]` shadows the real `grpc` package and
  opensysml dies with `partially initialized module 'opensysml'`, which looks like a client bug.

## Fork/join and an action's shared value space (PR #170)

An action performance holds **one** value space (`ActionExecutor.data`, read back by `Results()` and
`Data()`); a fork duplicates control only, a join merges nothing, and a retiring token carries
nothing out. Consequences worth asserting whenever anything in `internal/core/runtime`'s action path
changes:

- Every branch's writes must appear in `Results:` together. The historical bug (pre-#170) was that
  each token carried its own copy of the data, so only the **last-retired** token's snapshot
  survived — a broken build still completes with `✓ Action completed`, just missing values, so
  "it ran" proves nothing. Always A/B against a binary built from the parent commit and assert the
  exact numbers.
- Fixtures that distinguish working from broken, and the values a correct build gives (an incorrect
  build zeroes the branch that retires first):
  `fork` + two branches assigning `x`/`y` → `x=1 y=2`; three branches → `1/2/3`; nested forks where
  the inner join's successor reads what the inner branches wrote; a branch whose body is a
  `while` loop (`n=10 i=5` — the loop runs entirely within one token step); a branch that reads a
  feature the other branch wrote (`seen=42`, since a read after the other branch's write sees it);
  an `accept` in one branch (`action rcv accept n : Integer;` with the other branch sending);
  a branch ending at a node with **no succession** (no join) — both branches' values must survive;
  a fork whose branch invokes a nested action with `in`/`out` params that itself forks (only
  parameters cross the boundary — the callee's own locals must NOT appear in the caller's results).
- Both branches assigning the **same** attribute is deterministic last-write-in-step-order, not a
  merge: assert a single stable value over repeated runs (`w=10` for branches writing 10 then 20)
  and that it never panics, hangs or drops the feature.
- `%tokens` prints token **locations** followed by one shared `Values:` block, never per-token
  values. At a fork expect `Token 2 @ left` / `Token 3 @ right` with a single `Values:` — per-token
  blocks or duplicated names are the pre-#170 display. After completion `%tokens` says
  `No active tokens` and the values only show in the `Results:` of the final `%continue`.
- An unsatisfiable `accept` inside a fork branch must still fail fast (exit 2, ~0.1 s) with
  `accept deadlock in action <name>: … ; N token(s) blocked for another reason` — never hang.
- Model-writing traps met while building these fixtures: `after` is a reserved keyword (bad node
  name); an accept's bound parameter is only resolvable from other nodes when the owner is written
  `action Foo { … }` rather than `action def Foo { … }` — otherwise the reader body reports
  `unresolved reference: n`; the CLI has **no** flag for action inputs, so exercise `in`/`out`
  parameters by having a caller action invoke `action call = Callee(a = 3, b = 4);`.

## Control flow inside an action node body

`while`, `loop … until` (braced and unbraced), `for … in` and `if`/`else` execute inside an
`action <node> { … }` body. The whole body runs within **one** token step, so `%step` past the node
jumps straight from `Token 1 @ <node>` to the successor with the loop's final values already in the
token data — do not expect one step per iteration. Drive these with the standard shape:

```bash
printf '%%load f.sysml\n%%action tally\n%%continue\n%%quit\n' | timeout 30 ./bin/sysml
```

and assert on the exact numbers in the `Results:` block; "it completed" proves nothing, since the
historical bug (pre-PR #79) was that the statements were *silently dropped at lowering* and the
action still completed, just with the wrong total. The cheapest way to prove a control-flow change
is real is an A/B against a binary built from `main` in a `git worktree` — same model, different
number.

Things that must fail rather than hang: the REPL builds its runtime context with a step budget that
defaults to **10000000** (`runtime.DefaultMaxSteps`, `internal/core/runtime/budget.go`; sessions
carry the five budgets via `Session.SetBudgets(runtime.Budgets)`), and every loop iteration spends
several steps, so a runaway loop (or an empty loop body, whose condition can never change) returns
`error: execution failed: eval … : evaluation step limit exceeded (10000000 steps; raise OPENSYSML_MAX_STEPS to allow more)`
in under a second (0.7 s measured). Always follow the failure with another meta-command (`%tokens`, `%instances`)
to prove the session survived — `%tokens` still shows the token parked at the node with its partial value. A
`for` over a non-collection gives
`action node <n>: 'for' collection must be a sequence or a set, got constant` before PR #202, and
`action node <n>: type mismatch: 'for' iterates a collection, and expression is not one` after it —
and only for a value that states a *computation* (a body expression). See the next section.

### `for` over a scalar is a typed error (PR #231)

Since PR #231 `forElements` (`internal/core/runtime/statements.go`) only iterates a **sequence** or a
**set**; `null` iterates zero times, and *everything else* — Integer, Real, Boolean, String, an
expression — is `type mismatch: 'for' iterates a collection, and <describeValue> is not one`. Exact
texts observed on `bin/sysml`, worth asserting verbatim:

| input | message |
|---|---|
| scalar `Integer` / nested `for` whose inner input is scalar | `error: execution failed: action node <n>: type mismatch: 'for' iterates a collection, and an Integer is not one` |
| `Boolean` | `… and a Boolean is not one` |
| `String` | `… and string is not one` (no article — `describeValue` renders it lowercase) |
| the same `for` in a **calc** body | `error: calc invocation failed: calc <fqn>: calculation body: type mismatch: 'for' iterates a collection, and an Integer is not one` |
| attribute declared with **no value** (`attribute missing : Integer;`) | `error: execution failed: eval 'for' collection: unresolved reference: missing` — the loop input never reaches `forElements`, so do not expect the new wording here |

Testing notes for this class of change:

- The pre-fix contrast is the convincing frame: build the parent commit into `/tmp/old-sysml` and the
  same model **completes** with the counter at `1` (and the scalar calc answers `4` instead of
  erroring) — a silent single iteration, which is exactly what a broken build looks like.
- Zero-iteration inputs must still *complete*: give the fixture an `assign marker := 1;` **after** the
  loops so the `Results:` block proves execution continued (`emptyVisited = 0`, `nullVisited = 0`,
  `marker = 1`), not just that nothing errored. `attribute absent : Integer[0..1] = null;` is the way
  to get a `ValNull` loop input.
- Check the strictness did **not** leak into `elementsOf` (collection operators, multiplicity):
  `%eval SequenceFunctions::size(7)` → `= 1`, `%eval 7->size()` → `= 1`,
  `%eval SequenceFunctions::isEmpty(7)` → `= false`.
- `%calc` takes **positional** arguments only: `%calc P::C 4`. `%calc P::C n=4` answers
  `error: named arguments are not supported here; pass arguments positionally`.
- An entry action nested inside a part is reachable by its qualified path (`%action test::p::a`), which
  is the REPL twin of the conformance harness's qualified `"evaluate": "test::p::a"`.
- After every failing case, run `%eval 1+1` → `= 2` to prove the session survived.

### Block token flows: nested actions and `perform` in a loop body / `if` branch (PR #202)

Before PR #202 a loop body or `if` branch containing an **action node** (a nested `action`
declaration, or a `perform`) aborted the run at lowering:
`action node iterate: action usage "scale" in a body is not executable` /
`'perform' in a body is not executable`. After it, the block becomes a token flow of its own
(`internal/core/lower/block_graph.go`) and runs. That old message is the **ideal A/B contrast** —
build the parent commit into `/tmp/old-sysml` (recipe above) and run the same model through both;
the old binary aborting while the new one prints numbers is far stronger than a screenshot of the
new numbers alone.

Things that cost time when writing models for this area:

- **A `perform Foo;` binds `Foo`'s `in` parameters by NAME from the enclosing scope**, and merges
  its `out` values back under their own names. So `action def Bump { in i; out doubled; }` performed
  inside `for i in 1..3` works because the loop variable is *also* called `i`; rename either side and
  the run dies with `invoke action Bump: … unresolved reference: i` — which reads like a feature bug
  but is just a name mismatch in the fixture. Always name the performed action's inputs after the
  values that exist where it is performed.
- **`perform <an action def>` also emits a pre-existing static diagnostic**
  `error: references target must be a usage, found actionDef` on *every* revision (the shipped
  conformance fixtures do it too) while still executing correctly. Do not report it as a regression;
  check it against the contrast binary first.
- **A nested action's own declarations do not leak outward.** Reading one after the node
  (`assign x := local;`) fails at *name resolution* time, i.e. already at `%load`, with
  `unresolved reference: local` — a cheaper and stronger no-leak assertion than inspecting Results.
  Conversely a value declared *before* the nested action IS readable after it (one block scope), so
  assert both directions.
- **`%trace on` is the best evidence here**: it labels each block-flow node
  (`stmt node scale`, `stmt node deeper`, `stmt node Bump` / `stmt perform`) under
  `iteration N`, so "the perform ran once per iteration" and "the untaken branch ran nothing"
  are directly visible (no `stmt node <else-branch-action>` line at all). Assert zero `ast.`
  substrings in the trace — a Go type name there means a node kind is missing from the label switch.
- The whole block flow still runs inside **one** token step, so `%step` jumps
  `Token 1 @ start` → `@ iterate` (values still at their initial values) → `@ end` (final values).
- A non-terminating `while` that performs an action fails with the **evaluation** step limit
  (`eval assignment RHS: evaluation step limit exceeded (10000000 steps; …)`) in ~4 s, not the
  action step limit — budget ~10 s, and don't assert on which of the five budgets trips.
- To reach the `for`-over-a-non-collection error you need a **body expression**, e.g.
  `attribute notACollection = { in j; j > 1 };` then `for i in notACollection`. Naming a `calc def`
  (`for i in Mk`) gives `unresolved reference: Mk` instead and does not exercise the path.
- **The shipped `*_block_flow_*` / `action_for_over_produced_collections` fixtures emit static
  `unresolved reference` errors when loaded in the REPL** (`select` → "did you mean
  `ControlFunctions::select`?", `Integer` → "did you mean `ScalarValues::Integer`?") because they
  import only what the conformance harness needs. Execution still yields the right values, so this
  is a fixture wart, not a runtime defect — but add the missing imports to your *own* models so the
  recording is clean.

### Raising the budgets (PRs #83, #87)

Five variables, one per runaway bound, each counting a different unit — raising one says nothing
about the others:

| Variable | Default | Counts |
|---|---|---|
| `OPENSYSML_MAX_STEPS` | 10000000 | expression evaluations |
| `OPENSYSML_MAX_ACTION_STEPS` | 1000000 | action token-flow steps |
| `OPENSYSML_MAX_EVENTS` | 1000000 | state machine events, and the events one `%advance` drains |
| `OPENSYSML_MAX_DO_STEPS` | 5000000 | do actions, ditto for `%advance` |
| `OPENSYSML_MAX_ELEMENTS` | 1000000 | collection elements one evaluation holds (~104 MB of `Value`s), the memory bound: `(1..2000000)` reports `collection element limit exceeded`, not the step limit, while a loop building a small collection many times is unaffected |

`%budget` prints the five bounds in force with the variable raising each, so a test can read what a
session runs on instead of inferring it from the environment.

Each bounds **one run** — one `%eval`, `%instantiate`, `%calc`, action or state machine, a stepped-
through run included — not a whole session, so a long session of small operations never runs out; a
run started inside another shares the outer one's budget. Testing the bound therefore needs a single
runaway run, not a sequence of evaluations.

Each overrides the default for both `bin/sysml` and `bin/sysml-grpc`: unset/empty → the default, a
positive integer (whitespace is trimmed) → that value, anything else → the binary refuses to start
(`sysml` exits **2** with `sysml: OPENSYSML_MAX_STEPS="…" is not an integer …` /
`… must be greater than zero …`; `sysml-grpc` exits **1** and logs the same error under
`msg="Invalid service configuration"`). Every unusable value is reported at once, not just the first.
Useful test model — a `while i < N { assign …; assign …; }` body costs roughly 10 evaluation steps
per iteration, so a 100 000-iteration loop completes at the default in 0.13 s and a budget of 100000
stops a 10 000-iteration one:

```bash
OPENSYSML_MAX_STEPS=100000 sh -c "printf '%%action loopn\n%%continue\n%%quit\n' | ./bin/sysml /tmp/loopn.sysml"   # step-limit error
printf '%%action loopn\n%%continue\n%%quit\n' | ./bin/sysml /tmp/loopn.sysml                                  # completes at the default
```

Note the `sh -c` wrapper: `VAR=x cmd | …` only exports to the first process of the pipeline, so put
the whole pipeline inside `sh -c` or the REPL will not see the variable. The gRPC side inherits the
variable through the opensysml auto-start path (the client spawns `~/.opensysml/bin/sysml-grpc` as a
child), so `OPENSYSML_MAX_STEPS=300000 python script.py` is enough — but `pkill -f sysml-grpc` first,
otherwise an already-running service from an earlier value keeps serving.

Tooling trap: running a opensysml script that auto-starts the service from a *non-tty* one-shot shell
tends to return no output at all (the spawned service holds the pipe). Run such scripts with a tty
shell (`tty: true`) or inside the GUI terminal, and use the venv interpreter
(`~/opensysml-venv/bin/python`) — the default `python` in a plain shell has no `opensysml`.
**The venv is not always at `~/opensysml-venv`**: some sessions ship `/home/ubuntu/sysml-venv`
instead, and the system `python` may have a protobuf too new for the generated stubs. Resolve it
once with `ls -d /home/ubuntu/*venv*` and check `<venv>/bin/python -c 'import opensysml'` before
blaming the client. A one-shot non-tty `<venv>/bin/python script.py` did work in practice, so try
it before falling back to a tty shell.

A name declared inside a loop or branch body lives in a block frame and must **not** appear in the
action's `Results:` — check for the *absence* of the line, not just the right total.

## Calc usage semantics: activation snapshots, body-local usages, integer ranges (PR #118)

These four runtime areas are cheap to test from the prompt and each has an unambiguous A/B against
a pre-fix binary, so build the contrast binary from the merge-base first (see above).

- **A calc usage's outputs must all come from one input binding per activation.** The probe is a
  pair of calc defs that differ only in *statement order*: one reads `p.a`, assigns the feature the
  input names, then reads `p.b`; the other reads both outputs before the assignment. Both must give
  the **same** number — the read order is not observable. The pre-fix signature is the interleaved
  one answering from mixed state (e.g. `1020.00` against `1010.00`). `%calc` on each is enough;
  ready-made as `internal/core/runtime/testdata/conformance/calc_usage_outputs_one_binding.sysml`.
  Two assertions must accompany it, since the fix relaxes the memoization key and could over-share:
  a genuine output cycle (`out a = b + 1.0; out b = a + 1.0;`) still has to report
  `cyclic output dependency: output a of calc … depends on itself`, and two usages of one calc def
  with **different** arguments must keep their own values (`p1` at `k=1.0` next to `p2` at `k=5.0`
  → `1005.00`, never `1001` or `5005`).
- **Body-local calc usages inside a `while`/`if`/`for` body** used to fail at
  `calc usage "<name>" in a body is not executable`. The realistic fixture is
  `conformance/calc_rk4_lunar_descent.sysml` (four body-local stage usages `k1..k4` in a `for` body
  plus a nested steering usage): `%calc test::Propagate 3` → `= 15001.72`, and the gRPC side gives
  the full-precision `15001.719185373526` that the `.expected.json` pins. **The REPL prints two
  decimals**, so assert exactness through opensysml `eval`, not the prompt. Keep a two-line `while`
  variant around too — it shows the removed error message on camera, which the RK4 file cannot,
  because on a pre-fix binary RK4 dies earlier on `..`.
- **`..` is the ordered integer sequence the stdlib declares**, not a value kind of its own
  (pre-fix: `unsupported operator: '..': a range is not a value kind the runtime carries`). Five
  prompt evals cover it: `(1..5)->collect { in i; i * i }` → `[1, 4, 9, 16, 25]`, `3..1` → `[]`
  (descending is empty, not an error), `1..2.5` → `type mismatch: '..' requires Integer bounds`,
  `(1..4)->SequenceFunctions::subsequence(2, 3)` → `[2, 3]` (a stdlib function defined *via* a
  range), and `conformance/calc_integer_range.sysml`'s `%calc test::RangeSum 4` → `= 44`, which
  folds `collect`/`select`/`for i in 1..n`/`sum`/`size`/`#(2)` into one number. Loading that
  fixture prints pre-existing tier-2 `unresolved reference: collect` / `select` diagnostics on
  **every** revision — not a regression. Note ranges need a loaded document: on a bare REPL the
  evals answer `no declarations loaded`, which is not the range path at all.
- **A calc usage nested in a part is readable through a feature chain.** `%eval Pkg::lander.mass.mDry`
  and `%eval Pkg::lander.dryMass` (an attribute whose default reads the usage) both answer the
  number; pre-fix both were `usage Pkg::lander has no value`. Reading the usage **without** naming
  an output must name the outputs instead:
  `no value: calc usage mass computes output features (mProp, mDry, mWet); read one of them`.
  The model lives in `internal/core/runtime/part_feature_chain_test.go` as `partChainModel` — copy
  it rather than inventing constants, and take the expected value from that test
  (`mDry = 100.0 + 250.0 * 0.4` → `200.00`); task descriptions quoting other magnitudes usually
  refer to an earlier draft of the model.
- **`opensysml` `Model.find` must accept the FQN it reports**: `m.find("Rhs")` and
  `m.find("test::Rhs")` return the same symbol (`.id == 'test::Rhs'`) and `m.find("test::Missing")`
  is `None`. Remember the package name in the fixture decides the FQN spelling.

Because all five reach the *same* statement engine, feature-chain evaluation and calc memoization,
follow them with the cheap canaries: `%action tally` + `%continue` → `total = 5`, a
`%state Debug::Cycle` + `%advance 1` + `%advance 9` sweep → `working` at `Time: 10.00`, and a gRPC
`get_feature` on `Demo::Vehicle` (`mass` → `materialized=True kind=real_value`, `engine` →
`kind=instance_id`).

## Session-accumulation trap (bites both testers and features)

Whether re-typing a namespace **adds to** it or **replaces** it depends on where the earlier one
came from (`mergeSubmission`, internal/repl/merge.go):

- **Typed earlier at the prompt → merged.** `package Demo { part def Trailer; }` folds into the
  `package Demo` already typed: `note: added to the existing package Demo (its other members are
  kept)`, and `Demo::Vehicle` still resolves. Re-typing a *member* replaces that member and says
  so (`note: added to the existing package Demo, replacing part def Wheel`), which is also what
  drops the instances built from it.
- **Loaded from a file (`%load`) → replaced.** A loaded snippet keeps its identity, so
  `package ActionExecutorDemo { part def Trailer; }` after loading that example reports
  `note: replaced package ActionExecutorDemo (action def SimpleAction, action sequential, action
forkJoin, action conditional no longer declared)` — every member it declared — and the
  file's members are gone. This is the shape that bites a test written against a `%load`ed
  fixture — use a **different package name** to add declarations there.
- An **empty body** (`package Demo { }`) is the deliberate way to empty a namespace, and a
  submission with a different header (or declaring more than one thing) replaces rather than merges.

`Submit` **carries instances over** what a submission did not change (`internal/repl/carryover.go`,
`runtime.Adopt`): after an unrelated `part def B;`, `%instances` still lists the instance with the
**same ID**, `%features` still prints its values, and the next `%instantiate` gets a *fresh* ID rather
than `ID: 1`. What the submission invalidated still goes — redeclaring the instance's own definition,
or a declaration its features are typed by — and then the notice
`note: N instance(s) … dropped because the declarations changed` is expected, with `%instances`
saying so too (`(no instances created; …)`, or `(… also dropped …)` when only some went). An active
`%action`/`%state` debugging session likewise survives an unrelated submission and ends, with a
notice naming the declaration, when what it depends on changes.

Recipes that actually distinguish working from broken here (used to verify PR #168):

- **Survival:** `part def A { attribute x : ScalarValues::Integer = 1; }` + `%instantiate A` +
  `part def B;` → no drop notice, `%instances` → `A (ID: 1)`, `%features A` → `x = 1`. The parent
  commit's binary (see the contrast-binary recipe) prints the drop notice and
  `error: no instance of "A"` on the same input, so run both on camera.
- **Partial loss:** instantiate two definitions, then redeclare only one → `note: 1 instance was
  dropped …` plus `%instances` listing the survivor and
  `(1 instance was also dropped when the declarations changed at submission N — re-run %instantiate)`.
  A `part def D { part t : T; }` instance is dropped by redeclaring `T`, not only `D`.
- **Fresh IDs:** the next `%instantiate` after a survival gets the next unused ID (2, 3, …); an ID
  restarting at 1 while an older instance still lists is the failure signature.
- **Debugger:** `%tokens` is the action-side inspector — `%current` answers
  `no active state machine session` for an action session, which is not a bug. After the session
  ends, `%step`/`%current` report `error: no active … session: … ended when <name> was redeclared at
  submission N`.
- **Performer-based end notice:** to lose the *object* without superseding the behavior, keep them in
  separate packages (`package P { part def Ship { action tally { … } } }`,
  `package Q { part s : P::Ship; }`, `%instantiate Q::s`, `%action P::Ship::tally Q::s`) and then
  resubmit `package Q` with an extra member → `note: action debugging session for "P::Ship::tally"
  ended (the object Q::s performing it was dropped)`. Redeclaring `P` instead names the declaration.
- **Value expressions are re-derived, not invalidated** (as of the `runtime.Adopt` change in
  PR #168): a carried slot whose feature has a value expression (`attribute m = double(3.0);`) is
  reset to unmaterialized, so the new context recomputes it from the declarations the expression
  reads *now*. Redeclaring `calc def double` with `x * 3.0` therefore keeps `ID: 1` and prints **no**
  drop notice, while `%features A` moves `6.00 → 9.00`; the same holds through chains
  (`outer` calling `inner`: `7.00 → 10.00`; `attribute h = g * 2.0` read by `m`: `11.00 → 15.00`).
  The assertion that catches the earlier stale-value bug is **`%eval` must agree with `%features`** after
  every such change — a `%features` that keeps the old number while `%eval` of the same expression
  returns the new one is the failure signature. Composite parts keep their carried values *and*
  nested instance IDs (`w = Instance(ID: 2)`). Drops are still expected for the instance's own
  redeclared definition and for a change to a declaration one of its features is typed by.
- **What is read again vs kept on carry-over** (`adopt.go` `derivedSlot`/`connectorSlot`/
  `collectedSlot`, as of 4947ca3 + 65b04ec). Four distinct classes, each with its own recipe:
  - *Connector slots are read again under the same identity.* `connection c1 connect a.x to b.y;`
    where `a.x = double(3.0)`: after redeclaring `double` with `x * 3.0`, `%features` must show
    `x = 9.00` **and** `c1 = Instance(ID: 4)` (unchanged) **and** `source = 9.00`. A `source` that
    stays `6.00` next to `x = 9.00` is the bug. This applies to named *and* anonymous connectors.
  - *A variation's selected variant is carried, not derived* — its default names a variant rather
    than a value. `part :>> engine = engine::electric;` must keep `electric (Instance ID: 2)` across
    an unrelated `part def Widget;`; an id that moves (2 → 3) means the variant slot was wrongly
    treated as derived. Changing the selection to `engine::petrol` must still drop with the notice.
  - *A collection of scalars is collected again* (its members are copies of what the subsetting
    features hold): `attribute pool : Real[*]; attribute one :> pool = double(3.0);` must go
    `pool = [6.00] / one = 6.00` → `pool = [9.00] / one = 9.00`. `pool` stuck at `[6.00]` while
    `one = 9.00` is the failure — the object then reads two values for one thing.
  - *A collection of objects is kept*, since those objects carry over under their identities:
    `part xs : B[3]` must print the identical `[Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]`
    after an unrelated submission, and the next `%instantiate` must not reuse 2/3/4.
- **A stale carried value only shows if the slot was materialized before the change.** These
  carry-over slot bugs need a `%features` (or other read) *between* `%instantiate` and the redeclaration:
  without it the slot was never materialized, so there is nothing stale to keep and even a broken
  build prints the right number. Contrast runs against a binary built from the parent commit
  (`git worktree add /tmp/wt-old <parent> && go build -o /tmp/old-sysml ./cmd/sysml`) are the cheapest
  way to prove a case actually discriminates — but include that intermediate read in both runs.
  Conversely, the *anonymous* connector-id case must also be checked with **no** read in between
  (two unrelated submissions back to back, then one `%features`), which is a separate code path.
- **Never put a literal TAB in piped REPL input** (`printf '…\t…' | ./bin/sysml`): readline enters
  completion mode and the process dies with `panic: bytes: negative Repeat count`. Use spaces in
  one-line rehearsal snippets.

Also: analysis still runs over the whole accumulated buffer, but since PR #65 the **report** is
scoped to the submission just made, so one bad snippet no longer keeps re-printing its error on
later submissions. Two consequences when testing:

- Reported line/column numbers are **relative to what you just typed** (`Result.Offset` /
  `baseLine()` in `internal/repl/render.go`), so a one-line submission reports `1:36:` no matter how
  much is already in the buffer. Only `%verbosity debug` numbers against the whole buffer.
- While an earlier error is unresolved, the next clean submission prints
  `note: deeper checks may not have run here: the error on buffer line N is unresolved (see it with
  -debug)` before its summary. That note is expected, not a failure — and it is printed **once**, not
  on every later submission, so its absence on the ones after is also expected.

Still true: accidentally typing a shell command like `clear` at the `>` prompt is parsed as SysML
and pollutes the buffer. `%clear` resets the session.

## The session-long symbol index and wildcard re-exports (PR #95)

`Session.symbolIndex()` (`internal/repl/session.go`) keeps **one** `symbols.Index` for the whole
session: the stdlib is loaded into it once (`model.LoadStdlibInto`) and only the session document is
re-indexed, when `doc.Version` changed. So stale/duplicated symbols are the failure mode to hunt,
and every assertion should be re-checked *late* in a long session, not only on the first submission.

The re-export cycle is the highest-value test, and it distinguishes working from broken in three
steps (a REPL built before this work fails at the very first one):

```
package Lib { part def Widget { attribute size = 3.0; } }
package P { public import Lib::*; }
%instantiate P::Widget      -> ✓ Created instance of Lib::Widget    (resolved FQN is the DECLARING one)
package P { }               -> replaces the earlier snippet (same declared name)
%instantiate P::Widget      -> error: unresolved reference: P::Widget  (re-export unwound)
%instantiate Lib::Widget    -> ✓ Created instance of Lib::Widget    (purge must not over-remove)
package P { public import Lib::*; }
%instantiate P::Widget      -> ✓ ... again, and never "is ambiguous"
```

Notes that save time:

- A re-export registers the symbol under the importing namespace but **never copies its subtree**:
  `P::Widget` resolves while `%eval P::Widget::size` is `unresolved reference: …`. Use the declaring
  FQN (`%eval Lib::Widget::size` → `= 3.00`). This is intended (confirmed by the maintainer).
- Resubmitting the same importing package several times must keep resolving and must never produce
  `is ambiguous` — duplicate re-export registrations would surface as ambiguity, so assert the
  negative explicitly.
- `%clear` drops the document from the index (`idx.RemoveDocument`) but **keeps the loaded library**,
  so `%clear` followed by more work exercises the removal path directly: expect
  `error: no declarations loaded (literals work, but feature references need declarations)` (or
  `error: runtime init: no document loaded` for `%instantiate`) for the old names, then full
  wildcard expansion again for freshly submitted packages.
- Because the REPL has a **single** document, a re-index removes and re-adds all of its re-exports
  wholesale. Index bugs that need one document's member to change while a *different* importing
  document survives are therefore not reachable from the prompt — verify those in
  `internal/core/symbols` tests instead of hunting them at the REPL. In particular a top-level
  (outside any `package`) `import Lib::*;` followed by a submission that drops the surfaced
  declaration still correctly reports `not found` at the prompt.
- Stdlib staleness check: quantity/unit evaluation resolves its unit through the session index, so it
  is the cheapest canary. `package Q1 { import SI::*; attribute speed = 1.5 [m/s]; }` then
  `%eval Q1::speed` → `= 1.5 [m/s]`, and re-running that exact command after each of ~15 further
  submissions must print the identical string every time. Without `import SI::*;` the unit is
  `error: evaluation failed: not a quantity expression: not a measurement unit: unresolved unit m`,
  which is a fixture mistake, not a regression.
- Pre-existing noise not caused by index work: ISQ-typed attributes
  (`attribute m : MassValue = 12.0;`) emit `cannot bind Rational value to a feature typed by
  ISQBase::MassValue` and a top-level `import Lib::*;` typed before `Lib` exists emits
  `unresolved reference: Lib`. Both reproduce on `main`; the lookups still succeed. A/B against a
  `main` binary in a `git worktree` before reporting any diagnostic as new.

## Output modes and execution tracing (PR #65)

Two independent axes — diagnostics verbosity and execution tracing. Don't conflate them.

```bash
./bin/sysml -quiet          # errors only (suppresses warnings)
./bin/sysml                 # default: errors + warnings + summary
./bin/sysml -debug          # whole-buffer diagnostics, absolute lines, pass names
./bin/sysml -trace          # execution tracing on from the start
./bin/sysml -debug -quiet   # rejected: stderr message, exit 2
```

Prompt equivalents: `%verbosity [quiet|normal|debug]` and `%trace [on|off]`; with no argument each
**reports** the current setting (`verbosity: normal`, `trace: off`). A bad argument prints guidance
(`error: unknown verbosity "bogus" (want quiet, normal or debug)`) and leaves the session usable.

To prove quiet actually suppresses something you need a **warning**, which is easy to trigger with a
mismatched comparison: `attribute flag = 1 == "a";` yields
`warning: comparing Natural with String is always false`. The strongest test is an A/B in one
session — submit it at `normal`, then `%verbosity quiet`, then submit an equivalent snippet under a
different package name and show the warning is gone while the summary line remains.

`%verbosity debug` output starts with
`[debug] submission at buffer line N; M diagnostic(s) over the whole buffer` and tags each
diagnostic with the pass that produced it (`[syntax/syntax]`, `[type/type.expr]`).

Tracing prefixes every recorded line with `[trace] `. Evaluation entries are **post-order and
indented**: sub-expressions appear before, and one level deeper than, the expression that consumed
them (`internal/core/runtime/trace.go`). `%features` on a model with derived attributes is the easiest
way to see a full tree — `derived_package.sysml` gives `eval operator * -> 3000.0` and a nested
`eval feature power -> 300.0` / `eval operator * -> 270.0` / `eval operator + -> 1770.0`.

The recorder is **drained per command** (`drainTrace` in `internal/repl/trace.go`), so each command
reports only its own steps. Always assert the negative too: run an unrelated command such as
`%instances` straight afterwards and confirm it prints **no** `[trace]` lines — a recorder that is
not cleared would replay the previous command's tree.

### Regression watch: traces during a debugging session

`ActionExecutor.trace()` reads the recorder off `e.ctx` rather than caching it on the executor
(`internal/core/runtime/action_executor.go`). That is what makes `%trace on` reach an execution
already under way, and what makes **expression** traces appear alongside step traces. The
historically broken sequence, worth re-running after any change in this area:

1. `%load internal/repl/testdata/action_debug.sysml`, `%action tally`
2. submit an unrelated declaration, e.g. `package Unrelated { part def Widget { attribute size = 1.0; } }`
3. `%trace on`, then `%step` repeatedly

Expect **both** `[trace] step N: token 1@accumulate` **and** `[trace] eval feature total -> 0` /
`eval literal 5 -> 5` / `eval operator + -> 5`. Bare step lines with no `eval` lines is the bug
signature. Then `%trace off` must silence output in that same session.

Control nodes are named by what they do, not by Go type: an unnamed fork/join/final reports
`token 1@fork` / `@join` / `@final` (`nodeIdentifier` in `internal/core/runtime/trace.go`). A `*ast.`
type name in trace output means a node kind is missing from that switch.

## Spot-checking the docs against the binary

`docs/guide/` and `README.md` contain REPL transcripts that are easy to let rot. Verify them
by **typing them by hand** at the prompt in a GUI terminal, not over a pipe: some failure modes (a
blank line inside a braced declaration ending the submission early) only exist interactively.
Discover the expected values over a pipe first, then do one clean recorded pass.

As of PR #107 the guide and README action- and state-debugging transcripts are real captured output
and match the binary verbatim, including the hint lines
(`Use %step to advance, %tokens to inspect, %continue to run to completion`,
`Use %tokens to inspect, %step or %continue to resume`,
`Use %events to see queue, %current for state, %advance <time> to step`) and the `✓ <kind> <name>`
echoes (`✓ calc distance`, not `✓ distance`). Treat a mismatch there as a real doc regression now.

Traps worth re-checking after any doc or REPL edit:

- **A blank line inside braces ends the submission.** A transcript that shows one is un-typable; the
  remainder arrives as a separate submission and produces diagnostics. Assert the whole declaration
  comes back as a single `✓ state X` with zero diagnostics.
- **`%save` over an existing file appends `(replaced the existing file)`.** A doc block whose earlier
  step created or loaded that same file must show the suffix. Byte counts are exact and
  content-dependent (`saved 181 bytes of sysml` / `saved 1872 bytes of ttl` for the
  `MyModel` file of guide chapter 7) — recompute them whenever the sample model changes.
- **Snippets using `Real` need `import ScalarValues::*;` in the session**, or the submission is
  rejected with `error: unresolved reference: Real` and no `✓` echo. A snippet relying on an import
  made in an earlier, separate doc section fails for a reader who starts a fresh REPL there.
- The binary's continuation prompt is `  ...>` (two leading spaces); some doc blocks write `...>`.
- Konsole ligatures render `<=` as `≤` on screen — cosmetic, not a REPL difference.
- Typing `clear` at the `sysml>` prompt pollutes the buffer (see the session-accumulation trap);
  restart the REPL rather than trying to recover mid-transcript.

## The gRPC service and the `opensysml` Python client

The REPL is not the only user-facing surface: `cmd/sysml-grpc` plus `clients/python/opensysml` is the path a
Python user takes, and the two can disagree. When a change touches `internal/grpc/convert.go` or
the runtime's slot evaluation, **test both and diff them** — that comparison is the highest-value
assertion available.

```bash
export PATH=/usr/local/go/bin:$PATH
make build && make build-grpc              # -> bin/sysml, bin/sysml-grpc
mkdir -p ~/.opensysml/bin && cp bin/sysml-grpc ~/.opensysml/bin/   # where the client looks
pip install -e clients/python/
```

Do **not** start the service by hand for model-semantics work. `Connection._ensure_service`
(`clients/python/opensysml/connection.py`) spawns a **private child** of the interpreter on `-port 0` and
learns the address from the child's stdout, which is the realistic user path. There is no pidfile,
no lockfile and no adoption of a service the client did not start: a service you started yourself is
reached only by naming it (`connect(host, port)`, `OPENSYSML_SERVICE=host:port`, or
`auto_start=False`).

### Driving the service on an ephemeral port (process-lifecycle work, PR #249)

When the thing under test is the **process** (flags, exit status, graceful shutdown, leaks) rather
than the model semantics, a hand-started service on `-port 0` is the right harness and
`opensysml.connect(host, port, auto_start=False)` attaches to it without the client taking any
ownership of it. Pass `-health-port` too: it defaults to 8081 and collides with an already-running
service. `-report-address` prints the dialable address as one line on stdout, which is easier to
read than the log, and `-exit-with-parent` makes the service exit at end of file on its stdin.
**Never pipe a command whose exit code you are asserting** — `… | tail` reports `tail`'s
status, which has produced a false pass on this very exit-code matrix; run it bare and echo `$?`.
And prove "never listened" with `ss -ltn`, since an exit code alone does not. Two more traps cost
real time:

- `-port 0` makes the *resolved* port observable only in the log line
  `msg="gRPC server listening" addr=[::]:<port>`. The `-health-port 0` line logs the literal
  `addr=:0`, so a naive `grep -o 'addr=[^ ]*' | head -1` grabs the health line and you dial port 0
  ("Connection refused"). Always filter on `gRPC server listening` first, and take the port with
  `${ADDR##*:}` — `cut -d: -f3` yields `]` for `[::]:41325`.
- Expected values at 0cf94e80 for `internal/grpc/testdata/conformance/instantiate_derived_slot.sysml`:
  `mass` → `materialized=True kind=real_value 1500.0`, `doubled` → `real_value 3000.0`; a missing
  model path raises `opensysml.errors.ModelFileNotFoundError` ("file not found: open …") and the
  server logs `code = NotFound` for `/sysml.SysMLService/ParseFile` while staying alive. An already
  occupied `-port` exits **1** with `msg="Failed to listen" port=<port>`; `kill -TERM` exits **0**
  after `Shutting down gracefully...` / `gRPC server stopped`. `pgrep -x sysml-grpc` (never `-f`)
  before and after is the leak check. Rest of the matrix: an unknown flag exits **2**;
  `-cache-size 0` exits **1** with `cache maxSize must be positive`, raised in `NewService` *before*
  `net.Listen`; a bogus `model_hash` is `NOT_FOUND` / `ModelNotFoundError`. Shutdown with a client
  channel still open exits 0 and makes the client's next call raise `UNAVAILABLE`
  (`opensysml.ConnectionError`) — bound that call with a timeout so a hang fails instead of hanging
  the run.

<a id="venv-trap"></a>
**Python interpreter trap on this box** (bites every opensysml section below): whatever `python3`
resolves to in a tool shell may be another project's venv, and a venv built from it gets a
mismatched `sys.path` — `pyvenv.cfg` naming one minor version while `bin/python` runs another, so
the editable install lands in a `site-packages` the interpreter never searches and `import opensysml`
(or `import grpc`) fails right after a *successful* `pip install -e clients/python/`. Always build the venv
from an explicit real interpreter (`/home/ubuntu/.pyenv/versions/3.12.8/bin/python3.12 -m venv ~/pv`,
or `/usr/bin/python3.10`) and verify `<venv>/bin/python -c 'import opensysml'` before blaming the
client. `$HOME/pv` is created by the blueprint, so prefer reusing it.

Do not assume `$HOME/pv` is the one that works: on a session seen in Aug 2026 `~/pv` and
`~/fprime-venv` both failed `import opensysml` while the *editable* install lived in
`/home/ubuntu/repos/fprime/fprime-venv/bin/python3` (3.12.8, `grpc` 1.83.0) — which happened to be
first on the tool shell's `PATH` as plain `python3`. Probe every candidate in one go
(`for p in /home/ubuntu/pv/bin/python /home/ubuntu/*venv*/bin/python python3; do $p -c 'import
opensysml,grpc'; done`) and use the absolute path of whichever answers. Critically, **a GUI Konsole
does not inherit the tool shell's `PATH`**: there `python3` is `/usr/bin/python3` (3.10, no
`opensysml`), so a demo script that worked in `exec` dies with `ModuleNotFoundError` on camera.
Always type the absolute interpreter path (or `export PATH=...`) in the recorded terminal.

When checking a documented exit status ("a failing check exits nonzero"), never pipe the binary into
`tail`/`grep` — `$?` then reports the pager's status and a failing command reads as `exit=0`.
Redirect to a file and echo `$?` first, then `tail` the file.

### Service lifecycle, the stale-service check and `require_capabilities` (PR #181)

Since PR #181 `Connection` interrogates whatever is *already* listening (`GetServerInfo`) and
compares it against the release asked for (`connect(version=...)` or `$OPENSYSML_GRPC_VERSION`) plus
`require_capabilities=[...]`; a mismatch raises `opensysml.StaleServiceError`. To test that surface
you **do** need a hand-started service, named explicitly:

```bash
./bin/sysml-grpc -port 50099 &                   # a port other than 50051 keeps the auto-start
                                                 # tests independent
OPENSYSML_GRPC_VERSION=v0.0.7 python -c '...connect(port=50099)...'   # -> StaleServiceError
```

- Ownership needs no record: the client holds the `Popen` of the child it started and signals only
  that. A named service is never stopped — assert `psutil.pid_exists(pid)` *and* that a subsequent
  connect still serves (`model.instantiate(...)`), not just that an exception was raised.
- A locally built binary reports the **commit** as its version (`version e695687`), not a `vX.Y.Z`
  tag, so any `OPENSYSML_GRPC_VERSION=v0.0.x` is a mismatch — handy, and it also means asking for a
  tag while a dev build runs will *always* raise.
- Capability names the service reports today: `convert`, `query`, `type_facts`, `verification`.
  A bogus `require_capabilities=['time_travel']` surfaces as `MissingCapabilityError` whoever
  started the service (only a release mismatch is a `StaleServiceError`), and it resolves in
  <0.2 s — time the run so a hang is visible as a number.
- With `auto_start=False` the release check is lazy: a service that was not listening when the
  client was built is checked at the first call of *any* kind, so assert it through `conn.load(...)`
  and not only through `conn.server_info()`.
- `opensysml` has **no module-level `load_from_content`**; use `conn.load_from_content(...)`.
- Lifecycle state lives only in the process: `opensysml.connection._private_services` maps a
  required release to the one child serving it. `~/.opensysml` holds the binary cache and nothing
  else, so there is no file to reset — `pkill -x sysml-grpc` is enough (`-x`, never `-f`, which
  matches your own shell — see the pkill trap below). Refcount behaviour worth asserting within one
  process (two `connect()`s): 1 → 2 → 1, the service still serving the remaining holder and stopped
  only when the last one closes. Across two processes there is nothing to share: each starts its own.
- **A leftover service may be answering 50051 from a path you never built.** A previous session can
  leave e.g. `/tmp/sysml-grpc` listening, in which case `python -m pytest clients/python/tests/test_runtime_integration.py`
  reports `N passed` in ~0.05 s against *unknown* code — the integration suite neither skips nor
  tells you whose binary served it, so a green run proves nothing about your commit. Before trusting
  any client result, run `pgrep -af sysml-grpc` — `-x` does find `/tmp/sysml-grpc` (it matches the
  executable's basename, truncated to 15 chars), but only `-af` prints the path that is serving,
  which is the thing you need to see. Kill the PID you found **by number**, start your own
  (`nohup ./bin/sysml-grpc >/tmp/grpc.log 2>&1 &`) and confirm the log's `Connect server listening`.
  When proving a runtime fix, re-point the same test at a service built from the parent revision
  (`git worktree add /tmp/wt-old HEAD~1 && (cd /tmp/wt-old && make build-grpc)`) and require it to
  **fail** — that is the only cheap check that the client assertion is load-bearing rather than
  vacuous.

Client API shapes that are easy to get wrong:

- `Model.find(name)` returns **one `Symbol` or `None`**, not a list — iterating it raises
  `TypeError: 'Symbol' object is not iterable`. Use `.id` (FQN), `.name`, `.kind`.
- `opensysml.instantiate(fqn, file_path=...)` and `opensysml.evaluate(expr, file_path=..., context_symbol_id=...)`
  each take *exactly one* of `file_path` / `model_hash`.
- `Instance.get_feature(name)` returns the raw protobuf `FeatureValue`. Read it as
  `sv.materialized` and `sv.value.WhichOneof('kind')` → `real_value` / `int_value` / `instance_id` /
  `null`. Printing the `Instance` alone hides exactly the detail under test.

What the feature-value kinds mean (`ValueToProto`, `convert.go`):

- A **derived** attribute (`attribute doubled = mass * 2.0;`) must arrive as
  `materialized=True kind=real_value`. `kind=null value='unsupported'` is the pre-fix signature.
- `null: 'unsupported'` is the generic fallback arm for a feature value returned **unmaterialized
  without an error** — a feature with no default and no composite type. Both a bare
  `attribute d : Real;` and a **constraint usage** land here, so the REPL's
  `massOK: <constraint: satisfied>` has no gRPC equivalent. Check whether that divergence is
  intended before filing it.
- `FeatureValue.error` is the real error arm (the value is left unset). Force it with cyclic derived
  attributes (`attribute a = b + 1.0; attribute b = a + 1.0;`) — expect
  `feature value Loop.a: feature value Loop.b: cyclic feature value dependency: Loop.a`, promptly,
  raised as `FeatureValueError` by
  the client, and prove the service is still alive afterwards with a follow-up
  `opensysml.evaluate('1 + 1', ...)`.
- A nested `part engine : Engine;` still marshals as bare `instance_id=N`, but
  `InstantiateResponse.instances` carries every instance reachable from the root, so Python
  expands the child too (`inst.engine.power`). An id is only resolvable against the response that
  carried it — runtime instances do not survive the request.

`execute_action` is the gRPC twin of the REPL's `%action` + `%continue`, and it is the cheapest way
to A/B the two surfaces on the same model. The call shapes are **not** the ones the docstrings
suggest: there is no `parse_file` on `Connection` — use `c.load_from_content(src)` (or `c.load(path)`),
which returns a `Model` whose hash is `model.hash` (not `.model_hash`). Then
`c.execute_action("Pkg::action", model.hash)` returns a plain `{name: value}` dict, and a runtime
failure raises `opensysml.errors.RuntimeError` with the executor's message
(`action execution failed: execute action: …`). If you must attach to a service you started
yourself, `opensysml.connect("localhost", 50551, auto_start=False)` avoids the
`Binary not found at ~/.opensysml/bin/sysml-grpc` error — but prefer the auto-start path per above.

Fixture trap when proving `Model.execute_action` / `Model.execute_state` "exist and work": an action
that only declares parameters (`action add : Add { in x = 2.0; out z = x + y; }`) validates clean but
raises `ExecutionError: initialize action: no initial node found in action add` — there is nothing to
execute. Do not read that as a broken RPC; borrow a body-bearing fixture instead, e.g.
`internal/core/runtime/testdata/conformance/action_body_local_calc_usage.sysml`
(`execute_action("test::run")` → `{'v': 2.0, 'i': 3, 'doubled': 4.0, 'acc': 12.0}`) and
`state_anonymous_action_body.sysml`
(`execute_state("Test::Bodies")` → `states_visited ['start','working','nstart','nested','done']`,
`final_context {'log': 321879}`). Those two doubles are the cheapest end-to-end proof that both
RPCs really run rather than merely being present on the object.

Suite baseline: `cd python && python -m pytest tests/ -q` with no service running should be
`148 passed, 24 skipped` (~40s; it was `75 passed, 18 skipped` before the Tier 1/Tier 2 client
work), and `158 passed, 14 skipped` with a service running. As of the 0.0.8 prep branch
(`b0f5f23`) that baseline is `368 passed, 26 skipped` in ~42 s from the repo root
(`python -m pytest clients/python/tests/ -q`), with one expected `UserWarning` from
`test_a_cache_survives_a_replacement_that_cannot_be_downloaded` — re-measure rather than trusting an
older count. The skips are the integration
tests gating on a live service. `pytest` is **not**
installed in `~/opensysml-venv` by default — `~/opensysml-venv/bin/pip install pytest` first, or
`python -m pytest` fails with `No module named pytest` (the `pytest` on `PATH` belongs to an
unrelated venv and cannot import `opensysml`).

A test that skips with no service and fails with one is never actually green — treat it as a
reportable defect, not a known gap.

Liveness check: after `test_lifecycle` runs, `pgrep -af sysml-grpc` still lists a `<defunct>`
zombie, so it lies. Use `ss -ltn` to decide whether a service is really listening.
`pkill -9 -f sysml-grpc` matches your own shell's command line — use `pkill -9 -x sysml-grpc`.
A full-suite run leaves an operator-started service alone; the private children it starts are gone
with the interpreter that started them.
To hold a service alive for a whole test run, keep a client process open, e.g.
`(setsid python -c "import opensysml,time; opensysml.connect(); time.sleep(300)" &)` — a plain
backgrounded `python -c` from a non-tty shell may exit before it prints, so verify the port.

Download paths (`clients/python/opensysml/binary.py`) are testable without a real release: move
`~/.opensysml/bin/sysml-grpc` aside, unset `OPENSYSML_GRPC_VERSION`, and call `ensure_binary()`,
`resolve_latest_version()`, `download_binary('latest')`. All three must raise `ConnectionError`
naming the path or URL. `OPENSYSML_GITHUB_REPO` overrides the repo. Beware: these hit the
unauthenticated GitHub API, so repeated runs flip from a truthful `HTTP Error 404: Not Found` to a
misleading `HTTP Error 403: rate limit exceeded` — rehearse sparingly and report the 404 wording,
not whichever one the recording happened to catch.

#### The stale-*cache* decision is testable offline (PR #178)

`stale_cache_reason(version, github_repo=None)` decides whether `~/.opensysml/bin/sysml-grpc` may
answer for a requested release, and it needs no network — drive it directly and write the sidecar
`~/.opensysml/bin/sysml-grpc.json` (`{"version":…, "sha256":…, "repo":…}`) by hand. The four shapes
worth asserting, with the wording each produces:

| sidecar | asked for | reason |
|---|---|---|
| absent | `v0.0.8` | `… was not downloaded by this client, so which release it is cannot be told` |
| `v0.0.7` + **true** sha256 of the file | `v0.0.8` | `… is v0.0.7, but v0.0.8 was asked for` |
| `v0.0.7` + wrong sha256 (hand-swapped binary) | `v0.0.8` | falls back to the "not downloaded by this client" wording |
| `v0.0.8` but `"repo":"someone/OpenSysML-fork"` | `v0.0.8` | `… was downloaded from someone/OpenSysML-fork, but v0.0.8 of Open-MBEE/OpenSysML was asked for` |

- The digest is re-verified (`cached_release`), so a *true* sha256 in the sidecar is what makes the
  "is v0.0.7" branch reachable — a placeholder digest silently tests the wrong branch.
- `stale_cache_reason(None)` is `None` **by design**: with `$OPENSYSML_GRPC_VERSION` unset any cached
  binary is taken on faith. So before any Python check, `cp bin/sysml-grpc ~/.opensysml/bin/` —
  otherwise a binary from an older release answers as if it were your build, and
  `~/.opensysml/bin/sysml-grpc -version` is the one-line way to confirm which build you are testing.
- `ensure_binary(version=…)` on a stale cache emits `UserWarning: Replacing the cached sysml-grpc: …`
  and then, when the download fails (403/404 in a sandbox), a second
  `UserWarning: Keeping the cached sysml-grpc … could not be downloaded` and returns the old path.
  That is the only observable half of the replacement without network; say so rather than claiming
  the replacement was proven. Always back up the cache + sidecar and restore them afterwards.
- **On a box that *does* reach GitHub the replacement completes**: `OPENSYSML_GRPC_VERSION=v0.0.8` will
  download that release over `~/.opensysml/bin/sysml-grpc` and write a matching sidecar, so your local
  `make build-grpc` binary is gone and `server_info().version` reports `v0.0.8` afterwards. Re-run
  `cp bin/sysml-grpc ~/.opensysml/bin/ && rm -f ~/.opensysml/bin/sysml-grpc.json` before the next test.
  The leftover sidecar is also the classic false negative when contrasting
  `OPENSYSML_GRPC_VERSION` (honoured, warns) against a pre-rename `PYSYSML_GRPC_VERSION` (must be
  ignored, no warning): if the sidecar already records the version you are asking for, *neither*
  warns and the contrast proves nothing. Delete the sidecar and pick a version the cache is not,
  then assert on the presence/absence of the `Replacing the cached sysml-grpc` `UserWarning`
  (capture it with `warnings.catch_warnings(record=True)`).
- `OPENSYSML_GRPC_BINARY` is read by no code (`get_binary_path()` is hard-coded to
  `~/.opensysml/bin`, as this skill notes) — treat any request to "verify OPENSYSML_GRPC_BINARY" as a
  claim to disprove, not a feature to exercise.

#### Proving a *pinned release digest* really unblocks a download (PR #316)

`PINNED_SHA256` in `clients/python/opensysml/binary.py` is what `download_binary(version)` verifies against;
without an entry for the tag, `expected_digest` raises `UnpinnedReleaseError` (a subclass of
`ChecksumMismatchError`) instead of trusting the `.sha256` served beside the asset. Verifying a new
pin end to end needs a **real download**, so isolate the cache first:

- `get_binary_path()` hard-codes `os.path.expanduser('~/.opensysml/bin')`. `$OPENSYSML_STATE_DIR`
  moves only the *lockfile* (`connection.py`), so a throwaway **`HOME=/tmp/…`** is the only way to
  force a download and to keep a locally built `~/.opensysml/bin/sysml-grpc` from answering. Run
  every step with the same throwaway `HOME` and `pkill -f sysml-grpc` first, otherwise a leftover
  service on :50051 is adopted and you never exercise the downloaded one.
- Assert the triangle, not just success: `sha256(downloaded file) == PINNED_SHA256[repo][tag][asset]
  == the freshly fetched .sha256 sidecar`, plus the `sysml-grpc.json` sidecar recording
  `{"version", "sha256", "repo"}`, plus `<path> -version` naming the tag. A pass on any one of those
  alone would look identical if the pin were wrong.
- The load-bearing check is a **contrast run on the parent commit** (opensysml is pure Python, so no
  rebuild): `git worktree add /tmp/wt <parent>` and run the same `download_binary('<tag>')` with
  `sys.path.insert(0, '/tmp/wt/python')`. It must raise `UnpinnedReleaseError` — otherwise the pin
  proves nothing. Remove the worktree afterwards (`git worktree remove --force`), and beware that
  while it exists the worktree's own `AGENTS.md`/`.agents/skills` also get picked up.
- Negative paths worth one run each, both of which must leave the cache byte-identical and no
  `sysml-grpc.tmp` behind: a published-but-unpinned tag (v0.0.9 is the standing example) ⇒
  `UnpinnedReleaseError`; and an in-memory tampered pin (`PINNED_SHA256[…][asset] = '0'*64`) ⇒
  `ChecksumMismatchError` naming both digests, refused before the ~24 MB binary is installed.
- `clients/python/scripts/pin_release_checksums.py --check` re-hashes every pinned asset and is the only
  coverage for the darwin/windows pins on a Linux box. It needs a token:
  `GITHUB_TOKEN=$(gh auth token) python clients/python/scripts/pin_release_checksums.py --check` (exit 0 and
  one digest line per asset). Confirm it is not vacuous by copying `clients/python/` aside, corrupting one
  digest and re-running — it must exit 1 with `… now hashes to X, but Y is pinned`.

#### Service start-up timing and its failure paths (PR #250)

`Connection._ensure_service` probes the service it spawns immediately and then backs off
(`START_PROBE_INITIAL_DELAY` 10 ms, doubling to `START_PROBE_MAX_DELAY` 250 ms) until
`START_TIMEOUT` (2.5 s). Timing claims here need a **contrast run against the parent revision**,
which needs no rebuild since opensysml is pure Python: `git worktree add /tmp/mainwt main`, copy the
generated `clients/python/opensysml/proto/*.py` in if they are missing, then run the same script twice, once
plain and once with `PYTHONPATH=/tmp/mainwt/python`, on the *same* `$HOME/pv` venv. Numbers seen at
c590253e on a free port with nothing listening: **21 ms on the branch vs 515 ms on main**; the
connection must then really work (`conn.load_from_content(...)` + `Model.eval('1 + 1') == 2`), since
a fast connect to a dead service would look the same.

Recipes for the failure paths, all with a port of their own so the :50051 tests stay independent:

- **A binary that exits at once** — point `$HOME` at a throwaway dir holding
  `.opensysml/bin/sysml-grpc` = `#!/bin/sh\nexit 3`. `get_binary_path()` is hard-coded to
  `~/.opensysml/bin`, so `$HOME` is the only injection point (there is no `OPENSYSML_GRPC_BINARY`);
  `OPENSYSML_STATE_DIR` moves the pid/lock files only. Expect
  `ConnectionError: Service exited with code 3 without serving localhost:<port>` in ~0.02 s.
- **A port that accepts TCP but never speaks gRPC** — `nc -l` is wrong: it exits after the first
  connection closes, the spawned service then binds the port and the test silently *passes*. Use a
  listener that stays up and never answers (bind + `listen(64)` + accept into a list), and pair it
  with a fake binary that does not exit (`exec sleep 120`) if you want the `START_TIMEOUT` branch
  rather than the "could not bind" one.
  **`START_TIMEOUT` bounds the sleeping, not the wall clock**: each probe may spend its RPC timeout
  (pre-spawn probe 5 s, later ones `START_PROBE_RPC_TIMEOUT` 2 s), so the observed failure is
  `ConnectionError: Service failed to start within 2.5s` after **~9 s** (3 probes) on the branch
  against ~17.5 s (6 probes) on main. Do not assert ~2.5 s wall time; assert the message, a
  single-digit probe count and the improvement over main. Count probes by wrapping
  `Connection._probe_service` with a timestamping function — that is also the cheapest proof of "no
  busy-spin".
- **A replacement that could not bind** (the `_wait_for_a_service_that_could_not_bind` /
  `START_CONFIRM_DELAY` path) — start a real service outside opensysml on the port, then push the
  client past the adopt step the way a release mismatch does — either
  `Connection._adopt_running_service = lambda self: False`, or without monkeypatching, ask for a
  release the running service cannot be: `opensysml.connect(port=P, version="v9.9.9")` (there is no
  `OPENSYSML_REQUIRE_VERSION` env knob). Expect `StaleServiceError` ("the service
  started here exited (1) while another one kept serving the address"), **no** ownership record
  (`~/.opensysml/sysml-grpc-<port>.pid` absent, `_OWNED_SERVICES` empty), the foreign pid still alive
  and a follow-up `Connection(auto_start=False)` still evaluating. Asserting only "an exception was
  raised" would miss the real regression risk, which is silently adopting a service this client did
  not start and killing it on close.
- **Ownership under a race** — two `python /tmp/x.py <port>` processes started together: exactly one
  prints `_holds_refcount=True` with `{'refs': 1}`, the other reports
  `origin=service already listening …`; exactly one `sysml-grpc` runs, and it stops when the owner
  exits. Pair it with a test that closes an *adopted* connection and asserts the hand-started
  service is still serving.

### Verification RPCs, typed errors and strict loading (`opensysml` Tier 3, PR #149)

The verification questions the REPL answers with `%constraint`, `%requirement`, `%satisfy` and
`%calc` are also RPCs (`internal/grpc/verify.go`), wrapped as `Model.verify_constraint /
verify_requirement / verify_satisfaction / satisfied / calc`. Testing them from Python:

- Use a **clean venv** — the box's default `python3` may carry an incompatible `protobuf`, which
  fails at `import opensysml`. A venv with `pip install -e clients/python/` (e.g. `~/pv`) is the reliable
  interpreter; rebuild with `make build-grpc` and **re-copy** `bin/sysml-grpc` to
  `~/.opensysml/bin/` after every rebuild or the client silently auto-starts the old binary.
- Argument order bites: `Connection.eval(expression, model_hash)`,
  `Connection.instantiate(symbol_id, model_hash)`, `Connection.verify_constraint(symbol_id,
  model_hash, subject_symbol_id=…)`, `Connection.calc(symbol_id, model_hash, arguments=[…])` —
  hash **second**. On `Model` the hash is implicit and the kwarg is `subject=`. There is no
  `Connection.evaluate`. Getting the order wrong yields a confusing
  `ModelNotFoundError: model not found: Demo::sedan`.
- `Instance.features` is a **property** (a dict), not a method; unmaterialized feature values
  (constraint and requirement usages) appear as `FeatureValueError` values inside it, as expected.
- The three-way semantics worth asserting separately: a condition evaluating **false** is a
  verdict (`holds False`, `.condition` set, `.error == ''`, `raise_for_error()` silent); a
  **failure to evaluate** is `.error` non-empty with `.evaluated False` and
  `raise_for_error()` raising `ExecutionError`; an **unanswerable request** (unknown symbol)
  raises `ExecutionError` from the call. Force the middle case with a requirement whose
  attribute is never bound (`requirement loose : SpeedRequirement;` with `maxSpeed` unbound) —
  the error reads `no value for feature maxSpeed`.
- A subject of an unrelated type is *not* rejected: `verify_constraint(c, subject=<an attribute
  or a calc>)` instantiates it and answers from declared defaults. If you need a raising subject,
  use a name that does not exist (`unresolved reference: …`).
- `verify_satisfaction(fqn)` narrowed to an element that states no assertion returns an **empty
  list**, so `satisfied()` is vacuously `True`; the `"<x> states no satisfaction assertion"`
  error branch in `verify.go` only fires for a scope-less symbol and is hard to reach.
- Each verdict from `verify_satisfaction()` carries the **whole response's** instance graph, so
  `verdict.instances` includes other assertions' subjects — filter on `verdict.instance_id`.
- Model-cache eviction is testable for real (the service caches 100 models, `-cache-size`):
  load a model, then `load_from_content` 120 throwaway packages, then call any RPC with the old
  hash → `ModelNotFoundError`. Takes ~1 minute; run it in the background.
- Capability negotiation against an "old" service can be simulated without an old binary:
  `conn._server_info = ServerInfo(version='old', capabilities=frozenset({'convert'}),
  answered=True, origin='simulated')` → the verify calls raise `MissingCapabilityError` before
  any RPC. Label it as simulated in the report.
- gRPC status translation lives in `opensysml/errors.py`: assert both the opensysml class and the
  builtin it also is (`ModelFileNotFoundError`/`FileNotFoundError`,
  `InvalidRequestError`/`ValueError`, `ConnectionError`/`ConnectionError`,
  `ExecutionError`/`RuntimeError`) and that `__cause__` is the original `grpc.RpcError`. A dead
  service is reproducible with `opensysml.connect(port=50123, auto_start=False)`.

### The shared library index, `OPENSYSML_GRPC_INDEX_POOL` (PR #252; shared base since slice A of L3)

`internal/grpc/libindex.go` builds **one** frozen standard library index and gives each model an
overlay over it (any positive `OPENSYSML_GRPC_INDEX_POOL` prewarms that build; `0` restores the
per-cache-miss build). It was a pool of N per-model indexes until slice A, so the drain-and-refill
behaviour below no longer applies: there is nothing to drain, and a tight sweep of distinct models
stays fast after the first build. How to observe it end to end, and what generalizes to any
service-side perf change:

- **Measure with DISTINCT model texts.** The service caches models by content, so a repeated model
  is a cache hit and shows nothing. Append a unique trailing comment (`// distinct model %d`) per
  iteration and time `conn.load_from_content(src)` client-side; a library-backed model (imports
  `ScalarValues`/`ISQ`, a derived attribute) is required, otherwise no library index is needed.
- Numbers observed at 607b0eb8 on a ~85-line model, 12 distinct models: **pool default (4) median
  4.4 ms**, `OPENSYSML_GRPC_INDEX_POOL=0` **median 112.5 ms**. The 1–2 spikes of ~140–155 ms that a
  pooled run showed were the drained pool rebuilding; with a shared base only the very first
  request can pay a build, so a spike after the first is now a regression rather than by design.
  Report the median plus any spike rather than the mean.
- **The env var reaches the service through the opensysml auto-start path** (the client spawns the
  child, so it inherits the env), so `OPENSYSML_GRPC_INDEX_POOL=0 python sweep.py` is enough — but
  `pkill -x sysml-grpc; rm -f ~/.opensysml/sysml-grpc-50051.pid ~/.opensysml/sysml-grpc-50051.lock`
  first, or you keep measuring the previously spawned service's configuration.
- **Equivalence is the assertion that catches a wrong index.** Have one script print a sorted JSON
  blob (diagnostics, `find()` id/kind, `eval`, instantiate slot kind+value, `execute_action`,
  `execute_state`) and `diff` the pool=4 and pool=0 runs: only the line naming the configuration may
  differ. A model writing into the shared base, or seeing another model's document, would show up
  here, not in the timings.
- Bad values are rejected in `NewService`, so `sysml-grpc` **exits 1 before listening**:
  `-1` → `library index pool size must not be negative, got -1 (OPENSYSML_GRPC_INDEX_POOL)`,
  `many`/`1.5`/`"4 4"` → `library index pool size must be an integer, got "many" (…)`. Assert the exit
  code *and* `ss -ltn | grep :<port>` empty — a service that started anyway is the real failure. An
  empty or all-whitespace value is treated as unset and the service starts normally.
- Client-side shapes that break these sweeps: proto diagnostics carry severity as a **string**
  (`d.severity == "error"`; there is no `sysml_pb2.SEVERITY_ERROR`), `Instance.feature_values` is a
  **map** so iterate `inst.instance.feature_values.items()` (and prefer the public `inst.features`), and
  `file_path="/virtual/…"` must never be passed together with `content=` — the service tries to open
  the path and answers `NOT_FOUND`. Keep a runner behind `if __name__ == "__main__":` so importing it
  to reuse its fixture generator does not execute the whole suite.
- Fixture syntax that silently ruins a pool sweep, since 3 diagnostics per model look like the pool
  corrupting resolution: write `transition first s1 accept after 1 then s2;` (not
  `transition s1 then s2 accept after 1;`), and give an `out` an expression
  (`action def Act { in x : Real; out y : Real = x * 2.0; }`).
- Prewarming must not block startup: the port accepts in ~30 ms with pool=4, and SIGTERM after
  prewarming exits **0** in ~13 ms (`Close()` waits for in-flight builds, so a hang here is the
  regression to watch). Time both with `date +%s.%N` around the launch/`kill -TERM`.
- Two cheap adversarial shapes worth keeping: `OPENSYSML_GRPC_INDEX_POOL=0` with 8 threads loading
  distinct models at once (all 8 must still answer
  `Perf::Engine`, `1+1 == 2` and the full `execute_action` dict), and `-cache-size 5` with 8 distinct
  models loaded (the 3 oldest hashes raise `ModelNotFoundError`, the 5 newest still evaluate) — an
  index handed to a model that is later evicted must not disturb the models still cached.
- Interpreter trap: see [the venv trap](#venv-trap) above before blaming `import grpc`/`import
  opensysml` on the change under test.
- **`Model.find()` does not answer library names.** `find("ScalarValues::Real")` and
  `find("ISQBase::MassValue")` are `None` even when the model resolves those types fine (the RPC
  searches the model's own document symbols). So a `find`-based "the library still resolves"
  assertion is **vacuous** — it is `None` on a working build and on a broken one alike. Prove library
  resolution with a value instead: a derived attribute over a library type
  (`attribute doubled : Real = power * 2.0` → `eval("Pkg::Vehicle::engine.doubled") == 300.0`) plus
  an empty `diagnostics` list. Keep the `find` entries in the sweep blob anyway — they are still a
  good *isolation* probe, since `find("OtherPkg::Engine")` must be `None`.
- `Model.execute_state(...)` returns a **plain dict**: subscript `r["states_visited"]` /
  `r["final_context"]`; `r.states_visited` raises `AttributeError: 'dict' object has no attribute`.
  `execute_action` is a dict too.
- `%search` matches the **qualified name as written in the library**, so `%search ISQ::MassValue`
  answers `no symbol matches` (the symbol is `ISQBase::MassValue`) while `%search MassValue` lists
  it. Use the bare name for a lookup assertion, or you record a false negative on camera.
- The strongest "nothing a user sees changed" evidence for this refactor is a **byte diff against
  the parent commit on both surfaces**: `/tmp/old-sysml` vs `./bin/sysml` over scripted REPL
  transcripts (`%search`/`%load`/`%eval`/`%instantiate`/`%features`/`%action`+`%continue`/`%state`+
  `%step`) and `-validate` over `examples/*.sysml`, plus the same sweep JSON against a hand-started
  parent-commit `sysml-grpc` (`/tmp/old-sysml-grpc -port 50123 -health-port 50124`, client
  `auto_start=False`). At 5a50e806 all of those are identical. Note a multi-model REPL session
  legitimately prints `note: deeper checks may not have run here: the error on buffer line NN is
  unresolved …` once the conformance fixtures are in the buffer — it reproduces on the parent binary,
  so do not report it as a regression.
- Post-first-load timings observed at 5a50e806 over 12 distinct library-backed models:
  `OPENSYSML_GRPC_INDEX_POOL=4` first 55 ms then median 4.2 ms (max 5.1 ms); `=0` first 77 ms then
  median 5.0 ms (max 5.6 ms) — i.e. with a shared base even `=0` is fast after the first request,
  and any post-first load above ~60 ms is a regression. Port accepts in ~10–12 ms and SIGTERM
  (including one sent immediately after the port opens, while the prewarm build is still in flight)
  exits 0 in a few ms.

#### The shared on-disk library index cache (`internal/core/libs/cache.go`)

The REPL, the LSP and the gRPC service all read `~/.cache/sysml-ls/libs` (or
`$XDG_CACHE_HOME/sysml-ls/libs`), 95 content-addressed `*.idx` files — so a change to that file needs
a cross-surface check, not just a gRPC one. Move the directory away (cold) and run
`./bin/sysml <model> -instantiate <fqn> -e <fqn>::attr` plus an LSP `initialize`/`shutdown`/`exit`
over `--stdio`, then repeat warm: expect roughly **860 ms cold vs 160 ms warm** for the CLI run, the
95 files rebuilt, no leaked `sysml-lsp`, and `ls -a` showing **zero** `.idx.tmp*` droppings (writes go
through a unique temp file plus rename). Overwriting one `.idx` with garbage — and, separately,
truncating one to 0 bytes — must degrade to a silent rebuild back to its original size (same output,
exit 0), never an error mentioning the cache.

### The `Query` RPC / `model.query(...)` (SysML v2 API & Services, PR #155)

`internal/grpc/query.go` + `clients/python/opensysml/query.py` implement the standard's Query resource
(`scope`/`select`/`where`, `PrimitiveConstraint` with `=`/`>`/`<` and `inverse`,
`CompositeConstraint` with `and`/`or`). Testing notes that generalize:

- **Refresh `~/.opensysml/bin/sysml-grpc` or you test a stale service.** A missing capability shows
  up as a `MissingCapabilityError` or an "unimplemented" RPC rather than a build failure, so start
  every run with `make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/sysml-grpc` and print
  `md5sum` of both plus `git log --oneline -1` on camera. `GetServerInfo().capabilities` should
  list `type_facts, convert, verification, query`.
- **The Python layer validates before the wire, so client-path tests cannot reach service-side
  faults.** Undefined payload keys, an empty composite and a missing operator are rejected locally
  as `QueryError`. To exercise the service's own validation (unknown property, unknown scope, `>`
  on a non-ordered property, non-numeric operand, `Constraint` with neither variant, unset query)
  call the raw stub: `model.connection._stub.Query(sysml_pb2.QueryRequest(...))`. Assert **both**
  paths — the typed client error and the service's `INVALID_ARGUMENT` message naming the problem.
- Typed client errors are the contract: `INVALID_ARGUMENT` → `InvalidRequestError` (also a
  `ValueError`), a bogus/evicted `model_hash` → `ModelNotFoundError` (**NOT_FOUND** is intended
  here, consistent with the other RPCs — don't file it as a wrong status). If a raw
  `grpc._InactiveRpcError` reaches the caller, the call is missing `translate_rpc_errors()`.
- **A query matching nothing must be an empty list, never an error.** Always assert that
  explicitly, otherwise a validation bug that silently answers `[]` looks like a pass.
- **Anonymous members are the classic scope-walk bug.** A `doc`/comment note, an anonymous usage or
  an anonymous `connect` has a degenerate FQN (`Pkg::`), so an unguarded walk answers non-unique
  `@id`s. Keep a fixture with all four shapes and assert, per model: no empty/degenerate `@id`, no
  duplicates, anonymous elements absent, **and** that named descendants of a named parent are still
  answered (the fix drops descendants of anonymous elements too, so it can over-prune). Element
  counts are the canary: `examples/rdf-interop-demo.sysml` is 22 elements with the fix (25 before),
  4 of them PartUsages, so an `inverse` partition is 18 + 4 = 22.
- **Stdlib elements restored from the library cache are lossy** and this is documented, not a bug:
  ~47 elements under `Base`/`Occurrences`/`Links`/`Clocks` come back with **no** `@type` (their
  symbol kind is `SymbolUnknown`), so they never match `@type =` but are kept by its `inverse`; ~12
  `*Definition`s (e.g. `ScalarValues::Boolean`) come back with **no** `isAbstract`. Any claim in
  docs/reference/api.md that the `@type` mapping is total is a doc bug unless it is scoped to "every kind a
  parsed declaration can have".
- Property-absence is the documented shape throughout: `owner` absent for a top-level package,
  `type` absent for an untyped attribute, `declaredName` absent for an *effective* name. A one-line
  fixture (`part :>> engine;` inside `part v`) gives the effective-name case, with a sibling
  `part motor2;` as the control that still carries `declaredName`.
- Good fixtures: `examples/rdf-interop-demo.sysml` (nesting + PartUsages),
  `examples/sysml-v2-training/04. Subsetting/Subsetting Example.sysml` (`[*]` vs `[4]` — proves `*`
  behaves as infinity for `>` and is excluded by `< 5`, plus `abstract part def`),
  `examples/state-machine-demo.sysml` (imports `ScalarValues::*`; an empty scope must still answer
  only its own 5 elements, not the stdlib).
- Drive it as one scripted assertion runner (`PASS/FAIL <id> <claim> | <evidence>` lines and a
  final `n/n assertions passed`) with an `input()` pause between sections when `QT_PAUSE=1`, then
  step the pauses on camera. Never type `clear` at such a pause — it is consumed as the Enter for
  the *next* section and you lose that section from the screen; use Konsole's `shift+PageUp`
  scrollback to recover it.

## Enumeration literals: runtime value and gRPC wire form (PRs #197, #208)

A literal's identity is its **declaration**, so nothing in this area should ever be tested by
comparing rendered text. The two flavors behave differently and both need a case: a plain literal
(`enum def Color { red; green; blue; }`) is its own value and crosses the wire as an
`EnumLiteral`, while a literal of an enumeration that specializes a scalar
(`enum def GradePoints :> ScalarValues::Real { A = 4.0; }`) evaluates to the **declared scalar**,
so `eval("D::GradePoints::A")` must be the float `4.0`, not an `EnumLiteral`. A literal can also own
attributes (`Level { low { :>> n = 1; } high { :>> n = 9; } }`), read as `eval("D::Level::high.n")`
→ `9`.

- **Stale-binary trap (costs an hour if missed).** `opensysml` auto-start runs
  `~/.opensysml/bin/sysml-grpc` and otherwise *downloads a release*, which lacks new capabilities.
  Always `make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/` first and prove it on camera with
  `./bin/sysml-grpc --version` (prints `commit: <sha>`) plus `ls -l` on both paths. Capability check:
  `conn.server_info()` must list `enum_values`
  (`['convert','enum_values','query','type_facts','verification']`).
- **Wire field numbers are shared real estate.** `api/proto/sysml.proto` has
  `Quantity quantity = 8;` and `EnumLiteral enum_literal = 9;` — the enum arm moved from 8 to 9 when
  `main`'s quantity support landed. After any merge that touches the `Value` oneof, re-run both
  arms *on the same part* (one `part def` with `attribute c : Color = Color::red;` **and**
  `attribute mass = 1500.0 [SI::kg];`): a field-number mismatch shows up as `None`/`unsupported`,
  not as an exception. `clients/python/tests/test_wire_compat.py` pins the numbers at unit level.
- **Incoming values go through one converter.** `ProtoToValueIn(pv, idx, sem)` in
  `internal/grpc/convert.go` dispatches the quantity and literal arms and recurses into sequences;
  it is called from `internal/grpc/service.go` (action inputs) and `internal/grpc/verify.go` (calc
  arguments). The error wording is layer-specific and worth asserting verbatim:
  calc → `calc argument could not be read: …`, action → `input "c" could not be read: …`.
- **Identity is `literal_id` alone** (`clients/python/opensysml/enumeration.py` marks `enumeration_id` and
  `name` `compare=False`). Comparing two *wire-populated* literals passes even when this is broken,
  so always include the bare-vs-populated cases: with `bare = EnumLiteral("D::Color::red")` and the
  feature value, assert `bare == car.c`, `hash(bare) == hash(car.c)`,
  `len({bare, car.c}) == 1`, `{car.c: "R"}[bare]` and `{bare: "R2"}[car.c]` both resolve, while
  `bare != EnumLiteral("D::Color::green")` and `len({bare, green}) == 2`. The broken shape reads
  `False False 2`. Also send a bare literal *to* the server (`calc IsRed([EnumLiteral(
  "D::Color::red")])` → `True`) — a description-free literal must still resolve against the index.
- **Type, not text.** Assert `isinstance(car.c, opensysml.EnumLiteral)` **and**
  `car.c != "Color::red"`; a string-shaped decode passes every `str()`-based check.
- **Input round-trips need a real executable action.** An `action def` whose body is only `in`/`out`
  parameters is not executable (`initialize action: no initial node found`). Use
  `action Classify { attribute c : Color = Color::green; attribute isRed : Boolean = false;
  first start; action inner { assign isRed := c == Color::red; } done end; then start inner;
  then inner end; }` and pass `inputs={"c": red}` — the in-model default being *green* is what
  proves the input actually bound.
- **Adversarial wire ids** (run on both the calc and action path): `D::Color::mauve` →
  `… D::Color::mauve is not an enumeration literal of this model`; empty `literal_id` →
  `enumeration literal: literal_id names no declaration`; a valid but non-literal FQN `D::Car` →
  `… D::Car is not an enumeration literal of this model`. Through `eval` the typed runtime error
  appears instead: `not a literal of the enumeration: mauve is not a literal of Color (literals:
  red, green, blue)`. Finish with `eval("1 + 1")` → `2` to prove the service survived. Carried over
  from #197 and still true: a *bare* REPL `%eval D::Color::purple` never reaches `ErrNotALiteral`,
  it answers `unresolved reference … did you mean <stdlib name>?`.
- **Quantities: received but not sendable from the public client.** `connection._python_to_value`
  has no `Quantity` arm (bool/int/float/str/None/Instance/EnumLiteral/list only), so a quantity
  *input* can only be exercised by building a `sysml_pb2.Value(quantity=…)` and calling the stub on
  the same channel. That asymmetry is `main`'s, not the enum PR's — report it as "found, not fixed"
  rather than a defect. **Trap:** do not fabricate the `unit_term`; naming `SI::kg` as a base factor
  yields the false-looking `incommensurable units: cannot express SI::kg (1000·SI::gram) in SI::kg
  (SI::kilogram)`. Echo back the reduction the service itself reported
  (`car.mass.unit` → `scale_num=1000.0`, `factors=(UnitFactor('SI::gram', 1.0),)`).
- **Codegen and REPL cross-checks.** `python -m opensysml.generate` must emit
  `def c(self) -> _t.EnumLiteral: return _t.feature_value(self, "c", _t.as_enum_literal)` and, for
  the quantity feature, `_t.Quantity` / `_t.as_quantity`; then read both off a live instance
  (`Car.from_instance(conn.instantiate("D::Car")).c`) so a wrong decoder raises `TypeMismatchError`
  instead of passing silently. In the REPL, `%features` prints `name = value`, i.e.
  `c = Color::red`, `palette = [Color::red, Color::green, Color::blue]`, `mass = 1500.00 [SI::kg]` —
  requests often phrase it as `c: Color::red`, which is the same thing.
- **Set membership** is only observable through `->includes`/`union`; no REPL syntax builds a `Set`,
  and a sequence literal keeps duplicates by design.

## Typed codegen (`python -m opensysml.generate`, Tier 2)

`python -m opensysml.generate <model.sysml> [-o out.py] [--host --port]` loads the model through
the **live service** (so it auto-starts `sysml-grpc`) and prints/writes one class per SysML
definition deriving from `opensysml.typed.TypedObject`. Useful facts when testing it:

- The reference fixture is `internal/repl/testdata/vehicle_package.sysml` and the committed
  golden is `clients/python/tests/golden/vehicle_types.py`; `cmp` them for a byte-for-byte assertion and
  generate twice + `cmp` for determinism. Emission is FQN-ordered with base classes first.
- Only instance feature usages become properties (`attribute/part/item/occurrence/port/enum`);
  `calc`, `constraint` and `requirement` members are deliberately absent — a generated class
  that grows a `withinMassLimit` property is a bug, not progress.
- Annotations are the whole point: `attribute power = 300.0;` must render `-> float` and
  `part engine : Engine;` must render `-> Engine`. If everything renders `object`, the typefacts
  path (`internal/grpc/typefacts.go` → `SymbolInfo.type_info`) is broken.
- Static-check evidence needs `MYPYPATH=<repo>/python mypy --follow-imports=silent script.py`
  and the venv mypy (`~/opensysml-venv/bin/mypy`). Without `MYPYPATH`, mypy silently treats
  `TypedObject` as `Any` and *misses* attribute-typo errors, so a "clean" mypy run proves nothing
  until you have seen it also flag a deliberate misuse (`v.mas`, `v.mass + "x"`).
- Adversarial cases that distinguish working from broken, all reachable through a generated
  property: cyclic derived attributes (`a = b + 1.0; b = a + 1.0;`) must raise opensysml
  `FeatureValueError` rather than returning `None`; and running *stale* generated code against a
  model whose attribute type changed (e.g. `mass = "heavy"`) must raise
  `TypeMismatchError: feature value 'mass': expected float, got 'heavy'`.
- **Pass the model path absolutely.** The path travels to the service, which opens it relative to
  *its own* CWD, so `-o` works but `../internal/...` fails with a gRPC `NOT_FOUND: file not found`
  traceback that looks like a client bug and is not one.
- **Capability gate (`type_facts`).** Generation calls `GetServerInfo` first and exits 1 without
  writing anything unless the service reports `type_facts`. To simulate a stale service, keep a
  pre-`GetServerInfo` binary around — `~/.opensysml/bin/sysml-grpc` from an older release is usually
  one; check it with a direct `stub.GetServerInfo(...)` (expect `StatusCode.UNIMPLEMENTED`), then
  run it on a spare port (`-port 50077 -health-port 8099`) and generate with `--port 50077`. Assert
  the negatives: target file still absent / identical sha256, and stdout mode piped to `wc -c`
  yields `0` (no silent all-`object` module). The message should name the capability, the service
  origin and `make build-grpc`. Note the repo blueprint copies the freshly built `bin/sysml-grpc`
  into `~/.opensysml/bin/`, so in a clean session the cached binary is *current* and no longer serves
  as the stale fixture — either keep a copy of an older release binary aside first, or stand up a
  stub gRPC server that answers `GetServerInfo` with `UNIMPLEMENTED`.
- **`--check`** requires `-o` (exit 2 otherwise), writes nothing, and exits 1 when the target is
  missing or would change, naming the exact regenerate command. The strong assertion is
  `sha256sum` before *and* after — exit code alone does not prove nothing was written.
- Generated modules carry `SYSML_GENERATOR_VERSION` and `SYSML_MODEL_HASH` (sha256 of the
  newline-normalized model source), so any model edit changes the stamp and therefore the bytes.
- **`TypedObject.from_instance` guard.** It rejects only an instance whose reported type belongs to
  a *different* generated class; a generated subclass and an unrecognized type both pass. To cover
  all three branches you need a fixture with a specialization and a usage, e.g.
  `part def SportsCar :> Vehicle { ... }` plus `part myCar : SportsCar;` — an instantiated *usage*
  reports its own FQN (`Demo::myCar`), which no class carries, so it must be accepted. `unchecked()`
  bypasses the check.
- **Restarting the service on 50051:** kill it by PID (`ls -l /proc/<pid>/exe` to confirm which
  binary), not `pkill -f 'bin/sysml-grpc'` — that pattern also matches the tool shell running the
  command and kills your own session. Rebuilding leaves the old process serving a `(deleted)`
  binary, which silently tests the previous revision.
- `clients/python/tests/test_lifecycle.py::TestLifecycleRobustness::test_service_shuts_down_when_last_process_exits`
  fails (`FileNotFoundError: ~/.opensysml/sysml-grpc.pid`) whenever an externally started service is
  already listening on 50051 — a known service-ownership gap, reproducible on `main`. Confirm on a
  `main` worktree before reporting it as a regression.
- Do not try to force a type mismatch inside the model — the checker rejects
  `attribute count : Integer = 4.0 / 2.0;` with `cannot bind Rational value to a feature typed
  by Integer`, and `Integer`/`String` need `import ScalarValues::*;` or generation exits 1 with
  `error: unresolved reference: Integer`.

## Built-in library functions (sqrt/sin/exp/ln/log/atan2 …)

The runtime supplies bodies for the function-library declarations in
`internal/core/runtime/library_functions.go`; the non-normative extensions
(`exp`, `ln`, `log`, `atan2`) live in
`internal/core/libs/stdlib/OpenSysML Libraries/OpenSysMLMathFunctions.kerml`.
Testing notes that generalize to any future built-in:

- The fastest end-to-end surface is the batch flag, which loads a model *and* evaluates
  expressions against it: `./bin/sysml -e "exp(1.0)" -e "log(8.0, 2.0)" model.sysml`. Several
  `-e` flags are allowed and each prints `✓ <expr>` then `  = <value>`; a failure prints
  `error: evaluation failed: ...` and the process still exits 0, so assert on the text, never on
  the exit code.
- **`-e` is evaluated in the root scope, not inside the model's package.** So a model that declares
  its own `calc def exp` is *not* exercised by `-e "exp(2.0)"` (that hits the built-in). To prove
  shadowing, either use the FQN (`-e "OwnExp::exp(2.0)"`) or read it out of an attribute default
  with `%instantiate`/`%features`. Getting this wrong looks exactly like a shadowing bug.
- Results print to **two decimals**, which hides precision differences. To assert exactness, compare
  in the model instead: `-e "log(1000.0, 10.0) == 3.0"` → `= true` (the naive `ln(x)/ln(base)`
  gives 2.9999999999999996 and would print `= 3.00` while being `false`).
- Domain/overflow handling is deliberate: `realResult` converts NaN/Inf into
  `arithmetic domain error` / `arithmetic overflow`, so a bad argument must yield an `error:` line,
  never a number. Worth checking per function (`ln(0.0)`, `log(4.0, 1.0)`, `atan2(0.0, 0.0)`,
  `exp(1000.0)`), plus wrong arity (`calc argument count mismatch`) and a non-numeric argument
  (`type mismatch: ... requires a numeric value`).
- A **bare** call with no `import` still evaluates (the unqualified-name table is always in force)
  but the checker prints `error: unresolved reference: <name>` for the same call inside a
  declaration. That divergence is a known rough edge of the built-in dispatch, not a new bug —
  report it as expected, and use `import OpenSysMLMathFunctions::*;` in fixtures to avoid it.
- Fixture gotchas when writing action fixtures by hand: `done` is a reserved keyword (use another
  name), a body has one start so only one `first` end (chain the rest as `first a then b; then b c;`
  — two `first` ends yield `action has multiple initial nodes`), and `and`/`or` are
  `unsupported operator` in constraint bodies — keep constraint expressions to a single comparison.

## `.kerml` vs `.sysml` file kind: which surfaces actually keep it

Anything in the parser gated on `p.src.Kind() == source.KindKerML` (e.g. `parser.unreserved` in
`internal/core/parser/notation.go`, which reclassifies SysML-only literals such as `at`, `while`,
`merge`, `decide` as names in a `.kerml` file) is **invisible on the REPL/`-validate` path**: the
session buffers every submission into one document named by the constant `docName = "<repl>"`
(`internal/repl/session.go:25`, opened at `session.go:728 ws.Open(docName, …)`), so
`source.KindOf("<repl>")` is `KindUnknown` and the gate never fires. `%load`ing a `.kerml` file
behaves the same way.

Surfaces that *do* pass the real path, and are therefore the ones to test file-kind behavior on:

- `sysml <file>.kerml -convert ttl` → `internal/core/export/convert.go:278 source.New(name, data)`.
- the LSP / `model.newDocument` (`internal/core/model/document.go:26`) with a real URI.
- the stdlib loader `internal/core/libs/loader.go` and `cmd/pilot-diff`.

Only the *pass* layer has a compensating hack for the buffer's missing kind
(`session.go dropKerMLNotationOfKerMLFiles` drops the `kerml-notation` warning for spans that came
from a `.kerml` snippet), so a `featured by` warning behaves correctly in the REPL while the
token-level reclassification does not. If a PR claims "these words are names in KerML", check both
paths and expect them to disagree until the buffer carries a per-snippet kind. Note `-convert` prints
no *warnings* at all, so a warning-suppression claim cannot be observed there — use an input where
the keyword breaks parsing outright (e.g. `public import merge;`, or `member step merge : T …`) so
the difference is an error count, not a warning.

## Testing parser changes end-to-end (keyword/symbol parity, dispatch rewrites)

A parser change is only convincing if the *same input file* behaves differently on the two binaries,
so build an A/B baseline first and keep it around:

```bash
git worktree add /tmp/mainwt main
go build -C /tmp/mainwt -o /tmp/mainwt/sysml-main ./cmd/sysml && go build -C /tmp/mainwt -o /tmp/mainwt/sysml-lsp-main ./cmd/sysml-lsp
```

Three cheap, high-signal sweeps:

1. **Corpus no-diff sweep** — load every model on both binaries and diff the output. Anything other
   than `0` differences is either the intended change or a regression, and it takes ~4 min.
   **Both `%load` and the argv positional split the path on whitespace**, and ~80% of the corpus
   lives under directories like `examples/sysml-v2-training/05. Redefinition/`, so copy each file to
   a space-free path instead of passing it — and count what you compared, or a sweep that loaded
   almost nothing still prints a reassuring zero:
   ```bash
   n=0; d=0
   while IFS= read -r -d '' f; do
     cp "$f" /tmp/sweep.sysml
     n=$((n+1))
     diff <(./bin/sysml -quiet /tmp/sweep.sysml </dev/null 2>&1) \
          <(/tmp/mainwt/sysml-main -quiet /tmp/sweep.sysml </dev/null 2>&1) >/dev/null \
       || { d=$((d+1)); echo "DIFF: $f"; }
   done < <(find examples testdata internal/repl/testdata -name '*.sysml' -print0)
   echo "compared $n, differing $d"
   ```
   A `for f in $(find …)` loop word-splits those paths and silently compares nothing for them: on
   PR #98 that hid three real output changes (better diagnostic spans on `part redefines engine = …`)
   behind a clean-looking 0.
2. **Twin table** — for a notation with two spellings, run every degenerate form of *both* spellings
   through a tiny script and print keyword/symbol/main side by side. Testing only the well-formed
   forms hides the interesting bugs: on PR #98 the well-formed forms were perfectly at parity while
   `redefines;`, `redefines = 5;`, `subsets ;`, `crosses ;` were accepted **silently** and their
   symbol twins `:>>;`, `:>> = 5;`, `:> ;`, `=> ;` all reported `expected a name`. A silently
   accepted member is invisible in a `✓ package T` line — always pair the load with
   `%instantiate`/`%features` to see what the member actually did (there, nothing).
3. **LSP diagnostics without an editor** — drive `bin/sysml-lsp` over stdio with ~15 lines of Python
   (`initialize`, `initialized`, `textDocument/didOpen`, sleep 3, read stdout) and count
   `"severity":1` in the `publishDiagnostics` notification. A file with **no** diagnostics produces
   **no** notification at all, so treat a missing notification as zero errors rather than a hang.
   `sysml-lsp: failed reading header line: EOF` on stderr at shutdown is normal.
4. **LSP hover/definition as a symbol-table probe** — the REPL has **no** `%hover`/`%def`, so a
   naming/symbol-registration change is best shown through `bin/sysml-lsp`: reuse the stdio driver
   and add `textDocument/hover` and `textDocument/definition` requests with `id`s, then parse the
   `Content-Length`-framed replies (searching for `"id":100` in the raw text prints the *tail* of the
   message, not its body — frame-parse instead). Hover returns a one-liner like
   `attributeUsage x`, which makes a mis-derived name obvious: on `main` a short-named redefinition
   hovered as `attributeUsage redefines` and its definition was `null`. Make the sleep before reading
   stdout configurable (e.g. `LSP_WAIT=8`); 4 s is occasionally too short and yields
   `NO RESPONSE`/zero diagnostics, which looks like a pass — re-run with a longer wait before
   believing an empty result.
5. **Naming probes for a `<shortName>` change** — for `attribute <sn> redefines x = 5;` the
   assertions that separate working from broken are: `%features` lists the member as **`x = 5`**
   (broken revisions show `sn = 5` with `x = 1`, or a bogus `redefines = <unknown>` slot);
   `%eval T::A::x` **and** `%eval T::A::sn` both evaluate; and an action body doing
   `assign total := total + x` completes instead of `unresolved reference: x`. Load-level `✓ package T`
   proves nothing here.

Go may not be at the blueprint's `/usr/local/go/bin` on every box; check `~/sdk/go/bin` too
(`PATH=$HOME/sdk/go/bin:$HOME/go/bin:$PATH`) before concluding the toolchain is missing.

### Calc-body member dispatch (trailing result expressions vs declarations)

A calculation body's *last member* may be a bare expression that is the calc's result
(`calc def Add { in x; in y; x + y }`). The parser decides per member whether it is looking at a
declaration or at that result expression, and every change to that predicate has broken a
neighbouring shape at least once. Test all four families on the same build, because they are
independent code paths and passing one says nothing about the others:

1. **Prefix operators** at the start of a trailing expression: `-x`, `+x`, `~x`, `not flag`.
2. **Parenthesis-less arrow invocations followed by a binary operator**: `xs->size - 1`,
   `xs->size + 1`, `-xs->size`, `xs->reduce '*'`, `xs->collect Twice`, `xs->includes(3)`. These
   share the argument-start predicate with the prefix family, so broadening one has silently
   broken the other (`expected an expression` on `xs->size - 1`).
3. **Word (keyword) binary operators**, which lex as keywords and so can be mistaken for a
   declaration head: `a and b`, `x or y`, `x xor y`, `x implies y`, `x as Real`,
   `x istype Real`, `x hastype Real`, `x meta …`. The failure signature is
   `error: expected a body member` with the caret on the keyword.
4. **Genuine declarations that must still win**: `x : Real;` inside the same body, `return r;`,
   `return r : Real = a;` — plus the quoted-keyword name form `in 'and'; 'and'`, which must stay
   a valid declaration *and* reference.

Adversarial cases that must diagnose and never panic: `a and;`, `a as;`, `a and b and`,
`a implies`, an unquoted `in and;` (expect `"and" is a reserved keyword, not a name … write
'and' to use it as a name`). Wrap each in `timeout 20` and assert on the *exit code plus the
first diagnostic line*, not just "it printed something".

Validation is not enough for these — evaluate them too, since only execution proves the trailing
expression became the result: `./bin/sysml -calc 'p::andc(true,false)' -calc 'p::orc(true,false)'
… file.sysml` accepts repeated `-calc` flags and prints one `= value` per invocation. Two
operators parse and validate but are **not evaluable** on any revision (verify against a parent
binary before reporting them as a regression): `~` → `unsupported operator: '~': bitwise
complement is declared by no function library the runtime applies`, and `as` →
`unsupported operator: 'as': a cast needs the runtime type of a value, which values do not carry
yet`.

Fixture hygiene for these files: use `public import ScalarValues::*;` (a bare
`import ScalarValues::*;` is itself a diagnostic), and import `SequenceFunctions` /
`ControlFunctions` / `NumericalFunctions` for arrow and operator-name shapes. Trailing
expressions *are* name-resolved as of PR #581, so a body that validated clean on an older binary
can now report `unresolved reference: <name>` — that is the intended tightening, not a bug. The
committed conformance fixtures `internal/core/runtime/testdata/conformance/calc_simple_add.sysml`
and `calc_unary_operators.sysml` lack the ScalarValues import and therefore exit 2 on
`unresolved reference: Integer/Boolean` **on the parent binary too**; judge such a run by the
absence of parse/`return`-related diagnostics, not by the exit code.

REPL trap when testing refusals: after **any** submission fails to parse, the poisoned line
stays in the session buffer, so a later `%eval <symbol>` reports `error: parse failed: <first
error>` for the rest of the session (reproducible on old binaries — pre-existing). `%eval 1 + 1`
and fresh `calc` declarations still work, so this is not a crash; restart `bin/sysml` between
error cases instead of assuming the buffer recovers.

### Naming changes that reach lowering and the runtime

When a parser change alters which declarations carry a name (e.g. `ast.EffectiveName` deriving the
name from a `redefines`/`references` target), the parse is the *least* interesting surface —
consumers reading `Usage.Ident.Name` break silently downstream. The probes that actually distinguish
fixed from broken, each with a visible A/B against `main`:

- **Attribute default** — `attribute redefines x = 5;` in an action body with a statement reading
  `x`; a lost name surfaces as `error: execution failed: eval assignment RHS: unresolved reference: x`,
  not as a parse error.
- **Step ordering** — `action redefines bump { … }` ordered by `then start bump; then bump end;`.
  A lost name fails at lowering: `succession edge references undefined target node`.
- **Trace naming** — `%trace on` then `%continue`; the step must print `token 1@bump`. A generic
  `token 1@usage_action` (the `nodeIdentifier` fallback in `runtime/trace.go`) is the bug signature,
  and it also reproduces with any unnamed step such as `perform worker;`.
- **Calc parameter** — `in redefines factor = 3;` overriding an inherited `in factor = 2`. The
  giveaway is a *wrong number*, not an error: the invocation silently uses the inherited default
  (`Scaled(7)` → 14 instead of 21), so assert the value, never just "it evaluated".
- **State** — `state redefines waiting; accept go then active;`. A lost name makes the sourceless
  accept vanish: `%state` shows `Events: 0` and `%advance 1` never leaves the initial state.

Ready-made fixtures for all of these live in `internal/core/runtime/testdata/conformance/`
(`action_redefined_attribute_default[_symbol]`, `action_redefined_step_ordering`,
`calc_redefined_parameter[_symbol]`, `state_redefined_state_accept[_symbol]`) — load them straight
into the REPL with `%action test::run` / `%eval test::Scaled(7)` / `%state Test::Machine` rather than
writing new models. Where only a keyword fixture exists, `sed 's/redefines/:>>/'` gives the symbol
twin. `action_redefined_step_ordering.sysml` emits a pre-existing
`name conflict: total is already the name of the inherited feature Base::total` on every revision —
not a regression.

Adversarial cases for name derivation: a member with two redefinitions
(`attribute <sn> redefines x, y = 9;`) derives *no* name from them, so it answers to its short name
(`%features` shows `sn = 9` and leaves `x`/`y` at their inherited values) with no diagnostic — assert
which key the value landed under rather than assuming it was dropped. REPL call syntax: named
arguments work as `Scaled(x = 7, factor = 5)`, but *mixing* positional and named
(`Scaled(7, factor = 5)`) is a parse error on every revision — don't read that as a
parameter-naming bug.

## Quantities, unit-name resolution and prompt scope

A name in the unit position of `x [u]` is an ordinary feature reference, so it resolves to the
**nearest** declaration and then must conform to a measurement unit. Testing anything in this area:

- There are **four** evaluator paths that reach a unit name and they must agree: a slot
  (`%instantiate` + `%features`), an action (`%action`), a calc (`%calc`), and a constraint
  (`%constraint`). The constraint path historically diverged — it reached past a nearer declaration
  and silently converted in metres, giving a *wrong answer with no error*, so always include
  `%constraint` and assert the diagnostic, never just "the other three agree".
- The high-value fixture shape is a body that declares `attribute m` (mass!) next to
  `500.0 [m]`, with `public import SI::*` in the enclosing package. Expect, verbatim, from all four:
  `not a measurement unit: m resolves to the attributeUsage m declared in <NS>, shadowing the
  measurement unit SI::metre — write SI::m to name the unit`. Ready-made:
  `internal/core/runtime/testdata/conformance/unit_shadowed_by_sibling_{slot,action,calc,constraint}.sysml`,
  plus `unit_shadowed_by_local_unit` (a sibling that *is* a unit must still evaluate) and
  `unit_undeclared` (`unresolved unit furlong` — a different message, assert it stays different).
- Assert the **neighbouring** quantity too: `1000.0 [kg]` next to the shadowing `m` must still print
  `m = 1000.00 [kg]`. A too-broad fix breaks that and no error-message check would notice.
- The remedy clause (`— write SI::m …`) comes from resolving the name again with the shadowing
  declaration hidden, so it is produced for a shadow declared inside a body *and* for one declared
  at package level next to the `import SI::*` itself. A message that stops at
  `… m resolves to the attributeUsage m declared in ADV` means that second lookup failed — worth
  reporting, since the package-level shadow is the likely real-world spelling.

**Prompt scope.** `%eval`/`%calc` evaluate in the scope of the **last namespace the session
declared** (`Session.promptScope`), which is what makes `%eval 1.0 [m/s]` work for a loaded package
that imports `SI::*`. Consequences to test deliberately, because they surprise users:

- Typing a *new* package mid-session moves the prompt scope: after
  `package Demo { attribute mass = 3.0; }`, `%eval mass * 2` = `6.00` but `%eval 1.0 [m]` starts
  failing `unresolved unit m` (Demo imports nothing).
- With two packages, only the last one's members resolve unqualified (`%eval b * 3` works,
  `%eval a + b` is `unresolved reference: a`); qualify them (`%eval P1::a + P2::b` = `3.00`).
- `%eval <unit>` (e.g. `%eval m`) resolves through that scope and answers
  `error: "m" has no value to evaluate`, not `symbol "m" not found` — the latter is the pre-fix
  signature.

**`%calc` arguments** are parsed as a list of expressions, so an argument containing spaces survives.
All of these must give the same result: `%calc P::Fall 10.0 [m/s], 3.0 [s]` (comma),
`%calc P::Fall(10.0 [m/s], 3.0 [s])` (invocation form), `%calc P::Fall 10.0 [m/s] 3.0 [s]`
(whitespace only), `%calc P::Fall (1.0 + 9.0) [m/s], 3.0 [s]` (parenthesized subexpression),
a nested invocation as an argument, and a trailing comma (accepted silently). Whitespace separates
two arguments only where each side is a complete expression, so `%calc add 5 -3` is two arguments
while `%calc add 5 - 3` is one subtraction (and then reports `parameter "y" has no argument`) —
check a signed second argument both ways. Malformed input must
diagnose and leave the session usable — named args → `named arguments are not supported here; pass
arguments positionally`; unbalanced paren → `failed to parse argument "(…"`; a lone `,` →
`failed to parse argument ","`; no args / `()` → `unbound parameter: …`; too many →
`calc argument count mismatch`. Pre-fix signature: `evaluation of argument "[m/s]," failed:
unsupported node type: *ast.ErrorNode`. Follow the sweep with `%eval 1 + 1` → `= 2`.

**Quantity rendering.** Two separate printers, both worth an A/B against the parent commit:
a violated assertion renders the bracket form as source (`Assertion evaluated to false:
1.0 [m] > 500.0 [m]`; a missing `*ast.IndexExpr` case shows `index > index`), and a result table
formats the magnitude like a bare Real (`%action test::propagate` +`%continue` on
`internal/core/runtime/testdata/conformance/action_body_quantity_descent.sysml` → `t = 17.20 [s]`,
`h = -0.42 [m]`, `v = -42.86 [m/s]`; raw floats such as `17.19999999999997 [s]` are the pre-fix
signature). Note the action in that file is named **`propagate`**, not `descent`.

## Multiplicity, subsetting and collection slots in `%features`

`%features` is the cheapest window on instantiation semantics, and the interesting values are all in
its output rather than in an exit code — so assert on the exact rendered text:

- `part xs : C[*]` should print `xs = []` (an empty collection). `<error: multiplicity violation:
  lower bound too large or infinite for slot "xs">` is the signature of `[*]` being read as `*..*`;
  that error is the *correct* answer only for an explicit `[*..*]` or an absurd bound like
  `[100000]`, both of which must return instantly rather than allocating.
- A nested `part a : Sub :> xs` makes `a` one of `xs`'s values, and a redefinition
  `part ys : C[*] :>> Xs` makes `ys` and `Xs` render the *same* list — check both names, not one.
- Feature chains over a collection (`sum(xs.m)`) flatten one level per step. Useful negatives:
  an empty collection gives `total = 0` (not an error); a chain reaching an unset slot gives
  `<error: … uninitialized slot: m>`; two features subsetting each other give
  `<error: … cyclic slot dependency: … subsets itself>`; mixing a bare number with a `[kg]` value
  gives `<error: … incommensurable units …>`. None of these may panic — follow each with
  `%eval 1 + 1` → `= 2` to prove the session survived.
- `sum`/`product` over quantities must keep the unit (`totalmass = 7.00 [kg]`); a bare `7.00` is a
  regression.
- Unset attributes print `<unknown>`, and an attribute typed by a plain `Real`/`String` with no
  value may instead materialize as a nested `Instance(ID: n)` with `(no features)` — that is
  pre-existing rendering, not evidence of a new bug, so do not report it as one without an A/B
  against `main`.

## Multi-file projects: `%load <path>...` and positional dirs/globs (PR #146)

`sysml <dir|glob|file>...` and `%load <path>...` expand to model files via
`internal/core/project.Expand`, and every file is accepted before one analysis pass
(`Session.SubmitAll`), so load order does not affect name resolution. Shapes to expect:

- More than one file prints a `loaded N files:` header listing each path (a single file prints no
  header — a good tell that the multi-file path was taken).
- Only `.sysml`/`.kerml` are collected; a `.md` sibling and any **hidden** directory (`.git`,
  `.hidden`) are skipped. Put an unparseable file inside a `.hidden/` dir: any diagnostic from it
  means the skip broke.
- Errors are worth asserting verbatim: `no .sysml or .kerml files in <dir>`,
  `no model files match "<pattern>"`, `load <file>: open <file>: no such file or directory`, and
  `usage: %load <file|dir|glob>...` for a bare `%load`.
- Duplicates dedupe by absolute path, so `file dir/` where `dir` contains `file` loads it once.
- `~` and quoted paths with spaces work; quote globs in the REPL (`%load "/tmp/p/*.sysml"`) and on
  the CLI so the shell does not pre-expand them.
- `-convert` still takes exactly one file: a directory gives
  `cannot tell the format of "<dir>": it has no extension, so pass -from`.

**Two regressions fixed late in #146 — re-check both after any change to the load path:**

1. *Per-file diagnostic positions.* Diagnostics from a load are printed as
   `<file>:<line>:<col>: ...` with the line counted from that file's start. With `a-ok.sysml`
   (5 lines) sorting before `b-bad.sysml` whose line 3 is `attribute z = ;`, a directory load must
   report `/tmp/bp/b-bad.sysml:3:17:` — the same position the file reports when loaded alone. A
   joined-buffer line (e.g. `9:17`) or a missing filename is the bug returning
   (`Session.origins`/`Result.locate` attribute an offset to its file).
2. *Every loaded file survives the load.* Two files of one load declaring the same top-level name
   must both stay in the session (`%list` shows both); name-based snippet replacement only
   supersedes **earlier** submissions. Retyping a declaration at the prompt must still replace what
   a previous load put there.
3. *Symlinked directories are walked.* `%load /tmp/link-to-proj` loads the files behind the link,
   symlinked subdirectories included, each resolved directory at most once so a link cycle
   terminates; a dangling link is skipped. A project reached through a symlink is a realistic
   setup, so include one.

To prove order-independence is real rather than accidental, name the referencing file **first**
alphabetically (e.g. `a-uses.sysml` importing a package declared in `b-defs.sysml`). The pre-#146
binary emits `unresolved reference: Defs` / `Widget` / `size` on exactly that input, which is the
strongest available evidence.

Trap while driving the REPL on camera: typing `clear` at the `sysml>` prompt not only errors
(`1:1: error: expected a namespace member`) but leaves garbage in the session buffer that makes a
subsequent `%eval Pkg::part.attr` fail with a parse error. `%clear` and re-`%load` recovers.

## The stream/status contract of a non-interactive run (PR #161)

Since #161 every run that is not a prompt splits its output: **results on stdout** (evaluated values,
conversion bytes, verdict lines, `✓` echoes of what a load declared) and **findings on stderr**
(diagnostics, warnings, the `sysml: … did not analyse cleanly` note, `wrote <file> (ttl, N bytes)`),
with `0` = done, `1` = the model answered a check false, `2` = nothing could be decided. Any change in
`cmd/sysml/{main.go,status.go,check.go}` or `internal/repl/{load.go,render.go}` can break it silently,
so test it as a **table over every mode**, always with `>out 2>err </dev/null` and `echo $?`:
capturing `2>&1` hides exactly the defect. A ready-made driver pattern (one `PASS`/`FAIL` line per
row, asserting status + required stdout needles + stderr needles that must be **absent** from stdout
+ "stdout empty") lives in the session that tested #161; re-create it rather than eyeballing output,
because it is legible on camera and a leak turns a row red.

Values that held at `6079699` (broken = `package Bad { part def A { attribute x : ; } }`):

| run | exit | stdout | stderr |
|---|---|---|---|
| `-e 1+1 <good>` / `-calc`/`-action`/`-state` on a clean model | 0 | `✓ package …`, `= 2` / `= 18` / `total = 5` | empty |
| any of those over the broken model | 2 | **empty** | `error: expected a name`, `sysml: … did not analyse cleanly` |
| `-e nope <good>` (unresolved name) | 2 | `✓ package …` only | `sysml: unresolved reference: nope` |
| `-validate <broken>` | 2 | empty | `… did not analyse cleanly; no check was made` |
| plain `sysml <broken>` **non-tty** | 2 | banner only | diagnostics |
| `-convert ttl <model>` | 0 | Turtle | empty |
| `-convert ttl -o f` | 0 | **empty** | `wrote f (ttl, 1540 bytes)` |
| `-convert ttl <broken> -o f` | 2 | empty | syntax errors, **and no file is created** |
| `-constraint` false / holds / unresolved / warning-only | 1 / 0 / 2 / 0 | verdict line | warning + unresolved verdicts only |

Traps found while testing it:

- **The tty override is the one path unit tests cannot cover.** `atTerminal()` (`status.go`) forces
  status `0` when stdin is a terminal, so `./bin/sysml <broken>.sysml` at a real terminal must load,
  report on stderr, open the prompt and exit `0` on `%quit`, while
  `printf '%%quit\n' | ./bin/sysml <broken>.sysml` exits `2`. Verify the tty half in Konsole (or with
  `script -qec "./bin/sysml <broken>.sysml > /tmp/o 2>/tmp/e; echo TTYSTATUS=\$?" /dev/null < in`);
  a plain pipe silently tests the other branch.
- **`part def ;` is *not* a bad line** — it parses as an anonymous part def and prints `✓ part def`.
  To show the prompt surviving a bad line, use `%eval nope` (unresolved) and
  `part def B { attribute q : ; }` (syntax error), then a following `%eval 6*7` → `= 42` as proof the
  session continued. Expect the `note: deeper checks may not have run here: the error on buffer line
  N is unresolved …` line on the submission after the bad one.
- `-convert ttl` of most models fails on unsupported constructs — a **constraint member** is one
  (`cannot convert the constraint member at …`, exit 2). That makes a check-oriented fixture a useful
  negative row but useless as the clean-conversion row; use
  `package Demo { part def Engine; part def Vehicle { attribute mass : Real = 1200.0; part engine : Engine[1]; } }`
  (1540 bytes of Turtle) for the positive one.
- Real repo models to use instead of hand-written ones: `examples/state-machine-demo.sysml` (and 6
  others) analyse clean, while `examples/phase-c-behavioral-bodies.sysml` and
  `examples/parser_features_demo_action_semantics.sysml` do **not** — a free "broken model" that is
  more convincing than a toy fixture.
- Cheap contrast for this PR class: build the parent commit (`git worktree add /tmp/old<sha> <sha>`)
  and run `-e '1+1' <broken>.sysml` through both. Pre-fix prints the diagnostic **and** `= 2` on
  stdout with exit `0` — the single most convincing frame in the recording.

## Recording setup (Linux/Plasma box)

The GUI is on `DISPLAY=:0` (`:1` does not exist here — `wmctrl` will say "Cannot open display").

No GUI terminal emulator may be installed at all: install one first
(`sudo apt-get install -y xterm`) and maximize it with
`DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz`.

When killing a service from a *scripted* shell, `pkill -f sysml-grpc` matches the shell's own
command line and kills the test shell itself; use `pkill -x sysml-grpc` or the self-excluding
pattern `pkill -f 'sysml-grp[c]'`.

```bash
cd /home/ubuntu/repos/OpenSysML && (DISPLAY=:0 konsole --hide-menubar >/dev/null 2>&1 &)
DISPLAY=:0 wmctrl -a "Konsole"
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Enlarge the font before recording with the `ctrl+plus` key combo a few times (`ctrl+shift+plus`
types literal `+` characters into the shell instead of zooming). Konsole starts a shell whose PATH
lacks the Python that `pip install -e clients/python/` installed into, so `import opensysml` fails there while
it works from a tool shell; run `source ~/opensysml-venv/bin/activate` (or
whichever interpreter `python -c 'import sys; print(sys.executable)'` reports in the tool shell)
as a setup step before recording. `~/opensysml-venv` may not exist at all, and the default `python3`
on PATH can be another project's venv (e.g. `~/repos/fprime/fprime-venv`) whose older
`google.protobuf` makes `import opensysml` die with
`cannot import name 'runtime_version' from 'google.protobuf'`. The reliable fallback is a throwaway
venv off the system interpreter:
`/usr/bin/python3 -m venv /tmp/pv && /tmp/pv/bin/pip install -e clients/python/` (~1 min), then
`source /tmp/pv/bin/activate` in Konsole. Also re-copy the freshly built service
(`make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/`) or the auto-start path serves a stale
revision. Discover expected values with the
piped-stdin form *before* recording, so the recorded run is one clean pass; anything only verified
over a pipe is not visible in the video and should be reported as weaker evidence.

**`%stop` ends a debugging session but does NOT exit the REPL.** After `%stop` you are still at
`sysml>`, so a shell command typed next (`clear; ./bin/sysml`) is submitted as SysML and leaves an
unresolved session error that taints later submissions with
`note: deeper checks may not have run here…`. Use `%quit` when you mean to get back to the shell.

**Never type `clear` at the `sysml>` prompt while recording.** It is submitted as SysML, not run by
the shell, so it leaves an unresolved session error and the next submission gains a
`note: deeper checks may not have run here: the error on buffer line N is unresolved` line that
looks like a defect in the frame. Clear the screen *before* starting the REPL (`clear; ./bin/sysml`),
scroll instead with `shift+PageUp`/`shift+PageDown` when long output (`%builtins`, `%help`) runs off
the top, or quit and restart the REPL for a clean screen.

## Tab completion and anything else readline-driven (PR #148)

This section and the next describe behavior added by PR #148 (which also adds `%search` and
`%builtins`), so they apply only once that PR is on `main`.

`cmd/sysml` installs an `AutoComplete` on the readline config, and **readline disables completion
when the terminal reports a width of 0** — which is what a plain pipe reports. So
`printf '%%bui\t\n' | ./bin/sysml` proves nothing about completion: the TAB is just swallowed.
Two ways to drive it:

1. **Konsole** (what belongs in a recording): send TAB with the computer tool's
   `key: "Tab"`, and discard a line with `ctrl+u` between cases — note `ctrl+u` kills only to the
   left of the cursor, so add `ctrl+k` first if the cursor is mid-line.
2. **A pty harness** (for discovering expected values quickly, off camera): fork a pty, set the
   window size, then write keystrokes. The essential part is the ioctl:

   ```python
   pid, fd = pty.fork()                     # child: os.execv("./bin/sysml", [...])
   fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
   os.write(fd, b"%eval sqr\t")             # then read fd after a short settle
   ```

   The captured stream is full of `ESC[2K` redraws; grep the last redraw of the prompt line rather
   than trying to read it as plain text.

Completion cases worth covering, and their shapes: a unique meta-command prefix completes in place
(`%bui`+TAB → `%builtins`), an ambiguous one lists on the second TAB
(`%s`+TAB TAB → `%satisfy %save %search %features %state %step %stop`), names after `%eval` come from
session declarations, builtin function names and the library (`sqr`→`sqrt`), a qualified prefix
offers **one segment at a time** (`ScalarValues::`+TAB lists only that package's members, never the
whole library; `ScalarValues`+TAB inserts the `::` and lists the same members), and `%load`/`%save`
complete filesystem paths with a trailing `/` on directories.
Adversarial cases that all behaved correctly and are cheap to re-check: TAB with the cursor mid-line
must keep the text to its right, TAB on an empty line lists (capped) names without hanging,
completion into a nonexistent directory inserts nothing, and completion still works on a line long
enough to wrap several terminal rows.

## Where the prompt keeps its history (PR #148)

History is `$XDG_STATE_HOME/sysml/history` when that variable is set (directory created 0700, file
0600), else `~/.sysml_history`; older builds used `$TMPDIR/sysml-repl.history`, so a stale
`/tmp/sysml-repl.history` may be left over from a contrast binary — delete it before asserting "the
new build writes nothing to /tmp". Reuse across runs is testable without a second human: run the
REPL, `%quit`, start it again and press `Up`. An unwritable location must degrade quietly (prompt
starts, no warning, history in memory only) — exercise it with `XDG_STATE_HOME` pointing at a
`chmod 500` directory *and* at a regular file, since those fail in different places
(`MkdirAll` vs `OpenFile`).

## Connector usages and their ends (PR #132)

This section describes runtime materialization of connector usages, so it applies once that PR is
on `main`; the "old" shapes double as A/B canaries against the parent commit.

- **A connector's end IS the connected feature, not a copy.** The only visible signal is the
  instance ID, so read it carefully: for
  `part def Sys { part a : A; part b : B; connection link : Link connect a.p to b.q; }` the fixed
  build prints `a.p = Instance(ID: 4)` / `b.q = Instance(ID: 6)` and then `link = Instance(ID: 7)`
  with `source = Instance(ID: 4)` / `target = Instance(ID: 6)`. A pre-fix build prints *fresh* IDs
  at the ends (`source = Instance(ID: 7)`, `target = Instance(ID: 8)`). Pair the ID check with a
  **value** check by connecting a port that redefines an attribute
  (`port spare : P { :>> rate = 7.0; }`): the end must read `rate = 7.00`, which a default-typed
  copy would show as `3.00`.
- **Untyped/anonymous connectors used to read `<unknown>`.** `interface iface connect a.p to b.q;`
  (and `connection untyped connect …`, `allocation alloc allocate a to b`, KerML `connector`)
  materialize on the stdlib base. A bare `connect a.p to b.q;` has no slot name, so `%features` renders
  it as a synthetic `(anonymous <keyword>) = Instance(ID: n)` line with its ends indented under it —
  those lines are printed *after* all the real features, and their ends are shown without nested
  values. Pre-fix, the named untyped usage printed `iface = <unknown>` and the bare one printed
  nothing at all.
- **End labels tell arity apart.** A binary connector's ends are `source`/`target`; an n-ary
  `connect (a.p, b.q, c.r)` prints three ends all labelled `end` (they occupy the library's unnamed
  `participant` feature). A `connection def Derived :> Base { end :>> source; end :>> target; }`
  redefines inherited ends **by position** and must still attach.
- **Unattachable ends are a typed `<error:` on the slot, never `<unknown>` or a default.** An end
  naming a missing feature gives
  `<error: connector end cannot be attached: anonymous connector end "a.nope" at <repl>:8:17: member nope not found in instance>`
  — assert the `<repl>:line:col` is present, since that is exactly what the pre-fix path lacked.
  A connector with multiplicity (`connection multi : Link[2] connect a.p to b.q;`) gives
  `sys.multi end "multi" at <repl>:8:9: a connector of more than one object has no set of ends to
  attach` — located too, so assert the `<repl>:line:col` on both. Follow either with `%eval 1 + 1` →
  `= 2` to show the session survived.
- **A connector naming itself at an end is `ErrCyclicSlot`, not a hang.** `connection link connect
  link to a.p;` and a mutual pair (`connection here connect there to a.p;` /
  `connection there connect here to a.p;`) both report a cycle.
- **`flow f from a.p to b.p` and `binding bnd bind a.p = b.p` now parse as named declarations**
  (pre-fix: `expected 'to' between flow ends` / `expected '{' or ';' after declaration`). They are
  *not* connector usages for materialization purposes, so a named `flow` still renders
  `f = <unknown>` and a named `binding` gets no `%features` line at all. Don't plan an end-identity
  assertion on them.
- **Variant-interface routing is only partly reachable from the REPL.** `%features` proves the
  *materialization* side: in
  `internal/core/runtime/testdata/conformance/ballandchain_variant_configuration.sysml` the selected
  `engagementRingToBand = engagementRingToBandConnected (Instance ID: 24)` holds
  `engagementRing.ringPort` / `band.ringPort`, not the disconnected variant's ports. But the
  send/accept *routing* side cannot be driven: an action usage does not inherit its `action def`'s
  nodes (`initialize action: no initial node found`), `%action` cannot resolve a nested action of a
  selecting part usage (`unresolved reference: Route::sysDirect::comm`), and a sibling
  `ref :>> link = link::direct;` in the same body does not bind the selection — the run still ends in
  `accept deadlock in action comm: nothing can post the awaited message`. What *is* testable, and
  worth doing, is the negative plus a positive control: a plain (non-variation)
  `interface link connect outPort to inGood;` in an action body routes `send 100 via outPort` and
  completes with `atGood = 100`, while the same model with an unrealized `variation interface`
  returns the typed accept-deadlock instead of delivering to the wrong port or hanging. Treat
  selected-variant routing as unit-test-only coverage.
- **Cheap end-to-end fixture for the whole family:** `internal/core/runtime/testdata/conformance/`
  `connector_end_identity.sysml`, `ballandchain_interface_connected.sysml` and
  `…_disconnected.sysml`; each `.expected.json` has an `identical` / `distinct` array that names
  exactly which end must be which port — the cheapest source of the IDs `%features` should tie together.
- Ball-and-chain reference numbers, for asserting the cost roll-up did not shift:
  `totalCost = 1450.00`, `band` `bandCost`/`ringCost` `= 400.00`, `engagementRing`
  `engagementRingCost`/`ringCost` `= 500.00`, `diamondCost = 550.00`, and
  `engagementRingToBandConstraint: <constraint: satisfied>`.

## Port routing: direction, conjugation and connector end paths (PR #195)

Routing a `send … via p` is only observable through an `accept` that either receives or does not, so
every fixture needs **both** a sender node and a reader node writing an attribute, e.g.
`action reader accept n : Integer via dst { assign got := n; }` — then `%continue` prints
`Results: got = <n>` / `n = <n>`. A model with no reader completes either way and proves nothing.

- **Drive it with `%load <file>` + `%action <name>` + `%continue`** (or `%state <m>` + `%advance 1` +
  `%current` for a transition `accept … via p`). Put `import ScalarValues::*;` in every fixture,
  otherwise the load is noisy with `unresolved reference: Integer — did you mean
  ScalarValues::Integer?` (harmless, but it costs screen space on camera).
- **The two performer paths are different code and both need a run:** `%action Pkg::Part::ship`
  (no object → connectors of the *nearest enclosing part*) and `%instantiate <part usage>` followed
  by `%action Pkg::Part::ship <usage>` (object → connectors of the object's *type*). Same model, same
  expected value; a regression in either one is invisible if you only run one.
- **Direction/conjugation shape:** `port def Chan { out attribute v : Integer; }` with
  `port src : Chan; port dst : ~Chan; connect src to dst;`. `send via src` delivers; `send via dst`
  must fail with
  `error: execution failed: send reaches no receiving port: port "dst" is joined only to outbound
  ends (src)`. The mirror shape with an `in` feature (`port def Ctrl { in attribute c; }`,
  `cin : Ctrl`, `cout : ~Ctrl`) must fail on `send via cin` — worth running, because it catches a
  conjugation applied in only one direction.
- **A port with no flow features is undirected** (receives either way) and an `inout` feature also
  receives — assert a *successful* delivery for both, otherwise an over-strict direction check looks
  like a pass.
- **Nested-path routing needs a decoy to be a real test.** Declare `port p { port q; }` *and*
  `port other { port q; }`, `connect p.q to sink`. Two runs: `send via p.q` → reader gets the value;
  `send via other.q` → `port "other.q" is joined to no port that can receive it` **and** the reader's
  attribute stays at its initial value. Without the decoy the test passes on a build that routes by
  bare last segment.
- **Contrast binary is very cheap here and worth it** (`git worktree add /tmp/old<sha> <sha>`): on the
  pre-fix commit the direction fixture *completes silently* (message delivered nowhere, no error) and
  the nested/part-level fixtures end in `accept deadlock in action ship: nothing can post the awaited
  message (…)`. Those two failure shapes are the proof that the new run means something.
- **`connect a, b, c;` is a parse error** (`expected 'to' between connector ends`); the n-ary form is
  `connect (a, b, c);`. If a multi-end routing test "fails" with `joined to no port that can receive
  it`, check the load diagnostics first — the connector may never have lowered.
- `connect a to a;` (self-connection) reports `port "a" is joined to no port that can receive it` (the
  sending port is excluded from its own receivers), and an end naming a nonexistent feature
  (`connect a to nosuchthing;`) is *deliberately* treated as able to receive, so the action completes
  with no error and no panic. Assert both — they are the "must not crash" cases.

### Routing from a state machine declared inside a part (PR #195 follow-up, 8cffc9e)

`send … via p` inside a `state machine` nested in a `part def` has to walk out through state,
region and transition scopes to reach the part's `connect`. Testing it needs shapes the REPL will
actually descend into:

- **Fixture shape:** `part def Radio { port src : PingPort; port dst : PingPort; connect src to dst;
  state machine { attribute received : Integer = 0; … } }` with `port def PingPort { in item ping :
  Ping; }`. Assert on `%current`'s `State data:` block (`received = <n>`), and drive with
  `%state <Pkg>::<Part>::machine` — a bare `%state Radio::machine` fails with
  `unresolved reference`, the name must be fully qualified (or just `machine` if it is the only one).
- **Cover both send sites**: a state entry (`state waiting { entry send Ping() via src; }`) *and* a
  transition effect (`transition go first waiting do send Ping() via src then sent;`). At the
  pre-fix commit the entry form can already work while the **transition effect** fails with
  `error: event processing failed: transition effect: send reaches no receiving port: port "src" is
  joined to no port that can receive it` — so the transition-effect case is the discriminating one.
- **Nesting: a succession between substates does not descend on its own** in this build — a
  `state outer { state a { entry … } … }` machine whose inner state is only a succession target stays
  in `outer` and the inner entry never runs (reproducible with no part/ports involved, so it is a
  pre-existing state-machine limitation, not routing). Use the **entry-succession form instead**,
  which does descend and shows a `State stack (active configuration): 0. outer / 1. a` in `%current`:
  `state machine { entry; then outer; state outer { entry; then a; state a { entry send … via src; } … } }`
  A `region main { entry; then start; … }` wrapper works the same way.
- **Add a negative for over-reach:** a machine at package level with an unconnected port, and a
  second part with its own `connect ox to oy`. The send must still fail with the typed error — if the
  scope walk goes too far it could pick up the unrelated part's connectors.

## Driving a state machine and its transition effects on camera

- **`-e` is not a file flag.** `sysml -e <expr> [file]` evaluates an expression; to load a model
  non-interactively use `sysml file.sysml` (it loads, then starts the REPL, so pipe `%quit` in) or
  `%load` from the prompt. Passing a path to `-e` yields the misleading
  `error: no declarations loaded (literals work, but feature references need declarations)`.
- **`sysml -trace <file>` plus `%state <name>` then `%advance <n>` is the strongest available
  evidence that a transition effect ran and in what order.** The trace prints
  `exit: <src>` → the effect's `eval …` lines → `enter: <tgt>` → `transition: src -> tgt`, so an
  effect that was parsed but dropped is visible as a missing `eval operator …` line. Note the
  trace of a state machine's own attribute is *not* readable with `%eval <attr>` afterwards —
  that reports the declared default (e.g. `= 0`), not the executed value, so assert on the trace
  line (`eval operator + -> 1`) instead of on `%eval`.
- **`%state` works on a `state def` as well as a state usage** (`%state P::S`); the executor starts
  in whatever state the `entry; then <s>;` chain reaches.
- **Conformance fixtures under `internal/core/runtime/testdata/conformance/` often write bare
  `Integer`**, which the conformance harness resolves but the REPL does not: loading them prints
  `error: unresolved reference: Integer`. That is REPL-only noise, not a regression — when a test
  asserts "loads with no diagnostics", copy the fixture through
  `sed 's/\bInteger\b/ScalarValues::Integer/g'` first.
- **A `do perform <Action>` effect needs the action to have a body.** `action def Bump;` with no
  body parses fine but fails at run time with
  `event processing failed: transition effect: invoke action Bump: initialize action: no initial
  node found in action Bump` (the constructor-succeeds/`initialize()`-errors contract). For an
  executing `perform` effect use a fixture whose action has `first`/`then` nodes, e.g.
  `conformance/state_transition_effect_perform.sysml` (counter 1 → 11).
- For transition-terminator/`;` work specifically, the discriminating inputs are:
  `transition first a do assign x := x + 1 then b;`, `transition first a do … then b;`,
  `do { … ; };` (braced), `do perform A;`, plus the negatives `;;`
  (`error: expected a body member`), no `;` (both `expected ';' after assignment` *and*
  `expected ';' after transition`), braced with no trailing `;`
  (`expected ';' after transition`), and a nested statement inside a braced effect missing its own
  `;`. An A/B against a binary built from the parent commit is what separates "fixed" from
  "never broken" here.

## Reading a state machine's attribute values, and notation that actually drives it

Discovered while testing inline `entry action { … }` bodies and calc `out` assignment (PR #135).

- **`%current` is the way to read a state machine's attributes.** It prints a `State data:` block
  (`c = 6`) plus the active configuration / state stack for composite and orthogonal machines.
  `%eval <attr>` is *not* a substitute: it reports the declared default, and for a name declared in
  two `state def`s of one package it fails with
  `error: symbol "c" is ambiguous: P::S1::c, P::S4::c (use a qualified name)`.
  `%advance <n>` additionally prints `Do behavior actions run: <k>`, which is how you tell a do
  behavior actually ran from a machine that merely idled (`No pending work - simulation time is now …`).
- **`transition first a accept when true then z;` never fires.** A `when` trigger is a change
  trigger evaluated on an event, and a machine with no events sits in `a` forever — so a fixture
  written that way silently tests *only* the entry behavior and never the exit behavior. To exercise
  exit behaviors and ordering, use **completion transitions**: `entry; then start; … then start work;
  then work done;` (the `state_anonymous_action_body.sysml` conformance fixture is the model to copy).
- **An inline body is one action per do round.** After a do body has run to its end the state has
  no more pending work, so further `%advance` calls do not re-run it; a counter incremented by a
  `do action { … }` reaches 1 and stays there unless a transition re-enters the state. The
  one-action-per-statement `do { … }` form is what interleaves and re-runs per statement.
- **Notation gotchas that cost fixture rewrites:**
  - a self-send must name the machine, statement style: `entry action { send Ping to Driver1; }`
    with `item def Ping;` — `send Sig() to self` with an `attribute def` parses but never delivers.
  - inside a body write `perform Work;`, never `perform Work();`
    (`error: expected ';' or '{' after 'perform' action reference`).
  - an action a `perform` targets needs `first`/`then` nodes, else
    `initialize action: no initial node found`.
  - `then work done do assign x := 1;` does not parse (`expected ';' after succession edge`);
    use the `transition first work do assign x := 1 then done;` form for an effect.
- **`perform <Action>;` inside an inline body** works as of PR #135 (before it, it failed with
  `state behavior <anonymous>: action usage "Work" in a body is not executable` while the same
  `perform` in the statement-form body `entry { … perform Work; }` executed). It is a good
  discriminating fixture whenever body lowering changes.
- **Calc `out` features:** read them through a usage (`calc c : Def { in n = 5; } attribute a :
  Integer = c.a;`) and inspect with `%features <part>`; a failing read shows inline as
  `a: <error: slot p.a: …>` rather than aborting the listing, so one `%features` can carry several
  independent negative assertions. Useful expected strings: `no value: output never assigned`,
  `output bound more than once` (declaration value plus a body assignment; two *body* assignments
  are legal, the last one winning), `assignment outside the calculation body: <name> is not declared
  by the calculation`, `no result expression: calc … has no return expression`.

## "Did you mean" suggestions on unresolved references (PR #167)

The suggestion is produced in the resolver (`internal/core/suggest` + `internal/core/resolve/suggest.go`),
so it belongs to the diagnostic and every surface renders the same string. Verify all three surfaces —
they used to disagree, and the REPL used to post-annotate its own copy:

```bash
./bin/sysml -validate f.sysml            # CLI: "... — did you mean ScalarValues::Integer?", exit 2
printf 'part def A { attribute x : Integer = 1; }\n' | ./bin/sysml | grep -c 'did you mean'   # must be 1
```

- Canonical fixture: `part def A { attribute x : Integer = 1; }` → `unresolved reference: Integer —
  did you mean ScalarValues::Integer?` and exit `2`. A bare library name is *supposed* to fail:
  only top-level packages are in the root namespace, so the fix is `private import ScalarValues::*;`
  inside a `package`, which must then validate `✓ … no errors` / exit `0`.
- **Duplication is the regression to watch in the REPL.** Count `did you mean` occurrences rather
  than eyeballing them, and compare the text after `error: ` with the CLI's byte for byte. A
  parent-commit binary is the cheapest contrast: pre-fix the CLI prints *no* suggestion while the
  REPL prints one, so "old REPL also showed a hint" is expected and not evidence of duplication.
- The LSP path is checkable in ~15 s without VS Code: drive `./bin/sysml-lsp` over stdio with a
  three-message script (`initialize`, `initialized`, `textDocument/didOpen`) and read
  `textDocument/publishDiagnostics`; the message must match the CLI exactly.
- Shapes with deliberately different behavior, all worth asserting:
  - a **qualified** unresolved name gets NO `did you mean` — it reports
    `(no namespace "Nowhere" is loaded; "Integer" is declared as …)` instead;
  - operator members are never offered (`typable()` filters `IntegerFunctions::+`, `#`, `..`), so a
    typo like `Plu` yields no suggestion while `Abz` → `RealFunctions::abs`;
  - a long garbage identifier (≥ 100 chars) must yield zero suggestions and still exit `2`;
  - edit-distance tolerance widens with length (1 / 2 / 3 at ≥ 6 / ≥ 9 runes), so a 6-char typo like
    `Intger` legitimately offers noisy extras (`Items::Item::boundingShapes::edges::inter`,
    `VectorFunctions::inner`) after the right answer — assert the *first* candidate, not the list.
- **`examples/` is a gate.** All 20 files under `examples/` (excluding `examples/sysml-v2-training`)
  must validate with exit 0:
  ```bash
  for x in $(find examples -name '*.sysml' -o -name '*.kerml' | grep -v sysml-v2-training); do
    ./bin/sysml -validate $x >/dev/null 2>&1 || echo "FAIL $x"; done
  ```
- Fixtures for the parser/semantics fixes shipped alongside (each fails on the parent commit):
  `binding b of f = v;` (old: `type must be a definition, found attributeUsage`),
  a **constraint usage** — not `constraint def` — declaring `in` params plus `assert`/`assume`
  conditions (old: `expected '{' or ';'`), and
  `vertices->ControlFunctions::exists{p : Point; p.x > 0}` (old: `no scope for member lookup in p`).
- **REPL adversarial gotcha:** typing unbalanced braces (`part def }}} {{{ ;;;`) puts the prompt into
  multi-line continuation (`...>`) and silently swallows subsequent lines. Press `ctrl+c` to abandon
  the buffer — it flushes the accumulated parse errors and returns a clean `sysml>` prompt, which is
  itself a good no-panic assertion.

## Constraint/requirement checks: which object the verdict is about (PR #176)

`-constraint`/`-requirement` (and `%constraint`/`%requirement`) pick their subject in this order:
the object instantiated under the name the condition was reached by, else the *single* session
object whose type conforms to the type declaring the condition, else declared defaults.

- The suffix is the tell: a verdict about an object reads
  `✗ Constraint P::Sensor::inRange failed (on P::hot ID: 1)`, a defaults-only verdict has **no
  `(on …)` suffix**. Assert the suffix, not just ✓/✗ — a defaults evaluation of a fixture whose
  defaults happen to hold looks like a pass either way.
- Exit statuses (`cmd/sysml/status.go`): 0 holds, 1 fails, **2 unevaluable/unresolved**. The
  ambiguity answer is exit 2 with the message
  `X is carried by more than one object of this session (a, b): check it on one of them, …`,
  written to **stderr** with a `sysml:` prefix (`error:` in the REPL) and with **no ✓/✗ line** —
  so a build step never reads a verdict that was not decided.
- Build a fixture where the *def* declares a default that satisfies the condition and the *usage*
  redefines it to violate it (`attribute reading = 50.0` vs `:>> reading = 150.0`). Without the
  default, the pre-fix binary errors with `unresolved reference: reading` instead of the silent
  `✓ passed`, and the contrast is much less convincing.
- Carriers include subtypes: `part def Heavy :> Sensor`, `part hotter :> hot`, an abstract def's
  usage, and a `variation part` all count — instantiating two of them makes the check ambiguous.
  An instance of an unrelated type does **not** count and leaves the defaults path.
- Since PR #180 a condition that **could not be evaluated** (uninitialized slot, unbound subject)
  prints `? <what> could not be evaluated` + `  Error: <why>` and exits **2**; it keeps the
  `(on …)` suffix when it is about an object. Before #180 it printed `✗ … failed` / `✗ … fails`.
  Assert all three states in one pass — `✓ … passed` exit 0, `✗ … failed` exit 1 (with
  `Assertion evaluated to false: <condition>`), `? … could not be evaluated` exit 2 — and grep the
  **verdict line only** for the old wording: the `Error:` reason legitimately contains the words
  "evaluation failed", so a bare `grep -c failed` over the whole output is a false positive.
  `%features` mirrors the three states as `<constraint: satisfied>` / `<constraint: violated>` /
  `<constraint: not evaluated: …>` (the `not evaluated:` prefix is #180's). `-json` reports
  `"status": "unresolved"`, `"exit": 2` and the same `?` line.
- CLI checks refuse to run at all on a model that does not analyse cleanly
  (`… did not analyse cleanly; no check was made`, exit 2), so an unevaluable-verdict fixture must
  itself be error-free — put the `nonexistent > 0` style constraint in a *separate* file
  from the unbound-subject requirement, or the CLI never reaches the verdict.
- PR #180 made `%eval` pick the same subject as the checks (`Session.subjectFor`): with `P::hot`
  instantiated, `%eval P::Sensor::reading` answers `= 140.00 (on P::hot ID: 1)`; with `P::hot` and
  `P::cold` both instantiated it refuses with `error: … is carried by more than one object of this
  session (P::cold, P::hot): name one of them, …`; with nothing instantiated it answers the
  declared default and prints **no** `(on …)`. Subtype carriers (`part hotter :> hot`) work too.
- Still a blind spot after #180: a **nested part** whose value is redefined on the instantiated
  object is not the subject. With `part o : Outer { part :>> b { attribute :>> c = 99.0; } }`,
  `%features A::o` shows `c = 99.00` and `small: <constraint: violated>`, yet `%eval A::Outer::b::c`
  answers `= 5.00` and `%constraint A::Inner::small` says `passed` (identical on the parent
  commit, so it is pre-existing, not a regression — but it reads as a contradiction and may be
  worth flagging). Also `%eval A::b::c` (skipping the def segment) is `unresolved reference` in
  both revisions; the resolvable spellings are `A::Outer::b::c` and `A::Inner::c`.
- **The nested-subject blind spot above was fixed (PRs #206/#219/#236).** Now `%constraint`,
  `%requirement` and `%satisfy` evaluate the nested object and *name* it: with
  `part o : Outer { part :>> b { attribute :>> c = 50.0; } }`,
  `%constraint A::Inner::small` answers `✗ Constraint A::Inner::small failed (on A::o::b ID: 2)` —
  `<session-held root>::<feature path>`, matching the `b = Instance(ID: 2)` line of `%features A::o`.
  Assert the *whole* suffix: a build that regressed to the outer holder still prints a `✗`, and the
  pre-#236 binary prints the same `✗` line with **no suffix at all**, so ✓/✗ alone proves nothing.
  Ordinary (non-nested) checks keep labelling the object handed in (`(on A::w ID: 3)`), and a
  no-object run still prints no suffix.
  - Such a label is **descriptive, not addressable**: `%features A::o::b` answers
    `error: no instance of "A::o::b"` and `%instances` lists only `A::o`. Worth reporting whenever
    label spelling changes, and a good adversarial step after any "name the subject" PR.
  - Multiplicity-materialized subjects read `(on A::car::wheels ID: 2)` in the REPL while gRPC still
    reports the *definition* (`instance_type_id 'A::Bolt'`/`'A::Wheel'`) — the two surfaces disagree
    by design so far; check both rather than assuming one from the other.
- **Ambiguity carrier labels: check every fixture shape, they regress independently.** The message is
  `ambiguous subject: <cond> is carried by <Def> #<id> (<feature path>), …`. Three shapes worth
  running together, because a change that improves one can flatten another:
  `front`/`rear` nested parts → `Bolt #4 (front::bolt), Bolt #5 (rear::bolt)` (pre-#236 both read
  `(bolt)`); subsetting into a shared collection (`part subsystem : Component[*]` fed by
  `part small : Component :> subsystem` / `large`) → at `658076e9` `Component #2 (small),
  Component #3 (large)` but at #236 both read `(subsystem)`, because the label now shows the
  features *walked* (the collection slot) rather than the declaration materialized; recursive
  containment → `Leaf #2 (leaf), Leaf #5 (next::leaf)`. Always time the recursive/mutual fixtures
  (`time ./bin/sysml recursive.sysml < cmds`, ~0.22 s) whenever the occurrence key changes.
- **gRPC classifies an ambiguity since PR #236:** `FailureReason.FAILURE_REASON_AMBIGUOUS_SUBJECT`
  (enum 3, `api/proto/sysml.proto`). opensysml exposes **no public property** for it — read
  `verdict._pb.failure_reason` and name it with
  `from opensysml.proto import sysml_pb2 as pb; pb.FailureReason.Name(...)`; `Verdict` has no
  `failure_reason` attribute and `connection._failure_of` maps only `WRONG_KIND`, so a client
  wanting to branch on ambiguity today reaches into a private field. An ordinary violated condition
  comes back `holds=False` with `FAILURE_REASON_UNSPECIFIED` (0), i.e. "plain violation" is the unset
  value — assert the ambiguous *and* the violated case in the same run so a hard-coded reason cannot
  pass both.
- `-instantiate` objects are created **before** `-e` expressions since PR #180, so
  `sysml -instantiate P::hot -e P::Sensor::reading m.sysml` prints the instance line first and
  answers `140.00 (on P::hot ID: 1)`; the parent binary answered `0.00`. Any change to
  `cmd/sysml/check.go`'s ordering needs the other combinations re-walked: multiple `-e` (still in
  flag order), `-e` with `-constraint` (exit 1), a failing `-e` (aborts with exit 2 before later
  `-e`s), `-validate`, and `cat m.sysml | sysml -e '1+1'` (piped stdin works for `-e`, but named
  checks over piped stdin refuse with `no model to check; name the file …`).
- Konsole typing trap: the `✗` glyph does not survive the computer tool's `type` action into a
  shell command (it arrives empty, and `grep -e ''` then matches every line). Build the pattern in
  the shell instead: `X=$(printf '\u2717'); … | grep -nE "$X|could not be evaluated"`.
- `%features <usage>` is the independent oracle — `inRange: <constraint: violated>` must agree with
  the verdict for the same object. Declaring anything new in the REPL drops all instances, so a
  following check silently reverts to defaults (assert the missing `(on …)` suffix there).

## A lone `-` as standard input (PR #179)

`internal/core/project.ReadFile` reads `os.Stdin` once (memoized with `sync.Once`) whenever a path
is exactly `-`, names it `<stdin>` in diagnostics, and refuses a terminal with
`standard input is a terminal; redirect it or name a file`. That makes stdin usable in every
path-taking mode: `-validate`, `-e`, `-calc`, `-constraint`, `-action`, `-state`, `-convert`, and a
plain positional load.

The convincing test is a **paired run**: `bin/sysml -validate m.sysml` against
`cat m.sysml | bin/sysml -validate -`, for a clean *and* a broken model. Everything but the model
name must match, including the line:column of each diagnostic and the exit status (0 / 2). Build the
parent revision (see the contrast-binary recipe) to show the same pipe answering
`sysml: cannot read -: no such file or directory` before the change.

Cases worth walking, with what they should answer:

| command | expected |
|---|---|
| `cat m | sysml a.sysml - -validate` | `✓ a.sysml, <stdin>: no errors` — flags may follow the paths |
| `cat m | sysml -validate - -` | `✓ <stdin>, <stdin>: no errors`, exit 0 (one memoized read, no hang) |
| `: | sysml -validate -` | `✓ <stdin>: no errors`, exit 0 — empty stdin is an empty model, not an error |
| `head -c 200000 /dev/urandom | sysml -validate -` | ~1800 `<stdin>:L:C: error: expected a namespace member`, exit 2 |
| `sysml -convert ttl -` | refused: `standard input carries no file name to take the format from`, exit 2 |
| `cat m | sysml - -convert ttl -from sysml` | conversion on stdout; `-o f` writes `wrote f (ttl, N bytes)` |
| `cd d && sysml -validate ./-` | reads a file *really* named `-`; a bare `-` in that directory still reads the pipe |
| `%load -` at the interactive prompt | `error: cannot read <stdin>: standard input is a terminal…`, and the REPL keeps going |

Two traps when testing this:

- **Always wrap in `timeout`.** The whole risk of the change is a run that blocks on a stdin nobody
  redirected. For the terminal case a pipe is not enough — give the process a real tty with
  `script -qec "timeout 10 bin/sysml -validate -; echo TTYSTATUS=\$?" /dev/null </dev/null`; it must
  print the refusal immediately with status 2.
- **Do not read the exit status through `| head`.** `$?` is then `head`'s, and `${PIPESTATUS[0]}` is
  `cat`'s; a SIGPIPE kill shows up as 141 and looks like a defect. Redirect to files
  (`>out 2>err; echo $?`) and grep them instead.
- Piping a *model* into a plain `sysml -` also consumes the prompt stream, so the REPL prints its
  banner and exits at once with the load's status — expected, not a hang. Inside a piped prompt
  session, `%load -` swallows the remaining script lines as model text, which is worth noting but is
  inherent to one stdin.

## `sysml-lsp` command line (PR #179)

`cmd/sysml-lsp` parses args with a `flag.FlagSet`: `-version`/`--version`/`-v` print the version and
`-h`/`-help` the usage, both on **stdout** with status 0; an undefined flag or a stray positional
argument prints `flag provided but not defined: …` / `sysml-lsp: unexpected argument "…"` plus the
usage on **stderr** with status 2, and nothing on stdout. Assert the stream split
(`>out 2>err`, then `wc -c <out`), not just the text — the point of the change is that a misuse no
longer enters protocol mode and dies with `failed reading header line: EOF` (which is what the parent
binary does, exit 1).

With no args it still speaks LSP. Drive it from Python: write
`Content-Length: N\r\n\r\n<json>` frames to stdin, read the header line, blank line, then N bytes.
`initialize` must answer `"capabilities"` with `completionProvider`, and `shutdown` `{"result":null}`.
**Pre-existing, not a regression:** after the `exit` notification the process only ends when stdin is
closed, and then reports `sysml-lsp: failed reading header line: EOF` with status 1 — identical on the
parent binary, so verify against the contrast binary before reporting it.

## Static dimension (unit-commensurability) warnings (PR #184)

`checkDimensions` (`internal/core/passes/typecheck_dimension.go`) emits a **type-tier warning** for
`+ - < > <= >= == !=` when both operand dimensions are statically known and incommensurable, e.g.
`operator '<' combines incommensurable quantities: ISQBase::MassValue (dimension M) and m (dimension L)`.
It is a warning, so `-validate` still exits **0**; evaluating the same constraint still fails with
`incommensurable units: cannot express m (SI::metre) in kg (1000·SI::gram)` and exit **2**. `-quiet`
suppresses it; `-json` reports it as `"severity":"warning","pass":"type","code":"type.expr"` with
`"exit": 0`.

Fixture gotchas that cost time here:

- Every fixture needs `public import ScalarValues::*;` if it mentions `Real`/`Boolean`, else
  `unresolved reference: Real — did you mean ScalarValues::Real?` aborts before any check runs
  (`did not analyse cleanly; no check was made`, exit 2) and the test proves nothing.
- Unit names are the stdlib's: `t`, `deg`, `degC` do **not** resolve; `°`/`°C` do not even lex in a
  `[...]` unit position. Use `g`, `K`, `rad`, `Hz`, `Bq`, `N * m`, `J`. `ISQ::TemperatureValue` is an
  **alias** for `ISQBase::ThermodynamicTemperatureValue`.
- **Naming an attribute `m` shadows `SI::metre`**, so `1.0 [m]` inside that scope is not a quantity
  at all and every dimension check there goes silent. The runtime says so explicitly
  (`m resolves to the attributeUsage m declared in …, shadowing the measurement unit SI::metre`).
  Never name a fixture attribute after a unit unless shadowing is the thing under test.

Cache interaction: `libs.formatVersion` bumped 15→16 and the cache persists `DimensionFacts`
(`~/.cache/sysml-ls/libs`, or `$XDG_CACHE_HOME/sysml-ls/libs`). To exercise both paths,
`rm -rf ~/.cache/sysml-ls/libs` and run **the very first** validation on the file you care about —
only that first run is cold, since it repopulates the cache for every later file in the same loop.
Known cosmetic divergence: cold names the quantity type by leaf name (`MassValue`), warm by
qualified name (`ISQBase::MassValue`); assert verdicts and exit codes rather than byte-identical text.

Silent-by-design shapes (assert zero `warning:` lines): commensurable comparisons (mm vs m, m/s vs
km/h), dimensionless arithmetic, `xs#(1)` indexing, an untyped attribute later `assign`ed a quantity,
a calc *invocation* result operand, and a calc body written without `return` (`calc def C { in q : …;
q < 1.0 [m] }` never warns, while the `return : Boolean = …` form does). A **typed** parameter
(`in q : ISQ::MassValue`) *does* warn — its dimension is statically known — and an operand whose type
is reached through a stdlib **alias** currently does **not** warn even though the un-aliased twin does.

Cheap false-positive sweep, worth running for any diagnostic-adding pass:

```bash
for f in $(find examples testdata -name '*.sysml'); do ./bin/sysml -validate "$f" 2>&1 \
  | grep 'incommensurable quantities'; done   # expect no output (403 files, ~90 s)
```

## Verifying hand-written release notes against built binaries

When asked whether release notes are honest, judge each sentence separately and re-measure every
number rather than quoting the table.

- **Transcripts are layout-sensitive.** A quoted diagnostic like `model.sysml:6:9: warning: …` is
  reproducible only if the fixture puts the offending expression on that exact line/column. Put the
  comparison on its own line at the quoted column (`constraint c {` on the line above) and copy the
  fixture to the quoted file name (`/tmp/model.sysml`) before deciding a transcript is wrong.
- **Model sweeps: quote your loop.** `examples/sysml-v2-training/*` directory names contain spaces,
  so `for f in $(find examples -name '*.sysml')` word-splits and inflates the count (395 tokens for
  110 files) while every conversion fails on the broken path. Always use
  `while IFS= read -r f; do … done < <(find examples -name '*.sysml' | sort)`.
- **Round-trip claims need a validity control.** Many training files do not validate standalone
  (their dependencies live in sibling files), so a ttl→sysml re-validation failure is not proof that
  the round trip is lossy. Measure three numbers: converted, converted-and-original-validates, and
  round-tripped. Over the 120 `.sysml` + `.kerml` models under `examples/` at 0.0.8 that is
  71 / 44 / 44 (65 / 38 / 38 counting the 110 `.sysml` files alone — state the denominator, since
  the published limitation counts both languages).
- **Counted rows go stale fast, and the Python row depends on the environment.** With no service
  listening `pytest clients/python/tests/ -q` was 369 passed / 26 skipped at 0.0.8; with a service already
  listening the integration tests run instead of skipping (since PR #204 nothing fails either way,
  and CI now starts a service). Say which way a row was measured. `go test -race -count=1 ./...`
  was 3,682 pass / 5 skip / 3,687 total at 0.0.8, and 4,440 / 7 / 4,447 at 0.0.9 — recount rather
  than trusting either. Count **first-level** subtests: `variant_connection_per_owner` registers one
  sub-subtest per owner, so counting every `=== RUN` line reported 299 conformance cases where 297
  fixtures exist, and inflated the robustness row from 173 to 199. Four documents are allowed to
  state counts (`docs/project/spec-compliance.md`, `README.md`, `docs/project/roadmap.md`,
  `docs/project/training-examples.md`) and must be updated in one commit; anywhere else, link to
  spec-compliance rather than adding a fifth copy.
- **Error-class claims: check the export path.** A class can exist in `opensysml.errors` and be absent
  from the package surface — `hasattr(opensysml, name)` is the check, and
  `TestPackageSurface` in `clients/python/tests/test_errors.py` now locks every exception in
  `errors.__all__` onto `opensysml`.
- **LSP capability claims** are cheap to check with a framed JSON-RPC driver: assert
  `semanticTokensProvider` has `full: true`, `range: true` and no `delta` key anywhere, that
  `range` returns strictly fewer tokens than `full`, that `semanticTokens/full/delta` answers
  `-32601` and the server still serves a later request, and that each code action carries a
  `WorkspaceEdit` whose `newText` actually fixes the file. A missing-semicolon fix needs a fixture
  the parser recovers with a fix on — `action def A { first start }` yields `Insert ';'`; a plain
  attribute declaration yields a diagnostic with no fix.

## Subject-aware `Evaluate`, attribute metadata and generated classes (Track P, PR #218)

- **`pandas` is not installed by the blueprint but `Symbol.to_dataframe()` needs it.** If
  `to_dataframe()` raises `ModuleNotFoundError: No module named 'pandas'`, run
  `$HOME/pv/bin/pip install pandas -q` first — the failure is environmental, not a product defect.
- **Subject evaluation is the difference between the *declared default* and the *object's slot*.**
  A fixture that distinguishes them is mandatory: `part def Vehicle { attribute mass = 1500.0; }`,
  `part def Sedan :> Vehicle { attribute :>> mass = 1200.0; }`, `part sedan : Sedan;`. Then
  `m.eval('mass', context_symbol_id='Demo::Vehicle')` must be `1500.0` while
  `m.eval('mass', subject='Demo::sedan')` must be `1200.0`. Without the redefinition both paths
  return the same number and the test proves nothing.
  - On `Model` the kwarg is `subject=`; on `Connection.eval` it is `subject_symbol_id=` and the
    model hash comes **second**. Module-level `opensysml.evaluate` also takes `subject=`; the
    deprecated `opensysml.eval` forwards to it and warns `DeprecationWarning`.
  - A `subject=` plus an explicit `context_symbol_id=` still reads the object, and derived slots
    (`attribute doubled = mass * 2.0`) follow the object too.
  - A *definition* as subject is accepted (it is instantiated and reads its own redefinition), and an
    unrelated subject (e.g. an attribute) is tolerated rather than rejected — record those as
    observed behavior, not failures. An unknown FQN gives `ExecutionError: subject not found: <fqn>`.
  - A subject with a cyclic slot raises `ExecutionError: ... cyclic slot dependency: <feature>` and
    the service must still answer afterwards — always follow such a case with `m.eval('1+1') == 2`.
- **Cross-checking against the REPL needs fully-qualified expressions.** `%eval mass` on a model with
  several `mass` features fails with `symbol "mass" is ambiguous`, whereas gRPC `subject=` resolves
  the unqualified name inside the subject's scope. Use `%instantiate Demo::sedan` then
  `%eval Demo::sedan::mass` / `%features Demo::sedan` to compare; that is a surface divergence in name
  resolution, not a wrong value. Drive it non-interactively with
  `printf '%%load …\n%%instantiate …\n%%eval …\n%%quit\n' | ./bin/sysml`.
- **Attribute metadata checks worth making**: own attributes come before inherited ones; a
  redefinition masks the attribute it redefines (`Sedan.mass` = 1200.0, not two `mass` rows);
  `attributes()` ids point at the *declaring* supertype (`Demo::Vehicle::name`); a **non-constant**
  default such as `mass * 2.0` must report `value=None` rather than a guessed number; a quantity
  default `120.0 [kg]` reports value and `unit='kg'` separately; an element with no attributes yields
  `[]` and an empty DataFrame that still has all 8 columns
  (`name,kind,id,type,multiplicity,value,unit,inherited`).
- **Stdlib elements themselves are not reachable through `Model.find`/`m['ISQ::MassValue']` (returns
  `None`)**, so "attributes of a stdlib element" can only be covered indirectly: declare
  `attribute m : ISQBase::MassValue = 3.0 [kg];` in your own model and assert the *resolved* type
  string `ISQBase::MassValue`.
- **Generated classes**: `python -m opensysml.generate model.sysml -o out.py` then actually `import` the
  module — an MRO bug only surfaces at import as
  `TypeError: Cannot create a consistent method resolution order`. Assert `cls.__mro__` names against
  the model (include multiple supertypes and a diamond), that a `:>>`/`subsets` feature inherits the
  type/multiplicity it does not restate (`String[0..*]` → `-> list[str]`), and that any base Python
  cannot linearize, or that has no generated class, is dropped **with a comment** such as
  `# specializes G2::Hybrid, left out: Python cannot linearize it with the bases above` /
  `# specializes ISQBase::MassValue, which has no generated class`.
- **Quantity results carry the scalar on `Quantity.magnitude`, not `.value`** — a probe using `.value`
  reports a false failure even when the runtime is correct.
- **A stale `~/.opensysml/bin/sysml-grpc` silently blocks the subject/attribute surface.** These features
  are capability-gated (`evaluate_subject`, `symbol_attributes` in `opensysml/capabilities.py`), so a
  service built before they landed makes `conn.eval(..., subject_symbol_id=…)` /
  `attribute_facts()` / `to_dataframe()` raise `MissingCapabilityError` instead of answering — which
  looks like a client bug. Always reinstall the binary before testing a merge:
  `make build-grpc && pkill -x sysml-grpc && rm -f ~/.opensysml/sysml-grpc-50051.pid ~/.opensysml/sysml-grpc-50051.lock && cp bin/sysml-grpc ~/.opensysml/bin/`
  (the `cp` fails with `Text file busy` while the old one still runs), then assert
  `sorted(conn.server_info().capabilities)` contains both names before trusting any result.
- **The auto-started service dies with the session that started it.** After an interactive
  `opensysml` REPL exits, `OPENSYSML_REQUIRE_SERVICE=1 pytest tests/` aborts during collection with
  `$OPENSYSML_REQUIRE_SERVICE is set … but none answers on localhost:50051`. Start one yourself first:
  `nohup ~/.opensysml/bin/sysml-grpc -port 50051 >/tmp/svc.log 2>&1 &`.
- **`PINNED_SHA256` is nested `repo -> version -> asset`.** To exercise the *contradicted* digest arm
  you must inject the key for the repository actually in use, e.g.
  `binary.PINNED_SHA256['Open-MBEE/OpenSysML'] = {'v0.0.8': {'sysml-grpc-linux-amd64': 'de'*32}}`
  (`DEFAULT_GITHUB_REPO`, looked up case-sensitively, so the spelling must match exactly);
  a mis-cased, flat or `in`-substring patch leaves the pin absent and you silently re-test the *unpinned* arm
  (`UnpinnedReleaseError` + kept cache) while believing you tested the contradiction.

## Symbol *kind* changes: `%search` is the only REPL probe (PR #210)

When a change moves a declaration from one `symbols.SymbolKind` to another (modifier-driven usage
kinds, KerML classifier classification, `classifyUsage` edits), `-validate` proves nothing: a model
can be clean on both revisions while every kind is wrong. The probe is the REPL's
`%search <prefix>`, which prints `<fqn>  <kind>` from `sym.Kind.String()`
(`internal/repl/discover.go`, names in `internal/core/symbols/symbol.go`):

```bash
printf '%%load /tmp/m.sysml\n%%search Pkg::\n%%quit\n' | timeout 30 ./bin/sysml -quiet
```

- **Always search a `Pkg::` prefix, not a bare word.** `%search Observe` matched ~20 stdlib symbols
  (ISQLight, VerificationCases) and truncated with `(9 more; narrow the search)`, burying the
  model's own symbols. The trailing `::` also drops the package row itself.
- `%list` only echoes the submitted source text and `renderMember` never reaches action-body
  parameters, so it cannot show a parameter's kind. There is no `%kind`/`%hover` in the REPL.
- **Anonymous usages are invisible to `%search`** (no name to index). To show that
  `individual part : Vehicle;` or `in snapshot ;` still built something, use
  `./bin/sysml <m> -convert sysml -o /dev/stdout` and assert the member round-trips
  (`in snapshot;`, `in event;`, `in;`).
- Warning-only changes need the **exit code** asserted explicitly: `-validate` prints warnings and
  still ends `✓ no errors` with exit 0, so `echo "exit=$?"` is the difference between "warning" and
  "error" in the recording. Count them with `-validate f 2>&1 | grep -c 'reserved keyword'`.
- The A/B main binary is essential here: on `main` the same fixture printed
  `attributeUsage`/`partUsage` for `individual i : V` and `in snapshot atStart : Flight`, which is
  the only visible difference between fixed and broken.

Pre-existing traps worth not re-reporting as regressions:

- A bare `event e : O;` **declaration** reports `error: unresolved reference: e — did you mean
  Demo::e?` on every revision — `event <name>` is read as naming an existing occurrence. Use
  `occurrence takeoff : Flight; event takeoff;` for a clean fixture; the symbol is still built as
  `occurrenceUsage`.
- Fixtures using `Real`/`Integer` need `private import ScalarValues::*;` or `-validate` fails with
  `unresolved reference: Real` and masks the kind assertions.
- **Run the suite with a cold library index when you change classification.** The on-disk index
  (`$XDG_CACHE_HOME/sysml-ls/libs`) caches stdlib symbol kinds, so a warm cache keeps returning the
  old kinds and `go test ./...` passes locally while CI fails. Gate with
  `export XDG_CACHE_HOME=$(mktemp -d) && go test -count=1 ./...`; this is how PR #210's
  `function` → `kermlType` regression (every `IntegerFunctions` operator stopped being a calc,
  `internal/repl/discover_test.go` pinned `ScalarValues::Integer attributeDef`) stayed hidden.

## Multiplicity of a feature's default value (PR #199, Track A / A2)

A default bound to a feature is checked against the feature's multiplicity in **two independent
tiers**, and testing one proves nothing about the other:

- **Static (type tier)**: `sysml <m> -validate` reports counts it can see in the source, from
  `passes.checkValueCount` over `exactCount(u.Value)`. Wording:
  `N value(s) bound to a feature with multiplicity lower|upper bound M`, exit **2** with
  `did not analyse cleanly; no check was made`.
- **Runtime**: `Context.checkDefaultCount` fires only when the slot is **materialized**, i.e. when
  something reads it. Wording adds a prefix:
  `slot <Type>.<name>: multiplicity violation: N value(s) bound to …`.

Consequences worth checking on every change here:

- `%features <instance>` renders a bad slot inline as `name: <error: slot …>` and the REPL still
  **exits 0** — a violation is not a session error. For an exit status you need a run that reads the
  slot, e.g. `sysml m.sysml -instantiate test::bad -eval 'test::bad.few'` → exit **2** with
  `evaluation failed: slot bad.few: multiplicity violation: …`.
- `sysml m.sysml -instantiate X -validate` prints `no errors` and exits **0** even when every
  default in the model violates its multiplicity, because `-instantiate` creates the object without
  materializing its slots. Do not use it as the "does the runtime check fire" probe.
- **The two tiers can disagree, and a collection holding a reference is where they do.** `exactCount`
  flattens nested collection literals just as binding does, so
  `attribute nested : Real[4] = ((1.0, 2.0), (3.0, 4.0));` counts 4 statically and materializes as
  `[1.00, 2.00, 3.00, 4.00]` with no diagnostic; but a collection whose elements are references has
  no statically known count, so `attribute two : Real[2] = (src, src);` (with `src : Real[3]`)
  passes statically and errors at runtime with "6 value(s) … upper bound 2". Include both shapes in
  any regression fixture.
- A feature that declares **no** multiplicity is deliberately not held to the assumed `1..1`
  (`EffectiveFeature.MultiplicityStated`), so `attribute anyN = both.volume;` over a `[0..*]`
  sibling must show `[2.00, 3.50]` with no diagnostic. A test asserting an error there is wrong.
- Multiplicity is inherited through `:>>` when the redefinition does not restate it, in both tiers:
  `part def Base { attribute xs : Real[2]; }` + `attribute :>> xs = (1.0, 2.0, 3.0);` must
  `-validate` as an error (exit 2) — this is the cheapest static probe of that path, and it is
  clean on any build predating the fix.

Fixtures live in `internal/core/runtime/testdata/conformance/multiplicity_default_*.sysml`
(merged / composite / nonconforming / redefinition). Drive them over a pipe to discover values:

```bash
printf '%%load internal/core/runtime/testdata/conformance/multiplicity_default_merged.sysml\n%%instantiate test::ranges\n%%features test::ranges\n%%quit\n' | timeout 60 ./bin/sysml
```

Expected: `exact = [1.00, 2.00, 3.00]`, `star = [1.00, 2.00]`, `empty = []`, `plus = [5.00]`,
`masses = [1.00 [kg], 2.00 [kg]]` (units survive), and in the composite fixture
`stowed = [Instance(ID: 2), Instance(ID: 3)]` reusing **left/right's own IDs** rather than fresh
ones — comparing IDs is the only way to tell "held the objects the default names" from
"instantiated two new ones".

Two gotchas when recording this area in Konsole: the REPL does **not** expand shell variables, so
`%load $CONF/x.sysml` fails with a path error — type full relative paths; and `part derived : …`
draws a `"derived" is a reserved keyword` warning that is unrelated to the defaults under test.

## String values and StringFunctions (PR #211)

A model that declares `String`/`Integer`/`Boolean` attributes needs `import ScalarValues::*;` —
without it every type annotation reports `unresolved reference: String — did you mean
ScalarValues::String?` and `-validate` exits 2 before any string behavior runs, which looks like a
feature failure but is a fixture bug. Add `import StringFunctions::*;` to call `Length`/`Substring`
unqualified inside a model (the REPL's `-e` mode resolves those two unqualified with no import,
because they are aliased in `library_functions.go`'s unqualified map; the other StringFunctions
names always need the `StringFunctions::` prefix).

Discriminators that separate a working string runtime from a broken one:

- **Characters, not bytes.** `Length("héllo")` is 5 (6 bytes), `Length("日本語")` is 3 (9 bytes),
  `Substring("héllo", 2, 3)` is `"él"`. A byte-based implementation answers 6/9 or returns mojibake,
  so always use a non-ASCII fixture — an ASCII-only test passes either way.
- **Substring is 1-based and inclusive**, with `lower > upper` returning `""` rather than erroring,
  while `lower < 1` or `upper > Length` gives `sequence index out of range: function
  StringFunctions::Substring lower 0 is outside 1..5` and exit 2.
- **The `==` split is deliberate**: `"a" == 3` evaluates to `false` (it specializes
  `DataFunctions::'=='`), while the explicit `StringFunctions::'=='("a", 3)` is a type mismatch.
  Test both; only one of them being right is the likely regression.
- A string against an Integer/quantity/sequence must say
  `type mismatch: operator '<' is not defined for a string and a quantity` (naming both types).

### Driving these cases through a GUI terminal

- **xdotool cannot type CJK into Konsole** — `type` with `日本語` silently delivers nothing, so
  `Length("日本語")` arrives as `Length("")` and answers 0, which reads as a failure in the video.
  Latin-1 accents (`héllo`, `naïve`) do type fine. Put any CJK (and any quoted-operator name such as
  `StringFunctions::'=='`, which shell quoting mangles when typed inline) into a small
  `/tmp/*.sh` written from a tool shell, then `cat` it and `bash` it on camera — the `cat` shows the
  reviewer exactly what ran.
- At the `sysml>` prompt a bare expression like `("a","b")->includes("b")` is parsed as a
  declaration (`1:1: error: expected a namespace member`). Prefix expressions with `%eval`.
- `%features <PartDefName>` (not the part usage path) is what follows `%instantiate <PartDefName>`;
  `%features Msg::g` reports `no instance of "Msg::g"`.
- **Escapes are stored raw**, so `Length("a\"b")` is 4 and the value renders as `"a\\\"b"`. This is
  pre-existing lexer behavior (identical on the parent commit) — verify against a contrast binary
  before reporting it as a string-runtime defect.
- A 512 KiB string built by doubling inside a `for` loop (`acc = acc + acc` ×16, then
  `Length(acc)` → 524288) completes in seconds; use it as the cheap no-hang/no-quadratic check.
  Note that a calc whose body assigns to a `return acc` feature returns no value when *invoked from
  another calc's expression* (`calculation returned no value`), so compute the length inside the
  same body rather than wrapping the loop calc in another one.

## Runtime budgets, and `calc` recursion in particular (PR #198)

Every budget in `internal/core/runtime/budget.go` is reachable from the CLI as an env var and is
listed by `%budget` in the REPL — that meta-command is the cheapest proof a new bound was wired
into `internal/repl/meta.go`. `OPENSYSML_MAX_CALC_DEPTH` (default 10000) bounds nested `calc`
invocations, i.e. recursion depth, and is the only budget with a **ceiling** (25000).

Four distinct surfaces to assert for any budget change, since they fail independently:

1. **Under the bound the run must succeed with the right number**, not merely "not error".
   `%calc P::sumTo 5000` → `= 12502500`. Deep-but-terminating recursion is fast (~0.3 s), so a
   long pause is itself a signal.
2. **Over the bound: a typed error, promptly, with a sane exit status.** Both
   `%calc P::spin 1` in the REPL and `./bin/sysml m.sysml -eval 'P::spin(1)'` (exit **2**, wall
   time well under a second). Wrap in `timeout` so a hang shows as exit 124, and read the output
   for `fatal error: stack overflow` — a Go runtime crash rather than a reported error is the
   failure mode the budget exists to prevent. Follow the REPL error with `%eval 1 + 1` → `= 2` to
   show the session survived.
3. **A low value must refuse input that otherwise evaluates** (`OPENSYSML_MAX_CALC_DEPTH=50` on a
   200-deep recursion). Without this contrast the env var could be ignored entirely and the test
   would still look green.
4. **Invalid/out-of-range values are refused at startup**, before any model work, naming the
   variable: `=abc` → `is not an integer … (default 10000)`; `=30000` → `must be at most 25000 …`.
   Also assert the ceiling value itself (`=25000`) is *accepted* — an off-by-one in the comparison
   is otherwise invisible. Both refusals exit 2 and print no evaluation output.

Budget interactions matter: under `OPENSYSML_MAX_STEPS=100` a runaway recursion must report the *step*
limit, not the depth limit. Whichever budget is spent first wins, so a change that reorders the
checks shows up here and nowhere else.

### Fixture notes for recursive `calc` models

- Bare `Integer`/`Boolean` do **not** resolve in a hand-written file (the conformance harness
  supplies the imports). Start the package with `private import ScalarValues::*;` or every line
  errors with `unresolved reference: Integer — did you mean ScalarValues::Integer?`.
- A recursive calc is just `calc f { in n : Integer; return : Integer = if n <= 1 ? 1 else n * f(n - 1); }`.
  An **implicit-result** body drops `return :` and ends in a bare expression:
  `calc f { in n : Integer; if n <= 1 ? 1 else n * f(n - 1) }`. Both must be exercised — they take
  different paths in `passes/typecheck.go` (`checkBehaviorMember`) and only the explicit one was
  type-checked before #198.
- Adversarial shapes worth having in one file: a non-recursive calc *usage* nested inside a
  recursive calc; recursion through a calc named like a library function (`max`); mutual recursion
  (`isEven`/`isOdd`, whose result for an odd argument is `false`).
- **There is no `%check` command.** To evaluate an `assert constraint` whose body calls a recursive
  calc, use `%instantiate <DefName>` then `%features <DefName>` — note both take the **def** name, not
  the usage (`%features P::w` answers `no instance of "P::w"`). A satisfied constraint renders
  `deepOK: <constraint: satisfied>`; a runaway one renders per-slot as
  `spinny: <constraint: not evaluated: … calc recursion limit exceeded …>` and the session survives.

### Type diagnostics on implicit-result bodies

`-validate` is the surface: an implicit result is typed like any expression, so
`calc def C { in n : Integer; n + "one" }` must report
`error: operator '+' is not defined for Integer and String` (exit 2) and an incommensurable pair
(`ISQ::LengthValue` + `ISQ::DurationValue`) must report
`warning: operator '+' combines incommensurable quantities: … (dimension L) and … (dimension T)`
(exit 0). Always pair these with a **clean** implicit-result body (`n + 1` → `no errors`) — a pass
that over-reports would look identical otherwise. Because the pre-fix binary printed `no errors`
for all three, the contrast binary from the parent commit is what makes this evidence conclusive.

## opensysml service ownership and the require-service gate (PR #204)

Ownership is the claim worth testing by hand, because pytest can pass while the invariant is
broken. Three probes:

```bash
PY=~/opensysml-venv/bin/python          # ls -d /home/ubuntu/*venv* if it is missing
cp bin/sysml-grpc ~/.opensysml/bin/     # what CI does; otherwise ensure_binary downloads
```

- **A service you started:** run `bin/sysml-grpc -port 0 -report-address` from the shell and reach
  it by name (`Connection(host, port)`, `OPENSYSML_SERVICE=host:port`, `auto_start=False`),
  including a connection left open at exit. The shell's pid must still be alive afterwards, and
  `~/.opensysml` must hold nothing but the binary cache.
- **A private child:** `opensysml.connect()` starts one on a kernel-assigned port; assert the port
  is neither 50051 nor 0 and that a second `connect()` reuses the same pid. Two connections must
  keep it alive when the first closes, and the last close must stop it.
- **No orphans:** `kill -9` the interpreter holding a private child and assert the child is gone
  within a second or two, then repeat with `os._exit(1)` and with a `fork()`ed child exiting. The
  mechanism is the pipe on the child's stdin, so `pgrep -x sysml-grpc` is the whole probe.
- Use `conn.load(path)` (not `load_model`) for a real RPC that proves the connection talked to the
  service rather than failing early.

Suite counts as PR #204 merged (`cd python && $PY -m pytest tests/ -q`): **413 passed / 13 skipped**
with no service; **423 passed / 3 skipped** with a service on 50051 and
`OPENSYSML_REQUIRE_SERVICE=1`, the
3 remaining skips being mypy-not-installed and a manual-binary-cache case, never a service skip.
With `OPENSYSML_REQUIRE_SERVICE=1` and no service, collection must **error** (exit 2,
"none answers on localhost:50051"), never skip. A whole run must leave an operator-started service
on 50051 with the same pid, and must leave no `sysml-grpc` of its own behind.

## Orthogonal regions and cross-region transitions (`internal/core/runtime/state_region_transition.go`)

A transition whose source and target sit in different orthogonal regions of the same composite
state must exit only its source side, not the enclosing composite. The REPL surfaces needed to
see that at all:

- **`%trace on` is the only surface that shows exit/entry order.** `%current` shows the resulting
  configuration; only the trace distinguishes "exited the source" from "exited and re-entered the
  whole composite". Turn it on *before* the `%advance` that fires the transition, and assert on the
  absence of lines too (`no exit: <composite>`, no second `enter: <composite>`).
- **`%current` prints one active state per active region**, joined by ` | `, and each name is the
  state the region itself **declares** — a composite (`wrapper`), not the nested target inside it.
  So `wrapper | rtarget` is the correct shape for a target nested one level deep, and a **repeated
  name (`rtarget | rtarget`) is a bug signal**, not a rendering quirk: it means an outer region
  recorded a state it does not declare. Do not dismiss duplicates as legitimate.
- An emptied source region simply disappears from the list, so "the source region has no active
  state" is asserted by its absence.
- **Counter attributes, not state names, are the discriminator.** Give every state on the path an
  `entry`/`exit` action incrementing its own counter (`wrapperExits`, `runningEntries`, …) and read
  them from `%current`'s "State data". Exactly-once claims (`exit ran once`, `composite entered
  once`) are unprovable from the configuration alone, and double-exit bugs are invisible without
  them.
- **Always build a contrast binary and run the same model through both** (recipe earlier in this
  file). Cross-region models are easy to write so that they pass on the broken build too; the
  contrast run is what proves the model is adversarial. **Pick the contrast commit by where the
  defect lives:** the parent commit for a regression introduced by the PR, but `origin/main` when the
  bug is pre-existing — a main-based contrast additionally proves the PR fixes a shipped defect
  rather than one of its own making. Useful discriminating values seen historically: a pre-fix build
  re-enters the composite forever and dies at the event budget; a build with the wrong recorded
  active state prints a duplicated name; a build missing the widened concurrency test leaves the
  target region's old state running (`<state>Exits = 0`) and prints it alongside the target
  (`lstate | mid`); a build with the old `leaveRegion` clearing rule runs the same exit actions twice
  (`midExits = 2`, `wrapperExits = 2`).

Shapes worth covering, in increasing order of subtlety — each has broken independently:

1. plain sibling-region target (`state_transition_cross_region.sysml`);
2. a third sibling region that must keep its state (`..._third_region.sysml`);
3. target nested one and two levels under a composite the target region is **already running**
   (`..._nested_target.sysml`) — the abandoned inner state must exit;
4. target nested under a composite that is **not** active there (`..._inactive_wrapper.sysml`);
5. target region running a **different** composite, whose nested regions must be torn down;
6. **source nested deeper than the target's region** (`..._deep_source.sysml`) — the source side
   must be exited up to the shared region-set level (`exit: ideep` → `exit: wrapper`) while the
   composite is untouched;
7. outward exit past the composite, and a nested non-orthogonal move that must still stop at the
   LCA (regressions for the `leaveRegion` / `getLCA` paths);
8. **a region whose owning state is a plain substate** — the single highest-value shape, because it
   broke the dispatch walk and the exit walk independently, in consecutive commits. Write it as
   `region right { entry; then rs; state rs; state wrapper { state mid { region inner { … state ideep … } } }
   then rs mid; }`: `mid` is declared directly in `wrapper`'s body, so it is **not** in
   `graph.RegionOf`, and `then rs mid;` is what makes the inner region active. Any outward walk
   written as `RegionOf[RegionOwner[…]]` returns nil here and silently takes a wrong branch, and any
   "clear the region only when its entry is exactly this state" rule misses that `regionStates[right]`
   is `mid`, a *descendant* of the `wrapper` being exited. Cover both directions from `ideep`: a
   cross-region target (sibling region keeps a stale state → `lstate | mid`, `lidleExits = 0`) and a
   target **outside** the whole composite (`mid`/`wrapper` exit actions run twice). The shipped
   fixtures are `..._cross_region_substate_owner.sysml` and
   `state_transition_leave_composite_substate_region.sysml`;
9. ping-pong: two cross-region transitions targeting each other must stop with the typed
   `Stopped at the event budget (N events; raise OPENSYSML_MAX_EVENTS to allow more)`, never hang or
   panic. Run the REPL as `OPENSYSML_MAX_EVENTS=2000 ./bin/sysml` so the stop takes a second, and
   follow it with `%eval 1+1` → `= 2` to show the session survived.

**History interacts with region bookkeeping — always test it, with a control model.** A `deep
history resume;` vertex inside a composite with orthogonal regions is the natural victim of any
change to how a region records its active state: at ca7f3a89 the restore entered the region's
*initial pseudostate* instead of the recorded composite, so the inner region restarted and the deep
state was lost. To attribute such a failure correctly, write a control model with **no cross-region
transition at all** (region reaches a nested state, machine leaves the composite, history resumes)
and run it on both binaries — that separates "the cross-region path broke history" from "history is
broken for any composite-in-a-region". Shallow history legitimately restarts the inner region, so
only the deep variant discriminates. Read the restore in `%trace on`: `enter: <region initial
pseudostate>` in place of `enter: <recorded state>` is the signature. Cover the **plain
non-orthogonal** nesting too (one region, `first -> second`, `second::inner` running `a -> b`): it
broke separately from the orthogonal case and its signature is an *inconsistent* `%current` such as
`first | b`, where the outer region records a state whose inner region holds a descendant of a
different one. Any change to `leaveRegion`/`exitState` bookkeeping can move history even when it
looks like a pure exit-ordering fix, so re-run `restarts = 1` (shipped
`state_deep_history_region_composite.sysml`) plus a control model whose restored configuration is
visible (`lidle | wrapper | deepState`) as part of every such pass.

**Write history/nesting models with an explicit `region <name> { … }` wrapper.** Substates declared
directly in a state body (`state outer { state first; state second; }`) did not start in a history
model — the machine stalled at `outer` and no transition fired — so a model written that way looks
like a runtime failure when it is really an authoring shape the engine does not drive. Declare
`state outer { region only { entry; then os; state os; state first; … then os first; } }` instead. Note the
exception exercised deliberately by shape 8 above: a substate *owning* a region
(`state mid { region inner { … } }`) does run, provided the enclosing region's initial transition
names it (`then rs mid;`).

### Engine limitations that constrain how these models can be written

All reproduce on several commits (not caused by any one PR), but they silently make a test vacuous:

- **A transition whose source is a composite state never fires.** Put the "later trigger" that
  leaves a composite on a nested **leaf** state instead; it exercises the same `leaveRegion` path.
- **A timer declared directly on a composite state entered by a cross-region move is never
  scheduled.** Use `%events` to confirm what is actually queued before trusting a `%advance`; leaf
  timers are scheduled correctly.
- **A guard-only transition is not re-evaluated for regions other than the one where the event
  occurred**, so "a timer in region C flips a flag that fires a guarded transition in region A"
  never happens. Drive each region with its own timed transition.
- Stale-reaction probes are still the best evidence that an abandoned state is really gone: give it
  `accept after N then boom` where `boom` sets a counter, advance past N, and assert the counter is
  still 0 — a state left running in a region reacts and flips it.

Practical notes for a recorded pass: shipped conformance fixtures under
`internal/core/runtime/testdata/conformance/` emit harmless
`unresolved reference: Integer — did you mean ScalarValues::Integer?` diagnostics (they omit the
`ScalarValues::*` import); the runtime still executes and the `.expected.json` next to each fixture
is the cheapest source of expected counter values. `/tmp` is wiped between sessions, so adversarial
models and contrast binaries must be recreated each run — and when you recreate a model, **re-derive
its state-def name from the file** (`grep -n 'state def' /tmp/…/model.sysml`) instead of trusting a
name written in a previous run's plan: a rebuilt model can end up with a different name, and
`%state Adv::WrongName` just reports `unresolved reference`, which is easy to misread as a runtime
bug mid-recording. Typing `clear` at the `sysml>` prompt is parsed as SysML and errors — `%quit`
first, then clear at the shell (this also rules out `clear; %load …` as a one-liner).

`clients/python/scripts/pin_release_checksums.py --check` hits the GitHub releases API for every pinned
asset and dies with `HTTP Error 403: rate limit exceeded` once the unauthenticated budget is spent;
it reads `$GITHUB_TOKEN`. Set it without putting the token on camera:
`read -rs GITHUB_TOKEN; export GITHUB_TOKEN`. Careful with `--version <tag> --write`: it edits
`clients/python/opensysml/binary.py`, so `git checkout clients/python/opensysml/binary.py` afterwards. For the
"release publishes no assets" refusal use an old tag (`v0.0.4`) — v0.0.5..v0.0.8 all publish
binaries now. The unpinned-download refusal is testable offline-ish with
`HOME=/tmp/fakehome $PY -c "...ensure_binary(version='v9.9.9')"`, which keeps the real
`~/.opensysml/bin` cache intact; the opt-in out of it is per repository
(`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>`, or `=1` for any).

## Quantities on the wire: `Value.quantity` and the Python `Quantity` (PR #200)

Once the service can marshal `runtime.ValQuantity`, a quantity feature no longer reads as
`FeatureValueError: feature value 'm': unsupported: quantity value` but as
`opensysml.values.Quantity`. The
highest-value evidence is the **parent-commit contrast** (build `/tmp/old-sysml-grpc` from the
commit before the change, swap it into `~/.opensysml/bin/sysml-grpc`, clear
`~/.opensysml/sysml-grpc.{pid,refcount}`, run the same script): the old service raises the error
while a plain `ScalarValues::Real` feature still reads `2.0`, so the frame proves the delta.

What to assert on the decoded value, and why each one distinguishes working from broken:

- **The magnitude must stay in the unit written.** `5.4 [SI::km/SI::h]` must decode as magnitude
  `5.4` with `unit.text == "SI::km/SI::h"` and `scale_num/scale_den == 5.0/18.0`; a reduction done
  behind the caller's back shows up as `1.5`. `in_unit(Unit(text="m/s", factors=(m^1, s^-1)))`
  must be **exactly** `1.5` — a float-sloppy conversion gives `1.4999999999999998`.
- **Integer and Real stay apart:** `3 [SI::m]` → `isinstance(magnitude, int)`.
- Base units are reported by **FQN of the base unit**, not the written unit: `SI::kg` reduces to
  `1000/1·SI::gram`, `SI::W` to `1000·SI::gram·SI::metre^2·SI::second^-3`. Assert
  `unit.exponents()`, which is order-independent.
- A **dimensionless** ratio (`3.0 [SI::m] / 1.5 [SI::m]`) does not arrive as a quantity at all — the
  runtime reduces it to a plain `float`. Don't write a test that requires a `Quantity` there.
- An exponent survives: `(2.0 [SI::m])**2` → magnitude `4.0`, `unit.text == "(SI::m)**2"`,
  `SI::metre` exponent `2.0`.
- Quantities also come back from `conn.eval("5.4 [SI::km/SI::h]", hash, context_symbol_id=pkg)`,
  from a nested child (`inst.engine.power`), inside sequences (`LengthValue[3]` → `list[Quantity]`),
  and on a `verify_constraint` verdict — filter `verdict.instances` on `verdict.instance_id` and
  read the feature value off that instance.
- Comparison semantics worth pinning: `1 [SI::km] == 1000.0 [SI::m]` is True and the two hash
  alike; ordering/adding incommensurable units raises `IncommensurableUnitsError` naming both
  reductions (`cannot order a quantity in [SI::km] and one in [SI::kg]: … 1000·SI::metre …
  1000·SI::gram`); ordering against a bare `float` raises `TypeError`, while `q == 5.0` is just
  `False`.

**Inbound (client → service) is a different code path.** `Connection._python_to_value` has no
`Quantity` arm, so a quantity argument cannot be sent through `conn.calc(...)` — build the request
proto by hand and send it on the client's stub (still the real service over gRPC):

```python
q = sysml_pb2.Quantity(real_magnitude=2.0, unit="SI::m",
    unit_term=sysml_pb2.UnitTerm(scale_num=1.0, scale_den=1.0,
        factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0)]))
c._stub.EvaluateCalc(sysml_pb2.EvaluateCalcRequest(model_hash=m.hash,
    symbol_id="q2::Double", arguments=[sysml_pb2.Value(quantity=q)]))
```

Only the paths that decode with the model's `symbols.Index` work. `EvaluateCalc` does, and returns
typed errors worth asserting verbatim (`calc argument could not be read: unknown base unit:
SI::nope` / `unit scale is not a usable ratio: 1/0` / `quantity in "SI::m" carries no magnitude`,
all with `FAILURE_REASON_EVALUATION`, and the service stays alive). `ExecuteAction` (in
`internal/grpc/service.go`) decodes its `inputs` map the same index-aware way, so a quantity input
binds (`ExecuteActionRequest(inputs={"mass": Value(quantity=q)})` on an action whose `in mass :
ISQ::MassValue` body does `assign heavier := mass + 1.0 [SI::kg]` returns `6 [SI::kg]`), and a
malformed one comes back as `ExecuteActionResponse.Error` = `input "<name>" could not be read:
<err>` with **zero** outputs rather than a bound `ValInvalid`. If you ever see outputs coming back
`null: "unsupported"` for a quantity input, that call site has regressed to a context-free decoder.
Worth checking cheaply: a commensurable-but-different unit (`2000 [SI::g]`), an un-normalized
reduction (`SI::gram^2 · SI::gram^-1`, which the service re-normalizes), and a bad quantity nested
inside a `ValueSequence` input (the error still names the top-level input). Traps: the field is
`action_symbol_id`, not `symbol_id`, and an action **usage** (`action showIt : Show;`) reports
`no initial node found`, so execute the **def** that carries the body.

### Generated typed classes and mypy

`opensysml-generate <model> -o out.py` (or `python -m opensysml.generate`) types a quantity property
`-> _t.Quantity` with `_t.feature_value(self, "x", _t.as_quantity)`. Only features with a **declared** quantity
type get it: an untyped derived attribute (`attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];`)
has no type facts and still generates `-> object` / `_t.as_object`, even though the runtime value is
a `Quantity`. Don't read that as a bug in the quantity typing.

To make mypy actually enforce it, **set `MYPYPATH` to the repo's `clients/python/` directory** — without it
mypy cannot resolve the editable-installed `opensysml`, silently treats `_t.Quantity` as `Any` and
reports *no* errors on obvious misuse (a false pass that looks like a passing test):

```bash
cd /tmp/qw && MYPYPATH=/home/ubuntu/repos/OpenSysML/python \
  $HOME/pv/bin/python -m mypy --no-incremental --no-error-summary --follow-imports=silent misuse.py
# -> Unsupported operand types for + ("Quantity" and "float")  [operator]
# -> Incompatible types in assignment (expression has type "Quantity", variable has type "float")
```

`mypy` must be present in the same venv as `opensysml` ($HOME/pv here); otherwise the typed-codegen
tests skip rather than fail.

## The evaluate/eval split and generated-base planning (PR #218)

- **Since the rename, `opensysml.evaluate` is the real module-level evaluator and `opensysml.eval` is a
  forwarder that emits `DeprecationWarning` and is out of `opensysml.__all__`.** Test both sides:
  `evaluate(...)` must produce **zero** DeprecationWarnings (catch them with
  `warnings.catch_warnings(record=True)` + `simplefilter("always")`), `eval(...)` must return the
  identical value and warn exactly once with a message naming `opensysml.evaluate`, and
  `from opensysml import *` must bind `evaluate` but not `eval`. `subject` is the **last** parameter of
  `evaluate` precisely so a pre-rename positional call
  `eval(expr, None, hash, None, host, port)` still binds host/port — prove that argument really is the
  host by also calling it with a bogus address (`"203.0.113.9", 59999`) and requiring a
  `ConnectionError`; that call takes ~30-60s to time out, so give the runner a generous timeout.
- **`opensysml.errors.RuntimeError` is a warn-on-access alias of `ExecutionError`** served by the module
  `__getattr__` and absent from `errors.__all__`. Check it by *catching a real failure* with it
  (`except opensysml.errors.RuntimeError` around a cyclic-slot eval), not just by identity.
- **`generate.py` elides a base that another declared base already specializes, silently and by design**
  (`_without_implied`): `part def Backwards :> Vehicle, Hybrid` where `Hybrid :> Vehicle` emits
  `class Backwards(Hybrid):` with **no** comment, because `Vehicle` is still in the MRO. Only a base the
  class genuinely does not inherit gets `# … left out: Python cannot linearize it with the bases above`.
  To exercise the comment path you need a real C3 conflict, e.g. `X :> A, B`, `Y :> B, A`, `M :> X, Y`
  → `class M(X):` plus the `left out` note. Asserting the comment on a merely redundant base is a
  false negative.

## State-machine transition endpoint checking (PR #225 class)

The check that a transition names a vertex of *its own* machine (and that a routing pseudostate is
left by something) is a **name-resolution-tier pass**, so it is fully observable from
`./bin/sysml -validate <f>` — no execution needed. Codes: `endpoint-not-of-machine`,
`no-outgoing-transition`; messages are
`transition endpoint <as written> names a <state|pseudostate|start marker|end marker|element> that
is not a vertex of this state machine` and
`<kind> <name> has no outgoing transition, so a transition reaching it terminates nowhere`.

Because the pre-fix binary reports **nothing** for these models, the parent-commit contrast binary
(see the worktree recipe above) is the strongest single frame: old = `✓ no errors` exit 0,
new = error + `did not analyse cleanly` exit 2 on identical input.

Fixture shapes that actually discriminate (all one-liners; `state def` and `state` usage both work):

- Illegal, target of another machine: two `state def`s in one package, `transition first busy then
  Other::running;` → reported at the endpoint's column.
- Illegal, marker as target: `first begin then off;` plus `transition first on then begin;` → the
  message says **"start marker"**, which is how you tell `VertexKind` is being consulted.
- Illegal, dangling routing pseudostate: `junction route;` + `transition first busy then route;` and no
  transition out → reported at the `junction` declaration, not at the transition.

False-positive traps to always include as *legal* rows, since each exercises a different branch:

- A transition into a **sibling orthogonal region** (`transition first lidle then rtarget;` across
  `region left` / `region right`) — legal per UML §14.2.3.9.
  `internal/core/runtime/testdata/conformance/state_transition_sibling_region.sysml` is the shipped
  one; run it with `%state TransitionSiblingRegion` + `%advance 1` → `Current state: lidle | rtarget`
  and `crossed = 1`.
- `entry point into;` / `exit point outOf;` as endpoints (`state_entry_exit_points.sysml`).
- A sourceless `accept after 5 then <s>;` written after a state (the source is the state declared before it).
- `first start then off;` with no `initial`.
- A junction left by a **succession** (`route then finishedUp;`) rather than a `transition`, and a
  `fork`/`join` reached by one — the pass tracks succession sources *by name*, so a regression here
  shows up as a bogus `no-outgoing-transition`.
- Two states with the **same name** in sibling regions (both `idle`) — must stay clean.

Cheap whole-repo false-positive sweep (~40 s, 83 models at 11d0ed72, expect `differing: 0`):

```bash
for f in $(grep -rl 'state def\|state .*{' --include=*.sysml examples \
             internal/core/runtime/testdata/conformance testdata | sort); do
  a=$(./bin/sysml -validate "$f" 2>&1; echo $?); b=$(/tmp/old-sysml -validate "$f" 2>&1; echo $?)
  [ "$a" != "$b" ] && echo "DIFFERS: $f"
done
```

Error-timing contract worth re-checking whenever this area moves: a structurally empty
`state def Empty { }` must `-validate` **clean** (exit 0) and only fail at
`-state Empty` with `failed to create executor: initialize state machine: no initial state found in
state machine Empty`. A check-time complaint there would be the regression.

Two message hygiene probes: grep the diagnostics for `ast\.|StateNode|%!` (must be zero hits — the
messages come from `lower.NotAVertexFormat`/`VertexKind`, not `%T`), and endpoints written as deep
qualified names (`Outer::Inner::Deep::busy`) must be echoed **in full** in the message.

Also note `done` is an inherited feature of `States::StateAction`, so naming a state `done` in your
own fixture draws `name conflict: done is already the name of the inherited feature …` plus a
reserved-keyword warning — pick `finished`/`finishedUp` instead, or the legal row fails for an
unrelated reason.

### Proving a succession or a pseudostate edge really fires (not just validates)

`-validate` cannot tell you whether lowering silently *dropped* an edge — a dropped succession looks
identical to a clean model. Make the edge observable instead: give the destination state an
`entry { flag = 1; }` and read both the active state and the attribute back:

```bash
printf '%%state <Def>\n%%advance 5\n%%current\n%%quit\n' | timeout 60 ./bin/sysml <model>.sysml
```

`%current` prints `Current state: …` plus a `State data:` block with the attribute values, so
"reached the state" and "ran its entry action" are separate evidence. Compare against a binary built
from the *previous revision of the same branch* (not the PR base) to attribute the delta to one
change. Useful discriminators in this area:

- `x then <junction>; <junction> then <s>;` — an implementation that resolves succession endpoints by
  simple *state* name drops the pseudostate hop entirely and the machine just stalls one state early.
- Same-named `junction route;` in two sibling orthogonal regions, each routing to its own region's
  end state with its own flag — if pseudostates are keyed by simple name, one overwrites the other
  and *neither* flag gets set. Run it ~5 times; the ordering must be identical every run.
- A regression row worth keeping: a succession to a **qualified nested** target
  (`gate then P::Def::outer::r::deep;`) usually works on both old and new revisions, so it proves the
  rewrite lost nothing rather than proving anything new.

Two traps when writing these fixtures:

- An unqualified target in another region resolves by ordinary name resolution first, so
  `lidle then rtarget;` fails with `unresolved reference: rtarget — did you mean A::B::right::rtarget?`
  on *every* revision. That is not a false positive; write the qualified name.
- A succession carries **no guard**, so a cross-region succession re-enters the composite state, the
  source region restarts, and the edge re-fires forever: the run ends on `Stopped at the event budget
  (1000000 events; raise OPENSYSML_MAX_EVENTS to allow more)`. That is a clean bounded stop, not a hang —
  the state/attribute evidence is still valid. Use the guarded `transition first a if flag == 0 then b;` form
  when you want the run to settle instead.
- Running a shipped conformance model straight from the CLI can report `unresolved reference: Integer`
  because the conformance harness supplies imports the file omits; add `import ScalarValues::*;` in
  your own fixtures, and treat that one as pre-existing (the sweep confirms it is identical on both
  binaries).

## Hierarchical / composite state machines: outward transitions, timers, history (PR #230)

Whole families of state-executor changes (transitions declared on a composite state while a
substate is active, exit-chain ordering, timer withdrawal, change triggers) are testable from the
REPL with **no ports and no signals** if you encode observations as *decimal digits* in one
attribute. This is the single most useful technique in this area:

```
attribute log : Integer = 0;
state Outer { entry assign log := log * 10 + 1; exit assign log := log * 10 + 2; }
```

`%current` then prints e.g. `log = 32419`, which reads left-to-right as the exact order of the
entries/exits/effects that ran. Give **every nesting level its own digit** — a duplicated exit
(`…22…`) or a missing one is otherwise invisible, and a doubled exit is exactly the defect class
that shows up when exit chains are refactored. Keep the digits distinct per level and put the
transition effect last.

Driving and reading:

- Put fixtures under `$HOME` (e.g. `~/fixtures/pr<N>/`), never `/tmp` — `/tmp` is wiped between
  sessions and you will lose an afternoon of fixtures. `%load` needs an **absolute** path.
- A nested state reference must be **qualified**: `start then Working::Step1`, not `then Step1`.
- A nested entry succession only takes effect inside an explicit `region { … }`. `state Outer {
  entry; then o; state o; state Mid { … } transition first o then Mid; }` enters **Outer only** —
  `ActiveStates()`/`%current` show
  no substate at all, and any test built on that shape silently probes the wrong thing. Wrap both
  levels in regions and assert the pre-condition configuration before the interesting step.
- `ActiveStates()` for a region-wrapped chain lists the *innermost* composites and leaves
  (`map[Mid:true leaf:true]`), not every enclosing composite — assert on the leaf plus the state
  you expect to survive, not on the outermost name.
- In Konsole use `%quit`; `Ctrl-D` closes the terminal window and kills the recording. `ctrl+plus`
  (not `ctrl+shift+plus`) changes the font size.
- Batch a whole scenario into a `.script` file and pipe it (`timeout 30 ./bin/sysml < z.script`) to
  discover values, then re-run the interesting ones interactively for the camera.
- A per-fixture expectation table plus a tiny runner that prints `fixture | final state | log |
  PASS/FAIL` turns a 30-fixture regression matrix into one screen of camera-friendly evidence, and
  it fails loudly if a refactor moves any established value.

Timers (`accept after N` + `%advance`) are the cheapest scheduling probe:

- Advance to the **exact** expected instant and read `Remaining events:` (`1` = armed/re-armed,
  `0` = spent) plus `No pending work - simulation time is now …` — that line is the clearest
  signal that a timer was never re-armed.
- To prove a queued time event is *withdrawn* when its state exits, the state must be left **and
  re-entered**: `accept after 10` left at t=3 and re-entered at t=5 must fire at t=15, not t=10.
  Add a second region with its own clock (`after 12`) so the ordering of two digits (`21` vs `12`)
  is the discriminator instead of one absolute time.
- Bound repeated exit/re-entry with a counter guard (`if trips == 0 do assign trips := trips + 1`)
  or the fixture becomes an infinite loop.
- **Pre-existing on every binary tested (not a PR regression):** a state that owns a
  time-triggered transition ignores its own *signal* transitions, and signal transitions on leaves
  inside top-level regions never fire. So drive timer fixtures purely with `accept after`, and
  verify any such oddity against a contrast binary before reporting it.
- `deep history resume;` works in REPL fixtures (`transition first away then resume`) and doubles as an
  exit-chain probe, since the return path re-runs the entry digits.
- Once composite self-transitions re-enter their regions, a fixture whose signal poster sits
  *inside* the self-transitioning composite becomes an infinite event loop; post from a sibling
  region that is never re-entered.

Change triggers (`accept when`) are reachable through state `%step`, which calls
`PollChangeEvents`. A clean end-to-end example is:

```text
%load internal/repl/testdata/change_condition_object.sysml
%instantiate Watch::Sensor
%state Watch::Sensor
%step
%invoke Watch::Sensor trip
%step
%current
```

Before `trip` the machine waits in `idle` with a false condition. After invocation,
`%step` reports `Change event dispatched` and `idle → alerted`; `%features Watch::Sensor`
shows `tripped = true`. The autonomous conformance fixture also works via repeated `%step`,
but bare `Integer`/`Boolean` types can produce missing-import diagnostics during `%load`.
Do not confuse runtime progression with a diagnostic-free model load.

For a runtime routing fix, build a parent binary in an isolated worktree and load the same
absolute model paths in both binaries. Enable `%trace on` before `%state`: the port-addressed
fixture distinguishes an erroneous `choice`/`strayed` from `received` without a choice.
Record the binary versions and full traces; final state alone can hide a wrong alternative.

Designing discriminating probes here is genuinely tricky — always confirm a probe **FAILs on the
parent** before trusting it:

- `getParentChain` is **leaf-first**, so a single active leaf already gets innermost-first
  treatment; a probe with one leaf plus enclosing composites will pass on both revisions and prove
  nothing.
- The discriminating shape for innermost-wins/dedupe in the poll path needs a **sibling region
  whose leaf watches nothing**, so its outward walk reaches the composite's own watched transition
  while another region's leaf has its own: only a correct implementation keeps the composite alive.
- Keep probe files out of the repo tree before running the gates (`rm` it, keep a `.txt` copy in
  the fixtures dir) and confirm `git status --porcelain` is empty on camera, so the gates clearly
  reflect the committed tree.

## Guarded successions in action flows (PR #266 and anything touching `enabledSuccessions`)

Guards on successions out of *ordinary* nodes (`first s1 if c then s2;`, `then s1 s2 if c;`) are
evaluated in `ActionExecutor.enabledSuccessions`; a failing guard prunes the edge and the flow just
ends there (`Final state: Completed`, not an error). Test it end to end as
`%load <file>` → `%action <pkg>::<action>` → `%continue` → read the `Results:` block.

Traps and recipes:

- **The result values are the only evidence.** A guarded flow that skips its successor still prints
  `✓ Action completed / Final state: Completed`, identical to one that ran it — only the attribute
  values differ. Always design the skipped branch to write a distinctive attribute (`y := 9`) that
  stays at its initial value when the guard prunes the edge.
- **Guards are evaluated when the token LEAVES the node**, so they see what that node wrote: with
  `x = 3`, `action s1 assign x := x + 4;` and `first s1 if x > 5 then s2;` the branch is taken
  (`x = 7`, `y = 9`). A "reads the pre-write value" regression shows up as `y = 0`.
- **A/B against a binary from the parent commit is cheap and decisive here**, because a pre-fix
  build silently ignores the guard: same file, `y = 9` / `ignited = 1` instead of `0`. Build it with
  the worktree recipe above and run both in the recording.
- **Restart the REPL between fixtures.** All the guard fixtures declare `package test`, so a second
  `%load` in the same session makes `%action test::seq` ambiguous/unresolvable
  (`unresolved reference: test::seq — did you mean SequenceFunctions::union::seq1 …`). Ctrl-D and
  relaunch per model; do not chain loads.
- Write fixtures with `import ScalarValues::*;`, otherwise every `attribute x : Integer` emits
  `unresolved reference: Integer` noise on camera (execution still works).
- **The REPL still executes a model with parse errors.** `first s1 if c;` (guard with no `then`) is
  rejected at parse time with `expected 'then' after guard condition`, yet the rest of the model
  loads and runs — read the diagnostics, don't just read the `Results:`.
- Error wordings to assert: non-Boolean guard →
  `error: execution failed: type mismatch: node s1: guard must evaluate to boolean, got constant`;
  unresolvable name in a guard → `error: execution failed: eval guard of node s1: unresolved
  reference: nosuch`. Two guards out of one action node that both hold →
  `error: execution failed: more than one succession is enabled: action node check has multiple
  successors` (wraps the `ErrAmbiguousSuccession` sentinel; the run stops with the token still on the
  node and neither branch's attribute written — assert that with `%tokens`, not just the message).
- Places a guard could still be silently ignored, all worth a one-liner fixture: succession out of a
  real initial node (`then start s1 if …;`), out of a `merge`, out of a `join` (must not deadlock —
  the branch tokens are consumed either way), and one whose target is `done`.
- **A guard inside a nested action body is not observable**: an action declared inside an action body
  that itself declares `first … then …` is never executed (pre-fix binaries behave identically), so
  the enclosing flow's later nodes run while the inner writes never happen. Prove it is pre-existing
  with the A/B binary and report it as untested rather than a failure.

- **A guard on the succession out of a `merge` needs a token-ordering fixture to be tested at all.**
  A merge passes every arriving token (one `MergePerformance` per arrival), running its body before
  the outgoing guard is read and retiring only a token whose outgoing succession is pruned (a merge
  body write is therefore visible to its own guard), so the discriminating shape is a fork with one short branch
  (`a -> mg`) and one longer branch (`b1 -> b2 -> b3 -> mg`) where only the last node of the long
  branch writes the feature the guard reads. `%step` steps every live token per step, so a guard
  reading a value written by a *sibling* branch node in the same step is not discriminating — put at
  least one extra node after the write. Expected on a correct build: `%tokens` shows the short
  branch's token at `mg` with the guard still false, that token disappears (retired), then the long
  branch's token traverses `mg` and the node after it runs. A build that closes the merge after one
  traversal instead discards every later arrival: a loop back into a merge (`action_merge_loop_reenters`
  in the conformance corpus) ends after one pass without reaching its exit, and `%tokens` shows no
  token where the loop should be. An unguarded loop through a merge (`start -> m -> a -> m`) is the
  step budget's to stop, on every build: `%step` circles `m` and `a` one node at a time and
  `%continue` ends in `error: execution failed: execution exceeded max steps (1000000 steps; ...)`.

### Typed failure classes for fork/join/merge/decision control nodes (PR #449 class)

Any PR that claims "typed user-facing errors" for the control nodes is really claiming a *prefix*
per failure class, so the discriminating evidence is A/B against the parent commit: the values and
the fact that an error appears are usually unchanged, only the wording gains its class. Observed at
6b1c1f76 (new) vs its parent (old):

| failure | new wording | parent wording |
|---|---|---|
| fork with no outgoing succession | `invalid action flow: fork node split has no successors` | same text, **no** `invalid action flow:` prefix |
| merge with no outgoing succession | `invalid action flow: merge node converge has no successors` | same, unprefixed |
| join that can never be satisfied | `action deadlock: 1 token(s) stuck, no progress made` | `deadlock detected: 1 token(s) stuck, …` |
| decision whose every guard is false | `no enabled succession: decision node choose has no true guard` | `decision node choose: no true guard` |

All four arrive as `error: execution failed: <typed message>` on `%continue`. Minimal probe shapes
(each its own file, since they all declare `package test` — see the restart trap above):

- **fork/merge with no successors:** declare the node and route a token into it but give it no
  outgoing `then`: `first start; fork split; then start split;` (same with `merge converge;`).
  Parse succeeds, so the failure is genuinely a runtime one.
- **join starvation:** `first start; action stranded; join sync; done end; then start sync; then
  stranded sync; then sync end;` — `sync` has two incoming edges but `stranded` is unreachable, so
  one token waits forever. This is the case worth wrapping in `time timeout 20` on camera: a correct
  build fails in ~0.1s, whereas the pre-fix signature for join/merge bugs is spinning to the step
  budget (see the merge note above), which a bare interactive run makes look like a hang.
- **decision with no true guard:** `attribute enabled : Boolean = false;` plus `decide choose; if
  enabled then selected;` — one guarded branch that is false, and no `else`.

Also worth asserting: **the REPL survives each of these**. Follow the error with `%eval 1 + 1` and
check `= 2`; an executor that leaves the session wedged is a separate defect from the wording.

Fixture noise to expect, not report: `internal/core/runtime/testdata/conformance/
action_fork_branches_share_features.sysml` omits `import ScalarValues::*;`, so `%load` prints two
`unresolved reference: Integer — did you mean ScalarValues::Integer?` errors before running to
`x = 1, y = 2` correctly. It is pre-existing (identical on the parent binary); the sibling
`action_decision_merge_guarded_branch.sysml` loads clean, so a reviewer seeing one noisy and one
quiet load is looking at fixture hygiene, not a regression.

## Verifying the RDF "experimental" marking, `%print`, `%view`, guards and addressed sends

The wave-1/0.1.0 surfaces below were verified end to end at `870da1fd`. Each entry is the
assertion that actually distinguishes working from broken.

### The single experimental notice (`internal/core/export/experimental.go`)

`export.ExperimentalNotice` is the one wording; `export.IsExperimental(from,to)` is true iff either
side is Turtle (notation→notation is never experimental). **Read the constant from source at the
start of a run instead of hardcoding it — the wording has already changed once** (220 chars on the
#261 branch, 239 chars on main; the current text names "the behavior its bodies state").

Assertions that hold, and how to word them:

```bash
./bin/sysml examples/rdf-interop-demo.sysml -convert ttl >out 2>err   # exit 0
grep -c '^note:' err            # 1 — the notice is stderr-only
grep -c experimental out        # 0 — the graph on stdout stays machine-readable
./bin/sysml m.sysml -convert sysml -o n.sysml 2>&1 | grep -c '^note:'  # 0 (notation→notation)
```

- Do **not** assert "stderr is empty" for notation runs: the `wrote … (sysml, N bytes)` line also
  goes to stderr. Assert *no `^note:` line*.
- `-o /dev/stdout` still keeps the graph clean; refusals print the notice *before* the error, exit 2,
  write nothing.
- REPL: `%save x.ttl` prints the notice then `saved N bytes of ttl`; `%save x.sysml` prints none, and
  neither written file contains a `note:` line.
- Cross-surface identity is one diff: compare the CLI note (minus the `note: ` prefix) with
  `opensysml.conversion.EXPERIMENTAL_NOTICE`, the gRPC `ConvertResponse.experimental_notice` and the
  docs string. All four must be byte-equal.
- opensysml: one `ExperimentalFeatureWarning` per RDF conversion, `Conversion.experimental` True with
  the notice; sysml→sysml gives `False`/`""`/0 warnings; a refused RDF conversion warns *then* raises
  `ConversionError`; `warnings.simplefilter("ignore", ExperimentalFeatureWarning)` silences it while
  the conversion still returns the graph.

### Corpus claim (`README.md`, `docs/reference/rdf-mapping.md`)

The claim is a measured number and has moved (71/120 → **102/120** after behavioral nodes were
mapped). Re-measure, never trust the doc:

```bash
ok=0; bad=0
while IFS= read -r f; do
  if ./bin/sysml "$f" -convert ttl -o /dev/null >/dev/null 2>&1; then ok=$((ok+1)); else bad=$((bad+1)); fi
done < <(find examples \( -name '*.sysml' -o -name '*.kerml' \))
echo "converted=$ok refused=$bad total=$((ok+bad))"     # 102 / 18 / 120 at 870da1fd
```

Use process substitution (not `basename`) — many training paths contain spaces. Every refusal must
name a construct **and** `file:line:col` (`succession`, `operator expr`, `prefix metadata`,
`` `snapshot` declaration``, `duplicate declaration of "X"`); a bare "cannot convert" or a silent
drop is the failure shape.

### Round-trip fidelity for behavior (`.sysml → .ttl → .sysml`)

A subaction's `do` and comment contents are the fragile parts. Fixture shape that works:

```
state off { entry do { perform Warm; } }
//* a note naming entry and do and exit */
state starting {
    // a line comment naming exit
    do action running : Warm;
    /* a block comment naming entry do */
    exit perform Warm;
}
```

`entry do { … }`, `do action running : Warm;` and `exit perform Warm;` must all come back, and the
kind keywords inside the three comment styles must introduce **no** spurious members. Note plain
comments themselves are not carried through RDF — that is expected, not a failure. `entry do action
heat : Warm;` does **not** parse (`expected an action, an action reference or '{' after 'entry'`).

### `%print`

Read-only printer of the session buffer or one element:

- `%print` → whole model with its comments; `%print Top::'My Pkg'::Car` → just that element (quoted
  segments work). Empty session → `nothing to print: the session is empty`; a bad name →
  `error: unresolved reference: …`. No RDF wording, no `note:` line.
- Round trip without a GUI: pipe `%load f\n%print\n%quit`, strip the banner/`goodbye` lines with
  `sed '1d;2d;$d'`, reload that output and print again — the two must be byte-identical.
- Regression to keep: `%print` during a live `%action`/`%state` debugger leaves it alone —
  `%instances` still reports none created and the next `%step`/`%continue` still works.

### `%view` (viewpoint conformance)

`%view Demo::report` prints `exposes` then `viewpoint conformance` with
`satisfy structure (from Demo::StructureView): violated` and one line per concern
(`conforms` / `violated (framed by the viewpoint but not by the view)` / `unevaluable` + reason).
A satisfy target that is a `requirementUsage` is diagnosed at load *and* reported as
`unevaluable (satisfy target spec is a requirementUsage, not a viewpoint)` — never a silent pass.
`%view` registers no object: `%instances` afterwards says none created, a repeat is identical, and a
following `%constraint`/`%satisfy` gives a typed message (`not a constraint: MassBudget is a concern
def …`) rather than an ambiguity error.

### Succession guards and addressed sends

- A guard on a succession leaving an *ordinary* action node is evaluated: fixture with `level = 4`
  and branches `if level > 10` / `else` must end with the false branch's counter at 0.
- Fork fixtures under `internal/core/runtime/testdata/conformance/` (e.g.
  `action_succession_guard_fork_branch_pruned.sysml`) run fine in the REPL but **emit unresolved
  `Integer` diagnostics** because they rely on the test harness's implicit `ScalarValues` import.
  Copy the fixture and add `import ScalarValues::*;` if you want a clean transcript.
- Addressed sends carry object identity: a send to `Ident::alpha::reader` must **not** be consumed by
  the sending object's own same-named `reader`. The proof of correctness is a *deadlock error*
  (`accept deadlock in action talk … accept n waiting since step 3`); a broad-delivery bug would
  complete instead. An unresolvable address gives the typed
  `send reaches no receiving port: "Unrouted.alpha.count" names no port of an object the sender can
  address`.
- For the performed/indirect case use `send_identity_performed_object.sysml` (a state entry sends to
  `worker`, a two-level `perform`ed action accepts it) and drive it with `%instantiate` + `%state` +
  `%advance 1` → the machine reaches `heard`. Hand-writing this is easy to get wrong: `send 7 to
  reader` from a package-level scope reports `unresolved reference: reader`, and a qualified
  package-level receiver is *unroutable* because it belongs to no object.

### Change/time triggers that are still absent (issue #268 at 870da1fd)

Confirm absence rather than assuming it, and keep the fixture shape right — the body must hang off
the **state usage**, not the `state def`, or you get
`initialize state machine: no initial state found`:

```
part def Machine {
    attribute ready : Boolean = true;
    state def Modes;
    state modes : Modes { entry; then idle; state idle; state done;
        transition idle_to_done first idle accept when ready then done; }
}
```

`%state …::modes`, `%advance 5`, `%current` → still `Current state: idle` (nothing in
`RunToCompletion` polls change events; documented as ⚠️ Approximate in
`docs/project/spec-compliance.md`). A unit-carrying time trigger (`accept after 5 [SI::s]`) fails
with the typed `initialize state machine: schedule events: time duration must be constant, got
quantity`, while the unitless `accept after 5` fires at t=5 — that pair is the cheapest way to show
the feature is absent-but-typed rather than half-present.

## Source-preserving edits: `ApplyEdits` / `model.edit()` (PR #282)

The edit surface is only reachable through opensysml (`model.edit()` → `set_value` / `rename` →
`apply()` → `save(path)`); there is no REPL meta-command for it, so the REPL is only useful
afterwards, to prove the edited file still parses/instantiates.

Setup that actually matters: the client auto-starts `~/.opensysml/bin/sysml-grpc`, so a stale copy
there serves an old build and `apply()` fails as `MissingCapabilityError('apply_edits')` — which
looks like a client bug. Always `go build -o bin/sysml-grpc ./cmd/sysml-grpc`, then
`pkill -x sysml-grpc` (the file is `Text file busy` while it runs) before
`cp bin/sysml-grpc ~/.opensysml/bin/`.

Fixture shape that discriminates a broken implementation: one file with line comments, a block
comment, blank lines and **deliberately mixed tab/space indentation** (some lines space-indented
inconsistently). A reformatting implementation normalizes those lines, so the assertion is
`diff -u orig edited` showing exactly **2** changed lines per operation (`diff | grep -c '^[<>]'`),
never "the file still validates". `cat -A` the fixture on camera to show the tabs are real.

Targets and evals worth using (they cover the interesting locate.go branches in one file):

| target | notes |
|---|---|
| `Demo::sc::unitMass` (`attribute redefines unitMass = …;`) | the `redefines` shorthand path |
| `Demo::SC::margin` (`attribute margin : ISQ::MassValue;`) | value **added**: `AppliedEdit.length == 0`, `new_text == ' = 5.0[SI::kg]'` (a space is inserted before `=`) |
| `Demo::SC::avionics::board::count` | deeply nested; eval it as `eval("avionics.board.count", subject="Demo::sc")` |
| `Demo::SC::total = unitMass * 2` | expression referencing another feature; evaluates against the *redefined* value (1200 → 2400) |

Refusals and the exact class each raises (all leave the file byte-identical — sha256 it before and
after every case, since "an exception was raised" says nothing about writes):

| case | class / `failure` |
|---|---|
| unknown FQN, **and any stdlib element** (`ISQ::MassValue` reports `no element named … in this model`) | `EditTargetError` / `EDIT_FAILURE_UNKNOWN_TARGET` |
| a `part def` target | `EditTargetError` / `EDIT_FAILURE_NOT_VALUED` |
| `"1050.0[SI::kg"`, `""`, rename to `part` or `2bad` | `InvalidEditError` (`INVALID_VALUE` / `INVALID_NAME`) |
| a value that parses but does not resolve (`Nope::missing`) | **`EditResultError` / `EDIT_FAILURE_RESULT_INVALID`** — not `InvalidEditError`; it is caught by re-analysis, and `diagnostics` names the *model* file and line |
| two `set_value`s on one feature | `OverlappingEditsError` |
| `apply()` twice | plain `builtins.RuntimeError` (client-side, **not** a `OpenSysMLError`) |
| `apply()` with no ops | `NoEditsError` |
| non-`str` value | `TypeError` |
| renaming a referenced declaration | `RenameReferencedError`, `referring_elements == ['Demo::SC', 'Demo::sc']` |

Two cases need process work rather than a Python call:

- **Evicted model → `ModelNotFoundError`.** Hand-start the service (`bin/sysml-grpc -port 50123
  -health-port 8123`) with `opensysml.connect(port=50123, auto_start=False)`, load, kill and restart
  it, reconnect, then rebuild the editor against the *new* connection with the *old* hash:
  `opensysml.edit.Editor(m._hash, c2)`. Killing the auto-started 50051 service mid-script instead
  tends to hang the run — a `connect()` right after a `pkill -x sysml-grpc` did not return.
- **`MissingCapabilityError` before the RPC.** Don't chase the v0.0.7 download; build the merge-base
  with `git worktree add /tmp/oldmain main` + `go build -o /tmp/old-sysml-grpc ./cmd/sysml-grpc`
  and run it on port 50099 (`-health-port 8099`; 8081 collides). Assert the old service still
  serves reads (`load` + `eval`) and that the raise is `MissingCapabilityError`, not
  `UnsupportedOperationError`/UNIMPLEMENTED — that class difference is the proof the check ran
  client-side. The service logs no per-RPC lines at INFO, so "no ApplyEdits in the log" proves
  nothing on its own.

`Model.find` returns **None** for a missing symbol (only `model[name]` raises), so the
"old name is gone after a rename" assertion must compare against `None`; a `try/except` around
`find` passes even when the rename did nothing.

### Traps that cost time when re-testing the edit surface

- **`clients/python/tests/test_edit.py`'s `real_service` fixture prefers `<repo>/bin/sysml-grpc` over
  `~/.opensysml/bin/sysml-grpc`** (`GRPC_BINARIES`, test_edit.py:61). A stale `bin/sysml-grpc` left
  from an earlier snapshot therefore fails all 13 `TestEditRoundTripAgainstRealService` cases with
  `MissingCapabilityError('apply_edits')` / `assert has('apply_edits') == False`, which reads like a
  product bug. Always `make build-grpc` and check `./bin/sysml-grpc -version` prints the current
  commit *before* running the suite; the same applies to the copy in `~/.opensysml/bin`.
- `MissingCapabilityError` lives in **`opensysml.capabilities`**, not `opensysml.errors`
  (`opensysml.errors.__getattr__` raises `AttributeError` for it). It is still a `OpenSysMLError` and is
  *not* an `UnsupportedOperationError`.
- `test_generate_golden.py::test_typed_codegen_modules_are_mypy_clean` may fail for reasons unrelated
  to any change: with mypy 2.3.x and the venv's numpy stubs it reports
  `numpy/__init__.pyi:737: error: Type statement is only supported in Python 3.12 and greater`.
  Reproduce with `echo 'import numpy' > /tmp/nm.py; $HOME/pv/bin/python -m mypy --no-incremental
  /tmp/nm.py` before blaming a PR; pinning/refreshing numpy stubs is the likely fix.
- Non-ASCII **identifiers** (`package Démo`) do not parse, so a Unicode fixture must keep the
  accents/emoji in comments and strings only. That still exercises the byte-vs-rune offset risk:
  assert `saved == orig[:a.offset] + a.new_text.encode() + orig[a.offset + a.length:]`, which is the
  strongest single assertion available for `AppliedEdit` (it proves offsets are byte offsets and
  that nothing outside the span moved).
- CRLF: build the fixture as `orig.replace(b"\n", b"\r\n")` and assert the saved file has the same
  `\r` count, **zero bare LFs** (`b.count(b"\n") - b.count(b"\r\n") == 0`) and no `\r\r`. Test the
  *insertion* case (a feature with no value) on CRLF too, not just replacement.
- Unwritable targets: `save()` raises plain `PermissionError`/`FileNotFoundError` from `open()`, so
  sha256 the victim file before/after — the point of the case is that it is not truncated first.
- An edit is allowed on a file that already has errors elsewhere (name *or* syntax), and the saved
  file keeps exactly those pre-existing diagnostics; a refusal there is the regression the
  "baseline diagnostics" fixes in PR #282 were about.

## `%check` and the SMT solver driver (PR #285)

`%check <name>` asks an external solver about a constraint def, requirement def or satisfaction
assertion. z3 may already be installed (`/usr/bin/z3`, 4.8.12 seen); discovery is `OPENSYSML_SMT`
(path to an executable) → `z3` → `cvc5` on PATH, and `OPENSYSML_SMT_TIMEOUT` (default `10s`) bounds
one query. Every solver-availability path is reachable from the shell without touching code, so
drive each in its own REPL and show the env on camera:

```bash
OPENSYSML_SMT_TIMEOUT=1ms ./bin/sysml            # "? … is undecided (z3, 1ms)" + "Reason: the solver ran out of time after 1ms"
OPENSYSML_SMT=/tmp/fake_unknown.sh ./bin/sysml   # sh script: printf 'unknown\n(:reason-unknown "incomplete")\n'; then `while read -r line; do :; done`
OPENSYSML_SMT=/tmp/fake_fail.sh ./bin/sysml      # sh script: exit 3 → "error: the SMT solver did not answer: … failed at check-sat"
env -u OPENSYSML_SMT PATH=/tmp/emptybin ./bin/sysml   # "error: no SMT solver found: install z3 …"
```

The fake-unknown script *must* keep draining stdin after printing, or the driver's later writes hit
a closed pipe and you get a process error instead of `unknown`. Note `env -u … PATH=/tmp/emptybin`
also removes `timeout` from PATH — call `/usr/bin/timeout` by absolute path in piped dry runs.

Fixture shapes that actually discriminate:

- **Naming a satisfaction:** `%check` on a `requirement usage` reports the *requirement*, not the
  assertion. To reach a satisfaction, `%check` the element whose body holds `assert satisfy r by o;`
  (e.g. a `part context { assert satisfy fastEnough by car; }`) → `✓ Satisfaction satisfy fastEnough
  by car is satisfiable`. Bare `satisfy X by y;` at package level is rejected at parse/semantics
  ("satisfy target must be a requirement usage") — always go through a named requirement usage.
- **Truncating `/` and `%`:** use a *symbolic* dividend/divisor (`a == -7 and b == 2 and q == a / b`),
  because a constant `-7 / 2` is const-folded to the evaluator's real quotient (`%eval -7 / 2` = `-3.50`),
  so `q : Integer == -7 / 2` is reported **unsatisfiable** by design. The discriminating pair is
  `q == a / b and q == -3` (sat) vs `q == -4` (unsat, the floor answer); same for `r == a % b` with
  `-1` vs `1`. A literal zero divisor is refused with "operator `/` by zero not translatable".
- **Untranslatable:** `i ** 2 == 4` → `error: constraint X: operator `**` not translatable for
  solving: …` (no verdict). Incommensurable units are refused the same way (`L against M`).
- **Read-only:** start `%action <A>`, run `%check`, then `%step` — the token must still advance
  (`Token 1 @ start` → `Token 1 @ step1`).
- **Assignments use qualified OpenSysML names** (`Check::Satisfiable::i = 4`,
  `Check::SpeedReq::'craft.topSpeed' = 100`, enum as `Check::Gear::high`). A quantity is a magnitude
  in the *base units* a written unit reduces to, named as such (`1500.0 [kg]` comes back as
  `1500000.0 [gram]`), so don't expect the unit as written.

## `%optimize` and the optimization queries (PR #305)

`%optimize <name>` optimizes an `analysis def`'s (or analysis *usage*'s) `objective`s. Everything
below was observed at 41dc35cb with `/usr/bin/z3` 4.8.12; `bin/sysml` needs no extra setup beyond
`make build-sysml` and z3, both already in the blueprint.

- **Model contract for writing fixtures:** direction comes from the type
  (`TradeStudies::MinimizeObjective` / `MaximizeObjective`), the optimized value from
  `attribute :>> best = <expr>;` inside the objective body, and feasibility from the case's
  `require`/`assume` conditions plus the objective's own. `private import TradeStudies::*;` is
  needed. A ready fixture with a dozen discriminating cases is
  `internal/core/solve/testdata/objectives.sysml` — start there rather than hand-writing one.
- **The three headers are the fastest read:** `✓ … is optimized`, `! … has no optimum: an objective
  improves without limit`, `! … is satisfiable, but its optimum was not established`, and
  `✗ … has no values satisfying its conditions` for unsat. A number must never appear on the
  objective line for the two `!` cases; a feasible witness is labelled
  `the assignment below attains <v>` instead, which is the tell that no optimum was fabricated.
- **z3 4.8.12 is wrong on strict inequalities** — for `margin < 10.5` maximized it answers `9.5`.
  The verification step catches this and reports `no optimum reported — the solver reported 9.5,
  but a strictly greater value is feasible, so it is no optimum`. This is the single most valuable
  assertion in the area: a build that skipped verification would print `9.5` as the optimum.
- **Lexicographic order needs a coupled pair to be visible.** Two independent objectives look the
  same either way. Use `cost` in `[3,9]` and `margin` in `[0, cost*2]` with `minimize cost` declared
  first: lex gives `cost 3` then `margin 6`, while any other priority gives `cost 9`/`margin 18` or
  `margin 0`. (`CostThenMargin` in the fixture is exactly this shape.)
- **Refusals are per-reason and carry a `<repl>:line:col` location.** Worth covering each, since they
  are distinct code paths: a nonlinear value (`a * b`) → `it states a nonlinear value`; an objective
  typed by something that is not a Minimize/Maximize objective → `it states no direction to improve
  its value in`; `objective goal : MinimizeObjective;` with no body → `it states no value to
  improve`; a case with conditions but no `objective` → `error: analysis X: states no objective`.
- **Quantities come back in base units**, as with `%check`: `10 [kg] .. 90 [kg]` minimized reads
  `10000.0 [gram]`. Mixed units in one case (`>= 500 [g]`, `<= 2 [kg]`) are reconciled correctly
  (`2000.0 [gram]`) — a good adversarial case that a naive translation would get wrong.
- **Bad input:** `%optimize` alone → `usage: %optimize <name>`; an unknown name →
  `error: unresolved reference: X`; a package/part def/constraint def →
  `error: not an analysis case: X is a <kind>, not an analysis case definition or usage`
  (the article follows the kind's first letter, `articleFor` in `internal/core/runtime/describe.go`).
- **Read-only check that actually discriminates:** `%action <A>`, `%instances`
  (`(no instances created)`), `%optimize <case>`, then `%step` twice to `State: Completed` with the
  right `Results:` — and `%instances` still `(no instances created)`. Run `%optimize` twice in a row
  too; answers must be byte-identical.
- **Absent solver** is one env var: `OPENSYSML_SMT=/nonexistent/x ./bin/sysml <f>` then `%optimize` →
  `error: no SMT solver found: OPENSYSML_SMT names "/nonexistent/x", which is not an executable
  file`. cvc5 (in `$HOME/.local/cvc5/bin`) is the other interesting backend: optimization is a z3
  extension, so it must be a typed error rather than a plain check-sat presented as an optimum.
- Writing an `action` fixture to host the read-only test: successions are
  `first s1 then s2;` (see `internal/core/runtime/testdata/conformance/action_succession_guard_holds.sysml`).
  `then first second;` / `first then second;` do **not** parse or lower — budget a minute for this
  rather than inventing syntax.

## Value bodies over inherited values, require bodies, and the extension math functions (PR #296)

- **A nested value body over an inherited value** (`part def Ring { attribute cost : Cost = template; }`
  plus `part r : Ring { attribute :>> cost { attribute :>> v = 11.0; } }`) is observable purely
  through `%instantiate` + `%features`: the governing body shows `cost.v = 11.00` while the source
  object (`template`) keeps its own values, and pre-fix builds show the inherited `9.00` *and* a
  shared instance ID between `cost` and `template` — the **instance IDs in `%features` are the
  cheapest tell** that a body materialized an object of its own rather than aliasing the source.
  Ready-made fixtures with in-model `assert constraint`s live at
  `internal/core/runtime/testdata/conformance/attribute_body_over_inherited_value*.sysml`; loading
  one and reading `<constraint: satisfied>` / `<constraint: violated>` in `%features` is a stronger
  frame than eyeballing numbers. Note `%satisfy` answers `no satisfaction assertion in the session`
  for such a fixture (those are `assert constraint`, not `assert satisfy`) — not a failure.
- Discriminating negatives to keep in any test of that area: a body that only *re-declares* a nested
  feature (`attribute :>> kept { attribute :>> v; }`) must still read the inherited value, and a
  value **plus** a restatement on the *same* declaration
  (`attribute :>> ringCost = template { attribute :>> v = 5.0; }`) must still fail with
  `feature value <inst>.<feat>: feature both valued and restated in a body: v` (exit 2).
- **Names nested inside a `require`/`assume` body** are only checked through the CLI's analysis, so
  test them with `sysml -validate <file>`, not the REPL: a typo at depth 2/3 inside
  `require Q::r { part p : P { attribute x = typo; } }` must report
  `unresolved reference: typo` with the caret under the name and exit 2. Pre-fix builds print
  `no errors` and exit 0, so **only a contrast binary proves this class of fix**. Pair it with a
  legal deep body (must stay clean) and a body-local name read from the enclosing namespace (must
  still be reported as unresolved — the body is a scope of its own).
- **The OpenSysML extension math functions (`exp`, `ln`, `log`, `atan2`)** are no longer answered
  unimported: `%eval exp(1.0)` gives
  `error: evaluation failed: function is not in scope: exp is declared by OpenSysMLMathFunctions, a
  OpenSysML extension no OMG library declares: write \`import OpenSysMLMathFunctions::*;\` to call it`.
  OMG names (`sqrt`, `sin`, …) and the qualified `OpenSysMLMathFunctions::exp(1.0)` still evaluate
  unimported — check both, or a change that broke the OMG fallback looks like a pass.
- A prompt-level `import OpenSysMLMathFunctions::*;` submitted **alone** used to leave `exp` out of
  scope, because a document declaring nothing carried no symbol for the scope tree's document name
  (fixed in PR #296, regression test `TestRootImportInDocumentDeclaringNothingElse`). Worth retesting
  in that exact shape — an import followed by any declaration takes a different route and passes even
  when the bare form is broken.

## `%explain <name>` — unsat cores (PR #291)

`%explain` asks the solver which conditions of an unsatisfiable constraint/requirement/satisfy
element conflict. It shares `solveQueries` with `%check` (`internal/repl/check.go:149`), so the two
must always reach the same verdict; the split is that `%check` prints a satisfying assignment and
`%explain` never does. Rendering lives in `internal/repl/explain.go`, core reduction in
`internal/core/solve/core.go`.

`internal/repl/testdata/explain_conflicts.sysml` is the fixture that covers every core-row shape at
once, so prefer it over hand-rolled models. Values observed at 04d0c4b with z3 4.8.12, loading that
file **alone** (locations are buffer-relative — see below):

| `%explain Conflicts::…` | header | rows |
|---|---|---|
| `Contradictory` | `✗ … 2 conditions conflict` | `required condition: \`i > 8\`` @10:29, `\`i < 3\`` @11:29 |
| `NatBound` | `✗ … 2 conditions conflict` | `declared domain: \`a Natural is not negative\` — declaration Conflicts::NatBound::n` @20:9, `required condition: \`n < 0\`` @21:29 |
| `ZeroDivisor` | `✗ … 2 conditions conflict` | `well-definedness: \`a / b == 1\`` @28:29 **then** `required condition: \`b == 0\`` @27:29 |
| `Derived` | `✗ Requirement … 2 conditions conflict` | `\`x > 10\` — requirement Derived, declared by Base` @34:30, `\`x < 1\` — requirement Derived` (no "declared by") @38:30 |
| `Satisfiable` | `✓ … is satisfiable, so no conditions conflict` | none, plus `Use %check … for a satisfying assignment.` |
| `rig::always` | `✗ Constraint always (negated) … 1 condition conflicts with itself` | `denied conditions: \`not (i == i)\`` @47:9 |

Every unsat header is followed by one minimality line before the numbered rows.

Things that look like bugs but are not, and traps:

- **Rows are in query-assertion order, not source order.** `ZeroDivisor` lists the hoisted
  divisor guard (line 28) before the condition that sets the divisor to zero (line 27). Don't
  report this as mis-ordering without checking `translate.go`'s assertion order first.
- **The location column always names `<repl>`, never the loaded file, and counts lines from the
  start of the joined buffer.** With one file loaded the numbers happen to match the file; load a
  second file first and the same condition moves (`<repl>:23:29` for what is really
  `explain_conflicts.sysml:10:29`), while load-time *analysis* diagnostics in the same session do
  name the real file and count from its start. The unit tests assert `<repl>`, so this is the
  REPL's implicit-document convention rather than a regression — but it is misleading, and any
  location assertion must therefore fix the load order. Always `%load` the fixture **alone**, or
  pass it as a CLI argument (`./bin/sysml <fixture>`), when asserting line/col.
- **A 1-member core has its own minimality wording**: `The condition below is the whole conflict:
  nothing else is needed for it.` (`internal/repl/explain.go` `minimality`), not the multi-condition
  `dropping any one leaves the rest satisfiable`. `rig::always` is the case that exercises it.
- **`String` IS in the translatable subset** (`SortString`, `internal/core/solve/reference.go:214`),
  so `constraint { s == "x" }` answers `satisfiable` and is useless as an "outside the subset" case.
  Untranslatable cases that do work: a **calc invocation** in a condition
  (`assert constraint { Twice(i) > 4 }` → `invocation not translatable for solving: it is outside
  the subset`) and an **unresolved reference** (→ `it resolves to nothing`).
- **z3 decides nonlinear reals**, so `x*y == 7.5 and x*x + y*y == 2.0` comes back `unsat` with a
  real core, not `unknown`. The `? … is undecided, so there is nothing to explain` branch is hard
  to reach on purpose; use `OPENSYSML_SMT_CORE_BUDGET` (a tiny value) if you need to exercise the
  non-minimal `Note` wording instead.
- **Header durations include core-reduction time** (since 04d0c4b), so an unsat `%explain` reads
  ~3x the matching `%check` (24–30ms vs 7ms). Never assert exact milliseconds.
- **`%explain` is read-only.** An `%action Debug::tally` session (fixture
  `internal/repl/testdata/action_debug.sysml`) must survive it: `%step` after `%explain` still
  prints `✓ Step complete` and the run ends `total = 5`. A regression here shows up as
  `error: no active action session`.

The no-solver path is the one case needing a special launch: `mkdir -p /tmp/nosolver` and run
`env PATH=/tmp/nosolver ./bin/sysml`, which yields `error: no SMT solver found: install z3 … or set
OPENSYSML_SMT …; looked for [z3 cvc5] on PATH`. Note `Discover()` is consulted per command, so this
must be set on the process, not toggled mid-session.

## Rendering a view: `%render` and `sysml -render` (PR #288 class)

A view's `render` member is consumed by `internal/core/view`. Kinds: **tree** (default when the view
states no rendering), **interconnection**, **state**, **action**, **table**
(`render asElementTable;` or a `StandardViewDefinitions::GridView`-typed view).

Forms and defaults (`internal/core/view/form.go`):

| kind | text | machine form (`Kind.MachineForm()`) |
| --- | --- | --- |
| tree, action | indented text | `mermaid` → `flowchart TD` |
| interconnection | indented text | `mermaid` → `flowchart LR` |
| state | indented text | `mermaid` → `stateDiagram-v2` |
| table | space-aligned columns | `markdown` (pipe table) |

- **REPL defaults to `text`; the CLI defaults to the kind's machine form.** Don't assume they match.
- Wrong form for a kind is a typed `*WrongFormError`:
  `<view>: a table rendering is not written as mermaid; ask for text or markdown`. An unknown form in
  the REPL: `unknown form "svg"; usage: %render <name> [text|mermaid|markdown]`; via the CLI:
  `unknown rendering form "svg"; -render-form takes text, mermaid, markdown`, exit 2.
- Always assert **0 bytes on stdout** on refusals, not just the message.
- CLI stream contract: artifact on **stdout only**; `✓ package …` load report, `note: … renders
  empty`, "cannot represent" notices and `wrote <file> (form, N bytes)` on **stderr**.
  `-render` with `-convert` → exit 2 `-convert and -render each write a document out; ask for one per
  run`; with `-validate` → exit 2 "check it in its own run".
- Table row shape: `Element | Kind | Type | Declared in`; exposed elements use qualified names,
  elements declared inside them use local names; a nested view is a row followed by its own exposures.
- **Empty state rendering must be `state "the view exposes nothing; the rendering is empty" as empty`,
  never a bare `note "…"`** — a bare top-level `note` is invalid Mermaid. Fixture:
  `internal/core/view/testdata/errors.sysml` → `ErrorViews::emptyStateView`. Proof pattern for this
  class of fix: build the pre-fix commit with `git worktree add`, render the same view, and show
  `mmdc` failing (`Parse error on line 2`, exit 1) on the old artifact and passing on the new one.
- Mermaid grammar check (independent of Go tests):
  `cd /tmp && npm install @mermaid-js/mermaid-cli`, write
  `/tmp/pc.json` = `{"args":["--no-sandbox","--disable-setuid-sandbox"]}`, then
  `/tmp/node_modules/.bin/mmdc -p /tmp/pc.json -i x.mmd -o x.svg`. **`/tmp` installs do not survive
  between sessions — re-check `ls /tmp/node_modules/.bin/mmdc` before relying on it (a missing binary
  shows up as `mmdc exit=127`).** Open the SVG in Chrome (Ctrl+A in the omnibox first, then
  `file:///tmp/x.svg`) for visual proof.
- **`%render` must be read-only.** Proof sequence: `%action Gear::Spin` → `%step` → `%tokens` →
  `%render …` → `%tokens` (identical) → `%step` → `%continue` (completes) → `%instances`
  (`(no instances created)`). `internal/core/view/testdata/action.sysml` is **unusable** for this —
  its `action provide : Provide` has no initial node, so `%continue` fails there on `main` too; write
  your own steppable action.
- Byte-identity regression harness: build the previous commit via `git worktree add`, then loop the
  four graph kinds × `text`/`mermaid` through both binaries and `diff` — expect all `IDENTICAL`.
- Known nit (unfixed): completion of a *partially typed quoted* name offers nothing
  (`%render Quoted::'My` + Tab) because `nameWord` keeps the leading `'` while the index holds
  unquoted FQNs (`internal/repl/complete.go`); with a trailing space inside the open quote it dumps
  every library name. Forms are correctly withheld until the quote closes.
- `filters.sysml` → `FilteredViews::safetyView` duplicates `Systems::Airbag` and
  `Systems::Braking::Brake` through the shipping CLI/REPL while the goldens list them once. It
  reproduces on `main` via `%view`, so it is pre-existing exposed-set behavior, not a rendering bug.
- Pitfall: never type `clear;` at a `sysml>` prompt — it parses as SysML, errors
  (`expected a namespace member`) and pollutes the buffer, which later shows as
  `note: deeper checks may not have run here: the error on buffer line N is unresolved`. Use
  `ctrl+l`, and `%quit` before running shell commands.
- After the OpenSysML rename the module is `github.com/Open-MBEE/OpenSysML` and env vars are
  `OPENSYSML_*`, but the **checkout directory may still be named `Systemica`**. When grepping output
  for stale branding, exclude build paths, and expect the deliberate legacy RDF namespace
  `urn:systemica:sysml:` (`internal/core/rdf/vocab.go`) to remain — it exists so a pre-rename graph is
  refused rather than misread.

## SMT logic selection, the capability model, and `%optimize`

The REPL never prints the logic a query set, so to test logic selection **wrap the real solver in a
tee** rather than reading the goldens. `newSolver` picks the CLI flags from the wrapper's *base
name*, so a wrapper whose name contains `z3` is invoked with z3's own `-smt2 -in`:

```sh
mkdir -p /tmp/fakesmt /tmp/smtlog     # tee fails if the log directory is absent
cat > /tmp/fakesmt/z3-tee <<'EOF'
#!/bin/sh
tee /tmp/smtlog/script-$$.smt2 | /usr/bin/z3 "$@"
EOF
chmod +x /tmp/fakesmt/z3-tee
```

`OPENSYSML_SMT=/tmp/fakesmt/z3-tee ./bin/sysml <model>` answers normally and leaves one script per
solver invocation. **Capability probes run through the same wrapper**, so the log also holds the
probes' own scripts; select the real query's with `grep -l <ModelName> /tmp/smtlog/*.smt2` (each
script starts `; OpenSysML SMT-LIB2 translation of constraint <Name>`). Expected logics from
`internal/core/solve/testdata/logic_selection.sysml`: `CratesPerPallet` (`crates / 12`) → `QF_LIA`
(integer division does **not** widen the logic); `CratesPerRun` (variable divisor) → `QF_NIA`;
`MassAndCrates` (Int + Real) → `AUFLIRA`; a datatype/variant model (`ring_variants.sysml`) → `ALL`,
preceded by `; no SMT-LIB logic covers algebraic datatypes (declare-datatypes), …`.

### `%optimize`
`%optimize <name>` takes an **analysis case** (`runtime.RequireAnalysis`), not a constraint — and
conversely an analysis def is not a `%check`/`%explain` target (`%check test::SomeAnalysis` answers
`error: no satisfaction assertion in …`). Fixtures live in
`internal/core/solve/testdata/objectives.sysml`, e.g. `test::CrewSizing` → `maximize largestCrew =
`crew`: 7`, `test::CostThenMargin` → `minimize cheapest = `cost`: 3` then `maximize widestMargin =
`margin`: 6` (lexicographic, in declaration order), `test::UnboundedLoad` → `! … has no optimum: an
objective improves without limit` plus `the assignment below attains 1.0` — a bound or a feasible
value is never printed as an optimum.

Optimization is a **z3 extension**, so only z3 can be used to prove the happy path. cvc5 must refuse
through the capability model (`Solver.requireOptimization` preflights `CapOptimization` *and*
`CapOptimizationPriority`), printing
`error: the SMT solver does not implement optimization: cvc5 lacks `(maximize …)`/`(minimize …)`
with `(get-objectives)`, a solver extension: it rejected the script: Parse Error: …; install z3 or
set OPENSYSML_SMT to it`. Probe results are cached per executable+args, so a second `%optimize` in
the same session must print the identical refusal — a differing second error means the cache is not
holding. `internal/repl/optimize_test.go` skips its solver cases through
`requireOptimizingSolver`, so **`go test` alone proves nothing about `%optimize` on cvc5**; drive the
REPL, or read the `TestPortability -v` report, which must show `refuse objective optimization` for
cvc5 and `pass` for z3.

### Fake backends for the failure modes
Name them so they contain neither `z3` nor `cvc5` (otherwise they inherit that family's flags); the
name is also what the message reports as the solver.

- **Refused feature** → typed `UnsupportedCapabilityError`: a python wrapper forwarding to z3 but
  answering `unsupported` to `(get-unsat-core)`. `%explain` then names backend, feature and
  operation while `%check` in the *same* session still answers — the refusal must be scoped to the
  feature, not the backend.
- **Dead backend** (`#!/bin/sh` + `exit 3` or `exit 0`) → `error: the SMT solver did not answer:
  <name> failed at check-sat: it stopped without answering`, never a capability error. A dead
  backend leaves the probe *undetermined* (`capUnknown`), which deliberately does not refuse.
- **Hanging backend** (`cat > /dev/null; sleep 600`) → bound it with `OPENSYSML_SMT_TIMEOUT=5s` and
  time the run: expect `? … is undecided … Reason: the solver ran out of time after 5s` in roughly
  3× the timeout (probe + query + verification each get their own budget), not a hang.
- **Garbage-echoing non-solver** (replies `i am not a solver` to everything) → a *process* error, not
  a capability refusal: `error: the SMT solver did not answer: <name> failed at capability check: it
  answered `i` rather than an SMT-LIB response`. Only an `(error …)` or `unsupported` reply is a
  missing feature (`capability.go` `smtlibResponse`), so when checking a refusal message make sure the
  fake backend answers one of those rather than arbitrary text.

`OPENSYSML_SMT=/nonexistent/no-such-solver` gives `error: no SMT solver found: OPENSYSML_SMT names
"…", which is not an executable file` for `%check`, `%explain`, `%configure` and `%optimize` alike —
discovery precedes both the capability preflight and the "states no variation point" complaint.

Cross-solver agreement worth asserting (z3 4.8.12 vs cvc5 1.3.4): the verdict line, the witness
values (`crates = 36`; `MassPerCrate` → `crates = 1`, `mass = 0.0`), the unsat-core rows of
`internal/repl/testdata/explain_conflicts.sysml`, and the **count and set** — not the order — of
`%configure test::ringFamily::variantsAgree all` on `nested_variants.sysml` (3 selections; the
solvers list them in different orders, which is not a defect). `%configure` names the **constraint**
(`…::variantsAgree`), not the part.

## Proving parser fixes: "was the body actually read?" and corpus sweeps

For parser PRs that make a previously-rejected notation parse (connector/interface/flow bodies,
accept nodes as statements, …), a clean `-validate` alone is weak evidence: the parser could be
skipping the body it now tolerates. Two probes settle it, and both are worth having in the plan:

- **Members of an *anonymous* usage are not in the symbol index at all.** `%search wireGauge` after
  loading `connect a.p to b.p { attribute wireGauge : Gauge; }` answers `no symbol matches` — and so
  does a plain anonymous `part : Sensor { attribute anonPartMember : Gauge; }`, so this is
  pre-existing ownership behavior, **not** a dropped body. Don't report it as a bug; always compare
  against an anonymous *part* body before concluding anything.
- **The authoritative body-visibility probe is a deliberately unresolvable type inside the body**:
  `connect a.p to b.p { attribute a : NoSuchType; }` must report
  `unresolved reference: NoSuchType` at the in-body column. A dropped body prints nothing and
  exits 0. Use the named form (`connection c connect a to b { … }`, `action n accept e : E { … }`)
  when you want `%search` to show `Pkg::Owner::c::member` instead.
- For `accept`, assert the action name, the payload and any body member resolve as three separate
  rows: `%search engineStart` yields `Demo::Startup::engineStarted::engineStart attributeUsage`
  while `%search engineStarted` yields the `actionUsage` — proof they did not collapse into one.

**Corpus sweep against a contrast binary** over `examples/pilot-corpora` (for the corpus roots and
their file counts see [docs/project/pilot-differential.md](../../../docs/project/pilot-differential.md)) is the cheapest
regression net, but two traps ruin it:

- The corpus paths **contain spaces**: `for f in $(find …)` word-splits and every validate silently
  runs on a nonexistent fragment, producing a bogus "0 differences, all identical" result. Use
  `find … -print0 | while IFS= read -r -d '' f`.
- Compare **per-file `grep -c 'error:'` counts**, not the clean/total ratio: most corpus files fail
  for unrelated unimplemented features, so the ratio barely moves while individual files improve.
- Expect some files to get *more* errors after a syntax fix. Validation is tiered, so a file that
  used to die at Tier 1 (`did not analyse cleanly; no check was made`) now reaches name resolution
  and reports pre-existing unresolved references. Confirm this reading by validating the file
  **together with the model it imports** (copy both into a temp dir and `-validate <dir>/`); if the
  remaining diagnostics are all `unresolved member/reference` in other feature areas, it is
  tier-unmasking, not a regression.
- To show that leftover errors in a partially-fixed file are only **cascade** from a known
  unimplemented gap, patch a temp copy replacing just that construct (e.g. a body on
  `first a then b { … }` / `merge m { … }` → `;`) and validate the copy — clean output proves the
  downstream diagnostics were recovery noise.

## Authoring edits (`model.edit()` add_member / delete) via the Python client

The edit API is service-side byte splicing plus re-analysis, so testing it needs the **freshly
built** `bin/sysml-grpc` copied to `~/.opensysml/bin/sysml-grpc` (`pkill -x sysml-grpc` first) and
a protobuf-new-enough interpreter — `/home/ubuntu/opensysml-proto-venv/bin/python` works
(grpcio 1.83 / protobuf 7.36); the system Python's protobuf is too old for the regenerated stubs.
Confirm the surface is actually present before blaming the client:
`connect().server_info().capabilities` must contain `authoring` and `inline_language`.

- **Good fixture:** `examples/rdf-interop-demo.sysml` — 4-space indent, a leading `//` comment, a
  `doc`, blank lines between declarations, a body-less `port def TelemetryPort;`, a referenced
  `part def Battery`, and a trailing `comment about Wheel`. That one file covers golden path,
  body-less owner growth, delete-referenced refusal and comment-preservation in one go.
- **Preservation must be proven by diff, not by eyeball:** run
  `difflib.unified_diff(original, edited, n=0)` and assert **zero** `-` lines. Body-less owners are
  the one legitimate exception (the `;` line is rewritten into `{ … }`).
- **Known cosmetic artifact:** inserting into an owner whose closing `}` sits on an indented line
  leaves a whitespace-only line (`"    \n"` / `"\t\n"`) where the indentation used to be. Byte
  content elsewhere is intact, so treat it as a formatting nit, not a preservation failure — but do
  flag trailing-whitespace-sensitive linting.
- **KerML vs SysML discriminator:** `class B specializes A;` is clean in KerML and reports
  `only a definition may specialize; found a usage` in SysML. Use it to prove
  `loads(src, language='kerml')` really switched languages; plain `class`/`struct`/`feature`/`assoc`
  declarations parse clean in *both* and prove nothing.
- **Refusal expectations:** unknown owner → `OwnerNotFoundError`; a usage/attribute owner (or a
  body-less usage) → `OwnerNotNamespaceError`; wrong-language kind → `IllegalMemberKindError`;
  duplicate name → `MemberNameTakenError`; unresolvable type target → `EditResultError` (the whole
  batch is refused after re-analysis, so include a *valid* sibling add in the same batch and assert
  it did not land). Delete of a referenced declaration → `DeleteReferencedError` naming the
  referrer.
- Editors are single-shot: a second `.apply()` raises `RuntimeError`, an empty one `NoEditsError`.
  Build a new editor from the reloaded model instead of reusing one.
- `Symbol.parts()` exposes `multiplicity` but leaves `type_name` `None` even for pre-existing
  members — verify added typing via the written notation or by resolving the type's FQN, not by
  `type_name`.
- **Proving `loads(..., language='kerml')` really analysed as KerML** needs a probe sensitive to the
  KerML *implicit library base* (`class` → `Occurrences::Occurrence`, `struct` → `Objects::Object`,
  `datatype` → `Base::DataValue`). The one that works: `class C { feature redefines endShot; }` —
  clean inline-as-KerML and in a `.kerml` file, but reports
  `unresolved reference: endShot` when the same text is loaded with no `language` argument.
  Diagnostics parity between the inline load and the identical text saved as `.kerml` (compare the
  messages with the file-name prefix stripped, since inline diagnostics are prefixed `<content>`)
  is the assertion to make. Note `Symbol.specializations`/`type_facts`/`attributes()` do **not**
  expose implicit library supertypes, so don't try to observe the base there.

## Proving an *executable* body ran — and ran exactly once

A parser PR that makes an optional body legal on a node (control node, initial node, succession,
transition) has a second, sharper failure mode than "the body was not read": the body parses and
lowers but never *executes*, or executes more than once. `-validate` cannot see either. Drive the
built REPL and judge on exact attribute values, never on `Action completed`:

- Make every body write a **counter** (`assign forkRuns := forkRuns + 1;`) rather than a constant,
  then read `Results:` after `%continue`. A constant assignment cannot distinguish "ran once" from
  "ran three times"; a counter can.
- Chain the reads so a dropped body is visible downstream: `fork split { assign x := 1; }` →
  `action left { assign y := x + 1; }` → `join sync { assign z := y + 1; }` must give exactly
  `x = 1, y = 2, z = 3`. A dropped fork body shows up as `y = 1`, not as an error.
- The duplicate-execution hazards are structural, so build for them explicitly:
  - a **fork** with two outgoing branches (a body run per emitted token gives `forkRuns = 2`);
  - a **join** whose branches are of unequal length, so the short branch's token parks at the join
    and the executor retries it for several steps (a body run per *arrival* gives `joinRuns > 1`).
    `%step N` + `%tokens` shows `Token 2 @ sync` waiting with the counter still `0`, which is the
    screenshot that proves the retry actually happened before `%continue` finishes.
- For transitions, `%state <qname>`, then `%current` before and after `%advance 1`: the body effect
  and the entry action of the substate reached by a dotted end (`then beta.work`) must be separate
  attributes so `State data:` distinguishes them. `%current` also prints the `State stack (active
  configuration)`, which is how you confirm a dotted end entered the *nested* state.
- A body statement the runtime cannot execute should surface as
  `error: execution failed: action node <n>: … is not executable`. If a construct is documented as
  "parses but unsupported at runtime" (e.g. `send x via p to r`), assert that error text explicitly —
  otherwise you cannot tell "reported as unsupported" from "silently skipped".
- Adversarial: put a non-terminating `while true { … }` **inside a node body**. Expect
  `evaluation step limit exceeded (… steps; raise OPENSYSML_MAX_STEPS to allow more)` within a second,
  and assert the session survives by running `%eval 1 + 1` right after in the same REPL.

### A/B against a binary built from the parent commit

The cheapest proof that a syntax fix is real (and the best screenshot) is a side-by-side with the
pre-change compiler:

```
git worktree add /tmp/wt-old <parent-sha> && (cd /tmp/wt-old && go build -o /tmp/old-sysml ./cmd/sysml)
/tmp/old-sysml -validate <fixture>   # expect the old parse error, exit 2
./bin/sysml    -validate <fixture>   # expect "no errors", exit 0
```

Compare `grep -c 'error:'` counts per file for the corpus files the PR claims to unblock. Beware
`| grep …` when you also want the exit status — `$?` is the exit of `grep`, so capture
`${PIPESTATUS[0]}`.

### REPL pitfalls that cost time here

- Shell built-ins typed at the `sysml>` prompt are parsed as SysML: `clear; %tokens` yields
  `error: expected a namespace member` and, worse, leaves an unresolved error in the buffer, so
  later `%load`s print `note: deeper checks may not have run here…`. Quit the REPL before running
  shell commands, or use `printf '%%load …\n%%continue\n%%quit\n' | ./bin/sysml` per case.
- Naming fixture attributes after keywords or inherited features wastes a cycle: `after` is a
  reserved keyword, and `done` collides with the inherited `Actions::Action::done`, which downgrades
  the run to `did not analyse cleanly; no check was made`. Pick neutral names (`next`, `flag`).

## Testing behavior/expression parser fixes (F64/F66/F71-style PRs)

Parser PRs that "unblock a form" are best proven with three surfaces, in this order:

1. **`-validate` on minimal fixtures**, A/B against the parent binary (above). Positive fixtures must
   go from an old parse error to `no errors`/exit 0; the PR's known-blocked fixtures must still exit 2
   with a located `file:line:col: error:` and zero `panic`/`goroutine` lines.
2. **The REPL for execution**, because parsing a form is not executing it. A returned usage
   (`calc scaled { in x : Real; return attribute doubled : Real = x * 2.0; }`) is only proven by
   `%calc Sc::scaled 21.0` printing `= 42.00`. Typed runtime errors are visible the same way, e.g.
   `error: evaluation failed: unsupported declaration in a body expression: …` for a collection body
   that declares a feature, and
   `error: calc invocation failed: no result expression: …` for a returned usage with no value.
   Always follow an error with `%eval 1 + 1` → `= 2` to show the session survived.
3. **A whole-corpus A/B sweep** for regressions:

```bash
while IFS= read -r -d '' f; do cp "$f" /tmp/sw.sysml
  diff <(./bin/sysml -validate /tmp/sw.sysml 2>&1; echo $?) \
       <(/tmp/old-sysml -validate /tmp/sw.sysml 2>&1; echo $?) >/dev/null || echo "DIFF: $f"
done < <(find examples internal/core/runtime/testdata/conformance -name '*.sysml' -print0)
```

Copy each file to a fixed path first — corpus paths contain spaces, and comparing the *output plus
exit status* catches wording changes a count would miss.

**Expect "tier unmasking" and classify it, do not report it as a regression.** Because higher
validation tiers are skipped when a lower tier errors (see AGENTS.md §4), a parser fix makes files
that previously died at a parse error reach name resolution, where *pre-existing* problems appear —
so the raw `grep -c 'error:'` count can *rise* on a file the PR improved. Distinguish the two by
reading both outputs: old = `expected ';' after return expression`, new = `unresolved reference: …`
or `name conflict: …` at a different line is unmasking; a new *syntax* error at a line the old
binary accepted is a real regression.

**Known limitation to check for on F64-style body-declaration work:** the parser may keep the
features a body expression declares (`ast.BodyExpr.Members`) without the resolver contributing them
to the body's scope, so a model that *uses* the declared name still fails with
`unresolved reference: <name>` even though it now parses. Test the using case explicitly
(`(1..n)->forAll { in i; private attribute k = r * i; k > 0.0 }`), and don't assume a clean parse
means the form works end to end.

**Adversarial forms worth having as files** (each must exit 2, never 124, and never panic): truncated
`return x :>` at EOF, `assume constraint c1 :`, an unclosed `[` in a returned usage multiplicity,
EOF mid-declaration, and — for notation changes that reclassify keywords per file type — a `.kerml`
file using the word as both a name and a modifier (`snapshot timeslice x : T;`). Verify the same
change did *not* break the word's modifier role in `.sysml` (`snapshot start; timeslice cruise;`
inside an `occurrence` must still validate clean).

**An unclosed bracket in the REPL is not a diagnostic yet.** The prompt switches to `...>` and keeps
buffering, so a following `%eval` is swallowed as model text. Press `ctrl+c` to flush the buffer —
the diagnostics then print and the session stays usable. Budget for this when driving malformed
input on camera, rather than reading it as a hang.

## RDF expression trees, reference-rewriting rename, declaration-aware Query (PR #390)

Three surfaces changed together here; each has a cheap, high-signal probe.

**Expression trees in Turtle.** Every expression-valued position (feature value, multiplicity
bound, condition, guard, payload, loop collection) emits a node in `urn:opensysml:expr:`
(`@prefix expr:`). A one-file fixture that reaches value/bound/literal/boolean/condition at once:

```
package P {
    part def Panel {
        attribute width : ScalarValues::Real;
        attribute height : ScalarValues::Real;
        attribute total = width * height + 2;          // nested operator tree
        attribute count : ScalarValues::Integer[1..width * 2];  // bound
        attribute label = "panel";                     // literal
        attribute ok = total > 10 and count < 5;
    }
    constraint def Fits { attribute w : ScalarValues::Real; assert constraint { w < 100 } }
}
```

Assert structurally, not by grepping the source text: count `expr:` subjects and require the same
number of `sysx:sourceText` triples on them (source text must be on *every* node), require
`sysml:operator` + two `sysml:argument` nodes with distinct `sysx:argumentIndex` on the operator
node, and require `sysml:lowerBound`/`sysml:upperBound`/`sysx:condition` to point at `expr:` IRIs
rather than plain literals. A stale binary emits `sysml:value "width * height + 2"`, which that
assertion rejects — this is the check that proves you rebuilt.

Connector/binding heads (`connect`/`bind`/`flow`/`allocate`/`succession`) additionally emit
`sysx:relatedFeature` end nodes carrying `sysx:endIndex` and, for directed heads, `sysx:endRole`
`"source"`/`"target"`. Round-trip these too (`c.ttl -> c.sysml -> d.ttl`, `diff` empty).

**Source text stays authoritative for ttl→sysml.** A malformed expression node that still has
`sysx:sourceText` converts *successfully* — that is by design, not a bug. To reach the structural
decoder in an adversarial fixture you must delete the parent node's `sysx:sourceText` as well as
the triple you are breaking. Broken graphs then exit 2, write no output file, and the message
names the exact node IRI, e.g.
`cannot convert the expression <urn:opensysml:expr:P__Panel__total.value.a1>: a literal expression
states the value it evaluates to`.

**opensysml API details that cost time:**

- `Model.edit()`, not `Model.editor()`.
- `Model.hash`, not `Model.model_hash`.
- `ApplyEditsResponse` has **no** `success`/`message` fields — read `resp.error` (empty string means
  success), `resp.failure` (e.g. `EDIT_FAILURE_INVALID_NAME`), `resp.content`, `resp.referring_elements`.
  Checking "empty content on refusal" on the wire means asserting `resp.content == ""` from a raw
  stub call, not just catching `InvalidEditError`.
- Never name a probe script `grpc.py` — it shadows the `grpc` package.

**Rename fixture that covers all the rewrite/leave-alone rules in one file:** a declaration, an
unqualified reference (`attribute doubled = mass * 2;`), a qualified reference
(`R::Widget::mass`), an `alias m for R::Widget::mass;`, an explicit `import R::Widget::mass;`, a
wildcard `import R::Widget::*;` (must not change), a same-named member in an unrelated package
(must not change), a sibling `weight`, and comments. Assert the output re-validates
(`bin/sysml -validate`) and that it is *not* byte-identical to the input (catches a silent no-op).

**Query property names are JSON-LD-ish:** `@id` and `@type`, not `id`/`type`. Requesting an unknown
property returns an error listing the valid set. `isIndividual` is **not** in `QueryPropertyNames()`
— individual status is only observable through `@type` = `OccurrenceDefinition`/`OccurrenceUsage`.
If a PR description claims `isIndividual`, verify whether it is a queryable property or only a
type refinement, and report the difference.

Values worth asserting after the declaration-aware change (contrast against a parent-commit service
on another port to make the delta visible; the parent returned `Feature` or an absent `@type`):
connector ends `ReferenceUsage`, `interface def` ends `PortUsage`, `class`→`Class`,
`struct`→`Structure`, `assoc`→`Association`, `behavior`→`Behavior`, `predicate`→`Predicate`,
`interaction`→`Interaction`, individual def/usage → `OccurrenceDefinition`/`OccurrenceUsage`,
alias → `Membership`.

**Konsole font size on camera:** use `ctrl+plus` / `ctrl+minus`. `ctrl+shift+plus` is not a Konsole
binding and types literal `+` characters into the shell (`+++cd: command not found`).

## Driving state self-transitions and budget failures from the REPL (PR #411)

Self-transition semantics (`StateExecutor.encloses`: `source == target` is *external*, so exit →
effect → entry run and the state is visited twice) are fully REPL-observable, but only if the
fixture's trigger is one the REPL can produce:

- **The shipped conformance fixtures mostly use `accept again` (a signal), which the REPL cannot
  inject** — they park in their start state forever. Re-author the same model with
  `transition first s accept after 5 do assign log := log * 10 + 9 then s;` and drive it with
  `%state <Pkg>::<Machine>`, `%trace on`, `%advance 5`, `%current`. The `log` digits then read as a
  transcript of the ordering, so one integer is a complete assertion — e.g. simple state
  `log = 1291`, composite `1342913`, orthogonal `17342913`, matching the `.expected.json` values.
- **Change triggers (`accept when <cond>`) *are* dispatched by `%advance`** despite the folklore
  that only `RunToCompletion` polls them: the rising-edge model reaches `waiting` with `log = 1`
  from the REPL and gRPC `execute_state` agrees (`states_visited ['start','waiting','waiting']`).
  Always run both surfaces and diff them — that pair is what would expose a debugger-only gap.
- **`./bin/sysml <model> -state <machine>` does NOT run to completion.** It only prints
  `Started state machine executor …` / `Current state: <initial>` and exits 0, so it is useless as
  an execution oracle; a model that "stays in `start`" there proves nothing. Use the REPL's
  `%advance` or gRPC `execute_state`.
- **A `region` needs its own `initial`**; `entry; then x;` inside a region fails lowering with
  `region <r> in state <S> has no initial state`, which looks like a runtime bug but is fixture
  shape.
- **Untriggered (completion) self-transitions are the budget test.** They are *not* dispatched when
  `%state` starts the machine — you must `%advance 1`, which then reports
  `Stopped at the event budget (1000000 events; raise OPENSYSML_MAX_EVENTS to allow more)` in ~2 s and
  leaves the REPL usable (`%eval 1 + 1` → `= 2`). The object-exhibited path fails earlier and
  louder: `%instantiate <Pkg>::<PartDef>` on a `part def` with `exhibit state …` returns
  `instantiation failed: exhibited state machine <M> of object #1: state machine exceeded max
  events (…), possible infinite loop` plus `(no instances created)` — there is no need for a
  `%state <M> <obj>` step, and that object name never exists to be referenced.
- A/B against the parent commit is cheap and decisive here: the pre-fix binary prints `log = 19`
  for the simple fixture (internal transition, no exit/entry) while composite and orthogonal
  fixtures are byte-identical across both binaries.
- The blueprint's `maintenance` step already builds a opensysml venv at **`~/pv`** and copies
  `bin/sysml-grpc` into `~/.opensysml/bin`; use `~/pv/bin/python` instead of building a new
  `/tmp/pv` venv. Copies of hand-written fixtures must qualify stdlib types
  (`ScalarValues::Integer`) or add `private import ScalarValues::*;` when moved out of the
  conformance directory.

## Per-source-language keyword reservation (wave 7C, PR #428)

Reservation is decided per *source language* (`lexer.IsKeywordIn` + parser `reservedWord`), so the
file extension of the fixture is part of the test, not an incidental detail.

- A word only `KerML.xtext` spells (`chains`, `type`, `namespace`, `all`) is an ordinary name in a
  `.sysml` file: `part chains : T;`, `part type : T;` validate clean (exit 0). The same word as a
  *feature name* in a `.kerml` file must still error — always test both directions, one fixture
  per extension, or you will pass a test that only proves the lexer is permissive everywhere.
- Words that are SysML-only syntax (`frame`, `render`, `state`, `part`, `action`) are ordinary
  names in `.kerml` (`feature frame : T;` clean) but are **syntax** in SysML positions:
  `viewpoint def V { frame; }` and `view def V { render; }` must be diagnosed.
- Watch the *severity*: `part frame : T;` in `.sysml` currently yields a **warning**
  (`"frame" is a reserved keyword; write 'frame' to use it as a name`) and exit 0, not an error.
  Assert on exit code *and* on whether the line says `warning:` or `error:`.
- `%search <Pkg>::` is the cheapest confirmation that a name-shaped keyword really became a symbol:
  it prints quoted names such as ``PosK::'frame'  attributeUsage``.
- Recovery noise is expected on some of these: `viewpoint def V { frame; }` emits two diagnostics
  (the real one at the `;`, plus `expected a body member`). Report it as message quality, not as a
  missing diagnostic.

### The REPL buffer has no extension

The REPL document is `<repl>` with no extension, and unknown-extension buffers read as **SysML**
for reservation. Two consequences worth knowing before writing assertions:

- You cannot prove `.kerml`-specific parser behaviour by typing into the prompt. Use
  `%load /path/x.kerml` (kind comes from the loaded file) or the CLI, and use the prompt only for
  the SysML-side names.
- REPL diagnostics print **`1:12: error: …`** — line:col with **no `<repl>` filename prefix**
  (CLI output does print `path:line:col:`). If a test plan says "confirm diagnostics report
  `<repl>:line:col`", that expectation does not match the current binary; check with the lead
  before calling it a pass or a bug.

### Escaping the continuation prompt

Incomplete input (`part def U { ref redefines x[4;`) puts the REPL into a `...>` continuation
prompt, and *anything* typed there — including `%eval 1+1` — is swallowed as more model text.
Press **Ctrl-C** to abandon the continuation: the buffered text is then parsed, its diagnostics
print, and the `sysml>` prompt returns usable. Always follow a malformed submission with
`%eval 1 + 1` (expect `= 2`) to prove the session survived. Never type shell words like `clear`
at the prompt; it parses as a model line and produces `expected a namespace member`.

### Kindless parameters and the RDF shape

A kindless parameter (`in x : Real`, `out mass : Real`) is a kindless/attribute usage, so
`sysml -convert=turtle` emits `a sysml:AttributeUsage`. Hand-written fixtures are often *not*
discriminating (both old and new binaries agree); the repo fixture
`internal/core/export/testdata/convert/views_flows_parameters.sysml` is, because its
`action def Measure { out mass : Real; }` prints `AttributeUsage` on the new binary and
`PartUsage` on a parent-commit binary. Prefer an A/B against `/tmp/old-sysml` over asserting a
single output.

### Cross-checking against the pinned pilot validator

`./build/pilot-validator/validate-sysml <file>` takes ~20 s per file (it reads the whole standard
library) and prints hundreds of `Reading …` lines plus `log4j:WARN` noise — filter with
`grep -v '^Reading \|log4j'` and capture `${PIPESTATUS[0]}` for the real exit code. The pilot is
*narrower* on keyword-as-name forms: `part frame : T;` and `part all : T;` are
`no viable alternative at input …` (exit 1) there while OpenSysML accepts them. Classify that as
wider/narrower, not automatically as an OpenSysML defect.

## The conformance harness loads no libraries; the REPL does (wave 7D)

`internal/core/runtime/testdata/conformance/*.sysml` runs with **no standard library loaded**, so a
fixture that passes there can still fail at the CLI, where the stdlib is always present. Two defects
of that exact shape were only visible through `bin/sysml`:

- A receiving node named `receiver` made a routed `send x via p to receiver` report
  `send receiver is unreachable`, because in the sending action's scope that name also resolves to
  the inherited `Transfers::SendPerformance::receiver`. Renaming the node to `reader` "fixed" it —
  that asymmetry is the tell for a name-shadowing bug, not a routing bug.
- Any hand-written model, and any conformance fixture opened in the REPL, needs
  `private import ScalarValues::*;` (or qualified `ScalarValues::Integer`) or the session fills with
  unresolved-`Integer` noise that hides the result you are checking.

So: for any change under `internal/core/runtime`, re-run the shipped fixture through the REPL with
libraries loaded, and add a library-loaded unit test (`buildRuntimeWithLibraries`) beside the
library-free one. Prefer a name a library also declares (`receiver`, `source`, `target`) when
choosing fixture names — those are the ones that break.

## Checking a mutated object is seen through *every* path that reaches it

The high-value bug class for `%invoke`/calc work is a plausible wrong number, with no error. Drive
it as a matrix, not a single call: mutate a feature through an action, then invoke calcs that reach
it by different routes, and assert the mutated value in each.

```
%invoke bot drain                  # action assigns charge := 3
%eval in Pkg::bot : charge         # = 3
%invoke bot direct                 # charge + 100      -> 103
%invoke bot anon                   # charge * 2        -> 6
%invoke bot nested                 # anon() + 1        -> 7    (was 21: declared default)
%invoke bot usesDirect             # direct() + 1000   -> 1103 (was 1110)
```

The two-hop rows are the ones that regress: a calc reached through a *calc invocation expression*
took a different code path from a calc *usage* read, and only the usage shape had a unit test. Any
"the body now sees the object" claim should be checked with at least one two-hop case per shape
(anonymous result and named result), at the CLI, since the unit test passing proved nothing here.

Bounds are worth a sweep in the same session — each of `OPENSYSML_MAX_STEPS`, `OPENSYSML_MAX_ACTION_STEPS`,
`OPENSYSML_MAX_EVENTS`, `OPENSYSML_MAX_DO_STEPS`, `OPENSYSML_MAX_ELEMENTS`, `OPENSYSML_MAX_CALC_DEPTH` set to a tiny
value must error naming that bound and return *no* result; a truncated-but-returned answer is the
failure to look for.

## Testing name-resolution / visibility changes at the CLI

Resolution changes (visibility, imports, qualified names) are best proved with an **A/B contrast**
against a build of the parent commit, because the interesting evidence is "this used to analyse
cleanly and now reports an error":

```bash
git worktree add /tmp/wt-old <parent-sha>
(cd /tmp/wt-old && go build -o /tmp/old-sysml ./cmd/sysml)
./bin/sysml f.kerml </dev/null; /tmp/old-sysml f.kerml </dev/null   # compare output AND exit code
```

Pitfalls that cost time:

- `bin/sysml <file>` **drops into the interactive REPL after analysing** when stdin is a TTY. In a
  recorded Konsole always append `</dev/null` (or plan to type `%quit`), otherwise every subsequent
  shell command is parsed as SysML.
- Use `.kerml` fixtures for KerML shapes (`classifier X specializes Y`, `feature f references g`).
  The same text in a `.sysml` file can fail earlier with `only a definition may specialize; found a
  usage`, masking the behaviour under test.
- A batch regression sweep is cheap and is the strongest "no false positives" evidence: run every
  file in `examples/` and `testdata/{passes,resolve}` under both binaries and require byte-identical
  output plus matching exit status.
- Cold vs warm run under a scratch `XDG_CACHE_HOME` catches resolution that depends on the on-disk
  symbol index; diagnostics must be byte-identical.

### Proving parser *recovery* (not just the diagnostic) with `%search`

For a change that claims "malformed X no longer abandons the enclosing body", the diagnostic text is
only half the evidence — you must show the members *after* the malformed one still exist in the tree.
`-convert kerml` refuses a file with syntax errors, so use the symbol index instead:

```bash
printf '%%load /tmp/f.kerml\n%%search P::\n%%search P::B::\n' | ./bin/sysml
```

Build the fixture with a sentinel member inside the broken body (`feature c;`) and a `tail`
declaration after it. A working recovery lists `P::B::c` *and* `P::tail`; a broken one hoists the
member to the wrong scope (`P::c`) and drops `tail` — exactly what a parent-commit control binary
shows. Note that not every dot shape is covered by such a fix: chains like `a...b` and `..a` may
still cascade and drop later members identically on both binaries, so A/B every shape before
calling a residual cascade a regression.

Also worth checking on such changes: whether the changed member form still *declares* a name.
A reference member (`perform doIt;`, plain `exhibit s;`) is expected to resolve an existing name, so
an undeclared target must produce `unresolved reference: …`; the "did you mean <the same FQN>?"
suggestion is normal because `%search` sees the reference-derived index entry.

### Which REPL surfaces are visibility-aware

`%search` and readline name completion browse the **raw symbol index** (`internal/repl/discover.go`,
`complete.go`) and are *not* filtered by member visibility, so a `private` member is still listed
even when resolution rejects every reference to it. The visibility-filtered surface is
`model.Workspace.VisibleNames/VisibleNamesAt`, which today is reached only from `cmd/pilot-xpect`
scope checks and not from any REPL meta-command — do not report a `%search` listing of a private
name as a regression without A/B-ing it against the parent build first. `%view <name>` *is* useful
for `expose`: it lists what a view exposes, and an `expose`/`import all` is expected to reach its
target's own private members.

## End-to-end testing `ApplyEdits` / `model.edit()` over gRPC (PR #509 and later edit work)

Any change in `internal/core/edit` (index reuse, reparse/validate ordering, refusal kinds) is
testable entirely through the real service plus the Python client; the strongest evidence is
**a batch of N dependent operations against the same operations sent one per request**.

```bash
export PATH=/usr/local/go/bin:$PATH
make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/
XDG_CACHE_HOME=$(mktemp -d) ./bin/sysml-grpc -port 50123 &     # -port, not -addr
/home/ubuntu/pv/bin/pip install -e clients/python/                     # see the venv trap above
```

- Drive it with `opensysml.connect(port=50123, auto_start=False)` and
  `conn.load_from_content(text)` (there is no module-level `load_from_content`).
  `m.edit()` collects `add_member(owner, kind, name, type=…)`, `set_value`, `rename`, `delete`
  and `apply()` sends them as one `ApplyEdits`. An `Editor` is single-use: build a new one from
  the reloaded model for the next request.
- A fixture that actually reaches the library-index overlay needs a library-resolving type:
  `package Rig { private import ISQ::*; part def Sensor { attribute reading : ScalarValues::Real; }
  part def Frame; part sensor : Sensor; attribute margin = 1.0; }`. Ops whose later members are
  owned by earlier ones (`add Rig::Mount`, then `add Rig::Mount::height`) are the reparse path;
  ops that don't nest are applied as one batch without any reparse, so a flat op list proves nothing.
- Equivalence assertions worth making: `str(result)` byte-identical, the
  `[a.target + " -> " + a.new_text for a in result.applied]` sequence identical (operation indexes
  differ between one request of N and N requests of one, so leave them out), reparse diagnostics
  identical, and `m.get(fqn)` over both the added and the pre-existing names identical.
- **`Model.get()` and `Model.find()` return `None` for a name they don't hold — they do not raise.**
  A "no cross-model leakage" check written as `try: m.get(x); leaked=True` passes vacuously and
  then fails for the wrong reason; assert `m.get(x) is None`. `m[name]` is the raising form.
- Refusals arrive as typed `opensysml.errors.EditError` subclasses with `.failure` and
  `.diagnostics`. Useful trio, all of which must refuse with empty content and leave the cached
  model untouched: a later add taking a name an earlier add wrote
  (`EDIT_FAILURE_MEMBER_NAME_TAKEN`, message names the colliding operation pair), an add naming a
  type nothing declares (`EDIT_FAILURE_RESULT_INVALID`, with an `unresolved reference` diagnostic),
  and an add owning something no operation created (`EDIT_FAILURE_OWNER_UNKNOWN`).
- For perf/refactor claims, run a **contrast service from the parent commit on a second port**
  (`git worktree add /tmp/old <sha>; go build -o /tmp/old-sysml-grpc ./cmd/sysml-grpc;
  /tmp/old-sysml-grpc -port 50124`) and drive both from one script: assert byte-identical
  notation and identical refusal kind/message/diagnostics, and time `apply()` only. At 8cef2b61 an
  8-operation request measured ~17 ms median vs ~60 ms on HEAD~1.
- Cheap leak checks that would catch a request-local index escaping: send the same request 3x
  against the same model hash (content and applied lists must be identical each time), and edit two
  different models alternately (neither result's text nor lookups may contain the other's names).
- Recording: this surface has no GUI, so record a maximized Konsole
  (`setsid konsole … &; wmctrl -a Konsole; wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz`)
  and `tee` the script output, then page it with `sed -n 'a,bp'` so each section fits one screen.

### Devin Secrets Needed
None — the service, client and stdlib are all local.

## Proving "the stdlib is parsed on every load path" (record-format waves, e.g. formatVersion 25)

When `internal/core/libs` changes what the on-disk cache persists (derived facts only:
`Supers`/`Unit`/`Dimension`/`Abstract`, installed onto already-parsed symbols), the load-bearing
property is **cold == warm == no-cache**, observed from outside the Go tests. Three cache states,
one scratch dir:

```bash
C=$(mktemp -d)
XDG_CACHE_HOME=$C   ./bin/sysml ...   # cold (dir empty)
XDG_CACHE_HOME=$C   ./bin/sysml ...   # warm (ls $C/sysml-ls/libs | wc -l  -> ~95 *-v25.idx)
XDG_CACHE_HOME=/proc/self/nope ./bin/sysml ...   # no-cache fallback
```

- There is **no `-no-cache` flag**; point `XDG_CACHE_HOME` at an uncreatable path and the loader
  logs `WARN stdlib symbol cache unavailable, loading without cache error="mkdir ..."` on stderr
  and continues. Compare after `grep -v 'stdlib symbol cache unavailable'` or with stderr dropped,
  otherwise the diff is only that warning and you will misreport a failure.
- Cheap sanity that the warm run really hit the cache: wall time drops (~1.0s -> ~0.28s for a
  single `-validate`; ~3.5s -> ~2.7s for a multi-command battery) and the `*-v25.idx` files exist.
- A battery worth diffing per cache state (all against one library-leaning fixture):
  `-validate`, `-e '2.0 [SI::kg] + 3.0 [SI::kg]'`, `-e <lib-typed attr>`, `-constraint <c>`,
  `-calc 'Sum(2, 40)'`, `-instantiate <def> -json`, `-query`, `-convert sysml`, plus the
  pilot-reject negative fixture. Then repeat over
  `internal/core/runtime/testdata/conformance/*.sysml` (360 files) for a corpus-level A/B.

### Library feature multiplicity: declared `0..1` vs assumed `1..1`
Neither `-query` nor gRPC `GetSymbol` reaches standard-library symbols, so you cannot read a
library feature's multiplicity directly. The observable surface is
`internal/core/passes/multiplicity_conformance.go`: redefine a library feature with a wider or
weaker bound and check the warning text.

```sysml
package MultD { occurrence def Probe :> Occurrences::Occurrence {
  occurrence mid : Probe[0..*] redefines Occurrences::Occurrence::middleTimeSlice; } }
```
-> `Subsetting/redefining feature should not have larger multiplicity upper bound`; redefining
`timeSlices` with `[0..1]` -> `Redefining feature should not have smaller multiplicity lower
bound`. A build that only has declaration-free library records stays **silent** on both (it falls
back to assumed `1..1`), so silence-vs-warning is the discriminator; `[0..1]` against a declared
`0..1` library feature must stay clean.

### gRPC/Python control when library attributes are NOT withheld
With no L3-3 projection, `GetSymbol` on a part returns own attributes **first, in declaration
order**, then ~55 inherited from `Occurrences`/`Objects`/`Base` (e.g. `demo::Car` in
`internal/grpc/testdata/conformance/symbol_attributes.sysml`: 6 own + 55 = 61). Assert the head
order and that the client can read every row; don't assert a total.
Two traps that reproduce on **base too** (do not attribute them to a record-format PR):
- A `@Metadata` annotation written *inside* a part def collapses that symbol's gRPC attribute list
  to its own declarations only. Put the annotation on a `part` usage instead if you want the
  inherited tail. Always A/B this against the parent commit before reporting it.
- A user package named exactly like a bundled library package (`package Occurrences { ... }`)
  reports no reachable children over gRPC (`Model.get("Occurrences::Thing") is None`); the REPL is
  fine.
- `instantiate` reports a constraint feature as `feature value 'massOK': feature value is not
  materialized` — also present on base.

## Proving a pure-performance change is behavior-preserving (line-index / rollback work)

Perf PRs whose claim is "identical output, less cost" are only testable as a **differential**
against a binary built from the PR's merge-base (`git merge-base HEAD origin/main` — note
`origin/main` may have moved past the branch point, so use the merge-base, not `main`, or you
attribute unrelated commits to the PR). Build both `cmd/sysml` and `cmd/sysml-lsp` from each side.

What to diff, in order of how much it catches:

- **`-validate` over off-by-one bait.** A finding on line 1 and on the final line, a file with no
  trailing newline, a zero-byte file (must still print `✓ <name>: no errors`, exit 0), one very long
  single line with many findings, and multi-byte UTF-8 (comments with `é`/`—`/emoji) *before* a
  finding on the same line — columns are **byte** columns, so an index that recounts differently
  shows up here. Diff `stdout+stderr+exit` as one blob.
- **A large warning-heavy model**, for output volume: `/home/ubuntu/perf/models/m{4000,8000,16000}.sysml`
  emit warnings on every element (m4000 is ~16.8k output lines); `clean{3000,6000,12000}.sysml` emit
  none and are the control that the change did not just move cost around.
- **Multi-submission REPL transcripts.** `%load` several files, then type two or three multi-line
  package snippets at the prompt, interleaving `%list`/`%print`. Findings in loaded files must be
  reported as `file:line:col` relative to that file, prompt findings as bare `line:col` relative to
  their own submission. Re-issuing `%list` after later submissions is the staleness probe.
- **Failed-materialization rollback.** The cheapest reliable failure is a part def whose exhibited
  state machine has no body: `part def Broken { exhibit state empty { } }`. `%instantiate` of it
  fails outright; nest it as `part bad : Broken;` inside a `part def` (also 4 levels deep, and as a
  `part bads[3] : Broken;` collection) and reach it with `%features` or
  `%eval in <Def> : a.b.c.bad` — children are materialized lazily, so instantiating the parent alone
  proves nothing. Diff the whole transcript **including instance IDs**: the ID gaps left by
  discarded objects are the sharpest signal that the rollback bookkeeping matches the old one.
  Repeat each failing read 2–3 times and re-read a sibling afterwards to prove re-materialization.
- **LSP edit cycles**, for cache staleness: drive `bin/sysml-lsp` over stdio with `initialize`,
  `didOpen`, then several full-text `didChange`s that cycle content v1→v2→v3→v1, printing every
  `publishDiagnostics` range. Diff the sequences from both binaries. (Documents are rebuilt per
  version in `internal/core/model/document.go`, so a per-SourceFile memoized index is safe — but
  this is the test that would catch it if that ever changes.)
- `go test -race` on `./internal/core/source/... ./internal/repl/... ./internal/core/runtime/... ./internal/lsp/...`,
  plus, for a memoized accessor, a throwaway package inside the module (`mkdir tmp_racecmd`, one
  `_test.go` firing 64 goroutines at `sf.Lines().PosAt(...)`, then `rm -rf` it) — an in-repo dir is
  required because `internal/...` is unimportable from outside the module.

Measure with `python3 /home/ubuntu/perf/timeit.py <cmd>` (wall + peak RSS; GNU `time` is absent).
Baseline observed for the line-index memoization: m4000 8.06s→0.55s, m8000 31.7s→1.07s,
m16000 124.3s→2.18s, clean models unchanged (~0.43s / ~1.4s). Clean-model load being slower than
some older baseline is a **separate known regression**, not a perf-PR failure.

## Testing generated typed views (Tier 2) over a live service

The typed helpers in `clients/python/opensysml/typed.py` are only reachable through *generated* modules, so
assert on generated code, never on hand-built protobuf messages:

```bash
/home/ubuntu/pv/bin/python -m opensysml.generate /abs/model.sysml -o /abs/out_types.py
grep -nE "_t\.(optional_|list_)?feature_value" /abs/out_types.py
```

`_accessor()` in `generate.py` picks the helper purely from multiplicity: collection →
`_t.list_feature_value`, `0..1` → `_t.optional_feature_value`, otherwise `_t.feature_value`. So the
model's multiplicity is the only lever for which helper a property compiles to. Generation needs the
service to report `type_facts`; check with
`opensysml.connect(auto_start=False).server_info().capabilities`.

**Tier 1 read shapes for valueless features** (observed via `instance[name]`, and matching the
REPL's `%features`) — know these before designing a no-value test, or an assertion can be vacuous:

| declaration | Tier 1 value |
|---|---|
| `attribute x : Real;` (required) | `UNSET` (`<unset>`) |
| `attribute x : Real [0..1];` | `UNSET` |
| `attribute x : Real [0..1] = null;` | `None` |
| `attribute x : Real [0..*];` / `[0..3]` | `[]` (**never** bare `UNSET`) |
| `attribute x : Real [1..*];` | `[<unset>]` — a list *containing* `UNSET` |
| `attribute x : Real [2..5];` | `[<unset>, <unset>]` |
| `part p : Def [0..*];` (valueless) | `[]` |

Consequences worth writing into any report:

- A `value is UNSET` guard inside `list_feature_value` is **unreachable from real models** — valueless
  collections already arrive as `[]`. Test a collection change against the pre-fix code to check
  whether the assertion actually discriminates.
- Lower-bounded collections (`[1..*]`, `[2..5]`) still raise
  `TypeMismatchError: expected float, got <unset>` because the `UNSET` sits in the list *elements*,
  which the per-item decoder sees. If a change claims "collections holding no value read empty",
  probe these two shapes explicitly.

**Cheap non-vacuity control without touching the checkout** (no worktree, no reinstall): copy the
package next to your scratch script, swap in the parent revision's file, and re-run the same script
with `PYTHONPATH`:

```bash
cp -r clients/python/opensysml /home/ubuntu/scratch/prefix/
git show <fix-sha>^:clients/python/opensysml/typed.py > /home/ubuntu/scratch/prefix/opensysml/typed.py
PYTHONPATH=/home/ubuntu/scratch/prefix /home/ubuntu/pv/bin/python run.py   # must fail where the fix bites
```

Client API traps for these harnesses: `Connection` has **no** `evaluate()` — the expression escape
hatch is `Model.eval("1 + 1")` (or `connection.eval`). And an `Instance` is not iterable over feature
*names*: `{k: inst[k] for k in inst}` dies with `TypeError: bad argument type for built-in
operation`; list the names yourself.

A cyclic derived attribute is the negative control that a no-value change did not swallow real
evaluation errors:

```sysml
part def Loop { attribute a : Real = b + 1.0; attribute b : Real = a + 1.0; }
```

reading `a` must still raise `FeatureValueError: … cyclic feature value dependency: Loop.a`, and the
service must still answer `model.eval("1 + 1")` afterwards.

## `sysml-grpc -transport {grpc|connect|stdio}` (transport prototypes, PR #563 class)

`cmd/sysml-grpc` can serve three transports. Test each against the *same* build so a
difference is the transport and not the service.

```bash
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH   # grpcurl lives in ~/go/bin
make build-grpc && cp bin/sysml-grpc ~/.opensysml/bin/sysml-grpc   # never test a stale copy
pkill -x sysml-grpc                                # -x, never -f
```

The Python client needs a protobuf the ambient interpreter does not have. The blueprint
provisions `~/pv`, but it can be stale — check before trusting it, and reinstall if needed:

```bash
~/pv/bin/python -c "import opensysml, grpc, google.protobuf as p; print(p.__version__)" \
  || ~/pv/bin/pip install -e clients/python/
```

Client API names that are easy to guess wrong: `Connection(port=…, auto_start=False)`,
then `server_info()`, `load_from_content(content)` → a `Model` whose hash is `m.hash`
(not `model_hash`), `eval(expr, model_hash)`, `query(model_hash, …)`. There is no
`get_server_info`/`parse_file`.

### Connect transport

#### Diagnostic codes across Connect and Python gRPC

For local wire checks, rebuild `make build-grpc`, run
`bin/sysml-grpc -transport connect -port 50123 -health-port 0`, and attach the
editable Python client with `opensysml.connect(port=50123, auto_start=False)`.
Set `OPENSYSML_BINARY` to the absolute rebuilt binary; confirm the startup log's
commit rather than trusting a cached auto-started service. This recipe disables
the deprecated separate health listener.

- POST `ParseSources` with `documents: [{name, content}]`, not `sources`. Feed its
  `modelHash` to `ExecuteAction` (`actionSymbolId`) or `ExecuteState`
  (`stateMachineSymbolId`, `events`). Check `diagnostics` is nonempty before
  asserting exact `code`, severity, and span. Compare parsed Connect JSON with
  `MessageToDict` of the real gRPC response.
- Fixtures using Integer need `private import ScalarValues::*;`. Execution can
  still run when parse diagnostics contain unresolved types, so explicitly assert
  the runtime fixture parsed without errors before relying on its run.
- Inspect public Python return shapes before claiming runtime notes are exposed:
  successful `execute_action`/`execute_state` wrappers may return only outputs or
  state data. For wire-specific coverage use the live `connection._stub` response,
  wrap each protobuf diagnostic with `opensysml.diagnostic.Diagnostic`, and assert
  `.code` and `repr`. Report this as raw-stub/wrapper coverage, not as successful
  public execution methods exposing notes. Public `load_from_content` diagnostics
  and strict `ModelError.diagnostics` can be tested without private APIs.
- Keep empty-code compatibility distinct from live emissions: if no current
  producer emits uncoded diagnostics, Go `protojson.Marshal` of an uncoded
  generated Diagnostic plus Python JSON/binary decoding checks omission/defaults
  only. Do not describe that as a real uncoded RPC response. Unknown future codes
  should roundtrip unchanged.
- A divide-by-zero guard after the selected branch is a preview note
  (`guard-unevaluable`); the same failure in the first evaluated guard is a run
  error, not that note. This is a useful negative control.

##### Devin Secrets Needed

None for these local service/client checks.

`-transport connect -port P` puts gRPC, gRPC-Web, the Connect protocol, reflection and
`/health` on port P. Note the separate `-health-port` server **still binds** in this mode, so
`/health` answers on both ports; "the 8081 port stops being necessary" is a statement about
future work, not something the flag already does.

Probes worth running, in this order:

```bash
grpcurl -plaintext localhost:P list                       # reflection
grpcurl -plaintext localhost:P list sysml.SysMLService | wc -l   # RPC count (15 today)
curl -s --http1.1 -i -X POST http://localhost:P/sysml.SysMLService/ParseFile \
  -H "Content-Type: application/json" -d '{"content":"package Demo { part def Rover { attribute mass = 12.5; } }"}'
curl -s -o /dev/null -w '%{http_code}\n' -X POST \
  http://localhost:P/sysml.SysMLService/GetServerInfo -H "Content-Type: application/grpc-web-text"
```

Expectations that are easy to misread as bugs:
- An `int64` over Connect-**JSON** is a JSON **string** (`{"result":{"intValue":"7"}}`); over
  Connect-**protobuf** it is a number. Both are correct.
- A failed call over Connect-JSON is an HTTP status plus `{"code":"not_found","message":…}`,
  not gRPC trailers.
- `application/grpc-web-text` answers **415**: connect-go (checked at v1.20) ships
  `application/grpc-web`, `+json` and `+proto` but no base64 text variant. Confirm with
  `grep -rn "grpc-web-text" ~/go/pkg/mod/connectrpc.com/connect@vX.Y.Z/`.
- gRPC-Web binary needs the 5-byte length prefix (`struct.pack(">BI", 0, len(msg)) + msg`) and
  answers a message frame plus a trailer frame carrying `grpc-status: 0`.

`ParseFile` of `package Demo { part def Rover { attribute mass = 12.5; } }` hashes to
`6245ef4849a9019f3cbc64b03a54880f61d0fc3178f9be9d204d026f8007e78d` on every transport — a
cheap cross-transport identity check.

### stdio transport

`-transport stdio` serves one client over pipes with LSP framing. Drive it from Python with
three pipes and a stderr drainer (an undrained stderr can deadlock the child), and parse
stdout **strictly** — read the headers, then exactly `Content-Length` bytes. That strictness is
what proves the "stdout carries frames only" claim: run with `-log-level debug`, issue N calls,
close stdin, and assert the remaining stdout is empty.

Two encodings:
- `Content-Type: application/json` with a JSON-RPC 2.0 envelope (`jsonrpc`/`id`/`method`/`params`).
- `Content-Type: application/proto` plus `Sysml-Method` and `Sysml-Id`; the body is the bare
  protobuf request. Replies carry `Sysml-Id`, `Sysml-Status-Code` and, on failure,
  `Sysml-Status-Message` with an empty body.

Status codes are the gRPC ones: unknown `modelHash` → 5 (`NotFound`), unknown method → 12,
decode failures → 3. Compare any surprise against the same call over gRPC before calling it a
bug — e.g. `Query` with no `query` field is `InvalidArgument "query is unset"` on *both*
transports, and unknown JSON fields are silently discarded (`DiscardUnknown`), so
`{"bogusField":1}` surfaces as the *next* validation error, not as an unknown-field error.

Distinguish two classes of bad input, because they behave differently by design:
- **Body-level** (non-JSON body, `jsonrpc:"1.0"`, unknown method, wrong-typed field) — answered
  with an error frame; the session continues, and a following valid call must still succeed.
- **Framing-level** (missing/unparseable/negative `Content-Length`, a header line with no
  colon, a truncated body) — fatal: the process logs `stdio session ended in a protocol error`
  to stderr and exits **1**. That is not a crash; check for `panic` in stderr to tell them apart.

An oversized `Content-Length` is rejected before any allocation (`… exceeds the
134217728-byte limit`), so watch for a *fast* exit rather than for RSS growth — the process is
usually gone before `/proc/<pid>/status` can be read.

Concurrency is the headline claim: pipeline a large `Query` (use the 73 KB
`examples/pilot-corpora/sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml`)
followed by a cheap `Evaluate` and `GetServerInfo` **without reading in between**; the small
answers arrive first, so ids are mandatory. Closing stdin ends a healthy session with exit 0.

### The benchmark harness

`~/pv/bin/python clients/python/scripts/bench_transports.py --iterations 30 --spawns 3 --json out.json`
runs in a couple of minutes and reproduces the published *shape*: large-model `Query` costs
~6-7 ms in protobuf on all transports and ~40-47 ms in JSON (≈6-7×, serialization CPU, not
bytes), cold start is ~4 ms for stdio vs ~6-8 ms over TCP, and every small-payload cell has an
sd of the same order as its p50 — quote those as noise, never as a per-call transport win. The
reported payload sizes (467,971 proto / 513,339 JSON for the large `Query`) are stable enough
to diff against a document's table verbatim.

## Driving state-machine completion (`then done;`) and its surfaces

Use `%send <SignalName> [to <object>]` followed by `%step` to drive signal transitions.
The signal lists in `internal/core/runtime/testdata/conformance/*.expected.json` belong to the
conformance harness and are not automatically injected by the REPL. Alternatively, write
fixtures with **timed triggers** (`state a; accept after 5 then done;`)
and step them with `%advance <t>`; each region can be given
a different delay so a partial configuration is observable. `sysml <model> -state <name>` only
*starts* the executor and prints the initial configuration — it does not run to completion, so use
the REPL for completion claims.

Completion (a transition whose endpoint is the unqualified `done`) shows up as, in order,
`Current state: done`, a blank line, and ``✓ State machine completed (a transition reached `done`)``;
`%current` then reports `Execution state: Completed`. Useful shapes when testing it:

- **Exit action of the completing state:** give the source state `exit assign log := "...";` and read
  `State data:` in `%current` after completion — that proves entering `done` ran the exit behaviors.
- **Orthogonal machines** must stay `Running` while the configuration is e.g. `done | rstart`, and
  only report completion at `done | done`.
- **A machine declaring its own `state done;`** resolves the endpoint to that state: it is entered as
  an ordinary state (`Running`) and can transition onward. This is the key "not silently completed" case.
- **A qualified endpoint (`Other::done`) or `done` as a transition *source*** is a typed error at
  executor creation (`lower state machine: transition endpoint … names a state that is not a vertex of
  this state machine`, or `no initial state found`) — never a panic and never a completion.
- **Nested orthogonal regions are the discriminating shape.** A `done` reached inside a region owned by
  a *composite* state (so `graph.TopRegions` does not describe it) must **not** complete the machine
  while a sibling region is still active; `completeIfDone`/`machineComplete` walk outward through each
  enclosing region set (`RegionOwner`/`CompositeStates`) and require every concurrent region to have
  completed. An implementation that only checks `graph.TopRegions` completes early and still prints the
  same completion line, so always drive the fixture with *staggered* `accept after N` delays and assert
  `Execution state: Running` at the first completion:

  ```sysml
  state def M { entry; then outer;
      state outer parallel {
          state r1 { entry; then x; state x; accept after 5 then done; }
          state r2 { entry; then y; state y; accept after 7 then done; } } }
  ```
  `%advance 5` → `done | y` + `Running`, no completion line; `%advance 2` → `done | done` + `Completed`.
  Add a *two-level* variant (a `parallel` state inside a region of another `parallel` state) whose inner
  region finishes **after** an outer sibling region, so the walk is exercised in both directions and
  order-independently. Contrast against a binary from the commit before the fix — the buggy behaviour is
  a completion line at the *first* region's timestamp with the sibling still in its own state.
- `then done;` in an **action** body is unrelated: it is the action final node, checked with
  `sysml <model> -action <name>` (expect `Action completed` and the result values).

### Seeing a state view (`completes` detail)

The `(initial)` / `(completes)` state details only appear in a **state rendering**. A view is only a
state rendering if it specializes the standard view definition by its qualified name:

```sysml
view MView : StandardViewDefinitions::StateTransitionView { expose M; }
```

`view def StateTransitionView;` declared locally, or `render asTreeDiagram;`, both fall back to the
tree rendering, which prints only `state a` and hides the detail. `render Views::asTreeDiagram;` is
the correct spelling for a rendering reference (bare `asTreeDiagram` is unresolved).

### Removed state-body notation

`state def S { state s; final s; }` is a **parse** error, reported identically at every surface:
CLI `-validate` → `4:9: error: expected a body member` with a caret under `final` and exit 2; the REPL
prints the same located error for `%load` and for a typed-in declaration; `bin/sysml-lsp` publishes
severity **1**, code `syntax`, on the `final` token's range. `final` itself is unreserved, so
`attribute final : ScalarValues::Boolean;`, `state final;` and `transition first final … ` still parse
(they only trigger the ordinary duplicate-name warning when both are declared). A cheap non-vacuity
check for a removed diagnostic is `strings bin/sysml | grep -c '<removed message fragment>'` compared
against the contrast binary.

## Capability availability and the test-only withholding switch

`sysml-grpc` reports 14 capability names from `internal/grpc/service.go` (`capabilities`), and
`OPENSYSML_TEST_WITHHOLD_CAPABILITIES=<comma list>` makes a hand-started service behave as a build
that lacks them. The switch is validated at startup: an unknown name aborts with
`level=ERROR msg="Invalid service configuration" error="unknown capability \"…\""` and exit 1 (grep
for `unknown capability` without the quotes — the log escapes them), while empty/whitespace/`,,`
values withhold nothing. That startup refusal is the cheapest proof the switch is not a no-op.

The three classes behave differently and each needs its own service:

- **Unconditional request gates** (`convert`, `verification`, `query`, `apply_edits`) — the RPC
  fails `UNIMPLEMENTED: capability "<name>" is unavailable`.
- **Conditional request gates**, only when a field is set: `strict_conformance`
  (`ParseFile.strict_conformance`), `inline_language` (inline `content` + non-empty `language`),
  `oslc_query` (`QueryRequest.oslc_query`), `authoring` (an `add_member`/`delete` edit),
  `evaluate_subject` (`EvaluateRequest.subject_symbol_id`). The unset-field form of each must still
  succeed — that pair is the assertion worth writing, since a gate applied unconditionally passes a
  naive "is it refused?" test.
- **Response-only** (`type_facts`, `symbol_attributes`, `feature_values`, `enum_values`,
  `unset_value`) — no RPC is refused; fields are dropped or downgraded in
  `internal/grpc/capability_response.go`. Only assert these against a **default service snapshot of
  the same model**, otherwise "absent" proves nothing.

Fixture shapes that actually exercise the value filters: an `enum def` plus
`attribute color : Color = Color::red;` and a valueless `attribute pending : Real;` inside a part
def, then `Instantiate` the part — `feature_values["color"]` is an `enum_literal` and
`["pending"]` is `unset` by default, and become `null: "unsupported: enumeration literal"` /
`"unsupported: unset value"` when withheld. Note that **`SymbolInfo.attributes` carries no value for
an enum default** (const folding does not produce one), so a symbol-attribute assertion there is
vacuous; use `Instantiate` or `Evaluate` (`Evaluate` of `Color::red` also flows through
`valueToProto`) instead.

Driving it: start services with `-port 0 -health-port 0 -report-address` and read the address off
stdout, so several differently-configured services coexist. Raw `sysml_pb2_grpc.SysMLServiceStub`
calls are required to see a refusal at all — the Python client preflights against the cached
`ServerInfo` and raises `MissingCapabilityError` with `__cause__ is None` before any RPC. To observe
the **service-side** translation, simulate a stale handshake: overwrite
`conn._server_info` with a `ServerInfo` that still claims the capability, then call `conn.query(...)`
— the refusal becomes `MissingCapabilityError` with the `grpc.RpcError` kept as `__cause__`. An
UNIMPLEMENTED unrelated to capabilities is reachable without touching product code by calling a
method that does not exist on the live channel inside `translate_rpc_errors()`; under the default
`connect` transport its details read `Received http2 header with status: 404`, and it must stay
`UnsupportedOperationError`. `conn.query(where=…)` takes a constraint object
(`{"property": "@type", "operator": "=", "value": "PartUsage"}`), not `{"@type": "PartUsage"}`.

`make conformance` is the committed gate for this area: it runs the suite twice, the second time
with `-withhold-capabilities strict_conformance,oslc_query`, and prints per-scenario
`the service does not report oslc_query, so the without-capability expectation applies`.

Trap when writing a Convert row: `-convert ttl`/`to_format="ttl"` refuses a model containing a calc
body or constraint member (`cannot convert the operator expr at …`), so use `to_format="kerml"` for
the "convert works" check and keep RDF out of capability fixtures.

## grpc-go made test-only; transport error-parity probes (PR #612)

Production code no longer imports `google.golang.org/grpc`: service errors are
`connect.NewError(connect.CodeX, ...)` (`internal/grpc/`), the legacy `-transport grpc`
server lives in `cmd/sysml-grpc/grpcserver.go` behind an interceptor translating
`*connect.Error` → grpc statuses, and `scripts/check-grpc-imports.sh` gates imports in CI.
When testing error-message parity across transports:

- Run two servers at once (`./bin/sysml-grpc -port A -health-port A2` and
  `... -transport grpc -port B -health-port B2`) and drive **both** with the same raw
  grpcio stub — connect's gRPC-compat protocol lets one client compare
  `(e.code().name, e.details())` tuples byte-for-byte.
- Canonical probes: `Query` with `model_hash="deadbeef"` →
  `NOT_FOUND` / `model not found: deadbeef`; `Query` with both `query` and `oslc_query`
  set → `INVALID_ARGUMENT` / `query and oslc_query are mutually exclusive`.
- `QueryRequest.query` is a `Query` **message**, not a string — build it with
  `opensysml.query.build_query(None, where={"property": "@type", "operator": "=",
  "value": "PartDefinition"})`. `oslc_query` is a plain string.
- To prove the connect-native (non-gRPC) error path, POST JSON to
  `http://localhost:A/sysml.SysMLService/Query` with `Content-Type: application/json`;
  in JSON the operator enum is spelled `PRIMITIVE_OPERATOR_EQUAL` and a value list is
  plain strings (`"value": ["PartDefinition"]`). Expect e.g. HTTP 404 with body
  `{"code":"not_found","message":"model not found: deadbeef"}`.
- `Model` exposes `.hash` (not `.model_hash`); query results are `QueryElement`s whose
  display string is `P::Vehicle (PartDefinition)` — there is no `.name` attribute.
- stdio framing check: send `Content-Length`/`Content-Type: application/json` CRLF
  frames; unknown method answers `error.code == 12` with
  `SysMLService has no method "X"` and the server keeps serving; closing stdin exits 0.
- The service RPC is `ParseFile` (field `content`), not `ParseModel`.

## Old-vs-new differential runs (behaviour-neutral perf PRs)

For any PR claiming behaviour neutrality, build a contrast binary without touching the
checkout: `git worktree add /tmp/oldmain origin/main && (cd /tmp/oldmain && go build -o
/tmp/old-sysml ./cmd/sysml)`, then diff outputs of `./bin/sysml` vs `/tmp/old-sysml`.

Pitfalls that produce *false* differences (each one cost real time):

- `-convert` diagnostics echo the `-o` path, so give both binaries the **same output
  basename** in different directories (`$W/A/o.ttl`, `$W/B/o.ttl`) and `sed -i
  "s#$W/[AB]/#OUT/#g"` the stderr before comparing.
- When a conversion is *refused* (exit 2, e.g. RDF write-back refusal) **no output file is
  written**. `cmp -s A B` then fails on two missing files and every refused file looks
  different. Compare hashes with an explicit absent sentinel:
  `ha=$(md5sum < A 2>/dev/null || echo ABSENT)`.
- Run per-format sweeps in isolated working dirs; concurrent sweeps sharing `/tmp` files
  clobber each other.
- `%features` needs `%instantiate <Def>` first, otherwise you only diff an error message.

`-memstats` is the cheap way to show a perf PR actually helps:
`./bin/sysml -validate -memstats <big model>` prints wall time, MiB allocated, allocation
count — compare against the old binary on the same file.

Useful corpora for a sweep: `examples`, `testdata`, `internal/repl/testdata`,
`internal/core/runtime/testdata/conformance` (~750–900 files, a few minutes per pass).

## Numeric display: which surfaces render a Real, and how to compare them

Real rendering is centralised in `runtime.FormatReal` (`internal/core/runtime/value.go`).
When a PR touches numeric display, every one of these surfaces must be checked, because
each has its own call site and they have drifted apart before:

| surface | how to drive it |
| --- | --- |
| evaluation result | `%eval <expr>` at the prompt, `sysml -e "<expr>" <file>` non-interactively |
| feature value listing | `%instantiate <Def>` then `%features <Def>` (nested parts render indented) |
| quantity magnitude + unit text | `%eval 0.0001 [SI::m]`, `%eval (3.0 [SI::m/SI::s]) ** 2.0` |
| execution trace | `%trace on`, then `%eval` — `[trace] eval operator …` lines carry their own formatter (`trace.go formatConst`) |
| simulation clock | `%state <m>` / `%advance <t>` / `%current` / `%step`, and `sysml -state <m> -advance <t> <file>` |
| action results | `sysml -action <name> <file>` → `Results:` block |

Values that actually distinguish a working formatter from a rounding one: `0.0001`
(two-decimal formatting shows `0.00`), `1.0/3.0`, `1.0e-7` (exponent form), `2.0` and `0.0`
(a whole Real must keep `.0` so it is not read as an Integer), `123456789.987654`,
`2.0**70` (`1.1805916207174113e+21`), and a **fractional `%advance 0.001`** — the clock is
the surface most likely to be missed. Also assert the negatives: Integers
(`%eval 42`, `%eval 2**40`) must stay free of a decimal point, and a unit exponent stays
integral (`(SI::m/SI::s)**2`, never `**2.0`) — that is a deliberately separate helper
(`quantity.go exponentText`), so a careless unification breaks it.

The cheapest strong evidence is an **A/B against the parent commit**:
`git worktree add /tmp/base <parent-sha> && (cd /tmp/base && go build -o /tmp/sysml-base ./cmd/sysml)`,
then run the same piped script through both and diff. Remove the worktree afterwards
(`git worktree remove /tmp/base --force`) so the repo is left clean.

Doc transcripts are part of the contract: `docs/project/demo.md` opens by claiming every
block was "pasted verbatim … character for character", and `docs/guide/*.md`,
`docs/reference/cli.md` embed sample output too. After a display change, sweep for stale
samples with
`grep -rEn '(= |Time: |Advanced to |at: )-?[0-9]+\.[0-9]{2}( |$)' docs/`
and reproduce any hit against the built binary — a PR that respells the guide can easily
leave `docs/project/demo.md` behind.

Two REPL pitfalls specific to this kind of session:

- **`%clear` drops the imports too.** After `%clear`, a quantity or library-typed `%eval`
  answers `error: no declarations loaded (literals work, but feature references need
  declarations)`. Re-`%load` a file that does `private import SI::*;` before continuing.
- **`%state` cannot reach a state nested in a package.** `%state Pkg::TrafficLight` answers
  `unresolved reference: Pkg::TrafficLight` (and then `no active state machine session` for
  every follow-up). Declare the `state` at the top level of the loaded file — that is also
  how the guide's samples are written.

## Content-only demo rewrites: separating a regression from a pre-existing limit

When a PR only rewrites `examples/*.sysml` notation (`then done;` for a standalone `done;`,
`entry`/`do`/`exit <action>`, `accept when <event>`, `assert constraint`, view-usage
reordering) the binary is unchanged, so any difference must come from the file. Isolate it
by running the *old* copy of the same file through the *same* binary:

```bash
# the branch point, not whatever origin/main has moved to since
git show "$(git merge-base HEAD origin/main)":examples/<file>.sysml > /tmp/old.sysml
printf '%%state Pkg::Machine\n%%quit\n' | ./bin/sysml /tmp/old.sysml > /tmp/old.out 2>&1
printf '%%state Pkg::Machine\n%%quit\n' | ./bin/sysml examples/<file>.sysml > /tmp/new.out 2>&1
diff -u /tmp/old.out /tmp/new.out
```

Compare the whole run, not its last few lines: a difference earlier in the output (a warning,
a diagnostic, a different starting state) is exactly the regression you are looking for. An
empty `diff` means the behavior is pre-existing and not the PR's doing — report it as a
walkthrough/claim gap rather than a regression.

Known shape worth expecting: `examples/phase-c-behavioral-bodies.sysml` used to validate
clean while none of its state machines (`PhaseC::Running`, `ConnectionStateMachine`,
`VehicleOperating`, `AutopilotMode`) could be started — `%state` answered `failed to create
executor: initialize state machine: no initial state found`, because those states declared
substates with no transition out of the entry action naming the one to start in. A guard over
an attribute with no value fails the same startup a step later (`eval guard of transition
Active -> Paused: unresolved reference: lowBattery`). Both are fixed in that file now, but a
demo whose walkthrough was written against a machine that never started is a shape to expect:
check the old copy before calling such a failure a regression.

The committed walkthroughs to diff a demo run against are
`examples/VIEWS-DEMO.md`, `examples/SOLVER-DEMO.md`,
`examples/disposal-robot-demo/README.md` and — for `action-executor-demo.sysml`, whose own
`examples/ACTION-EXECUTOR-DEMO.md` is only a pointer — `docs/guide/06-behavior.md`
("Token-flow patterns").

## Parse-memoization / "reuse across invocations" refactors in `internal/repl`

When a PR caches what command text parsed to (argument lists for `%calc`, the name a run
target is looked up by for `%calc`/`%action`/`%state`/`%instantiate`), repeated successful calls
prove almost nothing — a cache that wrongly held *evaluated values* would print the same `= 8`.
The shape that actually distinguishes working from broken:

- **Identical text, changed state.** `attribute k = 2;` → `%calc add k 1` → `= 3`; then type
  `attribute k = 10;` at the prompt (this *replaces* the member, `note: replaced attribute k`, no
  `%clear` needed) and re-run the byte-identical `%calc add k 1` → must be `= 11`. Then `%clear`,
  redeclare only the calc, and the same text must fail with
  `error: evaluation of argument "k" failed: unresolved reference: k`; declare `k` again and it
  works. A `= 3` after the redeclaration is the stale-cache signature.
- **Re-submit the target's definition** (`package 'My Pkg' { calc def add { … x * y … } }` →
  `note: added to the existing package 'My Pkg', replacing calc def add`) and re-run the
  identical `%calc 'My Pkg'::add 1 2` — the cached *name* must still resolve to the new symbol.
- **Spacing variants are distinct keys**: run `%calc add 5 3` and `%calc add 5  3` both; the echo
  line normalizes to `add(5, 3)` for both, so the check is only that the value is right.
- **Malformed input twice**, comparing each pair byte for byte: `%calc add (5 3` →
  `error: failed to parse argument "(5 3"`; `%calc add a=1 2` →
  `error: named arguments are not supported here; pass arguments positionally`; `%calc add 5` →
  `error: calc invocation failed: unbound parameter: calc add parameter "y" has no argument and no default`.
  A cached parse error must be reported identically, and `%eval 1 + 1` → `= 2` afterwards shows
  the session survived.
- **Pipe the whole script through the parent-commit binary too** (contrast recipe above) and
  `diff` the outputs — an empty diff over ~150 lines is the strongest "behavior unchanged"
  evidence and takes seconds. Fixture scripts used for this live under `~/fixtures/pr765/` when
  that session's box is still around; otherwise rebuild from the bullets above.

## The embedded stdlib snapshot: proving the fast path and the fallback are both live (PR #776)

`libs.SharedBase()` decodes `internal/core/libs/stdlib.snapshot` instead of parsing the 97 bundled
library files, and falls back to parsing when the snapshot's recorded digest, format version or
stream structure does not match. `bin/sysml -memstats -e '2+3' model.sysml` is the whole
instrument: the two paths differ by an order of magnitude in allocations, so the memstats line
tells you which one ran without any debug flag.

| path | how to force it | expected memstats (b7cfcf19) |
|---|---|---|
| snapshot | default, or `OPENSYSML_LIBRARY_PATH=<byte-identical copy of internal/core/libs/stdlib>` | 13–17 ms, ~67k allocations |
| parse fallback | `OPENSYSML_LIBRARY_PATH=<copy with one comment appended to any .kerml>` | ~70 ms warm / ~220 ms cold cache, ~455k allocations |
| structurally corrupt blob | worktree, truncate `stdlib.snapshot`, `make build-sysml` | `WARN stdlib snapshot unreadable … pack: corrupt stream`, then the fallback numbers |

- The unmodified-copy case is the one that catches a digest computed over the *path* instead of
  the *content*: it must be as fast as the default, not as slow as the edited copy.
- `cp -r internal/core/libs/stdlib /tmp/x` keeps LICENSE/NOTICE; only `.kerml`/`.sysml` enter the
  digest, so editing those is a no-op for the fallback test — append to a `.kerml`.
- **Byte flips inside the payload must be refused, not misread.** The header carries a CRC-32C over
  the stream, so flipping 64 bytes in the string table (offset ~200000 of the 3.4 MB blob) has to
  produce `WARN stdlib snapshot unreadable … checksum mismatch` and the fallback numbers. Before the
  checksum existed such a blob decoded at full speed and silently dropped
  `KerML::Kernel::Interaction` and `Connector::association` from `%search` — that is the failure
  this check exists for. `TestDecodeSnapshotRejectsCorruption` covers the same flips in-process.
- Differential battery that proved behavior-neutrality: run `-validate` over
  `examples/*.sysml` + `testdata/passes/*.sysml`, a piped REPL transcript (`%load` robot demo,
  `%search`, `%eval 1 [SI::m] + 2 [SI::m]`, `%instantiate`/`%features`, `%print`), `-e 2+3` and
  `-convert ttl -o /dev/stdout`, each with `echo "exit=$?"` appended, under snapshot / edited-copy
  (cold and warm `XDG_CACHE_HOME`) / unmodified-copy / merge-base binary, then `diff -r` the four
  output dirs against the snapshot one. 16 files, 59 KB, all identical at b7cfcf19.
- `Instance.features` key order from the gRPC client varies run to run on **both** old and new
  services (Go map iteration) — not a snapshot difference; compare as sets.

### Recording pitfalls seen while doing this
- xdotool cannot type the `✓` glyph the CLI prints; `grep -v '^✓'` typed on camera becomes
  `grep -v '^'` and filters everything. Filter on `'= 5'` / `'sysml:'` instead.
- Konsole starts at a small font: `Ctrl++` three times before recording makes memstats lines legible.
- The blueprint's venv is `~/pv` (see the maintenance block), but it can exist without the editable
  `opensysml` install; `~/pv/bin/pip install -e clients/python` (or a fresh `python3 -m venv`) takes
  under a minute and the non-tty one-shot `python script.py` with auto-start worked (35 ms connect).

## Library feature tiers on `%features`/`%eval` (PR #830) and headless screenshot fallback

`buildFeatures` now skips only the Kernel *frame* (`self`, `portions`, `timeSlices`, `snapshots`,
`startShot`, `endShot`, `incomingTransfers`, `outgoingTransfers`) and materializes Systems/Domain
library features (`Parts::Part` → `subitems`, `subparts`, `checkedConstraints`, `ownedPorts` …;
`ShapeItems::Box` → `shape`, `voids`, `isSolid = true`, `faces`, `edges` …). Things that only bite
when you try to prove this from the binary:

- **The differential is `%eval box.isSolid`**: merge-base prints
  `error: evaluation failed: member isSolid not found in instance`, the branch prints `= true`.
  Build the old binary in a `git worktree add /tmp/wt-old $(git merge-base HEAD origin/main)`;
  `make build-sysml` there and **copy `bin/sysml` out** before removing the worktree.
- **Every `part def` now lists ~13 inherited lines** (`ownedPorts = []` … `checkedConstraints = []`),
  so a "plain model shows only `x`" expectation is false by design. Assert `x = 1` is present and the
  frame names are absent (`grep -cwE 'self|portions|timeSlices|snapshots|startShot|endShot'` → 0)
  rather than asserting line counts. The robot demo's `%features Robot::fielded` grew from 87 to
  ~430 lines with 14 `… (listing truncated)` markers; still < 0.1 s.
- **`%eval box.shape` prints `[]` while `%features` prints `shape = <unset>`** for the same
  `[0..1]` feature (the evaluator returns an empty sequence; the listing renders the scalar). gRPC
  `Instance.features["shape"] is opensysml.UNSET`, `model.eval("Demo::box.shape") == []`. Accept
  either but note the split if the spec you're given says one word.
- **`subject veh : V = v;` binds since 60b8118f**: `%features` shows `veh = Instance(ID: n)` and
  `subj = Instance(ID: n)` (merge-base showed `veh = <unknown>`; before that commit `<unset>`, with
  only the `:>> veh = v;` spelling binding). `%eval r.veh` with a bare requirement name still fails
  (`unresolved reference: r`) — read it from `%features`.
- **Qualified valueless optional (9bbb2dcb)**: `%eval Q::names` is intercepted by the REPL
  (`error: "Q::names" has no value to evaluate`) on every build, so the evaluator fix is invisible
  there. Use an expression: `%eval Q::names->size()` → `= 0` on the fix, `usage Q::names has no
  value` before; required `Q::weight->size()` must keep the error on both.
- `%eval usage.x` with a bare usage name resolves only when the file has a single package;
  `examples/disposal-robot-demo/robot.sysml` needs `%eval Robot::fielded.mass`. Requirement usages
  are never resolvable bare (`unresolved reference: massLimit`) — spell them `Demo::massLimit.limit`,
  which then reports `usage Demo::massLimit has no value` (pre-existing).
- **`XDG_CACHE_HOME` cold/warm is a no-op on the default path**: the embedded snapshot serves the
  library and the dir stays empty. To exercise the on-disk cache, point `OPENSYSML_LIBRARY_PATH` at
  a copy of `internal/core/libs/stdlib` with a comment appended to a `.kerml`; the cold run then
  writes 97 `*.idx` files under `$XDG_CACHE_HOME/sysml-ls/libs` and the warm run must diff empty
  against both cold and the default-path transcript.
- `printf "$CMDS"` with `%load` in the variable is a format-string bug (`0ad …` on line 1): use a
  heredoc file and `./bin/sysml < cmds.txt`, or `printf '%%load …'`.
- `pip install -e clients/python/` into `~/pv` took ~1 min; `sysml-grpc -port 50123 -health-port 0`
  then `opensysml.connect(port=50123, auto_start=False)`. `Model.load` needs an **absolute** path
  (the service resolves relative paths against *its* cwd). `pkill -x sysml-grpc`, never `-f`.

### When the computer-use tool is unavailable (`enigo init failed`)
The recording tool needs the desktop; without it, still get PR-comment screenshots by rendering a
Konsole under Xvfb: `Xvfb :7 -screen 0 1600x1000x24 &`, `DISPLAY=:7 konsole --geometry 1600x1000+0+0
-p Font="DejaVu Sans Mono,11" -e /tmp/demo.sh &` (the script ends in `sleep 600`), then
`import -display :7 -window root shot.png`. Verify text landed with
`convert shot.png -threshold 50% -format '%[fx:mean]' info:` (≈0.02–0.06, not 0). Do **not** clean
up with `pkill -f demo.sh` from the same one-shot shell — it matches its own command line and kills
the shell before `import` runs; use `pkill -x konsole` and a separate call.

## `%state <machine>` alone: exhibitor lookup and its two traps (PR #845 class)

Since PR #845 the one-argument `%state <machine>` form walks the *held* objects for exhibitors of
the machine and attaches to the one found (`Debugging state machine "lp" exhibited by object #1
of "TA::Sys"`), refusing with a typed `ExhibitorsError` for zero or several. The load-bearing
probe is `internal/repl/testdata/exhibited_timer.sysml`: `%instantiate TA::Sys`, `%state lp`,
`%advance 2.5 [s]`, `%features #1` → `x = 0.6000000000000001`, `n = 3`; a detached run (the
pre-#845 behaviour, or any regression to it) leaves `x = 0.2`, `n = 1` while still printing
`Advanced to 2.5 (2 event(s) processed)`, so assert on `%features`, not on the advance line.
`%state #1` is the oracle: both forms must agree.

Two things that look like the several-exhibitor path but are not:
- `%instantiate TA::Sys` twice does **not** hold two objects — the second *supersedes* the first
  (`note: TA::Sys now denotes this object; object #1 is no longer named`), so `%state lp` attaches
  to the new one. Use two differently named usages (e.g. `nested_machine.sysml`'s `Fleet::rover` and
  `Fleet::driver`, whose `driver.r` is a second `Rover`).
- Nested parts count only once **materialized**. `%instantiate Fleet::rover` + `%instantiate
  Fleet::driver` then `%state Fleet::Rover::modes` attaches to `#1 of "Fleet::rover"` with no
  error, because `driver.r` has not been reached yet; run `%features Fleet::driver` first and the
  same `%state` refuses with `2 objects of this session exhibit … (#1 of "Fleet::rover", #4 of
  "Fleet::driver::r")`. The walk deliberately materializes nothing, so the answer follows what
  the session has reached; materialize first when a nested exhibitor is meant to count.

Zero exhibitors is type-aware: a `state def` bound through typed usages (`shared_machine.sysml`:
`exhibit state front : Blink`) refuses before any `%instantiate`, naming `"Shared::Lamp"`, and so
does any usage on the way to the body (`named_usage_machine.sysml`: `state spare : Blink; exhibit
state active ::> spare;` — `%state Relay::Beacon::spare` and `%state Relay::Blink` both refuse, then
attach to `active`). Only a `state def` no type exhibits (`performed_machine.sysml`: `Two::Check`)
still prints `Started state machine executor`. The CLI mirrors the REPL: `-state lp` without
`-instantiate` exits 2 with the same message prefixed `sysml:`.

Branch-moved trap: the lead may push a follow-up commit mid-run. Compare `./bin/sysml --version`
with `git rev-parse --short HEAD` before every batch and rebuild when they differ; the version
string is the only thing that tells a stale binary from a fresh one.

## `%state <Def> [<obj>]`: attach vs detached, and the typed-body exhibit form (PR #856 class)

The load-bearing distinction for exhibited-machine addressing is the first line `%state` prints:

- Attached to the object's *running* machine:
  `✓ Debugging state machine "<usage>" exhibited by object #N of "<obj>"` followed (when a
  definition was named) by `note: object #N ... already exhibits "<Def>", so this session attaches
  to that running machine rather than starting a second performance of it (as `%state <obj>` would)`.
- Detached second performance (correct for performed or unrelated machines, a bug for anything the
  object exhibits): `✓ Started state machine executor for "<Def>"` / `Performed by object #N of
  "<obj>", which exhibits no running machine of this kind`.
- Ambiguous one-argument lookup: `error: 2 objects of this session exhibit "<Def>" (#3 of "...",
  #1 of "..."), so naming the machine alone attaches to none of them: use %state <object> or %state
  <Def> <object> to name one`.

Cover all three shapes of exhibit, since they resolve through different paths: the definition
bound by name (`exhibit state m : Mission;`), the body stated inline with no type (`exhibit state
modes { ... }`, `nested_machine.sysml`), and the usage **typed by a definition while stating its own
body** (`exhibit state m : Mission { state extra; }`, `typed_body_machine.sysml`). The last one is
matched through the usage's type (`ObjectBehavior.kinds`), and also through the definitions that type
specializes: with `state def Blink :> Base` and `exhibit state tuned : Blink { ... }`, `%state Base
obj` attaches to `tuned`, while an unrelated def still starts detached.

Proof of "no double entry": pick a fixture whose entry action writes a slot (`typed_body_machine.sysml`
writes `tank.m.log`/`level`), read `%features <obj>.<usage>` before and after `%state`, and confirm
the values did not accumulate (`"W"`/10, not `"WW"`/20). `%current` on the attached session prints
the same `State data:` slots, tying the debugger to the object's instance.

Two distinct exhibitors: do not instantiate the same usage twice (it supersedes). Use a model with two
different parts typed by the same machine def so `%state <Def>` alone is refused naming both.

Contrast binary: `git worktree add /tmp/wt-old $(git merge-base HEAD origin/main)`, `make build-sysml`
there, copy `bin/sysml` to `/tmp/old-sysml`, then `git worktree remove --force /tmp/wt-old`. The CLI
form `./bin/sysml -instantiate Fleet::tank -state "Fleet::Mission Fleet::tank" <file>` is the
single-frame comparison: the attach note versus "Started ... exhibits no running machine of this kind".

Konsole `clear` wipes the scrollback, so a `%state` result that scrolls off before the screenshot is
gone; take the screenshot before the next long `%features`, or start the next REPL without `clear`
and use <kbd>Shift</kbd>+<kbd>PageUp</kbd>.

## `%state <Def> [<obj>]`: attach vs detached, and the typed-body exhibit form (PR #856 class)

The load-bearing distinction for exhibited-machine addressing is the first line `%state` prints:

- Attached to the object's *running* machine:
  `✓ Debugging state machine "<usage>" exhibited by object #N of "<obj>"` followed (when a
  definition was named) by `note: object #N ... already exhibits "<Def>", so this session attaches
  to that running machine rather than starting a second performance of it (as `%state <obj>` would)`.
- Detached second performance (correct for performed or unrelated machines, a bug for anything the
  object exhibits): `✓ Started state machine executor for "<Def>"` / `Performed by object #N of
  "<obj>", which exhibits no running machine of this kind`.
- Ambiguous one-argument lookup: `error: 2 objects of this session exhibit "<Def>" (#3 of "...",
  #1 of "..."), so naming the machine alone attaches to none of them: use %state <object> or %state
  <Def> <object> to name one`.

Cover all three shapes of exhibit, since they resolve through different paths: the definition
bound by name (`exhibit state m : Mission;`), the body stated inline with no type (`exhibit state
modes { ... }`, `nested_machine.sysml`), and the usage **typed by a definition while stating its own
body** (`exhibit state m : Mission { state extra; }`, `typed_body_machine.sysml`). The last one is
matched through the usage's type (`ObjectBehavior.kinds`), and also through the definitions that type
specializes: with `state def Blink :> Base` and `exhibit state tuned : Blink { ... }`, `%state Base
obj` attaches to `tuned`, while an unrelated def still starts detached.

Proof of "no double entry": pick a fixture whose entry action writes a slot (`typed_body_machine.sysml`
writes `tank.m.log`/`level`), read `%features <obj>.<usage>` before and after `%state`, and confirm
the values did not accumulate (`"W"`/10, not `"WW"`/20). `%current` on the attached session prints
the same `State data:` slots, tying the debugger to the object's instance.

Two distinct exhibitors: do not instantiate the same usage twice (it supersedes). Use a model with two
different parts typed by the same machine def so `%state <Def>` alone is refused naming both.

Contrast binary: build one from the last commit *before* the change under test — the PR's merge base
while the PR is open, but once the fix is on `main` that is `<fix-commit>^` (for the typed-body fix,
`a21b6d77^`), since a merge base against current `main` already contains it and both binaries attach.
`git worktree add /tmp/wt-old <sha>`, `make build-sysml` there, copy `bin/sysml` to `/tmp/old-sysml`,
then `git worktree remove --force /tmp/wt-old`. The CLI form `./bin/sysml -instantiate Fleet::tank
-state "Fleet::Mission Fleet::tank" <file>` is the single-frame comparison: the attach note versus
"Started ... exhibits no running machine of this kind".

Konsole `clear` wipes the scrollback, so a `%state` result that scrolls off before the screenshot is
gone; take the screenshot before the next long `%features`, or start the next REPL without `clear`
and use <kbd>Shift</kbd>+<kbd>PageUp</kbd>.

## Structured runtime values: Array, vector, vector quantity (PR #883)

- **Package-level attributes are the surface for Array/vector-quantity values.** `%eval P::grid` on
  `attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }` (with
  `private import Collections::*; private import ScalarValues::*;`) prints
  `Array(2, 3)[1, 2, 3, 4, 5, 6]`, and `.rank`/`.flattenedSize`/`.dimensions`/`.elements`,
  `P::grid#(2, 1)` and `CollectionFunctions::'array#'(P::grid, (2, 1))` all read from it. On an
  **instantiated part**, the same Array attribute renders as `grid = Instance(ID: n)` with nested
  `dimensions`/`elements` lines in `%features`, and `%eval Def::grid` on the instance answers
  `Instance(ID: n)` — not `Array(2, 2)[…]`. Do not read that as the Array kind being broken; it is
  the instance-materialisation path, unchanged from before, and worth flagging as a gap rather than
  a regression. Vector (`⟨1.0, 0.0, 0.0⟩`) and vector-quantity (`⟨2.0, 4.0⟩ [m]`) attributes do
  render structured in `%features`.
- **A qualified package attribute inside a compound `%eval` does not resolve** (`%eval V::vq * 2`
  → `unresolved reference: V::vq`) on both the new and the merge-base binary, while `%eval V::vq`
  alone works and `%eval P::grid#(1, 3)` works. Put arithmetic on quantities/vectors into an
  attribute's own `= expr` and `%eval` the attribute, or write the operands inline
  (`%eval 3 * VectorFunctions::VectorOf((1.0, 2.0))`).
- Vector quantities need `private import Quantities::*; private import SI::*;` (ISQ optional) for
  `2 [m]`; `VectorFunctions::cartesianZeroVector` needs `VectorValues::*` imported. A vector is
  `==`-distinct from a sequence (`VectorOf((1, 2, 3)) == (1, 2, 3)` is `false`) — a cheap on-camera
  proof that `⟨…⟩` is not a pretty-printed `[…]`.
- Adversarial Array probes that must stay typed errors: `#(0, 0)`, `#(3, 1)`, `#(1, 2147483648)`
  (index out of range), one index into rank 2 (multiplicity violation), a literal past int64
  (arithmetic overflow), `elements` shorter than `dimensions` (multiplicity violation naming the
  flattenedSize), `dimensions = (0, 3)` / a negative dimension (Positive typing). Dimensions whose
  product overflows int64 (`(4611686018427387904, 4)`) must say "flattenedSize … exceeds the Integer
  range" (arithmetic overflow), never a wrapped `flattenedSize 0`. Writing a two-component vector to
  a feature typed `CartesianThreeVectorValue` (or a def specializing it) must refuse with
  `it declares dimension = 3`; `grid.label` on an `attribute def … :> Array` with its own members
  must answer the member, not `array has no feature label`.
- gRPC: a vector/vector-quantity feature crosses as `kind=null` with
  `unsupported: vector ⟨1.0, 0.0, 0.0⟩` / `unsupported: vector quantity ⟨2.0, 4.0⟩ [m]`; assert on
  `inst.get_feature(name).value.WhichOneof('kind') == 'null'` and the text, since the Python
  attribute access (`inst.dir`) raises `FeatureValueError` by design. `m.eval('2 [m] * 3')` on the
  model fails with `unresolved unit m` (model-scope eval sees no SI import) — use unitless
  expressions (`m.eval('3 * 4')`) as the service-survival check. A throwaway venv
  (`python3 -m venv ~/pr-venv && ~/pr-venv/bin/pip install -e clients/python`) is ~1 min; drive the
  freshly built `./bin/sysml-grpc -port 50123` with `auto_start=False`.

## Sets, tensors, and capability refusal

- Use `conformance/fixtures/set_tensor.sysml` for an m-based rank-three tensor and
  a duplicate/out-of-order set. Evaluate `s.elements`, not `s`, to obtain the
  `SetValue` rather than the Collection object. `TensorQuantity.dimensions` is
  `(2, 2, 2)` and all eight `components` should retain unit text `m`.
- If a qualified compound expression such as `T::cube#(2,1,2)` reports an
  unresolved reference after importing quantity libraries, pin the scope with
  `%eval in T : cube#(2,1,2)`. In noninteractive `-e`, use `cube#(2,1,2)`
  in the file's package context; `-e 'in T : ...'` is not REPL-command syntax.
  Python's corresponding option is `context_symbol_id="T"`.
- SysML tensor indices are one-based, while Python `cube[(1,0,1)]` is zero-based;
  both address the sixth component of shape `(2,2,2)`, expected `6.0 m`.
- For a real capability-refusal check, run a second freshly built service with
  `OPENSYSML_TEST_WITHHOLD_CAPABILITIES=set_values,tensor_values`
  and `-port 50124 -health-port 0`. Confirm absent capabilities, a working scalar
  evaluation, then `MissingCapabilityError` for direct and nested outbound values.
  Count delegated `EvaluateCalc` calls to demonstrate refusal before sending;
  `Connection._stub` is a read-only property, backed by `_service`.
- Library-backed collection regression probes need valid declarations:
  `Collections::Array` requires dimensions matching its elements, and Map /
  OrderedMap elements must be `KeyValuePair` objects, not raw integers.
- In Konsole, use `Ctrl++` (not Ctrl+Shift++) to enlarge the font. Display version
  with shell `bin/sysml --version`; `%version` is not a REPL command.

## Region-order choice points through CLI and REPL

For an externally driven state fixture, use the unchanged model in the REPL:
`%state test::Machine`, `%send Go`, `%advance 1`, `%current`. An undeclared
signal can be matched by name; the REPL explicitly reports that fallback.
Turn `%trace on` before advancing to see the chosen order. Set `%schedule
seed:1` before restarting `%state` to replay from a fresh executor.

The CLI has no external event injection flag. For CLI coverage, create a
scratch analogue outside the repository, declaring `item def Go;` and adding
`entry { send new Go() to Machine; }` to one initial leaf. Preserve both
original accepting transitions. Bare `Go` and `Go()` may be rejected by
analysis; use standard constructor syntax `new Go()`.

Run `sysml -state test::Machine -advance 1 -trace <scratch.sysml>`.
`-schedule declared` and default/reverse should select declaration order;
`-schedule seed:1` should select the other order for the two-region fixture.
Compare two seed runs byte-for-byte. `-schedule explore -trace` should expose
both final writes and each witness's order, not merely report two runs.
Replace the second region's trigger with another signal for a load-bearing
negative control: one transition still fires, but no choice is reported.

CLI `-json` places runtime notes in `checks[].lines`, not `diagnostics`
(which is for model-analysis findings). Without `-trace`, assert the choice
count; with `-trace -json`, assert the full choice line. Do not expect the
gRPC `choice-point` diagnostic encoding on the CLI surface.

## The shared simulation clock: `%advance` across action and state debuggers, `-advance`, `final_time` (PR #136)

- Build both CLI and service via Makefile; stop any older service before attaching
  with `Connection(port=50051, auto_start=False)`. Check `server_info().version`
  and capability `final_time` before trusting response values.
- Use `internal/repl/testdata/timed_action.sysml`: start `Timed::pinger` and
  `Timed::listener`, step twice, advance 2 then 3. The action's `%step` wait hint
  reports its current clock; `%current` reports the state debugger only.
  Expect action count 1, listener pinged, Last event at 5. The listener has no
  transition to done and therefore settles rather than reporting Completed.
- For load-bearing scheduling evidence, run
  `clock_action_state_due_together.sysml` in fresh REPL sessions. Set `%trace on`
  and `%schedule reverse` or `declared` before `%action test::watcher`, then
  `%advance 5`. The action materializes beacon and its state machine itself:
  do not pre-instantiate it, which changes executor creation order.
  Reverse produces sawLit true; declared produces false, with a due-order trace
  at t=5. Merely seeing a choice line does not prove the scheduling policy works.
- Use `clock_action_accept_at.sysml` to test absolute waits: advance 3 reaches
  count 11 and parks at 8 (the intervening past instant 1 must not rewind).
  Advance 4.5 then 0.5 finishes with count 111. A single executor must not
  generate a due-order choice.
- High-level Python `execute_state` returns `final_time`; high-level
  `execute_action` returns outputs only. Inspect the raw ExecuteAction response
  for its final_time. Integer protobuf values use `int_value`, not integer_value.
  Follow timed RPCs with an untimed fresh run whose final_time must be zero.
- CLI exit matrix on `timed_action.sysml`: `-action Timed::pinger -state Timed::listener -advance 5`
  exits 0; `-action Timed::pinger -advance 2` exits 2 and names the wait at t=5.0; `-action` alone
  runs to completion (exit 0); `-advance` with no behavior, or a negative one, exits 2.

### Devin Secrets Needed

None for these local REPL/gRPC checks.
