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
| `rpc` | all | `GetSymbol`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState`, `ApplyEdits`, `RunDocumentQuery` or `EvaluateCalc` |
| `expression` | Evaluate | expression source to evaluate |
| `context_symbol_id` | Evaluate | optional FQN whose scope the expression is evaluated in |
| `subject_symbol_id` | Evaluate | optional FQN of a part/usage instantiated and evaluated against, so features read its feature values |
| `symbol_id` | GetSymbol, Instantiate, ExecuteAction, ExecuteState, EvaluateCalc | FQN of the subject |
| `arguments` | EvaluateCalc | positional arguments bound to the calc's parameters |
| `tools` | EvaluateCalc | stand-ins under `internal/exec/analysis/testdata` built and registered from a manifest before the service is built |
| `inputs` | ExecuteAction | parameter name → value, bound before execution |
| `events` | ExecuteState | event names injected, in order |
| `instantiate` | RunDocumentQuery | FQNs `Instantiate` creates objects of first, in order, so the query can bind and enumerate them |
| `query_id` | RunDocumentQuery | FQN of the document query to run |
| `bindings` | RunDocumentQuery | list of `{parameter, values}`, each value a document value bound to the parameter |
| `expected_columns` | RunDocumentQuery | full ordered projected-column name list |
| `expected_rows` | RunDocumentQuery | full ordered row list, each `{element, cells}` — the row's own document value and one list of document values per column |
| `expected_result` | Evaluate, EvaluateCalc | expected `Value` |
| `expected_attribute_names` | GetSymbol | full ordered attribute name list, own then inherited |
| `expected_attributes` | GetSymbol | attribute name → `{type, value_kind, value, unit}`; no `value_kind` requires no value |
| `expected_feature_values` | Instantiate | feature name → `{materialized, value_kind, value, error}` |
| `expected_instance_count` | Instantiate | number of reachable instances in the response graph |
| `expected_outputs` | ExecuteAction, EvaluateCalc | output name → expected `Value`; for EvaluateCalc it is the outputs a calc usage computes when invoked without arguments |
| `expected_states_visited` | ExecuteState | full ordered state-visit trace |
| `expected_final_context` | ExecuteState | context entry name → expected `Value` |
| `expected_error` | all | substring the RPC's in-band `error` must contain (for `RunDocumentQuery`, the status error the call fails with) |

A case without `expected_error` requires an empty `error` field. A case with `expected_error`
asserts only the error, and is how failure modes (for example an action with no initial node)
are pinned.

A feature value's `error` is a substring its `FeatureValue.error` must contain; one without an
error must carry none.

A value is `{"kind": <oneof field of pb.Value>, "value": <literal>}`, where `kind` is one of
`int_value`, `real_value`, `bool_value`, `string_value`, `instance_id`, `quantity`, `null` or
`unset` — the last being a materialized feature value holding no value, as a valueless feature of a
value type does. The assertion checks the oneof arm as well as the payload, so a value returned
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
