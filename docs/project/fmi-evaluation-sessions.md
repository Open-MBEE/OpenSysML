# Reusable FMI evaluation sessions

## The missing capability

`tool:fmi` evaluates an FMU by handing one request to an external runner and
binding its single reply to the annotated calculation's outputs. The reply is
read at one time, `fmi:stopTime`. That is enough to answer "what is the state at
the end of this experiment", but not "what is the state halfway through". A
downstream consumer that asks for an intermediate value — a check of a mid-run
temperature, a plot of a trajectory, a sensitivity taken at several times — can
only start a new process and simulate the whole experiment again for every
point. Each point pays the full simulation, and nothing in the answer says the
points came from one experiment.

The missing piece is an evaluation session: initialize a runner's FMU state
once, advance that state to a requested time, and read variables from the state
reached. N intermediate readings should cost one runner invocation, not N.

## Baseline

At `4d67c7a98f` (v0.12.1 lineage), `tool:fmi` was introduced by
[PR #663](https://github.com/Open-MBEE/OpenSysML/pull/663). The state of that
surface, still present in the verification tree:

- `internal/exec/fmi/engine.go` builds one experiment from the reserved inputs
  `fmi:startTime`, `fmi:stopTime`, `fmi:stepSize` and `fmi:tolerance` and calls
  `engine.Run` once per compute question. Outputs, including the reserved
  `fmi:time`, bind at `stopTime`.
- `internal/exec/fmi/runner.go` writes one `fmi/1` JSON object to the runner's
  standard input and reads one JSON object from standard output; the runner
  exits after that exchange.
- No handle outlives the call: no session object, no step or read API, no
  runner protocol for a long-lived process. The only reserved output is
  `fmi:time`.
- No upstream issue or pull request proposing a session surface was found for
  this capability; the design below is therefore recorded here rather than
  attributed to an upstream ticket.

## What the change implements

An additive session layer over the same engine seam:

1. A long-lived `fmi/2` command protocol on one runner process. `init` carries
   the FMU, interface, experiment and start values; `step` carries a time;
   `read` carries the variables to answer. Each command has exactly one JSON
   answer line, or an `error` member that fails the performance as a
   `*fmi.RunnerError`.
2. `SessionManager` opens one session per caller and request, keyed by the
   caller identity and the hash of the init command. Re-opening the same
   request returns the same live session. Sessions are bounded: at most 16
   open, 60 seconds idle, 10 minutes absolute lifetime and 100,000 steps by
   default, with a janitor reclaiming ones nobody returns to. Each command
   answers within `OPENSYSML_TOOL_TIMEOUT` and is bounded by
   `OPENSYSML_TOOL_MAX_OUTPUT`.
3. `Session.Step` validates the time locally (finite, not backwards, not past
   `fmi:stopTime`) before asking the runner; `Session.Read` types each answer
   through the same variable reader the one-shot reply uses, so both paths bind
   values identically.
4. `SessionManager.ToolRunner` wraps an existing tool runner. A tool call that
   binds the new reserved input `fmi:stepTime` steps the caller's session and
   binds the values read there; every other call, and every call naming another
   tool, goes to the wrapped runner untouched. This is the in-model leg: a set
   of annotated calc usages that each ask for a different time share one
   initialized runner.
5. A stepped call that reaches the one-shot engine without a session runner is
   refused with a `*fmi.SessionRequiredError`; it is never silently answered
   with the value at `stopTime`.

The one-shot path is byte-compatible: a call without `fmi:stepTime` writes the
same `fmi/1` request and reads the same reply as before.

## Design decisions and alternatives rejected

- **A long-lived runner process with a command protocol, not a batch of
  outputs in one `fmi/1` reply.** Asking the existing runner for a series in its
  one reply would still compute a whole experiment in one invocation and would
  not give callers independent "step to t, then read" control. The session
  protocol preserves the meaning of a step and keeps the one-shot reply shape
  untouched.
- **A separate `fmi/2` protocol, not an extension of `fmi/1`.** Adding fields
  to the one-shot request or reply would change bytes existing runners and
  existing tests depend on. A distinct protocol lets a runner opt in and lets
  the one-shot path stay byte-identical.
- **An in-process Go API and a tool-runner adapter, not a new RPC or wire
  field.** A gRPC method or a `Value`/capability field would require changes to
  every client library. The session is consumed by hosts that already hold a
  `runtime.Context`; the adapter reaches it through the existing tool-runner
  interface, so no client is touched.
- **Keying by caller as well as request, not a single global session.** A
  shared workspace or subscription already needs per-principal isolation in
  this codebase; an FMU's state is no different. Two principals stepping the
  same FMU to different times must not read each other's state.
- **Bounded manager and lazy janitor, not an unbounded cache.** A long-lived
  host can create sessions for many FMUs. Count, idle, lifetime and step bounds
  plus a janitor keep that map and its runner processes from growing forever.
- **A per-key in-flight placeholder, not a global lock during init.** Waiting for
  an unanswered runner under the manager's lock would block every other open,
  close and reclaim. Concurrent opens for one key now wait on that key's
  placeholder; the manager's lock is held only for map updates. The step budget
  is checked only when another step is asked for, so the last allowed step's
  value can still be read. A budget-exhausted session is still reclaimed by the
  janitor on a later sweep, so the final value must be read before that sweep.

## Public surface added

All new names are in `internal/exec/fmi`; no protobuf, RPC, capability flag,
CLI flag or environment variable changed.

| Surface | Name / shape |
|---|---|
| Constants | `SessionExchange` (`fmi/2`), `StepVariable` (`fmi:stepTime`) |
| Constructor | `NewSessionManager(look)`, `NewSessionManagerWith(look, config)` |
| Configuration | `SessionConfig{Idle, Lifetime, MaxSessions, MaxSteps}` and `DefaultSessionConfig()`; defaults 60s, 10min, 16, 100000 |
| Manager | `(*SessionManager).Open(ctx, caller, call) (*Session, error)`, `(*SessionManager).ToolRunner(caller, delegate) runtime.ToolRunner`, `(*SessionManager).Close() error` |
| Session | `Step(ctx, t)`, `Read(ctx, names)`, `Time()`, `Caller()`, `FMU()`, `Close()` |
| Typed errors | `SessionError`, `SessionExpiredError`, `SessionLimitError`, `StepError`, `SessionRequiredError`, all matching `ErrSession`; `ErrSessionExpired`, `ErrSessionLimit`, `ErrStep` for sentinel checks; runner faults reuse `RunnerError`, process and output faults reuse `runtime.ToolError` |
| Wire format | `fmi/1` unchanged: one JSON object in, one out. `fmi/2`: newline-delimited JSON commands (`protocol: 2`, `action: init\|step\|read`, `time`, `outputs`, `error`) and one answer line per command; version 2 throughout, never mixed with version 1 |
| Reserved input | `fmi:stepTime` selects a session; `fmi:startTime`, `fmi:stopTime`, `fmi:stepSize`, `fmi:tolerance` and `fmi:time` keep their meanings |
| Documentation | `docs/reference/fmi.md`, `docs/project/spec-compliance/15-analysis-engines.md`, this record |

## Acceptance criteria and their tests

| Criterion | Named test |
|---|---|
| A fixture FMU with a known ODE | `decayFMU` in `internal/exec/fmi/session_test.go`: `y' = -k*y`, `y(0) = y0`, solved analytically by the stand-in runner |
| One initialize plus N steps equals N whole runs | `TestSessionStepsMatchWholeRuns` compares four stepped readings with four whole runs of the unchanged one-shot engine |
| A counting runner invoked once | The same test asserts one process start and one init for the session path and four starts for the whole-run path; `TestSessionToolRunnerInModel` asserts one start and one init across four calc usages |
| Go test over the engine seam | `TestSessionStepsMatchWholeRuns`, `TestSessionProtocolCommands`, `TestSessionRunnerFaults`, `TestSessionStepFaultIsTyped` |
| In-model analysis leg | `TestSessionToolRunnerInModel` drives four `calc` usages through a session-backed `runtime.ToolRunner` |
| Byte-compatible one-shot behavior | `TestSessionToolRunnerDelegatesOneShot`, the unchanged `engine_test.go` request assertions, and `TestSessionProtocolCommands` pinning the additive command bytes |
| Bounded sessions | `TestSessionBounds` (count, steps, idle lifetime), `TestSessionReadsTheLastAllowedStep`, `TestSessionBudgetReadsEveryStep` (the last allowed step's value stays readable), and `TestSessionManagerZeroValue` |
| Isolated per caller | `TestSessionIsolatesCallers` (two principals at different times, plus re-open returning the same session) |
| Typed failure modes | `TestSessionRefusesABackwardStep`, `TestSessionRefusesAnUnknownOutput`, `TestOneShotRefusesASteppedCall`, `TestSessionRunnerFaults`, `TestSessionStepFaultIsTyped`, `TestSessionCleanExitIsTyped` (a runner that exits successfully before answering yields a typed process fault, not a panic) |
| Safe concurrent reclamation and startup | `TestSessionJanitorReclaimWhileStepping` races an idle reclaim against Step's reads of the session's close state and process handle; `TestHungInitDoesNotBlockManager` keeps the janitor and a second caller responsive while init is unanswered; `TestSameKeyJoinsInFlightAtCapacity` joins a same-key pending init at capacity while a different key still gets the limit error; `TestInitDeadlineWithBlockedWrite` ends an init larger than the pipe's capacity at its deadline when the runner never reads stdin |

## Verification

Gates run on this branch:

- `gofmt -l .` — empty.
- `go build ./...` — clean.
- `go vet ./...` — clean; `go vet ./internal/exec/fmi` — clean after the final
  edits.
- Targeted: `go test ./internal/exec/fmi -count=1` — passing, including all
  session, in-model and negative tests; `go test ./internal/exec/engines
  ./tests/fmi -count=1` — passing.
- `python3 scripts/changelog.py check` and `python3 scripts/check-doc-ids.py` —
  passing. The added-comment gate over `git diff <baseline> -U2 -- '*.go'`
  prints nothing: no added comment run exceeds two lines.
- `go test ./tests/hygiene -count=1` — passing.
- A deterministic race-exercising test is present for the janitor/`Step`
  interaction, but this host's Go 1.27.4 toolchain has no race runtime
  (`__tsan_free` link errors), so `-race` was not run here and no race-test
  result is claimed.

`go test ./...` was attempted on the full tree. The runs did not finish: the
environment ran 19 or more test binaries concurrently from separate worktrees,
which exhausted the Go build temporary space and made long packages exceed
their timeouts. Every failure observed in those runs was traced to that
contention — stand-in runner scratch files shared between test binaries, a
go-build directory removed under the runner, and a package timeout — and each
affected test passed when run on its own. No failure was traced to this change.
The session tests now give their stand-in runner a private scratch directory so
they cannot read or overwrite another binary's mode, reply or counter files.
Optional corpus gates (the OMG corpora, the Reference-FMUs bundle and the PDF
converters) skip when their downloaded inputs are absent; none was needed here.

## Known limitations and follow-ups

- The reference FMI runner shipped with the Python client speaks `fmi/1` only.
  Sessions require a runner that implements the documented `fmi/2` commands;
  the one-shot path works with every `fmi/1` runner. Adding session support to
  that runner is the natural next step.
- The session is an in-process Go surface. Exposing it as an RPC would need a
  handle and step/read messages in the proto and matching updates to every
  client; none were added here.
- Bounds are configuration on the Go manager rather than environment variables
  or model metadata. A host that wants different limits passes a
  `SessionConfig`.
- The in-repo stand-in runner realizes the ODE fixture analytically; the
  protocol, not the integrator, is what the tests pin.
