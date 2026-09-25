---
name: testing-notation-warnings
description: How to end-to-end test a new `nonstandard-notation` warning in internal/check/passes/nonstandard_notation.go — which surfaces show it, how to build non-vacuous false-positive scans over the four OMG corpora, how strict-conformance escalation is observed, and how to referee the boundary against the pinned pilot validator.
---

# Testing a new `nonstandard-notation` warning

`internal/check/passes/nonstandard_notation.go` emits one code, `nonstandard-notation`, at
`LevelSyntax`. Each rule matches an AST node shape and reports
``<spelling> is an OpenSysML extension with no SysML v2 production: <advice>``. Testing a newly
added rule means proving four things: it fires at the user-facing surfaces, it does **not** fire on
the legal spellings, strict mode escalates it, and the pilot reference agrees on which side of the
boundary each spelling sits.

## The CLI strict flag is `-strict`, not `-conformance strict`

`bin/sysml` has **no** `-conformance` flag: asking for one prints
`flag provided but not defined: -conformance` plus the whole usage block and exits **2**, which
reads exactly like the model failing to analyse. `-conformance auto|default|strict` belongs to
`tools/referee/reject` (and the other pilot harnesses) only. The CLI spelling is `-validate -strict`,
the REPL's is `%strict on`, and the LSP's is the `strictConformance` initialization option.

## Severity-only claims must be tested against the pass tiers, not just the message

`nonstandard-notation`, `import-visibility` and friends live at `LevelSyntax`, and the pass
runner **skips the higher tiers when a lower tier errors** (AGENTS.md §4). A notation finding
therefore sets `Diagnostic.Notation`, which exempts it from that gate; without the flag, promoting
a syntax-tier finding to error silently deletes every name-resolution, type and constraint
diagnostic in the document. Test it like this, always against a `main` binary built from a
worktree so the comparison is real:

```bash
git worktree add /tmp/osm-main main && (cd /tmp/osm-main && go build -o /tmp/sysml-main ./cmd/sysml)
cat > /tmp/blast.sysml <<'EOF'
package Q { part def A; }
package R {
  import Q::*;          # the newly-escalated construct
  part x : NoSuchType;  # a higher-tier finding that must survive
}
EOF
/tmp/sysml-main -validate -debug /tmp/blast.sysml | grep -cE 'error:|warning:'   # e.g. 2
bin/sysml        -validate -debug /tmp/blast.sysml | grep -cE 'error:|warning:'   # 1 ⇒ regression
```

`-debug` is what makes this legible: it appends `[tier/code]` to every line, so you can see which
tier stopped producing output. A count that drops is a lost diagnostic, not a severity move.

## "No diagnostics lost" is not enough — also check what the model can still *do*

A severity move can preserve every diagnostic and still change behavior, because several gates
ask "is there an error?" independently of the pass runner. Grep for them before believing a
severity-only claim; as of wave 10C the predicate is `Diagnostic.Blocking()`
(`SeverityError && !Notation`) and the gates that use it are `Registry.Run`'s `hasError` and the
REPL's `hasError` + `analysisBlocked` (`internal/frontend/repl/render.go`), while these deliberately keep a
**raw** `SeverityError` check:

- `internal/frontend/repl/run.go`'s `Session.hasAnalysisErrors` → `HasErrors()`, which gates the CLI check
  flags (`cmd/sysml/check.go`), `%query` (`internal/frontend/repl/oslc_query.go`) and `LoadReport.Errors`
- `internal/check/edit/validate.go`'s `errorsOnly`

```bash
grep -rn 'SeverityError' --include=*.go internal/ cmd/ | grep -v internal/check/passes/
```

So test three distinct things, not one: (1) no diagnostic is lost, (2) the REPL still prints its
`✓ <member>` declared-members summary, and (3) the model can still be *used* — instantiated and
checked. A notation error that blocks (3) is a change to model use even though every diagnostic
is still reported.

The trap to watch for: a diagnostic that is **both** `Notation` and `SeverityError by default`
will disagree with `Blocking()` on every raw-severity gate. Find them with

```bash
grep -rn -B8 'Notation:\s*true\|Notation\s*=\s*true' --include=*.go internal/check/passes/ | grep -E 'Severity|func '
```

