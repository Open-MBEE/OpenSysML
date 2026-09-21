# Which query is which

OpenSysML answers questions about a model through several surfaces that all
carry the word *query* or behave like one. They read different things — the
model's elements, the objects a session holds, one expression's value, a
solver's assignment — and each is blind to what the others see. This page
places them side by side: what each takes, what it returns, what it cannot
see, and where to read the details. The pages linked from each section are
the reference; this one only draws the boundaries.

| Surface | Reads | Returns | Cannot see | Entry points |
|---|---|---|---|---|
| [Document queries](#document-queries-over-elements) | The model's elements: declarations, ownership, relationships, declared values | Ordered rows of elements, projected into typed columns | Objects unless the session holds them; nothing a solver would infer | `%run-query`, `-run-query`, `RunDocumentQuery`, tables and lists in a document |
| [Object and verdict rows](#object-rows-and-verdict-rows) | The objects a session holds and the assertions about them | Rows standing for objects (by path and id) or for verdicts | Objects no session created; the model's relationships over an object row | The same, in a session with `-instantiate`/`%instantiate` |
| [State and event rows](#runtime-state-and-event-queries) | The state machines the held objects exhibit, as they stand now, and the trace the session recorded | One row per active leaf state; one row per trace record, at its instant | Anything outside the current session; a run no trace recorded | The same, in a session with a run and `-trace`/`%trace on` |
| [API `Query`](#the-api-query-over-a-project) | The elements of one loaded model, by property | Elements with their properties, in declaration order | Objects, values, the library, traversal beyond containment | `Query` RPC, `model.query(...)` |
| [OSLC Query text](#the-api-query-over-a-project) | The same elements, by prefixed property | Element identification only | The same; `or` | `%query`, `-query`, `oslc_query` in `Query` |
| [`Evaluate`](#evaluate-one-expression-in-one-scope) | One expression in one scope, over a held object when named | One value — a scalar, a sequence, an object, an element | Rows, columns, anything the expression's scope does not reach | `%eval`, `-eval`, `Evaluate` RPC |
| [`all T` and collection operations](#the-runtime-population-all-t) | The instances a definition classifies, as an expression | An ordered sequence, and whatever the library functions make of it | Data-type values; the state a machine is in; the trace | Inside `%eval`, `-eval`, `Evaluate`, any expression |
| [`solve` and its siblings](#solve-over-constraints) | A constraint, requirement or satisfaction assertion, with what is already fixed | A satisfying assignment, `unsat` with the conflict, or `unknown` | What a run *did*; anything the solver's theory does not cover | `%check`, `%solve`, `%explain`, `%configure`, `%optimize`; the `solve` engine |

## Document queries over elements

A **document query** is a `calc def` specializing `DocumentQueries::Query` whose
body composes the library's operations — `OwnedElements`, `Descendants`,
`WhereType`, `WhereFeature`, `Project`, `OrderBy` and the rest of the
[vocabulary](introduction.md#the-vocabulary) — into a relation over the model.
It answers with **rows**: each row stands for an element, and `Project` gives
the rows named, typed columns read from the element's properties or computed
by a `Column(name, expression)`. The order of the rows is the model's
declaration order until an `OrderBy` says otherwise, which is what makes a
document regenerate byte-identically.

What it reads is the *declared* model: an element's name, type, multiplicity,
documentation, the value an attribute is declared with or redefined to, and
the relationships the model draws (`RelatedElements`). A derived value the
declaration does not spell out is not there — a `WhereFeature` on `mass`
reads the declared or redefined `mass`, not an expression's result — unless
a `Column` computes it or the row is an [object row](#object-rows-and-verdict-rows).

Rows live in one place: a document's `Table` or `List` renders them, and
`%run-query`/`-run-query` print them. The [query cookbook](query-cookbook.md)
is the recipe book; the [command reference](../reference/cli.md#command-reference)
lists the flags, and [Interfaces](interfaces.md) the gRPC and Python calls.

What it cannot do: it does not evaluate arbitrary expressions over the model
(that is [`Evaluate`](#evaluate-one-expression-in-one-scope)), it does not
infer values (that is [`solve`](#solve-over-constraints)), and it does not
read objects a session has not created.

## Object rows and verdict rows

In a session that holds objects — `-instantiate <name>` on the command line,
`%instantiate` at the prompt, `Instantiate` over gRPC — the same operations
read the **objects** as well as the elements. A binding written as a usage's
name binds the held object under that name while the session holds one, and
the element otherwise; `Objects(type = T)` enumerates every held object of a
type without a binding. An object row's `WhereFeature`, `Project` and
`OrderBy` read what the object holds **now**, after a run changed it, and the
row renders by path with its id (`Cookbook::telescope.primaryMirror (#2)`).
`Verdicts(source, kind)` turns object rows into **verdict** rows, one per
assertion checked on the object: `holds`, `violated` or `undecided`, with the
reason. See [Objects the session holds](query-cookbook.md#objects-the-session-holds)
and [Which constraints and requirements hold](query-cookbook.md#which-constraints-and-requirements-hold).

Two boundaries are worth keeping in mind. The session is the whole world:
`Objects` returns no rows in a session that holds nothing, and is refused
with a typed `no-runtime` error where there is no session at all — the
library evaluating a document on its own, as the editor's preview does.
And an object row is not an element: `RelatedElements` reads the model's
relationships and is refused over one (`object-row`); traverse from the
element and bind what you find.

Over gRPC, `RunDocumentQuery` answers an object row in the `object` arm of
`DocumentValue` and a verdict row in the `verdict` arm; see
[Native document queries and rendering over gRPC](../reference/api.md#native-document-queries-and-rendering-over-grpc).

## Runtime state and event queries

Object rows tell you *what an object holds*; three more operations tell you
*where its behavior stands* and *what happened to it*. They are document-query
operations like `Objects` and `Verdicts`: they take object rows, the row
operations (`WhereType`, `WhereName`, `WhereFeature`, `Project`, `OrderBy`,
`Column`) read their rows, and they are refused with the same typed
`no-runtime` error where there is no session.

**`States(source)`** answers, for each object row it is given (or each
object the session holds of an element named), the states the object's
exhibited machines are in now — one row per active leaf state, so a machine
in a `parallel` state contributes one row per region. The row is a **state**:
`object` and `path` are the object, `machine` the exhibited machine (`lp`),
`name` the leaf's own name, `statePath` its name qualified by the states
enclosing it (`on.run`), `region` the orthogonal region it runs in (`""`
outside one) and `enclosing` the composite states active with it, outermost
first; `WhereName` reads `name`, and the row answers the state declaration's
own properties too. It prints as `lamp1.lp in on.run`. An object exhibiting
no state machine is a typed `no-state-machine` error, not an empty row set.
A machine that has terminated has no active state and so contributes no row,
while a machine that has completed reports its final state; an object the run
destroyed is a typed `object-destroyed` refusal, for `Events` too.

**`InState(name)`** is the inverse: the objects the session holds whose
machine is in the state named — a leaf or a state enclosing one, by name or
by dotted path (`on.slow`) — each object once, as object rows, so anything
that reads an object row reads them. A name no held object's machine declares
is a typed `unknown-state` error.

**`Events(source, kind, since, before)`** answers the trace as a relation in
the order it was recorded: one row per signal accepted (`accept`), per signal
sent (`send`), per transition fired (`transition`), per state `entry`, `exit`
and `do` step, per `choice` point the run drew (a due order among concurrent
reactions, a junction split) and per `guard` it could not evaluate. Each row
is an **event**: `time` is the instant read from the runtime clock, as a
duration in the clock's second when the library defines it; `object`, `path`
and `machine` say whose behavior made the record; `state`, `from` and `to`
name the state entered, exited or transitioned between; `event` the trigger;
`target` the object a send was addressed to; `payload` an accepted or sent
signal's parameters as `name = value`; `alternatives` and `taken` a choice's
draw; and `text` the line `-trace` prints. `source` keeps the rows of the
objects given (every object's when absent), `kind` the kinds named (`all`, or
kinds separated by commas), and `since` and `before` bound the instant as a
duration or a bare number of clock units — inclusive at `since`, exclusive at
`before`, so `[0 [s], 1 [s])` and `[1 [s], 2 [s])` partition the trace. It
prints as `t=1 lamp1.lp: accept Dim`.

The rows are the typed record the trace is kept as, which `-trace` and
`%trace` print from — not a parse of the printed lines. So a trace query needs
a session that records one: `-trace` on the command line, `%trace on` at the
prompt before the run (the population `Instantiate` builds over gRPC is
traced from the start, keeping the most recent `OPENSYSML_GRPC_MAX_HELD_EVENTS`
records; an interval reaching back past them is a typed `trace-truncated`
error naming the instant history is kept from, never a shortened relation);
without it the query is a typed `no-trace` error rather than an empty
relation, and `%trace off` discards the record. A bound that is not a
duration (`1 [m]`), an interval with `before` at or before `since`, and a
`kind` the trace does not record are `invalid-interval` and `invalid-argument`
errors; `States(source = Events(...))`, or `Events` over a verdict row, is an
`event-row`/`verdict-row` error, as a state or event row given to a
model-only operation such as `RelatedElements` is.

On the command line, `-run-query` runs after the `-state`/`-action` behaviors
and the `-advance` named, so a state query reads where the run left each
machine and an event query reads what the run recorded. The cookbook's
[Where the objects stand and what they did](query-cookbook.md#where-the-objects-stand-and-what-they-did)
section has the recipes for the three questions this vocabulary exists to
answer.

What these cannot see: another session's run, a machine no object exhibits,
and the future — the rows are what *has* happened up to the instant the clock
reads now. Advancing the run is the REPL's job (`%advance`, `%step`, `%send`);
a query only reads, and never moves the clock or a queue.

## The API `Query` over a project

The **SysML v2 API & Services `Query`** is an interoperability surface: the
structured `Query` shape the standard defines — `scope`, `select`, a `where`
tree of primitive and composite constraints — filtering the elements of one
loaded model by their properties (`name`, `@type`, `multiplicityLower`, …). It
answers elements with their properties, in declaration order. **OSLC Query
text** is a second spelling over the same elements (`oslc.where=sysml:name="wheel"`
with `oslc.select`, `oslc.orderBy`, `oslc.properties` and `oslc.searchTerms`),
accepted for element identification by tools that speak OSLC. The two differ
in what they can express — structured queries support `or`, OSLC compound
terms only `and` — so neither subsumes the other.

```console
$ sysml cookbook.sysml -query 'sysml:name="primaryMirror"'
✓ package Cookbook
Cookbook::telescope::primaryMirror  PartUsage
```

Both are element identification and nothing more: no traversal, no joins, no
ordering or paging beyond the OSLC `orderBy`, no derived values, and nothing
about objects, runs or verdicts. Neither reaches the standard library unless
a scope names a library element. An unknown parameter or property is refused
as written (`unknown OSLC query parameter "rdf:type"`), not answered empty.

Read [SysML v2 API & Services `Query`](../reference/api.md#sysml-v2-api--services-query)
for the structured form and its comparison semantics, and
[OSLC Query text](../reference/oslc-query.md) for the text grammar; the
entry points are the `Query` RPC (`query` or `oslc_query`), `model.query(...)`
in Python, `%query` at the prompt and `-query` on the command line.

## `Evaluate`: one expression in one scope

`Evaluate` is not a query over a set: it computes **one expression** and
answers **one value**. The expression is any SysML expression the evaluator
understands — arithmetic, feature chains, library function calls, `all T`,
a quantity with units — resolved in one scope, and the answer is what it
comes to: a number, a string, a sequence, an object reference, an element.

```console
$ sysml cookbook.sysml -eval "Cookbook::telescope.instrumentCluster.mass"
✓ package Cookbook
✓ Cookbook::telescope.instrumentCluster.mass
  = 4.5
```

The scope is the difference between the surfaces' spellings. `-eval` and `%eval`
resolve the expression in the last namespace the session declared, reading the
model — a feature the model leaves open is `<undetermined>`; `%eval in <name> :
<expression>` resolves it in a named element's namespace or, given an object
reference, on that held object, so a bare feature name reads the value the
object holds now, as an object row does; the `Evaluate` RPC takes the same as
`context_symbol_id` and `subject_symbol_id`. See `%eval` and `%eval in` in the
[REPL command reference](../reference/repl-commands.md) and the
[`Evaluate` RPC on the wire](../reference/wire-contract.md#evaluate).

What it cannot do: produce rows or columns (an expression's sequence renders
as one value), read a trace, or find values a model does not fix — an
unbound attribute evaluates to nothing, it is not solved for.

## The runtime population `all T`

`all T` — KerML's extent operator — is the **expression form of a runtime
query**: the ordered sequence of the instances of `T` the run has, every
object the definition classifies, nested usages included, in declaration
order; for an enumeration its literals, for a variation its variants. It is
an expression, so it lives inside `Evaluate` and the collection operations of
the library apply to it:

```console
$ sysml cookbook.sysml -eval "(all Cookbook::Subsystem).mass"
✓ (all Cookbook::Subsystem).mass
  = [10.0, 4.5, 15.0]
$ sysml cookbook.sysml -eval "ControlFunctions::select(all Cookbook::Subsystem, {in s : Cookbook::Subsystem; s.mass > 5.0})"
✓ ControlFunctions::select(all Cookbook::Subsystem, {in s : Cookbook::Subsystem; s.mass > 5.0})
  = [Instance(ID: 2), Instance(ID: 4)]
$ sysml cookbook.sysml -eval "RealFunctions::sum((all Cookbook::Subsystem).mass)"
✓ RealFunctions::sum((all Cookbook::Subsystem).mass)
  = 29.5
```

Without an `-instantiate` the population is the declared one — the objects
the model's usages stand for; with one, the objects the session created.
`all T` over a scalar or structured data type is refused
(`unbounded extent: ScalarValues::Real is a data type, whose values are not
enumerated`), since a run creates no data values to enumerate. See
[Extents](../guide/05-checking.md#casts-the-unbounded-value-and-metadata).

Where the two runtime forms meet: `all T` is to `Objects(type = T)` what an
expression is to a relation. `all T` answers a sequence a further expression
consumes; `Objects` answers rows a document renders, a `Project` gives
columns and a `WhereFeature` filters. Neither sees which state an object's
machine is in or what the run accepted — that is what
[`States` and `Events`](#runtime-state-and-event-queries) are for.

## `solve` over constraints

`solve` does not read what the model or a run *has*; it asks an external SMT
solver (`z3` or `cvc5`) what *could* be. `%check <name>` asks whether a
constraint, requirement or satisfaction assertion can be satisfied and
answers `sat` with an assignment, `unsat` or `unknown`; `%solve <name>` asks
for values that satisfy it while keeping what is already fixed — the values
an object holds, or failing that the ones the model declares — and names the
fixed values that conflict when none exist; `%explain` reduces an `unsat` to
its conflicting conditions; `%configure` and `%optimize` ask the same
question over variation points and an objective. The same solver is the
`solve` analysis engine `-engines` tables, whose verdicts carry a
`standing:` of `satisfiable`/`unsatisfiable`.

```console
sysml> %solve Cookbook::MirrorAssembly::lightweight
✗ Constraint lightweight has no values consistent with the value already fixed (z3, 9ms)
  Already fixed:
    Cookbook::MirrorAssembly::mass = 10  (declared)
  In the conflict: Cookbook::MirrorAssembly::mass = 10
  standing: unsatisfiable (proved over inputs: 1 query by solve)
```

Satisfiability is not evaluation: `%solve` finding an assignment says nothing
about whether the object *holds* — `%constraint`, `%satisfy` and the
`Verdicts` rows say that. Nor does it see a run: the trace, the state a
machine is in and the messages in flight are outside the solver's theory.
There is no `Solve` RPC: `ListEngines` advertises the `solve` engine, but no
gRPC call poses a satisfiability question, and `-solve` is not a command-line
flag. See the solving commands in the
[REPL command reference](../reference/repl-commands.md#checking-every-schedule-of-an-action-or-a-state-machine),
[Analysis engines](../reference/cli.md#analysis-engines) and
[installing a solver](../guide/01-install.md#installing-a-solver-optional).

## Choosing

- A table or a list in a document, or any question whose answer is *rows* —
  a document query; over held objects and their verdicts when the session
  holds them; over their states and their trace with `States`, `InState`
  and `Events`.
- A tool that speaks the SysML v2 API or OSLC and wants elements by property
  — `Query`.
- One value — `Evaluate`, with `all T` when the value is a population.
- Values that do not exist yet, or whether any could — `solve`.
