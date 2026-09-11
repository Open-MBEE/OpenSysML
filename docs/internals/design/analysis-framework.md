# The analysis framework: registered engines, one result vocabulary, parallel runs

A design for the layer that answers questions about a model — *does this requirement hold*,
*what are the outcomes of this behavior*, *which inputs satisfy these constraints*, *what does
this external tool compute for this analysis case* — by dispatching each question to one or
more **engines** registered against a common contract, running them in parallel under one
budget, and composing their answers under one vocabulary of evidence. The interpreter, the
`explore` scheduling policy, the parameter sweep, the SMT constraint solver, the proposed
behavioral model checkers and external simulators all become engines. The note fixes the
engine contract, the registry, how a question chooses engines, what a composed result may
claim, how runs are isolated so they can be parallel, and how the existing surfaces migrate
without changing what they mean.

**Status.** Stages 1, 2 and 4 of the [stages](#stages) below are implemented: `internal/core/analysis`
holds the contract (`Question`, `Engine`, `Result`, `Claim`, `Strength`, `Bounds`, `Budget`),
the registry and `auto` dispatch, and the `run`, `explore`, `sweep` and `solve` engines as
adapters over the interpreter, `runtime.Explore`, `Context.RunSweep` and `internal/core/solve`.
The REPL session and the gRPC service each own a `Default()` registry and put every question the
migration table lists to it, each plan on a worker of its own. Stages 3, 5 and 6 — parallel
runs, tools and the model checkers — are not implemented, so the sections on parallel execution
beyond one worker per plan and on external tools still describe a design. Two readings
the implementation took where the note left room: the `%run` in the migration table is the REPL
commands that share the CLI flags' code (there is no meta-command of that name), and a `solve`
question is one per element with that element's condition sets as its queries, so the solver is
found once per element and its absence is reported once, as before. Of the `Budget`, the
fields whose limits an engine applies in its own unit are applied by it — `Runs` as `explore`'s
runs, `sweep`'s rows (the limit `Context.RunSweep` now takes in place of the context's) and
`solve`'s queries (the rest left unasked and the set *not covered*), `Depth` by `explore`,
`Solver` by `solve` — and each result names the bound it ran under; `Deadline` is applied by
`Registry.Answer`, which bounds the plan's context by it so an engine that meets it returns
`context.DeadlineExceeded` and stops the plan on that step; `Steps` and `Memory` are taken by a
run-owned context at construction and are the limits a context the surface holds already
enforces, carried so a result can name them; `Jobs` is carried unread until
the parallel-runs stage gives it a coordinator. `BudgetOf` fills `Runs` in the unit of the
question's kind, so a sweep asked under an exploring schedule is bounded by
`OPENSYSML_MAX_SWEEP_RUNS`, not by the schedule's exploration runs.

This is the framework that the two model-checking designs are written into:
[bounded model checking](bounded-model-checking.md) explores the executor and
[SMT bounded model checking](smt-model-checking.md) encodes it for a solver, and each describes
by hand — a referee gate, a fallback, a verdict qualified by a bound — what this note makes the
shared contract. In the architecture's tiers it is the substance of
[Tier 6, analysis and verification drivers](../architecture.md#tier-6--analysis--verification-drivers--future).

## The problem this answers

Five places in the code already choose *how* to answer a question about a model, and each is a
separate plug point with its own selection flag, budget, result type and report:

| Plug point | Selection | Budget | Result type | Runs |
|------------|-----------|--------|-------------|------|
| Scheduling policy — `reverse`, `declared`, `seed:<n>`, `explore` (`runtime.SchedulePolicy`, `ParseSchedulePolicy`) | `-schedule`, `%schedule`, the `schedule` request field | `explore:runs=<n>,depth=<d>`, `OPENSYSML_MAX_ACTION_STEPS` | `runtime.Outcome`, `runtime.Exploration` with `ExplorationStatus` | one at a time; `Explore` builds a fresh `Context` per linearization and visits them in sequence |
| SMT backend — z3, cvc5, or a named binary (`solve.Discover`, `solve.Solver`) | `OPENSYSML_SMT` | `OPENSYSML_SMT_TIMEOUT` | `solve.Result` with `Status` `sat`/`unsat`/`unknown`, `SolveReport` with `SolveStatus` | one process per query |
| Parameter sweep and sampling (`Context.RunSweep`) | `-sweep`, `-samples`, `%sweep`, `%samples`, `RunSweep` | `OPENSYSML_MAX_SWEEP_RUNS`, the request's `context.Context` | `SweepTable` of `SweepRow` | one row after another |
| Analysis and verification cases (`RunAnalysis`, the verification RPCs) | `-analysis`, `-requirement`, `-constraint`, `-satisfy`, `%run`, `VerifyConstraint`… | `OPENSYSML_MAX_STEPS`, `OPENSYSML_MAX_CALC_DEPTH` | `Verdict` with `VerdictStatus` holds/fails/unresolved; `VerificationVerdict` pass/fail/inconclusive/error on the wire | one run |
| External analysis tools — `AnalysisTooling::ToolExecution` and `ToolVariable` in the standard library | metadata on the analysis case's actions | none | none | not run: the metadata is parsed and resolved, and nothing in `internal/core/runtime` reads it |

Each was the right shape for the question it answered. Together they have four costs:

- **No shared vocabulary of evidence.** `holds`, `pass`, `unsat`, `complete (6 runs)` and the
  proposed `proved within 40 moves` all mean "no violation found", with very different strength:
  one run under the default policy, every run of a sweep, no assignment of the free variables,
  every linearization within a budget, every schedule and input within a bound. A reader has to
  know which plug point produced a line to know what it is worth, and nothing stops a report from
  composing them — a verification case whose requirement was checked under `explore` prints the
  same `pass` as one checked under `reverse`.
- **Composition is by hand.** The SMT design's referee (its outcome sets must equal `explore`'s),
  its fallback (a refused body goes to `explore`), and its replay gate (a witness must reproduce
  in the interpreter) are each a paragraph describing how one engine calls another. The next
  engine — a state-machine checker, a simulator — would write the same paragraphs again.
- **Nothing runs in parallel.** `Explore` visits linearizations in sequence, `RunSweep` rows in
  sequence, `Session.mu` is held for the length of a REPL command, and the gRPC service handed
  each request exclusive use of one resolver and semantic model per model because both memoize
  into plain maps (undone by the isolation stage below). A sweep of 10 000 rows on a 16-core
  machine uses one core.
