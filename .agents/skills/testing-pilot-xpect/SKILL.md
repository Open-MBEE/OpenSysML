---
name: testing-pilot-xpect
description: How to verify the advisory pilot Xpect oracle harness (cmd/pilot-xpect + scripts/download-pilot-xpect.sh) end to end on Linux — provisioning the pinned .xt suites, reproducing the committed baseline, proving determinism under -jobs concurrency, spot-checking the oracle's truthfulness against an independent surface, and the adversarial mutations worth trying.
---

# Testing the pilot Xpect oracle harness (`cmd/pilot-xpect`)

Third sibling of `testing-pilot-differential` and `testing-grammar-coverage` (same pin
`scripts/pilot-pin.sh`, same "committed artifact, testable by reproduction" shape, same
`build/`-only unvendored corpus). This one reads the OMG pilot's own Xpect `.xt` test files as
*data* — no Java, no Xtext — and adjudicates each **declared** expectation
(`errors`/`warnings`/`noErrors`/`linkedName`, and since wave 7F also `scope`) against our
front end. Only `exportedObjects` (1 assertion) is still `not adjudicated`.

## Prerequisites

- `export PATH=/usr/local/go/bin:$PATH`, `git`, `jq`, network to github.com. No Java, no Maven —
  unlike `pilot-diff`, this harness runs nothing from the pilot.
- `./scripts/download-pilot-xpect.sh` — ~10 s sparse blobless clone into
  `build/pilot-xpect-corpus/{kerml,sysml}`, gitignored. Expect **303 KerML + 125 SysML = 428**
  `.xt` files. Not in the blueprint; re-provisioning from scratch is cheap, so always test it.
- `make build` (for `bin/sysml-lsp`) if you want the independent spot-check surface below.

## The core check (~5 s per run)

```bash
rm -rf build/pilot-xpect && go run ./cmd/pilot-xpect
cmp build/pilot-xpect/pilot-xpect.json docs/project/pilot-xpect-baseline.json   # must be silent
```

Observed on `main` after the wave-6 round: `428 .xt file(s), 0 unparsed; 1261 assertion(s),
1326 expectation(s): 449 agree, 646 disagree, 0 unlocated, 231 not adjudicated`. After wave 7F
(scope adjudicated) plus the central #421 rebaseline: `522 agree, 803 disagree, 0 unlocated,
1 not adjudicated`; per-suite kerml 303/968/454/513/1, sysml 125/358/68/290/0. After wave 8F
(per-fixture declared resource set, foreign-diagnostic filter, `exportedObjects` adjudicated):
`564 agree, 762 disagree, 0 unlocated, 0 not adjudicated`, plus a new line
`19 diagnostic(s) another declared resource raised, not adjudicated against the file`; per-suite
kerml 457/511/0, sysml 107/251/0. The scope row and its classes are unchanged by 8F
(`73 agree / 157 disagree`, `other-paths 12 | extra-names 32 | missing-names 10 |
missing-and-extra 7 | library-names 96`). After the wave-8 rebaseline (#436/#445):
`630 agree, 696 disagree, 0 unlocated, 0 not adjudicated`, `errors` kind
`rows 513 | agree 95 | same-location 246 | same-line 62 | severity-differs 16 | elsewhere 57`,
`warnings` `rows 113 | agree 23 | severity-differs 60 | elsewhere 7`, `scope` `74 / 230`.
After the wave-9 rebaseline (merged `main` `8faed39a`):
`816 agree, 510 disagree, 0 unlocated, 0 not adjudicated`, `errors` kind
`rows 513 | agree 95 | same-location 232 | same-line 62 | severity-differs 24 | elsewhere 51`
(silence 49), `warnings` `rows 113 | agree 89 | same-location 3 | severity-differs 4 | elsewhere 13`
(silence 4), `noErrors` `254 / 275`, `scope` `183 / 230` with classes
`other-paths 0 | extra-names 3 | missing-names 2 | missing-and-extra 14 | library-names 28`.
After the wave-10A scope round (`maxNameOccurrences=2` + `enterFor` inherited-import step accounting
in `scope_names.go`, `declaredTypeFeatureBase` in `semantics/implicit.go`): `845 agree, 481 disagree,
0 unlocated, 0 not adjudicated`, `foreignDiagnostics 18`, per-suite kerml 642/326 and sysml 203/155,
`scope` `212 / 230` with classes
`other-paths 0 | extra-names 5 | missing-names 2 | missing-and-extra 3 | library-names 8`;
the `errors`/`warnings`/`noErrors`/`linkedName` rows are unchanged from wave 9 (silence 49 / 4).
After the wave-10 rebaseline (all slices merged, plus the central `wording-only` class):
`1172 agree (of which wording-only 239), 154 disagree, 0 unlocated, 0 not adjudicated`, per-suite
kerml 875/93 and sysml 297/61, `errors` kind
`rows 513 | agree 418 | wordingOnly 239 | same-location 9 | same-line 25 | severity-differs 11 |
elsewhere 42` (silence 8), `warnings` `rows 113 | agree 99 | same-line 7 | severity-differs 5`
(silence 2), `noErrors` `248 / 275`, `scope` `212 / 230`.
`noErrors` falling 254 → 248 is wave 10E's unavoidable cost, not a regression to chase: the six
visibility fixtures declare file-wide silence and the protected-import errors at once, so no
implementation satisfies both.
Read the live totals from the baseline rather than this paragraph; it is an anchor, not the check.

**`wording-only` is a verdict, not a tolerance.** `cmd/pilot-xpect/wording.go` admits a row into
agreement only when the declared and our message state the same rule about the same element; the
caller has already matched severity and offset, and those two alone are never enough. Rows that keep
the offset but change the rule stay `same-location` disagreements, so a jump in `agree` after
touching this file must be reconciled against the `wordingOnly` sub-count before it is reported as
detection.

**The scope narrative section of `docs/project/pilot-xpect.md` is a separate staleness risk from the
headline tables.** A rebaseline that updates the summary block, the kind table and the per-suite
table can leave the `## scope — N of 230 agree exactly` heading, the ToC anchor, the "result, by
class" table and the numbered worklist items (per-fixture name counts like "declares 829 and we offer
3362") holding the previous wave's numbers, and **no test catches it** — `w6f_skill_counts_test.go`
only guards the derived headline count lines. Always grep the doc for the old agree count and for the
old class-table numbers after a scope-moving wave.

