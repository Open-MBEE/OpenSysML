# Execution Conformance Schema

This directory contains behavioral execution conformance tests. Each test consists of:

1. **`<case>.sysml`** - The behavioral model (action/state/calc/constraint/requirement)
2. **`<case>.expected.json`** - Expected execution outcome
3. **`<case>.trace.golden`** - Optional ordered execution trace (see [Golden Traces](#golden-traces))

## Entry Points

A case is driven through one behavior. Without `evaluate` the harness searches
the document for the first action, state, calc, constraint or requirement it
declares — which only reaches an element declared by the document's own packages,
not one nested deeper. A case whose subject is nested — `part p { action a { … } }`
— names it by qualified path instead, and the named element must be of the kind
the case's `type` asks for:

```json
{
  "type": "action",
  "evaluate": "test::p::a",
  "outputs": {"total": {"type": "Real", "value": 6.5}}
}
```

The path is the element's fully-qualified name, `::`-separated from the outermost
package down through every owner, and must name exactly one element. It selects
the entry behavior for the golden trace too, so a nested behavior is traced as a
top-level one is. `instantiate` names an instance case's type the same way.

## Schema Format

### For Actions (`ExecuteAction`)

```json
{
  "type": "action",
  "outputs": {
    "paramName": {"type": "Integer", "value": 42},
    "result": {"type": "Real", "value": 3.14}
  },
  "tokenCount": 5
}
```

- `outputs`: map of output parameter names to their final values. The action's
  own parameters and attributes are keyed by their bare names. A nested action
  node performs in a frame of its own, so its pins — the parameters and
  attributes it declares, and those of the action it performs — are keyed by
  the node's path from the action, `p.v` or `leg.inner.v`, holding what the
  node's latest performance left there. A value a bare `perform` in a node's
  body returns to no declared feature is reported under that node the same way.
- `tokenCount`: number of tokens processed (optional, for regression detection)
- `evaluate`: qualified path of the action to execute (see [Entry Points](#entry-points))
- `error`: text the execution must fail with, for a case whose contract is a
  diagnostic rather than a result — a loop that never terminates must end with
  the step budget's error. Set it instead of `outputs`; a case without it must
  run to completion. Such a case has no golden trace, since the trace harness
  drives the same execution to the end.

### For States (`ExecuteState`)

```json
{
  "type": "state",
  "events": [{"signal": "sigB"}],
  "finalState": "Active.Cruising",
  "stateVisits": ["Off", "Idle", "Active", "Active.Accelerating", "Active.Cruising"],
  "outputs": {
    "stateData": {"type": "String", "value": "cruising"}
  }
}
```

- `events`: ordered list of events to inject into the machine. Each entry names
  either a signal (`{"signal": "sigB", "args": {...}}`, driving
  `AcceptEvent`-triggered transitions; `args` bind the signal's features by name,
  or `"value": {"type": "Integer", "value": 3}` carries one bare value as
  `send 3` would) or an operation invocation
  (`{"call": "setSpeed", "args": {"value": {"type": "Integer", "value": 55}}}`,
  driving `CallEvent`-triggered transitions), with `args` optional. Events are
  delivered in order. Optional; omit for autonomous (time/completion-driven)
  machines.
- `finalState`: qualified name of final reached state
- `stateVisits`: ordered list of states visited (optional, for golden trace verification)
- `evaluate`: qualified path of the state machine to execute (see
  [Entry Points](#entry-points))
- `outputs`: map of state machine outputs
- `performers`: objects that each perform the machine, for a case whose contract
  depends on which object performs it (two objects selecting different variants
  of one variation route over their own connections). Each entry names the
  object's usage plus the `events` / `finalState` / `stateVisits` / `outputs`
  expected of that object's performance:
  `{"object": "P::alpha", "finalState": "arrived", "stateVisits": ["start", "sending", "arrived"]}`.
  Omit for a machine performed by no object.

### For Calculations (`InvokeCalc`)

```json
{
  "type": "calc",
  "inputs": [
    {"type": "Real", "value": 10.0},
    {"type": "Real", "value": 2.0}
  ],
  "result": {"type": "Real", "value": 12.0}
}
```

- `inputs`: ordered list of input arguments
- `result`: returned value
- `evaluate`: qualified path of the calc to invoke (see [Entry Points](#entry-points))

### For Constraints (`EvaluateConstraint`)

```json
{
  "type": "constraint",
  "bindings": {
    "pressure": {"type": "Real", "value": 50.0},
    "temp": {"type": "Real", "value": 100.0}
  },
  "satisfied": true
}
```

- `bindings`: variable bindings for constraint evaluation
- `satisfied`: boolean, whether constraint is satisfied
- `evaluate`: qualified path of the constraint to evaluate, for a case declaring
  more than one. Omit to search the model for the first one (see
  [Entry Points](#entry-points))
- `instantiate`: qualified name of an object to materialize before evaluating,
  for a case whose contract is the subject the runtime picks — a condition of a
  nested definition is about the object redefining that nested feature. Omit for
  a case about the declaration.

### For Requirements (`EvaluateRequirement`)

```json
{
  "type": "requirement",
  "bindings": {
    "vehicle.speed": {"type": "Real", "value": 120.0}
  },
  "satisfied": true
}
```

- `bindings`: variable bindings for requirement evaluation
- `satisfied`: boolean, whether requirement is satisfied. `false` means a
  condition evaluated to false (`ErrViolated`), not that evaluation failed.
- `evaluate`: qualified name of the element to evaluate, for a case declaring
  more than one — a usage and the definition it is typed by. Omit to search the
  model for the first requirement (or constraint) it declares.

### For Satisfaction Assertions (`EvaluateSatisfaction`)

```json
{
  "type": "satisfy",
  "evaluate": "test::analysisContext",
  "assertions": {
    "satisfy touchdown by slowLander": true,
    "not satisfy touchdown by fastLander": true
  }
}
```

- `evaluate`: qualified name of the element stating the assertions, since
  `assert satisfy r by p;` is anonymous and is reached through its owner. Omit
  to evaluate every assertion in the model.
- `assertions`: expected verdict per assertion, keyed by the assertion as
  written (`not ` prefixed for a negated one). `false` means the requirement
  evaluated to false against the object its subject binds (`ErrViolated`), not
  that evaluation failed.
- `satisfied`: the verdict, for a case stating exactly one assertion.
- `error`: text the evaluation must fail with, for a case whose contract is a
  diagnostic — satisfying a requirement that states no condition. Set it
  instead of a verdict.

### For Verification Cases (`RunVerification`)

```json
{
  "libraries": true,
  "type": "verification",
  "evaluate": "test::checkZeroed",
  "verdict": "pass",
  "subcases": {"test::plan::checkDrifted": "fail"},
  "verdicts": {"obj": "satisfied"}
}
```

- `verdict`: the `VerdictKind` the run of the case's body produced — `pass` or
  `fail` as the library's `PassIf` calculation computed it, `inconclusive` for a
  body that bound no verdict value, `error` for a body whose run could not be
  carried out.
- `verdictDetail`: text the verdict carries, matched as a substring — the
  message of an `error` verdict, or why an `inconclusive` one decided nothing.
- `subcases`: the verdict of each verification case the body performs, by
  qualified name. The library states no roll-up of a subcase's verdict into its
  parent's, so each is stated on its own.
- `evaluate`, `subject`, `inputs`, `bindings`, `outputs`, `verdicts` and `reads`
  mean what they mean for an analysis case: a verification case runs the same
  body and reports the same objective and assertion verdicts beside its own.
  A case whose body could not run states no outputs or verdicts.

### For Instances (`Instantiate`)

```json
{
  "type": "instance",
  "instantiate": "test::Vehicle",
  "slots": {
    "mass": {"type": "Real", "value": 1500.0},
    "doubled": {"type": "Real", "value": 3000.0}
  },
  "constraints": {
    "withinLimit": true,
    "overLimit": false
  }
}
```

- `instantiate`: qualified name of the type to instantiate
- `slots`: expected values of the instance's own slots, materialized on demand
  — a default expression that reads sibling features (including through a
  nested part, `mass + engine.derated`) is evaluated against this object rather
  than constant-folded. Keys are slot names, not paths.
- `constraints`: expected verdict per constraint feature the instance carries,
  evaluated bound to the instance. `false` means the assertion evaluated to
  false (`ErrViolated`), not that evaluation failed.
- `error`: text the instantiation must fail with, for a case whose contract is a
  diagnostic — a declaration valuing one feature under two of its names. Set it
  instead of `slots`.

## Diagnostics

```json
{"diagnostics": ["expected ';' after transition"]}
```

A case fails on any diagnostic its model reports that it does not declare here,
matched as a substring, and on a declaration nothing reported. Omit the field for
a model that parses clean, which is what a case asserting a result should be:
declare a diagnostic only when reporting it is part of the case's contract.

## Standard Library

```json
{"libraries": true}
```

Loads the standard library into the case's index, for a case whose model names
library elements the runtime resolves — the measurement unit of a quantity
expression (`1.5 [m/s]`) is one. Omit it otherwise: a case that needs no library
is indexed from its own source alone.

## Value Format

All values use this format:

```json
{"type": "TypeName", "value": <JSON-serializable>}
```

Supported types:
- `Integer`: JSON number (no decimals)
- `Real`: JSON number (may have decimals)
- `Boolean`: JSON boolean
- `String`: JSON string
- `Null`: JSON null
- `Quantity`: JSON number, with the `unit` the magnitude is written in
- `MeasurementRef`: the `unit` a measurement reference names, and no `value`
  (`{"type": "MeasurementRef", "unit": "m**2"}`)
- `CoordinateFrame`: the `text` a coordinate frame or measurement scale prints
  as, its name over its axes, and no `value`
  (`{"type": "CoordinateFrame", "text": "spatialCF [m, m, m]"}`)
- `CoordinateTransformation`: the `text` a transformation prints as, its name
  over the frames it relates (`{"type": "CoordinateTransformation", "text": "trs (datum → lbcf)"}`)
- `Complex`: JSON number, the real part, with the imaginary part as `im`
  (`{"type": "Complex", "value": 0.0, "im": 1.0}`)
- `Sequence`: the `elements` it holds, in order, instead of `value` — for a
  multi-valued feature, whose order is part of its contract
- `Instance`: an object, whose identity a case does not pin (no `value`)
- `Unset`: a valueless feature of a value type, holding no value (no `value`)
- `Variant`: the name of the variant a variation feature is bound to, as a JSON
  string (`{"type": "Variant", "value": "cutIdeal"}`)
- `EnumLiteral`: the enumeration literal a value is, written as the enumeration
  declaring it qualifies it (`{"type": "EnumLiteral", "value": "Color::red"}`)

In place of a value, `error` states the text producing that value must fail with,
for a slot or result whose contract is a diagnostic (`{"error": "not a
measurement unit: …"}`).

## Adding New Cases

1. Create `<case>.sysml` with the behavioral model
2. Create `<case>.expected.json` with expected outcome
3. Run `go test ./internal/core/runtime/ -run TestExecutionConformance -v`
4. If test fails but behavior is correct, verify and update expected file
5. If behavior is unimplemented, add case name to `known_failures.txt`

## Golden Traces

A case may also carry a `<case>.trace.golden` recording the order in which the
case executes, checked by `TestExecutionTrace`. Calc and constraint cases record
calc evaluation: parameter binding, every sub-expression, and results.

```
enter calc test::scale
  bind x = 3 [argument]        # argument, or default when none is passed
  bind factor = 4 [default]
    eval feature x -> 3        # indentation is sub-expression nesting
    eval feature factor -> 4
  eval operator * -> 12
exit calc test::scale -> 12    # or `-> error: <typed error>`
```

Entries are canonical, never positions or addresses: parameters bind in
declaration order (inherited parameters first, at the position the declaring calc
gives them), and an unordered value such as a set renders sorted.

`go test ./internal/core/runtime -run TestExecutionTrace -update-traces`
regenerates every golden the harness owns in one run, so an intentional ordering
change needs no hand-editing; review every diff — a reordered entry is a
behavior change, not noise. A no-op run rewrites the same bytes and leaves
`git status` clean.

The harness owns a golden for a case that already carries one, and for a case
whose expectation sets `"trace": true`, which is how a new case asks for one to
be written. Every owned case must execute and produce a trace: an update run
reports one that does not rather than leaving a stale golden behind. State cases
that broadcast an event over orthogonal regions have no order-stable trace yet,
so they neither carry a golden nor opt in.

## Known Failures

Cases in `known_failures.txt` are skipped (logged as `SKIP`). Remove from file when implemented.

## Provenance

Where possible, expected outcomes are derived from the OMG SysML v2 Pilot Implementation (commit 4c289b926). Cases with pilot-derived expectations are marked in comments.