- **External tools have a spec-native binding and no runtime.** A model can already say, with
  standard-library metadata, that an action is computed by a named tool and which of its
  parameters map to which tool variables. The runtime cannot act on it, and if it could, nothing
  would say how the tool's answer differs in standing from the evaluator's.

The framework is one answer to all four: an engine contract that every plug point implements, a
result type that every engine returns and that names the strength of its evidence, a coordinator
that runs engines in isolated contexts in parallel under one budget, and a dispatch rule that
turns a question into the engines that can answer it.

## What exists to build on

- **`Explore(stop, policy, fresh, run)`** is already the isolation shape a parallel run needs:
  the caller supplies a function that builds a fresh `runtime.Context`, and each linearization
  runs in its own. Nothing about a run leaks into the next except the recorded choice prefix
  that picks the next one, and a caller that goes away ends the exploration between runs.
- **`solve.Capabilities`** and **`solve.Discover`** are a capability declaration and a probe: a
  backend is asked once what it supports (models, unsat cores, incremental checks, datatypes,
  strings, nonlinear arithmetic, optimization) and a query that needs more is refused with a
  typed error before it is sent. The engine contract generalizes exactly this.
- **`Context.RunSweep(stop, target, plan, runs, run)`** already separates the plan (the rows)
  from the run (one execution of a row), bounds the rows by `runs` (the context's
  `OPENSYSML_MAX_SWEEP_RUNS` when zero) and checks a `context.Context` between rows. Its rows are
  independent by construction; only the loop is sequential.
- **The frozen symbol index.** `symbols.Index.Freeze` bars an index from writes so that every
  model shares one copy of the standard library, and `NewOverlay` builds a writable index over a
  frozen one. Model-derived state that is immutable and shared beneath run-derived state that is
  owned is the pattern the coordinator applies one level up.
- **Service capabilities.** The gRPC service advertises what it can do as named capabilities
  (`verification`, `schedule`, `schedule_explore`, `case_evaluations`, …) and clients check them
  before asking. Registered engines are advertised the same way.
- **Three verdict disciplines already in force**, which the framework adopts rather than
  invents: `solve` keeps `sat`, `unsat` and `unknown` distinct and reports a solver that cannot
  answer as a typed error; `explore` prints `incomplete: runs hit after 1024 runs` rather than
  `complete`; viewpoint conformance returns `unevaluable` with a reason and never a pass it
  could not compute.
- **`AnalysisTooling`.** The standard library's `ToolExecution { toolName; uri }` and
  `ToolVariable { name }` metadata, and the pilot example that binds an analysis action to a
  named tool and maps its parameters to the tool's variables.

## The contract

### Questions

A **question** is what is asked, independently of who answers it. It names a subject in the
model, what is asked of it, the bindings that are fixed, and what is left free.

| Question | Asked of | Free | Today's surface |
|----------|----------|------|-----------------|
| `evaluate` | a calc, an analysis case, an action or state machine, a constraint or requirement on a subject | nothing: one execution under stated bindings and a stated scheduling policy | `-calc`, `-analysis`, `-action`, `-state`, `-constraint`, `-requirement`, `-satisfy`, `%run`, the verification RPCs |
| `outcomes` | a behavior | the schedule | `-schedule explore` |
| `holds` | a condition over a behavior or an analysis case | the schedule and/or the unbound inputs, within stated bounds | proposed by both model-checking designs |
| `sensitive` | a behavior | the schedule | proposed by the SMT design |
| `satisfiable` | a set of conditions | the unbound features | `%check`, `%explain`, `%solve`, `%configure`, `%optimize` |
| `sweep` | a calc or analysis case | nothing per row; the rows enumerate or sample a domain | `-sweep`, `-samples`, `RunSweep` |
| `compute` | an action annotated `ToolExecution` | nothing: the tool is asked once | none |

The distinction that matters is what is **free**: `evaluate` and `sweep` ask about executions
whose every choice is fixed, `outcomes`, `holds` and `sensitive` ask about *all* executions
within bounds, `satisfiable` asks whether *some* assignment exists. A result's strength (below)
is judged against what the question left free, so an engine that fixed something the question
left open cannot claim to have answered it.

### Engines

An **engine** answers a set of questions for a set of subjects. The contract, as the Go
interface the coordinator sees:

```go
// Engine is one registered way of answering questions about a model.
type Engine interface {
	// Name is the stable identity the user selects by (-engine <name>).
	Name() string
	// Describe reports what the engine can do, fixed at registration.
	Describe() Description
	// Covers says whether the engine can answer q for this model, before any
	// work is done. A refusal names the construct or condition it cannot
	// handle; a refusal is never a result.
	Covers(model *Model, q Question) Coverage
	// Run answers q within the budget, in a context of its own. It never
	// mutates model; it stops when ctx is done and reports how far it got.
	Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error)
}
```

`Description` is the engine's capability declaration: the question kinds it answers, the subject
kinds it accepts, whether it needs an external process and which (a solver, a tool binary), the
bounds it takes, whether it produces witnesses that can be replayed, and its **authority** —
the strongest evidence it can ever produce (next section). `Covers` is `solve`'s refusal made
uniform: the SMT design's *a body the encoding cannot express refuses the whole behavior before
any query runs* is this method, and `explore` refuses a question with free inputs, since it runs
concrete values only.

`Model` is the model-derived, immutable input every engine reads and none writes: the frozen
symbol index, the lowered `ActionGraph`s and `StateGraph`s, the condition expressions. A run's
mutable state — a `runtime.Context`, a solver process, a tool process — is the engine's own,
made inside `Run` and released before it returns.

The engines this note names, each an adapter over code that exists or is designed:

| Engine | Answers | Over | Authority | Adapter over |
|--------|---------|------|-----------|--------------|
| `run` | `evaluate`, `sweep` (one row) | the interpreter under a fixed scheduling policy | *observed* | `runtime.Context`, `RunAnalysis`, `CheckConstraintOn`, the evaluator |
| `explore` | `outcomes`, `holds` (concrete inputs), `sensitive` (from the outcome table) | every linearization within `runs`/`depth` | *proved* over schedules on `complete`; *witnessed* for a violation | `runtime.Explore` |
| `sweep` | `sweep` | one `run` per row, rows in parallel | *observed* per row | `Context.RunSweep`'s plan; the row loop moves to the coordinator |
| `solve` | `satisfiable` and its variants (explain, synthesize, configure, optimize) | an external SMT process | *proved* for `unsat` over the encoded fragment; *witnessed* for a `sat` model the evaluator confirms | `internal/core/solve`, `SolveReport`; the evaluator's confirmation of a `sat` model is the one addition |
| `smt` | `holds`, `sensitive`, `outcomes` (as a bounded enumeration) | schedules and free inputs symbolically, within `k` moves | *proved* (with induction), *bounded*, *witnessed* | the [SMT design](smt-model-checking.md) |
| `check` | `outcomes`, `holds`, deadlock | the executor with snapshots and partial-order reduction | *bounded*; *witnessed* | the [explicit-state design](bounded-model-checking.md) |
| `tool:<name>` | `compute` | one external process per invocation | *observed* | `AnalysisTooling` metadata and a process protocol (below) |