**The "nothing / silence" column is `rows - agree - sameLocation - sameLine - severityDiffers -
elsewhereInFile`.** Subtracting only the tolerance fields is a bug that stays invisible while a
kind's strict `agree` is 0 — it was, for `errors`, until wave 8, and both doc guards
(`doc_counts_test.go`, `w6f_skill_counts_test.go`) had to be corrected. When reviewing any change to
the silence/nothing numbers, recompute the column yourself from the committed baseline JSON with
`jq`/python rather than trusting the guard, because the guard and the doc can be wrong together in
the same direction.

**A wave that moves the verdicts must also rebaseline.** `cmp` against
`docs/project/pilot-xpect-baseline.json` is the only thing that catches a missed rebaseline:
`cmd/pilot-diff`'s `TestW6FXpectDocumentCountsMatchBaseline` only guards `pilot-xpect.md`
*against the baseline JSON*, so a branch that leaves **both** stale still passes `go test ./...`.
Always run the `cmp` explicitly and diff `jq .totals` of the two — this was a real finding on the
wave-8F branch (harness emitted 564/762/0 while the committed baseline and doc still held
522/803/1).

Cross-check every numeric table in `docs/project/pilot-xpect.md` (kind table, tolerance tables,
per-suite table, scope class table) against the fresh `.txt` — those tables are hand-maintained and
are the usual thing to go stale after a merge or rebaseline.
See `testing-pilot-differential/SKILL.md` for the complete mechanical-guard scope and review-only surfaces.

**Determinism under concurrency is the main risk** (`compareAll` in `main.go` fans N goroutines
into one shared result slice). Test it explicitly, not just by repeating the default run:

```bash
for j in 1 3 16 0; do go run ./cmd/pilot-xpect -jobs $j -out /tmp/xj$j; done
for j in 1 3 16 0; do cmp /tmp/xj$j/pilot-xpect.json docs/project/pilot-xpect-baseline.json; done
```

`-jobs 0` is clamped to 1. Compare the `.txt` too, not only the `.json`.

## Provisioning checks that distinguish working from broken

- The script is `pilot_fetch_subtrees` from `scripts/pilot-pin.sh` with `PILOT_FETCH_GLOBS=('*.xt')`,
  so every property of the shared downloader (commit-verified clone, `.pilot-pin` stamps, stale and
  unstamped suites re-fetched) holds here too; see `testing-pilot-corpora-gate`.
