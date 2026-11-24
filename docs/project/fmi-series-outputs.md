# Sampled trajectory outputs for `tool:fmi`

This is the design and verification record for adding trajectory (series) outputs to the
`tool:fmi` engine. It is written for a reader who has only this tree.

## The missing capability

A tool-computed SysML calc sends one request to an external runner and binds one reply.
Before this change the `tool:fmi` engine bound every output exactly once, at `stopTime`.
A downstream consumer that needs a property of the whole run — the peak temperature of a
boiler, the minimum pressure in a transient, the area under a response curve — cannot
decide that property in a model: it has one sample, not a trajectory. The consumer has to
leave the model, run the FMU itself, and carry the answer back as an external assertion.

The capability added here lets a calc ask for a sampled trajectory, bind it to a
multi-valued parameter, and let an ordinary SysML requirement decide a max- or
min-over-trajectory condition from that one run.

## Exact baseline

The pinned engine at `4d67c7a98f` (`nightly-20261118-4d67c7a98f`,
`v0.12.1-2379-g4d67c7a98f`; the preceding stable tag is `v0.12.1`) had:

- `internal/exec/fmi/engine.go` binding outputs once at `stopTime` in `replyOf`.
- Reserved inputs `fmi:startTime`, `fmi:stopTime`, `fmi:stepSize`, `fmi:tolerance` and the
  single reserved output `fmi:time`. Any other `fmi:` name was refused as an unknown
  variable. There was no series-shaped output and no sample-count input.
- A runner exchange of one JSON object each way (`fmi/1`): the request carried `protocol`,
  `fmu`, `interface`, `experiment`, `start`, `outputs`; the reply carried `protocol`,
  `time`, `outputs` or `error`. Every reply value was a scalar or one fixed FMI array.