Historically `import-visibility` is the only such case (`nonstandard-notation` is a warning by
default and an error only under strict), which makes "a file whose only error is a bare `import`"
the single fixture that exposes this class of bug. Build it so the bare import is the *only*
finding — declare the imported package, and qualify library types as `ScalarValues::Natural`, or
an unresolved-reference error will mask the effect on both binaries:

```sysml
package Q { part def Helper; }
package P {
  import Q::*;                                    // the ONLY finding
  part def Widget {
    attribute mass : ScalarValues::Natural = 5;
    constraint massOK { mass > 3 }
  }
}
```

```bash
# must behave the same on main and on the branch in DEFAULT mode
$BIN -instantiate 'P::Widget' -constraint 'P::Widget::massOK' /tmp/barecheck.sysml; echo $?
# 0 + "✓ Constraint ... passed"  vs  2 + "did not analyse cleanly; no check was made"
```

Note the asymmetry that makes this subtle: `cmd/sysml/strict_test.go` legitimately requires a
**strict**-mode notation error to exit 2, so the raw check cannot simply be swapped for
`Blocking()`. Any fix has to distinguish default from strict rather than treating all notation
alike.

## Surface names in this repo (do not trust a brief's shorthand)

There is **no `-check` flag** and **no `%run` REPL command**, though both get referred to that way.
Verify with `sysml -h` and `%help` before building a plan around them:

- the "check" surface is the *check flags* — `-instantiate`, `-constraint`, `-satisfy`,
  `-requirement`, `-calc`, `-action`, `-state`, `-eval`, `-query` — whose refusal message is
  `"... did not analyse cleanly; no check was made"`
- the REPL has `%query` (OSLC), `%load`, `%print`, `%strict`, `%trace`, … and `%query` needs real
  OSLC syntax (`oslc.where=...`); a bare name is rejected by the parser *before* the error gate,
  which silently makes the probe vacuous

## Corpus differentials: one file per process, parallel, and prove non-vacuity

`-validate` accepts many files at once, but batching lets them resolve against each other and
invents cross-file diagnostics — keep one file per invocation and parallelize instead. At ~60 ms
per file, 429 files × 2 binaries × 2 modes finishes in well under a minute:

```bash
find examples internal/workspace/libs/stdlib -name '*.sysml' -o -name '*.kerml' | sort > /tmp/files.txt
xargs -a /tmp/files.txt -d '\n' -P8 -I{} /tmp/scan.sh /tmp/sysml-pr "" {} > /tmp/diag_pr.txt
```

Compare severity-insensitively (`sed -E 's/: (error|warning|note): /: /' | sort -u`) so a severity
move is not counted as a loss plus a gain. Two vacuity controls are mandatory, or the scan proves
nothing: the gained count must be **> 0**, and the OMG/stdlib roots must be demonstrably scanned —
report files-per-root alongside rows-per-root, because those roots are nearly clean and a broken
path glob looks exactly like a clean result.

## Driving the LSP surface without an editor

The third surface is worth a real check, and a 30-line JSON-RPC client is enough — it also proves
the strict option is read, which the CLI cannot:

```python
import json,subprocess,time
def fr(o):
    b=json.dumps(o).encode(); return b"Content-Length: %d\r\n\r\n"%len(b)+b
msgs=[{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":None,"rootUri":None,
        "capabilities":{},"initializationOptions":{"strictConformance":True}}},
      {"jsonrpc":"2.0","method":"initialized","params":{}},
      {"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{
        "uri":"file:///tmp/f.sysml","languageId":"sysml","version":1,"text":open("/tmp/f.sysml").read()}}}]
p=subprocess.Popen(["bin/sysml-lsp"],stdin=subprocess.PIPE,stdout=subprocess.PIPE)
for m in msgs: p.stdin.write(fr(m))
p.stdin.flush(); time.sleep(3); p.stdin.close()
for chunk in p.stdout.read().split(b"Content-Length: "):
    if b"publishDiagnostics" in chunk:
        for d in json.loads(chunk.split(b"\r\n\r\n",1)[1])["params"]["diagnostics"]:
            print(d["severity"], d["range"]["start"], d.get("code"))
```