- Move, do not delete: `mv build/pilot-xpect-corpus /tmp/xpect-corpus-backup`. After a fresh
  download, `diff -r /tmp/xpect-corpus-backup build/pilot-xpect-corpus` must be empty apart from
  the `.pilot-pin` stamps if the backup predates them.
- Idempotence: second run prints `Already present at ... (pin 2026-07 c7fc737d…)` twice, exit 0.
  Prove it by `stat -c '%n %Y'` on the two suite dirs before/after, not by the message.
- Bogus pin: `PILOT_TAG=9999-99 ./scripts/download-pilot-xpect.sh` → git `fatal: Remote branch
  9999-99 not found` then `error: could not clone …`, **exit 1**, and `build/pilot-xpect-corpus`
  is left as it was (everything is staged in a `mktemp -d` and only `mv`d in at the end). Check for
  a half-populated corpus with `find … -name '*.xt' | wc -l` (0 from absent, 429 from present).
- Moved tag: `PILOT_COMMIT=0000000000000000000000000000000000000000 ./scripts/download-pilot-xpect.sh`
  → `error: <repo> tag 2026-07 resolves to c7fc737d…, scripts/pilot-pin.sh pins 0000…`, exit 1,
  nothing provisioned.
- Nothing vendored: `git status --porcelain` clean after every step.

## No-corpus degradation

`mv` the corpus aside, then `go run ./cmd/pilot-xpect -out /tmp/nocorpus` → **exit 1**, stderr
`skipping kerml: build/pilot-xpect-corpus/kerml is absent (run scripts/download-pilot-xpect.sh)`
(same for sysml) then `pilot-xpect: no suite found under build/pilot-xpect-corpus; …`, and
`/tmp/nocorpus` is never created. It must never exit 0 or report 100 % agreement.

## Isolating a merge's effect on the adjudication (fast, covers all rows)

When a branch merges `main` and you must prove no verdict silently moved, do not re-spot-check by
hand: diff the *rows* of the old and new baselines. `git show <pre-merge-sha>:docs/project/pilot-xpect-baseline.json`
and compare keyed on `suites[].files[].path` + `rows[].line` + `rows[].kind`, comparing
`(verdict, tolerance, names, ours)`. If the two baselines are byte-identical (`cmp`), the merge
provably changed nothing in the oracle's output. Beware: `suites[].dir` is the **full corpus path**,
and agreeing rows are *pruned* from the JSON — never infer "agree" by matching on `dir`, and treat
"absent from JSON" as agree only after confirming the totals.

## Spot-checking the oracle's truthfulness (the failure mode that matters)

Agreeing rows are pruned from the baseline (`report.go: pruned`), so "absent from the baseline"
must be shown to mean *agree*, not *silently dropped*. Use `bin/sysml-lsp` over stdio as the
**independent** surface — it is the only easy one that honours the file's language and resolves
sibling files:

- `textDocument/definition` on the `linkedName` target text tells you which symbol we bound.
  Historical example, and the one that proved the surface works: at `19a3ce03`, in
  `testsuite/MemberNameTests_LocalNamedMember.kerml.xt`, `A_alias` (line 37, declared `test.A`)
  jumped to the `alias A_alias for A;` line → ours `test.A_alias`, which was the report's headline
  finding (an alias resolving to itself, 41 of the 43 `linkedName` disagreements). Wave 6 fixed it
  and that column is now 194 of 194, so the same probe today lands on `classifier <A_Id> A;`.
- `textDocument/publishDiagnostics` after `didOpen` reproduces `noErrors`/`errors` rows. Copy the
  `.xt` verbatim to a `.kerml` file (the XPECT notes are ordinary comments, so it is valid source)
  and, when the `XPECT_SETUP` `ResourceSet` names `/src/...` files, drop those beside it and pass
  the directory as `rootUri`. Confirmed exactly the reported single error
  `ambiguous reference: SamePackage::container (2 candidates)` at line 29 of
  `imports/global/DependencySamePackageName.kerml.xt`.
- `bin/sysml`'s REPL is **not** useful here: `%print` echoes the declaration and there is no
  meta-command that prints a resolved FQN, and `cmd/sysml` analyses under the name `<repl>` so the
  KerML language tier is not honoured (see `testing-pilot-differential`).
