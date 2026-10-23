# gRPC execution conformance cases

Layer 2 of the AGENTS.md §5.2 test contract for the gRPC service. Each case is a pair:

- `<name>.sysml` — a real model, parsed through the `ParseFile` RPC (stdlib loaded, semantic
  passes run). A case whose model reports a diagnostic *error* fails; import what you use
  (`import ScalarValues::*;`) rather than relying on an implicit library import.
- `<name>.expected.json` — the RPC to drive and the expected response.

`TestGRPCConformance` in `tests/grpc/conformance_test.go` discovers every `.expected.json`
in this directory, so adding a case is a data-only change.

## Expectation schema

| Field | Applies to | Meaning |
|---|---|---|
| `rpc` | all | `GetSymbol`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `ApplyEdits`, `RunDocumentQuery`, `EvaluateCalc`, `RunRuns`, `SelfCheck`, `RecordRun` or `RenderView` |
| `view`, `compartments` | RenderView | the view or `#<kind>:<name>` pseudo-view to render and the compartment folding (`positioned`, `most` or `ports`; empty is `positioned`) |
| `expected_node_names`, `expected_compartments`, `expected_edge_labels` | RenderView | the nodes drawn in order, the compartment each row is drawn in by the row's name, and the edge labels in order; `expected_error` a refusal instead |
| `rpc` | all | `GetSymbol`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `ApplyEdits`, `RunDocumentQuery`, `EvaluateCalc`, `ImportData` or `CompareResults` |
| `imports`, `import_dry_run` | ImportData | the imports (`name`, `content`, optional `import_format` and `import_map`), in order, and whether to report without changing the model |
| `expected_assignments` | ImportData | the assignments' descriptions over every import, in order; `expected_content` is the edited model, `expected_error` a refusal that leaves it unchanged |
| `results`, `actions`, `runs`, `seed`, `observe` | CompareResults | the results index as JSON, the configurations selected, the runs per configuration, the seed and stored-name → feature pairs |
| `expected_configurations` | CompareResults | per configuration in index order: `name`, `status`, `runs`, `completed`, `stored_runs`, `observables` (stored name → compared), `unresolved` and `lines_contain` |
| `rpc` | all | `GetSymbol`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `ApplyEdits`, `RunDocumentQuery`, `EvaluateCalc` or `CheckBehavior` |
| `rpc` | all | `GetSymbol`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `GetDiagnostics`, `ApplyEdits`, `RunDocumentQuery` or `EvaluateCalc` |