`explore` is at the same time the engine that runs the real interpreter over every schedule and
the **referee**: any engine whose witness is a schedule must replay it through `explore`'s
replay policy, and any engine that enumerates outcomes must agree with `explore`'s table on the
conformance corpus. That role is a property of the framework, not of the SMT design alone.

### Results and the strength of evidence

Every engine returns one `Result`, and the report prints it in the model's terms:

```go
type Result struct {
	Question Question
	Engine   string        // who answered, and its version
	Claim    Claim         // what is asserted: holds, violated, sensitive, a value, a table, nothing
	Strength Strength      // how the claim is supported (below)
	Bounds   Bounds        // every bound the run took, and which were reached
	Witness  *Witness      // a schedule and inputs the interpreter replays, when the claim has one
	Reason   string        // for not-covered: the construct, the unknown, the budget, the disagreement
	Values   []Evaluation  // the feature values, outputs or rows the question asked for
	Elapsed  time.Duration
}
```

`Strength` is what the five plug points lack today: a single scale on which one engine's answer
can be set beside another's.

| Strength | Means | Produced by |
|----------|-------|-------------|
| **proved** | the claim holds for everything the question left free, with no bound reached | `solve` on `unsat`; `smt` when k-induction closes; `explore` on `complete`, for the schedules of a model whose inputs are concrete |
| **bounded** | the claim holds for everything free *within the stated bounds*, which the result names | `smt` on `unsat` at `k`; `check` within its state and depth budget |
| **witnessed** | a concrete execution exhibits the claim, and the interpreter has replayed it | a violation, an outcome or a `sat` model whose replay reached the described state |
| **observed** | the claim held on the concrete executions that were run, which fixed something the question left free | `run`; `sweep`; `tool:<name>`; `explore` on `incomplete` for the outcomes it did find |
| **not covered** | nothing is claimed; the reason is printed | a refusal from `Covers`; a solver `unknown`; a budget reached without completing; a witness that did not replay |

Two orderings live on this scale, and keeping them apart is the whole point. For a **universal**
claim (`holds`, *not sensitive*, the completeness of an outcome set) the order is
`proved > bounded > observed > not covered`, and a result may only be printed with the strength
its engine earned: `smt`'s `unsat` at `k = 40` is *bounded*, never *proved*; `run` under
`reverse` is *observed*, never *bounded*; `explore`'s `complete` is *proved* for the question it
answered — every schedule, for the inputs as written — and `explore` refuses a question that
leaves inputs free, since it runs concrete values only. For an **existential** claim (a
violation, a sensitivity, a satisfying assignment) *witnessed* is the only strength that is a
claim at all: a solver's `sat` without a replay, or a simulator's report of a failure the
interpreter does not reproduce, is *not covered* with the disagreement as its reason.

Three rules compose results from several engines:

1. **A witnessed violation dominates every universal claim about the same question.** If `run`
   under `reverse` observed the requirement holding and `explore` witnessed a schedule violating
   it, the composed result is *violated* with the witness; the observation is kept as evidence
   that the default linearization passes, which is itself useful.
2. **A universal claim is composed at the strongest strength any engine earned, never stronger,
   and the weaker results are not discarded.** `smt` *bounded at 40 moves, inputs free* beside
   `explore` *proved over schedules, inputs as written* is reported as both, since neither
   implies the other: one covers every input to a bound, the other every schedule for one input.
   Completion order plays no part: a fast *observed* result does not become the answer because
   the *bounded* one is still running.
3. **Two engines that contradict each other produce a disagreement, not a winner.** `smt`
   *bounded* that the requirement holds and `explore` *witnessed* a violation is the referee
   gate firing: the interpreter is normative, so the violation stands, and the `smt` result is
   demoted to *not covered* with the disagreement as its reason and the case recorded for the
   corpus. Two *observed* results that differ (`reverse` gives `x = 1`, `declared` gives
   `x = 2`) are not a disagreement; they are a *witnessed* sensitivity.

Today's statuses map onto the scale without changing what any of them prints:

| Today | Engine | Claim | Strength |
|-------|--------|-------|----------|
| `VerdictHolds` / `pass` | `run` | holds | observed |
| `VerdictFails` / `fail` | `run` | violated | witnessed (the run *is* the replay) |
| `VerdictUnresolved` / `inconclusive`, `error` | `run` | none | not covered |
| `ExplorationStatus` complete | `explore` | the outcome set is complete | proved, over schedules for the inputs as written |
| `ExplorationStatus` incomplete | `explore` | these outcomes exist | observed |
| `unsat` | `solve` | no assignment | proved over the encoded fragment; *not covered* when the conditions round in floating point, as `%check` already downgrades an exact-real `unsat` |
| `sat` | `solve` | this assignment | witnessed once the evaluator confirms the assignment satisfies the conditions; *not covered* with the disagreement when it does not |
| `unknown`, `unavailable` | `solve` | none | not covered |
| `unbounded` | `solve` | the objective has no optimum | proved over the encoded fragment |
| `no-optimum` | `solve` | none: the conditions are satisfiable but the optimum was not established | not covered, with the bound or unverified answer kept in `Values` |
| `proved`, `bounded`, `violated`, `sensitive`, `not covered` in the SMT design | `smt` | as named | proved, bounded, witnessed, witnessed, not covered |

## The registry

Engines are registered by name into a registry value, the shape the validation passes use
(`passes.NewRegistry`, `Registry.Register`, `Registry.Passes`):

```go
// NewRegistry returns an empty registry.
func NewRegistry() *Registry

// Register adds an engine; a second engine with the same name is a typed
// error, not a silent replacement.
func (r *Registry) Register(e Engine) error

// Engines returns every registered engine in name order.
func (r *Registry) Engines() []Engine

// Default returns a registry holding every engine the build knows.
func Default() *Registry
```