- **Probing a `scope` row:** copy the `.xt` to `Main.kerml` in a temp dir together with the `/src/`
  files its `XPECT_SETUP` `ResourceSet` names, insert `alias __pN for <name with '.' → '::'>;` into
  the *same namespace* as the anchor, and read `publishDiagnostics`: an in-scope name is clean, an
  out-of-scope one gives `unresolved reference: …`. `model.VisibleNames`/`ElementOnPath`/`FQNOf`/
  `ScopeAt` have no caller outside `cmd/pilot-xpect/scope.go` (grep to re-confirm), so the LSP
  really is an independent surface.
- **Caveat that will confuse you:** a scope "missing" name is missing from the *enumeration*, not
  from the resolver. `VisibleNames` truncates a path at the first repeated element, so e.g.
  `testsuite/ShadowingTests_CircleProblem2.kerml.xt:22` reports 3 missing names that all resolve
  fine through the LSP. The direction is conservative (it under-reports our agreement) and is
  documented in `pilot-xpect.md`; do not file it as a scoring bug.
- Each fixture loads exactly the resources its `XPECT_SETUP` `ResourceSet` names — its own body, the
  `/src/` files and the `/library*` copies — never our embedded stdlib in their place, so an
  independent LSP check must open the same set to be comparable.

### Auditing the foreign-diagnostic filter (`compare.go: foreignDiagnostic`)

A fixture's declared library set is usually incomplete, so unresolved references raised *for a
library file* arrive attached to the file under test. The harness drops them **by byte offset**:
`source.Span` carries no file identity, so "outside the fixture's model text" (past EOF, or inside a
note per `xtFile.Noted`) is the only signal. That filter can only make the comparison weaker, so
audit it rather than trusting the count. The cheap, decisive probe is a throwaway
`cmd/pilot-xpect/zz_tmp_*_test.go` (package `main`, delete it afterwards) that walks the corpus,
calls `loadResourceSet` + `ws.Diagnostics(main)` exactly as `compareFile` does, and for each
diagnostic where `foreignDiagnostic(f, off)` is true prints the offset, the fixture's own text at
that span, and the text at the *same span* in every declared resource. Provenance is proven when the
text at that offset in some declared `/library*` file is literally the identifier the message names.
At wave 8F the 19 drops split 14 past-EOF / 5 inside a note, and all 19 matched a library file
exactly (`Objects.kerml` → `Link`/`BinaryLink`/`Occurrence`, `Connections.sysml` →
`BinaryLinkObject::source`, `States.sysml` → `StatePerformance`, `Metadata.sysml` → `Metaobject`).
Pair it with a negative control: inject a genuine `part zzzProbe : ZZZ_no_such_type;` into a
fixture's *model body* — the new diagnostic must flip that fixture's `noErrors` row to `disagree`
and must **not** raise `foreignDiagnostics`.

`exportedObjects` (`exported.go`) is adjudicated as of 8F, so `notAdjudicated` is 0. Prove the
adjudicator is live in both directions on the sole fixture,
`indexing/NameEscape.kerml.xt`: adding a bogus `sysml::Package: ZZZ_bogus` line inside the note's
`---` fence must give `disagree … missing 1`, and adding a real `class zzz_extra {}` to the body
must give `disagree … extra 1 (sysml::Class: NameEscape::zzz_extra)`.

### Proving a fix wave's trade in both directions (recoveries *and* regressions)

A wave that claims "N rows recovered" must also be shown to have given nothing back — a net total
can hide an equal-sized swap, and on a stacked branch the committed baseline is often deliberately
stale, so `cmp` against `docs/project/pilot-xpect-baseline.json` cannot serve as the check. Capture
the pre-change run as a snapshot (`go run ./cmd/pilot-xpect -out build/pilot-xpect-before` on the
parent commit, or `git show <parent>:docs/project/pilot-xpect-baseline.json`) and diff the *keys* of
the non-`agree` rows, keyed on `suite name + file path + row line + row kind`:

```python
def keys(p):
    d = json.load(open(p)); out = {}
    for s in d.get("suites", []):
        for f in s.get("files", []):
            for r in f.get("rows", []):
                if r.get("verdict") != "agree":
                    out[(s.get("name"), f["path"], r.get("line"), r.get("kind"))] = r
    return out
# before - after = recoveries; after - before = regressions (must be empty)
```

