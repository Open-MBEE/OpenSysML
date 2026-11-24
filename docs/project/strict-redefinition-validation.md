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

A new constraint-tier pass, `StrictRedefinitionPass`, registered in the default pass registry. In strict mode it walks the document's features; for each `:>>` clause it takes the written member name, resolves the target, and asks whether the type declaring the feature offers a member of that name. It does so with the *declaring* member view, so the redefinition being checked does not mask its own target, and it considers the declared types of the owning usage and of the enclosing usages — the latter is what makes a snapshot or timeslice redefinition of the snapshotted occurrence's type legitimate. When no type in that context offers the name, the pass reports severity `error`, code `redefinition-resolves-to-nothing`, at the written target span, with a message naming the member and the type.

Chain redefinitions (`:>> a.b`) are left to the rule that already judges them by where their prefix is declared. Names that resolve to no element are left to name resolution. The pass returns no diagnostics at all in the default mode, so models using OpenSysML's looser name lookup are unchanged.

## Design decisions and alternatives rejected

- **A new pass rather than widening `checkRedefinition`.** The existing check is always-on. Adding a finding there, even gated on mode, would couple a strict-only rule to an always-on checker and make the default-mode no-op harder to guarantee. A separate pass makes the mode guard the first statement and keeps the strict behaviour independently testable.
- **No change to `CanAccess` or the featuring-type rules.** Those model a different question (whether a feature can be accessed from a featuring context) and are deliberately lax for unfeatured library features. Tightening them would change hidden behaviour in default mode and break existing models.
- **No parser or AST work.** The notation is already valid; this is a semantic validity question about the relationship's target, so it belongs in a check pass over the symbol tree and the semantic model's member view.
- **No export-side repair.** Dropping the dangling handle in the writer would hide the invalid input instead of rejecting it, and would leave strict validation answering a different question from the element form.
- **No new flag or mode.** Strict conformance already exists on every surface; a separate switch would fragment the meaning of `-strict`.
- **Name-based lookup in the declaring view, not symbol identity.** A qualified target that names an equivalent inherited member is accepted rather than reported; this favours a small number of missed exotic cases over false positives on conforming models.
- **Only portions widen the declaring context.** An earlier version added every enclosing scope owner, which let a member of an enclosing type stand in for a member of the declaring type. The context now widens only while the immediate owner is a snapshot or timeslice, because those bodies read names from the occurrence they portion. An ordinary nested definition or usage contributes nothing from its parent.
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
| Snapshot and timeslice bodies of an occurrence stay legal | `TestStrictRedefinitionAcceptsSnapshotOfEnclosingType` |
| A member of an enclosing type is not borrowed as a member of the declaring type | `TestStrictRedefinitionDoesNotBorrowAnEnclosingTypeName` |
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
| `go test -p 2 -count=1 -run 'TestStrictRedefinition' ./internal/check/passes/` | pass — nine tests: code/message/span/severity, default no-op, declared/inherited/owned, snapshot, enclosing-name rejection, inheritance-rule deferral, snapshot naming, chains, unresolved names |
| `go test -p 2 -count=1 -run TestStrictConformanceRejectsRedefinitionResolvingToNothing ./cmd/sysml/` | pass (exit 2, code, message, `10:27`) |
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

The corpus gates printed explicit `GATE NOT RUN` banners for the absent OMG training corpus, pilot corpora, specification examples and PSSM suite; those require the download scripts and are not present in this checkout. The committed examples sweep was attempted but was OOM-killed and then timed out under the same machine load. That sweep runs in the default mode, where this pass returns no diagnostics, so its verdicts cannot move.

## Known limitations and follow-ups

- The check is name-based: a qualified target that names a different feature while a same-named member exists in the declaring context is accepted. This avoids false positives on conforming models but leaves a narrow class of wrong-qualifier cases to the existing accessibility rules.
- The declaring context widens only through snapshot and timeslice owners. A redefinition inside an ordinary nested definition or usage cannot reach a member of its enclosing type, which is deliberate; a legitimate redefinition of such a member must be written where that type is declared or inherited.
- Chain redefinitions whose leaf is absent from the type reached by their prefix are not covered here; they remain under the chain-specific rules.
- The in-tree `packaging/agent-skills/` copy of the CLI skill is updated here. The corresponding change in the separate `Open-MBEE/OpenSysML-MDK-Skills` repository is not part of this tree and remains to be opened there; this tree does not claim it was done.
- The downloaded OMG corpora and specification examples are not present in a fresh checkout, and their gates skip with explicit banners rather than passing silently. The committed examples were swept in strict mode and only the intentional repro fixture produced the new finding.
- No upstream issue tracks this capability; the nearest prior art is the unresolved-reference work in issues [#649](https://github.com/Open-MBEE/OpenSysML/issues/649)/[#650](https://github.com/Open-MBEE/OpenSysML/issues/650) and the featuring-type checks in [PR #2306](https://github.com/Open-MBEE/OpenSysML/pull/2306).
