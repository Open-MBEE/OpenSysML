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
2. Sends an optional `series` member in the runner request, with the sample count, any
   explicitly requested instants and the FMU variables to sample; scalar `outputs` stay
   exactly as they were.
3. Validates the reply's series: one ordered, finite instant and one correctly typed value
   per sample for every requested variable. When `fmi:sampleInterval` named the instants,
   the reply's times must match them within a small tolerance, so a non-dividing interval
   is never respaced across the whole experiment. The `fmi:seriesTime` output binds the instants
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
- **Explicit instants for an interval.** Sending only a count would let a runner respace
  the points evenly: a 2 s interval over a 0–3 s run would become 0 s and 3 s. The request
  carries the requested instants, the reference runner interpolates between recorded steps
  at them, and the engine refuses a reply whose times differ from the request.
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
  `series: {samples: <int>, times?: [<seconds>, …], outputs: [<variable>, …]}` — `times`
  is sent when `fmi:sampleInterval` named the instants and omitted otherwise. Reply:
  `series: {times: [<seconds>, …], outputs: {<variable>: [<value>, …]}}`. The `protocol`
  value stays `1`; both members are omitted for scalar-only calls.
- **Reference runner:** `client/python/opensysml/fmi_runner.py` serves the optional members;
  it honours requested instants by interpolating between recorded steps. Its
  `run(request) -> reply` signature and all existing reply keys are unchanged.
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
| A non-dividing interval is sampled at the requested instants, and a respaced reply is refused | `TestSeriesIntervalSpacingIsHonoured` |
| Invalid series variables are refused | `TestSeriesVariableErrors` |
| The reference runner samples a trajectory and honours requested instants | `test_series_answers_evenly_spaced_samples`, `test_series_only_variables_are_requested_from_fmpy`, `test_requested_sample_instants_are_honoured`, `test_requested_instants_interpolate_between_steps`, `test_bad_series_requests_are_error_replies` (`client/python/tests/test_fmi_runner.py`) |

## Verification

Run in the isolated worktree:

- `gofmt -l .` — no output.
- `go build -p 1 ./...` — exit 0, no output.
- `go vet -p 1 ./...` — exit 0, no output.
- `go test -count=1 ./internal/exec/fmi/...` — `ok github.com/Open-MBEE/OpenSysML/internal/exec/fmi 9.032s`.
- `go test -count=1 ./tests/hygiene` — `ok github.com/Open-MBEE/OpenSysML/tests/hygiene 176.901s`.
- Comment gates — `git diff --merge-base origin/develop -U2 -- '*.go' | awk '/^-/{next} /^[+ ][[:space:]]*\/\//{n++; a=a||/^\+/; if(n>2&&a) print; next} {n=0;a=0}'`
  prints nothing, and the same gate over all changed files and the `#` gate over Python
  files print nothing.
- `python3 scripts/check-doc-ids.py` — "No internal work-item labels in reader-facing documentation".
- `python3 scripts/changelog.py check` — exit 0, no output.
- Python runner tests: `pytest` 9.2.0 is present. Without `pytest-mock` the module reported
  `2 passed, 2 skipped, 13 errors`; after installing `pytest-mock` into a temporary directory
  on `PYTHONPATH`, `15 passed, 2 skipped`. The two skips are the Reference-FMU tests whose
  FMU corpus and `fmpy` are absent in this environment.

A full-suite re-run reproduced a pre-existing nested-divergence timeout in `cmd/sysml` (a test
this change does not touch) and found no failure attributable to the diff; the FMI package,
the hygiene suite and both documentation checks pass directly. Earlier attempts under heavy
concurrent build load also lost shared-cache artifacts and killed `cmd/sysml-lsp` after a
twenty-second silence; those were environment failures in untouched packages.

## Known limitations and follow-ups

- A series samples scalar FMU variables only; arrays cannot be sampled into a trajectory.
- When only a sample count is requested, the reference runner selects recorded simulation
  steps nearest to each evenly spaced instant; when `times` is present it interpolates the
  model's variables between steps at the requested instants. A runner with a different
  simulator may sample more precisely, as long as it answers the requested count in order
  and at the requested instants when they were given.
- Producing a series still requires a runner that understands the optional request member;
  an older runner answers no `series` and the evaluation fails as a protocol error rather
  than falling back to one value. Updating the reference runner is the shipped path.
- A min/max-over-trajectory condition is decided by the ordinary requirement evaluator
  over the bound sequence; this change adds no new aggregation function to the language.
- **Skills follow-up, not done here.** The `analysis engines` skill in the separate
  `OpenSysML-MDK-Skills` repository documents the tool surface and must be updated in its
  own pull request: add the `fmi:samples`, `fmi:sampleInterval`, `fmi:seriesTime` and
  `fmi:series:<var>` names, describe the optional `series` request/reply members (including
  requested `times`), and stamp the skill with the `sysml -version` it was verified against.
  That repository is not present in this worktree, so no claim of an update is made.
