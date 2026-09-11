# Environment variables

These variables are read by `sysml`, `sysml-lsp` and `sysml-grpc` alike. Each budget turns a
run that would never finish into a reported error instead of a hang.

| Variable | Default | Meaning |
|----------|---------|---------|
| `OPENSYSML_LIBRARY_PATH` | unset (use the bundled standard library) | Directory to load the SysML/KerML standard library from instead of the embedded copy |
| `OPENSYSML_MAX_STEPS` | `10000000` | Evaluation step budget: the number of expression evaluations one run may spend before it is reported as a runaway |
| `OPENSYSML_MAX_ACTION_STEPS` | `1000000` | Token-flow steps one action run may perform |
| `OPENSYSML_MAX_EVENTS` | `1000000` | Events one state machine run may dispatch, and the events one `%advance` drains |
| `OPENSYSML_MAX_DO_STEPS` | `5000000` | Do actions one state machine run may perform, and the ones one `%advance` drains |
| `OPENSYSML_MAX_ELEMENTS` | `1000000` | Collection elements one evaluation may hold — the bound on the memory a run holds rather than on the work it does |
| `OPENSYSML_MAX_CALC_DEPTH` | `10000` (ceiling `25000`) | Nested `calc` invocations one run may hold on the stack, which is what a recursion spends |
| `OPENSYSML_MAX_SWEEP_RUNS` | `1000` | Runs one parameter sweep or sample may make (`-sweep`/`-samples`, `%sweep`/`%samples`, `RunSweep`), each a whole analysis or calc run with the budgets above of its own |
| `OPENSYSML_JOBS` | the number of CPUs | Runs of one check that may go concurrently (`-jobs`, `%jobs`; the gRPC service reads it at startup), each on a worker of its own over the shared model. Bounds how many runs go at once, not the work or memory of any one of them: a fleet of `n` workers may hold `n` times `OPENSYSML_MAX_ELEMENTS`. The result of a check does not depend on it |
| `OPENSYSML_CALC_COMPILE` | unset (on) | Set to `0`, `false`, `off` or `no` to run every `calc` on the reference evaluator, instead of compiling a pure scalar body to a closure fast path on its first invocation; results, errors and step counts are the same either way, so this is a bisecting aid |
| `OPENSYSML_SMT` | unset (look for `z3`, then `cvc5`, on `PATH`) | Executable `%check`, `%explain`, `%solve`, `%configure` and `%optimize` drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental); `%optimize` needs `z3` in particular, as `(minimize …)`/`(maximize …)` is a z3 extension cvc5 does not implement |
| `OPENSYSML_SMT_TIMEOUT` | `10s` | How long one solver query may take, as a Go duration (`5s`, `500ms`), after which the verdict is `unknown` |
| `OPENSYSML_SMT_CORE_BUDGET` | `30s` | How long `%explain` may spend reducing an unsat core to a minimal one, as a Go duration; past it the solver's own core is reported, said not to be necessarily minimal |
| `OPENSYSML_SMT_MAX_CONFIGURATIONS` | `32` | How many variant selections `%configure … all` may report before saying the enumeration was cut short at the bound |
| `OPENSYSML_GRPC_INDEX_POOL` | `4` | Whether `sysml-grpc` builds the one shared standard library index ahead of the requests needing it; any positive value prewarms, `0` builds it on the first request instead |

Every variable above uses the `OPENSYSML_` prefix. The eight that predate it
(`OPENSYSML_LIBRARY_PATH`, the six `OPENSYSML_MAX_*` budgets and
`OPENSYSML_GRPC_INDEX_POOL`) also answer to their legacy `SYSML_`-prefixed
names (`SYSML_LIBRARY_PATH`, `SYSML_MAX_STEPS`, and so on), which remain accepted
indefinitely. When a variable is set under both prefixes and the `OPENSYSML_`
value is non-empty, the `OPENSYSML_` value wins. Setting only the legacy name
prints a one-time deprecation warning to standard error that names the
`OPENSYSML_` form to switch to.