LSP `severity` is **1 = error, 2 = warning**, so strict escalation shows up as the same spans
flipping 2 → 1. Omit `initializationOptions` for the default-mode run.

## Proving strict moves severity and nothing else

Diff the two runs with the severity word normalised; only the trailer may differ:

```bash
diff <(bin/sysml -validate f.sysml 2>&1 | sed 's/warning:/SEV:/') \
     <(bin/sysml -validate -strict f.sysml 2>&1 | sed 's/error:/SEV:/')
```

A clean diff over the diagnostic lines is the assertion — same message, same `line:col`, same
caret line. Eyeballing two screenshots is not.

## `TestStdlibConformance` is a parser-only gate

It parses the 95 embedded stdlib files and asserts zero **parse** diagnostics against
`testdata/stdlib_known_failures.txt`; it never runs the passes. So it is *not* evidence that a new
pass rule or a severity change leaves the stdlib alone. For that, grep the stdlib for the
construct directly (e.g. `grep -rlE '^[[:space:]]*import ' internal/workspace/libs/stdlib` returns 0
files, which is the real reason an import-visibility change cannot touch it).

## Recording the CLI/REPL surfaces

There is no xterm on the box; `konsole` is the terminal emulator that exists. Launch, focus and
maximise it before recording, then zoom the font so the long messages stay readable:

```bash
DISPLAY=:0 nohup konsole --hide-menubar --hide-tabbar --workdir "$PWD" -e /bin/bash &
DISPLAY=:0 wmctrl -a Konsole && DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Zoom in with `xdotool key ctrl+plus` (**not** `ctrl+shift+plus`, which types literal `+`
characters into the shell); ~112 columns is a good target. The diagnostic messages are ~180
characters and will wrap — that is fine, but it means a `clear` before each command is worth it.

## Surfaces that show the warning (and the one that is language-blind)

| Surface | Command | Notes |
|---|---|---|
| CLI, single file | `bin/sysml -validate <f>` | GNU-format `path:line:col: warning: …` plus a caret line. Exit **0** when warnings are the only findings, and it still prints `✓ <f>: no errors`. |
| CLI, strict | `bin/sysml -validate -strict <f>` | same lines as `error:`, then `sysml: <f> did not analyse cleanly; no check was made`, exit **2**. |
| REPL | pipe snippets into `bin/sysml`, then `%strict on` | The buffer is named `<repl>` and has **no file kind**, so this is the surface that proves the rule is not gated on the file extension. `%strict on` **re-reports the whole buffer** at the new severity, which is the cheapest strict-mode proof. |
| differential | `build/pilot-diff/pilot-diff.txt` | `opensysml:` message lines, grouped under root + file. |

**There is no `%validate` meta-command** — the REPL analyses on every submit, so just submit the
snippet and read the diagnostics. Asking for `%validate` prints
`unknown command "%validate" (try %help)`, which is easy to mistake for the rule not firing.

Driving the REPL non-interactively (`%%` is needed because `printf` eats a single `%`):

```bash
printf 'state def S { state a { defer Ping; } }\nconstraint k { in x; x >= 0 }\n%%strict on\n%%quit\n' | bin/sysml
```

## The notation pass runs even when the file has resolution errors

The passes are tiered, but `nonstandard-notation` is a **syntax-tier** rule, so it is emitted even
when the same file collects `unresolved reference: …` name-resolution errors. That matters a lot for
scanning: a bare `package P { … : Real … }` with no `private import ScalarValues::*;` yields
`unresolved reference: Real` **and** the notation warnings. So a single-file `bin/sysml -validate`
over a corpus file that imports siblings is still a valid surface for *presence/absence of a
notation message*, even though it is not a valid surface for the file's overall cleanliness.

Use an explicit `private import ScalarValues::*;` only when you need the file to analyse **cleanly**
(i.e. for the "exit 0 in default mode / exit 2 under `-strict`" assertion).

## False-positive scan over the four OMG corpora — and making it non-vacuous

The OMG-authored corpora are asserted clean, so **any** hit is a defect. Roots:
`examples/pilot-corpora` (includes `sysml-examples` and `kerml-examples`),
`examples/sysml-v2-training`, and the stdlib `internal/workspace/libs/stdlib`.

```bash
while IFS= read -r f; do
  bin/sysml -validate "$f" 2>&1 | grep -E "<the new message text>" | sed "s|^|HIT |"
