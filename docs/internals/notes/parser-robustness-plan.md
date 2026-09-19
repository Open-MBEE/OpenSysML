# Parser Robustness & Correctness — COMPLETE

**Status:** All Phases Delivered (Phases 1-6)
**Audience:** Historical reference for parser evolution. For active development, see test contracts in `docs/internals/architecture.md`.

---

## Summary

Parser evolved from coverage-driven (per-file whack-a-mole) to grammar-driven design with comprehensive test harness. All six phases delivered:

### Phase 1: Conformance Gate ✅
- Created `internal/workspace/libs/stdlib_conformance_test.go`
- Stdlib gate: 94/94 files clean (no allowlist)
- Hard failing signal for parser regressions

### Phase 2: Correctness Harness ✅
- Golden AST fixtures: `tests/parser/testdata/parse/*.{sysml,golden}`
- Negative tests: `internal/syntax/parser/negative_test.go`
- Round-trip tests: `internal/syntax/parser/integration_test.go`
- Catches silently-wrong ASTs (not just diagnostics)

### Phase 3: Unified Member Parsing ✅
- Replaced per-body keyword whitelists with general member parser
- Added checkpoint/restore backtracking infrastructure
- Removed terminal errors from body parsers (graceful fallback)
- Stdlib: 94/94 clean maintained

### Phase 4: Syntax/Semantics Separation ✅
- Semantic decisions inventoried (`docs/phase4_semantic_inventory.md`)
- Datatype def-vs-usage inference fixed
- Parser focuses on syntactic structure, semantics downstream

### Phase 5: Grammar Traceability ✅
- Vendored OMG SysML-v2 grammar (KerML.xtext, SysML.xtext)
- Created `docs/grammar/PRODUCTION_MAP.md` (50+ mappings)
- ADR 0001: Rationale for hand-written recursive descent parser

### Phase 6: Documentation Reconciliation ✅
- Status claims updated to measured reality (no superlatives)
- Four-layer test contract documented in `ARCHITECTURE.md`
- Contributing guidelines reference test contract

---

## Test Coverage

**Parser Test Layers:**
1. **Conformance Gate**: 94/94 stdlib files clean (zero allowlist)
2. **Golden ASTs**: 17 fixtures (10 structural + 7 behavioral)
3. **Negative Tests**: 15 cases (malformed syntax)
4. **Round-trip**: Integration tests verify parse → dump → parse stability

**Key Achievements:**
- Zero terminal errors in body parsers (graceful fallback)
- No keyword whitelists (general member grammar unified)
- All syntax decisions in parser, semantic decisions downstream
- Grammar traceability to OMG pilot reference implementation

---

## Active Test Contracts

See `docs/internals/architecture.md` for current test requirements:
- **Parser Test Contract**: 4 layers (conformance, golden, negative, round-trip)
- **Behavioral Test Contract**: 4 layers (golden ASTs, conformance, traces, robustness)

---

## Verification Commands

```bash
# Stdlib conformance gate
go test ./internal/workspace/libs/ -run TestStdlibConformance -v

# Golden AST fixtures
go test ./tests/parser -run TestGolden -v

# Negative tests
go test ./tests/parser ./internal/syntax/parser -run TestNegative -v

# Round-trip tests
go test ./internal/syntax/parser/ -run TestIntegration -v

# All parser tests
go test ./internal/syntax/parser/ -v
```

---

## Historical Implementation Details

For detailed implementation notes, see git history:
- Phase 1: Commits establishing conformance gate
- Phase 2: Commits adding golden/negative test infrastructure
- Phase 3: Commit 2c9e6ac (unified member parsing)
- Phase 4: Commits 3eee3d4, cd5c162 (datatype fix)
- Phase 5: Commit 8bfaf0c (grammar traceability)
- Phase 6: Commit f18af39 (doc reconciliation)

Parser evolution complete. Focus shifted to behavioral robustness (see `docs/BEHAVIOR_ROBUSTNESS_PLAN.md`).
