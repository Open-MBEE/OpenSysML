# Testing Strategy

OpenSysML uses a **multi-layer test contract** to keep parsing, semantic analysis, and execution correct and to catch regressions in each.

## Test Organization

```
internal/core/
├── parser/
│   ├── golden_test.go              # Golden AST snapshots
│   ├── negative_test.go            # Malformed input handling
│   └── testdata/parse/             # Test fixtures + goldens
├── runtime/
│   ├── conformance_test.go         # Execution outcome verification, under every policy
│   ├── trace_test.go               # Execution ordering/scheduling
│   ├── explore_test.go             # Every linearization within a budget
│   ├── robustness_test.go          # Failure mode handling
│   └── testdata/conformance/       # Behavioral test cases
└── libs/
    └── stdlib_conformance_test.go  # Standard library gate
```

---

## Parser Test Contract

New grammar features require a **four-layer test contract**:

### 1. Conformance Gate

**Purpose:** Ensure standard library continues to parse cleanly

- **Test:** `TestStdlibConformance` (internal/core/libs/)
- **Coverage:** 100/100 bundled library files — the 94 official SysML v2 standard library files and six non-normative OpenSysML extensions
- **Acceptance:** All stdlib files parse without errors
- **Allowlist:** `testdata/stdlib_known_failures.txt` (currently empty)

```bash
go test -v -run TestStdlibConformance ./internal/core/libs
```

### 2. Golden AST Snapshots

**Purpose:** Verify AST structure matches expected output

- **Test:** `TestGolden` (internal/core/parser/)
- **Fixtures:** `testdata/parse/*.sysml` and `*.kerml` (one representative file per construct)
- **Goldens:** `testdata/parse/*.golden` (AST dumps)
- **Acceptance:** Parse output matches golden file

**Update goldens after intentional changes:**
```bash
go test -run TestGolden -update ./internal/core/parser
```

**Coverage includes:**
- Package/namespace declarations
- Part/attribute definitions and usages
- Connections and relationships
- Requirements and constraints
- State machines and transitions
- Actions (control flow, parameters, nested)
- Calculations and expressions
- Enumerations and metadata

### 3. Round-Trip Serialization

**Status:** Explicitly deferred (no faithful SysML printer exists)

Future work: If SysML printer added, verify `parse(print(parse(input))) == parse(input)`

### 4. Negative Test Suite

**Purpose:** Verify parser rejects malformed input gracefully

- **Test:** `TestNegative` (internal/core/parser/)
- **Coverage:** one subtest per malformed input — count in [the measured counts](../project/spec-compliance.md)
- **Acceptance:** Each case produces diagnostics (never panics)

**Examples:**
- Unclosed blocks
- Unexpected tokens
- Invalid syntax
- Incomplete behavioral members

---

## Behavioral Test Contract

New behavioral features (actions, states, calc, constraints, requirements) require a **four-layer test contract**:

### 1. Golden AST Fixtures

**Purpose:** Lock in parse structure before execution changes

- **Location:** `internal/core/parser/testdata/parse/` (behavioral fixtures)
- **Coverage:** the behavioral fixtures among the whole set — count in [the measured counts](../project/spec-compliance.md)
- **Acceptance:** `TestGolden` passes, AST dumps match expectations

**Behavioral fixtures:**
- `action_control_flow.sysml`, `action_if_branch_body.sysml`, `action_mixed_params.sysml`, `action_send_port.sysml`
- `state_full.sysml`, `state_transition_variants.sysml`, `state_call_trigger.sysml`, `state_def_region_pseudostate.sysml`, `state_defer.sysml`, `state_fork_join.sysml`, `state_history.sysml`, `state_timed_triggers.sysml`, `state.sysml`
- `calc.sysml`, `calc_defaults_and_invocation.sysml`, `calc_return.sysml`, `calc_return_parameter.sysml`
- `constraint_assert_assume.sysml`
- `requirement.sysml`, `requirement_members.sysml`

### 2. Execution Conformance Gate

**Purpose:** Verify behavioral execution produces expected outcomes

- **Test:** `TestExecutionConformance` (internal/core/runtime/)
- **Format:** `.sysml` + `.expected.json` pairs
- **Schema:** `internal/core/runtime/testdata/conformance/README.md`
- **Allowlist:** `known_failures.txt` (currently empty)
- **Admissible outcomes:** a case whose model admits several complete results lists them under `outcomes`, each with an `admissible` citation into [the semantic oracle](../project/behavior-semantic-oracle.md) deriving why the library leaves the order open; the observed run must match exactly one. The case is also explored (`explore` policy): every listed outcome must be reached, nothing unlisted may be, and the exploration must complete within the case's `exploreBudget`
- **Policy sweep:** `TestExecutionConformanceUnderPolicies` runs the whole suite under `declared` and `seed:1`. A case with no `schedule` pin was recorded under the default and must hold under any policy; one that differs was pinning a scheduling artefact and fails rather than being skipped. Pinning `schedule: "reverse"` declares the result one linearization, kept only until its admissible set is derived or the bug it pins is fixed
- **Design:** [scheduling policies, choice points and exploration](design/scheduling.md)