done < <(grep -rl --include=*.sysml --include=*.kerml -E '(^|[^A-Za-z_])(return|assert|assume) ' \
           examples/pilot-corpora examples/sysml-v2-training internal/workspace/libs/stdlib)
```

- Use `grep -rl` on the *keyword* to pick candidate files (99 files for `return`/`assert`/`assume`),
  then grep the *validator output* for the message — grepping the source for the message is
  meaningless.
- Anchor the keyword grep with `(^|[^A-Za-z_])` or you match `returnValue`, `asserted`, etc.
- **A zero-hit scan proves nothing on its own.** Always run the identical `grep -E` against a
  known-positive file and show a non-zero count (`examples/repl-behavioral-demo.sysml` gave 12).
  Without that control, a typo in the message pattern looks exactly like a clean corpus.
- Independent cross-check: `awk` the differential report to attribute every occurrence of the new
  message to its root and file, and confirm no OMG root appears:
  ```bash
  awk '/^[a-z-]+ \(/{root=$0} /^  [^ ]/{file=$0} /<message>/{print root " ||" file}' \
      build/pilot-diff/pilot-diff.txt | sort | uniq -c
  ```

## The boundary fixture must contain the positive twins

A fixture of only-legal spellings that stays silent is indistinguishable from a rule that never
fires. Put the must-warn and must-stay-silent shapes in **one** file and assert the exact set of
warned line numbers. For the `return` / `assert` family the shapes that matter:

- silent: `return a;` and `return result : Real = a;` (result *parameter declarations*, not
  computed results), a keyword-less trailing expression `{ in x : Real; x * 2.0 }`, a keyword-less
  trailing condition `{ in x : Real; x >= 0 }`, `assert constraint c1 : C;`,
  `assert satisfy R by q;`, `assume #goal constraint m;`
- warned: a state's `defer Ping;`
- rejected outright (no longer a warning): `return a * 2.0;`, `return 42;`, `assert x >= 0;`,
  `assert not x < 0;`, `assume x >= 0;`, `require x > 0;` — a computed `return` expression and a
  keyworded inline condition are parse errors. So are the removed state-machine notations
  `initial a;`, `final b;` and `transition a to b;`; write `entry; then a;`,
  `transition first a then done;` and `transition first a then b;`

## Refereeing the boundary against the pinned pilot

`build/pilot-validator/validate-sysml <f>` (~15 s, capture `$?` **without** a pipe) is the oracle
for "does a production admit this spelling". The interesting asymmetry:

- pilot **exit 1** with `no viable alternative at input '<keyword>'` + our warning ⇒ correct.
- pilot **exit 0** + our warning ⇒ **false positive of the rule**, the worst outcome.
- pilot **exit 1** + our **silence** ⇒ a *coverage gap*: the reference has no production for it and
  we accept it silently. Not a regression, but exactly what such a PR is meant to remove, so hunt
  for these deliberately.

