# Strict validation of redefinitions that name no member

## Why this capability is needed

A `:>>` redefinition replaces a feature that the type declaring it owns or inherits. A model can spell a redefinition of a name that the resolver reaches through an import or an enclosing namespace while the declaring type has no member of that name: the name resolves, so name resolution is silent, and the definition is syntactically valid. Nothing in a strict conformance run rejected it, yet the exporter wrote a `redefines` handle pointing at a feature the type does not have. A downstream consumer that treats strict validation as a conformance gate therefore accepted a model whose redefinition has no target and received an element form with a dangling relationship. This change makes strict validation reject that model at the clause that caused it.

## Exact state at baseline `4d67c7a98f` / `v0.12.1`

At that revision the validation pipeline already distinguished strict conformance in `diag.ConformanceMode`, and `-validate -strict`, the REPL's `%strict on`, the editor's `strictConformance` and the service's `strict_conformance` all selected it. strict mode promoted notation no pinned grammar production admits and indistinguishable owned member names to errors; it did not check whether a redefinition had a target.

Three adjacent surfaces existed, none covering the case:

- The name-resolution tier reported `unresolved` when a reference named no element at all (the earlier unresolved-reference work in issues [#649](https://github.com/Open-MBEE/OpenSysML/issues/649) and [#650](https://github.com/Open-MBEE/OpenSysML/issues/650)). Here the name *does* resolve — to a library feature brought in by an import — so there was no finding.
- The constraint tier's `checkRedefinition` reported `redefinition-no-inherited` when a redefinition target was not accessible to the redefining feature, using `Model.CanAccess` (KerML 1.1 §8.3.3.3.4). A feature with no featuring type is featured by `Anything`, so a library attribute reached by an import satisfies `CanAccess`; the check stayed silent.
- The featuring-type checks added in [PR #2306](https://github.com/Open-MBEE/OpenSysML/pull/2306) compare the redefining and redefined features' derived featuring types. They likewise accept a target that has no featuring type of its own.

No upstream issue or pull request tracked this specific gap. The result was a clean `-validate -strict` run on `part pipe : NuclearReactor::HotLegPipe { attribute :>> length = 10.0 [SI::m]; }` where `HotLegPipe` declares no `length` but the enclosing namespace imports one from `ISQBase`.

## What the change implements

A new constraint-tier pass, `StrictRedefinitionPass`, registered in the default pass registry. In strict mode it walks the document's features; for each `:>>` clause it takes the written member name, resolves the target, and asks whether the declaration's scope offers a member of that name. It does so with the *declaring* member view, so the redefinition being checked does not mask its own target, and it considers the declared types of the owning usage and of the enclosing usages — the latter is what makes a snapshot or timeslice redefinition of the snapshotted occurrence's type legitimate. Three further scope paths reflect where a declaration legitimately reads names: a typed usage that also redefines or subsets a usage contributes its own declaring view (so members inherited from the target are visible); a redefinition written in a **definition** may name a feature of its enclosing namespace, including one reached by a private import (KerML 8.2.3.5.2); and a redefinition written in a usage that redefines or subsets may name a feature of the enclosing types the original feature belongs to. A target owned by a type that contributes members to the declaring context is likewise visible, and so is the leaf of a feature chain declared by a feature of that context (the root type's `self` chains `things.that`, which makes `that` a name every occurrence can redefine). The declared type remains the one named in the diagnostic. When no path supplies the name, the pass reports severity `error`, code `redefinition-resolves-to-nothing`, at the written target span, with a message naming the member and the type. A redefinition written in a **plain typed usage** borrows nothing: it must find the name among the members of its declared type. That is what keeps the canonical `part pipe : HotLegPipe { attribute :>> length … }` repro rejected while conforming library definitions are accepted.

Chain redefinitions (`:>> a.b`) are left to the rule that already judges them by where their prefix is declared. Names that resolve to no element are left to name resolution. The pass returns no diagnostics at all in the default mode, so models using OpenSysML's looser name lookup are unchanged.

## Design decisions and alternatives rejected

- **A new pass rather than widening `checkRedefinition`.** The existing check is always-on. Adding a finding there, even gated on mode, would couple a strict-only rule to an always-on checker and make the default-mode no-op harder to guarantee. A separate pass makes the mode guard the first statement and keeps the strict behaviour independently testable.
- **No change to `CanAccess` or the featuring-type rules.** Those model a different question (whether a feature can be accessed from a featuring context) and are deliberately lax for unfeatured library features. Tightening them would change hidden behaviour in default mode and break existing models.
- **No parser or AST work.** The notation is already valid; this is a semantic validity question about the relationship's target, so it belongs in a check pass over the symbol tree and the semantic model's member view.
- **No export-side repair.** Dropping the dangling handle in the writer would hide the invalid input instead of rejecting it, and would leave strict validation answering a different question from the element form.
- **No new flag or mode.** Strict conformance already exists on every surface; a separate switch would fragment the meaning of `-strict`.
- **Name-based lookup in the declaring view, not symbol identity.** A qualified target that names an equivalent inherited member is accepted rather than reported; this favours a small number of missed exotic cases over false positives on conforming models.
- **Only portions widen the *type* context.** An earlier version added every enclosing scope owner to the member view, which let a member of an enclosing type stand in for a member of the declaring type. The type context now widens only while the immediate owner is a snapshot or timeslice. The later scope paths are separate: they ask where a declaration may read a name, not which type owns the member. A plain typed usage gets neither an enclosing type nor an imported name.
- **Definitions and redefining usages, but not plain usages.** A definition's enclosing namespace is where its redefinition targets are looked up, so an imported library feature is a legitimate target there — this is how the bundled standard library is written. A usage that redefines or subsets stands in for the original feature and may reach names from the types enclosing it. A usage with only a typing clause has neither route, so the canonical repro stays rejected even though its import would answer a plain name lookup.
- **Hierarchy-owned targets and chained base features.** A target owned by a member source of the declaring context is reachable through that source, and a target that is the leaf of a chain a feature of the context declares is reachable through the chain. The second path is what makes `that` (the leaf of `Anything::self`'s `chains things.that`) a legitimate target in `Items::Item::envelopingShapes`.
- **Defer inaccessible targets to the inheritance rule.** A target the redefining feature cannot access is already reported by `redefinition-no-inherited`. The strict pass declines those relationships instead of adding a second, contradictory finding; it reports only where the target is accessible yet absent from the declaring type.
- **Name the declared type, not a bare usage.** When an immediate owner such as a snapshot has no declared typing, the diagnostic names the enclosing occurrence's declared type, since that is the type whose members were actually searched.

## Public surface added

- Diagnostic code `redefinition-resolves-to-nothing` (severity `error`, tier `constraint`): the code in text output, in the `-json` `code` field, in the editor's `codeDescription`, and in the wire diagnostic. It is covered by a section in [the diagnostics reference](../reference/diagnostics.md#redefinition-resolves-to-nothing) and is available through `sysml -explain redefinition-resolves-to-nothing`, `%explain-code` and `explain_diagnostic`.
- Exported Go identifiers `passes.CodeRedefinitionResolvesToNothing` and `passes.StrictRedefinitionPass`; the pass is registered in `passes.DefaultRegistry`.
- No new command-line flag, REPL command, RPC, protobuf message, protobuf field, capability gate or environment variable. Strict conformance is selected through the existing `-validate -strict`, `%strict on`, editor `strictConformance` and service `strict_conformance`.
- One fixture in SysML textual notation under `tests/testdata/passes/`; no new file format or format version.

## How each acceptance criterion is locked

| Criterion | Test |
|---|---|
| Repro model is rejected in strict mode and accepted by default | `cmd/sysml/strict_redefinition_test.go:TestStrictConformanceRejectsRedefinitionResolvingToNothing` asserts exit status 2 with the code, the message naming `length` and `HotLegPipe`, and `10:27`; `internal/check/passes/strict_redefinition_test.go:TestStrictRedefinitionIsADefaultModeNoOp` asserts silence by default |
| Named diagnostic pins code, message, span and severity | `internal/check/passes/strict_redefinition_test.go:TestStrictRedefinitionResolvesToNothing` |
| Declared and inherited targets stay legal | `TestStrictRedefinitionAcceptsDeclaredAndInheritedMembers` |
| A typed usage that redefines or subsets a usage lends its inherited members to nested redefinitions | `TestStrictRedefinitionAcceptsMembersOfRedefinedTyping` (both `:>>` and `subsets`), `cmd/sysml/strict_redefinition_test.go:TestStrictConformanceAcceptsMembersOfRedefinedTyping` |
| A definition may redefine a name from its enclosing namespace's private import | `TestStrictRedefinitionAcceptsImportedTargetInDefinition` |
| A usage that subsets an enclosing feature may redefine a member of the feature's type | `TestStrictRedefinitionAcceptsTargetOfSubsettedType` |
| No committed model gains the finding | `TestStrictRedefinitionCommittedModelSweep` walks every model under `examples/`, `tests/testdata/` and the bundled `internal/workspace/libs/stdlib/`, excluding only the intentional repro fixture |
| Snapshot and timeslice bodies of an occurrence stay legal | `TestStrictRedefinitionAcceptsSnapshotOfEnclosingType` |
| A member of an enclosing type is not borrowed by a **plain typed usage** as a member of its declared type | `TestStrictRedefinitionDoesNotBorrowAnEnclosingTypeName`; the canonical repro remains rejected by `TestStrictConformanceRejectsRedefinitionResolvingToNothing` |
| An inaccessible target is left to the inheritance rule, with exactly one finding | `TestStrictRedefinitionDefersToTheInheritanceRule` |
| A snapshot failure names the occurrence's type, not the snapshot usage | `TestStrictRedefinitionSnapshotFailureNamesTheOccurrenceType` |
| Chain targets stay with their own rule | `TestStrictRedefinitionLeavesChainsToTheirOwnRule` |
| Unresolved names stay with name resolution | `TestStrictRedefinitionLeavesUnresolvedNamesToNameResolution` |
| Code is documented and explainable | `internal/check/codes` `TestCatalogueIsCurrent`, `TestCatalogueAnchorsEveryCode`; `cmd/sysml/explain_test.go` |

The repro fixture is `tests/testdata/passes/strict_redefinition_resolves_to_nothing.sysml`, read by both the pass test and the CLI test so the two cannot drift apart.

## Verification results

All commands ran from the worktree root. Parallel builds in sibling worktrees exhausted the shared `/tmp` quota, so Go's work and temporary directories were redirected with `GOTMPDIR` and `TMPDIR` to a dedicated directory; that is an environment workaround, not a change to the project.

| Gate | Result |
|---|---|
| `gofmt -l .` | clean (no output) |
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `go test -p 2 -count=1 -run 'TestStrictRedefinition' ./internal/check/passes/` | pass — thirteen tests: code/message/span/severity, default no-op, declared/inherited/owned, redefined typing, imported definition target, subsetted-type target, snapshot, enclosing-name rejection, inheritance-rule deferral, snapshot naming, chains, unresolved names, committed-model sweep |
| `go test -p 2 -count=1 -run TestStrictConformanceRejectsRedefinitionResolvingToNothing ./cmd/sysml/` | pass (exit 2, code, message, `10:27`) |
| `go test -p 2 -count=1 -run 'TestStrictRedefinition' ./internal/check/passes/ ./cmd/sysml/` (after the typed-redefining-usage fix) | pass — includes both new redefines/subsets cases and the committed-model sweep |
| `GOMAXPROCS=2 go build -o /tmp/opencode/sysml ./cmd/sysml` (after the fixes) | pass; the SportsCar reproducer exits 0 in `-validate -strict`, the canonical repro still exits 2 at `10:27` |
| `GOMAXPROCS=2 go test -p 2 -count=1 -run 'TestStrictRedefinition|TestStrictConformance' ./internal/check/passes/ ./cmd/sysml/` (after the scope refinement) | pass (both packages) |
| CLI `-validate -strict` sweep of every model under `internal/workspace/libs/stdlib/` | pass — **0 of 10 stdlib files** report `redefinition-resolves-to-nothing` (was 10 before the scope refinement) |
| CLI `-validate -strict` sweep of the six bundled runtime fixtures that had reported the code | pass — all six now report 0 |
| `make diagnostics-check` / `go test -count=1 -run TestCatalogueIsCurrent ./internal/check/codes` | pass (0.013s) |
| `go test -p 2 -count=1 ./tests/hygiene/` | pass (34.417s) |
| `go test -p 2 -count=1 -run TestSelfModelPassRegistryMatchesImplementation ./examples/` | pass (0.242s) — the self-model's `PassRegistry` now declares the pass and `passCount = 67` |
| Bounded full suite `go test -p 1 -count=1 -timeout 45m ./...` (GOMAXPROCS=2) | cancelled by a server restart after roughly 50 minutes; before cancellation `cmd/sysml` (855.398s), `internal/check/passes` (182.267s), runtime (513.918s), SMT (474.156s) and the other reached packages passed. The one failure, `TestSelfModelPassRegistryMatchesImplementation`, was fixed by updating `examples/self-model/pipeline.sysml` and now passes |
| `python3 scripts/check-doc-ids.py`, `python3 scripts/spec_compliance.py check`, `python3 scripts/changelog.py check` | pass; 57 compliance sources well-formed |
| Comment-length `awk` gate over the added Go files | empty (no comment block over two lines) |
| `make diagnostics-catalogue` | regenerated `docs/reference/diagnostics.md` from the registry |
| CLI probes for an enclosing-name case and a qualified-target case | enclosing name: only `redefinition-resolves-to-nothing` (`in Inner`); qualified target: only `redefinition-no-inherited` |
| Strict-mode sweep of every committed model under `examples/` and `tests/testdata/` | only the intentional repro fixture produced `redefinition-resolves-to-nothing`; the two snapshot/timeslice false positives found during development were removed |
| `go test -count=1 -run '^$' ./...` | pass — every test package compiles |

`go test -count=1 ./cmd/sysml/...` ran with the CI timeout of 55 minutes but the whole package hit that ceiling while the machine was contended; every test before the ceiling passed except two time-budget checks (`TestEngineSMTDecidesSensitivity`, `TestExploreTablesTheSpacecraftRaceWithinItsBudget`), which are unrelated to this change and pass when run alone: `go test -count=1 -timeout 20m -run 'TestEngineSMTDecidesSensitivity|TestExploreTablesTheSpacecraftRaceWithinItsBudget' ./cmd/sysml/` — pass (923.418s).

The two differential-baseline provenance tests and `make docs-counts` still fail because the new fixture and self-model edit move the OpenSysML-owned input counts and digests. Both recorders were attempted and blocked by the absent pinned pilot tools, and both download scripts were attempted and blocked by the absence of a JDK; the exact messages are in the known-limitations section above. Neither baseline was edited by hand.

The corpus gates printed explicit `GATE NOT RUN` banners for the absent OMG training corpus, pilot corpora, specification examples and PSSM suite; those require the download scripts and are not present in this checkout. The committed examples sweep was attempted but was OOM-killed and then timed out under the same machine load. That sweep runs in the default mode, where this pass returns no diagnostics, so its verdicts cannot move.

## Known limitations and follow-ups

- The check is name-based: a qualified target that names a different feature while a same-named member exists in the declaring context is accepted. This avoids false positives on conforming models but leaves a narrow class of wrong-qualifier cases to the existing accessibility rules.
- The *type* context widens only through snapshot and timeslice owners. A plain typed usage cannot reach a member of its enclosing type or of an imported namespace; only a definition or a usage that itself redefines/subsets gets the separate scope paths described above. This is what keeps the canonical repro rejected.
- Chain redefinitions whose leaf is absent from the type reached by their prefix are not covered here; they remain under the chain-specific rules.
- The in-tree `packaging/agent-skills/` copy of the CLI skill is updated here. The corresponding change in the separate `Open-MBEE/OpenSysML-MDK-Skills` repository is not part of this tree and remains to be opened there; this tree does not claim it was done.
- The downloaded OMG corpora and specification examples are not present in a fresh checkout, and their gates skip with explicit banners rather than passing silently. The committed examples and the bundled standard library were swept in strict mode and only the intentional repro fixture produces the new finding.
- The reviewer's 38 OMG-corpus files (28 pilot, 3 OMG training, plus the other corpus models) **cannot be re-measured in this checkout**: the download scripts were not run, and no recorded per-file verdict for `redefinition-resolves-to-nothing` exists in [pilot-corpora.md](pilot-corpora.md), [omg-issues.md](omg-issues.md) or the differential baselines. What would settle them: download the corpora at the pinned revisions and run `sysml -validate -strict` on each file beside the pinned `validate-sysml-batch`/`validate-kerml`; any file where the pilot is silent and this check reports needs its exact clause adjudicated here. The scope paths added in this round cover the same four shapes found in the bundled library (imported library target in a definition, member of a subsetted feature's type, inherited member masked in the declaring view, and a chained base feature), so the same findings are expected to clear, but that is a prediction, not a measurement. The recorded pilot rules relevant here are `validateSubsettingFeaturingTypes` / `validateRedefinitionFeaturingTypes` ([pilot-rejection-baseline.json](pilot-rejection-baseline.json), `k79`, `s104`, `s105`): the pilot judges featuring-type accessibility, not "is a member of the declaring type", which is why a library feature reached by an import is accepted there. The canonical repro is deliberately stricter than the pilot on a plain typed usage and stays rejected.
- No upstream issue tracks this capability; the nearest prior art is the unresolved-reference work in issues [#649](https://github.com/Open-MBEE/OpenSysML/issues/649)/[#650](https://github.com/Open-MBEE/OpenSysML/issues/650) and the featuring-type checks in [PR #2306](https://github.com/Open-MBEE/OpenSysML/pull/2306).
- The two differential baselines are **not re-recorded here**: the new fixture takes `tests/testdata` from 42 to 43 files and changes the `testdata` and `examples` digests, so `go test -C tools ./referee/diff ./referee/apijson -run TestCommittedBaselineStatesThisRepositorysProvenance` and `make docs-counts` still fail on provenance. Re-recording needs the pinned pilot tools, which are unavailable in this environment: `go run -C tools ./cmd/pilot-diff -update` reports `pilot validator not found at build/pilot-sysml-validator/validate-sysml-batch: run ./scripts/download-pilot-sysml-validator.sh`, and `go run -C tools ./cmd/pilot-apijson-diff -update` reports `pilot JSON exporter is absent at build/pilot-json-exporter/export-json; run ./scripts/download-pilot-json-exporter.sh to provision it`. Both download scripts fail during their Maven build with `Fatal error compiling: error: release version 21 not supported` because this machine has only a JRE (`javac` is not installed). `go run -C tools ./cmd/pilot-apijson-diff -reindex` refuses to vouch for moved provenance. A maintainer with a Java 21 JDK must run both scripts, then `go run -C tools ./cmd/pilot-diff -update` and `go run -C tools ./cmd/pilot-apijson-diff -update`, adjudicate any moved buckets in `docs/project/pilot-differential.md` and `docs/project/api-json-differential.md`, and re-run the provenance tests and `make docs-counts`. The new model is clean in the default mode and the self-model edit changes only an OpenSysML-owned input digest, so no compared-root verdict is expected to move, but that must be confirmed by the re-record rather than assumed.