`sysml`, `sysml-lsp` and `sysml-grpc` each build one `Default()` registry at startup and hand it
to the coordinator, so the three binaries answer with the same engines. Engines that
need an external process — `solve`, `smt`, `tool:<name>` — register unconditionally and report
the process's absence through `Covers`, so `sysml -engines` lists every engine the build knows
with its status (`solve: z3 4.13 at /usr/bin/z3` or `solve: no solver found (OPENSYSML_SMT
unset, z3 and cvc5 not on PATH)`), and a question that needs one gets a *not covered* result
naming it, as `%check` reports an absent solver today.

External tools are the one class of engine not known to the build. They are registered from a
**tool manifest**, a directory named by `OPENSYSML_TOOLS` holding one entry per tool: the
executable, the `toolName` it answers to, its version, and the tool-variable names it accepts.
Each entry becomes a `tool:<name>` engine. A `ToolExecution` whose `toolName` has no manifest
entry is refused by `Covers` — *tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS* —
never skipped, and never replaced by evaluating the action's body as if the metadata were not
there.

Two things the registry deliberately is not:

- **Not a plugin loader.** Go's `plugin` package is limited to Linux, FreeBSD and macOS,
  requires the plugin and host to be built with identical toolchains and dependencies, and
  cannot unload; a plugin
  ABI would be a compatibility surface the wire contract does not want. In-process engines are
  compiled in; out-of-process engines are processes with a protocol. How a user brings an
  engine, a strategy or a tool of their own — a process speaking the protocol, a WebAssembly
  module, or Go over the public package — and the standing its answers are given is
  [its own note](bring-your-own-engines.md).
- **Not global mutable state a test can trip over.** There is no package-level registry; the
  binaries own theirs, and a test builds its own with `NewRegistry()` and passes it to the
  coordinator, so a test that registers a fake engine cannot see or disturb another's.

## Dispatch

A question is answered by a **plan**: the engines that cover it, in the order their authority
ranks them for that question's kind, and how they are combined.

```
-engine <name>      exactly this engine; a refusal or a not-covered result is the result
-engine auto        the strongest covering engine; on not covered, the next; the plan names each
-engine all         every covering engine, in parallel, composed under the rules above
```

`auto` advances on every *not covered* result, whether it came before the run (`Covers` refused
a construct, the engine's process is absent) or after it (the solver returned `unknown` or hit
its timeout, the budget was reached without completing, the witness did not replay). Each
result it advanced past stays in the plan with its reason, beside the answer the fallback
gave, so a reader sees both that `smt` timed out and that `explore` then enumerated. It does
not advance on an `error` from `Run`: an error is a fault — the subject did not resolve, the
model has no lowered graph for it, an engine broke its own contract — that no other engine
would answer differently, and it stops the plan and is reported as an error, not composed. An
engine that cannot get an answer for a reason particular to itself (a solver process that died
mid-query, a solver that lacks a capability the query needs) returns *not covered* with that
reason, as `solve` and `%check` already distinguish an unusable solver from a malformed query.
The rule does not reach into a run: a tool process that fails *inside* an execution fails that
performance (below), as a failed calc does, and the question that ran it reports the failure
the way `run` reports a failed calc today, since nothing else answers `compute`.

`auto` is the default and preserves every existing surface: for `evaluate` the only covering
engine is `run`; for `sweep` it is `sweep`; for `satisfiable` it is `solve`; for `outcomes` it
is `explore` today and `smt` or `check` once they exist and cover the model. The plan is
printed with the result (`plan: smt refused (RealFunctions::sqrt at node 'settle');
explore`), so a *bounded* answer from the fallback is never mistaken for the *proved* one the
user may have expected — the SMT design's *not covered → run explore* row is this rule.

`all` is the referee mode. It is what the SMT design's referee section runs over the conformance
corpus, and what an engineer runs when a proof matters enough to be cross-checked: `smt` proves,
`explore` enumerates, and the composed result carries both strengths or the disagreement.
Until the parallel-runs stage puts `all` on its work queue, the surface stage runs the covering
engines one after another in name order; the composition, the marking of a cancelled engine and
the disagreement result are defined over the set of results and do not depend on the order the
engines ran or finished, so moving `all` onto the queue changes no answer.

Explicit selection is never overridden: `-engine smt` on a model `smt` refuses prints the
refusal and stops. The framework's job is to make the answer's standing legible, not to be
clever about which engine to use.

## Budgets and bounds

A **budget** is what a run may spend; a **bound** is what a claim is qualified by. They meet
when a run spends its budget without finishing: the result's `Bounds` records the bound that was
reached and the strength drops from *bounded* to *observed* (for a universal claim) or the result
becomes *not covered*. A budget reached is never a proof, and every result prints the bounds it
took whether or not it reached them, so `explore`'s `complete (6 runs)` becomes
`complete: 6 linearizations within runs=1024 depth=64`.

```go
type Budget struct {
	Deadline time.Time     // wall clock for the whole plan
	Jobs     int           // concurrent runs, -jobs / OPENSYSML_JOBS, default NumCPU
	Runs     int           // explore runs, sweep rows, solver queries — the unit is the engine's
	Depth    int           // moves per run
	Steps    int           // OPENSYSML_MAX_STEPS and its kin, per run, unchanged
	Solver   time.Duration // OPENSYSML_SMT_TIMEOUT, per query, unchanged
	Memory   int           // OPENSYSML_MAX_ELEMENTS per run, unchanged; times Jobs is the fleet
}
```

The per-run limits that exist today keep their names and meanings. A context of a run's own
(`Model.NewContext`) takes `Steps` and `Memory` from the budget at construction: a positive
`Steps` is the context's `MaxSteps` (`OPENSYSML_MAX_STEPS`, expression evaluations per run) and
nothing else, a positive `Memory` its `MaxElements` (`OPENSYSML_MAX_ELEMENTS`), and a zero field
leaves the bound at what the surface's `Fresh` built — the engine's own default. The other step
kin (`OPENSYSML_MAX_ACTION_STEPS`, `OPENSYSML_MAX_EVENTS`, `OPENSYSML_MAX_DO_STEPS`,
`OPENSYSML_MAX_CALC_DEPTH`) are never derived from `Steps`: the runtime counts them in
different units (token-flow steps, events, do-steps, nesting depth), so any arithmetic from the
one figure onto them would give a kin a meaning it does not have, while the identity onto the
kin the field is named after is the one mapping under which `BudgetOf` followed by construction
reproduces the context's limits exactly. A budget that does not set `Steps` leaves every kin at
the context's value. A context the surface holds (`Model.Context`) is not rebuilt and keeps its
limits, and `BudgetOf` reads its budget from them. What the framework adds is the two that
only make sense once several runs share a machine: `Jobs`, and a `Deadline` that applies to the plan. Cancellation is the `context.Context` that `RunSweep`
already threads through: `Registry.Answer` derives the plan's context from `Deadline` and
checks it before it consults each engine and before it takes an engine's answer, every engine's
`Run` observes the context between units of work and returns its error, and a deadline met is
an error that stops the plan on the step that met it, not a *not covered* result; an answer that
arrives after the deadline is not taken. A unit of work itself — one evaluation, one row, one
run — is bounded by the runtime's own per-run limits, not by the clock. The
`all` coordinator of the surface and parallel-runs stages is what returns what it has — so
`-engine all` with a solver that will not answer still returns when the `explore` half is done —
by composing the finished engines' results around the stopped step.

