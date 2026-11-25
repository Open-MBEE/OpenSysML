### Independent Static Cross-Check — Sensmetry SysIDE

A third implementation ([SysIDE](https://github.com/sensmetry/sysml-2ls), TypeScript, pinned at
`0.9.1` with the `2024-12` standard library) gives an optional third verdict per file in
`tools/referee/diff`, provisioned by `scripts/download-syside.sh`. **It corroborates; it never
adjudicates:** the pinned OMG pilot (`2026-09`) stays the reference every conclusion in
[pilot-differential.md](pilot-differential.md) rests on, and the third column is additive — with
SysIDE absent the report is byte-identical to the committed baseline.

**What it can be evidence for:** static checking only — parsing and notation acceptance, name
resolution, static expression typing, and the KerML/SysML validation rules. **SysIDE executes
nothing**, so it is not evidence for or against any behavioral row of this document (Action, State
Machine, Classifier Behaviors, expression-body scope): those are execution semantics and no
`syside` verdict speaks to them. Its standard library is also one release behind the pilot's, so a
parse or resolution difference may be a `2024-12`/`2026-09` difference rather than a finding.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| A third implementation's static verdict is reported per file beside ours and the pilot's — which tool said what is always visible, and no existing classification, total or bucket changes when it is present | `tools/referee/diff/syside.go` `attachSyside`, `compareSysideFile` (a second, independent partition of the same tuples), `categorizeSyside` (deliberately under-mapped: an unrecognized SysIDE rule stays `unmapped` rather than manufacturing agreement) | `tools/referee/diff/f7_syside_test.go` | ✅ Faithful (static rows only — 349 files: all three agree exactly on 248, SysIDE corroborates all 20 of our agreed diagnostics, sides with us against the pilot on 7 and with the pilot against us on 37; behavioral rows are out of its scope by construction) |

---