Note the pilot's syntax error recovery cascades (`missing '}' at 'a'`, `extraneous input '}'
expecting EOF`) and it also emits its own `Duplicate of other owned member name` warnings on
`calc c { in a : Real; return a; }` — judge the verdict by the presence of the
`no viable alternative at input '<keyword>'` line and the exit code, not by output volume.

## Requirement-body members are a *different* AST node from constraint-body members

The single most likely coverage gap when a rule covers "assert/assume conditions": a condition in a
**constraint** body is `*ast.ConstraintMember` (fields `Keyword`, `IsNegated`, `Expression`, `Name`,
`Body`), but the same-looking condition in a **requirement** body is `*ast.AssumeMember` /
`*ast.RequireMember` (`internal/syntax/ast/behavior.go`, "Phase C2: Requirement Body Members"), with a
different field set. A rule keyed on `*ast.ConstraintMember` therefore covers a constraint-body condition and
misses the requirement-body ones. The keyworded inline condition
(`constraint c { assert x >= 0; }`, `requirement r { require x > 0; }`) is now rejected by the
parser rather than warned, matching the pinned pilot's
`no viable alternative at input 'assert'` / `'assume'` / `'require'`; the placement rule for
`assume`/`require` outside a requirement body is a separate, retained warning. `examples/phase-c-behavioral-bodies.sysml`
contains both families, so always grep the whole keyword inventory of a fixture
(`grep -n 'assert \|assume \|require '`) and reconcile *every* line against warned/not-warned rather
than only checking the lines the task named.

## Verify oracle baselines against a LIVE run, not just against the docs

`tools/referee/diff/doc_counts_test.go` guards documentation prose against the **committed**
`docs/project/pilot-xpect-baseline.json` / `pilot-rejection-baseline.json`. It therefore cannot
notice that both the prose *and* the committed baseline have drifted away from what the code now
does — they stay self-consistent while both go stale, and `go test ./...` stays green. A change
that makes notation errors non-blocking moves the xpect numbers, because findings that used to be
gated now surface.

So always run the oracle and diff the totals yourself:

```bash
go run -C tools ./cmd/pilot-xpect -jobs 8
cmp build/pilot-xpect/pilot-xpect.json docs/project/pilot-xpect-baseline.json   # may legitimately differ
python3 -c "
import json
b=json.load(open('docs/project/pilot-xpect-baseline.json'))['totals']
l=json.load(open('build/pilot-xpect/pilot-xpect.json'))['totals']
[print(k,'baseline',b[k],'live',l.get(k),'<-- DIFFERS' if b[k]!=l.get(k) else '') for k in b]"
```

Watch `agree`, `disagree` and especially `foreignDiagnostics` — a drop of the last to 0 means
diagnostics that were previously attributed elsewhere are now landing on the expected rows.
`docs/project/pilot-xpect.md` carries the same numbers in prose (`agree N | disagree N | ...`),
so grep for the live figures there too. Report a mismatch as stale-baseline, not as a test failure.

### Attribute the drift before blaming the branch

A live-vs-committed mismatch does **not** by itself mean the branch under test moved the numbers —
the baseline may already have been stale on `main`. Always re-run the same oracle in a `main`
worktree before escalating, because "the branch broke the oracle" and "the baseline was already
stale" need completely different responses:

```bash
cd /home/ubuntu/wt-main && git log --oneline -1 && go run -C tools ./cmd/pilot-xpect -jobs 8
```

A worked example: xpect measured 939/387/0 live against a committed 845/481/18, which looked like a
branch regression — but `main` (`4b25f85b`) produced the *identical* 939/387/0 with the same
`files`/`assertions`/`rows`, so the drift predated the branch entirely. Identical secondary totals
alongside moved primary totals is the signature of a stale baseline rather than a provisioning or
behavior difference on the branch.

## A new notation code can add strict-mode errors to OMG-published files

A notation rule that re-derives its finding from the lexer's keyword set rather than from what the
parser recovered fires on legal text: `enum = 60 [mm];` inside an `enum def`
(`examples/pilot-corpora/sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml`)
and `attribute type : String[0..1];` in the bundled stdlib both became `-strict` errors that way,
refusing files that validate clean. `enum = <value>;` is an unnamed usage the parser models as a
member named `enum` (hence its `Duplicate of other owned member name` warning in every build), so a
keyword-as-name rule that trusts the tree inherits the mis-parse; `keywordAsName` now escalates only
the spans the parser itself reported as recovered. Treat any new notation code as suspect until you
have run it over the stdlib and all pilot corpora **in strict mode** and inspected each hit by hand:

```bash
sysml -validate -debug -strict "<file>"   # per file; gains under OMG/stdlib roots are the red flag
```

A gain under an OMG root or the stdlib is not automatically a bug, but it is never "expected" —
adjudicate it against the source text before calling it correct.

## A parser-gated escalation must be tested in BOTH directions

Once `keywordAsName` was narrowed to escalate only spans the parser reported
(`keywordNameSpans(ctx.ParseDiagnostics)` collecting `Code == CodeReservedKeywordName` offsets, then
`if !w.keywordName[id.NameSpan.Offset] { return }`), the rule has two distinct failure modes and the
obvious tests only catch one:

- **Over-broad** — the false positive survives. Caught by validating the reproducers.
- **Over-narrow / vacuous** — the span map is empty in the real pipeline, so *nothing* escalates.
  This is the dangerous one, because every "clean" result gets *better* as the fix gets more broken:
  the reproducers pass, the OMG files match `main`, and the corpus differential shows zero gains.

So always pin the positive case on the **real binary**, not only in unit tests:

```bash
G=tools/referee/reject/testdata/negative/grammar/g15-keyword-as-name.sysml   # part def part;
sysml -validate        $G   # expect: warning only at 3:14, exit 0
sysml -validate -strict $G   # expect: error at 3:14 + "did not analyse cleanly", exit 2
go run -C tools ./cmd/pilot-reject -conformance strict   # expect 116 both reject / 3 pilot-only
```

A strict run that produces no error, or a rejection count slipping to 115/4, means the escalation
has gone vacuous. Keep `k02-sysml-keyword-in-kerml.kerml` as the neighbouring control — its strict
error is `kerml-notation`, a *different* code, so it must stay byte-identical across the change.

The escalation depends on parser **warnings** reaching `ctx.ParseDiagnostics` with their `Code`
intact. Two forwarders do that, and if either dropped the code the rule would silently stop firing
everywhere:

```bash
grep -n "Code:" internal/workspace/model/workspace.go   # ~244, CLI + LSP path
grep -n "Code:" internal/frontend/repl/session.go           # ~555, REPL path
```

So a keyword-as-name check that passes only in `internal/check/passes` unit tests proves nothing about
the CLI, REPL or LSP — exercise at least one real surface.

## Every diagnostic from the notation pass is Notation, whatever its code

Do not build a hardcoded list of "notation codes" for a differential harness. The pass marks its
whole output in one loop (`for i := range w.diags { w.diags[i].Notation = true }`), so
`nonstandard-notation`, `kerml-notation`, `sysml-notation` **and** `reserved-keyword-name` are all
notation. A hand-maintained set will misclassify one of them and produce a false
"NON-NOTATION ESCALATION" alarm. Grep the pass instead:

```bash
grep -rn "Notation:\s*true\|\.Notation = true" --include=*.go internal/ | grep -v _test
grep -n "Code[A-Za-z]* =" internal/check/passes/nonstandard_notation.go
```

## Surface and flag names that waste time

- The CLI strict flag is **`-strict`**, not `-conformance strict`. The latter is a flag-parse error:
  it dumps usage and exits 2, which looks exactly like a legitimate refusal and will silently fake
  a "strict refuses" pass. Confirm with `sysml -h | grep -i strict`. (`tools/referee/reject` *does*
  take `-conformance default|strict` — the two binaries differ.)
- There is no `-check` flag and no `%run` command. Use the check flags
  (`-instantiate`, `-constraint`, `-satisfy`, `-query`, ...) and `%strict on|off`.
- REPL `%load` goes through `Session.LoadFile`, **not** `LoadFileSummary`, so it does not exercise
  `renderSyntax`. Only `sysml -validate <file>` (via `cmd/sysml/check.go`) does. If you are asked to
  prove a load-time rendering fix, `%load` is a regression control, not the discriminator — say so
  rather than reporting it as evidence of the fix.

## Measure "exactly once" against a build that gets it wrong

For a de-duplication fix, keep the pre-fix binary and count on all three builds
(`main`, pre-fix, fixed). A bare "1 occurrence" on the fixed build is vacuous — 1 is also what you
get if the diagnostic is emitted once for an unrelated reason, or if your grep pattern is too
narrow. The useful shape is baseline **1**, pre-fix **2**, fixed **1**:

```bash
for b in main prefix fixed; do
  echo "$b: $(/tmp/sysml-$b -validate /tmp/loadbare.sysml 2>&1 | grep -c 'visibility indicator')"