## Parallel execution

### What may be shared

The rule is the one the runtime already lives by, made a contract: **model-derived state is
immutable and shared; run-derived state is owned by one run.** The AST is immutable after
parsing, and the frozen index is built to be read concurrently. Two components are not safe to
share, and they are the actual work of this section:

- **The resolver and the semantic model memoize into plain maps.** Two runs on two goroutines
  resolving the same name would race. The gRPC service once serialized every runtime request on
  a model behind one shared pair for this reason.
- **`runtime.Context`** is a run's mutable state by design and is never shared; `Explore`'s
  `fresh` already builds one per linearization. The memo tables the runtime derives from the
  model — calc shapes, write and invocation targets, literal caches, compiled calc closures,
  effective features — live in `runtime.Model`, which memoizes into plain maps as the resolver
  does and is shared the same way: one per worker, read by every context built over it.

The design takes the first apart by **giving each worker its own resolver, semantic model and
`runtime.Model` over the shared frozen index**, rather than by making the memoization
concurrent. A worker (`analysis.Worker`) is built once per plan and reused across its runs, so
the memoized resolutions and the runtime's derived tables are paid `Jobs` times, not once per
run; the index, the standard library and the lowered graphs are built once and read by all.
Making the lazy, recursive resolver lock-safe instead was considered and rejected below. The
`libs` snapshot already makes the frozen standard-library index cheap to share; the per-worker
cost is the model's own resolutions, and the framework measures it (`plan: 8 workers, 1.2 s
warming`) so the trade is visible. The figure is carried in the result and printed under
`-json` alone, as the `plan` key's `workers` and `warming` (milliseconds); the human-readable
report does not print it, because warming is wall time and the standing line reports evidence,
not cost — printing it would make every human-readable report non-reproducible for a figure
that describes the run, not the answer.

The surface hands the framework an `analysis.Model` with three ways to a context: `Context`,
the context the surface itself holds (the REPL's own, whose objects a `%run` names); `Semantics`,
how to build a worker's `runtime.Model` — resolver, semantic model and the runtime's memo
tables — over the shared index; and `Fresh`, how to build a run's context over a worker, which
allocates the run-derived state alone (objects, lifetimes, the bus, the clock, the scheduler).
`Registry.Answer` gives each plan a copy of the model, so two plans on one model never share a
worker, and builds the plan's worker on its first run-owned context; the count and the time it
took are the result's `Workers` and `Warming`. Both are lazy, so `Warming` is the cost of
construction; the resolutions themselves are paid inside the runs. A plan in a context the
surface holds builds no worker: that context is the surface's state, and
rebuilding it would lose the objects the plan was asked about.

The gRPC service builds a worker per request over the cached model's index and no longer holds
a lock between requests. The REPL session keeps a command lock, so a second command waits for
the first, and a state lock for the readers that run beside a command — completion, the
getters — which an exploration releases while its plan runs on a worker and contexts of its own.
What the explored runs name — the performers and subjects to instantiate, the held object owning
a nested case, the exhibits declared — is resolved into a plan before the release, so a run reads
nothing the state lock guards. A prompt run in the session's own context (`%run`, `%check`, `%sweep` on held objects) keeps
the state lock: its context is what the readers read, and releasing it there would be a shared
`Context`.

### Units of work

A run is the unit the coordinator schedules. Each engine says what its runs are:

- **`sweep`**: one row. Rows are independent by construction; the table is assembled in plan
  order, never arrival order, so the output is the sequential output.
- **`explore`**: one linearization. `Explore` finds the next prefix from the choice points the
  last run recorded, which is sequential as written; the parallel form is a **work queue of
  prefixes**. The first run records every choice point it passed; every untried alternative at
  every point is a subtree whose root prefix goes on the queue, and a worker that takes one runs
  it and enqueues the alternatives it finds below. The queue is ordered by prefix, so a free
  worker always takes the least prefix not yet run, and the `Runs` budget is a **cut in plan
  order, not a count of executions**: the result covers the `Runs` least linearizations, which
  are exactly the ones the sequential `Explore` runs before it stops, and a run that turns out
  to lie past the cut is discarded and charged to nothing. `Runs` is also a resource limit
  today, and stays one: a run is **committed** once every prefix before it has completed, so
  its position is fixed and below `Runs`; a run that has started but is not committed is
  **speculative**; and a plan may discard at most `Jobs` speculative runs in its lifetime. Until
  that allowance is spent, a free worker takes the least prefix not yet run whose position
  among the prefixes discovered so far is below `Runs`, with at most `Jobs` speculative runs in
  flight; once it is spent, a worker starts only a prefix that is already committed — the
  sequential frontier, which always has a next run until the tree or the budget is exhausted —
  so the plan still finishes exactly the first `Runs` linearizations and never executes more
  than `Runs + Jobs`. Speculation is what parallelism buys on a wide tree; on an adversarial
  one (an early prefix that runs slowly while later ones fan out) it degrades to the sequential
  rate rather than to unbounded work. The outcome table is merged by outcome identity, and the
  witness kept for an outcome is the one with the least choice prefix, not the first to arrive,
  so the table, its witnesses and its `incomplete: runs hit` cut are the sequential ones.
- **`solve`**, **`smt`**: one query, one solver process. The SMT design's sensitivity procedure
  asks three queries whose answers are independent; the induction proof's base and step are
  independent; each is a run.
- **`tool:<name>`**: one invocation, one process.
- **`run`**: one execution; it has no internal parallelism and gains from `Jobs` only under
  `-engine all`, beside other engines.

### Determinism

The result of a plan does not depend on `Jobs`. This is a test, not an aspiration: every engine
that fans out runs `-jobs 1` and `-jobs N` on the conformance corpus and the two `-json` reports
are byte-identical. What makes it hold: results are assembled in plan order; `explore` keeps the
least witness; seeds are fixed in the plan; solver processes are given the same query text and a
solver's `unknown` on one run and `unsat` on another (a timeout hit differently) is the one
admitted source of variation, and it is reported as *not covered* with the timeout, so the
variation is between an answer and an honest absence, never between two answers.

### Stopping early

A witnessed violation answers an existential question, so a plan may cancel the runs that remain
once one is in hand — but *which* runs remain is decided by plan order, not by the clock, or
the determinism above would not hold. `explore`'s witness is the violating linearization with
the least prefix in plan order. A violation found at prefix `p` cancels only the runs whose
prefixes order after `p`; runs ordered before `p` finish, and if one of them also violates, it
becomes the witness and cancels from its own position. The witness is final only when every
prefix before it has completed — and every such prefix is known, because a prefix's ancestors
order before it, so the subtree before `p` is discovered entirely by runs that themselves order
before `p`. The prefixes *after* `p` are not known: the tree is discovered by running it, and
how much of the later tree a plan had uncovered when the witness became final depends on
`Jobs`. So the report does not list cancelled prefixes. It reports the witness, the outcomes of
the linearizations before it, and the cut itself — *stopped at the witness; the linearizations
after it in plan order were not explored* — and discards whatever a worker had already learned
past the cut, whether or not that run had finished. That is the same report under `-jobs 1`
and `-jobs 8`, and it is what the sequential `Explore` would print if it stopped at `p`. The
`Runs` budget is the same kind of cut, and the two compose the same way: a witness at `p` is
final only if `p` lies within the first `Runs` linearizations, and a run past either cut is
discarded without being charged to `Runs`, so a plan under `Jobs=8` finishes exactly the
prefixes before `p` that `Jobs=1` would, however many later runs its workers had started.

A plan does not cancel runs that serve a *universal* claim: a plan whose question is `holds`
under `-engine all` lets `smt` finish even after `explore` has completed, because the two
bounds are different evidence and the user asked for both. A cancelled *engine* is reported as
such, not dropped: the plan names it and the bound it had reached.

## External tools

A `ToolExecution` on an action of an analysis case names a tool and a URI the tool understands;
`ToolVariable` on its parameters names the variables the tool calls them. The `tool:<name>`
engine turns this into one process invocation per performance of the action:

1. The action's `in` parameters are read from the run, converted to the tool variables' names,
   and written to the process as one JSON object on standard input: the `toolName`, the `uri`
   passed through uninterpreted, and `inputs` keyed by `ToolVariable.name` with value and unit.
2. The process writes one JSON object to standard output: `outputs` keyed the same way, or an
   `error` with a message. The engine binds the outputs to the action's `out` parameters,
   converting units to the parameters' declared ones, and the run continues.
3. A non-zero exit, malformed output, a missing output, an output with no `ToolVariable` to
   receive it, or a timeout (`OPENSYSML_TOOL_TIMEOUT`, default the solver's 10 s) is a typed
   error that fails the performance, as a failed calc does. No default value is invented, and
   there is no fallback: `tool:<name>` is the only engine that answers `compute`, so a tool
   failure is never a *not covered* that `auto` advances past; it is the failure of the
   performance, and the enclosing question's result carries it.

Three things the framework asserts about the result and will not let a tool bypass:

- **A tool's answer is *observed*.** There is no referee for a tool: nothing in OpenSysML knows
  what the tool should have computed. Its output stands as the value of that performance, and an
  analysis verdict resting on it is *observed* even if every other step was checked.
- **Under `explore` and `check`, a tool is a black box the interpreter drives**; each
  linearization invokes it, so a non-deterministic tool makes the outcome table non-reproducible
  and the framework says so when two invocations with equal inputs return unequal outputs.
- **Under `smt`, a tool-computed output is a free input in its declared domain.** The checker
  cannot encode the tool, so it over-approximates it: a *proved* result is then a proof for
  *every* value the tool could return, which is stronger than needed and sound; a violation
  found with a tool output the tool does not actually produce fails replay, and is reported as
  *not covered: tool 'ModelCenter' returned 12.4 where the witness needs 15.0*, never as
  *violated*.

The pilot's `AnalysisAnnotation` example, with its `ModelCenter` tool and `deltaT`, `v0` and
`a` variables, is the first fixture, run against a stand-in executable the test suite provides.
The manifest names the executable; the model never does, so the same model runs against the
vendor's tool at one site and a surrogate at another.

## User surface

Existing flags, commands, RPCs and their outputs keep their meaning. What is added:

| Surface | Addition |
|---------|----------|
| CLI | `-engines` lists registered engines with status and authority; `-engine <name>\|auto\|all` selects; `-jobs <n>` sets concurrency; `-json` gains `plan` and one `results[]` entry per engine with `engine`, `strength`, `bounds`, `witness` |
| REPL | `%engines`; `%engine <name>\|auto\|all` for the session; `%jobs <n>` |
| gRPC/Connect | `ListEngines`; an `engine` field on the analysis, verification and sweep requests, unset meaning `auto`; `engine`, `strength` and `bounds` on their responses; the `engines` service capability |
| Environment | `OPENSYSML_JOBS`, `OPENSYSML_TOOLS`, `OPENSYSML_TOOL_TIMEOUT` |
| Report | every verdict line is followed by its standing: `holds (observed: 1 run under reverse)`, `holds (proved over schedules: 6 linearizations, inputs as written)`, `violated (witnessed: replay with -schedule replay:witness.trace)` |

`-schedule explore` remains the way to ask `outcomes` and is equivalent to `-engine explore`;
the SMT design's `-check-engine explore|smt|both` is this note's `-engine explore|smt|all`, and
its `-check-*` bounds become the engine's bounds under the shared `Budget`. Wire additions are
fields on existing messages and one new RPC; nothing is removed or renamed, so under the
versioning rule in `CONTRIBUTING.md` they are patch material. The `-json` additions are new keys
beside the existing ones, with no existing key changed — but the same rule names the shape of a
`-json` report among the changes that make a release minor when existing artifacts fail against
it, and a consumer that validates the report against a closed schema would. The stage that adds
them (stage 4 below) therefore carries that decision to the release checklist rather than
assuming patch; every stage before it changes no output at all.

## Migration

Each plug point becomes an engine behind its existing surface, in an order where every step
leaves the tree green and no user-visible output changes until the surface section above is
reached.

| Today | Becomes | What changes for a user |
|-------|---------|-------------------------|
| One run under `-requirement`, `-constraint`, `-satisfy`, `-analysis`, `-calc`, `-action`, `-state`, `%run`, the verification RPCs | `run` | nothing; the report gains the standing line |
| `-schedule explore`, `%schedule explore`, `schedule: explore` | `explore` | nothing; `-engine explore` is a synonym; `-jobs` runs linearizations in parallel with identical tables |
| `-sweep`, `-samples`, `%sweep`, `%samples`, `RunSweep` | `sweep` | nothing; rows run in parallel under `-jobs`; table order unchanged |
| `%check`, `%explain`, `%solve`, `%configure`, `%optimize`, `OPENSYSML_SMT` | `solve` | nothing; `solve.Discover` becomes the engine's status in `-engines` |
| Proposed `-check-engine smt` and `-check-*` | `smt` | the flags are `-engine smt` and the shared bounds |
| Proposed `-check-action`, `-check-state`, `-check-diverge` | `check` | selected by `-engine check` |
| `ToolExecution` and `ToolVariable`, parsed and unread | `tool:<name>` | new: analysis cases that name a registered tool run it |
| `Session.mu` held for a whole REPL command | a command lock held for the command and a state lock released while a plan runs on contexts of its own | completion and the session's getters answer during a long exploration, and a `%stop`-style interruption becomes possible; a second command still waits |
| `runtimeSemantics.mu` in the gRPC service | one resolver and semantic model per worker, a worker per request | concurrent requests on one model no longer serialize |

## Test contract

- **Registry:** duplicate registration is a typed error; `Engines()` order is name order; two
  registries in one process do not see each other; an engine whose process is absent lists with
  that status and refuses through `Covers` with a typed error.
- **Dispatch:** `auto` picks the strongest covering engine and names the fallback and the
  refusal in the plan; `auto` also advances past a run-time *not covered* (a solver `unknown`, a
  failed replay) and keeps that result in the plan; an `error` from `Run` stops the plan;
  `-engine <name>` on a refusing engine stops with the refusal; `all` runs every covering
  engine.
- **The strength scale:** a table-driven test over every (`Claim`, `Strength`) pair each engine
  may produce; a test that no path promotes *observed* to *bounded* or *bounded* to *proved*; a
  test that a budget reached lowers the strength and prints the bound; a test that a witness that
  fails replay is *not covered* with the disagreement, never *violated*; a test that a
  contradiction between `smt` and `explore` is a disagreement in the interpreter's favor.
- **Existing behavior through the framework:** every conformance case, golden trace, sweep
  golden and REPL/CLI/gRPC golden passes unchanged with the engines behind the surfaces, before
  any surface addition lands.
- **Determinism under `-jobs`:** for `explore` and `sweep` over the conformance corpus,
  `-jobs 1` and `-jobs 8` produce byte-identical `-json` reports, including the witness, the
  outcome table and the cut of a violating case whose later prefix violates faster than its
  earlier one and whose later subtree is wider than the earlier; the same case again with
  `runs` set just above the witness's position, so that speculative runs past the cut would
  exhaust a counted budget; and a case whose first prefix is slow while its siblings fan out
  wide, asserting the physical execution count never exceeds `runs + jobs`; run under `-race`.
- **Isolation:** two plans on one model in one process on two goroutines, under `-race`, with
  the resolver and semantic model per worker; the gRPC service serves concurrent runtime requests
  on one model.
- **Cancellation:** a deadline already past fails a plan before its first engine is consulted,
  whether it would have run or refused; a deadline met mid-plan stops it, the step that met it
  carrying `context.DeadlineExceeded`, an answer arriving after it dropped; a budget without one
  leaves the caller's context as it is. Under `all`, the coordinator returns
  every finished engine's result, every cancelled engine marked as such with the bound it
  reached, and the composed result at the strength the finished runs earned.
- **Tools:** the `AnalysisAnnotation` fixture against a stand-in executable, with the
  input/output protocol, a missing output, a non-zero exit, a timeout, an unregistered
  `toolName`, and a non-deterministic stand-in, each producing its typed error or its *observed*
  result and never a fabricated value.
- **Wire, CLI and REPL compatibility:** `make proto-breaking` against `develop`, `make
  man-check` and the REPL and CLI goldens pass; every existing field and flag keeps its
  meaning.

## Stages

Each stage is a pull request into `develop` that leaves the gate green and the user-visible
behavior unchanged until stage 4.

1. **Contract and registry.** `Question`, `Engine`, `Result`, `Strength`, `Budget`; the
   registry; `run`, `explore`, `sweep` and `solve` as adapters over existing code; every
   existing surface routed through `auto`. No output changes. *Implemented:*
   `internal/core/analysis`, with the registry and dispatch tests of the test contract, the
   `auto` clauses of its dispatch bullet, and the existing goldens passing through the engines.
   The `Strength` and `Claim` orderings are in place; the strength-scale tests proper, `all` and
   the disagreement result belong to stage 4.
2. **Isolation.** Resolver and semantic model per worker over the shared frozen index; the
   gRPC service and the REPL session release their locks while a plan runs; a context per run
   that takes the budget's `Steps` and `Memory` at construction, with the mapping of `Steps`
   onto the runtime's step kin; the `-race` isolation tests. *Implemented:* `analysis.Worker`
   and `Model{Context, Semantics, Fresh}` with `Model.NewContext` taking `Steps` as `MaxSteps`
   and `Memory` as `MaxElements` (the mapping above), `Result.Workers` and `Result.Warming`
   carried and not printed; `run` and `explore` build their contexts through the worker, and
   `Explore`'s `fresh` is `NewContext`. The gRPC service builds a worker per request; the REPL
   session's lock is split as described under *What may be shared*, and an exploration releases
   the state lock while it runs. Tests: two plans on one model on two goroutines under `-race`,
   concurrent gRPC requests on one cached model, completion beside an exploration, and
   budget-at-construction with a zero field the context's own for `run` and `explore`. As it
   applies: `sweep` builds no context — a row is the surface's closure over its own context
   (`SweepRun` takes none), so the engine refuses a model that holds none with the typed
   `NoRuntimeError` and its `Runs` bound is the row limit; `solve` builds none — the solver
   queries the semantic model. Both stay one worker per plan: a plan's runs are sequential
   until stage 3 puts several workers on them.
3. **Parallel runs.** `-jobs`/`OPENSYSML_JOBS`; `sweep` rows and `explore` prefixes on the
   work queue; the determinism tests. *Implemented:* `Budget.Jobs` filled by `BudgetOf` from
   `-jobs`, `%jobs` and `OPENSYSML_JOBS` (`analysis.ParseJobs`, `JobsFromEnv`, default
   `NumCPU`; a count below one or no integer is the typed `JobsError` before anything runs);
   the gRPC service takes the serving binary's jobs, no request field. `Model` holds `Jobs`
   worker slots per plan, built lazily by job index (`Model.WorkerAt`, `Model.NewContextOn`;
   `NewContext` is job 0), `Result.Workers` counting the ones a plan built and `Warming` their
   summed construction. `runtime.ExploreWith` is the work queue of prefixes described under
   *Units of work*, with `Explore` its one-job form: prefixes ordered as the sequential
   exploration takes them, committed and speculative runs, at most `Jobs` speculative runs
   discarded in a plan's lifetime, never more than `Runs + Jobs` executions and no more than
   `Runs` jobs put to work (the queue never holds more prefixes), the table merged by outcome
   identity with the least witness; a run's context is let go once its prefix is folded or
   dropped, so the queue holds one per job in flight besides the witnesses' the result keeps,
   and `Memory` times `Jobs` is indeed the fleet. The witness cut of *Stopping early* is not
   applied: `explore` answers `outcomes` alone, a universal question whose answer is every
   linearization's outcome, so a violating run is an outcome of the table as under one job and
   cancels nothing; the cut is the mechanism the `Runs` cut already is (`insert` drops what moves
   past it, a started run among it discarded), for the existential questions a later stage puts
   on the queue. `all` runs its covering engines concurrently on `min(Jobs, engines)` goroutines
   with the jobs divided among them, `Compose`
   unchanged over the set of results in name order; a fault or deadline cancels the engines
   after it in name order, each kept as a step marked cancelled with the bound it reached, and a
   universal run is not cancelled by a witness. The `-json` `plan` key gains `workers` and
   `warming`; the human-readable report prints neither (see *What may be shared*). Tests: the
   determinism bullet over the conformance corpus and its three fixtures (the violation a later,
   wider prefix reaches faster, under `internal/core/runtime/testdata/` because the harness
   admits no erroring outcome; `runs` just above its witness; the slow first prefix beside wide
   siblings, a conformance case whose slow body is a bounded recursion) on one job against
   eight, in the runtime and through the CLI's `-json`, with the physical count bounded by
   `runs + jobs`; the cancellation bullet under concurrent `all`; the isolation bullet with
   `Jobs` workers per plan; `make man-check`. **Known limitation:** `sweep` rows stay
   sequential. Stage 2 fixed a row as the surface's closure over its own context (`SweepRun`
   takes none), and every surface runs its rows in the one context it holds and reads their
   outputs back through it; putting rows on the queue needs `SweepRun` to take a context per
   row, the surfaces to re-instantiate the subject and arguments per row, and a rule for `%sweep`
   on held objects, none of which this note yet fixes. The sweep determinism test holds as the
   table is assembled in plan order regardless.
4. **Surface.** `-engines`, `-engine`, `%engines`, `ListEngines`, the response fields, the
   standing line on every verdict; the strength-scale tests; `all` and the disagreement result.
   The `-json` additions land here, and its release checklist records whether they are patch or
   minor under the versioning rule. *Implemented:* `Selection` and `Registry.AnswerWith` in
   `internal/core/analysis`, with `all` running the covering engines one after another in name
   order and `Compose` deciding the composed result, the demotion and the disagreement over the
   set of results; a cancelled engine kept in the plan as a step marked cancelled with the
   bound it reached; `Result.Standing` and `Plan.Standing` for the standing line the REPL, the
   CLI report and `-json` print after every verdict; `-engines`, `-engine`, `%engines`,
   `%engine`, `ListEngines`, the `engine` request field and the `engine`, `strength` and
   `bounds` response fields with the `engines` capability; the `-json` `plan` and `results[]`
   keys; the strength-scale, dispatch and cancellation tests of the test contract, the
   disagreement test over a test engine claiming *proved* in the shape `smt` will fill. The
   patch-or-minor question the `-json` keys raise is an item of the release checklist in
   `CONTRIBUTING.md`, undecided here; no version was bumped. Running `all` concurrently, `-jobs`
   and `%jobs` remain with the parallel-runs stage.
5. **Tools.** The manifest, the `tool:<name>` engine and its protocol, the `AnalysisAnnotation`
   fixture and the stand-in.
6. **The model checkers register.** `smt` and `check` land by their own notes' stages, each as
   an engine from its first stage, with `all` as their referee harness.

## What this does not change

- The interpreter is normative. An engine is a way of asking about executions the interpreter
  defines; a disagreement is resolved in the interpreter's favor and recorded as a bug in the
  other engine or, if replay shows the executor wrong, in the executor.
- Every per-run limit (`OPENSYSML_MAX_STEPS`, `OPENSYSML_MAX_ACTION_STEPS`,
  `OPENSYSML_MAX_SWEEP_RUNS`, `OPENSYSML_SMT_TIMEOUT`, `OPENSYSML_MAX_ELEMENTS`, …) keeps its
  name and its meaning; the framework adds `Jobs` and a plan deadline beside them.
- No accepted model is refused, no result changes, no flag, command, RPC or field is removed or
  renamed. Default traces, conformance expectations and golden files are unchanged through
  stage 3; stage 4 adds a standing line to reports and fields to `-json` and the wire.
- The two model-checking designs stand as written. This note replaces their user-surface
  paragraphs' engine-selection flags with `-engine` and makes their referee and fallback the
  framework's; their encodings, verdicts, bounds and stages are theirs.

## Alternatives considered

- **Make the resolver and semantic model safe for concurrent use** instead of one per worker.
  Both are lazy and recursive — a resolution triggers resolutions — so a lock per map deadlocks
  or serializes, and a lock-free memo table with recursive fills is a research project. One per
  worker is simple, measurable and correct; if the warm-up cost matters on large models, the
  memoized tables can be snapshotted after a warm sequential pass and cloned, which is an
  optimization within this design, not a change to it.
- **Load engines as Go plugins.** Rejected above: platform-limited, toolchain-locked, and an
  ABI the wire contract would then have to protect.
- **Engines as gRPC services.** Attractive for remote simulators and a natural second transport
  for the tool protocol; deferred because the first external engines are local executables and
  the standard-input protocol carries the same JSON a service would.
- **One `verify` command that runs everything.** The composed result would be one line the
  reader could not take apart; the plan and one result per engine keep each engine's standing
  visible, which is what a safety case cites.
- **A race: the first engine to answer wins.** The fastest engine is always `run`, and its
  answer is always *observed*. The strength scale exists so that speed is not standing.
