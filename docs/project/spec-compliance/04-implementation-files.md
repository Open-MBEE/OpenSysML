## Implementation Files

### Runtime Execution (`internal/exec/runtime/`)

| File | Purpose |
|------|---------|
| `context.go` | Execution context, constraint/requirement evaluation |
| `invoke_calc.go` | Calc invocation: parameter/result resolution across specialization, binding, recursion bound |
| `action_executor.go` | Token-flow semantics, control flow nodes, nested actions |
| `state_executor.go` | Event-driven state machines, transitions, hierarchical states, pseudostates |
| `eval.go` | Expression evaluation (operators, literals, features) |
| `value.go` | Runtime value representation (ValConst, ValString, ValInstance) |
| `trace.go` | Deterministic execution and calc-evaluation trace recording, canonical value rendering |
| `conformance_test.go` | Conformance gate |
| `robustness_*_test.go`, `robustness_test.go` | Failure-mode tests, one file per feature, with the shared helpers |
| `trace_test.go` | Golden trace test infrastructure |
| `trace_calc_test.go` | Trace determinism and canonical rendering unit tests |

### Run renderings

| Run output | Implementation | Tests | Status |
|---|---|---|---|
| State occupancy over clock time, per object machine, with transitions and choice/guard marks; parallel leaves are keyed by state and region, colliding labels use the shortest distinguishing state path or region, termination closes occupancy, and the 200-span cap retains stable lane prefixes ending no later than each lane's first dropped instant while preserving earlier ends across inactive gaps | `internal/exec/runtrace/runtrace.go` `Timeline`, `internal/exec/runtrace/timeline.go` `stateKey`, `applyTimelineLimit`; `internal/ir/view/run_timeline.go` text, Mermaid and PlantUML writers | `internal/exec/runtrace/runtrace_test.go`; parallel-region, termination and capped run-rendering goldens under `internal/exec/runtrace/testdata/` | ✅ Implemented and tested |
| Ordered sends and accepts between objects, including unmatched and environment messages; nonzero message serials pair exactly, while serial-zero traces retain FIFO event/target pairing | `internal/exec/runtrace/runtrace.go` `Sequence`; `internal/exec/runtime/trace.go` `TraceRecord.Message` | `internal/exec/runtrace/runtrace_test.go`; serial-pairing and existing run-rendering goldens under `internal/exec/runtrace/testdata/` | ✅ Implemented and tested |
| State trace records retain the written state path and innermost orthogonal region; termination records retain their printed line while exposing the ending instant as an `Events(kind = "terminate")` row | `internal/exec/runtime/trace.go` `TraceRecord`, `TraceTerminate`; `internal/exec/runtime/state_executor.go` `StateExecutor.RegionOf`, `StateExecutor.terminateMachine`; `internal/doc/queryexec/events.go` `eventKinds`; `internal/exec/runtrace/timeline.go` | `internal/exec/runtime/trace_records_test.go` `TestStateTraceRecordsCarryWrittenPathsAndInnermostRegions`; runtrace termination fixture and golden | ✅ Implemented and tested |
| Located declarations flow from state-entry/exit records into run views: PlantUML timeline lanes link to machine declarations and single-state spans to state declarations; run sequence participants link to their instance types, while parallel spans and Mermaid gantt timelines remain unlinked | `internal/exec/runtime/state_executor.go` `stateSourceOrigin`; `internal/exec/runtime/trace.go` `TraceRecord.Source`; `internal/exec/runtrace/timeline.go`, `sequence.go`; `internal/ir/view/run_timeline.go`, `links.go` | linked run-rendering goldens under `internal/exec/runtrace/testdata/`; `internal/ir/view/run_timeline_test.go`; CLI and REPL run-rendering link tests | ✅ Implemented and tested |

### Runtime bounds: every limit a model can reach

A run is bounded, and a bound that silently changed a result would be the worst outcome, so each
one is a typed error naming what it counts. The eight configurable bounds live in
`runtime/budget.go` (`Budgets`, `BudgetsFromEnv`), and an unusable value for any of them is
reported rather than silently replaced by the default.