done
```

Also check *position*, not just count: the pathology is the diagnostic printed **before** the
`✓ package ...` summaries (i.e. as a reason the file could not be read) and then again after.

## Recording CLI/REPL work on this box

`DISPLAY=:0` (not `:1`; check `ls /tmp/.X11-unix`). Konsole's font shortcut `ctrl+shift+plus` types
literal `+` characters through xdotool instead of resizing, so set the font at launch and maximize
with wmctrl:

```bash
DISPLAY=:0 konsole --hide-menubar -p "Font=Monospace,15" --workdir /path/to/repo &
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Under `-strict` a refused file prints no `✓ package ...` summary at all — that is the refusal path,
not a regression of the declared-members summary. Compare summaries in **default** mode.

## De-duplicating a parser warning against a notation error: sweep every construct

`dropEscalatedWarnings` (`internal/check/passes/analyze.go`) drops a parse **warning** only where a
pass reported an **error** of the same code at the same span, after the whole registry ran. Filtering
by code alone — as `SyntaxPass.Run` once did — is asymmetric and is the thing to attack:

- the parser emits the warning from **one** general site, `parseIdentification()` in
  `internal/syntax/parser/namespace.go` (~line 222), so *any* construct whose name flows through it
  can produce the warning;
- the notation walker calls `keywordAsName` from only a handful of cases in
  `internal/check/passes/nonstandard_notation.go` — the `Ident` of `ast.Namespace`, `ast.Package`,
  `ast.Definition` and `ast.Usage`.

