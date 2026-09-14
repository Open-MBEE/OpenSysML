# SMT bounded model checking of behaviors

A design for answering the safety-case question — *does any admissible execution violate this
requirement?* — with an SMT solver rather than by enumerating executions: the token-flow
semantics of an action are unrolled to a bounded number of moves and written as a satisfiability
query over the schedule, the feature values and the unbound inputs at once, so one solver call
searches every interleaving and every input value the model leaves open. Nothing here is
implemented. The note fixes what is encoded, what a verdict may claim, how a solver's answer is
turned into a witness the interpreter replays, and where the encoding stops and says so, so the
work can be reviewed before code is written.

It is the next stage of the model-checking track. The
[explicit-state design](bounded-model-checking.md) explores the executor itself, and the
`explore` scheduling policy that landed from it
([Exploring every linearization](../../reference/cli.md#exploring-every-linearization)) is the
concrete referee this design is checked against. The SMT layer (`internal/core/solve`, behind
the REPL's [`%check`, `%explain`, `%solve`, `%configure` and
`%optimize`](../../reference/repl-commands.md)) translates the conditions of constraints and
requirements and runs an external solver over them; this design gives that layer a notion of a
behavior's state.

## The problem this answers

`explore` closes part of the multiple-valid-execution problem: for a model whose inputs are
concrete and whose interleavings fit in a run budget, it visits every linearization at body
granularity and tables the distinct outcomes, so "the library leaves `x = 1` or `x = 2` open" is
found rather than assumed. Three things it structurally cannot do:

- **Scale.** Every linearization is a run. `n` concurrent branches of `k` nodes each have
  `(nk)!/(k!)^n` linearizations; `action_merge_fork_branch_and_loop`, a small conformance case,
  needs `explore:runs=10000` to complete, and a fork of five branches of ten nodes has more
  linearizations than any budget. Partial-order reduction (the explicit note's stage 2) reduces
  this by the number of *equivalent* orders, which is large for well-synchronized models and
  small for exactly the under-synchronized models a safety case is worried about.
- **Inputs.** A run holds the values the model or the caller fixed. "No schedule violates `R`"
  from `explore` means "no schedule violates `R` *for these values*"; the `in` parameter that
  was never bound, the attribute with no default, the range an interface promises, are not
  covered.
- **Sensitivity as a question.** `explore` reports the outcomes it reached; whether a feature's
  final value *depends on the schedule*, and which two moves decide it, is read off the table by
  a person.

A satisfiability query treats the schedule and the inputs as unknowns and lets the solver search
both. Its search is not an enumeration — learned clauses prune whole families of orders the
way partial-order reduction does, without a footprint analysis being written — and an
unsatisfiable query is a proof about every schedule and every input at once, within a stated
bound. That is the sentence a safety case wants to cite, and it is the sentence this design lets
the tool print, with its bound attached.

## What exists to build on

`internal/core/solve` already does the following, and this design reuses each rather than
writing a second one:

- **A term IR and an SMT-LIB2 writer.** Conditions become `Term`s over sorts `Bool`, `Int`,
  `Real`, `String` and finite datatypes (enumerations and variation points), with the narrowest
  SMT-LIB logic that covers the query, and are written as a script.
- **An external solver as a process.** z3 or cvc5 on `PATH` or named by `OPENSYSML_SMT`,
  speaking SMT-LIB2 on standard input; nothing is linked in. `sat`, `unsat` and `unknown` are
  kept distinct, a timeout is `unknown`, and a solver that is absent, crashes or answers
  unusably is a typed error rather than a verdict.
- **A capability model.** What a query needs of a backend (models, unsat cores, incremental
  checks, datatypes, strings, nonlinear arithmetic, the non-standard `ALL` logic, optimization)
  is probed once per executable and a backend that refuses one is an
  `UnsupportedCapabilityError` before any query runs, never a degraded question.
- **A translatable subset with a hard boundary.** Boolean operators, equality, comparisons,
  `+ - * / %` with the evaluator's truncating integer semantics, literals, quantities normalized
  to base units as exact rationals, scalar features and feature chains grounding in one,
  enumeration literals and variants. Everything else — collections and quantifiers, invocations
  (calc usages included), `**`, real `%`, classification operators, string operations beyond
  equality — refuses with `ErrNotTranslatable`, and one refused conjunct refuses the whole query.
- **Pins.** A partial assignment fixing some features to the values an object holds or the
  model declares, the rest free; unsat under pins says the fixed values conflict, and the unsat
  core says which.
- **Models rendered in the model's terms**, one witness among many, with `Query.Rounded`
  flagging a query whose evaluator computes in `float64` where the encoding is exact.
- **A differential agreement gate.** For every element whose conditions translate, and every
  concrete assignment the corpora provide, the query conjoined with the assignment is `sat`
  exactly when the evaluator says the conditions hold. The translation is evidence-backed against
  the evaluator, which stays normative.

What it has no notion of: **state**. A variable stands for the value a feature may take, with no
"before this move" or "after that one"; there is no token, no node, no succession, no clock and
no message. This design adds exactly that — a bounded transition relation over the lowered
`ActionGraph` — on top of the term IR, the writer, the solver process and the verdict
discipline the layer already has.

## The encoding

### The unit and the bound

The unit is the one `explore` takes: **one move is one token advancing one node** — its body run
to completion, its enabled successions followed, and stop. Statements inside a body are not
interleaved with another token's, so both engines describe the same state space and a witness
from one is a schedule of the other. The explicit note's argument for this granularity
(one performance, `HappensBefore` between whole occurrences, the coarse reading being the
executor's) applies unchanged.

The query is over **`k` moves**. State `s_0` is the initial state (the token at `Initial`, the
attribute defaults from `ActionGraph.Attributes`, the pinned and free inputs); `s_i` is the
state after move `i`. Every state component is a family of solver variables indexed by `i` —
the static-single-assignment shape a bounded model checker usually takes — and the transition
relation `T(s_i, c_i, s_{i+1})` says what the move chosen by `c_i` does. `k` is a bound, and the
verdict names it.

### The state

| Component | In the executor | In the query |
|-----------|-----------------|--------------|
| Tokens | `ActionExecutor.tokens`, ids assigned on spawn | `T` **token slots**, `T` computed from the graph: the widest set of tokens `k` moves can hold, one per fork branch reachable within the bound, plus one per nested flow. `at_i[t]` is a finite datatype over the graph's nodes with `Absent`, `Done` and `Parked` |
| Node performances | `stepToken`, `stepJoinNode`, `stepMergeNode` | `joined_i[j]` — how many of join `j`'s incoming successions have delivered. A merge keeps no state: it is one performance per arrival, so a token at a merge is described by its slot alone |
| Feature values | `Instance.FeatureValues`, `actionFrame.data` | one variable per scalar feature per state, `x_i`, in the sort the translator already gives the feature; a frame-local feature is one per (node performance, feature) |
| Pins and object flows | `PinBinding`, `DataFlows`, delivered `out` pins | the value carried on an object flow is a variable set by the source node's move and read by the target's |
| Messages | `Context.messages`, oldest first | a bounded bus of `M` slots, each `(present, signal type, payload, posted-at)`, `M` computed as the number of `Send` statements reachable in `k` moves |
| Clock | `Context.Clock()`, `accept after`/`at` | `now_i : Real` (seconds) and a due time per parked token; `now` never decreases and advances only when no token is enabled, to the earliest due time — time is not a choice, only ties are, as the explicit note says. There is no horizon: `k` moves alone bound the run. The library's `Clocks` and `Occurrences`, and `accept after`/`accept at` over them, fix *when* a timed accept becomes enabled and say nothing about how far a run proceeds; fUML, PSCS and PSSM define no clock at all. A horizon would be a bound the specification does not name, so the rule that adds nothing is the one taken, and `-advance` stays refused for `smt` as `check` refuses it |
| Paused nested flows | `Token.body`, `Token.resumable` | a nested flow's tokens are slots of their own, and the performing node is `Done` only when its `Finals` are reached (`Subflows`) |

What is *not* in the state — the memo tables, the lowered graph, the symbol tables — is not in
the query either: it is a function of the model, and the encoding reads it while being built.

### The enabled set and the choice

`c_i` is the choice variable of move `i`, a finite datatype over the token slots plus `Stutter`.
The constraint that the choice is admissible is the solver's copy of the executor's rules, one
per rule, each cited to what the executor does:

| Executor rule | Constraint on `en_i(t)` |
|---------------|-------------------------|
| A token at a plain node can act | `at_i[t] = n ∧ plain(n) ⇒ en_i(t)` |
| A token at a join acts when the join has collected (`stepJoinNode`) | `at_i[t] = j ⇒ (en_i(t) ⇔ joined_i[j] = indegree(j))` |
| A merge performs once per arriving token, a loop re-entering it and a fork's branches each traversing it (`stepMergeNode`) | `at_i[t] = m ⇒ en_i(t)`, for every slot at `m` |
| A parked accept acts when a matching message is on the bus (`acceptMatch`) | `at_i[t] = Parked ∧ accept(t) = a ⇒ (en_i(t) ⇔ ∃ slot: present ∧ type matches a)` |
| A parked `accept after`/`at` acts when due (`triggerHolds`) | `en_i(t) ⇔ now_i ≥ due_i[t]` |
| A paused nested flow resumes when it would go on (`Token.resumable`) | the nested slots' own enabledness; the performing node has no move of its own |
| An absent or done slot never acts | `at_i[t] ∈ {Absent, Done} ⇒ ¬en_i(t)` |

Then `c_i = t ⇒ en_i(t)`, and `c_i = Stutter ⇔ ¬∃t. en_i(t)`. A stutter with a token still
`Parked` or at an uncollected join, and no token elsewhere, is the deadlock state the executor
reports as `ErrAcceptDeadlock`/`ErrActionDeadlock`; a stutter with every slot `Absent` or `Done`
is completion. Once the run stutters it stutters to `k`, so a query at bound `k` covers every
schedule of *at most* `k` moves.

### The effect of a move

`T` for `c_i = t` at node `n`:

1. **Body.** `Bodies[n]` in order. An `Assign` of a scalar feature is `x_{i+1} = value(x_i, …)`
   through the existing expression translator, whose subset is the subset here; a `Declare`
   introduces a frame-local variable; an `If` is `ite`; a `Loop` is unrolled to a per-loop bound
   `L` with a flag that records the bound was hit on this path (see [Bounds](#bounds)); a `Send`
   sets the first free bus slot; a `Return` and an `Effect` follow the executor's rule for the
   statement; an `Unsupported` statement, or a statement whose expression refuses translation,
   makes the node **not encodable** and the query is refused for the behavior, naming the node
   and the construct.
2. **Successions.** `Edges[n]` with their guards, translated as Booleans over `s_{i+1}`. A plain
   node follows every guard that holds; a decision follows exactly one. The library fixes one
   (`DecisionPerformance::outgoingHBLink [1]`) but not which, and the executor reads every
   guard and resolves two holding ones as a *decision branch* choice point
   (`action_choice_decision_overlapping_guards` lists both outcomes), so the encoding follows the
   executor: the branch taken is part of the move's choice, and each holding guard is a schedule
   the solver ranges over rather than a defect it asserts away. A decision none of whose guards
   holds is the executor's error at that node, and the move is a failure.
3. **Placement.** The token moves to its single successor; a fork sets its branch slots to their
   first nodes; arriving at a join increments `joined[j]` and the arriving slot becomes `Absent`
   unless it is the one that collects; arriving at a merge is arriving at a plain node, and two
   slots may sit at the same merge; arriving at a final sets `Done`; an `accept` sets `Parked`
   with its due time or signal.
4. **Frame.** Every variable the move does not write is equal to its previous value. This is the
   bulk of the script and the reason `T` is built per node from the graph rather than by hand.

The statement kinds are the closed `Statement` set the lowering layer owns, so a kind this list
does not cover cannot appear; adding a kind to `lower` adds a row here or a refusal, never
silence.

### Inputs

A feature the model binds — a default, a value the performing object holds, a value the caller
fixed — is asserted equal to it in `s_0`, through the same `Pins` machinery `%solve` uses. A
feature the model leaves unbound is **free**: an `in` parameter with no argument, an attribute
with no default, a feature the user names as an input (`-check-input inletTemp`). Its
domain is its sort, narrowed by the declared type (`Natural ≥ 0`, an enumeration's constructors,
a `Real` in a quantity type) and by any constraint on the performing object the user asks to
assume (`-check-assume Vehicle::EnvelopeLimits`, translated like any condition and asserted over
`s_0`). A free feature whose multiplicity admits no value (`x : Integer[0..1]`) may also be
absent, and the state the interpreter reaches with it absent — the feature reads as the empty
sequence, which fails a comparison — is among the states the query ranges over: its presence
is a variable of `s_0` too, and a condition reading it while absent is undefined there, as a
read of a feature no move has written is. Everything about `x` that the model says is in the
query; nothing the model does not say is.

### Properties

The query asserts the *negation* of the property over the reached states, so `sat` is a
counterexample and `unsat` is the proof:

- **Requirement or constraint** `R`, named as the CLI names them today (`-requirement`,
  `-constraint`, `-satisfy`): `∃ i ≤ k. ¬R(s_i)`, with `R` translated by the existing translator
  over the state-`i` copies of the features it reads. A property is evaluated only at move
  boundaries, so a transient value inside one body is never a false violation, as the explicit
  note stipulates.
- **Deadlock**: `∃ i. c_i = Stutter ∧ ¬complete(s_i)`.
- **Typed runtime error** a body would raise: division by a computed zero is already a
  `RoleDefined` side condition in the translator, and its violation is `∃ i. divisor_i = 0` on a
  path that evaluates it; an out-of-range enumeration or a decision with no holding guard are
  encoded the same way.
- **Schedule sensitivity** of a feature `f`, with no requirement written: two copies of the
  unrolled system, `A` and `B`, sharing `s_0` and every input variable, both complete, with
  `f^A_k ≠ f^B_k`. `sat` yields two schedules from one initial state that end with different
  values of `f`; the earliest move at which `c^A` and `c^B` differ, and the two moves they take,
  are the pair the report names. `unsat` says `f`'s final value is a function of the inputs
  alone — **provided every schedule completes within `k`**. The `both complete` conjunct makes
  the query vacuous otherwise: at a `k` too short for any run to finish, no pair of completed
  copies exists and `unsat` says nothing, and the same holds when some schedules never complete
  at all. So `unsat` is reported as *not sensitive* only when a second query,
  `∃ schedule. ¬complete(s_k) ∨ loopflag` — some run is still live at `k`, deadlocked, ended in a
  runtime error, or hit a body-loop bound — is itself `unsat`, which is the statement that every
  schedule completes within `k`. A `sat` model is one schedule of the solver's choosing, so the
  disjuncts are not classified from it; they are asked in order, each on its own. The deadlock
  and typed-error properties above run first, and a `sat` on either is that *violated* finding,
  with `f` having no final value on the witness. Only when both are `unsat` is the remaining
  query, `∃ schedule. cut_k ∨ loopflag`, asked; `sat` there is *no sensitivity found within k
  moves*, named as the bounded result it is, with the still-live schedule or the cut loop printed
  as the reason. Only *not sensitive* closes the compliance map's *approximate* row for a given
  model, and not for the runtime in general.

One query per property, or several properties in one query with a labelled disjunct each, so the
model says which failed.

## Verdicts and what they may claim

A solver answer is never the verdict; the verdict is the answer *qualified* by everything the
query assumed, and the report prints the qualification with it. The four verdicts:

| Verdict | Solver | Claim | What is printed with it |
|---------|--------|-------|-------------------------|
| **Proved** | `unsat` | No schedule of at most `k` moves, for any value of the free inputs in their declared domains, violates `R` | `k`; the loop bounds `L` and whether any unroll flag could be set; the free inputs and their domains; the token and bus bounds; `over exact arithmetic` when `Query.Rounded` holds |
| **Violated** | `sat`, and the witness **replayed** | A concrete schedule and concrete inputs violate `R` | the schedule in the trace format below; the inputs the solver chose; the state at the violating move; the replay command |
| **Sensitive** | `sat` on the two-copy query, both witnesses replayed | `f`'s final value depends on the order of two named moves | both schedules, the diverging move, the two final values |
| **Not covered** | `unknown`, a refusal, or a witness that does not replay | Nothing | the reason: the construct that refused translation and the node it is in; the solver's `unknown` and its timeout; the bound whose unroll flag the proof needed; the replay disagreement |

Three rules make these honest:

1. **`unknown` is not `unsat`.** A solver that gives up — nonlinear arithmetic, a timeout — has
   established nothing. The verdict is *not covered*, and the report says which query and why.
   `internal/core/solve` already keeps the three answers distinct; this design keeps them
   distinct in the user's terms.
2. **A refusal is not a proof.** A body the encoding cannot express refuses the whole behavior
   before any query runs, as one untranslatable conjunct refuses a condition today. No partial
   query is ever sent, so no proof is ever about a smaller system than the model.
3. **A witness is a claim about the interpreter, checked against it.** Every `sat` model is
   decoded to a schedule and an input assignment and **replayed** through the executor under a
   replay scheduling policy that follows the schedule move for move. The verdict is *Violated*
   only when the replay reaches the state the solver described and `R` evaluates `false` there
   in the evaluator. A witness the interpreter does not reproduce is a bug in the encoding, or
   in the executor; either way the verdict is *not covered* with the disagreement printed, and
   the case goes into the referee corpus below. The engineer is never asked to trust the solver.

### The report

Every line is meant to be pasted into a hazard analysis or a review comment as it stands. Model
terms only — names, values, moves; no SMT-LIB, no `sat`/`unsat`, no run counts as a headline.

```
✓ proved Plant::MaxPressure over Plant::Reactor::regulate (reactor)
  every schedule of at most 40 moves, 3 concurrent branches, 7 token slots;
  inletTemp : Real free in [250.0, 400.0] [K] (assumed Plant::EnvelopeLimits);
  loop 'retry' unrolled 4 times, never cut; over exact arithmetic (the evaluator rounds)
```

```
✗ violated Plant::MaxPressure over Plant::Reactor::regulate (reactor)
  inputs: inletTemp = 399.5 [K], valveOpen = false
  schedule: step 3: 2@heat first of 2@heat, 3@vent; step 4: 2@raise first of 2@raise, 3@vent;
            step 5: 3@vent first of 3@vent
  at step 5: pressure = 12.4 [bar] > 12.0 [bar]
  replay: sysml -action "Plant::Reactor::regulate reactor" -schedule replay:regulate-violation.trace \
                -requirement Plant::MaxPressure -trace
```

```
! sensitive x in test::wake
  x ends 1 or 2 depending on which of writeOne, writeTwo runs last; no succession orders them
  x = 1: step 3: 2@performed first of 2@performed, 3@direct; step 4: 2@writeOne first of 2@writeOne, 3@direct; …
  x = 2: step 3: 3@direct first of 2@performed, 3@direct; step 4: 3@writeTwo first of 2@performed, 3@writeTwo; …
```

```
? not covered Plant::Settled over Plant::Reactor::regulate (reactor)
  node 'estimate' calls RealFunctions::sqrt, which the encoding does not translate;
  explore reached 214 linearizations of these inputs without a violation (runs budget 1024 not hit)
```

The schedule lines are the `choice` lines `explore` already prints, so a witness reads beside an
exploration table and against the
[oracle](../../project/behavior-semantic-oracle.md) the same way; with `-trace` the replay prints
the full trace of the witness run. The *not covered* line ends with what `explore` could say
about the same question on concrete inputs, since existence of a violation is still checkable
when its absence is not.

## The referee

The encoding is a second statement of the executor's semantics, in logic. Anywhere the two
differ, the solver proves something about a system that is not OpenSysML, and no one would
notice from the solver's side. The defense is the same one the SMT layer already uses for
conditions — differential agreement against the evaluator — applied to behaviors, with `explore`
as the evaluator's side:

1. **Outcome sets agree.** Every conformance case that lists `outcomes` is encoded and its
   distinct final states are enumerated (one `check-sat` per outcome, asserting the negation of
   the previous, the way `Configurations` enumerates variants); the set must equal the set
   `explore` tables, which the harness already requires to be exact and complete within each
   case's budget. A case whose body refuses
   translation is recorded as *refused*, not skipped silently, so the coverage of the corpus is a
   number that is reviewed.
2. **Witnesses replay.** Every `sat` model over the corpus, decoded and replayed, reaches the
   state the model describes. A replay that diverges fails the gate.
3. **Proofs agree with exhaustion.** For every case and every requirement the case names, an
   `unsat` at bound `k` implies `explore` (complete, with depth ≥ `k`) found no violation; and an
   `explore` violation implies `sat` at that depth. Disagreement in either direction fails.
4. **Randomized models.** The randomized differential gate the SMT layer runs over conditions is
   extended to generate small action graphs — forks, joins, a merge-loop with a bound,
   guarded decisions, a send/accept pair — and applies checks 1–3 to each.

The interpreter is normative where the two differ, with one exception the explicit note also
makes: where the executor is wrong against the library — the known failures the oracle records
— the encoding follows the executor, so that the gate is a check of faithfulness and not a
second reading of the library. Fixing those is upstream of this work.

## Coverage

What each construct gets, by which engine, and what is reported where it gets nothing. *SMT*
is this design; *explore* is the explicit engine on concrete inputs; a row with both means the
solver answers and `explore` confirms.

| Construct | SMT | explore | If not covered |
|-----------|-----|---------|----------------|
| Successions, fork, join, decision with translatable guards | encoded | yes | — |
| Merge, merge-loop | encoded; a back edge is a succession like any other, so a merge-loop's iterations are bounded by `k`, not unrolled | yes, to `depth` | a loop still live at `k` sets `cut_k`: *no violation within k moves*, not *proved* — unless k-induction closes it |
| Body `assign` of Boolean, Integer, Natural, Rational, Real, enumeration, variation features; `if`; local `Declare` | encoded through the existing translator | yes | — |
| Quantities with units | encoded, normalized to base units as exact rationals | yes | proof is qualified *over exact arithmetic* when the evaluator rounds |
| Feature chains grounding in a scalar of the performing object | encoded | yes | — |
| Object-valued pins, chains through an object a pin carries, `new` | not encoded: no heap | yes | *not covered: dynamic target at node* — a bounded heap of `N` objects per type is a later stage |
| Collections, `->` operators, indexing, ranges | not encoded | yes | *not covered: collection at node* — bounded expansion is a later stage |
| Calc invocations in a body or guard | not encoded: the translator refuses invocations | yes | *not covered: invocation of <calc>* — inlining a pure, loop-free calc is a later stage |
| `**`, `^`, real `%`, `RealFunctions` | not encoded | yes | *not covered: operation* |
| Products and quotients of two computed values | encoded as nonlinear; the solver may answer `unknown` | yes | *not covered: solver undecided* |
| Strings beyond equality | not encoded | yes | *not covered* |
| `send`/`accept` within the checked behavior | encoded over a bus of `M` slots | yes | a bus that fills is a bound, reported |
| `accept after`/`accept at`, one clock | encoded: `now`, due times, ties as choices; no horizon, `k` moves alone bound the run | yes, with or without `-advance` | — |
| Performed actions with their own flow, paused and resumed | encoded as nested slots | yes | — |
| Requirement, constraint, `satisfy` over scalars | encoded as the negated property | yes, at every state | a condition the translator refuses: *not covered: condition* |
| Deadlock | encoded as a stutter short of completion | yes | — |
| Schedule sensitivity of a feature | encoded as the two-copy query | read off the outcome table | — |
| Typed runtime errors (division by zero, no guard holds) | encoded as side conditions | yes, as an error outcome | — |
| Unbound inputs | **free variables in their declared domain** | fixed at the caller's value | — |
| State machines, orthogonal regions, `do` behaviors, deferred events | not in this design | explicit note stage 3 | *not covered: state machine* |
| Time triggers and change events of a state machine | not in this design | explicit note stage 3 | *not covered* |
| Signals to other objects' running machines | not in this design | explicit note stage 5 | *not covered: across objects* |
| Liveness (`done` is eventually reached) | not a safety property; only deadlock within `k` | not asked | *not covered: liveness* — needs a cycle detection or a separate encoding, its own note |
| Interruptible regions, expansion regions, streaming pins, and the other executor gaps | not executed by the runtime | not executed | the runtime's own typed refusal |

Every *not covered* row is a reported reason, never a pass; and every one has `explore` as the
engine that can still show a violation exists on concrete inputs, which the report says.

## Bounds

The query is exact for what it encodes and bounded in five places. Each is a parameter, each is
printed with a *proved* verdict, and none is ever silently exceeded:

| Bound | Default | Flag | What happens at it |
|-------|---------|------|--------------------|
| Moves `k` | 40 | `-check-depth N` (the explicit engine's depth flag; the two bounds mean the same) | A schedule longer than `k` is not covered. A schedule that has not completed or stuttered by move `k` sets `cut_k`; a *proved* verdict with `cut_k` satisfiable is downgraded to *no violation within k moves*, distinctly named |
| Loop unrolling `L` | 4 per body `Loop` statement | `-check-unroll N` | A body `Loop` (`while`/`for` inside one node, run within one move) is unrolled `L` times; an iteration past `L` sets the loop's flag; same downgrade. Loops through the graph (a merge back edge) are not unrolled: they are moves, bounded by `k` |
| Token slots `T`, bus slots `M` | computed from the graph and `k` | — | Never hit by construction; reported for the record |
| Heap `N` (later stage) | — | `-check-objects N` | An allocation past `N` refuses the query |
| Solver time | the SMT layer's `OPENSYSML_SMT_TIMEOUT` (10 s) | `-check-timeout D` | `unknown`, reported as *not covered: solver undecided within D* |

Two verdicts therefore sit under *proved*: `proved` when no bound flag can be set within the
query (the solver shows `¬cut_k ∧ ¬loopflag` is implied), and `no violation within k moves`
when one can. Only the first is a proof, and it is a proof relative to the atomicity rule, the
translated subset and the input domains stated with it. The sensitivity query has the same
pair — *not sensitive* and *no sensitivity found within k moves* — decided by the same flags.

### From a bound to a proof: k-induction

For a property that is *inductive* — holding after any `k` consecutive moves implies it holds
after one more, from any state satisfying it, not only the initial state — the bounded result
extends to every schedule of any length:

- **Base**: `R(s_0) ∧ … ∧ R(s_k)` is not violated from the initial state (the bounded query).
- **Step**: `I(s_i) ∧ R(s_i) ∧ … ∧ R(s_{i+k}) ∧ T … ⇒ R(s_{i+k+1})`, from an arbitrary state —
  the same one-move transition relation with `s_0` unconstrained except by `R` and by a
  **structural invariant** `I`, defined below, that says the state is one the encoding can
  represent.

The step is sound only over a `T` with **no cut in it**: every transition of the real system
must be a transition of `T` from every state, or the step proves invariance of a smaller
system. Three places in the bounded encoding are cuts, and each must be absent or the behavior
is excluded from the unbounded verdict:

- **Body loops.** A merge-loop qualifies — its progress is token position, which is state, and
  its back edge is an ordinary move of `T`, so the step covers iteration `L+1` as readily as
  iteration 1. A body `Loop` does not: it is unrolled `L` times *inside* one move, so `T` has no
  transition for an iteration past `L`, from any state, and an `unsat` step says nothing about
  it. Lifting a body loop into moves would change the atomic unit, and is not done.
- **Token slots.** The bounded query sizes the slot set from `k`, and a slot set is a cut
  wherever a fork finds every slot occupied: `T` then has no transition for the spawn the
  executor would make. The step runs from arbitrary states and for any length, so two things are
  needed. First, the token population of *every* run must be bounded by the graph: it is when no
  cycle of the graph contains a fork whose branches can return to it without being collected by
  a join on that cycle, and the step is refused otherwise, since a cycle through an uncollected
  fork grows the population by one each time round and no finite slot set covers it. Second, the
  arbitrary start state must be one that a run of that graph could be in — the graph-wide
  maximum bounds runs from `s_0`, not states the solver invents with every slot full and a token
  before a fork. So the step's slots are **assigned by structure**, one slot per static branch
  region of the fork nesting, and the antecedent carries the structural invariant `I`: a region
  holds at most one live token, a fork's branch regions are empty while a token is in the region
  that contains the fork, and `joined[j] ≤ indegree(j)`. Under `I` every fork in `T` finds its
  branch slots free, so `T` has the spawn. `I(s_0)` holds by construction, and `I ∧ T ⇒ I'` is
  asked as a query of its own before the step; if it is `sat`, `I` is not inductive for this
  graph and the step is refused rather than run over an antecedent it cannot justify.
- **Bus slots.** The same for `M`: a cycle containing a `Send` can grow the bus without bound,
  and the step is refused when the graph has one. Otherwise each `Send` statement owns one bus
  slot, and `I` adds that a statement's slot is empty while a token is at or before it in its
  region, so every `Send` in `T` has a free slot. (The bounded query is unaffected, its `M`
  being sized from `k`.)

The refusal is printed in the report, and stops there:
`proved within k moves; loop 'name' unrolled L times, not inductive`,
`…; fork 'spawn' on a cycle can grow the token population without bound, not inductive`,
`…; the structural invariant is not inductive, not inductive`.

If the step is `unsat`, `I` is inductive and no cut is in `T`, the verdict is
`proved, unbounded (k-induction at k)`, and a merge-loop with a loop-invariant requirement is
proved without running it to termination. If the step is `sat` the counterexample starts from a
state that may be unreachable; it is *not* reported as a violation — the verdict stays the
bounded one, and the report says the induction failed and at which `k`. Strengthening `R` with
an auxiliary invariant so the induction closes is the user's move, and a later stage may search
for one. This is a later stage in any case; the bounded verdict is what ships first.

## User surface

The flags of the explicit note, with the engine selected and the inputs and bounds this design
adds. The engine is the framework's `-engine`: absent, `auto` ranks the covering engines as
the framework note says; `smt` needs a solver on the path and refuses with the SMT layer's
existing error otherwise; `all` runs every covering engine, so the solver's answer stands
beside `check`'s or its refusal, which is the referee run a review would ask for.

```
sysml plant.sysml -instantiate Plant::reactor \
    -action "Plant::Reactor::regulate reactor" -engine smt \
    -check-property Plant::MaxPressure \
    -check-input inletTemp -check-assume Plant::EnvelopeLimits \
    -check-diverge pressure \
    -check-depth 40 -check-unroll 4
```

| Flag | Meaning |
|------|---------|
| `-engine auto\|explore\|check\|smt\|all` | The engine, as the framework spells it; `auto` is the default |
| `-check-property <condition>` | The requirement or constraint to hold at every state; repeatable |
| `-check-input <feature>` | Leave the feature free in its declared domain; repeatable. Absent, every unbound input is free and every bound one is pinned |
| `-check-assume <constraint>` | Assume a constraint over the initial state; repeatable |
| `-check-diverge <feature>` | Compare the feature's final value across schedules, the flag the `check` engine reads for its divergence search; naming one makes the question `Sensitive`; repeatable; under `smt` it is the two-copy query (stage 3) |
| `-check-depth`, `-check-unroll`, `-check-timeout` | The bounds: moves `k`, loop unrolling `L`, solver time |
| `-check-witness <dir>` | Write each witness as a replayable trace file |
| `-schedule replay:<file>` | The replay policy: fix the witness's inputs, then follow it move for move, then behave as `reverse` if it runs out. Refused when a move in the file is not enabled at that point, naming the move, or when an input names a feature the behavior does not have — a witness that cannot be followed is never silently resolved |

`-json` carries `engine`, `verdict` (`proved`, `bounded`, `violated`, `sensitive`, `not-covered`),
`bounds` (each with its value and whether it could be reached), `inputs` (each with its domain
and, for a witness, its value), `witness` and `replay` (the command), and `reason` for a
*not covered*. The REPL gains `%check-action … engine=smt` with the same words, and `%replay
<witness>` from the explicit note walks a solver witness in the debugger exactly as it walks an
explored one. The wire and the clients gain the engine as an option of the existing exploration
request once the CLI surface has settled, advertised as a capability of its own; that is still
deferred, no stage so far touching the wire.

The witness file is the one the explicit note's `replay:` policy reads, one line per move, and
opens with the inputs the solver chose when the witness fixes any:

```
input inletTemp = 399.5
input mode = Plant::Mode::Fast
step 3: 2@heat first of 2@heat, 3@vent
```

An `input` line spells the value as the notation does — an integer, a rational as `-1.0`, a
Boolean, an enumeration or variation value by its qualified constructor — and is evaluated and
fixed before the action's defaults, through the same path a caller's inputs take. A file with
no `input` line is a stage-1 or `check` witness and replays as before.

Solver requirements are stated through the capability model: the query needs `CapModels`,
`CapDatatypes` (nodes, slots and choices are finite datatypes) and therefore
`CapNonStandardLogic`, and `CapIncremental` for the outcome enumeration and for k-induction;
nonlinear arithmetic only where a body computes a product of two computed values. Both verified
backends have all of these, so the design is not z3-specific, and no optimization capability is
needed.

## Test contract

Written before the code, as the behavioral contract asks:

1. **Encoding faithfulness.** The referee gate above, over every conformance case with
   `outcomes`, every case naming a requirement or constraint, and the randomized graphs. Runs
   with `OPENSYSML_REQUIRE_SMT=1` in CI, as the SMT layer's gates do, and skips with a named
   reason without a solver.
2. **Witness replay.** Every witness written for the corpus replays to the state it claims, and
   the replay's trace equals the witness's `choice` lines. A `replay:` file with a move that is
   not enabled is refused with a typed error naming the move.
3. **Verdict discipline.** A model with a computed product answers `unknown` under a short
   timeout and is reported *not covered*, never *proved*. A model with an untranslatable body is
   refused before any query, naming the node. A merge-loop of more iterations than
   `-check-moves` allows reports `no violation within` and not `proved`; the same loop with a
   loop-invariant requirement and k-induction reports `proved, unbounded`; a body `while` under
   `-check-unroll 1` reports `no violation within`, and with k-induction reports
   `proved within k moves … not inductive`, never `unbounded`; so does a merge-loop that
   re-enters a fork whose branches are never joined. A behavior whose every schedule deadlocks
   at a join reports the deadlock and never *not sensitive*; one with a schedule that divides by
   zero at move 12 and another still live at `k` reports the error, not the bounded sensitivity
   result. A fork whose two branches both
   arrive at one merge, and a merge-loop re-entered three times, produce the executor's outcomes
   (two merge performances; three), pinned as referee cases.
4. **Inputs.** A requirement that holds for the model's default input and fails for another
   value in the domain is *violated* under `smt` and *no violation* under `explore` on the same
   command, and the report of each says why they differ.
5. **Sensitivity.** `action_fork_branches_write_one_feature` reports `x` sensitive with the two
   witnesses and `leftRan`, `rightRan` not sensitive;
   `action_explore_performed_and_accept_due_together` reports `x` sensitive with the paused body
   and the sibling accept as the diverging pair; `action_join_waits_for_slowest_branch` reports
   `arrived` not sensitive. `action_fork_branches_write_one_feature` at a `-check-depth` short
   of its completion reports `no sensitivity found within k moves`, not *not sensitive*. These
   expectations are derived from the oracle, not from the checker.
6. **Portability.** One query per feature the encoding emits — datatypes for nodes, the
   two-copy query, the incremental enumeration, the induction step — added to the SMT layer's
   portability harness, so a backend that refuses one is reported as refusing it.
7. **Regression of what exists.** No golden trace, expected output or `explore` result changes;
   the replay policy under a witness that `explore` produced reproduces that run's trace.

## Stages

Each stage leaves `develop` green, ships behind `-engine smt` (the framework's spelling of
`-check-engine smt`), and is useful on its own.

1. **The transition relation for actions on concrete inputs.** Straight-line bodies, fork, join,
   merge, body loops with unrolling, decisions, pins and object flows; no clock, no messages, no
   nested flows. The requirement and deadlock properties, the *proved*/*bounded*/*violated*/*not
   covered* verdicts, witness decoding and the `replay:` policy. Referee checks 1–3 over the
   corpus cases these constructs cover. This is where the encoding is proved faithful and is the
   stage whose review matters most. *Implemented:* `internal/core/smt`, beside `solve` and
   `analysis`: `Encode` builds the relation over a lowered `ActionGraph` (`state.go`,
   `encode.go`), the properties and the cut, unroll and overflow flags are `property.go`, the
   `smt` engine (`engine.go`) answers `analysis.Question.Holds` with the schedule free and refuses
   every other kind, free inputs and a fixed schedule with a typed reason, and `witness.go`
   decodes a `sat` model to `ChoiceTaken` lines and replays it under `replay:`. The engine is
   `analysis.External` over the SMT layer's solver discovery (`OPENSYSML_SMT`, then `z3`, then
   `cvc5`). `k` is `Budget.Depth`, the solver time `Budget.Solver`; the unroll bound `L` has no
   `Budget` field and is the engine's own, 4 by default, reported in `Bounds` as `unroll`. An
   `unsat` is *proved* only when the cut, unroll and slot-overflow flags are all unsatisfiable
   at `k` too — every schedule finishes within the bounds — and *bounded* otherwise; `unknown`,
   a timeout, a refusal, a witness that does not replay and a replay that disagrees are *not
   covered*, and so is an `unsat` over arithmetic the evaluator rounds (`Query.Rounded`): the
   framework's strength table downgrades it as `%check` does, rather than qualifying a proof
   *over exact arithmetic*, because a violation that exists only after `float64` rounding has
   no witness to replay. State zero is the values the started performance holds — the inputs
   the question's `Start` supplies ahead of the defaults the action declares. The engine is
   **registered in the build's registry**, `engines.Default()` (`internal/core/engines`), which
   composes `analysis.Default()` — the framework's own engines, which `smt` imports and so cannot
   be constructed from — with `smt`; the CLI, REPL and gRPC service build that registry at
   startup, so `-engine smt` and `%engine smt` reach it, and `-engines`, `%engines` and
   `ListEngines` list it with its solver as its status (*not covered: no solver* in a plan when
   none is found); no wire request asks `holds`, so over gRPC it is listed and not reached. It
   is registered at its authority, *proved*, and ranks ahead of `check` for a `holds` question
   under `auto`; since `holds` is asked only under `-engine check`, `-engine smt` or `-engine
   all` with a `-check-*` flag, never under `auto`, registration moved no verdict, plan line or
   golden and made no output depend on a solver being installed — what moved is the engine
   listings (a sixth engine) and the self-model's engine count. A surface that later asks
   `holds` without naming an engine will reach `smt` first; ranking it below `check` instead is
   a one-line change in `engines.register` should that be wanted. The referee harness
   (`referee_test.go`) compares it against `explore`: checks 1–3 run over every corpus action
   case with `outcomes` whose body encodes and record the rest as *refused*; no
   corpus action case names a requirement, so check 3's requirement side is over pinned referee
   models (`discipline_test.go`). Check 4, the randomized graphs, is not built. Every solver test
   skips with a named reason without a solver and fails under `OPENSYSML_REQUIRE_SMT=1`. Test
   layers 1–3, 6 and 7 of the contract are met for these constructs; layer 5 and the
   k-induction sentences of layer 3 are later stages.
2. **Free inputs.** `-check-input`, `-check-assume`, domains from declared types, input values in
   witnesses. Test layer 4. *Implemented:* `Encode` takes the features the started performance
   holds and the names to release (`input.go`, `encode.go`): a feature the model binds — a
   default, a value the performer holds, a value the question's `Start` fixed — is pinned in
   `s_0` as before, and is checked against its declared domain there, so a default outside it
   is a failed start rather than a silent state; a feature the model leaves unbound, or one
   `-check-input` names, is a variable of `s_0` ranging over its sort narrowed by the declared
   type: `Boolean`; `Integer` within the interpreter's `int64`, since a value beyond it has
   nothing to replay; `Natural` as `Integer ≥ 0`; `Real`, `Rational` and a quantity type over
   them as the solver's reals; an enumeration or variation as its constructors. A free feature
   whose effective multiplicity has lower bound zero (`lower.Attribute.Optional`, which the run
   resolves as it does the feature's scope) is also flagged, its presence in `s_0` the solver's
   to choose; a condition reading it while absent is undefined there, so a requirement true of
   every value yet undefined without one is *violated*, and the witness spells the absence
   `input x = null`, which the replay fixes the feature at and the run reads as the interpreter
   reads a feature nothing supplied. Such an input is reported *free … or absent*
   (`analysis.Input.Optional`, `optional` in `-json`). A declared type
   the translator gives no sort — `String`, a collection, an object-valued feature, a type
   without a translation — is `analysis.DomainError` naming the feature and the type, before any
   query, and the verdict is *not covered*; a released name that is no feature of the action, or
   one the action writes back rather than reads, is `analysis.InputError` naming it. The
   performing object's own features are not encoded (stage 6), so releasing one is refused as no
   feature of the action. `-check-assume` names constraints and requirements; each is translated
   as a condition (`Encoding.Assume`, `property.go`) and asserted over `s_0`, one the translator
   refuses refusing the question before any query; the first query asks whether the assumptions
   admit an initial state at all, and an assumption set that admits none is *not covered:
   assumptions admit no initial state* — never *proved*, since `unsat` over no initial state
   establishes nothing about the model; when that query rounds in floating point
   (`Query.Rounded`) its `unsat` decides nothing either way and is *not covered* naming the
   rounding, as a rounded property's is. The question carries what the asker fixes or frees:
   `HoldsAsk.Inputs` (the releases) and `HoldsAsk.Assume`, with `FreeInputs` set when either is
   given, and `Registry.Check` sets `FreeInputs` on a `holds` question too when starting the
   action leaves an input unbound (`runtime.Held.Unbound`), so `check` refuses it rather than
   evaluating a value it does not have and `-engine all` shows the refusal beside `smt`'s
   answer; which inputs are unbound and their domains is what the engine finds, reported per
   input in `Result.Inputs` (name, type, domain, free or its value) with `Result.Assumptions`,
   and the answered question's `Free` gains `FreeInputs` when any input ranged, so the standing reads
   *proved over schedules and inputs: inputs free in their domains: …, assumed …* for a proof
   and *inputs chosen from their domains: …* for a witness, against `explore`'s and `check`'s
   *inputs as written*. `-json` carries `inputs` and `assumptions` on the result and the chosen
   `inputs` on the witness. A `sat` model's free inputs are decoded to `runtime.InputTaken`
   (`witness.go`), spelled as the notation does — rationals exactly, constructors qualified —
   and replayed with the moves under `replay:`: the witness file opens with `input <feature> =
   <value>` lines ahead of the `choice` lines, `runtime.ReplayOf` fixes them through the path a
   caller's inputs take (`ActionExecutor.bindInputs`, ahead of the defaults, on the action the
   run begins on alone — a behavior the performing object runs of its own, or a nested step,
   leaves them), an input naming a
   feature the behavior lacks or that cannot be set is `runtime.WitnessInputError` naming it,
   and a witness whose inputs cannot be set or that does not reach the state it claims is *not
   covered* in the interpreter's favor, as moves are; a file with no `input` line replays as in
   stage 1. `Budget.Unroll` is the loop bound `L` (`analysis.DefaultUnroll`, 4, when zero),
   `-check-unroll` sets it, and `k` is the shared `-check-depth`; `-check-input`,
   `-check-assume` and `-check-unroll` sit on the `check` engine's flag struct, any of them
   makes the question `holds` as `-check-property` does (`analysis.CheckKind`), so under
   `-engine all` it reaches `smt`, and each surface refuses a flag only the engine it left out
   reads (`-check-states` under `smt` alone; the three above under `check` alone) naming the
   flag, rather than dropping it; `-check-witness` writes an `smt` witness
   with its inputs as it writes a `check` one.
   Referee check 4 (`TestRefereeInputs`) explores the layer-4 models with a violated witness's
   inputs pinned and requires the violation reproduced; the corpus cases pin every input, so
   their fourth column is zero. Layer 4 is met; layer 6 gains the enumeration-domain and
   real-input queries. Bodies performing a `tool:<name>` engine stay refused, so the framework
   note's `smt` clause on tool outputs does not yet apply. The engine is in the build's registry
   as stage 1 now says; a wire request that asks `holds` and its client option stay deferred
   until the CLI surface has settled, so the gRPC service lists `smt` but no request reaches it.
3. **Sensitivity.** The two-copy query, the diverging pair, `-check-diverge`. Test layer 5.
   *Implemented:* `smt` answers the framework's `Sensitive` question (`sensitive.go`), which is
   the `check` engine's question under the same flag: `-check-diverge <feature>` and
   `%check-diverge` name the features on `HoldsAsk.Diverge`, naming one makes the question
   `Sensitive` (`analysis.CheckKind`), so under `-engine all` one question reaches both engines
   and `compose.go` sees two answers about the same feature; there is no `-check-sensitive`,
   and the refusal of `-check-diverge` under `smt` alone that stage 2 recorded is gone
   (`-check-states` under `smt` alone stays refused). The two copies are built from the
   encoded relation, not its slot layout (`twocopy.go`): copy `B` is `Encoding.Query` with
   every variable but those of `s_0` and the free inputs renamed under `B/`, its assertions
   rewritten by `solve.Substitute`, so a stage that adds slots to the state vector adds them
   to both copies without change here. The three queries are asked in the order the
   *Properties* paragraph spells, each on its own and none incrementally, so the stage needs
   no `CapIncremental`: `complete^A_k ∧ complete^B_k ∧ f^A_k ≠ f^B_k`, then the deadlock and
   typed-error properties (a `sat` on either is that *violated* finding), then `cut_k ∨
   loopflag`, whose `sat` is *no sensitivity found within k moves* at `ClaimHolds`/`Bounded` —
   the pair the bounded negative of a `Holds` question already uses — and whose `unsat` is
   *not sensitive* at `ClaimHolds`/`Proved`. A `sat` two-copy model decodes to two schedules
   (`Encoding.Diverging`), both replayed through `runtime.ReplayAction` before the verdict is
   `ClaimSensitive`/`Witnessed`; a copy whose replay ends with another value of `f` is *not
   covered* naming the disagreement, in the interpreter's favor. The report prints the two
   final values, the earliest move at which the schedules differ and the two moves taken, and
   both witnesses with their replay commands; `-json` carries the pair as `witness` and
   `contrast`; `-check-witness` writes `<checked>-<feature>-A.witness` and `-B.witness`, each
   replayable through `-schedule replay:` and `%replay`. The default feature set, with
   `-check-diverge` absent on a `Sensitive` question, is the `check` engine's — every
   attribute of the action and, with a performer, of the performing object — and since the
   performing object's features are not encoded before stage 6, `smt` refuses each of those
   per feature as *not covered* naming the construct rather than narrowing the list — the
   action's own features on the list are still asked, a sensitivity found among them is the
   answer, and a negative over a list with a refused feature, or one the solver left
   undecided, is *not covered* naming it, never a proof over the whole list; the CLI
   and REPL only ask `Sensitive` when the flag names a feature, so no `holds` question gained
   a sensitivity answer and no `-engine smt` or `-engine all` expectation moved. Layer 5:
   `action_fork_branches_write_one_feature` reports `x` sensitive, both witnesses replayed to
   the two values the case's `outcomes` list, `leftRan` and `rightRan` not sensitive;
   `action_join_waits_for_slowest_branch` reports `arrived` not sensitive; the fork case at a
   depth short of completion reports *no sensitivity found within k moves*, never *not
   sensitive*; a join every schedule deadlocks at reports the deadlock
   (`sensitivity_test.go`). The row `action_explore_performed_and_accept_due_together` needs
   the performed action and the clock stage 4 encodes; it stays *not covered* naming the
   construct, pinned as such, and is the row stage 4 completes. The referee gains a fifth
   column: each feature of every completing corpus case is asked across schedules, a
   *sensitive* answer has both witnesses replayed to two distinct values among the case's
   `outcomes`, and a *not sensitive* feature has one value across them. Layer 6 gains the
   two-copy query. The wire and the clients still do not carry the question.
4. **Clock, messages and nested flows.** `now` and due times, the bounded bus, performed actions
   as nested slots, so the whole action fragment the runtime executes is covered. The corpus's
   `accept` cases join the referee. *Prepared, not implemented:* `Flow` (`support.go`) numbers
   the root graph and every flow a node states of its own as frames (`Flow.Frames`,
   `Flow.FrameOf`), each with its own node range, labels prefixed by the performing node and a
   slot count summed into `T`, and records the `Send` and `accept` sites it meets (`Flow.Sends`,
   `Flow.Accepts`, `Flow.Bus` as `M`); `State` (`state.go`) declares the parked and due
   variables, `now` and the bus slots only for a flow that has accepts, timed accepts or sends,
   and lists its variables as a named vector (`State.Vector`) for a query over two copies of
   the relation. No transition reads them yet: `send`, `accept`, `accept after`/`accept at` and
   a node stating a flow of its own are refused before any query, naming the node and the
   construct, exactly as stage 1 left them; the `Bounds` line lists no `bus`; the referee's
   tally is stage 2's. The horizon question is settled above, in "The state": no horizon, and
   `-advance` stays refused for `smt`.
5. **k-induction.** The step query and the `proved, unbounded` verdict.
6. **Bounded heap and calc inlining.** Object-valued pins over `N` objects per type; inlining a
   pure, loop-free calc body. Each moves rows of the coverage table from *not covered* to
   *encoded*, and the referee corpus grows with them.
7. **State machines.** Its own note, on the pattern of this one, once the explicit engine
   explores state machines and there is something to referee against.

Stages 1–3 answer the safety-case question for the action fragment `explore` covers today, and
add what `explore` cannot: scale and inputs.

## What this does not change

- **The evaluator and the executor stay normative.** The solver proves things about the
  encoding; the referee proves the encoding is faithful on the cases it covers; a replayed
  witness is a run of the interpreter. Where they ever differ the interpreter wins and the
  encoding is fixed.
- **`explore` stays.** It is the concrete engine, the referee, the fallback for every *not
  covered* row, and the engine that needs no solver installed.
- **The compliance map's honesty.** SysML v2 states no solving semantics; this is an OpenSysML
  extension, marked so as the SMT layer is, and no row moves on its account. A model proved
  insensitive is a fact about that model, not about the runtime's scheduling.
- **The SMT layer's contracts.** Verdicts stay distinct, refusals stay whole-query, backends stay
  external processes, `Rounded` stays reported.

Under the [analysis framework](analysis-framework.md) this checker is the `smt` engine: the
referee and the fallback above become the framework's `-engine all` and `auto` dispatch, the
four verdicts map onto its strength scale (*proved*, *bounded*, *witnessed*, *not covered*), and
`-check-engine explore|smt|both` is written `-engine explore|smt|all`. The encoding, the
verdicts, the bounds and the stages here are unchanged by it.

## Alternatives considered

- **Path-wise symbolic execution: `explore` the schedules, solve the data.** Each explored run
  yields a path condition over the free inputs; one solver query per run asks whether any input
  violates `R` along it. No second semantics — the path condition is read off the interpreter —
  and inputs become symbolic. But every schedule is still a run, so the scale problem stays,
  and the referee this design gets for free (agreement with `explore`) is the whole method
  there. It is a legitimate first deliverable for the *inputs* half alone if stage 1 proves
  larger than expected, and the encoding's expression translator is shared with it.
- **Translating to an existing model checker** (SPIN, TLA+, UPPAAL, nuXmv). The same second
  semantics, in a third language, with the translation kept in step by hand; and no path back to
  the interpreter for a witness without writing the replay this design writes anyway. The SMT
  layer's process-and-script contract is the smallest surface that gets the algorithms without a
  new dependency.
- **A symbolic interpreter.** Running the executor over symbolic values would answer data and
  schedules together with one semantics. The value domain — quantities, collections, strings,
  object graphs — makes a symbolic state a much larger project than this encoding, whose subset
  is the SMT layer's already-translatable one; and its witnesses would need the same replay.
  Left where the explicit note left it.
- **Encoding statement-level interleaving.** The finer reading the library admits (two unordered
  performances may overlap). Multiplies the schedule space by the product of body lengths and
  puts the two engines at different granularities, so no referee. If a property ever needs it,
  both engines take it together under one flag.