`ApplyEdits` sequence operations may carry the recursive action-body fields
`condition`, `value`, `target`, `via`, `until`, `body`, `else_body`,
`multiplicity` and `parameter`. Nested items use the same shape without an
owner or insertion point. An empty `else_body` means no else branch; explicit
empty else blocks are not authorable.
| `expression` | Evaluate | expression source to evaluate |
| `context_symbol_id` | Evaluate | optional FQN whose scope the expression is evaluated in |
| `subject_symbol_id` | Evaluate | optional FQN of a part/usage instantiated and evaluated against, so features read its feature values |
| `symbol_id` | GetSymbol, Instantiate, ExecuteAction, ExecuteState, EvaluateCalc, CheckBehavior | FQN of the subject |
| `arguments` | EvaluateCalc | positional arguments bound to the calc's parameters |
| `tools` | EvaluateCalc | stand-ins under `internal/exec/analysis/testdata` built and registered from a manifest before the service is built |
| `inputs` | ExecuteAction | parameter name → value, bound before execution |
| `runs`, `seed`, `draws`, `observe_features` | RunRuns, RecordRun | the run count, the seed the draws start from (`seed` must be set unless `draws` fixes them; RunRuns refuses an unseeded request with a status error, which `expected_error` matches too), and the action features RunRuns tables; `symbol_id` names the action, `context_symbol_id` the Monte Carlo case run on `subject_symbol_id` |
| `self_check_package` | SelfCheck | rule packages applied beside `SysMLValidation` |
| `record_into` | RecordRun | the package the records go to (`Records` when empty); `symbol_id` names the case |
| `expected_row_count`, `expected_observables` | RunRuns | runs tabled, every one completed, and the observables each distribution counts |
| `expected_status`, `min_checks` | SelfCheck | the verdict (`holds` or `fails`) and the least number of constraint applications |
| `expected_package`, `expected_records` | RecordRun | the records generated, each resolvable by `GetSymbol` in the recorded model |
| `advance`, `seed`, `draws`, `clock_step` | ExecuteAction | the run options of `sysml -advance`, `-seed`, `-draws` and `-clock-step`; absent sends none |
| `disable_lint`, `enable_lint` | GetDiagnostics | lint codes left out of, and opt-in lint codes kept in, the diagnostics, as `sysml -disable-lint`/`-enable-lint` select them |
| `expected_codes`, `absent_codes` | GetDiagnostics | diagnostic codes the answer must carry, and must not |
| `events` | ExecuteState | event names injected, in order |
| `trace`, `expected_trace_objects` | ExecuteState | ask for the run's trace, and the usages the objects taking its transitions are held under, in first-appearance order, one per object (a machine run on the exhibited usage's own path traces its owner's one run) |
| `advance`, `expected_final_time` | ExecuteState | the simulated time to run for after the events, and the clock the run ended at |
| `performer_symbol_id` | CheckBehavior, ExecuteAction, ExecuteState | FQN of the part/usage whose object performs the behavior, or a path from one into what it holds; an ExecuteAction on it joins the performance that object already holds or runs, so `inputs` must be the bindings its declaration states (an `instance_id` input names an object of the instantiated performer) |
| `check_property` | CheckBehavior | constraints or requirements evaluated at every stable state |
| `check_diverge` | CheckBehavior | features whose final value the schedule may decide |
| `expected_verdict` | CheckBehavior | `holds`, `fails` or `undecided` |
| `expected_violations` | CheckBehavior | the violations witnessed, in order: `kind`, `property`, `depth` |
| `expected_divergences` | CheckBehavior | feature → the final values it ends as, in order |
| `instantiate` | RunDocumentQuery | FQNs `Instantiate` creates objects of first, in order, so the query can bind and enumerate them |
| `query_id` | RunDocumentQuery | FQN of the document query to run |
| `bindings` | RunDocumentQuery | list of `{parameter, values}`, each value a document value bound to the parameter |
| `expected_columns` | RunDocumentQuery | full ordered projected-column name list |
| `expected_rows` | RunDocumentQuery | full ordered row list, each `{element, cells}` — the row's own document value and one list of document values per column |
| `expected_result` | Evaluate, EvaluateCalc | expected `Value` |
| `expected_attribute_names` | GetSymbol | full ordered attribute name list, own then inherited |
| `expected_attributes` | GetSymbol | attribute name → `{type, value_kind, value, unit}`; no `value_kind` requires no value |
| `expected_metadata` | GetSymbol | metadata key → value the symbol must carry (`variation`, `variant`, `abstract`, `type`, …) |
| `absent_metadata` | GetSymbol | metadata keys the symbol must not carry |
| `expected_feature_values` | Instantiate | feature name → `{materialized, value_kind, value, error}` |
| `expected_instance_count` | Instantiate | number of reachable instances in the response graph |
| `expected_outputs` | ExecuteAction, EvaluateCalc | output name → expected `Value`; for EvaluateCalc it is the outputs a calc usage computes when invoked without arguments |
| `expected_states_visited` | ExecuteState | full ordered state-visit trace |
| `expected_final_context` | ExecuteState | context entry name → expected `Value` |
| `expected_error` | all | substring the RPC's in-band `error` must contain (for `RunDocumentQuery`, the status error the call fails with) |
| `expected_final_time` | ExecuteAction | the simulation clock, in seconds, the run ended at |

A case without `expected_error` requires an empty `error` field. A case with `expected_error`
asserts only the error, and is how failure modes (for example an action whose successions leave no step to start at)
are pinned.

A feature value's `error` is a substring its `FeatureValue.error` must contain; one without an
error must carry none.

A value is `{"kind": <oneof field of pb.Value>, "value": <literal>}`, where `kind` is one of
`int_value`, `real_value`, `rational_value`, `bool_value`, `string_value`, `instance_id`,
`quantity`, `null`, `unset` or `sequence` — a `sequence` carrying its `elements` as values of this form in place of a literal, as a `[0..*]` output holding one value does; `unset` being a materialized feature value holding no value, as a valueless feature of a
value type does. A `rational_value`'s literal is the string `"<numerator>/<denominator>"`, sent
as written and answered in lowest terms. The assertion checks the oneof arm as well as the payload, so a value returned
with the wrong type fails; `instance_id`, `null` and `unset` assert the arm only, since instance
ids are assigned at runtime.

A document value (`RunDocumentQuery`) is `{"kind": <oneof field of pb.DocumentValue>, "value":
<literal>}`. Bound, `element_id` is the element's qualified name, `object` is `{"instance_id": n}`,
`{"path": "car.wheels[2]"}` or both, and `int_value`, `real_value`, `bool_value` and
`string_value` are their literals. Answered, `element_id` renders as `"<fqn> (<element_type>)"`,
`object` as `"<path> (#<id>) : <usage fqn> (<element_type>)"` — the path the object is reached by
from the binding (`#1.wheels[2]` bound by id, `Garage::car.wheels[2]` bound by name), its id, and
the usage it stands for — `verdict` as `"<text> on <path>: <verdict>"`, `state` as
`"<object path>.<machine> in <state path> (<region>)"`, `event` as `"<kind> at <time>: <text>"`
with the time in the spelling of its own arm, `quantity` as below, `infinity` asserts the arm
only, and the scalar arms compare their literals.

A `quantity`'s literal is the string `"<magnitude> [<unit as written>] = <reduction>"`, for
example `"5.4 [SI::km/SI::h] = 5/18·SI::metre·SI::second^-1"`: the magnitude in the unit it
was written in, then what that unit reduces to. A reduction of `1` is a dimensionless unit and
`absent` is a unit the service could not reduce.