| Bound | Sentinel | Variable | Default | Robustness case |
|---|---|---|---:|---|
| Expression evaluation steps | `ErrStepLimitExceeded` | `OPENSYSML_MAX_STEPS` | 10,000,000 | `robustness_run_budget_test.go:non_terminating_loop_performing_an_action`, `range_test.go:TestIntegerRangeSpendsTheStepBudget` |
| Action token-flow steps | `ErrActionStepLimitExceeded` | `OPENSYSML_MAX_ACTION_STEPS` | 1,000,000 | `budget_test.go:TestActionStepBudgetIsConfigurable` |
| State machine events | `ErrStateEventLimitExceeded` | `OPENSYSML_MAX_EVENTS` | 1,000,000 | `budget_test.go:TestStateBudgetsAreConfigurable`, `robustness_object_behaviors_test.go:object_exhibited_machine_never_settles` |
| Do action steps | `ErrDoStepLimitExceeded` | `OPENSYSML_MAX_DO_STEPS` | 5,000,000 | `budget_test.go:TestStateBudgetsAreConfigurable` |
| Collection elements one evaluation holds | `ErrElementLimitExceeded` | `OPENSYSML_MAX_ELEMENTS` | 1,000,000 | `range_test.go:TestIntegerRangeExtremeBounds`, `robustness_lazy_required_test.go:reading_every_value_past_the_element_budget`, `:listing_every_value_is_budgeted`, `:every_feature_of_the_holder_is_read`, `grpc/robustness_lazy_required_test.go:TestGRPCRobustnessLazyRequired` |
| Nested calc invocations | `ErrCalcRecursionLimit` | `OPENSYSML_MAX_CALC_DEPTH` | 10,000 (ceiling 25,000) | `robustness_calc_invocation_test.go` calc recursion cases |
| Bits of one Integer a run computes | `ErrIntegerSizeLimit` | `OPENSYSML_MAX_INTEGER_BITS` | `semantics.DefaultMaxIntegerBits` (floor `MinMaxIntegerBits`) | `budget_test.go` (the `MaxIntegerBits` cases), `compile_test.go` |
| Runs one parameter sweep or sample may make | `ErrSweepBudget` | `OPENSYSML_MAX_SWEEP_RUNS` | 1,000 | `sweep_test.go:TestSweepBudgetIsRefusedBeforeRunning`, `:TestSweepBudgetBoundsTheProduct`, `:TestSamplesBeyondTheBudgetAreRefused` |

Three more bounds are not budgets and are stated here because they shape what a caller is told:

- **The objects one context holds are bounded where a host asks.** `Context.SetMaxInstances`
  bounds the objects registered at once, nested ones counted; `materialize` refuses the one past
  it with `ErrInstanceLimitExceeded`, and the creation or read that reached it is abandoned whole
  (`instance_limit_test.go:TestInstanceLimit_CountsNestedObjects`, `:TestInstanceLimit_RootFailsWhole`).
  `sysml-grpc` sets it from `OPENSYSML_GRPC_MAX_HELD_OBJECTS` (default 10,000) on the runtime each
  cached model holds its objects in and answers the refusal as `RESOURCE_EXHAUSTED`
  (`grpc/objects_test.go:TestHeldObjectsAreBounded`); no other surface sets it.
- **Suspension is bounded by the run, not by the call.** An accept reached through actions
  performed synchronously parks the whole chain, which resumes when a message arrives from anything
  that can still run: a sibling branch, another object, `%send`, the clock. A run driven by
  `RunToCompletion` with nothing left that can post the awaited message, and whose every token is
  parked, reports `ErrAcceptDeadlock` rather than hanging
  (`robustness_send_routing_test.go:routed_send_receiver_name_mismatch_deadlock`,
  `:injected_message_names_a_receiver_no_accept_has`). A message no accept of the run ever takes is
  seen through the accept that waited for it; a run that completes with no accept waiting at all
  leaves it unconsumed in the queue, which is a limitation and not a verdict on the model.
- **A large required lower bound costs only what is read of it.** A population held as reserved
  identities (*Instance Semantics*, "A required lower bound is a count of values") is counted and
  indexed without the element budget, since `size` and `x#(i)` touch one value or none. Work that
  reaches every value makes each one: an evaluation charges them to the element budget and is
  refused past it with `ErrElementLimitExceeded`, naming the count and the feature, and the objects
  it made are abandoned with it; a listing of all of them (`HeldElements`: the REPL, gRPC and JSON
  object views, a document query) is refused the same way before anything is made, as is a
  subsetted union (an object's inherited `parts`) reaching such a population, so serializing the
  object reports the budget on that feature and lists the rest. A lower bound of `*` is not a
  bound but `errInfiniteLowerBound`
  (`robustness_lazy_required_test.go:TestRuntimeRobustnessLazyRequired`,
  `grpc/robustness_lazy_required_test.go:TestGRPCRobustnessLazyRequired`).
- **The materialization check is bounded but says so.** `runtime/materialize.go` walks an object's
  feature values to a depth of 8 and a budget of 1,000 and returns `bounded` when it did not read
  everything; `repl/instantiate_report.go` carries that flag into the report, so an incomplete
  check is presented as unchecked rather than clean (`materialize_test.go:TestMaterializationErrorsBoundsAWideModel`).

### Symbol Resolution (`internal/semantic/resolve/`)

| File | Purpose |
|------|---------|
| `document.go` | Name resolution, inheritance chain lookup |
| `qualified.go` | Qualified name resolution (A::B::C) |

### Symbol Tables (`internal/semantic/symbols/`)

| File | Purpose |
|------|---------|
| `builder.go` | AST → symbol table, control flow node registration |
| `scope.go` | Scope tree, member lookup |

---