- No issue or pull request in the upstream tracker proposed a series surface. The engine
  itself was introduced by upstream PR [#663](https://github.com/Open-MBEE/OpenSysML/pull/663).

## What the change implements

A tool calc may now name `fmi:series:<variable>` as an output, bound to a parameter whose
multiplicity admits more than one value. The engine:

1. Reads `fmi:samples` (an integer from 2 to 10000) or `fmi:sampleInterval` (seconds,
   converted from a measured value when the declaration carries a unit) to fix the number
   of samples; with neither, it asks for 11 evenly spaced samples.
2. Sends an optional `series` member in the runner request, with the sample count and the
   FMU variables to sample; scalar `outputs` stay exactly as they were.
3. Validates the reply's series: one ordered, finite instant and one correctly typed value
   per sample for every requested variable. The `fmi:seriesTime` output binds the instants
   as a sequence measured in seconds, and each `fmi:series:<name>` output binds its values
   as a sequence in the variable's coherent unit.
4. Binds the result through the existing `runtime.ToolCall.Bind` path, so a sequence-valued
   parameter is checked against its declared multiplicity exactly as any other tool answer.

No scalar output changes: a call with no series output sends byte-for-byte the same
request as before. A reply that lacks the `series` member, has the wrong sample count, is
unordered or non-finite, names an unknown/array/input variable, or is bound to a
single-valued parameter is a typed refusal — the scalar value at `stopTime` is never
substituted for a trajectory.

## Design decisions and alternatives rejected

- **A reserved output prefix rather than an overloaded scalar name.** A separate
  `fmi:series:` name makes the intent explicit at the declaration site and keeps the old
  scalar name's meaning unchanged. Overloading `fmi:time` or an FMU variable's own name to
  mean "sometimes a sequence" would have made the binding depend on the parameter's
  multiplicity and silently changed existing models.
- **Sample count or interval, never implicit from an array length.** The count is stated
  by `fmi:samples` or derived from `fmi:sampleInterval` and the experiment span; both
  together are refused instead of resolved by a silent precedence rule. This keeps the
  runner's work bounded before a process starts.
- **Times and values as parallel lists, not rows of objects.** A `series` member with
  `times` plus one list per variable maps directly onto the runtime's sequence values and
  onto the collection functions a requirement uses. Repeated output rows would have
  required a new structured value kind in the evaluator for no gain here.
- **Optional protocol members at `protocol: 1`.** The new request and reply members are
  omitted when no series is asked for, so existing runners and existing scalar calls are
  unaffected. Bumping the protocol version would have refused every existing runner for
  a purely additive change; a runner that does not understand `series` answers no series
  and the engine reports a named protocol error.
- **Scalar variables only.** FMI arrays already bind at `stopTime`; sampling an array at
  every instant would produce a sequence of arrays, which the runtime has no value form
  for. Naming one as a trajectory is refused rather than flattened.

## Public surface

- **Model surface (SysML `ToolVariable` names):** `fmi:samples` (input, integer 2–10000,
  default 11), `fmi:sampleInterval` (input, seconds), `fmi:seriesTime` (output, sequence of
  seconds), `fmi:series:<FMU variable>` (output, sequence).
- **Runner protocol:** the `fmi/1` JSON exchange gains two optional members. Request:
  `series: {samples: <int>, outputs: [<variable>, …]}`. Reply:
  `series: {times: [<seconds>, …], outputs: {<variable>: [<value>, …]}}`. The `protocol`
  value stays `1`; the members are omitted for scalar-only calls.
- **Reference runner:** `client/python/opensysml/fmi_runner.py` serves the optional members;
  its `run(request) -> reply` signature and all existing reply keys are unchanged.
- **RPC, protobuf, capability flags, environment variables and CLI flags:** unchanged. No
  client wire update is required. `Describe()` still reports `Compute` at observed
  authority with the same process and bounds.
- **File formats:** `modelDescription.xml` reading is unchanged. The committed model
  fixture `internal/exec/fmi/testdata/series.sysml` states the tool calc and both forms of
  the peak-temperature requirement.

## Acceptance locked by named tests

| Criterion | Test |
|---|---|
| One run returns the trajectory and the series shape is pinned | `TestSeriesReturnsTheTrajectory` (Go, `internal/exec/fmi/series_test.go`) |
| A `require constraint` on `max(T)` over the trajectory decides the check in one run | `TestSeriesRequirementDecidesInOneRun` |
| A series is never silently collapsed to the `stopTime` scalar | `TestSeriesIsNeverACollapsedStopTimeValue`, `TestSeriesRefusesAScalarParameter`, `TestSeriesRejectsAWrongSampleCount` |
| Scalar outputs stay byte-identical | `TestScalarOutputsRemainUnchanged` and the pre-existing `TestEngineRunDrivesTheRunner` |
| Sample configuration, units and sample times | `TestSampleCountValidatesTheRequest`, `TestSeriesCarriesUnitsAndTimes` |
| Invalid series variables are refused | `TestSeriesVariableErrors` |
| The reference runner samples a trajectory | `test_series_answers_evenly_spaced_samples`, `test_series_only_variables_are_requested_from_fmpy`, `test_bad_series_requests_are_error_replies` (`client/python/tests/test_fmi_runner.py`) |

## Verification

Run in the isolated worktree with an isolated Go build cache:

- `gofmt -l .` — no output.
- `go build -p 1 ./...` — exit 0, no output.
- `go vet -p 1 ./...` — exit 0, no output.
- `go test -count=1 ./internal/exec/fmi/...` — `ok github.com/Open-MBEE/OpenSysML/internal/exec/fmi 7.196s`.
- `go test -count=1 ./tests/hygiene -run 'TestComplianceTablesNameLiveGoIdentifiers|TestDocumentedFigures'`
  — `ok github.com/Open-MBEE/OpenSysML/tests/hygiene 23.636s`.
- `python3 scripts/check-doc-ids.py` — "No internal work-item labels in reader-facing documentation".
- `python3 scripts/changelog.py check` — exit 0, no output.
- `python3 -m py_compile client/python/opensysml/fmi_runner.py client/python/tests/test_fmi_runner.py`
  — exit 0. A direct `fmi_runner.run()` probe answered
  `{'protocol': 1, 'time': 4.0, 'outputs': {'h': 16.0}, 'series': {'times': [0.0, 2.0, 4.0], 'outputs': {'h': [0.0, 4.0, 16.0]}}}`.

The Python test module needs `pytest` and `pytest-mock`, which are not installed here, so
those tests were not executed; the runner was exercised directly instead. Optional corpora
(training, pilot, specification examples, PSSM, OMG JSON schema and the Modelica
Reference-FMUs) are absent in this environment and their gates skip.

The full `go test ./...` gate could not complete cleanly under the machine's concurrent
build load: the first attempt lost standard-library export artifacts from the shared build
cache while other worktrees compiled, and an isolated-cache run hit the harness's ten-minute
timeout in `cmd/sysml` and a twenty-second silence kill in `cmd/sysml-lsp`
(`TestInitializeReportsTheVersionOfTheBinary`). Both are resource failures in packages this
change does not touch; the FMI package and the documentation/hygiene gates were re-run
directly and passed.

## Known limitations and follow-ups

- A series samples scalar FMU variables only; arrays cannot be sampled into a trajectory.
- The reference runner selects the recorded simulation steps nearest to each evenly spaced
  instant; it does not interpolate between them. A runner with a different simulator may
  sample more precisely, as long as it answers the requested count in time order.
- Producing a series still requires a runner that understands the optional request member;
  an older runner answers no `series` and the evaluation fails as a protocol error rather
  than falling back to one value. Updating the reference runner is the shipped path.
- A min/max-over-trajectory condition is decided by the ordinary requirement evaluator
  over the bound sequence; this change adds no new aggregation function to the language.