Key on `suites[].name`, **not** `suites[].dir` (which is the absolute corpus path and so is
machine-specific). Because agreeing rows are pruned, "absent from the after-file" means agree only
once the totals confirm it — always print `len(before)`/`len(after)` alongside, and check that
`before_disagreements - after_disagreements` equals the recovery count with zero regressions.
A stacked branch's parent snapshot lives in gitignored `build/`, so it survives a `git status`
cleanliness check.

### `bin/sysml -validate` is the cheapest independent surface for the visibility fixtures

Before reaching for the stdio LSP client, try the CLI: `-validate` honours the `.kerml` extension
(see `testing-pilot-differential`) and accepts **several files on one command line**, so a fixture's
whole declared resource set can be laid out flat and checked in one command:

```bash
C=build/pilot-xpect-corpus/kerml
cp "$C/src/DependencyVisibilityPackage.kerml" "$C/library/Base.kerml" /tmp/probe/
cp "$C/src/org/omg/kerml/xpect/tests/visibility/VisibilityTests_ProtectedImport_0.kerml.xt" /tmp/probe/Main.kerml
/tmp/sysml-<sha> -validate /tmp/probe/Main.kerml /tmp/probe/DependencyVisibilityPackage.kerml /tmp/probe/Base.kerml
```

The `.xt` copies verbatim (the XPECT notes are comments). It reports `unresolved reference: <name>`
with line/column and a caret, so the per-line verdicts the fixture declares are directly readable,
and `exit 2` + `did not analyse cleanly` vs `exit 0` + `no errors` is the whole-file summary. Our
embedded stdlib is also loaded (unlike the harness, which loads only the declared resources), so
expect an extra `User library packages should not be marked as standard` warning on the copied
`/library/Base.kerml`; it is noise, not a finding. Run the **same** command with a per-commit
`go build -o /tmp/sysml-<sha> ./cmd/sysml` from a `git worktree` of the base — identical text with
opposite verdicts on the two binaries is what proves a visibility change is live.