Any construct in the first set but not the second **loses its only diagnostic under strict**, making
strict *more permissive than default* — an inversion of the point of strict conformance. Do not try
to reason about this from the AST; sweep it. Generate one fixture per construct kind declaring a
reserved keyword as its name and compare warning/error counts across modes:

```python
CASES = {"package": "package part { }", "part def": "part def part;",
         "attribute": "attribute part : String;", "alias": "part def B; alias part for W::B;",
         "state usage": "action def A { state part; }", ...}
# for each: wrap in "package W { %s }", run `-validate` and `-validate -strict`,
# count lines matching 'reserved keyword' split by 'warning:' / 'error:'.
# Healthy dedup  => default 1w/0e, strict 0w/1e
# LOST diagnostic => default 1w/0e, strict 0w/0e   <-- bug
```

A ~20-construct sweep runs in seconds and localises the gap precisely. `alias` is the known offender
(both `.sysml` and `.kerml`); `doc`, `comment` and `dependency` never emit the warning at all, so a
`0w/0e` in both modes there is expected, not a loss. Note the escalated error and the dropped warning
may carry **different message text** for the same span, so match on a substring both share
(`reserved keyword`) rather than the full sentence.

## The corpora cannot test `reserved-keyword-name` at all — it is a vacuous gate

Before trusting a corpus differential as evidence for a keyword-as-name change, count the rows the
code actually produces across the corpora:

```bash
find examples/pilot-corpora internal/workspace/libs/stdlib -name '*.sysml' -o -name '*.kerml' \
  | xargs -P8 -I{} sh -c '/tmp/sysml -validate -strict "{}" 2>&1' | grep -c 'reserved keyword'
```

This returns **0** — no OMG-published or bundled file writes a reserved keyword as a name. A
"0 lost / 0 gained" corpus differential for such a change is therefore *structurally incapable* of
failing and must never be reported as evidence that no diagnostic was lost. Hand-written fixtures
plus the construct sweep above are the only real coverage. The same reasoning applies to the
corpus-gated `go test ./...`: it stayed fully green while strict silently dropped the `alias`
diagnostic, because no test pins that construct.

## Two corpus-wide invariants that CAN fail when the dedup is code-agnostic

The row-count differential is vacuous for `reserved-keyword-name` (above), but once the dedup stops
keying on a specific code — e.g. "drop a warning when some pass emitted an error with the same `Code`
at the same `Span.Offset`" — its blast radius becomes *every* code, and two corpus-wide properties
become both cheap and non-vacuous. Check them instead of, not in addition to, trusting row counts:

1. **Strict monotonicity.** For every corpus file, strict must never report fewer rows than default,
   and every span present in default must still be present in strict. This is the generalisation of
   the construct sweep to codes no hand-written fixture covers. A code-agnostic dedup that swallows
   a warning where no error replaces it shows up here as a vanished span.
2. **No span carries both a warning and an error** in a single run, in either mode.

Parse `^(.*?):(\d+):(\d+):\s+(warning|error):` from merged stdout+stderr, bucket by `(line, col)`,
and compare the default and strict sets per file. Expect `0` violations of both properties.

