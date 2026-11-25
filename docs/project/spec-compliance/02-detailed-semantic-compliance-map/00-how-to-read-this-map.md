## Detailed Semantic Compliance Map

### How to Read This Map

Each row documents one behavioral semantic feature:

- **Semantic Rule**: the governing reference — the SysML v2 metamodel or the bundled KerML semantic library, and UML 2.5.1 only where neither has the concept (such a row is 🧩, not ✅)
- **Implementation**: File:function implementing the semantics
- **Test Case**: Conformance/robustness test(s) exercising the feature
- **Status**: 
  - ✅ **Faithful**: Implements SysML v2 / KerML semantics — a clause of either specification or an element of the Kernel Semantic or Systems Library — with test coverage
  - 🧩 **Extension**: implemented and tested, but the normative source is not SysML v2 or KerML — UML 2.5.1, PSSM or fUML where SysML v2 has no counterpart (pseudostates, call-event triggers, state-body `fork`/`join`), or an OpenSysML library or notation (`OpenSysML Libraries/*`, document queries, analysis engines, solving). The row says which. Such a row is not a SysML v2 conformance claim
  - ⚠️ **Approximate**: Partial implementation or known deviations
  - ❌ **Not Yet Implemented**: Parsed but not executable
  - ⛔ **Deliberate Divergence**: not implemented on purpose — the row states the reason, the alternative it rejects, and the test that pins the refusal
  - 🚧 **Known Failure**: Test exists but fails

Two parts of this page are not written by hand. The row census at the top is counted from the rows
when the documentation site is built, and the [analysis-library table](#analysis-libraries-measured-domain-librariesanalysis-vectorfunctions-occurrencefunctions)
is rendered by `make docs-counts` from `analysis-library-census.json`, which a runtime test writes
and holds current; `go run -C tools ./cmd/doc-counts -check` fails in CI when the rendered block and the
file disagree. Edit neither block in place: regenerate them.
