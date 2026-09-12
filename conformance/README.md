# Conformance suite

This directory is the language-independent contract between `sysml-grpc` and its clients. A
scenario states one call and what the service must answer; nothing here names a transport, a
programming language, or a client's object model. The reference runner is
[`cmd/conformance`](../cmd/conformance), which builds and starts the service itself:

```bash
make conformance                       # the CI gate; writes bin/conformance-report.json and .xml
go run ./cmd/conformance -v             # print each scenario's normalized response
go run ./cmd/conformance -run evaluate  # only the scenarios whose id matches
go run ./cmd/conformance -binary ./bin/sysml-grpc   # test a binary already built
go run ./cmd/conformance -protocols grpc,connect,connect-json
```

`-report <file>` writes the machine-readable summary (`-` writes it to stdout). `-junit <file>`
additionally writes the same results as JUnit XML — one suite per configuration and protocol, one
case per scenario — the format CI systems render natively as a test report. The top-level
object contains aggregate totals and a `protocols` list. Each protocol entry retains the
capabilities and one result per scenario with its outcome, status, duration and, when it
disagreed, the list of mismatches. The default protocols are `grpc`, `connect` (protobuf body)
and `connect-json` (JSON body); all three runs use one service process. Use `-transport grpc
-protocols grpc` to exercise a grpc-go-only service. Connect protocol clients issue unary POSTs
to `/sysml.SysMLService/<Method>` with `application/proto` or `application/json` bodies.

## Layout

- `scenarios/*.json` — the scenarios, grouped by RPC and run in file then declaration order.
- `fixtures/*.sysml` — the models scenarios parse. A scenario names a fixture, never a path.

## A scenario

```json
{
  "id": "evaluate/a_subject_is_evaluated_against_that_objects_value",
  "description": "Why this is part of the contract, in one sentence.",
  "rpc": "Evaluate",
  "requires_capabilities": ["evaluate_subject"],
  "model": { "fixture": "vehicle.sysml" },
  "request": { "model_hash": "${model_hash}", "expression": "mass", "subject_symbol_id": "Demo::sedan" },
  "expect": { "response": { "result": { "real_value": 1200.0 } } }
}
```

| Field | Meaning |
| --- | --- |
| `id` | Unique; a report and `-run` address a scenario by it. |
| `rpc` | Method name, bare (`Evaluate`) or qualified (`sysml.SysMLService/Evaluate`). |
| `requires_capabilities` | Names `GetServerInfo` must report for `expect` to apply. |
| `expect_without_capability` | What a service **not** reporting them must answer instead. |
| `model` | A fixture parsed once per run before the call; its hash fills `${model_hash}`. |
| `request` | The request as protobuf-JSON. |
| `expect` | What the answer must be, by the rules below. |

Two placeholders may appear in a request: `${model_hash}` is the hash the service gave `model`,
and `${fixture:<name>}` is a fixture's source, for the RPCs that take content inline.

## Expectations

Every field of `expect` is optional and all of them must hold. An absent `status` means the call
must succeed.

| Field | Meaning |
| --- | --- |
| `status` | Canonical gRPC status name (`OK`, `INVALID_ARGUMENT`, `NOT_FOUND`, `UNIMPLEMENTED`, …). |
| `status_message_contains` | Substring the status message must contain. |
| `response` | Tree compared field by field against the answer; see below. |
| `non_empty` | Paths that must hold a value other than their default. |
| `absent` | Paths that must be unset or hold their default. |
| `contains` | Path → substring its text must contain. |
| `contains_all` | Path → strings all of which must be there: substrings of text, or members of a list. |
| `counts` | Path → exact number of entries of the list or map there. |
| `min_counts` | Path → lower bound on that number. |

A path is dotted, walking fields, map keys and list indices —
`instance.feature_values.mass.value.real_value` — and `*` takes one field from every entry of a
list or map, as in `elements.*.id`.

## Comparison rules

Getting these wrong is how a conformance suite becomes flaky or vacuous, so they are stated
rather than left to the runner:

- **`response` is compared where it is named, exactly.** A field the expectation does not
  mention is not compared, so adding a field to the schema does not fail a scenario. Every field
  it does mention must be equal, including the **length of any list it names**: a list of two
  expected entries does not match three actual ones.
- **An unset field and a field holding its default are the same thing**, because that is what
  they are on the wire. `"error": ""` matches an answer that carries no error, and `absent` and
  `non_empty` read the default the same way.
- **Reals compare within a relative tolerance of 1e-9**, which admits a different summation
  order and admits no difference a model would state. Integers, booleans and strings compare
  exactly. An enum compares by the name the schema gives it (`EDIT_FAILURE_UNKNOWN_TARGET`), not
  by number, so renumbering is caught and renaming is legible.
- **A status is compared by code**, spelled canonically (`NOT_FOUND`), never by message text
  unless `status_message_contains` says so.
- **A refused call and a failure the answer reports are different things**, and a scenario says
  which it expects. `status` is the transport's verdict; `error` (and `failure`,
  `failure_reason`) are fields of a successful answer. A scenario expecting `NOT_FOUND` fails if
  the service answers `OK` with an error field, and the other way round.

### Normalized values

These cannot be compared literally, so the runner replaces them before comparing:

| Value | Becomes | Why |
| --- | --- | --- |
| `ServerInfoResponse.version` | `${version}` | A build string; the contract is capabilities, not versions. |
| Any string equal to the model hash of the scenario's model | `${model_hash}` | Content-addressed and free to change with the parser. |
| Any absolute path (`Span.file`, echoed request paths) | `${path}` | Names the machine the service ran on. A relative name is kept. |
| Runtime instance ids (`Instance.id`, `Value.instance_id`, `Verdict.instance_id`, `Function.self_id`) | `@1`, `@2`, … | Assigned per call. Labelled in order of first appearance, so a scenario can still state that a feature value names the same object as an entry of `instances`. |

### Ignored values

- **Timing.** Durations are recorded in the report and compared to nothing.
- **Diagnostic message text and spans.** Scenarios pin the number of diagnostics and their
  `severity`; wording is not a wire contract, and a message is asserted only where a scenario
  says so explicitly with `contains`.
- **Field order.** Map keys are compared as a map, and repeated fields keep the order the
  service sent them, which for `verdicts`, `states_visited` and `applied` **is** part of the
  contract and is compared.

## Capabilities

A client selects behaviour by capability name, never by version string, so a scenario needing
one names it in `requires_capabilities`. If `GetServerInfo` does not report it, the scenario is
skipped — and a skip fails the run unless `-allow-skips` is passed, so a service quietly losing
a capability does not turn a gate green. A scenario may instead state what a service lacking the
capability must answer, in `expect_without_capability`; that is where the suite pins that an
unsupported request is refused with `UNIMPLEMENTED` rather than silently ignored.

What a request asks for is fixed per capability:

| Capability | Request-side contract when unavailable |
| --- | --- |
| `convert` | Refuse `Convert`. |
| `verification` | Refuse `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction` and `EvaluateCalc`. |
| `query` | Refuse `Query`. |
| `oslc_query` | Refuse `Query` only when `oslc_query` is set; structured queries still use `query`. |
| `apply_edits` | Refuse `ApplyEdits`. |
| `authoring` | Refuse `ApplyEdits` only when an operation is `add_member` or `delete`. |
| `inline_language` | Refuse `ParseFile` only when inline content names a language. |
| `strict_conformance` | Refuse `ParseFile` only when `strict_conformance` is true. |
| `evaluate_subject` | Refuse `Evaluate` only when `subject_symbol_id` is set. |
| `schedule` | Refuse `ExecuteAction`, `ExecuteState` and `RunAnalysis` only when `schedule` is set; an empty field runs under the default policy. |
| `type_facts` | Response-population capability: omit type facts; no request asks for them. |
| `symbol_attributes` | Response-population capability: omit symbol attributes; no request asks for them. |
| `feature_values` | Response-population capability: omit instance feature values; no request asks for them. |
| `enum_values` | Response-population capability: encode enum values as unsupported nulls; no request asks for them. |
| `unset_value` | Response-population capability: encode unset values as unsupported nulls; no request asks for them. |
| `complex_values` | Response-population capability: encode complex numbers as unsupported nulls; no request asks for them. |
| `structured_values` | Encode arrays, vectors and vector quantities as unsupported nulls; refuse a request carrying one, at any depth. |
| `measurement_refs` | Encode bare measurement references as unsupported nulls; refuse a request carrying one, at any depth. |
| `function_values` | Encode functions (a calc read as a value) as unsupported nulls; refuse a request carrying one, at any depth. |
| `metaobject_values` | Encode metaobjects (an element reflected on by `meta` or `.metadata`) as unsupported nulls; refuse a request carrying one, at any depth. |

The default service reports and supports every capability above. `make conformance` also starts a
second service with `strict_conformance` and `oslc_query` withheld, verifies that its advertisement
is exactly the default list minus those names, and requires both fallback expectations to execute
under gRPC, Connect and Connect-JSON. The exact default `GetServerInfo` scenario is replaced in that
configuration by this stronger set comparison.

Withholding is test-only. `cmd/conformance` passes
`OPENSYSML_TEST_WITHHOLD_CAPABILITIES` to the child process it starts; normal startup strips no
capability, and the variable is not a supported service configuration interface.

## Non-vacuity

A suite that passes against a broken service is worse than none, so what the scenarios catch is
verified rather than assumed:

- `cmd/conformance`'s own tests pin the comparison rules — tolerance, list length, default
  handling, path lookup, id labelling, status naming — with cases that must fail as well as
  cases that must pass.
- `TestEveryRPCIsCovered` fails if an RPC of the service is reached by no scenario, and
  `TestTheSuiteCoversBothKindsOfFailure` fails if the suite stops pinning refused requests or
  in-band failures.
- The suite is run against deliberately mutated builds of the service; the mutations and the
  scenarios that caught them are recorded in the pull request that added the suite.

## Porting a runner to another language

The scenarios are the specification and `cmd/conformance` is one reading of it. A runner in
another language needs: protobuf-JSON decoding of `request` into the RPC's request message,
the normalization table above, the comparison rules above, capability gating from
`GetServerInfo`, and the same report shape. Nothing else in this directory is gRPC-specific:
`rpc` names a method of `sysml.SysMLService`, and how that method is reached is the transport's
business.