**Give property 2 a vacuity control, because "0 duplicates" is also what a broken detector prints.**
Point the *same* parsing logic at `g15-keyword-as-name.sysml` on a build that predates the dedup
(`85aa4140`): it must report exactly **1** duplicate span (`3:14` holding both `error` and
`warning`), and the build under test must report **0** on that same file. Without that column, a
regex that silently matches nothing looks identical to a clean result. Note g15 lives under
`tools/referee/reject/testdata/`, which is *outside* the usual corpus roots, so the corpus scan alone
never touches the one file that exercises the pair.

## Testing the *removal* of a notation spelling (the mirror image of adding a rule)

When a PR deletes an OpenSysML-only spelling (e.g. the ACTION-node aliases `initial <name> [then
<target>];`, `final [<name>];`, `decision <name>;` in favour of `first`/`done`/`decide`), the
emit site in `nonstandard_notation.go` disappears **and** the parser stops matching the word, so the
warning is replaced by a parser error. Test it as a pair of claims, not one:

1. **Each removed spelling must now produce a located error**, not silence. `bin/sysml -validate
   <f>` should print `<f>:<l>:<c>: error: expected a body member` with the caret under the removed
   word, and exit **2**. One fixture per construct — parser recovery cascades and invents
   `expected a namespace member` errors on the closing braces if you put several in one file.
2. **The word must still work as an ordinary name**, because these words are *unreserved*
   (`internal/syntax/lexer/contextual.go`, `parser/notation.go` `notationWords`). Put
   `attribute final : Boolean;`, `action initial;`, `out decision : Boolean;`, a kindless `final;`
   and a kindless `decision;`, and `first initial then final2;` in one clean file: default mode must
   exit 0 and `-convert sysml` must reproduce every member verbatim.

**The `then <word>;` shape is the trap.** A removed word reached as a *succession end* is not a node
at all, so no notation rule sees it and the parser accepts it as a reference to a target with that
name. `action def A { first a; action a; then final; }` therefore validates **clean, exit 0, even
under `-strict`**, where the old build warned at the `final` span — and the only surface that
complains is execution: `bin/sysml -action A <f>` fails with
`lower action graph: succession edge references undefined target node "final"`. Confirm the same
output for `then zzz;` to establish whether it is a general permissiveness of the resolver rather
than something the branch introduced, and report it either way — the user asking for "bare
`then final;` diagnosed" will not get that from `-validate`.

**Always build a merge-base contrast binary**; "the alias is gone" is only observable as a
difference. `git worktree add /tmp/wt-old $(git merge-base origin/main HEAD)` then
`go build -C /tmp/wt-old -o /tmp/old-sysml ./cmd/sysml` — build **in the worktree**, since a plain
`go build ./cmd/sysml` from the repo root compiles the branch again and every comparison then reports
IDENTICAL (~2 min cold). Then assert the *unchanged* neighbours are
byte-identical rather than merely present:

```bash
for f in state_markers.sysml oneended_first.sysml; do for m in "" "-strict"; do
  a=$(bin/sysml -validate $m $f 2>&1; echo exit=$?); b=$(/tmp/old-sysml -validate $m $f 2>&1; echo exit=$?)
  [ "$a" = "$b" ] && echo "IDENTICAL $f [$m]" || diff <(echo "$b") <(echo "$a"); done; done
```

A whole-repo sweep of the same comparison (`examples`, `testdata`, `internal/*//testdata`;
~1440 files, ~4 min serial) is the false-positive control, and it is *non-vacuous* here because the
hand-written removed-spelling fixtures above do differ under the identical method — cite that as the
control rather than reporting "0 differences" alone.

**Editor surfaces move too.** Removing the word from `lexer.contextualWords` drops it from LSP
completion and from the generated TextMate grammars. Both are cheap to assert:
`grep -o decision editors/vscode/syntaxes/*.json | wc -l` (expect 0 new vs 2 old), and a completion
request in an action body counted old vs new (283 → 282 labels, `decision` absent, `decide` still
offered). Drive completion with a *persistent* `Popen` + a reader thread and `time.sleep` between
messages — `subprocess.run` with all messages piped at once returns **empty stdout and rc 0** from
`bin/sysml-lsp`, which looks exactly like "no completions offered".

## Devin Secrets Needed

None. Everything runs against the locally provisioned `build/pilot-validator` and the committed
corpora; network access is needed only to re-provision them.