The three `OPENSYSML_SMT*` variables belong to the experimental solving extension
(`%check`/`%explain`), which needs an external z3 or cvc5. Installing one is covered in
[1. Install: installing a solver](../guide/01-install.md#installing-a-solver-optional); the
extension follows the design of OpenMBEE's [HMF](https://github.com/hivecore-dev/hmf)
(see [Acknowledgements](../../README.md#acknowledgements)).
`OPENSYSML_SMT` takes an executable name or a path and is consulted before `PATH` is searched
(where `z3` is preferred over `cvc5`); a value that names no executable file is reported rather
than falling back to the search. It may name **any** solver that speaks SMT-LIB2 on standard
input, not only those two. The feature subset a backend must support, what z3 and cvc5 were each
measured to support, and how a backend that lacks a feature is reported are described in
[1. Install: solver compatibility](../guide/01-install.md#solver-compatibility--pointing-the-driver-at-another-solver).
Nothing else in the toolchain reads these variables, and the concrete evaluator needs no solver.

The budgets are what turn a run that would never finish into a reported error instead
of a hang. They count different things (expression evaluations, action token
steps, dispatched events, do actions, materialized collection elements),
so raising one says nothing about the others, and each has its own variable.
`OPENSYSML_MAX_SWEEP_RUNS` counts runs rather than work inside a run: a plan whose
ranges would make more runs than it allows is refused before the first one is
made, naming the count the plan asks for and the bound it exceeds.
`OPENSYSML_JOBS` is no budget at all but the width of the fleet: how many of one check's
runs — an exploration's linearizations, the engines `-engine all` consults — may go at
once. A value that is not a positive integer is refused at startup; `-jobs` and `%jobs`
override it for one invocation or session. See
[Running in parallel](cli.md#running-in-parallel).

A budget bounds **one run** (one `%eval`, one `%instantiate`, one `%calc`, one
action, one state machine), not a whole session, so a long REPL session of small
operations never runs out. A run started inside another, such as an action invoked
from an expression, shares the outer run's budget rather than getting a fresh
one, and so does a run stepped through with `%step`/`%advance`.

The step and event defaults are chosen by how long a runaway takes to report rather
than by memory. Those steps allocate nothing that outlives them (peak RSS is about 34
MB whether a run spends ten thousand steps or fifty million), and the only thing
they make grow is a `%trace`, at 34–83 bytes per entry. At the measured ~13.6M
evaluation steps/s and ~1.9M events/s, each default reports a runaway within about
a second, and a fully traced run at those four ceilings holds about 320 MB.

Collection elements are the exception, and `OPENSYSML_MAX_ELEMENTS` is the budget
that really is about memory: a materialized element is a 104-byte value that lives as
long as the collection holding it, and `1..10000000` creates one per step. Every way of
materializing a sequence is charged against it (a range, a sequence literal,
`->collect` and the other collection operations), so the default bounds the
elements held at once at about 104 MB, in the same range as the figures above:

```
error: evaluation failed: collection element limit exceeded
(1000000 elements; raise OPENSYSML_MAX_ELEMENTS to allow more)
```

Because it bounds memory rather than work, the count is what a statement's evaluation
holds at once: a loop building a ten-element collection a million times never approaches
it, while a single `1..2000000` exceeds it immediately.

`OPENSYSML_MAX_CALC_DEPTH` is about stack rather than work: a recursive calculation
evaluates to its result as long as it terminates within the depth, and one that
does not terminate is reported instead of exhausting the stack:

```
error: calc recursion limit exceeded: calc P::spin nested 10000 deep
(unbounded recursion?; raise OPENSYSML_MAX_CALC_DEPTH to allow more)
```

A nested invocation costs about 10 KB of stack, so this is the one budget with a
ceiling: a value above 25000 is refused, because past that point the goroutine stack
limit (a fatal error rather than a reported one) would be reached before the
budget was. A recursion that needs more depth than that should be rewritten as a loop.

The evaluation step budget:

```
error: execution failed: eval assignment RHS: evaluation step limit exceeded
(10000000 steps; raise OPENSYSML_MAX_STEPS to allow more)
```

A legitimately long run (a numeric integration in an action body, say) needs a
higher ceiling, so raise it for that run:

```bash
OPENSYSML_MAX_STEPS=200000000 sysml descent.sysml
```

Unset or empty means the default. Anything that is not a positive integer is
reported at startup (and at gRPC service construction) rather than silently
ignored:

```bash
$ OPENSYSML_MAX_STEPS=lots sysml model.sysml
sysml: OPENSYSML_MAX_STEPS="lots" is not an integer: set it to a positive number of evaluation steps (default 10000000)
```

The other budgets behave identically, and their errors name the variable that
raises them:

```
execution exceeded max steps (1000000 steps; raise OPENSYSML_MAX_ACTION_STEPS to allow more), possible infinite loop
state machine exceeded max events (1000000 events; raise OPENSYSML_MAX_EVENTS to allow more), possible infinite loop
state machine exceeded max do action steps (5000000 steps; raise OPENSYSML_MAX_DO_STEPS to allow more), possible non-terminating do behavior
```

A long simulation therefore raises the state machine bounds rather than the
evaluation one:

```bash
OPENSYSML_MAX_EVENTS=20000000 OPENSYSML_MAX_DO_STEPS=100000000 sysml descent.sysml
```

## The gRPC service's shared library index

`OPENSYSML_GRPC_INDEX_POOL` is no longer a count: any positive value asks
`sysml-grpc` to build the standard library index before the requests that need
it arrive, and `0` disables this prewarming. The library does not depend on the model
and is immutable once loaded, so the service builds and freezes **one** index and gives
each model a thin overlay on top of it holding that model's own document. A cache
miss adds its document to that overlay instead of loading and expanding the
library again (measured on a 163-line model: about 0.5–0.9 ms rather than
100–128 ms), and 100 cached models cost about 1 MiB in total rather than 1.6 GiB.

A model writes only into its own overlay, so cached models stay independent and
none can see another's document. A request that arrives before prewarming has finished
builds the shared index itself, so an answer never depends on how far prewarming
got, and concurrent requests wait for one build rather than starting several.

```bash
OPENSYSML_GRPC_INDEX_POOL=0 sysml-grpc   # load the library on the first request instead
```

Anything but a non-negative integer is reported at service construction rather
than silently ignored. The legacy `SYSML_GRPC_INDEX_POOL` name remains accepted
for compatibility with deployments that set it.