**Coverage (all passing, by fixture prefix; counts in [the measured counts](../project/spec-compliance.md)):**
- Calc: parameter binding, return values, defaults, inherited parameters, unary ops, qualified names, type coercion, body-local usages, statement bodies, nested and from-constraint invocation
- Action: token flow, outputs, nested invocation, send/accept, port communication, `perform` reference and shorthand, accept...then, flows, loops and decisions
- State: simple, do behavior, concurrent do, transition effect, choice/junction/fork-join pseudostates, orthogonal regions and region pseudostates, shallow/deep history, deferred/undeferred events, call and timed triggers, signal discrimination/unmatched, self signal
- Requirement: require, subject, actor, assume, nested
- Instance, unit and quantity, constraint assert/assume/negation, satisfy, variation, redefinition, variant, feature chains, string operations, nested behaviors, element filters, ball-and-chain, and one each of attribute, connector, cubesat and view

```bash
go test -v -run TestExecutionConformance ./internal/core/runtime
```

### 3. Golden Execution Traces

**Purpose:** verify *how* execution proceeds (ordering, scheduling), not only the final result

- **Test:** `TestExecutionTrace` (internal/core/runtime/)
- **Format:** `.trace.golden` files, one linearization each; a case with `outcomes` owns a `<case>.<policy>.trace.golden` per sweep policy beside the default one
- **Order constraints:** a `.trace.order` file states the partial order a trace must respect, one `a < b` per line — the first trace entry mentioning `a` before the first mentioning `b`; labels the same entry first mentions are unordered, and a label no entry mentions fails. It is checked beside a golden, or instead of one where every linearization is admissible; `TestTraceOrderViolationFails` proves a violated constraint fails
- **Determinism:** Token sorting by ID, fixed event queue tie-breaking; each `choice` the run made is a trace line naming the alternatives and the one taken
- **Coverage:** `.trace.golden` files for action, calc, state, constraint, accept and string execution

**Trace format examples:**
- Action: `step 1: token T1@node1, token T2@node2` (sorted)
- State: `entry: StateName [hasEntryAction]`, `transition: From -> To [event]`, `exit: StateName [hasExitAction]`
- Choice: `choice step 2: tokens 3@left, 4@right (unordered; took 4@right first)`

**Generate traces:**
```bash
go test -run TestExecutionTrace -update-traces ./internal/core/runtime
```

### 4. Runtime Robustness Tests

**Purpose:** Verify malformed/pathological behaviors fail gracefully

- **Test:** `TestRuntimeRobustness` (internal/core/runtime/)
- **Coverage:** one subtest per failure mode — count in [the measured counts](../project/spec-compliance.md)
- **Acceptance:** Typed errors, never panic, 60s timeout guard

**Failure modes:**
- Deadlocked action (join starvation)
- Decision with no satisfied guard
- State machine with dangling transition
- Sourceless accept...then written first in its body or after a member that is not a state
- Calc with unbound parameter, surplus or unknown-named arguments, no result, non-calc target, direct or mutual recursion
- Constraint referencing missing feature
- Step budget exceeded
- Fork/join misuse (branches sharing a region, join with one incoming branch)
- Region pseudostate with no satisfied guard, or a cycle
- Non-numeric time trigger
- Send that reaches only its addressee, accept of an unsent type, send through an unconnected port
- History outside a composite state, or without a record or default
- Defer of a non-deferrable trigger
- Non-terminating do behavior
- Call of an unhandled operation, call argument of the wrong type
- `perform` of a missing action, `perform` reference cycle

```bash
go test -v -run TestRuntimeRobustness -timeout 60s ./internal/core/runtime
```

---

## Unit & Integration Tests

- **Unit tests:** Per-package coverage (lexer, parser, semantics, runtime)
- **Integration tests:** End-to-end workspace/REPL scenarios (internal/core/model/)
- **Test fixtures:** `testdata/*.sysml`, `testdata/*.kerml`
- **Golden files:** Expected parse/resolve/diagnostic outputs

**Run all tests:**
```bash
go test ./...
```

**Run tests with coverage:**
```bash
go test -cover ./internal/core/...
```

---

## Contributing New Features

### Grammar Features

When adding parser support for new SysML v2 constructs:

1. ✅ Add representative example to `testdata/parse/*.sysml`
2. ✅ Run `go test -run TestGolden -update` to generate golden
3. ✅ Verify `TestStdlibConformance` still passes (no regressions)
4. ✅ Add negative test case if construct has error conditions

### Behavioral Features

When adding execution support for actions, states, calc, constraints, requirements:

1. ✅ Add golden AST fixture to `internal/core/parser/testdata/parse/` (if not already covered)
2. ✅ Implement semantics in `internal/core/runtime/` (executor or evaluator)
3. ✅ Add conformance case: `.sysml` + `.expected.json` in `internal/core/runtime/testdata/conformance/`
4. ✅ Add golden trace case: `.trace.golden` for ordering-sensitive features
5. ✅ Add robustness test for failure modes (deadlock, unbound params, missing refs)
6. ✅ Update `docs/project/spec-compliance.md` with semantic rule → implementation → test → status
7. ✅ Verify all tests pass: `go test ./internal/core/parser/ ./internal/core/runtime/`

---

## Test Coverage Policy

- **Parser:** Golden ASTs + negative tests + stdlib conformance
- **Behavioral execution:** Conformance + traces + robustness
- **Semantics:** Unit tests for resolution, type system, validation
- **No coverage target:** Quality over percentage (each feature has explicit test contract)

**Rationale:** Test *contracts* (what must pass) > coverage metrics (% lines hit). Each feature has defined acceptance criteria.