**The CLI cannot express a library-less resource set, but self-conjugation gets you the same
rules anyway.** Fixtures that exist precisely because the standard library is *absent* (the
KerML implicit-base family: `Must directly or indirectly specialize Base::Anything`,
`Features must have at least one type`, e.g. `Feature_invalid_noType`) are silent under
`bin/sysml -validate` on both base and HEAD, because the standalone CLI always loads the
embedded stdlib while the harness loads only what the `XPECT_SETUP` declares, and there is no
`-no-stdlib` flag (check `cmd/sysml/main.go`'s `flag.` block before assuming otherwise).
Do not read that silence as "the rule is missing", and do not try to force it by passing the
fixture's `/library*` copies on the command line — that just produces duplicate-library errors.
Instead reach the *other* arm of the same rule: conjugation replaces the implicit
specialization, so a **self-conjugating** declaration fires both messages with the stdlib
loaded and is fully CLI-observable:

```kerml
package P {
    classifier C ~ C;   // HEAD: "Must directly or indirectly specialize Base::Anything"
    feature c ~ c;      // HEAD: "Features must have at least one type"
}
```
The base binary reports only `participates in a specialization cycle` for both, and the valid
twin (`classifier C ~ D; feature d : D; feature c ~ d;`) is silent on both binaries — which is
what makes the pair discriminating rather than decorative. The same
`ParsingTests_CircularReferences.kerml.xt` rows are the harness's version of this case.

**Row-level regression join.** `pilot-xpect.json` prunes plain agreements: a file's `rows`
array holds only the wording-only and disagreeing expectations. So build the disagreeing-row
set per revision keyed by `(suite, file.path, kind, line, at, declared)` with a collision
suffix, and confirm `len(set)` equals the reported `totals.disagree` before differencing —
that check is what proves the key is unique enough to trust. `control - branch` is the
recoveries, `branch - control` is the regressions, and a claimed "+N recoveries" is only
believable when the second set is empty *and* `agree_control + N == agree_branch`.

### Probing `import all` widening: the obvious fixture does not exercise it

`r.allVisible` (`resolve/resolver.go: inAllVisible`) is nonzero only *while the target path of an
`import all`/`expose` is itself being resolved*. So a fixture that writes `import all P::*;` and then
references `P::PP` in the body proves **nothing** about the widening — the body reference resolves
outside `allVisible` and is unresolved on every revision. The widening is exercised by putting the
non-public segment **inside the import path**:

```kerml
package Outer {
	classifier P { protected classifier PP { public classifier Leaf; }
	               private classifier Hid { public classifier HLeaf; } }
	package WidenedProtected { private import all Outer::P::PP::*;  classifier w1 specializes Leaf {} }
	package WidenedPrivate   { private import all Outer::P::Hid::*; classifier w2 specializes HLeaf {} }
	package NotWidened       { private import Outer::P::PP::*;      classifier w3 specializes Leaf {} }
}
```

`NotWidened` is the load-bearing negative control: it must report `unresolved reference:
Outer::P::PP` while the two `all` packages stay clean, otherwise the "clean" lines are not evidence
of widening. Also note OpenSysML requires a visibility keyword on imports in `.kerml` too — a bare
`import all P::*;` errors with `import without a visibility indicator` and every following line
"fails" for that reason instead of the one under test.

### The discriminating-pair LSP probe for visibility rules

For a change that makes visibility *conditional on the referring namespace* (protected members
admitted from the owning/specializing namespace only), a one-directional probe proves nothing:
"it resolves" is also what a blanket "protected is public now" regression looks like. Build the
**same qualified text in two namespaces in one file** and read one `publishDiagnostics`:

```kerml
package Unrelated {
	public import VisibilityPackage::*;
	classifier P1 specializes c_Public::c_private{}     // private        -> must be unresolved
	classifier P2 specializes c_clazz::c_Protect{}      // protected, unrelated -> unresolved
	classifier Spec specializes c_clazz {
		classifier P3 specializes c_clazz::c_Protect{}  // same text, specializing -> clean
	}
	classifier Bogus specializes ZZZ_no_such_type{}     // liveness control -> unresolved
}
```

Identical spelling on the `P2` and `P3` lines with opposite verdicts is the only cheap evidence that
the admission is conditioned on the referrer. Always include the `ZZZ_no_such_type` line: it proves
the diagnostics channel is live, so a clean `P3` is a real resolution and not a silent LSP failure
to load the declared resources.

A ~50-line python stdio client is enough (`Content-Length` framing, `initialize` with `rootUri` and
`workspaceFolders` → sleep → `initialized` → `didOpen` → collect `textDocument/publishDiagnostics`,
filtering on the target's basename). Lay the fixture out exactly as its `XPECT_SETUP` `ResourceSet`
declares — copy the `.xt` verbatim to `Main.kerml` and put the declared `/src/...` and `/library/...`
files at those same relative paths under `rootUri` — or the protected owner will simply be missing
and every line will "fail" for the wrong reason. Give `initialize` ~2.5 s and `didOpen` ~4 s; a
shorter sleep silently yields "NO publishDiagnostics RECEIVED".

**Gotcha:** `go test … 2>&1 | tail` reports the *pipe's* exit status, so `echo exit=$?` prints 0 even
for a failing `OPENSYSML_REQUIRE_PILOT_XPECT=1` run. Assert on the `FAIL` / message text, not `$?`.

### `bin/` binaries go stale — rebuild per commit, or your before/after diff is a no-op

`bin/sysml-lsp` is a committed-path build artifact, not a per-commit one: `make build` at commit A
leaves it in place across a later checkout of commit B. Comparing "before" (`/tmp/lsp-A`) against
"after" (`bin/sysml-lsp`) then silently compares A against A and shows *identical* output, which
reads as "this commit changed nothing" — a false negative that looks exactly like a passing test.
Always `go build -o /tmp/lsp-<sha> ./cmd/sysml-lsp` **per commit under test** and diff two explicitly
named binaries, and re-run `make build` before trusting anything in `bin/`.

For a per-commit before/after on a clean tree, `git worktree add /tmp/wt-<sha> <sha>` is better than
`git stash`/checkout: it leaves the primary checkout and its gitignored `build/` corpus untouched, so
the oracle run and the LSP probe can proceed in parallel. Test-only files can be copied into the
worktree to check that a new regression test is load-bearing (fails at the parent, passes at HEAD);
`git worktree remove --force` afterwards, and confirm `git worktree list` shows only the primary.

### Diagnostic counts move between tiers — count per tier, not just per line

`internal/core/resolve` is not the only diagnostic producer, so "the resolver no longer complains"
and "the line is clean" are different claims. `resolve` emits `unresolved reference: …` (sole emitter:
`qualified.go`'s `unresolved`/`unresolvedNamespace`), while the tier-4 constraint pass in
`internal/core/passes/constraint.go` emits its own, e.g. `"%s redefines %s, but %s is not an inherited
member of %s"` (code `redefinition-no-inherited`). A fix that makes a reference *resolve* can hand the
line straight to a later tier, so an end-to-end `publishDiagnostics` count stays 1 while the message
changes completely. A unit test asserting `len(r.Diagnostics) == 0` on a bare `Resolver` is therefore
consistent with the LSP still showing one diagnostic — neither is wrong; they measure different tiers.
Report the message text, not just the count, and confirm which package emits it (`grep` the literal)
before calling a leftover diagnostic a regression.

To attribute a diagnostic definitively, temporarily instrument `Resolver.report` in
`internal/core/resolve/resolver.go` with an env-gated `debug.Stack()` dump and rebuild the LSP; if the
stack never fires for a message you can see in the editor, that message is coming from another package
entirely. Restore the file and `git update-index --refresh` before checking `git status --porcelain`.

## Adversarial mutations (mutate a copy or restore from the `mv`d backup, then `diff -r`)

| Mutation | Expected |
|---|---|
| delete the `END_SETUP` line | file listed under `unparsed files` as `XPECT_SETUP block is not terminated by END_SETUP`, `filesUnparsed` +1, `files` still 428 |
| `XPECT_SETUP` → `XPECT_SETUPX` | `no XPECT_SETUP block` |
| turn `// XPECT errors …` into `//* XPECT errors …` with no `*/` | `line N: `//* XPECT errors` note is not terminated` |
| drop one declared name from an *agreeing* `scope` note | that row flips to `disagree(extra-names)` with `extra 1` — proves the set comparison is live |
| add `ZZZ_not_a_name` to an agreeing `scope` note | `disagree(missing-names)` naming it, and *not* `other-paths`/`library-names` |
| add a real member (`class zzz_extra {}`) to a `library-names` fixture's namespace | class moves to `extra-names` — proves the `self`/`that` filter covers *all* differences, not just the 5 names the row text samples |
| delete a `/src/` file named by an `XPECT_SETUP` `ResourceSet` | totals report `N missing declared resource(s)` and list the declaring fixtures; assertion/expectation counts stay 1261/1326 — it must not silently agree |
| `END_SETUP` → `END_SETUPX` | `XPECT_SETUP block is not terminated by END_SETUP` — the terminator is the token `\bEND_SETUP\b` (`xt.go`), so a suffixed keyword does not terminate. Locked by `w5c_xt_test.go`; a plain substring search here was the one real defect testing found. |

An unparsed file lowers the assertion/row/disagree counts (it is counted, not compared), so always
restore and re-`cmp` against the baseline afterwards.

## Test-suite liveness

```bash
go test -count=1 ./cmd/pilot-xpect
OPENSYSML_REQUIRE_PILOT_XPECT=1 go test -count=1 ./cmd/pilot-xpect
```

With the corpus moved aside the plain run **skips** (`ok`, 0.001 s) and the `REQUIRE` run **fails**
with `build/pilot-xpect-corpus/kerml is absent; run scripts/download-pilot-xpect.sh`. Prove the
census in `w5c_census_test.go` is live two ways: perturb one pinned triple (e.g. kerml
`kindErrors: {295, 17, 328}` → `329`) → fails naming that kind; and perturb the *reader*
(narrow `xpectLineRe`'s `[ \t]` classes to `[ ]`) → fails with collapsed counts
(`errors = [14 0 14]`). Revert both and `git update-index --refresh` before checking
`git status --porcelain` — plain `git status` can report a stat-dirty file after `cp`-restores.

## Regression neighbour

`go run ./cmd/pilot-diff` (~1m12s) must still print the headline the *committed* baseline holds —
after the expressions example joined `examples/` that is `368 file(s), 337 fully agreeing; 34 agreed diagnostic(s), 21
only ours, 600 only the pilot's`. Read the number out of
`docs/project/pilot-differential-baseline.json` rather than trusting this line, since a landing fix
round moves it. When the baseline is itself stale (it was at `19a3ce03`, holding 273 / 281 / 317), a
failing `cmp` against it is *not* evidence of an Xpect regression — compare the summary line, and see
`testing-pilot-differential`'s "Isolating one change's effect" for the entry-keyed delta.

## Reviewing a wave that adds validation passes (not just the harness)

A wave whose passes move the Xpect verdicts (e.g. wave 9C's `internal/core/passes/w9c_*.go`) is best
judged by running the harness on **both revisions** rather than by trusting the committed baseline:

```bash
git worktree add /tmp/osml-main <base sha>
mkdir -p /tmp/osml-main/build
ln -s "$PWD/build/pilot-xpect-corpus"       /tmp/osml-main/build/pilot-xpect-corpus
ln -s "$PWD/examples/pilot-corpora"         /tmp/osml-main/examples/pilot-corpora
ln -s "$PWD/examples/sysml-v2-training"     /tmp/osml-main/examples/sysml-v2-training
(cd /tmp/osml-main && go run ./cmd/pilot-xpect -out /tmp/xpect-main)
```

The corpora are gitignored, so a fresh worktree has none — symlinking them in is what makes the
base-revision run comparable (and avoids a second ~10 s download). This is also the *only* honest
negative control for a new rule: build the base revision's CLI
(`go build -o /tmp/sysml-main ./cmd/sysml` inside the worktree) and show the same model draws no
warning there.

**Some waves are forbidden from rebaselining.** When a parent session does one central rebaseline at
the end of a wave, `cmp build/pilot-xpect/pilot-xpect.json docs/project/pilot-xpect-baseline.json`
*is expected to differ* on the branch and the headline tables in `pilot-xpect.md` are expected to be
stale. Confirm the intent with the lead before filing it — measure and report the live numbers instead.

**Do not forget `make docs-counts`.** Adding rows to `docs/project/spec-compliance.md` without
regenerating the three derived count lines (`README.md`, `docs/internals/architecture.md`,
`docs/project/spec-compliance.md:13`) fails `cmd/pilot-diff`'s
`TestPilotDifferentialDocumentCountsMatchBaseline` (`coverage total: want 690 …, got 688 — run
\`make docs-counts\``). This is only visible in a full `go test ./...`, so always run the whole gate,
and confirm the same test passes on the base revision before calling it a branch regression.

**Attributing an internal tolerance move.** `agree`/`disagree` can be unchanged while rows move
between tolerance buckets (wave 9C: `errors` severityDiffers 16→24, elsewhereInFile 57→49). To name
the exact rows, key the JSON rows on `(file path, line, index within the file's rows, tolerance)` and
diff the two revisions' multisets — keying on `(path, line)` alone silently collapses several rows
declared on the same line, and the plain row index shifts whenever a row is inserted above.

**Key the ratchet on the ordinal among rows sharing `(line, kind)`, never on the row's index within
the file.** Because agreeing rows are pruned, removing one row renumbers everything below it, so an
index-keyed diff reports phantom "added" rows: on the wave-10A branch it showed 3 added / 32 removed
(`CircleProblem4{,_FT,_Rdef}` line 32/43 rows "appearing") where the correct keying shows **0 added /
29 removed** plus 7 rows changing tolerance in place. Report the in-place tolerance changes too — a
row that stays `disagree` but moves `missing-and-extra` → `library-names`/`extra-names` is neither an
add nor a removal, and a *degradation* would hide there.

## Cache independence of a library-reading rule

`libs.NewCache()` resolves `$XDG_CACHE_HOME/sysml-ls/libs` (`internal/core/libs/cache.go`). A unit
test that just passes a `*libs.Cache` to `libs.NewLoader` **does not exercise a warm cache**: records
are only written by `Loader.Persist`, which the test helpers do not call, so the "warm" pass reads an
empty directory. To prove a rule behaves identically cold and warm, drive `bin/sysml` twice under a
fresh `XDG_CACHE_HOME` and diff the diagnostics:

```bash
CD=$(mktemp -d)
XDG_CACHE_HOME=$CD ./bin/sysml model.sysml > /tmp/cold.txt 2>&1   # populates ~95 .idx records
ls -1 "$CD/sysml-ls/libs" | wc -l
XDG_CACHE_HOME=$CD ./bin/sysml model.sysml > /tmp/warm.txt 2>&1
diff /tmp/cold.txt /tmp/warm.txt
```

Check the entry count between the runs — a still-empty `sysml-ls/libs` means the warm run was cold
again and the check proved nothing.

## Recording

Shell-only; no GUI is involved, so no recording is needed unless you deliberately drive a Konsole.

## Devin Secrets Needed

None. Network access to github.com is required only to provision the suites.
