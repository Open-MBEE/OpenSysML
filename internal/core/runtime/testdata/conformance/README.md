# Execution Conformance Schema

This directory contains behavioral execution conformance tests. Each test consists of:

1. **`<case>.sysml`** - The behavioral model (action/state/calc/constraint/requirement)
2. **`<case>.expected.json`** - Expected execution outcome
3. **`<case>.trace.golden`** - Optional ordered execution trace (see [Golden Traces](#golden-traces))
4. **`<case>.trace.order`** - Optional partial-order constraints the trace must satisfy (see [Order Constraints](#order-constraints))

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
- `schedule`: the scheduling policy the case was recorded under (see
  [Scheduling Policy](#scheduling-policy)); omitted means the default.

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
- `finalState`: qualified name of final reached state; for a machine ending in
  orthogonal regions, their active states joined by `+` in region name order
  (`d2+deep+r2`)
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
  Omit for a machine performed by no object. Such a case may carry a golden
  trace too: it records each object's materialization followed by its own
  performance, in the order the performers are listed.

### Admissible Outcomes

An action or state case whose model leaves more than one result open — two
concurrent branches writing one feature, whose order the Kernel Semantic Library
does not fix — lists every result it admits under `outcomes` instead of stating
one:

```json
{
  "type": "action",
  "outcomes": [
    {"outputs": {"x": {"type": "Integer", "value": 1}, "leftRan": {"type": "Boolean", "value": true}}},
    {"outputs": {"x": {"type": "Integer", "value": 2}, "leftRan": {"type": "Boolean", "value": true}}}
  ],
  "admissible": "Concurrent branches writing one feature: the value is open, the writes are not"
}
```

- `outcomes`: at least two complete results. Each entry carries the `outputs` of
  an action case, or the `finalState` / `stateVisits` / `outputs` of a state
  case, with the meaning those keys have above. The observed run must match
  exactly one entry: matching none fails the case as inadmissible, matching
  several fails it because the set is not distinct. `-v` output names the entry
  matched (`matched admissible outcome 1 of 2`).
- `admissible`: required beside `outcomes`. The exact title of the section of
  `docs/project/behavior-semantic-oracle.md` deriving that every listed outcome is
  valid under the Kernel Semantic Library. A missing citation, or one no section
  carries, fails the case — an admissible set exists to state what the library
  leaves open, never to accommodate a result the executor should not produce.

A case states either `outcomes` or the single `outputs` / `finalState` /
`stateVisits`; stating both is a schema error the test reports, as is `outcomes`
beside `performers`, on a calc, constraint, requirement or instance case, or
with an entry that states nothing. The `events` a state case injects are its
input and stay at the top level: every outcome is a result of the same run.

The set is exact. Besides checking the default run, `TestExecutionConformance`
explores every case with `outcomes` under the `explore` policy: the case is
replayed from the start once per linearization the library admits, each run on a
fresh context over the same lowering, and the case fails when a listed outcome
no run reaches (`admissible outcome 2 of 3 is unreachable`), when a run reaches
an outcome the set does not list or fails with an error — named with the choice
sequence of a witness run (`step 1: t2 first of t1, t2, t3; step 2: t1 first of
t1, t3`) — or when the exploration is incomplete. Exploration is bounded by a
budget of runs and of choice points per run, `runs: 1024, depth: 64` by default;
a case that hits it fails with a message telling the author to raise it:

```json
{
  "type": "action",
  "exploreBudget": {"runs": 4096, "depth": 128},
  "outcomes": [ ... ],
  "admissible": "..."
}
```

- `exploreBudget`: optional beside `outcomes`; each of `runs` and `depth` defaults
  to the default budget's when omitted. A budget without `outcomes`, a `runs`
  below 1 or a `depth` below 0 is a schema error the test reports. Raising a
  budget is the answer to a model with more linearizations than the default
  covers, never to an outcome the set is missing: an unlisted outcome is a
  derivation to add to the oracle or a bug to fix.

Cases without `outcomes` are not explored by the harness. The default schedule is
deterministic, so a case with an admissible set still keeps its exact golden trace.

### Scheduling Policy

The executor resolves the choice points a run reports — several steppable
tokens in one step, several holding decision guards, several transitions out of
one state enabled by one event, several executors due at one instant of the
clock — under a scheduling policy, spelled the same way everywhere
(`sysml -schedule`, `%schedule`, the `schedule` request field):

| Policy | Resolution |
|--------|------------|
| `reverse` | The default: tokens in reverse spawn order, the first holding guard, the first enabled transition, the executor started last first |
| `declared` | Tokens in spawn order, the first holding guard, the first enabled transition, the executor started first first |
| `seed:<n>` | Every resolution drawn from a pseudo-random sequence the non-negative integer `n` fixes; the same seed replays the same run |

Which of two same-step writes to one feature stands follows from the token order
the policy chose; a write conflict is reported, not resolved on its own.

A case may pin the policy it was recorded under with `"schedule": "<policy>"`;
the harness then runs it under that policy in every test, whatever policy the
test asked for. Omitted or empty means the default. A pin that names no policy
is a schema error the test reports.

`TestExecutionConformanceUnderPolicies` runs every case under `declared` and
under `seed:1`. A case pinning no policy was recorded under the default, so its
stated result, or one of its `outcomes`, must hold under any policy; one that
differs has been pinning a scheduling artefact as *the* result. Such a case is
pinned to `reverse` — never removed from the sweep — until either the outcomes
the library admits are derived in `docs/project/behavior-semantic-oracle.md` and
the case restated as an admissible set (`action_choice_shared_message_accept`,
two accepts racing for two sends, was), or the difference is found to be a bug
and the pin stays until the fix lands (`send_identity_same_named_ports` was
pinned while the via-less `accept Ping` over-matched a transfer addressed to
`alpha.inPort`; with a via-less accept held to the receiver the transfer reaches,
`waiting` has one enabled transition and the case runs unpinned).

A case with an admissible set also owns a `<case>.<policy>.trace.golden` for
each sweep policy (`declared`, `seed-1` — a colon is not a portable file-name
character), recording the linearization that policy takes; `-update-traces`
regenerates them beside the default golden.

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
- `Infinity`: the unbounded `*`, which is no number and carries no `value`
  (`{"type": "Infinity"}`)
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
- `Set`: the distinct `elements` it holds, in the canonical order a set
  enumerates in (booleans, numbers, strings, quantities, enumeration literals,
  objects; each class in its own order) — for a `Collections::Set`'s elements,
  or any other feature the library declares unique and unordered
- `Instance`: an object, whose identity a case does not pin (no `value`)
- `Unset`: a valueless feature of a value type, holding no value (no `value`)
- `Variant`: the name of the variant a variation feature is bound to, as a JSON
  string (`{"type": "Variant", "value": "cutIdeal"}`)
- `EnumLiteral`: the enumeration literal a value is, written as the enumeration
  declaring it qualifies it (`{"type": "EnumLiteral", "value": "Color::red"}`)
- `Function`: the qualified name of the calc a function value is a value of
  (`{"type": "Function", "value": "test::Sq"}`)

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

## Order Constraints

A case may carry a `<case>.trace.order` stating the partial order its trace must
respect, checked by `TestExecutionTrace` beside the exact golden, or instead of
one for a case that carries no golden and does not opt into one. Each line is
`a < b`: the first entry mentioning label `a` comes strictly before the first
entry mentioning label `b`. Blank lines and `#` comments are skipped; a file
with no constraint, or a line of any other shape, fails the case.

```
# The fork precedes both branches; the join waits for both.
split < left
split < right
left < sync
right < sync
```

A label is the text a trace entry names a performance or statement by, exactly
as it appears in `.trace.golden`:

- a **performance label** is a node identifier — `split` in
  `step 1: token 1@split`, `inner` in `enter action node: inner`, or a state name
  `Idle` in `enter: Idle` / `enter: Idle (entry action)`. A step entry mentions
  every node a token is at, so `left` and `right` are both mentioned by
  `step 2: token 2@left, token 3@right`;
- a **statement label** is the text after `stmt `, `assign x` in `stmt assign x`.

Token numbers, `eval` entries and values are not labels. Two labels first
mentioned by the same entry — two tokens stepped together — are unordered, so a
constraint between them fails in both directions; a constraint naming a label no
entry mentions fails as well. The failure reports which entries mention each
label.

## Known Failures

Cases in `known_failures.txt` are skipped (logged as `SKIP`). Remove from file when implemented.

## Provenance

Where possible, expected outcomes are derived from the OMG SysML v2 Pilot Implementation (commit 4c289b926). Cases with pilot-derived expectations are marked in comments.
