# Changelog

Notable changes per release. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) and, before
1.0, are decided by model compatibility as CONTRIBUTING.md § Versioning states. Cutting a
release is described in [docs/project/releasing.md](docs/project/releasing.md).

## Unreleased

## 0.6.1 — 2026-09-09

### Added

- **A conformance case can admit several outcomes and constrain the order of its trace.** Where
  the Kernel Semantic Library leaves more than one result open, `.expected.json` lists every
  admissible result under `outcomes` and must cite, in `admissible`, the section of the behavior
  semantic oracle deriving them; the run must match exactly one, and a missing or unresolvable
  citation fails the case. A `<case>.trace.order` file of `a < b` lines states the partial order
  the recorded trace must satisfy, beside or instead of an exact golden. The fork case that writes
  one feature from two branches now admits both `x = 1` and `x = 2`; the default schedule and every
  exact golden are unchanged.

- **A parameter of an analysis case or a calc can be swept or sampled, and each run is reported as a row.** `sysml -sweep "<param>=<from>..<to>[:<step>]"` runs the `-analysis` case or the `-calc` once per value of the range instead of once — several `-sweep` flags run their cartesian product, the first flag varying slowest — and prints the inputs bound for each run, what it computed, the objective's verdict and how long it took, as a text table or inside the existing `-json` report. `-samples <n> -seed <s>` draws `n` uniform values from each range instead of enumerating it, deterministically from the seed and in draw order, and echoes the seed with the table. `%sweep` and `%samples` do the same in the REPL, the `RunSweep` RPC over gRPC, and `Model.run_sweep` in the Python client. Each row is an ordinary run of the case with that value bound and every other argument as given, so a run that fails is a row carrying its error rather than the end of the table, and `OPENSYSML_MAX_SWEEP_RUNS` bounds how many runs one table may make. An endpoint carries the units an argument carries, a range between Integers with no step steps by one, and a range between Reals with no step, a step of zero, a step whose sign never reaches the end, a parameter the target does not declare or the arguments already bind, and a distribution asked for by name are refused as typed errors.

- **`x as T` is evaluated.** A cast selects the values of `x` that `T` classifies, in order, and
  answers the empty sequence when none does, so `4.0 as Integer` is `4.0`, `2.5 as Integer` is
  `()` and `(1, 2.5, 3) as Integer` is `(1, 3)`. Scalars are judged by their magnitude against
  the `ScalarValues` hierarchy, quantities by whether their unit is commensurable with the
  dimension the target fixes, arrays, vectors, vector and tensor quantities, measurement
  references, frames and transformations by their shape, units and frame, and objects and
  enumeration literals by the types they carry.
  A type composed of others classifies as they do — the values of a union are those of any of the
  types it unions, of an intersection those of every type it intersects, of a difference those of
  the first that are none of the rest, however deeply nested — so a cast to one keeps them, the
  feature it is written to holds them, and `istype` answers for them; casting a value of a union to
  one of its members is not reported as unrelated either, nor is casting a value to a type composed
  of one it relates to.
  A composed target a value's types leave open is read through its operands, so a bare quantity
  cast to a union of quantity types is kept by the operand whose reference its unit matches, and an
  operand the value settles nothing about is reported as undecided only where no other operand
  excludes the value outright.
  A composed type weighs all the types a value is of at once, whether they are the types a runtime
  value carries or those its feature is declared with, so an object held as a type a
  difference subtracts is none of its values, whether the difference is the target, one it
  specializes, or one an intersection of it reaches.
  Every type a value's feature is declared with counts among the types it is of, so a custom scalar
  subtype (`attribute e : Even = 4`) and a scalar-valued enumeration keep the values declared with
  them — a written sequence entry by entry, each judged by its own declaration however many values
  it holds and however deeply nested, so `(GradePoints::a, GradePoints::b) as GradePoints` keeps
  both and no entry is judged by another's type — and a quantity subtype narrowing its dimension by something a magnitude and a unit do not
  state keeps a value declared with it. An expression written as a value is kept by the evaluation
  type it is read as, a boolean body by `BooleanEvaluation`.
  A cast converts nothing: `ToInteger` and its siblings remain the library functions that do.
  A target that neither a value's types nor its content settles is reported rather than silently
  dropping the value.
- **Classifying a value is model-level evaluable.** `as`, `istype` and `hastype` read the type they
  name rather than folding their operand, so a metadata body may bind `x = 1 as Integer`.

- **A wire `Diagnostic` carries its `code`.** The gRPC/Connect `Diagnostic` message gains
  `string code = 4`, the stable identifier the runtime already assigns: `syntax` for a parse
  error, the pass or rule code for a validation finding, `choice-point` and `guard-unevaluable`
  for a run's notes. Every response that carries diagnostics carries it, so a client branches on
  `code` instead of a message prefix; a diagnostic whose producer assigned no code sends it empty.
  A service that populates it advertises the `diagnostic_codes` capability. The Go, Python,
  Node, Rust and Java clients expose it as `Diagnostic.code` and name the capability.

- **The executors report every choice point: a pick among alternatives the library leaves
  unordered.** Several steppable tokens in one action step, several holding guards at a decision
  node, several enabled transitions out of one state for one event or one change, and several
  tokens writing one feature in one step are each recorded as a `choice` trace line naming the alternatives and the
  one taken (`choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)`), as an
  informational diagnostic on `ExecuteAction`, `ExecuteState` and `RunAnalysis` responses, and as
  one summary line after `%step`, `%continue` and `%advance` (`2 choice points; %trace on to see
  them`). What the executor does is unchanged — reverse token order, first holding guard, first
  declared transition — so every existing result and trace is the same: the guards and
  transitions after the first that holds are read in a preview that is undone, and one that
  cannot be evaluated there is not an alternative and not an error but an informational
  `guard-unevaluable` diagnostic, an `unevaluable guard` trace line and a count in the summary
  (`1 guard not evaluable`). Writes to the performing part and through a feature chain count as
  the object's, so two chains reaching one object in one step are one conflict, as are a feature
  and one redefining it; a step's writes to one feature are one choice listing every token's last
  write, recorded once the step is complete. The
  innermost-transition-wins rule between a substate and the state enclosing it is spec-defined
  order and is not reported.

- **A calc is a value.** A calc definition, a calc usage awaiting an input, or an `in calc` parameter named where a value is expected is a function value: the calc together with the scope and object it was read in, invoked through a calc-typed parameter (`calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }` makes `Fn(Sq, 3.0)` `9.0`), passed positionally or by name, read off a part, returned from a calc, compared and adopted. `SampledFunctions::Sample` now samples a user calc, and a library function the runtime implements (`RealFunctions::sqrt`) is a value too. A calc declared in a behavior body closes over the innermost active run of that behavior alone — never a caller's parameters, and nothing when no such run is active. Calling a non-function, an arity mismatch and an unbound calc parameter are typed errors; `in calc` parameters parse in action bodies as they do in calc bodies. A call through a feature chain (`holder.scale(3.0)`) is now checked statically as a direct call is — arguments against the calc feature's inputs, the result against the declared type it binds to — applies the calc even when defaults supply every input or it has none (`holder.scaled()`, where the bare read `holder.scaled` computes its result), and a typed feature valued by such a call (`item t : Tallied = picker.pick(lead, trail)`) classifies the argument the calc returns as one valued by a direct call does.
- **Function values cross the API.** `Value.function` carries the calc's qualified name and the id of the object it was read off, under the new `function_values` capability, which the Go, Python, Node, Rust and Java clients expose as a typed value and refuse to send to a service without the capability. A function closing over a behavior body's bindings crosses as an unsupported null, since no name reconstructs it; one read off an object is refused as an argument to a later call, since that object lived only within the response that sent it. Native compilation refuses a calc that binds or applies a function value with a typed error.

- **`*` is a value.** In an expression position `*` evaluates to the unbounded value: it exceeds
  every finite Integer, Real and Natural, equals itself, prints as `*` in the REPL and in traces,
  and crosses gRPC on its own `Value.infinity` arm under the `infinity_value` capability — never
  as the ordinary string `"*"`. Arithmetic over it is refused with a typed error naming the
  operation rather than an infinity or a NaN.
- **`elem.metadata` reads an element's metadata.** `ref.metadata` yields the metadata annotating
  the element as a sequence of metadata instances, in declaration order, with the feature values
  the annotation body binds and the metadata type's own defaults where it binds none. An element
  with no metadata yields the empty sequence, and reading metadata off a value is a typed error.

- **The scheduling policy a run resolves its choice points under is selectable.** Where the
  library orders nothing — several steppable tokens in one step, several holding guards at a
  decision, several enabled transitions out of one state for one event, and so whose same-step
  write to one feature stands — the executors follow a named policy: `reverse` (the default:
  reverse token order, first holding guard, first enabled transition, so every existing result and
  trace is unchanged), `declared` (tokens in spawn order, guards and transitions in declaration
  order) or `seed:<n>` (a pseudo-random order the seed fixes, so one seed replays one run on every
  platform and two seeds may take two linearizations). The spelling is the same everywhere: `sysml
  -schedule <policy>` for `-action`, `-state` and `-analysis` (a calc's body performs nothing, so
  `-calc` has no choice to make); `%schedule [<policy>]` in the REPL, shown with no argument and
  applied to the runs started after it while a debugging session under way keeps its own; a
  `schedule` field on `ExecuteActionRequest`, `ExecuteStateRequest` and `RunAnalysisRequest`,
  empty for the default and advertised as the `schedule` capability, with the Go and Python
  clients taking it as an option (`opensysml.WithSchedule`, `opensysml.Schedule`, `schedule=`);
  and a `schedule` pin on a conformance case, which the harness runs under. A policy changes only
  which alternative each choice takes: every choice point a run reaches is reported and each `took
  …` is what the policy took, though another linearization may reach other choice points. A
  spelling naming no policy — an unknown name, `seed` or `seed:` without a number, `seed:-1`,
  `seed:abc` — is refused before anything runs, as `INVALID_ARGUMENT` on the wire. The conformance
  suite also runs whole under `declared` and `seed:1`, requiring every case that pins no policy
  and lists no `outcomes` to produce its default outputs. Two accepts racing for two sends now
  list both pairings as `outcomes`, with the derivation in the semantic oracle; a send to a
  same-named port pins `reverse` until the via-less accept that over-matches it is fixed.

- **The `explore` scheduling policy runs every linearization of a behavior and tables its
  distinct outcomes.** `sysml -schedule explore[:runs=N,depth=D]` with `-action`, `-state`,
  `-analysis` or `-calc` runs the behavior once, recording the alternative taken at each choice
  point, then replays it from the start on a fresh executor of the same loaded model — no object,
  message, clock, calc memo or note of one run is seen by the next — following the recorded prefix
  and taking the next untried alternative at the frontier, depth-first, until every choice
  sequence is spent or a budget is hit. Runs that agree on the observables the conformance
  harness compares (an action's outputs; a state machine's final state, states visited and values;
  an analysis case's outputs and verdicts) are one outcome; the report is one sorted row per
  distinct outcome with the number of linearizations reaching it and the choice sequence of one
  witness, then `complete (N runs)` or `incomplete: <budget> budget <limit> hit after N runs`
  (1024 runs and 64 choice points per run by default; hitting either is exit status `2`, never a
  silent truncation). A run that fails under some order is an outcome of its own (`error: …`), a
  behavior with no choice point explores in exactly one run, and `-trace` prints the witness run's
  trace under each outcome. `-json` carries the rows as `outcomes` and the status as `exploration`.
  The REPL refuses `%schedule explore` with a typed error naming the CLI and the wire, since its
  `%action` and `%state` debuggers step one run. On the wire, `ExecuteActionResponse`,
  `ExecuteStateResponse` and `RunAnalysisResponse` gain repeated `outcomes` (observables,
  `linearizations`, `witness`, `diagnostics`, `error`) and an `exploration` status (`complete`,
  `runs`, `budgets_hit`, `runs_budget`, `depth_budget`), advertised as the `schedule_explore`
  capability beside `schedule`; the Go client adds `ExploreAction`, `ExploreState` and
  `ExploreAnalysis`, the Python client `explore_action`, `explore_state` and `explore_analysis`,
  and the Node, Java and Rust clients the capability name. A malformed spelling — `explore:`,
  `explore:runs=0`, `explore:depth=-1`, an unknown or repeated option — is refused before anything
  runs on every surface.
- **A conformance case that lists `outcomes` is now explored, and the list is exact.** The
  harness runs every such case under `explore`, failing when a listed outcome is unreachable or
  an unlisted one is reached (naming the outcome and a witness choice sequence), and when the
  budget is hit, telling the author to raise it with `"exploreBudget": {"runs": N, "depth": D}`.
  Three cases derive their outcome sets in the behavior semantic oracle: three concurrent writers
  of one feature (six linearizations, three outcomes), a decision with two overlapping guards
  inside a loop, and a state machine with two transitions enabled by one event; a fourth lists both
  orders in which one event's transitions in two sibling regions fire, an order the library leaves
  open, which `explore` varies as a `ChoiceRegionOrder` choice point while the fixed policies keep
  region declaration order and report none. Exploring the
  whole suite found one case pinning a scheduling artefact — two accepts on one port, addressed
  by two sends, binding one `value` whose last writer is open — and it is restated as the two
  outcomes the oracle derives. Cases without `outcomes` are not explored, and nothing changes
  under the default schedule.

- **A `Collections::Set` holds a set.** Where the Kernel Data Type Library declares a collection's `elements` unique and unordered — `Set`, `UniqueCollection`, `Map` — the runtime now holds them as a set value: each member once, `size` counting members, equality that ignores the order the members were written in, and `contains`/`containsAll` as membership. A set consumed by an ordered operation (`collect`, `head`, `#`, a comparison against a sequence, a trace, a write into an `ordered` or `nonunique` feature) enumerates in one canonical order — Booleans, then numbers ascending, strings, quantities, enumeration literals, objects — so equal sets behave alike. What the library declares ordered or nonunique (`Bag`, `List`, `Array`, `OrderedSet`, `OrderedMap`, every `SequenceFunctions` result) is unchanged.
- **Tensor quantities of any rank.** A `TensorMeasurementReference` with three or more `dimensions` builds a rank-three-or-higher tensor whose `#` takes one index per dimension; the wrong number of indexes, an index out of its dimension's range, a non-Integer index, a component count off the flattened size and arithmetic between two shapes are each a typed error naming what was wrong, and the shape survives `+`, `-` and the scalar multiplications.
- **Sets and tensor quantities cross gRPC whole.** `Value` gains a `set` arm (the members as `Value`s, in canonical order, readable in any order, a repeated member refused by the service and by every client) and a `tensor_quantity` arm (`dimensions` and one `Quantity` per row-major component, at any rank), advertised as the `set_values` and `tensor_values` capabilities. The Go, Python, Node, Rust and Java clients decode both to native types that check their own invariants, send them as calc arguments and refuse them to a service that does not advertise the capability; a service withholding one reports the unsupported null it always did. Neither value has an RDF literal form — the mapping writes the model's expressions, which round trip exactly — and neither compiles natively: `sysml -compile` refuses a calc that uses one with a typed error naming the type.

- **A trade study runs as the library writes it.** Running a `TradeStudies::TradeStudy` definition or usage through `-analysis`, `%analysis`, `RunAnalysis` or a sweep over them executes the library's own expressions rather than a special case: the subject binds `studyAlternatives`, the case's `evaluationFunction` binds `tradeStudyObjective.eval` as a function value, `MinimizeObjective`/`MaximizeObjective` compute `best` with `->minimize {in x; eval(x)}`/`->maximize`, the inherited `require constraint { eval(selectedAlternative) == best }` is checked as the objective's own condition, and `selectedAlternative` is the first alternative `->selectOne` finds it holding for. The general rules this needed: a domain library's calc executes from its text as a model's does; an inherited expression reads a feature through the running case's redefinition of it (`studyAlternatives` is the case's `subject`); a requirement usage applies as a predicate with its subject as its first parameter (`tradeStudyObjective(selectedAlternative = a)`); and a redefinition stating no multiplicity inherits the redefined feature's, so `subject : Engine = (a, b);` is `[1..*]`. Every application of the case's calc is reported with the run, in subject order, with its result or error and marked `[selected]` for the one `selectOne` picked and the case returned (never for a result that merely equals an argument) and `[tied]` for a later one scoring the same; an alternative whose evaluation fails, an `evaluationFunction` left without a body and a subject listing no alternative or redeclared `[1]` are typed errors that leave the objective undecided, never a fabricated pick. A run one of whose outputs fails keeps the outputs and evaluations it reached but leaves every objective and assertion undecided for the failure, and each evaluation is reported once per distinct binding of the calc's parameters, its arguments spelled by parameter position. The trace shows each alternative scored and the pick in that order.
- **Case evaluations cross the API.** `RunAnalysisResponse.evaluations` and each `SweepRow.evaluations` carry one `CaseEvaluation` per application — the calc's id, its arguments (an alternative as an `instance_id` resolving in `instances`), the result or an `error`, `selected` and `tied` — under the new `case_evaluations` capability, kept on a failed run beside the outputs and verdicts it reached; `-json` writes the same `evaluations` on each check and sweep row. The Go package exposes them as `Analysis.Evaluations`/`Selected()` (`Evaluation`, its alternatives resolved by `Analysis.Instance`) and a failed run as `*AnalysisError`, whose `Partial` keeps what the run computed; the Python client as `AnalysisResult.evaluations`/`.selected` and `SweepRow.evaluations` (`CaseEvaluation` with `explain()`, and `AnalysisRunError.result` on a failed run); the Node, Java and Rust clients read them from the wire messages, the Rust client through the new `Connection::call`. `%optimize` refuses an objective whose `eval` is bound to the case's own calc — a choice among listed alternatives, not an optimum over a continuous domain — and points at the analysis run.

- **A verification case runs and reports the verdict of its body.** `sysml -analysis`, `%analysis` and the `RunAnalysis` RPC accept a `verification def` or `verification` usage and run it the way they run an analysis case — the same lowering, subject and input binding, and step execution. The `VerdictKind` the body produced is reported: `pass` or `fail` as the library's own `VerificationCases::PassIf` calculation computes it, a `VerdictKind` literal the body binds as it stands, `inconclusive` for a body that produced no verdict value, and `error`, carrying the message, for a body whose run failed.
- **The body verdict is reported beside requirement satisfaction, not instead of it.** `-requirement`, `-satisfy`, `%requirement`, `%satisfy` and the `VerifyRequirement`/`VerifySatisfaction` RPCs add one line per verification case verifying the requirement; what the requirement engine decided, and the exit status, are unchanged. A case performed as a step of another is reported on its own, marked as a subcase, since the library states no roll-up. Over gRPC the verdicts are added fields (`verification_verdicts` on `VerifyRequirementResponse`, `VerifySatisfactionResponse` and `RunAnalysisResponse`), advertised as the `verification_verdicts` capability, and `sysml -json` reports them under `verifications`. Each carries the `requirement_id` it was reported for, matching the `requirement_id` a `satisfy` verdict carries, so a satisfaction response covering several requirements is read per requirement. The Go and Python clients report the verdicts as `Verifications`/`verifications`, giving each satisfaction verdict the cases of its own requirement.

### Changed

- **Compatibility.** 0.6.1 is a patch release under the rule CONTRIBUTING.md § Versioning states.
  Every wire change since v0.6.0 is additive — `make proto-breaking BUF_BREAKING_REF=v0.6.0` passes —
  and no CLI flag, REPL command, RPC or wire field was removed or renamed: `sysml` gains `-schedule`,
  `-sweep`, `-samples` and `-seed`, the REPL gains `%schedule`, `%sweep` and `%samples`, and the
  service gains `RunSweep` and fields advertised under new capabilities. The results that change are
  corrections of results the Kernel Semantic Library derives otherwise: a join, or a node several
  successions reach, fires once per succession; a loop through a merge re-enters it; a breakpoint on
  a synchronized node pauses once; and a via-less `accept` no longer takes a transfer addressed to a
  port. The new execution surfaces — `-schedule` policies with the default order unchanged, so every
  existing result is the same under it, `-schedule explore`, `-sweep`, verification verdicts, and a
  trade study run through the library's own `evaluationFunction` and `MinimizeObjective` expressions
  (`%optimize` remains experimental) — change no result a 0.6.0 model produced. Two changes read as
  compatibility breaks by the letter and are corrections: a collection body whose result type does
  not fit the receiving feature (`accept when counts.{in n : Integer; n}`, `attribute i : Integer =
  xs.{ in x : C; 1.5 }`) is now refused where 0.6.0 let an ill-typed model through, and the golden
  execution traces gained choice-point lines and a step boundary at synchronized joins (one lost a
  duplicated guard evaluation) — debug output only; no `.expected.json` schema and no `-json` report
  shape changed, so a 0.6.0 checkout re-running `-update-traces` sees those lines and nothing else.

- **Pull requests run one CI, GitHub Actions; CircleCI runs on `main` and tags.** The CircleCI `build-test` workflow is filtered to `main`, so a pull request no longer runs the suite twice, and the checks that only CircleCI carried moved into the pull-request workflow: the protobuf lint and wire-compatibility check (against the branch the pull request merges into) join `Go static and integrity checks`, the documentation hygiene checks (`make docs-check`, `make man-check` and the census check) join `Documentation site`, the release-digest check and the check that the committed stubs are what buf generates join each client's job (the Python and Java stub checks were CircleCI-only), and `make conformance` with the `-transport grpc` run join the renamed `Conformance suite` job. Every stub check, in both configs, now also fails when a committed stub was deleted and regeneration brings it back.

- **Before 1.0, the version segment a release bumps is decided by model compatibility.** A release that still accepts every model the previous one accepted, with the same diagnostics and results, and removes or renames no flag, command, RPC or wire field, is a patch release even when it adds features; a release that refuses a previously accepted construct, changes a correctly derived result, removes or renames an interface, or changes a fixture or output format so that existing artifacts fail, is a minor release. CONTRIBUTING.md § Versioning states the rule and the Release Checklist asks for the chosen segment to be justified against it.

- **The documentation now states which orders a behavior leaves open and how each is checked.** Every open ordering in `docs/project/behavior-semantic-oracle.md` names the `outcomes` or `.trace.order` file that encodes it and the run and outcome counts `sysml -schedule explore` reaches — or the single-outcome fixture that pins an ordering nothing observable depends on — and `docs/project/spec-compliance.md` grades choice-point reporting of every kind, `guard-unevaluable` tolerance, the harness's unreachable- and unlisted-outcome checks, the exploration budget and per-driven-run state, with the scheduling-policy row now faithful rather than approximate.
- **The behavior guide has a section on models with more than one valid run.** It walks the three-writers fixture through `%trace on`, `seed:<n>`, `explore` and its `incomplete` status, and writing `outcomes`, `admissible` and `.trace.order` for a conformance case, with the output each command prints; the client chapter spells `schedule=`, `WithSchedule`, `explore_action` and `ExploreAction` per client and says which clients carry no execution surface at all.

- **The SonarCloud findings outside cognitive complexity are cleared again.** Duplicated literals are named constants, same-typed parameters share a declaration, `encodeMember` takes its member head as a struct, marker methods state their contract, unnecessary locals are inlined, the release-gate script reports errors on stderr, the MSI script names its positional parameters, the Java transport catches connect timeouts in their own block, and the Java and Python tests hold one call per exception assertion. The exhaustive switches of the AST codec and the planners' error kinds, and the sealed code-generation IR, are documented exclusions. No behavior changes.

### Fixed

- **A transition's `accept` with no `via` no longer takes a transfer addressed to a port.** An accept naming no port receives as the performer of the machine (SysML v2 §7.16.7), and a port is a sub-occurrence of its part, not the part, so `send new Ping() to alpha.inPort` — or a send routed to `inPort` over a connector — is now taken only by `accept Ping via inPort`; a via-less `accept Ping` on the same state is not enabled by it and no choice point is reported between the two. A transfer addressed to the part itself, `send new Ping() to alpha`, is still taken by the via-less accept and not by the `via` one. The state executor now judges every message by the same rule its dispatch check and the action executor already applied, so the two agree on what a machine can react to; call and change triggers are unaffected.

- **A breakpoint on a node several successions reach pauses once, before its one performance.** A join, or a plain action node two fork branches converge on, stopped a `%continue` once per arriving token and let a resumed run pause at it again. The run now stops there once, when every arrival is in, and resumes past the node; a node a loop re-enters still stops before each pass. A step of the executor now moves each token at most once — a token a step created or moved, a fork's branch, the one a synchronized node performs with or one sent into a nested flow, takes its first step in the next — so traces of forks and joins gain a step boundary between the last arrival and the node's performance, with the same statements in the same order.

- **A collection operation's static type follows what its declaration hands through, not the element type of the collection.** `xs->collect { in x : C; x.mass }` and `xs.{ in x : C; x.mass }` are typed by the body's result (`MassValue`), a nested collect by its innermost body, a body answering a sequence by every element type and `xs->collect f` by the named function's result; `select`, `reject` and `selectOne` keep the elements of `xs`, `reduce` follows its reducer's result — and the element a one-element collection hands back unreduced, unless the collection is known to hold two or more, by its own multiplicity, one it inherits by redefinition, or a chain through such features; a collection holding one at most is never reduced, so its element alone is the result — and `forAll`/`exists` stay `Boolean`, each with the multiplicity the Kernel Function Library declares. A body whose result cannot be typed keeps the library's `Anything`. Value conformance, a feature's bound value, invocation arguments, trigger arguments and enumerated values are judged by the specialized type, so `accept when counts.{in n : Integer; n}` is refused where it was silent, and `when counts.{in n; n > 3}` is accepted where it was refused. A scalar literal a body writes out is as exact as one bound directly: `attribute i : Integer = xs.{ in x : C; 1.5 }` is refused, and a quantity it writes out is measured against the target's dimension: `attribute t : DurationValue = xs.{ in x : C; 5 [m] }` is refused. `xs.?{…}` binds and is typed as `xs->select {…}` is — `(v1, v2).?{ in v : Vehicle; true }` is a `Vehicle` collection, not the `Anything` the sequence is; elements of sibling types share their nearest common supertype, so `(truck, car).?{ in v : Vehicle; true }` is a `Vehicle` collection too — and an element that is itself a collection value binds by the elements it holds, so `attribute i : Integer = xs.{ in x : C; xs.{ in y : C; 1.5 } }` is refused. A collection over `()` or a feature admitting no value keeps its declared type — `()->collect { in a : Integer; "s" }` is a `String` collection — but holds no element, so none is judged and its `reduce` takes nothing from an element it would hand back unreduced; so does one mapping every element to a `[0]` feature or function result — the multiplicity read through an alias, or from the feature or result a redefinition inherits it from — or any operation over such a collection, and an argument holding nothing is judged against no parameter type. An argument or constructor value that is a collection binds each element it holds on its own, so `Sail(vs.{ in v : Vehicle; (v, boat) })` and `new Fleet(vs.{ in v : Vehicle; 1.5 })` are refused by the element that does not bind where they were silent. A collection value binds as many values as it is known to hold, counted against the feature's multiplicity — `part b : Boat[1] = pair.items.{ in v : Vehicle; boat }` binds two — a `reduce` counting the one element it hands back unreduced or what its reducer yields over two or more, so `pair.items->reduce { in a : Vehicle; in b : Vehicle; (a, b) }` binds two and one over `()` none. A body reading the feature it values terminates as a self-referential argument does.

- **A paused debugger run resumes with its own state, not another run's.** An `%action` or `%state` session paused between steps while another run happened on the same session — an `%eval`, a `%calc`, an `%invoke`, a run to completion — resumed with what that run left behind: its step and element budget replaced the paused run's (so a step that fit the budget before could fail with the other run's spending, or a run could spend past its bound), its choice points and unevaluable guards stood in for the paused run's, and the calc usage evaluations of an activation paused at a breakpoint were dropped, so the next read ran the body again. Each run now keeps a state of its own — budget spent, notes, scheduler and calc usage memo — that every call into a driven executor installs, so the paused run continues where it left off and the run in between spends and notes on its own; the executor reports its own notes and the REPL's choice summary counts from them. The scheduler was already the run's own; the other three fields join it.

- **A node several successions reach is performed once, after one token has arrived over each of them.** A token now records the succession it travelled (`Token.Via`, the lowered `ActionEdge` with its `Source`), and a join — or a plain action node two or more successions reach — fires when every incoming succession has delivered one token, the arrivals collapsing into the one token that performs it; a second token over an already-delivered succession waits for the next firing instead of standing in for another succession, and a join one of whose successions no token can travel deadlocks (`ErrActionDeadlock`) rather than firing on a token count. A plain node in a loop or behind a decision still re-performs once per pass: it awaits a succession only while some token can still reach its source before the node performs. `action_join_one_token_per_incoming_succession` (`log = 12`) and `action_node_with_two_incoming_successions_runs_once` (`hits = 1`) leave the known failures; the REPL's `%tokens` says which succession a held token arrived over and which it awaits.

- **`make proto-breaking` works from a blobless checkout.** The wire-compatibility check now compares against a `git archive` of `api/proto` at the baseline ref (`BUF_BREAKING_REF`, `origin/main` by default) instead of pointing buf at the `.git` directory, which buf clones; a partial clone cannot serve that clone every object it needs, so from CircleCI's checkout the check failed with `could not fetch … from promisor remote` on any branch other than `main`.
- **The CircleCI jobs carry readable names.** The status checks now read `ci/circleci: Build and test`, `Python client tests`, `Rust client tests`, `Java client tests`, `Node client tests` and `SonarCloud scan`, with the release-workflow jobs named the same way, rather than the job keys.

- **The CircleCI suite on `main` and on tags finishes within the plan's 60-minute job limit.** The single `Build and test` job, which had grown to about an hour and timed out on most merges, is now four parallel jobs — `Go static checks`, `Go race tests`, `Go coverage profile` and `Go gates and binaries` — each well under the limit, so the status checks now read `ci/circleci: Go static checks` and so on. The client tests, the SonarCloud scan and every release workflow wait on all four, so nothing is scanned or published unless the whole suite passed.
- **The wire-compatibility check on `main` compares against the merge's own parent, and a release tag against the previous release.** The check used to fetch `origin/main` at run time, so a merge landing while the build ran made its own added fields read as deletions in an unrelated build; the baseline is now an ancestor of the commit being built, so the verdict cannot change with what merged afterwards. `make proto-breaking` still defaults to `origin/main` locally and honours `BUF_BREAKING_REF`.

- **A merge node passes every token that reaches it, so a loop through a merge runs to its exit.** The action executor used to record one traversal per merge for the whole run and retire every later arrival, so the specification's `ChargeBattery` loop stopped after one pass (`level = 50`, `passes = 1`) without ever performing `endCharging`. A merge is now one `MergePerformance` per arrival, as `Actions::MergeAction` declares: its body runs and the token is forwarded on every traversal, a loop re-enters it as often as its guard sends the token back (`action_merge_loop_reenters` leaves the known failures with `level = 100`, `passes = 3`), and a fork whose branches both reach a merge yields one downstream token per branch, as two merge performances do — collapsing them is a join's job. A merge's body now runs before the guard on its outgoing succession is read, as every other node's does, so a write in the merge's body decides its own guard and an arrival the guard turns away still performs the merge (`f63_merge_body_runs_on_traversal` counts both arrivals: `mergeRuns = 2`, `passed = 2`). A merge is also the one multi-incoming node the join synchronization does not wait at. Termination of a loop with no exit is the action step budget's job as before: `ErrActionStepLimitExceeded`, under `RunToCompletion` and under the REPL's `%continue` alike.

- **The Windows MSI builds again on the GitHub runners.** `scripts/build-msi.sh` read the
  command that runs `wix` from the `WIX` environment variable, which the preinstalled WiX v3
  on `windows-latest` already exports as its installation directory
  (`C:\Program Files (x86)\WiX Toolset v3.14\`), so the `msi` job of the v0.6.0 release failed
  with `error: C:\Program is required` and no `opensysml-0.6.0-windows-amd64.msi` was published.
  The override is now `WIX_CMD`.

- **The test-suite figures the documentation repeats agree again and match a real run.** `README.md`, `docs/project/spec-compliance.md`, `docs/project/roadmap.md` and `docs/project/training-examples.md` now all state the same counts from one `go test -v ./...` run: 671 execution conformance cases, 140 golden execution traces, 336 runtime robustness cases, 195 golden AST fixtures, 249 first-level `TestNegative` cases, 15 gRPC conformance and 8 gRPC robustness cases, and 15,139 tests and subtests, with the environment each skip depends on named. The roadmap's gate table reads the same commit and its census, rejection-oracle and RDF round-trip rows follow the committed baselines.
- **The saving-and-RDF guide shows what the binary prints.** Every model under `examples/` now converts, so the guide no longer presents `parser_features_demo_declarations.kerml` as refused; the refusal example is a name shared by two members of one namespace, which is what the mapping still refuses, and the byte counts in the transcripts are the current output. The `README.md` conversion row states the round-trip figure rather than a stale fraction.
- **The Python client test snippet in the release procedure installs `psutil`.** The lifecycle tests import it, and it is a development extra rather than a dependency of the client, so `pip install pytest pytest-mock` alone left the suite unable to import.

- **The Java client reads a whole unit scale without going through `BigDecimal`.** `Value.sameValue` compares quantities exactly over integer magnitudes and whole scales; the scale double is now converted to its integer directly instead of through `new BigDecimal(double)`. The integer is the one the double is, the same value the service and the other clients compute, so no comparison, membership or set-equality result changes.
- **Tests that could not fail now test what they name.** A Java test that compared a `SetValue` with a `Sequence` through `assertNotEquals`, which no two such values could ever satisfy, now asserts that the order of a sequence tells it apart from a set through `sameValue`; two Python assertions that compared an expression with itself now compare independently built measurement references. The Java exactness test also pins whole scales beyond a `long`.
- **The SonarCloud findings outside cognitive complexity are cleared again.** Duplicated Go literals are named constants, intentional no-op closures state their contract, a negated comparison is written directly, an underscore-suffixed local is renamed, unnecessary locals are inlined, the Java transport tells a connect timeout from a read timeout in one catch block, `exactBaseMagnitude` returns an empty array instead of `null`, `valueHash` lives in `SetValue`, and the Java and Python tests hold one call per exception assertion, one property per assertion, fewer than 25 assertions per method and a single argument order. No behavior changes.

### Performance

- **Loading a model no longer merges the library's visible member set once per declaration.** The inherited-name conflict rule looks each name up in the memoized member maps of a declaration's library bases and passed-through types instead of copying them into a fresh map per part, attribute, action and state; its diagnostics are unchanged. The OOSEM method rule memoizes a type's classification, so an attribute type shared by many features is conformance-checked once. Loading and validating a 4 000-element model is 15% faster and allocates a third fewer bytes than before; `sysml -validate` on 3 000–12 000-element models is now at or ahead of release 0.4.2. `docs/project/performance-release-0.6-vs-0.4.2.md` records the comparison, the remaining costs of the validation rules added since 0.4.2, and how to repeat it. The Apollo 11 load figure on the landing page and in `docs/internals/performance.md` is re-measured at 0.43 s: the earlier 0.37 s was taken while the model's three calculation-arity findings were still errors, before the higher validation tiers ran.

## 0.6.0 — 2026-09-07

### Added

- **An analysis case runs.** An `analysis` definition or usage is the calculation it is: `-analysis
  "Pkg::Case[(args)] [object]"` and `%analysis` run it and print its `out` and `return` values with
  their units, then the verdict of its `objective` and of every `assert constraint` in its body —
  `satisfied`, `not satisfied` with the condition that failed, or `undecided` with the reason — and
  exit 0, 1 or 2 accordingly. The `subject` is an `in` parameter: a usage binding it (`subject s =
  ship;`) needs nothing more, a definition or an unbinding usage takes the object the run names (one
  `-instantiate`/`%instantiate` created) and a case nested in another runs on the enclosing case's
  subject; a case run with no subject is refused naming it rather than run empty. Arguments bind the
  other `in` parameters positionally or by name, as `-calc` takes them. The body's `action`,
  `perform` and nested `analysis` steps run through the action executor as one flow over the
  successions they state (`then`, `first`, forks, joins, decisions and merges) or in declaration
  order where they state none, each a subperformance whose outputs later steps and the case's
  outputs read by `step.pin`; a step that fails, a body that deadlocks or exceeds its step budget, a
  case that runs itself and an `in` parameter left without a value are typed refusals naming the
  case. Reading an analysis usage's output as a feature — `An::shipCost.total` on a package-level
  usage, `holder.inner.total` on a usage a part owns, `attribute :>> x = a.result;` — runs the case
  the way a `calc` usage's output does, memoized until a value it read changes. The gRPC service
  gains `RunAnalysis` (symbol, optional subject, positional and named arguments; outputs, verdicts,
  the subject's instances, and a typed `failure_reason`), the Connect adapter and the Go client gain
  `RunAnalysis`, and the Python client gains `Model.run_analysis` answering an `AnalysisResult`.
  `-calc`/`%calc`/`EvaluateCalc` still refuse an analysis by kind and now say to run it as one;
  `%optimize` is unchanged. A verification case body shares the grammar and lowers the same way but
  is not yet run: its verdict stays what `-requirement`/`-satisfy` compute.

- **A binding connector's ends are connector ends, so they may be named.** `bind e1 ::> a =
  e2 references b;` and KerML `binding of e1 ::> a = e2 references b;` declare two end features
  `e1` and `e2` owned by the binding, each reference-subsetting the feature it binds, exactly as
  `succession first s1 ::> a then s2 ::> b;` and `connect c1 ::> a to c2 ::> b;` already did.
  `e1` was read as the binding's own name and `bind e3 ::> a = b;` dropped `e3` on the floor;
  both are now end names that hover, go-to-definition and rename find, and the semantic binding
  still joins the two referenced features. The RDF graph states both ends as `sysx:relatedFeature`
  end nodes carrying `sysx:endName`, `sysx:endIndex` and each end's multiplicity, instead of
  putting the second end in `sysml:value`; a graph stripped of its source text writes back to the
  same notation, spelling a named end `::>` unless `sysx:endReferencesKeyword` records the word
  `references`. An end with two names, a named end with no referenced feature, or a binding with
  fewer than two end nodes is refused by name rather than failing as a notation error.

- **Six more KerML structural rules are checked the way the reference does.** A binding
  connector whose effective ends — declared, positional, bound `references` targets and
  inherited alike — are not exactly two reports `Binding connector must be binary`. A feature
  chain whose later link is not a feature of the type the earlier link reaches, written in a
  usage header, a `references`, a connector end or a flow, reports it as not featured within
  that type, aliases and inherited members followed. An annotating element that annotates
  itself reports `Must own its annotating element`. An `end` feature with a direction reports
  `End feature cannot have direction`, and one that is derived, abstract, variation, composite
  or portion reports `End feature cannot be derived, abstract, composite or portion`. A
  conjugated classifier, which owns no specialization of its own, must still reach its kind's
  default supertype through the type it conjugates (`Must directly or indirectly specialize
  Objects::Object`, say), and a conjugated feature that reaches no type at all reports
  `Features must have at least one type`. Anonymous usages now keep their `abstract`,
  `variation`, `ref`, `derived`, `constant`, `var` and portion modifiers, so these checks see
  them.

- **A coordinate frame and a measurement scale are runtime values.** A usage typed `CoordinateFrame` with its `:>> mRefs` (Annex A's `spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }`) evaluates to the frame it declares, carrying its dimensions, one measurement reference per axis and the transformation it states; `MeasurementRefCalculations::'CoordinateFrame*'`/`'CoordinateFrame/'` and the `*`/`/` operators compose every axis with a unit (`velocityCF = spatialCF / s`), `VectorCalculations::'['` builds a vector quantity over a frame (`(1.0, 2.0, 3.0) [spatialCF]`) whose `mRef` is the frame itself, and a frame conforms to its declared type and its generals in the checker and the runtime alike. A measurement scale is the one-axis case: `SI::'°C_abs'`, `Time::UTC` and a model's `TimeScale` carry their unit, mapping and placement, a quantity on one (`21.5 [SI::'°C_abs']`) reads, and `ConvertQuantity` converts through the scale's placement (`ConvertQuantity(300.0 [K], SI::'°C_abs')` is `26.85 ['°C_abs']`, and back), refusing with a typed error a scale the library places nowhere (`Time::UTC`), a mapping that disagrees with the placement, or an origin that is not a quantity. `VectorCalculations::transform` re-expresses a vector over a transformation's source in its target for a `CoordinateFramePlacement` (the inverse of the stated placement, basis directions normalized), a `TranslationRotationSequence` (angles converted through their unit; intrinsic rotations about the moved axes), an `AffineTransformationMatrix3d` and a `NullTransformation`, and names any other transformation type, a vector over the wrong frame, or incommensurable components. Frames and transformations compare and hash, describe, trace, render in the REPL and `%features`, are refused by the solver, traversed by document queries, carried across re-analysis, and cross gRPC as an unsupported null naming them: the wire has no arm for a frame yet.

- **A derived `=` value follows the features it read.** `attribute a : Integer default 3;
  attribute d : Integer = a * 2;` read `d` as `6` once and kept it after `a` was assigned `9`;
  the runtime now records what a `=` value reads while it is derived and, when one of those
  features changes — an `assign` in an action or state body, a `SetFeatureValue` from the REPL or
  the gRPC service, a binding propagating a new value, a classifier's subsetter superseding a
  `default null` collection a roll-up had summed while empty — drops the derived value so the
  next read derives it again, transitively through the values that read it. Nothing is
  recomputed before it is read, a value a run assigned keeps the assignment, a probe or
  transaction that wrote such a feature is rolled back with the values that read it, and a value
  derived from itself is still `ErrCyclicFeatureValue`. The `in` parameters of a calc or action
  usage are unchanged: bound once per invocation, they stay bound while its outputs are read.

- **Queries project `shortName`, `declaredShortName` and `documentation`.** The three KerML `Element` features join the fixed property set beside `name` and `declaredName`: `Project`, `OrderBy`, `WhereFeature`, `Column` expressions (`Element::shortName`, `Element::documentation`), the gRPC/OSLC query surfaces and reflective access all accept them. `shortName` is the effective short name (`<'HLR-R001'>`), following redefinitions the way `name` does; `documentation` is the body of each `doc` comment in declaration order with its delimiters, indentation and the `*` margin a block comment runs down its left edge removed (a `*` the author wrote — emphasis, a bullet — stays) — the same normalization the LSP hover shows — so an element with two bodies projects two values in the cell. An element without a short name or without documentation projects an absent cell, as any missing property does.
- **Documents render query rows as prose with the `Definitions` content kind.** A `part … : Definitions` names a `term` and a `description` column of the query bound by its nested `calc`, and each result row becomes one entry: `**HLR-R001** — The mission shall …` in Markdown (and so in PDF), a `<dl class="sysml-definitions">` of `<dt>`/`<dd>` pairs carrying the row's element in HTML. A column the query does not project is a typed `unknown-definition-column` error at planning time when the projection is statically known, otherwise at evaluation; a missing `term`/`description` or a `Definitions` without a query is a typed planning error. `docs/manual/authoring.md` documents the construct and `docs/manual/examples/requirements.sysml` renders requirement identifiers and doc text both as a table and as prose.

- **End features, return parameters and conjugation are checked the way the reference does.** An
  end feature none of whose multiplicities — its own, or one it takes through subsetting,
  redefinition, typing, a reference, a feature chain or an implicit end — is exactly `1..1` is
  reported with the warning `End feature must have multiplicity 1`; a SysML end usage defaults to
  `1..1`, so only a declared own multiplicity other than `[1]` warns there, and the multiplicity
  written between `end` and the keyword (`end [0..*] item x : A`) belongs to the cross feature
  and is silent. A `return` parameter whose owner is no function, expression, calculation,
  constraint, requirement or case is the error `Return parameter membership not allowed`, and a
  type that declares a second conjugation (`classifier C ~A ~B;`) reports `Cannot have more than
  one conjugator` on each `~` past the first.
- **The multiplicity between `end` and the keyword is the cross feature's.** `end [m] item x : A`
  now declares an anonymous cross feature carrying `[m]`, as the grammar reads it, instead of
  copying `[m]` onto the end itself; an end that also declares its own `[n]` keeps both, and
  the multiplicity a query, type fact or RDF export reports for the end is its own. The RDF
  mapping writes the cross feature as a feature the end owns through an `OwningMembership`,
  with its own bounds and specializations, and reads it back. An end that declares its cross
  feature this way and also `crosses` another feature is reported `Must be the cross feature`,
  as the reference does.

- **`-html-mermaid` has an HTML document load Mermaid to draw its diagrams.** `-html-mermaid cdn` adds one `<script>` before `</body>` loading a pinned Mermaid release from jsDelivr, and `-html-mermaid <url>` loads the script from a URL of your own; `-render-documents` puts it on every page of the set. Diagram blocks keep their `<pre class="mermaid">` source, so a page still reads where the script cannot load, and the default output is unchanged — no script, no network reference. The option is HTML-only and is refused with `-html-fragment`, since a fragment has no page shell for the script.

- **`-html-theme` styles an HTML document with a bundled theme.** `modern` (clean corporate sans-serif with filled table headers and zebra rows), `report` (serif technical report with open, ruled tables and captions above) and `print` (monochrome, compact, page-break aware) are layered over the default stylesheet inside the same `opensysml` cascade layer, so unlayered `-html-css` sheets still win over both; `default` names the default sheet alone, which stays the default output. A `-render-documents` set writes the themed sheet as its shared `sysml-document.css`, and `-html-default-css -html-theme <name>` writes a theme's whole sheet to start from. The option is HTML-only and is refused with `-html-fragment` and `-html-no-default-css`.

- **A named measurement reference answers its declaration's members.** `SI::km.unitConversion.conversionFactor` is `1000.0`, `km.unitConversion.referenceUnit` is `m` (so `ConvertQuantity(3 [km], km.unitConversion.referenceUnit)` is `3000.0 [m]`), `km.unitConversion.isExact` is `true`, `m.quantityDimension.quantityPowerFactors#(1).exponent` is `1`, `m.unitPowerFactors#(1).unit` is `m` and `K.definitionalQuantityValues#(1).num` is `[273.16]` (`DefinitionalQuantityValue::num` is `Number[1..*]`): a reference naming a declaration reads the members it does not carry itself from the object that declaration materializes as — the one `%features SI::km` shows, kept through REPL re-analysis — with the library's redefinitions and defaults followed. A unit composed at runtime (`m / s`) names no declaration, so those members stay a typed error naming `MeasurementReferences::DerivedUnit` and the reduction. Which library attribute definitions are held as values and which materialize as objects is now decided by specialization (a scalar, an enumeration, a `TensorQuantityValue` or a `TensorMeasurementReference` — frames and scales included — is a value; `UnitConversion`, `UnitPrefix`, `QuantityDimension`, `QuantityPowerFactor`, `UnitPowerFactor`, `DefinitionalQuantityValue`, `QuantityValueMapping` are records), so a model's own `attribute myConv : ConversionByPrefix { :>> prefix = kilo; :>> referenceUnit = m; }` lists and answers `conversionFactor = 1000.0`, and a model's own `TensorMeasurementReference` usage that does not restate `isBound` answers the inherited `default false` instead of "member isBound not found".

- **A measurement reference is a runtime value.** `SI::m`, `SI::'m/s'` and `MeasurementReferences::one` evaluate to a measurement reference carrying the unit's spelling, the declarations it is composed of and its reduction to base units; `m * s`, `m / s` and `m ** 2` compose one, and two references are equal when they reduce alike (`SI::'m/s' == m / s`). `QuantityCalculations::'['(3.0, m)` builds `3.0 [m]`, `ConvertQuantity(3 [km], m)` is `3000.0 [m]` (incommensurable units stay `ErrIncommensurableUnits`), a quantity's `num` and `mRef` read (`q.mRef == SI::m`, a vector quantity's `mRef` when its axes share one unit), and `MeasurementRefCalculations::'*'`, `'/'`, `'**'`, `'^'` and `ToString` compute. A reference conforms to the unit definition of its dimension in the checker and the runtime alike (`m * m` to `AreaUnit` and to `DerivedUnit`, not to `LengthUnit`; `m` to no quantity value type; no unit to a measurement scale such as `Time::TimeScale`), is carried across re-analysis, refused by the solver, bound by document queries as a single unit, and crosses gRPC as an unsupported null naming it. Arithmetic the library does not declare (`m * 3`, `m + m`) is a type mismatch, and the declaration's own members (`m.quantityDimension`) report themselves by name, as does a measurement scale (`Time::UTC`, `SI::'°C_abs'`): the runtime holds a unit and its reduction, not a scale's origin, points or mapping. Coordinate frames (`VectorCalculations::'['`, `transform`, the `CoordinateFrame` operators) and tensors (`outer`, `TensorCalculations`) remain unevaluable by name: the runtime holds no frame or tensor value and the library gives them no bodies.

- **A bare measurement reference crosses the gRPC/Connect API whole, in both directions.** `Value` gains a `measurement_ref` arm carrying what the runtime value carries: the unit as written, its reduced unit term (required wherever the unit names one, the rule `Quantity` already follows), and `unit_id`, the fully qualified name of the declaration a named unit is (`SI::metre` for `m` and for the alias `SI::m`) — omitted for a composed unit such as `m / s`, which names no one declaration, and never to be fabricated by a client. Where the service used to answer `unsupported: measurement reference m` for `SI::m`, `m / s` or a quantity's `mRef`, it now sends the reference, and one supplied as an action input or calc argument decodes to the same runtime value, so `ConvertQuantity(q, ref)` converts through it; a malformed one — no unit and no id, a named unit without its reduction, an id naming nothing or something that is not a unit, or a reduction that disagrees with the declaration's own — is a typed error, never a value of another shape. The service advertises this as the `measurement_refs` capability, separate from `structured_values` so a client built against the three structured arms keeps reading a bare reference as the unsupported null it read before; a service withholding it keeps reporting that null and refuses a reference sent to it with `UNIMPLEMENTED`, and every shipped client checks the list before sending one. The Go, Python, Node, Java and Rust clients map the arm to native types — `opensysml.MeasurementRef`, the `MeasurementRef` dataclass over `Unit`, `{ kind: "measurementRef" }`, `Value.MeasurementRefValue`, `Value::MeasurementRef` — that check the same invariants, and the conformance suite exercises it over gRPC, Connect protobuf and Connect JSON.

- `OOSEM` library under `OpenSysML Libraries`: a non-normative SysML v2 vocabulary for the Object-Oriented Systems Engineering Method — definitions, base usages and semantic-metadata keywords for the enterprise, stakeholders, the four requirement levels, the system context (system of interest, external systems, users, environment), system and enterprise use cases, I/O entities and stores, logical and physical components, nodes, as-is/to-be marking and `OOSEMPackage` classification — re-exporting `ParametersOfInterestMetadata`, `TradeStudies`, `RequirementDerivation` and `CauseAndEffect`. Worked example under `examples/oosem-demo/`; design record in `docs/project/oosem-library.md`.

- `OOSEM` viewpoints and view definitions (`EnterpriseModelView`, `SystemContextView`, `SystemUseCaseView`, `RequirementsView`, `MeasuresView`, `LogicalArchitectureView`, `LogicalScenarioView`, `PhysicalArchitectureView`) that specialise the standard view definitions with the method's filters and renderings, so a model writes only `view v : RequirementsView { expose P::*; }`.
- OOSEM method checks (`oosem-requirement-not-derived`, `oosem-requirement-not-satisfied`, `oosem-logical-component-not-allocated`, `oosem-use-case-subject`): constraint-tier warnings that a requirement derives from the level above, is satisfied, that a logical component is allocated, and that a use case's subject is the system context or enterprise — each rule waits until the model has the level it traces to.

- **Three operator-expression checks of the reference validator.** The type tier now warns, as the OMG pilot does, when a KerML document indexes with `x[i]` instead of `x#(i)` (`bracket-operator`), when a cast `x as T` names a target unrelated to every type of its argument (`cast-conformance`), and when the unit of a quantity `10 [u]` is not a measurement reference — a number, a String, a quantity value, a dimensionless computation or an untyped feature (`quantity-unit`); arithmetic over units, a frame's `mRefs#(i)`, an aliased or feature-held unit stay silent.

- **Queries and documents evaluate attribute values written over other features.** `attribute :>> mass = dryMass + propellantMass;`, `attribute :>> powerLoad = commandModule.powerLoad + serviceModule.powerLoad;` and `attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);` — the shape a mass or power budget takes — now project, filter (`WhereFeature`), sort (`OrderBy`) and take part in `Column` arithmetic, and so reach `-run-query` and document `Table`, `List` and `Definitions` content in Markdown, HTML and PDF. A value the analyser cannot fold statically is evaluated by the runtime as seen from the row's element, so each leaf reads through that carrier's redefinition chain (type-level and usage-level `:>>`, `default` values, feature chains into owned parts) with the runtime's own arithmetic, unit conversion and library functions (`sum`, `size`, `#`, `->collect`): units are kept and `Integer` stays `Integer`. A leaf nothing binds makes the value absent — an empty cell — as a value-less feature already was; a value that depends on the model running (a calculation's `in` parameter, an action's state), a cycle, operands of different dimensions or a result no cell can hold (a part) is a typed `unevaluable-feature` error naming the query, the property, the row element and the runtime's reason, as `docs/manual/query-cookbook.md` now describes under "Derived values". One shape stays a typed error in the query as in the REPL: `sum` over an empty collection is the dimensionless `0.0`, so `mass + sum(subcomponents.totalMass)` on a component with no subcomponents reports incommensurable units. An expression over a feature holding no value reports which feature holds none (`NoValueError`) rather than a type mismatch.

- **RDF mapping: an anonymous `feature`, `event`, `snapshot`, `timeslice` or `assert` declaration is carried structurally instead of refused.** `sysml -convert ttl` used to refuse a synonym, portion, event or assertion keyword on a declaration with no name of its own (`feature :>> x;`, `snapshot :>> start { … }`, `event m.start;`, `assert c { … }`), because the notation could not be rebuilt from the graph without coming back as a different declaration. The graph now types the fact the keyword states — `sysml:portionKind` for `snapshot`/`timeslice`, the metaclasses `sysml:EventOccurrenceUsage` and `sysml:AssertConstraintUsage` (with `sysml:isNegated`) for `event` and `assert`, the occurrence or constraint they name as `sysml:references` — and records `sysx:declaredKeyword` on an anonymous declaration too, so the decoder spells the head from the typed facts and only picks the spelling from the keyword. KerML's `feature`, which no typed fact distinguishes from `attribute`, is carried by the keyword alone. A graph whose keyword contradicts its typing (`snapshot` with no or another `sysml:portionKind`, `event` on a `sysml:PartUsage`, an `AssertConstraintUsage` with another `sysx:declaredPrefix`) or a reference-taking keyword (`perform`, `event`, `assert`) with neither a name nor a `sysml:references` is refused naming the element. The 40 corpus files this refused move to `stable`, so every one of the 346 models under `examples/` now round-trips.

- **Arrays, vectors and vector quantities cross the gRPC/Connect API whole, in both directions.** `Value` gains three arms: `array` carries the dimensions and the elements flattened in row-major order (each element a `Value`, so arrays of quantities or of arrays nest), `vector` carries the numeric components as `Value`s so an Integer and a Real component stay distinct, and `vector_quantity` carries one `Quantity` per component with its magnitude, unit as written and reduced unit term, so a composed unit such as `m/s` and per-component units survive. Where the service used to answer `unsupported: array …`, it now sends the value, and one supplied as an action input or calc argument decodes to the same runtime value; a malformed one — an extent that is not positive, elements that do not fill the dimensions, a non-numeric vector component, an empty vector quantity, a unit without its reduction — is a typed error, never a value of another shape. The service advertises this as the `structured_values` capability; a service withholding it keeps reporting the unsupported null and refuses a structured input with `UNIMPLEMENTED`, and every shipped client checks the list before sending one. The Go, Python, Node, Java and Rust clients map the arms to native types — `opensysml.Array`/`Vector`/`VectorQuantity`, the `Array`/`Vector`/`VectorQuantity` dataclasses, `{ kind: "array" | "vector" | "vectorQuantity" }`, `Value.ArrayValue`/`VectorValue`/`VectorQuantityValue`, `Value::Array`/`Vector`/`VectorQuantity` — that check the same invariants, and the conformance suite exercises the three over gRPC, Connect protobuf and Connect JSON.

- **SysML v1 models exported from Cameo/MagicDraw migrate to v2.** `sysml Model.xmi -convert sysml` (or `ttl`) reads UML 2.5 XMI with the SysML profile applied, or a `.mdzip` archive, and writes SysML v2 notation: packages, blocks, value types, enumerations, properties with multiplicities and defaults, generalization and redefinition, interface blocks and ports, connectors, binding connectors and item flows, requirements with satisfy/verify/derive, constraint blocks, instance specifications, allocations, comments and custom stereotype tags. Behaviors, operations and units are not migrated yet. Every element is accounted for in a migration report — mapped, approximated, unmapped or skipped — written with `-migration-report FILE` (JSON by `.json` extension) and summarized on stderr otherwise; unmapped elements are left as comments where they would have gone. The gRPC `Convert` accepts `from_format: "xmi"` too. See `docs/reference/sysml-v1-migration.md`.
- **The SysML v1 migration is marked experimental.** Like RDF conversion, every run says so: `sysml -convert` from XMI prints a `note:` on stderr, the gRPC `ConvertResponse` sets `experimental` with the migration's own `experimental_notice` (both notices, migration then RDF, when converting XMI to Turtle), the Python client warns with `ExperimentalFeatureWarning` and `is_experimental` counts `xmi`/`mdzip` input, and the wording lives once in `export.MigrationNotice`. See `docs/reference/sysml-v1-migration.md` § Status.
- **The migration writes valid notation for Cameo shapes v2 cannot spell.** Two members sharing a name, a connection end named like a participant property, an anonymous property with no migrated type, string multiplicity bounds and `NaN` reals are each renamed, dropped or commented with an approximation in the report instead of stopping the conversion. The parser reads `ref x default = 4;` (a default straight after a modifier-only usage's name), and adding triples to an indexed RDF graph keeps the index instead of rebuilding it, so converting a large model to Turtle takes seconds rather than minutes. Found by migrating the OpenMBEE TMT model.

- **A tensor quantity is a runtime value.** `TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef)` over a model-declared `TensorMeasurementReference` (`:>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa);`) builds `Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]`: one magnitude and one measurement reference per component in row-major order, `dimensions`, `order`, `flattenedSize`, `elements`, `num`, `isBound` and `mRef` reading, and `#` indexing it as an `Array` (`t#(2, 1)` is `3.0 [Pa]`). The elements must number `mRef.flattenedSize`, and one reference does not broadcast to four components (`mRefs` redefines `Array::elements`, so it must fill the dimensions); both are `ErrMultiplicityViolation` naming the count and the shape. `'+'` and `'-'` (and the operators) are componentwise over two tensors of one shape, each right component converted into the left's unit (`ErrIncommensurableUnits` otherwise, a shape mismatch naming both shapes); `scalarTensorMult`, `TensorScalarMult`, `scalarQuantityTensorMult` and `TensorScalarQuantityMult` scale every component, the quantity forms composing each component's unit with the scalar's (`Pa*m`); `isZeroTensorQuantity` holds when every magnitude is zero; `isUnitTensorQuantity` decides a square order-two tensor against the identity and reports any other shape as unevaluable naming the shape it needs. Every one of these accepts a scalar or vector quantity where it declares a `TensorQuantityValue` and answers the operands' rank. A tensor conforms to `TensorQuantityValue` and `Collections::Array` in the checker and the runtime alike, is described, traced, compared and hashed by content, rendered by `%eval` and `%features`, carried across re-analysis with every component's unit rebound, refused by the solver and as a document-query binding, and crosses gRPC as an unsupported null naming it. `contravariantOrder` and `covariantOrder` are never fabricated: with no default in the library and none set by `'['`, reading one reports the `orderSum` constraint that alone binds them. `tensorVectorMult`, `vectorTensorMult` and `tensorTensorMult` stay unevaluable by name — the library states no contraction convention — as do `VectorCalculations::outer`, whose declared `VectorQuantityValue` return no outer product inhabits (drafted in `docs/project/omg-issues.md`), and `TensorCalculations::transform`, which needs a coordinate frame the runtime does not hold.

- **A composite `variant port` under a `variation port` owned by a port definition or port usage is reported.** A variant has no owning type, so it is not a subport and must be referential (`A port usage must be referential.`), as the pilot reports; write `variant ref port a : PD;`. The parser now also accepts the usage prefix after `variant` (`variant ref port`, `variant in port`, `variant end port`, `variant snapshot part`) that the pilot grammar admits and that previously failed to parse.

- **Windows releases ship an installer, `opensysml-<x.y.z>-windows-amd64.msi`.** A WiX v5 MSI
  (`packaging/msi`, `scripts/build-msi.sh`) installs `sysml.exe`, `sysml-lsp.exe` and
  `sysml-grpc.exe` to `Program Files\OpenSysML` on the system `PATH`, upgrades an older
  install in place, and offers the Z3 SMT solver (5.1.0, pinned by SHA256 in
  `packaging/msi/z3.pin`, MIT notice included) as an optional feature under `z3\` on `PATH`,
  so `%check`/`%explain` find a solver without configuration. The release workflow publishes it
  unsigned with `SHA256SUMS-windows-msi.txt`, or — once SignPath is configured — built from the
  signed executables and itself signed as `*-signed.msi` (the bundled `z3.exe` is never
  signed). The install guide recommends the MSI on Windows.
- **Scoop, winget and MSYS2 manifests are maintained in-repo as templates.**
  `packaging/scoop`, `packaging/winget` and `packaging/msys2` hold manifests that depend on the
  package manager's Z3 instead of bundling it, rendered from a release by
  `scripts/render-scoop-manifest.sh`, `scripts/render-winget-manifests.sh` and
  `scripts/render-msys2-pkgbuild.sh` like the Homebrew formula; each README documents how a
  maintainer submits to the external bucket or repository. Nothing is submitted automatically.

### Changed

- **An invocation that leaves a required parameter unbound is an advisory, not an error.** `F(1)` against `calc def F { in x; in y; }`, `F(y = 2)` or `F()` now report the warning `F leaves parameter y unbound, so the call cannot be evaluated` (code `unbound-parameter`), once per omitted default-less input, in every conformance mode including `-strict`; `-validate` exits 0 on such a model. KerML lists no constraint on the count of an invocation's arguments and the reference implementation validates and evaluates every omission form clean, so the expression is well formed — only its call cannot be evaluated, which the runtime still refuses with the same unbound-parameter error. The policy is the same at a bare call and at an invocation heading a feature chain: `A().y` now carries the advisory where it was silently accepted, and `A()` the advisory where it was an error. Too many arguments, a name no parameter carries, a parameter bound twice and an argument of the wrong type stay errors; a parameter a `default` reaches or whose multiplicity admits no value draws nothing.

- **Specialization and binding-conformance validation follows the reference more closely.** A
  class, data type or SysML definition specializing the wrong classifier family is reported
  through `:>` as well as `specializes`, in SysML with the reference's wording (`Cannot specialize
  attribute definition`, `Cannot specialize item definition`) in place of the former kind-mismatch
  message; a conjugated feature at the specific end of a standalone `specialization subset`,
  `redefinition` or `typing` is reported like a conjugated subclassifier. `Bound features should
  have conforming types` now also covers the bindings the language implies — a result expression
  against its result parameter, a `satisfy … by` operand and a nested requirement's or case's
  subject against the subject they fill. An invocation argument that corresponds to no input
  parameter is headed by the reference's `Must correspond to one input parameter of the invoked
  type`, at the argument itself.

- **An `%optimize` objective states the value to improve by redefining the trade-study library's `eval` calculation, not by rebinding its `best`.** Write `objective o : MinimizeObjective { subject :>> selectedAlternative; in calc :>> eval { expression } }` (or `{ return :>> result = expression; }`, which a redeclaring objective must use): `TradeStudies::TradeStudyObjective` declares `eval` as its extension point and derives `best` from it, so the pinned OMG pilot accepts the spelling silently, where the earlier `attribute :>> best = expression;` overrode a bound feature value (`Cannot override a binding feature value`, the `feature-value-overriding` diagnostic OpenSysML reports too). The runtime reads the objective's value from the lowered body of its `eval`; the solver's direction, conditions, lexicographic order, quantities and the equality between an inherited condition's `best` and the value improved are unchanged, and `%optimize` reports the same optima on the shipped demos. An objective that still rebinds `best` keeps its validation diagnostic and is refused by `%optimize` with a message pointing at the `eval` spelling, and an `eval` computing in steps rather than stating one expression is refused the same way. `examples/solver-demo.sysml`, `examples/disposal-robot-demo/robot.sysml` and `examples/disposal-team-demo/team.sysml` are migrated and now analyse clean; the known-gap entries carrying them in the examples gate are gone.

- **A feature valued only by a `[0..1]` binding end reports both ends of the binding.** `bind [0..1] tf.edges = [0..1] tfe` links one unspecified value of each end, so `tfe` stays the typed `ErrBindingEnd`; its text now reads "which makes some value of tfe a value of tf.edges without saying which value of either; the model does not state what tfe holds", on `-e`, `%eval`, `-instantiate` and `%features` alike. `ShapeItems::Box`'s edge and vertex groups (`box.tfe`, `box.tflv`, `box.tfe.length`, `box.vertices`) are pinned as this error in conformance, and the library question behind them — groups fixed only by partial bindings, and a `size(vertices) == size(edges)` its faces' vertices exceed — is drafted in `docs/project/omg-issues.md`.

- **`redefinition-type-mismatch` is now a warning.** A redefinition is a subsetting, so the redefining feature is typed by its own type *and* the redefined feature's (KerML §8.3.3.3.4, §8.3.3.3.6); neither KerML nor SysML v2 constrains the two to conform and the pinned pilot validator accepts `item :>> faces : Polygon` under `faces : StructuredSurface`. OpenSysML keeps reporting an unrelated redefining type as a likely slip, but no longer as an error: `ShapeItems.sysml` analyzed against the library carries no error.

- **A specialization or redefinition that inherits a result expression may not state a second.**
  `constraint def Sub :> Base { x > 1 }`, `require constraint :>> c { x > 1 }`,
  `calc def D :> C { x + 2 }` and a calculation or constraint usage typed by, subsetting or
  reference-subsetting (`constraint d ::> c { x > 1 }`) one that owns a body are rejected with
  `Only one (owned or inherited) result expression is allowed`, on the newly stated body, as the
  reference validators reject them; two generals each owning a
  result expression are reported on the declaration that inherits both, and a calculation,
  function or expression body listing two bare expressions (`calc c { 1 2 }`) is reported on
  the second, which the reference grammar does not admit. An empty or
  documentation-only redefinition (`:>> c;`, `:>> c { }`, `:>> c { doc /* … */ }`) keeps the
  inherited expression, and a nested `assert constraint { … }` remains a separate constraint, so
  a tighter requirement is written as a new or nested constraint rather than by replacing the
  inherited body. The runtime agrees: a calculation or constraint that states or inherits more
  than one result expression is refused with a typed error, naming each owner, instead of being
  evaluated with a silently chosen body, and a bodiless reference-subsetting calculation or
  constraint (`calc two ::> one;`, `constraint kept ::> c;`) computes or checks the body it
  inherits rather than reporting none.

- **The SonarCloud findings raised by the 0.5.1 changes are cleared.** Two repeated standard-library names in the implicit-base tables are named constants, argument checking passes the call as one struct instead of eight parameters, and the Windows VERSIONINFO check script uses `[[` tests and a local for its positional parameter. No behavior changes.

### Fixed

- **An actor bound without `:>>` is held to the actor it inherits.** A requirement or
  objective usage's `actor pilots = pilot;` did not redefine the definition's `actor pilots :
  Pilot[2]` — only `actor :>> pilots = pilot;` did — so one pilot, or a buoy, passed unchecked. An
  actor or stakeholder now implicitly redefines the general's actor at its position, as a subject
  redefines the subject and a parameter the parameter it follows (KerML §7.4.7.3), keeping or not
  the inherited name: it takes that actor's type and multiplicity, so one pilot is refused as
  `actor binding: multiplicity violation: 1 value(s) bound to a feature with multiplicity lower
  bound 2`, two buoys as a `type mismatch`, and two pilots satisfy the requirement; in an
  objective the refusal leaves the verdict `undecided` with that detail. An explicit `:>>` binding
  is unchanged. The redefined actor contributes its members and conformance wherever the
  redefining one is read, in the language server and the passes as at run time.

- **An analysis objective binds its requirement's subject by keyword alone.** An objective typed by
  a requirement definition (`objective : MassLimit { subject = ship; }`) was always `undecided: no
  value for feature s`, because only the requirement engine knew the keyword-only form. An
  objective's own members are now bound the way `-requirement` binds a requirement usage's, so
  `subject = ship;`, `subject s = ship;` and `subject :>> s = ship;` all decide the verdict, in a
  definition, a usage and through a nested analysis step, and the binding may read the case's steps'
  outputs, a nested case's or an action's (`subject = weigh.m;`), as may an `assert constraint` of
  the body. An objective that binds no subject takes the library's default, the case's result
  (`Cases::Case::obj` declares `subject subj default Case::result`); a result of the wrong type is
  `undecided` saying so (`subject s defaults to the case's result (Cases::Case::obj): type
  mismatch: 1000.0 (a Real) is not a Ship`), one of the wrong multiplicity (one `Ship` for a
  `Ship[2]` subject) is `undecided` as a multiplicity violation — an objective redeclaring the
  subject without one (`subject :>> pair;`) keeps the `[2]` — and a case returning none says to
  bind it or return one. An object bound to a subject, by the default or by an expression, in an
  objective or a requirement usage, is held as a value of it: a `Ship` bound to a `subject t :
  Tanker` gains `Tanker`'s features, so `t.cargo` answers where it read `member cargo not found`,
  and a value the subject cannot hold at all is refused as a `type mismatch` instead; an expression
  yielding more or fewer values than the subject declares (one `Ship` for `Ship[2]`, or none) is
  refused as a multiplicity violation, as the default already was. The object a satisfaction
  assertion supplies with `by` is held to the subject the same way: `satisfy laden by ship` reads
  `t.cargo` of a `Ship` bound to a `Tanker` subject, and a `Buoy` supplied for a `Ship` is refused
  (`subject: type mismatch: Buoy #1 (buoy) is not a Ship`) rather than checked.
- **A case's result is readable by its qualified name.** `MassCase::result` — the form the OMG
  examples use, `objective : MassAnalysisObjective { subject = MassAnalysisCase::result; }` — read as
  an empty sequence when the case's result was unnamed (a trailing expression or `return : Real`),
  leaving the objective `undecided: comparison operands must be constants`. A qualified feature the
  running case declares, or inherits from the library (`Cases::Case::result`), now reads the run's
  binding for it: in the objective's subject, in an `assert constraint` of the body, and as
  `inner.result` from the case performing `inner` as a step.
- **A recursive analysis step reports one line, not one per frame.** An analysis performing itself
  as a nested step, or a `calc def` recursing through its own `calc` usage member, hit the recursion
  limit with a message repeating `node again:` ten thousand times (hundreds of kilobytes). Those
  frames now collapse as a calc calling itself does — `analysis An::Rec::again: … 9999 frames:` —
  keeping the typed error and the hint to raise `OPENSYSML_MAX_CALC_DEPTH`.

- **An analysis case's later objectives keep their position under every general.** When two generals shared an ancestor and one of them restated an objective, the ancestor's objectives were listed for the first general only, so a specialization's second objective redefined nothing through the other and lost that objective's type and members. Each general is now listed in full and only identities are merged.

- **`Must own its annotating element` is reported even when the document has unrelated errors.** The annotation-ownership rule was skipped for the whole document as soon as any lower-tier diagnostic (an unresolved name, say) appeared anywhere in it, so a comment or metadata usage annotating itself went unreported until every other error was fixed. The rule now runs per annotation and only stands down for a reference that itself failed to resolve.

- **A `bool def` is checked like the other behavior definitions.** A Boolean expression
  definition specializing an attribute, item or part definition now draws the same `Cannot
  specialize …` report as a `calc def` or `constraint def` would; it was skipped by the family
  check because the kind had no entry.
- **Every KerML feature that subsets a classifier is reported, not only `feature`.** A `bool`,
  `expr`, `step` or other feature whose `:>` names a data type, class, structure or
  behavior is now reported (`subsets target must be a feature, found …`), as the reference
  implementation does when it fails to resolve the classifier at that position; only
  `feature f :> D` was checked.

- **Solving an analysis case that states or inherits more than one result expression is refused.**
  `analysis def Stated :> Base { size <= 4 }` over a `Base` that owns a result expression, or
  `analysis def Inherited :> Base, Other;` inheriting one from each, is rejected by validation
  with `Only one (owned or inherited) result expression is allowed`; the runtime's case
  conditions now record the same conflict, so `solve` refuses the analysis at the offending body
  or declaration instead of solving it with a silently chosen result, and does so before
  reporting a missing objective. A case inheriting a single result expression is unaffected.

- **A chain reads its next segment from the usage it passes through, not only from that usage's type.** `cf.edges` for `item cf : Surface [1] :> faces;` reaches `edges` through the subsetted `faces`, and `a#(i).b` names a member of one element of `a`; the first was reported as an unresolved member, so the geometry library's `ConeOrCylinder`, `Cone` and `Cylinder` failed name resolution, and the editor found no definition for the second. The document's walk and go-to-definition now share one lookup, so they agree on what a chain names.
- **Comparing a quantity to a bare number is not a dimension mismatch.** `xoffset > 0` on a `LengthValue` was warned about as combining incommensurable quantities, although the pilot validator accepts it and the geometry library is written that way (zero is the null quantity of every dimension, so it is read in the quantity's unit); comparisons with a bare zero are now silent, while `length + 5` and `length > 5` still warn.

- **A KerML `binding`/`succession` written without `of`/`first` puts a leading multiplicity on
  its first end.** `binding [1] a = [1] b;` and `succession [1] a then [*] b;` declare no
  connector of their own, so the grammar reads `[1]` as the first end's crossing multiplicity;
  the parser used to record it as the connector's multiplicity, which the RDF mapping then wrote
  back behind the second end and could not read again. Both ends now carry their multiplicities,
  the connector carries none, the RDF graph states each end's bounds on the end node, and the
  Kernel Semantic Library's `Occurrences.kerml` written back from its graph alone keeps every such
  site as written. `binding [1] of a = b;` and `succession [1] first a then b;` still give `[1]`
  to the connector.
- **A parameter's specializations may follow its multiplicity.** `in x : Integer[1] redefines
  A::x;`, `in y : Integer[1] :>> A::x;` and `return : Integer[1] ordered :>> C::r;` are accepted
  for `in`, `out`, `inout` and `return` parameters, with `ordered`/`nonunique` between, as
  `FeatureSpecializationPart` allows on any feature; they were reported `expected ';' or '{'
  after parameter`. The parameter path now shares the ordinary usage's specialization loop.

- **A cross feature may be `ordered` or `nonunique`.** `end x1 [1..*] ordered item x : C;` and `end [*] nonunique ordered :> g feature y : C;` are valid (the KerML `MultiplicityPart` admits both words after the multiplicity, in either order) and the pilot accepts them, but the parser stopped the cross feature at the multiplicity and rejected the kind declaration that followed. Both words now stay on the cross feature — in the AST and its serialization, the RDF export and import (`sysml:isOrdered`, `sysml:isNonunique`) and the uniqueness conformance check — and never move onto the end.

- **The prefix an end writes ahead of its cross feature belongs to the cross feature.** In `end in x1 : C [1] item x : C` and `end var x1 : C [1] feature x : C`, the direction and the `derived`, `abstract`, `variation`, `composite`, `portion`, `var`, `constant` and `ref` modifiers between `end` and the cross feature's declaration were recorded on the end `x`, so it was rejected with `End feature cannot have direction` or `End feature cannot be derived, abstract, composite or portion` where the pilot accepts the model. The parser now records them on the cross feature (KerML `BasicFeaturePrefix`, SysML `BasicUsagePrefix`), the variable-feature rules check the cross feature they modify, and the RDF mapping states them on the cross feature and writes them back there.

- **A cross feature declared ahead of its end keeps its own relationships.** In `end x1 : Sub1 [0..1] :> g feature x : C1`, the typing, subsetting, redefinition and reference clauses of the cross feature `x1` were also recorded as relationships of the end `x`, so `x` was typed by `Sub1`, subset `g`, and was checked against those relationships by the typing and conformance rules. The parser now stores them on the cross feature alone, every consumer (name resolution, typing, conformance, RDF export, references, completion) reads the end's and the cross feature's relationships separately, and the operator spellings `:` and `:>` are accepted there alongside `typed by` and `subsets`. A cross feature typed more narrowly than its end is now reported (`Cross feature must have same type as feature`), as the pilot reports it.

- **An unnamed cross feature may specialize after its multiplicity.** `end [1] :> g feature x : C` and `end [0..1] subsets g item x : C` are valid (KerML `FeatureSpecializationPart` lets the specializations follow the multiplicity) and the pilot accepts them, but the parser stopped the cross feature at `[1]` and rejected the kind declaration that followed. It now reads the specializations onto the cross feature, where the ones written ahead of the multiplicity already went; the end keeps only its own.

- **A KerML `disjoint X from Y;` member keeps which end is disjoined from which.** The
  keyword-first Disjoining is now a relationship element of its own, like `subtype A specializes B`
  and `inverse f of g`, with ordered ends, an optional `disjoining <id>` identification, a
  visibility prefix and a relationship body; both ends resolve, including feature-chain ends such
  as `disjoint earlierOccurrence.successors from laterOccurrence.predecessors;`. The RDF export
  writes it as a `sysml:Disjoining` with `typeDisjoined` and `disjoiningType`, and a graph
  without source text converts back to the same notation — previously it was an anonymous feature
  with two `disjointFrom` objects that came back as `disjoint from X, Y;`, which does not parse.
  The declaration clause `class C specializes A disjoint from B;` is unchanged.

- **`sum` over an empty collection of quantities is the zero of the collection's declared kind.** `sum(subcomponents.totalMass)` over no subcomponents is `0 [kg]` where `totalMass :> ISQ::mass`, in the kind's coherent SI unit, so a roll-up such as `attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);` evaluates to `mass` on a leaf instead of failing with `incommensurable units: cannot express 1 (1) in kg`. An empty feature, chain, `default null` collection, `select`/`reject` of nothing and a typed `collect` body carry the declared unit to the aggregate; `product` of none stays `1`, `size`/`#` stay counts, and `10 [kg] + 0 [m]` or `10 [kg] + 5` are rejected as before. The REPL, `-run-query`, documents and the gRPC feature-value path all show the typed value.
- **A multiplicity-many feature declared `default null` holds the members that subset it.** `part subcomponents : MassedComponent [*] default null;` with `part a : Leaf :> subcomponents;` and `part b : Leaf subsets subcomponents;` holds `a` and `b` — type-level, usage-level, inherited and redefined subsetting members alike — since a default is the feature's value only where nothing else populates it; a collection nothing subsets stays empty, and features subsetting each other report a cyclic feature value instead of recursing.

- **Only an `attribute` typed by an enumeration is held to one type.** `An enumeration attribute
  cannot have more than one type` was also raised on a `ref` or bare usage typed by an enumeration
  and another definition, which the reference implementation accepts as a reference usage; and an
  enumerated value whose value is typed by several types is judged by all of them, so `h =
  wrongOrLevel` with `ref wrongOrLevel : Wrong, Level` is reported as typed outside its enumeration.

- **An unnamed usage redefining several features reads back from the graph alone.** The RDF decoder derived an anonymous usage's effective name only when it redefined exactly one feature, so `attribute :>> Disc::innerSpaceDimension, faces::innerSpaceDimension;` was refused when a feature chain later named it. It now takes the name from the first redefinition, as the resolver does (KerML 7.3.4.5), unless that target is itself a feature chain.

- **A usage subsetting an inherited library feature now inherits that feature's own members.** The inherited-name conflict rule reached only the members of the library definitions a usage conforms to, so a redefinition naming a member through the subsetted feature (`ref :>> Polygon::edges, Polyhedron::faces::edges`) was still reported as a duplicate of the typing definition's namesake, and ShapeItems carried nine such false `Duplicate of inherited member name` warnings. The members of every type and feature passed on the way to a library base now take part, so such redefinitions are silent while a partial one, or an unredefined name reached from both sides, is still reported as the pinned pilot validator does. A document that re-declares a library file no longer sees the library copy's members as its own.

- **The inherited-name conflict rule now compares short names.** A member inherited from a library base was known under its primary name only, so an owned `attribute h` or `attribute <h> myHeight` beside an inherited `attribute <h> height` drew no `Duplicate of inherited member name` warning, and a diamond reached only through a short name went unreported. Each inherited member now counts under both identifiers, each owned member or alias is compared under both of its own, and the warning is placed at the identifier that repeats, as the pinned pilot validator places it. A member that redefines the inherited feature, or an alias for it, stays silent whichever name it reuses.

- `Only one subject/objective is allowed` now counts inherited subjects and objectives the way the pilot does: a case or requirement whose generals together supply two is reported on its declaration, an owned one that redefines them (by clause or by position) silences them, and an owned one beside an unredefined inherited one is reported on the owned member.
- An analysis case stating several objectives redefines its general's objective at each position, so a later unnamed objective keeps the inherited type and members (its `eval` resolves) instead of only the first; the solver likewise treats such a restatement as the inherited objective declared again, keeping the inherited place in the lexicographic order, rather than as one more objective beside or after it.
- A case inheriting the same general through two paths, one of which restates its objective, sees the restatement in the objective's place rather than both, whichever general is written first; a further specialization's objectives line up with it, and the solver optimizes it once.
- A case or requirement usage that reference-subsets another (`case c : A ::> b;`) inherits the referenced usage's subjects and objectives too, as the pilot does, so `Only one subject/objective is allowed` counts them and an owned role redefines them by position.
- The subject a satisfy-by or nested subject is judged against is the one that survives redefinition through a diamond of generals and referenced usages, whichever branch is written first; a branch restating the common subject with a narrower type no longer leaves the wider one in force.

- A view usage's `expose` now honours the `filter` conditions of its view definition and that definition's supertypes, not only the conditions written in the usage itself.
- An `expose` a view inherits from its definition or supertypes is now admitted against the inheriting view's own `filter` conditions too, so a view narrowing its definition exposes only what satisfies both.

- RDF: a connector end that declares a name ahead of the feature it references (`connector a ::> a.x to b;`, `connect bead ::> t.bead to …`) now relates that feature as `sysx:relatedFeature` and carries its own name as `sysx:endName`, so the head is written back from the graph without its source text; a named KerML connector (`connector c from a to b;`) keeps its `from` on the way back. Before, the end's name stood in for the feature and the name was lost.
- Parser: golden and negative coverage for every form of the KerML binary connector — the first end is an end, not the connector's name, unless `from` follows the declaration — and the AST dump shows an `all` prefix; symbols, name resolution and the LSP are tested to keep an end's name out of the type featuring the connector.

- **The v1 migration writes a read-only property as `constant`, and its prefix keywords in the grammar's order.** It wrote `readonly`, which is not a SysML v2 keyword, so a migrated model with a read-only value property no longer parsed once the parser stopped reading a stray name ahead of the kind keyword; and `derived`/`abstract` came before a flow property's `in`/`out` and a constraint parameter's `in`, which the pilot rejects. The prefix is now `in`/`out`/`inout`, `derived`, `abstract`, `constant`, `ref`, on every usage kind. A read-only property of a value type stays a plain attribute with the loss noted in the report, since the features of an attribute definition cannot vary and `constant` is not allowed on them. Found by the migration's own golden test.

- **A value typed by several types conforms when any one of them does.** `Bound features
  should have conforming types` and `cast-conformance` judged a multi-typed value (`part ab : A, B`,
  a calculation returning `A, B`, an element `abs#(1)` or `abs.?{…}` of such a sequence, a feature
  typed only by such a value) by its first type alone, so a result expression, a bound subject, a
  `satisfy … by` operand or a cast that conformed through the second type was reported. Every
  statically known type is now kept and the reference implementation's existential rule applied;
  arithmetic and conditional results, which are not statically known, stay silent as before.
  Conversely, a value whose types are all unrelated to a scalar-typed feature (`attribute
  a : Integer = b` with `part b : Boat`, or `part a : A, Integer = b`) is now reported instead of
  being skipped as a scalar case, and an indexed element `xs#(i)` is judged by the element's type.

- **A name written ahead of a kind keyword is a syntax error, not a renamed member.** `part def B
  { foo attribute bar : A; }` was accepted silently as an attribute named `foo`, and the declared
  name `bar` was dropped; likewise `x part def P;` became a definition named `x`. Neither the SysML
  nor the KerML grammar has such a form — a name always follows its keyword — and the reference
  implementation rejects it. The stray name is now reported (`expected a body member`, or `expected
  a namespace member` at package level), skipped, and the members after it still parse.

- **Cast, bracket and quantity warnings reach filter conditions and multiplicity bounds.** The
  operator-expression checks were only applied to values and behavior bodies; an
  `x[i]` in a KerML filter, an unrelated `as` target or a non-reference unit in `filter` or `[lo..hi]`
  is now reported once, like the same expression elsewhere. The declarations an expression body
  `{ … }` makes are reached too, and a bound's operators are judged by the type tier, so an
  unrelated error elsewhere in the document no longer silences them. Every bound the notation
  writes is covered: a `multiplicity m [lo..hi]` declaration, a body parameter's `in x : T[lo..hi]`,
  a cast's, a connector end's, a cross feature's and a subject's or assume/require's. The members
  a `multiplicity` or `specialization` declaration owns in its body are type-checked like any other
  body's, so an invalid cast or value in them is reported too, also when the declaration sits under
  a definition or package that a filter's expression body declares.

- **Parser: prefix metadata written ahead of `subject`, `actor`, `stakeholder`, `objective`, `variant`, `assume` or `require` is now a syntax error.** The grammar places it after these keywords (`subject #M s : T;`, `assume #M constraint a : C;`), which is what the pilot implementation accepts; `#M subject s : T;` was read silently as if written the other way, so a graph-only RDF round trip rewrote the source without a word. The error spans the `#` run, names the accepted spelling and offers the quick fix that moves the run; the member is still read so later diagnostics and editor features carry on. Prefix metadata ahead of an ordinary usage or definition and ahead of `assert` is unchanged.

- Parser: a `use case` definition or usage may now carry prefix metadata (`#structuredUseCase use case u;`); the two-word kind keyword was not recognised after `#` prefixes and the member was reported as `expected a namespace member`.

- **The Python client asks the service for uncompressed responses.** Under load, grpcio
  occasionally handed a gzip-compressed response to the protobuf parser, so an RPC the service
  had answered correctly failed with `Exception deserializing response!` or `Wire format was
  corrupt`. Every channel the client opens now accepts identity encoding only, which the service
  honours, so no response reaches the parser compressed.

- **`python -m opensysml.generate` connects on its own.** The generator went through the
  module-level default connection, which is kept per address for the life of the process along
  with the service's handshake. Run twice in one process against services that took turns on one
  port, the second run was judged by the first service's handshake and could refuse a current
  service as too old. Each run now opens a connection of its own and closes it when done.

- **A quantity compared with a bare zero evaluates.** `xoffset > 0` over a `LengthValue`, as the geometry library writes it, failed at evaluation with `cannot express 1 (1) in m (metre)` although the static check accepted it, so a constraint written that way never reached a verdict. Zero is the null quantity of every dimension, so a comparison (`>`, `<`, `>=`, `<=`, `==`, `!=`, and the `QuantityCalculations` forms) now reads a bare zero on either side in the quantity's unit, at evaluation, in model-level constant folding and in element filters alike; any other bare number stays incommensurable with a measured quantity, as do `length + 0` and `max(length, 0)`, whose result would have to name a unit, and the static check now warns on `length > 5` where it already warned on `length + 5`, and stays silent for any constant that folds to zero (`length > 1 - 1`), so the two tiers agree.

- **Document queries read quantity-valued attributes.** `attribute :>> mass = 5840 [kg];` —
  the form every Apollo 11 component uses — was refused by `Project`, `WhereFeature`, `OrderBy`
  and `Column` expressions with `cannot evaluate feature mass`, because the constant folder
  behind them knew literals but not the quantity operator. Quantities and the constant
  expressions over them (`2 [kg] * 3`) now fold in semantics into a value that keeps its
  magnitude, the unit the model spelt and the reduced unit term; the runtime shares that
  representation instead of owning its own. Cells render as the REPL prints them
  (`2290000 [kg]`) in `-run-query` listings, Markdown and HTML tables (the HTML span also
  carries `data-magnitude` and `data-unit`) and over gRPC as a `quantity` `DocumentValue`.
  `WhereFeature` compares a bare threshold against the magnitude in the attribute's own unit;
  `OrderBy` converts commensurable units (`500000 [g]` sorts below `119000 [kg]`) and refuses
  different dimensions with an `invalid-order` error naming both units; column arithmetic keeps
  and composes units (`mass / length` is `[kg/m]`) and refuses incommensurable operands with a
  `column-incommensurable` error naming the column and row. An attribute whose value is not a
  constant (`mass = dryMass + propellantMass`) stays a typed `unevaluable-feature` error, now
  naming the row element too.

- **Notation written from an RDF graph alone parses again for connectors, guarded successions
  and nested expressions.** With `sysx:sourceText` stripped, `sysml -convert sysml|kerml` wrote
  an anonymous connector's own multiplicity after its last end (`succession first a then b[n]`),
  where the grammar reads it as the end's; it is now written in the declaration slot
  (`succession [n] first a then b`, `bind [1] a = [1] b`), and a KerML `binding`/`succession`
  with a connector multiplicity always writes `of`/`first` (`binding [1] of a = b`), since
  without the verb the grammar gives the multiplicity to the first end.
- **The expression writer places parentheses by the parser's precedence table.** A
  conditional or any other loosely binding form used as an operand of a tighter operator is
  parenthesized (`size(ae) == (if isEmpty(af) ? 0 else 2) and …`, `(p ?? q) implies r`,
  `(a + b)[1]`, `(x as T).f`, `not (p and q)`), and redundant parentheses are no longer written
  around every operator (`x * 2`, `if x > 0 ? x else - x`); before, a nested conditional came
  back bare and did not parse, while every other operator was wrapped unconditionally.
- **A guarded succession records its syntax from the AST, not from the words ahead of it.**
  `public succession S first A1 if x == 0 then A2;` recorded the bare-source transition form
  because `first` was not among the first tokens, and came back as `transition S A1 if …`,
  which does not parse; the encoder now derives the form from where the AST places the source
  and keeps the `succession` keyword, and the decoder writes a named transition or succession
  with `first` and its visibility.

- **KerML `const` and `portion` feature prefixes survive a graph-only RDF round trip.**
  The decoder wrote `sysml:isConstant` back as SysML's `constant` under a KerML root, where the
  grammar spells it `const`, so a graph stripped of its `sysx:sourceText` came back unwritable
  (`cannot convert the reference to Prefixes::A from Prefixes::B::k`); it now spells the flag in
  the grammar the root's `sysx:sourceLanguage` records. The encoder did not export
  `Feature::isPortion` at all, so `portion feature p : A;` came back as `composite feature p : A;`
  from the graph alone; it now writes `sysml:isPortion`, which a KerML root reads back as
  `portion` in place of `composite`. On a SysML root the flag is the fact `snapshot`/`timeslice`
  states (the encoder now writes it for those too), and a graph stating it without a
  `sysml:portionKind` is refused rather than respelled, SysML having no `portion` prefix.

- **A KerML `member feature` is exported as a plain `sysml:OwningMembership` and comes back with its `member` prefix.** The RDF encoder typed every feature a type owns as a `sysml:FeatureMembership`, so `class C { member feature x; }` reached the graph as a feature of `C` and returned from it as `feature x;`, a different model. The membership is now the `OwningMembership` the grammar names, with none of the feature-ownership predicates, and the decoder writes `member` for a feature a type owns that way — except through a variant, result, metadata or head-declared cross-feature membership, which KerML writes otherwise. SysML has no `member` keyword, so a SysML-language type owning a feature through a plain `OwningMembership` is refused with an `UnsupportedError` rather than written as a feature of the type.
- **A `featured by` naming an anonymous redefining `portion` is written in a spelling that resolves on re-read.** Inside `portion :>> startShot { member feature s featured by CC1::startShot; }` the converter wrote `featured by startShot`, checked that spelling in the enclosing scope, and missed that the parser reads a head's `featured by` in the feature's own scope first, where the inherited `Occurrence::startShot` shadows the portion; the re-encoded graph then held the literal `"startShot"` where the original held the portion. References collected from a head relationship now carry that scope and are probed where the document reads them, so the converter writes `CC1::startShot`; a plain `:>>` target is read in the owning scope, as before.

- **RDF mapping: a named `event`, `perform`, `exhibit` or `assert` graph the notation would read back as a reference is refused instead of rewritten.** A graph stating `sysx:declaredKeyword "event"` together with a `sysml:declaredName` came back as `event e;`, which names an existing `e` rather than declaring one, so a second conversion produced a different graph without a word; the same held for a named `sysml:AssertConstraintUsage` that nothing but a body or a multiplicity follows (`assert c { … }`). Both are now typed refusals naming the element and the fact at odds, while `assert safe : Safe;` and `assert c references mc;`, which the parser reads as declarations, still round-trip from the graph alone.

- **A `portion` feature comes back from RDF as a portion.** The mapping stated a KerML `portion feature p : A` — and a `portion` cross feature — only as `sysml:isComposite`, so a graph without `sysx:sourceText` was written back as `composite feature p : A`. It now also states `sysml:isPortion`, and a feature stating `isPortion` is written back with `portion` whether or not the graph also states `isComposite`.

- **An unnamed feature takes a name from its reference only in the forms the reference
  validators name.** `perform a;`, `exhibit s;`, `include u;`, `require`/`assume`, `frame`,
  `render`, a variant and a state's `entry`/`do`/`exit` action are members of their owner under
  the referenced feature's name, inside it and through a qualified name or chain from outside
  (`h.a` names the performed `a`, and duplicates the inherited one, as `Duplicate of inherited
  member name` warns in both tools). An `assert q;`, a `satisfy r;`, an `event` and a plain
  `::> q` declare no name, so `h.q` written anywhere still names the inherited `H::q`; `assert
  h.q;` written inside `h` still does not find itself. A member redefining several features is
  named by the first (`part :>> engine :>> motor;` is `engine`), a declared short name suppresses
  the derived name (`part <e> :>> engine;` is `e` alone), a redefined feature chain names nothing
  (`part :>> p.q;`), and a `require`/`assume` stating a chain (`require h.rule;`) is a reference to
  its last feature, named `rule` and requiring that constraint's conditions. Only a redefinition hides the inherited member it names; an ordinary `:>`
  subsetting or a reference no longer masks it, so a feature reached through such a member
  resolves as the reference validators resolve it. A name-conflict warning on a member with a
  derived name is reported on the whole declaration, where the validators place it.
- **A parameter redefined under a short name alone is bound at run time.** `in <f> :>> factor
  default 3;` in a calc or action, and `in <a> :>> x = 4;` as the argument of a performed action
  or exhibited state machine, were dropped when the behavior ran, so the body read the inherited
  default instead; a redefinition under a new name (`in g :>> factor`) left the body's reads of
  `factor` unbound too. The body now reads the redefining feature's value under either name.

- **Both ends of a keyword-first KerML relationship are kind-checked.** `subtype`,
  `subclassifier`, `typing`, `subset`, `redefinition`, `conjugate`, `inverse`, `disjoint` and
  `featuring` members whose source or target resolves to an element the relationship cannot relate
  — a package at any end, a class where only a feature may stand (`subset C :> f`,
  `inverse C of f`, `typing C : T`, `featuring C by T`), a feature where only a classifier may
  (`subclassifier f :> B`) — are now reported at the type tier
  (`<keyword> source|target must be a type|classifier|feature, found <kind>`), as the reference
  implementation reports when it cannot link the typed cross-reference. Previously only the
  declaration clauses (`class C :> Q;`, `feature f : Q;`) were checked and
  `disjoint Q from B;` with `Q` a package analysed clean. Unresolved ends are still left to name
  resolution, and the declaration-clause reports are unchanged, with one exception: a named
  multiplicity is a feature and so a type (KerML 1.0 §8.3.3, Multiplicity specializes Feature),
  so with `multiplicity M [1..2] { feature x; }` the clauses `feature g : M;` and
  `feature g :> M.x;` are no longer reported as `type must be a type, found multiplicity` and
  `feature chain segment must be a feature, found multiplicity` — the reference implementation
  accepts both, as it does the keyword-first spellings.

- **REPL: `%eval` reads the object a submission carried over.** After an action wrote a part's feature (`holder.n := 5`) and an unrelated declaration (`part def Widget;`) re-analyzed the session, `%features Demo::holder` listed `n = 5` while `%eval Demo::holder.n` answered `<unset>` and `%eval Demo::holder.cells.rank` failed with `member rank not found`: the carried object was rebound to the model index's symbol for `holder`, and the prompt resolves the name to the buffer's own symbol, so evaluation materialized a second, unwritten object beside the carried one. The runtime now rebinds a carried object to the symbols the caller resolves in, so `%eval`, `%features`, a debugger still stepping and `===` all observe the one object; a resubmission that changes the holder's declaration still drops it and every surface reports the loss.

- **A redefinition of a namesake resolves to that namesake, whatever comes first in the declaration.** `causes[1..*] :> participant :>> causes` resolved its redefinition to the redefining feature itself when the subsetting was resolved first, so the feature redefined nothing (the standard library's `Multicausation::effects` lost its redefinition of `effects`) and the RDF export named the wrong target once the clauses were reordered. The redefinition's target is now resolved on the redefinition's own path, which the redefining feature never masks.

- **`shortName` is derived only for features that declare no identifier at all.** A feature written with a name of its own (`perform action turn references steer;`, `part b :>> a;`) projected the short name of the feature it references or redefines, so a query, a document or a filter read `turn` as `<ss>`. KerML derives `Element::shortName` from the naming feature only when both `declaredName` and `declaredShortName` are absent; such a feature now projects no short name, while an unnamed one (`perform providePower;`, `part :>> a;`) still takes the short name of its naming feature.

- **A feature redefining a sibling no longer hides that sibling from chains through its owner.** Redefinition removes what the owning type inherits, and a type never inherits its own members, so `feature slice redefines xs;` beside `feature xs;` left `sub.xs` (with `sub` typed by the owner) unresolvable and made the RDF writer spell it `sub.C::xs`. Such a redefinition now masks nothing, as in the pilot implementation; redefining an inherited feature still masks it.

- **A state usage typed directly by the library's `StateAction` is exhibited.** `exhibit state phases : StateAction;` and `: States::StateAction`, as the Apollo 11 model's `Mission` writes it, failed with `recursive state typing: state self is typed by StateAction, whose content contains it`: the runtime withheld the library definition's content, but answered lowering with the same result as an unresolved name, so lowering looked the name up again through the scope tree. A body-less usage runs the library definition itself as its machine, and inside that body `StateAction` is lexically in scope, so the content was materialized anyway; a definition `:> StateAction` or a usage stating its own body is lowered in the model's document, where the scope tree alone never reaches the library, which is why those forms already worked. The lowering contract now says *withheld* distinctly from *unresolved*, and only an unresolved name falls back to the scope tree. A usage typed by `StateAction` that states its own body runs it; one stating none has no initial state and fails as any such machine does, `no initial state found in state machine StateAction`.

- **The Linux `amd64` release binaries no longer require glibc 2.34.** `sysml`, `sysml-lsp` and `sysml-grpc` were linked against the build machine's libc, so they failed to start on Ubuntu 20.04, Debian 11, RHEL 8 and similar. The release job now builds with `CGO_ENABLED=0`, producing fully static binaries as the `arm64` ones already were; the only cgo users were Go's standard `net` resolver and `runtime/cgo`, so the pure-Go resolver is now used instead.

- **Every transition guard is checked for a Boolean value, and a state body accepts the `if … then` and `transition if … then` target-transition spellings.** The guard of an action body's guarded succession (`first a if "go" then b`, `succession s first a if x then b`, a decision's `if e then b`) was not type-checked; it now reports `transition guard must be Boolean, found String` like a state transition's guard does. `if ready then s2;` and `transition if not ready do action reset then s1;` in a state body, which SysML v2 admits as a transition from the enclosing state, parsed as `expected a body member`; they parse, resolve and lower with the enclosing state as their source, like `accept … then` already did.

- **A composite `variant port` is reported exactly once, and only under a port owner.** A `variant port` nested in a chain of variation ports drew `A port usage must be referential.` once per enclosing variation; and a `variation port` owned by a part reported its composite variant ports and variant parts although the pilot accepts them, since a variant is not a nested usage of the port that owns the variation.

## 0.5.1 — 2026-09-05

### Added

- **Association arity, binary-link end counts and multiplicity bound types are checked the way
  the reference does.** A concrete KerML `assoc`, `assoc struct` or `interaction` with fewer than
  two ends is reported `Must have at least two related elements`, as a SysML `connection def`
  already was; an interaction now implicitly specializes `Links::Link` (`Links::BinaryLink` when
  binary) and `Performances::Performance`, being both an association and a behavior: a step may
  invoke it, and its directed features are parameters, redefined by position like any behavior's.
  A connector, binding, succession, flow or association that conforms to `Links::BinaryLink` yet
  has more than two ends — positional `(x, y, z)` ends, declared `end` features and inherited
  ends counted alike — reports each end past the second, with the redefinition check no longer
  masking it. Whether a KerML association or connector implicitly takes the binary base follows
  the same count, so one that redefines two ends of a three-ended general and inherits the third
  stays n-ary. A multiplicity bound whose result type resolves to anything but an Integer-conforming
  data type — a feature typed by a class, a bare `part`, `item`, `port`, `action` or `step` typed
  only by its kind's library base, a call to a function whose result is a class, or a quantity
  such as `3 [kg]`, say — is rejected with `Must have a Natural value`; an unresolved or untyped
  bound stays silent.
- **KerML binary connectors parse as the grammar reads them.** `connector a to b`,
  `connector [0..1] a to [1..*] b`, `connector e ::> a.x to b.y`, `connector e references x to y`
  and `connector $::P::a to b` declare an anonymous connector with two ends; only `connector c from a to b` names one. The
  first end is no longer mistaken for the connector's name, so a model with two such connectors
  no longer reports a duplicate declaration.

- **Windows Authenticode signing through SignPath Foundation, ready to apply for.** The three
  Windows executables now embed a `VERSIONINFO` resource (`ProductName` `OpenSysML`,
  `ProductVersion` and `FileVersion` taken from the same `VERSION` the `-ldflags` carry,
  `CompanyName`, `FileDescription`, `LegalCopyright`, `OriginalFilename`), written by a pinned
  `go-winres` for `GOOS=windows` builds only and checked against the tag by
  `make windows-versioninfo-check`. A new GitHub Actions workflow, `release-windows.yml`,
  rebuilds them on every `v*` tag with the CircleCI release job's Makefile targets and version
  variables, submits them to SignPath for signing once the `SIGNPATH_API_TOKEN` secret and the
  `SIGNPATH_*` variables are configured — and otherwise stops after the build — and publishes the
  signed files as additional `*-signed*` release assets with a `SHA256SUMS-windows-signed.txt`,
  leaving the CircleCI-published assets, `SHA256SUMS.txt` and its cosign bundle untouched. The
  README gains the "Code signing policy" section SignPath's terms require (team roles, MFA,
  privacy statement), and the releasing guide the application, configuration and per-release
  approval procedure. Signing takes effect only after the project's SignPath application is
  approved.

### Changed

- Fully-qualified names no longer carry empty segments for unnamed enclosing elements (`Mid::inner`, never `Mid::::inner`), so name resolution stops re-normalizing every name it looks up: loading a real 28-file model allocates about 37% fewer objects and 8% less memory.

- **The validation-constraint census gate now verifies the evidence each row cites, not only that the cited files exist.** `go run ./cmd/validation-census -check` requires every implemented row's `Implementation` cell to cite at least one `internal/….go:function` location and fails when a citation is not repository-relative (`file.go:function`), when the named Go file declares no such function or method (methods on generic receivers count) or when the citation runs past the method (`Type.method.extra`); it reads every listed negative case and fails when its header names no constraint or when the header's `pilot validate…` token (or the specification constraint cited before it) attributes the rejection to a different constraint, when the case's pilot-rejection bucket is not a rejection at all (`both-accept`, or a value the referee does not record) or contradicts the row's status (a faithful row citing a case only the pilot rejects, a not-implemented row citing a case only OpenSysML rejects, an unknown row citing any case), or when a corpus case attributed to a constraint is missing from that constraint's row. Two stale implementation references and fourteen rows whose evidence had never been recorded were corrected by the first run; three pilot-suite cases (`p13`, `p17`, `p19`) listed under a neighbouring constraint moved to the row the pinned pilot actually reports, and five pilot-suite headers now name that constraint beside the fixture they derive from.

- **The conformance records under `docs/project/` are consolidated and renamed.** Seven historical
  adjudication records are folded into one `docs/project/adjudications.md` that keeps the durable
  decisions and drops rows later work closed; the three records that document live mechanisms are
  now `element-scoped-tier-gating.md`, `lossless-library-records.md` and `errata-overlay.md`, and
  the stale round-by-round movement tables are gone in favour of the current oracle figures.

- **The state machine executor refuses the time trigger argument validation refuses.**
  `accept after 5` and `accept at 2 [min]` were reported by validation (`trigger-after-duration`,
  `trigger-at-time-instant`) yet scheduled by the runtime as five seconds and as an instant. The
  executor now makes the same judgement validation does, from the same declarations, and refuses
  such an argument as `ErrTimeTriggerType` when the state is entered, before the argument is
  evaluated or anything is scheduled; only an argument the declarations leave open — a feature
  whose type does not resolve — is read from its value, and a value there that is no time is
  still `ErrIncommensurableUnits`.
  Write `after 5 [s]` and `at` a `TimeInstantValue` feature, as the shipped conformance fixtures
  now do.

- **Guide chapter 9 is one client guide for all five languages, not a Python page.** It opens with
  install, a worked task and the failure model in Go, Python, Node/TypeScript, Java and Rust tabs
  before the per-client sections, and it now lives at `docs/guide/09-clients.md`; the old
  `guide/09-python/` URL redirects there, and the old path keeps a pointer page for the links to
  it that GitHub serves.
- **The landing page points at the client documentation rather than at Python.** The client card
  links to the guide chapter and lists every client's API reference beside it.

### Fixed

- State machines with time-triggered transitions (`accept after d`, `accept at t`) ran about three times slower than before the trigger argument's type was checked at runtime: the check re-derived the argument's static type on every state entry. The verdict is now judged once per transition the first time its state is entered and reused thereafter, restoring the previous execution speed and allocation volume; a wrong-typed argument is still refused on state entry exactly as before.

- **An alias declared in a calc or action body is not executed.** `alias b for a;` inside a behavior body was lowered as a statement and failed the calculation with "not executable"; it is now a declaration like any other.

- **A metadata type's `annotatedElement` alternatives are found by specialization, not by name.**
  A body feature merely named `annotatedElement` that specializes nothing is a duplicate member,
  not a restriction on what the metadata type may annotate; `@Named about Q` and `#Named part p`
  used to be reported `Cannot annotate …` against its type and are now accepted, as the pilot
  implementation accepts them. Only features that redefine or subset
  `Metaobjects::Metaobject::annotatedElement`, at any distance, are read.

- **A feature whose value calls a function on itself types in finite time.** A rollup such as
  `attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);` — the value of
  `totalMass` calls `sum` on `totalMass` itself — sent the type checker round the same call
  without end, and `sysml -validate` died with a stack overflow on the Apollo 11 model. Typing an
  argument that leads back to the call being typed now selects that call on its argument count
  alone, once, so the model validates and reports its diagnostics.
- **An interface whose ends are named like the parts it connects validates.** With
  `interface def I { end plss : P; end psa : ~P; }` and
  `interface x : I connect plss.port to psa.port;` inside a part with parts `plss` and `psa`,
  the accessibility rule resolved each end against the interface's own inherited ends rather than
  the enclosing part's, and reported `Must be an accessible feature` on a legal model. Ends now
  resolve in the enclosing scope, as name resolution already did.
- **A state definition written `:> StateAction` instantiates.** Spelling out the specialization
  every state definition has implicitly made lowering materialize the library's content, whose
  `ref state self : StateAction` led back to `StateAction`, so `-instantiate` of a part exhibiting
  such states failed with `recursive state typing`. `States::StateAction` now contributes no
  content to a state machine, as the implicit specialization never did; a library's own state
  definitions still contribute theirs.

- **An `assert` or `satisfy` may reference a case's objective.** `assert obj;`, `assert not
  uc.obj;`, `satisfy uc.obj;` and `requirement r :> uc.obj;` used to report `assert target must be
  a constraint usage, found partUsage`: an objective is a requirement usage and is now judged as
  one, directly, negated, through a feature chain, through an alias and
  inside a constraint body. A `subject`, `actor` or `stakeholder` referent stays a part and stays
  rejected, as the pinned pilot rejects it.
- **A feature chain through the asserting usage's own owner names the owner's member.** An
  assertion borrows the name of the feature it references, so `part h : H { assert h.q; }` used
  to resolve `h.q` to the assertion itself and accept it; it now reaches `H::q` and reports
  `assert target must be a constraint usage, found partUsage`, where the pinned pilot reports
  `Must reference a constraint.`

- **A KerML `assoc struct` is an association structure.** The parser recorded it as a plain
  `struct`, so it specialized `Objects::Object` instead of `Objects::LinkObject`, its features
  subset `Objects::objects` instead of `Objects::linkObjects`, and its metaclass was `Structure`
  instead of `AssociationStructure`. It now keeps the compound keyword through the parser, the
  implicit-specialization tables, the metaclass table and the Xpect export.

- **A generalization written as a feature chain is kept when resolving it re-enters an
  active lookup.** `feature b subsets x.f` declared inside a feature that itself subsets
  `a::b` used to lose `f` as a supertype for good: the chain's lookup was cut short by the
  resolver's cycle guard and the incomplete answer was memoized, so `b` inherited nothing
  from `f`. The answer is now provisional, as it already was for a qualified-name target,
  and the next query resolves the chain.
- **A generalization that fell back to an outer name while its owner's supertypes were
  still being computed is no longer memoized.** A member's `specializes X` resolved while
  the owner was mid-way through its own supertype query could not yet see the `X` the
  owner inherits and settled for a same-named `X` in an enclosing namespace; that answer
  was cached by both the resolver and the semantic model, so the inherited general was
  lost for good. Such an answer now holds for the query that made it only, and the next
  query finds the inherited one.

- **An invocation heading a feature chain is checked.** `F(x = 1, x = 2).r` reports the duplicate binding under one or several chain segments, as a bare `F(x = 1, x = 2)` already did, along with an unknown parameter name, an argument of the wrong type and too many arguments; a required parameter left unbound stays accepted at a chain head (`A().y`), as the reference's own suite declares.

- **A constraint usage may be typed by a requirement, concern or viewpoint definition.** `constraint c : SomeRequirementDef;` (and `require constraint c : SomeRequirementDef { … }`) was rejected with `A constraint must be typed by one constraint definition.`; a requirement definition is a constraint definition (SysML v2 §8.3.19), and the pilot accepts the typing.

- **A control node's members are an action's.** `fork f { attribute a : Integer := 1; }` (and
  `join`, `merge`, `decide`) used to report `Initialized feature must be variable`: the node
  had no implicit base, so nothing made it an occurrence. A control node now specializes its
  control action (`Actions::ForkAction`, `JoinAction`, `MergeAction`, `DecisionAction`), so its
  members are an occurrence's and may be variable.

- **A cross feature is compared with its end by effective type, not by spelling.** `end b : A
  crosses a.x` with `feature x;` untyped, `end b crosses a.x` with an untyped end, and an end
  whose body declares `member feature ac : B` while the end itself is typed by `A` used to pass
  `Cross feature must have same type as feature`; they are now reported, as the pilot
  implementation reports them, because an untyped feature is typed by `Anything` and an owned
  cross feature is typed by its end. Each side's types are the KerML `Feature::type` set — the
  declared types of the feature and of every feature it subsets, redefines or references, less
  those a more specific one makes redundant — so `feature x : A subsets w` with `w : W` is
  reported against an end typed `A`, while a connector end that reference-subsets a feature is
  not reported for it.
- **The bundled library snapshot refuses a blob written before the cross-feature symbol kind
  existed.** Adding the kind renumbered the symbol kinds after it, which the snapshot stores as
  integers, so a snapshot from an older build could have been decoded with the wrong kinds; the
  format version now moves with the numbering and a test pins it.

- **An enumerated value whose value is an expression body is typed outside its enumeration.** `enum def E { a = { 1 + 2 }; }` types `a` by `Performances::Evaluation`, a second type beside `E`, and is now reported as the reference implementation reports it; the body was previously invisible to the one-type rule.

- **An enumerated value typed outside its enumeration is now an error, and one typed by a general of it no longer is.** A value written on an enumerated value types it (`enum def E :> Real { b = 3; }` types `b` by Integer; `a = x == y` or `a = new A()` types `a` by Boolean or `A`, the result the library function or constructor declares; `a = xs.?{…}` keeps the type of `xs`, whether `xs` declares it, inherits it or is itself typed only by its value; `a = x as T` types `a` by `T` and `a = { … }` by Evaluation), and so does a declared type; either counts as a second type beside the owning enumeration unless the enumeration specializes it, exactly as the reference implementation's `validateEnumerationUsageType` decides. `enum def E :> Real { a : Real; }` was wrongly rejected before.

- **KerML `const` is a variable feature.** `const feature k : C;` is a variable feature in KerML,
  so `portion const feature` now reports `A portion cannot be variable` and a `const feature` owned
  by a package or by a non-occurrence type reports `Must be owned by an occurrence type`, as
  `var feature` already did and as the pinned pilot reports.
- **`var feature` parses at namespace level.** A KerML `var` prefix on a package or root member
  used to stop the parser with `expected a namespace member`; the declaration now parses and the
  owner rule reports it.

- **A `metadata m : A { … }` usage body is checked at the same tier as an `@A { … }` annotation body.**
  The usage form's `Must redefine an owning-type feature` and `Must be model-level evaluable`
  reports used to be skipped whenever the document carried any type-tier error — including the
  very same violation written in a sibling `@A { … }` annotation — so only one of two identical
  bodies was reported. Both spellings are now reported together, as the pilot implementation
  reports them.

- **Named arguments bind at execution the way they validate.** A calc or action call that names a parameter by its short name (`<xs> x`), an alias, a qualified inherited name or a redefinition now executes — the runtime binds the label to the parameter the checker resolved instead of its written text — and two spellings of one parameter are rejected as a duplicate at the call, whether evaluated, compiled or built in. A qualified label the checker rejects (`F(Other::x = 1)`) is likewise an unknown parameter at execution rather than binding `x` by its last segment.

- **A named require/assume constraint typed by a constraint definition now reads its own parameter bindings when its requirement is checked.** `require constraint n : Below { in x = m; in limit = 400.0; }` used to fail with `no value for feature x` because the condition was evaluated against the requirement's features rather than the constraint's; the constraint's parameters now mask same-named requirement features, and a redefinition rebinding one parameter (`require constraint :>> n { in x; in limit = 200.0; }`) keeps the others: its parameters redefine the redefined constraint's by position, as a step's do, so `in x;` restates the inherited `in x = m`. A default the definition writes in terms of another parameter (`in margin : Real default = limit * 2.0;`) reads the parameters the usage binds, an overriding `in limit = 100.0;` included, rather than a same-named feature of the requirement. A named constraint nested in another (`requirement def Outer { in x : Real; require constraint inner : Below { in y = x; } }`) reads the enclosing constraint's bindings too, its own parameters masking same-named outer ones. The parameters also mask a subject or actor the requirement binds under the same name (`subject v : Vehicle;` with `constraint def Below { in v : Real; … }`), which used to be read in the parameter's place and compare an object against a number, while the argument expressions that bind the parameters (`in v = v;`, `in limit = limit;`) still read the requirement's subject and actor rather than the parameter being bound. A parameter bound to a part (`in limit = truck;`) reads that part's attributes in the constraint body (`limit.mass`).

- **A `require`/`assume constraint` body reaches the parameters it declares.**
  `require constraint q { in y : Integer; y > 0 }` used to report `y` as
  `Must be an accessible feature`, because the body was judged from the requirement rather
  than from the constraint usage it states. The body is now checked from that usage, so its
  own parameters and the requirement's features are both reachable, while a feature named
  through another type's namespace is still reported, as the pilot implementation does.

- The objective cardinality rule ("Only one objective is allowed.") now judges `verification` and `use case` declarations as well as `case` ones, and an objective now redefines the objective role of the types above it: a case owning one objective under an inherited one is silent, as it is in the reference, while a type owning two is reported at the second and at whatever inherits both. An inherited `objective : R;`, which binds no name, is counted too. Analysis cases stay exempt, since their objectives are improved lexicographically in the order declared.

- **Overriding a bound parameter of a redefined `require`/`assume` constraint is now reported.** The parameters of `require constraint :>> n { in limit = 200.0; }` redefine those of the constraint it redefines by position, so that `in limit` overrides the first parameter's binding (`in x = m`) whatever its name; the rule used to see no redefinition there and accept the override silently, where the pilot reports `Cannot override a binding feature value`.

- **A direction parameter may open with a short name.** `in <xs> x : Integer` parses; a short name before the parameter name was previously rejected after `in`, `out` and `inout`.

- **A `when` trigger written as a conditional is refused like the other triggers.**
  `accept when (if flag ? true else false)` was accepted because both branches are Boolean,
  while `after` and `at` already refused a conditional argument as the `Anything` its library
  function returns, and the reference validator refuses all three. The `when` argument is now
  judged the same way (`trigger-when-boolean`, naming the result of `if`); an ordinary
  condition — a guard, a constraint, an `if` — still leaves a conditional to evaluation.

- **An error in an owner's value no longer hides its members' variability diagnostics.**
  `Initialized feature must be variable` and `Only a variable feature can be constant` on a feature
  declared inside a usage's body used to go unreported when the usage's own value failed a lower
  tier (`part p : PD = missing { attribute a : Integer := 1; }`). A value does not decide whether
  the members are variable, so only the owner's head before its value gates them now, as the
  feature's own head already did.

- **Selecting a variant as the value of a feature typed by its variation is no longer reported as a type mismatch.** A variant is implicitly typed by the variation that owns it, so `part vp : V = V::v1;`, `part w : V = vp.v1;` and `bind w = vp.v1;` conform even when the variant declares another type (`variant part v1 : Base;`). They used to be rejected as `cannot bind a value of type Base to a feature typed by V` and warned as `Bound features should have conforming types`; the reference implementation accepts all three.

## 0.5.0 — 2026-09-05

### Added

- **An object carries the features the Systems and Domain libraries declare for it.** A
  `part box : ShapeItems::Box` used to expose only the `length`, `width` and `height` the model
  wrote: every library-declared member was left out of an object's shape so that the Kernel
  Semantic Library frame (`self`, `portions`, `timeSlices`, `snapshots`, `startShot`, …) would not
  bloat every instance, and `box.isSolid`, `box.voids` and `box.shape` were `member not found`.
  The loader now records which *tier* of the library each document belongs to — Kernel Semantic,
  Kernel Data Type, Kernel Function, Systems, Domain or OpenSysML — kept through the symbol cache
  and queryable on any library symbol, and the runtime leaves out only the Kernel frame. So an
  item or part carries `Items::Item`'s `voids`, `isSolid = isEmpty(voids)`, `shape`, `subitems`
  and `subparts`, a `Parts::Part` its `ownedPorts`, `ownedActions` and `ownedStates`, a
  requirement its `subj`, `actors`, `stakeholders`, `assumptions` and `constraints`, and a
  `Box` its Geometry faces — each with the default, derived expression and multiplicity the
  library wrote, masked by a model's own `:>>` as any inherited feature is. `%features box`
  lists them, `%eval box.isSolid` answers `true`, `box.voids` is `[]`, and the gRPC
  `Instantiate` response carries them as feature values. A `%features` listing always shows
  every feature of the object asked about; nested expansions share the lines that remain. The
  Kernel frame stays out; a model that inherits nothing from these libraries keeps its shape
  digest, which names a library type by the library's identity — a digest of every bundled
  document — rather than expanding it, so an object is carried across a re-analysis over the
  same library and refused by one over a library whose declarations differ; and a value the
  runtime cannot evaluate — the Geometry library's edge bindings among
  them — is the typed error it already was, not a silent null. A requirement's
  `subject vehicle : Part = box;` now binds the subject on the object — it was left unset, as the
  binding was only read while checking the requirement — and the inherited `subj` reads the same
  object.

- **The Connect + JSON wire contract is written down for clients with no library.** A
  MATLAB, R, Julia, C or shell program that posts JSON to `sysml-grpc` by hand had only the
  proto file and two rules on the transports page to decode answers with, and the questions
  that page leaves open — how long a `modelHash` lives and what a stale one answers, how the
  eleven arms of `Value` are told apart and which of `unset`, `null` and an absent `result`
  means what, how a parse diagnostic differs from an in-body `error` and both from a Connect
  `{"code","message"}`, and what `Instantiate`, the behavior calls, `Verify*`, `Query` and
  `RunDocumentQuery` answer — are now on
  [docs/reference/wire-contract.md](docs/reference/wire-contract.md), each with a request and
  the response captured verbatim from the service, and with a short illustrative decoder in
  each of the four languages that is explicitly not a shipped client.

- **`%features` reads out a whole object tree, as text or as JSON.** A large run could not
  be read out: the listing stopped at 200 lines with `… (listing truncated)`, so the
  counters two levels under a context of twelve parts were simply absent, and there was no
  flag to see them and no machine-readable form. `%features <name> all` now lifts both the
  size and the nesting bound and lists the tree in full; `%features <name> depth <n>`
  expands nesting `n` levels and names what it left alone (`machine : Machine (not
  expanded: depth 1)`); and `%features <name> json` writes the object and everything
  reachable from it as one document in the shape the API's `Instantiate` returns
  (`instance`, `instances`, `diagnostics`), so a client reads the same shape whether it
  asked the service or the prompt. The default stays bounded — reading a feature value
  builds the objects it holds, so an unbounded listing costs objects, not just output — but
  a listing that is cut short now says how to see the rest (`… (listing truncated;
  %features ctx all shows it whole, %features ctx depth <n> to a depth)`), and a JSON graph
  cut short at 1000 objects carries the same advice as a `warning` diagnostic. The options
  work in a piped session (`printf '%%instantiate ctx\n%%features ctx all json\n' | sysml
  model.sysml`), which is how a script gets the complete state of a run
  (Open-MBEE/OpenSysML#93).

- **The bundled standard library opens in the editor.** Go-to-definition, find-references
  and the diagram panel used to report a standard library declaration at a path no editor
  could open, so a click on `ScalarValues::Integer` went nowhere. `sysml-lsp` now reports
  such a location under the `sysml-stdlib:` scheme — the file's path within the library —
  with its line and column computed from the bundled text, and serves that text through the
  `opensysml/stdlibContent` request, announced as `openSysmlStdlibContent` in the
  `initialize` result. The VS Code extension registers a provider for the scheme, so
  <kbd>Ctrl</kbd>+click on a library name opens the bundled file in a read-only editor on the
  declaring line; hover, go-to-definition, the outline and semantic highlighting work inside
  it, so navigation continues from one library file into the next. Opening or closing such a
  document changes nothing, and an edit to one is refused with an error rather than applied:
  the library is what every diagnostic is judged against. Other LSP clients get the same by
  registering a content provider for the scheme that calls the request.

- **A model's change set applies to a live repository, keyed by identity.**
  `sysml model.sysml -sync-apply http://localhost:8083` diffs the model against its project
  branch on a running SysML v2 API (Flexo MMS) and writes the change set as one commit through
  the service's own commit path: a rename, move or retype under a retained id is an update of
  that element — never a delete plus a create — a new id is a create, and a delete goes only
  when the run confirmed deletes with `-sync-confirm-deletes`. A change set holding a conflict
  or an unconfirmed delete is refused, as a typed error, before any write; nothing is resolved
  silently. On success the commit the service names becomes the last-seen commit in
  `<model>.sync.json`, never in the notation, and it is the baseline of the next run, so
  repository changes made behind the sync's back surface as conflicts and a second apply finds
  nothing to change. An apply that finds nothing to change still records the branch head it
  compared against, so a model first pushed by other means gets its baseline from the first
  run. The change set is computed at one head commit, and the commit is refused if the branch
  has moved since — someone else's edit between the read and the write is a stale-head error
  to diff again after, not a silent overwrite. `-sync-diff` takes the same endpoint URL and
  stays a dry run; with neither flag nothing is written. A bearer token never goes over
  plaintext `http://` to a host other than this machine: the compose stack on `localhost` works
  as documented, anything else needs `https://` or an explicit `FLEXO_ALLOW_PLAIN_HTTP=1`. An
  apply that mints ids writes the `-sync-annotate` model only after the commit holding them
  lands, and is refused when a minted element has no name to annotate — an id the notation
  cannot keep would be minted again on the next run. The exit status keeps its contract: 0
  applied or nothing to do, 1 a refusal or a repository failure — a read the stack would not
  answer included, reported with each change's fate — 2 an unusable run. Both sides of the
  diff are compared under what the service can store, so the properties it has no place for
  are reported as not compared rather than diffed forever. The opt-in Flexo harness measures
  the apply against the real stack — an initial load, a revision with a retained-id rename and
  gated deletes, a conflict staged behind the sync's back — and records what read back at the
  recorded commit ([the report](internal/interop/flexo/testdata/identity_apply_expected.txt)).
- **Action and state execution has a referee outside the executor.** Six conformance cases —
  a join fed by branches of unequal length, a join fed twice over one succession, a node two
  successions reach, two fork branches writing one feature, the specification's `ChargeBattery`
  merge loop, and a transition's guard, exit, effect and entry made observable through values —
  carry expected outcomes and traces derived by hand from the Kernel Semantic Library
  (`Occurrences.kerml` `HappensBefore`, `Performances.kerml`, `ControlPerformances.kerml`,
  `StatePerformances.kerml`, `TransitionPerformances.kerml`) and the Systems Library
  (`Actions.sysml`), not recorded from the executor. The derivation, sentence by sentence, and
  the orderings the library leaves open are in
  [the semantic oracle record](docs/project/behavior-semantic-oracle.md). Three cases state what
  the executor does not do yet and are listed as known failures rather than recorded as goldens:
  a join fires on the count of parked tokens instead of one per incoming succession and then
  deadlocks, a node reached over two successions is performed twice, and a merge admits one
  traversal per run so a loop is left after its first pass. The executor is unchanged; the
  compliance rows for the join, the merge and concurrent same-feature writes cite the cases and
  stay approximate.
- **The RDF round trip is measured over every example, and pinned per file.** `TestCorpusRoundTrip`
  converts each of the 345 models under `examples/` — the committed models, the OMG training
  corpus and the three pilot corpora — notation → Turtle → notation → Turtle and compares the two
  graphs as triple sets, so a writer or encoder change that moves any file's verdict in either
  direction fails the suite and is adjudicated, as the pilot corpora gate already does for
  diagnostics. The baseline records 166 files stable, 71 stable up to the whitespace inside
  `sysx:sourceText`, 14 that come back as a different graph, 15 that cannot be written back,
  2 whose written notation no longer converts and 77 refused on the first hop, each refusal
  classed by the construct it names. The mapping's reference now states that measurement in place
  of the claim that a second conversion yields the same graph, which held for the fixtures alone
  ([docs/project/rdf-corpus-roundtrip.md](docs/project/rdf-corpus-roundtrip.md)).
- **The REPL sends a signal into a running machine.** `%send go` and
  `%send Dim(level=3+4) to bulb` put the signal on the runtime's message bus exactly as a
  `send` from an action body would, so a `transition ... accept go then on` is driven from the
  prompt without writing an action just to fire it: `%events` lists the signal in flight, and
  `%step` or `%advance` dispatches it. Without `to`, the signal goes to the object whose machine
  the `%state` session is debugging, and with no session the command says so rather than
  guessing. Arguments are written `<parameter>=<expression>` as for `%invoke` and are checked
  against the signal's declaration — the feature it names, and the type and multiplicity that
  feature admits — before anything is sent; an unresolved signal name gets the usual unresolved-reference
  report, an object that runs no machine is reported as such, and a signal nothing in the
  machine's current state accepts is refused up front with the state named, never queued to be
  dropped in silence — and so is one whose every triggered transition is held back by its guard,
  decided as the dispatch would decide it with the payload bound; a guard that cannot be evaluated
  is an error. A signal the current state defers rather than accepts is sent and said to be
  deferred: the step dispatching it holds it, `%events` lists it as held, and it is recalled to
  fire once the machine reaches a state that accepts it — as a machine now holds any message
  addressed to it that its active state defers, instead of leaving it on the bus. A signal in
  flight is due now, so a single step dispatches it ahead of a timer set
  for later, as a run holding time where it is would; a step that dispatches a signal no transition
  fires on, because the state or the data its guards read changed since it was sent, says so.
  When an object runs several machines, `%send` decides the signal with each of them and reports
  which would fire on it, and a machine whose guards would drop it leaves it in flight for a
  sibling that fires on or defers it — at the prompt and in a run alike — so the machine `%send`
  named as accepting a signal is the one that gets it.

- **The REPL addresses an object by id and by path, not only by name.** Every command that takes
  an object — `%features`, `%invoke`, `%eval in`, the object `%action` and `%state` work on, and
  the one `%send … to` delivers to — reads the same reference: the name the object was
  instantiated under, the id `%instantiate` printed (`%features #3`), or either followed by a path
  into the objects it holds (`car.fl.hub`, `#3.fl`) — parts, ports, connectors and structured
  attributes alike, every feature the runtime holds an object for — one element of a multi-valued
  feature picked by an index counted from 1 (`car.wheels[2]`). In a path `.` and `::` mean the same thing, except that what follows a `.` is
  always a feature of the object before it, never a declaration. The id is the object's identity
  for the session: it survives the carry-over an unrelated declaration triggers, and a second
  `%instantiate` of the same name, which re-points the name and now says how the first object is
  still reached; `%instances` lists such an object as `#3 (ID: 3, displaced from Demo::car)`, and a
  `%state` or `%action` session started on it stays with it under that id; a connector `%features`
  has shown, anonymous or named, keeps answering to its id across that carry-over though its ends
  are only attached again when it is next read — and a connector attached whole is kept, its
  writes with it, when an older object's behavior then fails answering it, that failure reported
  as the older object's; changing the run bounds,
  which drops every object as a reset does, ends such a session too, and the next `%step` or
  `%advance` says so. The old object still counts: a `%constraint`, `%requirement` or `%eval` that
  names no object and whose condition both carry says so and names both (`Demo::car, #3`) rather
  than answering about the new one — the elements of a multi-valued part among the carriers, each
  by its index (`car.wheels[2]`) — and `%state #3` debugs a state machine the session holds by id
  or path as it does by name. A nested object is reported with its features after `.`
  (`Demo::car.fl`, `#3.wheels[2]`), which typed back reaches that object even when the `::`
  spelling names a declaration of its own. A bad reference is reported in the same words by every
  command: an unknown id lists the ids there are, a segment that is no feature names the object
  and its features, an attribute at the end of a path says it holds a value, and a multi-valued
  part with no index says how many objects it holds and how to pick one. <kbd>Tab</kbd> completes
  references where a command takes one: `#` offers the ids, `car.` the objects `car` holds — a
  variation among them once a command has read which variant it selected, and of a part nothing
  has read yet the elements it will hold, the parts subsetting it counted before its lower bound,
  so an optional or abstract part is offered only once something subsets it
  ([reference](docs/reference/repl-commands.md#object-references)).
  Names that need quoting are completed as the notation writes them, `'the ra` to `'the rack'` and
  `Q::'the ra` to `Q::'the rack'`, the closing quote typed or not, and every object a command
  reports is spelled that way too, so a name that merely looks like an id or an index (`Demo::'#3'`,
  `car::'hub[2]'`) reads back as the name it is, and one holding `::` inside its quotes
  (`Demo::'left::right'`) stays one segment rather than reading back as two names.

- **Editor navigation on an ambiguous call names every tied overload, and never one of them.**
  A call the checker reports `invocation-ambiguous` used to navigate to whichever declaration
  name resolution found first. Go-to-definition now lists each overload the arguments leave
  tied, hover names them all with their qualified names, find-references on any one of the
  overloads leaves the ambiguous call out, and rename does too — the call is not rewritten to
  a name it was never bound to, and starting a rename from the call itself is refused with
  `the call is ambiguous between several overloads`. A call that selects one overload still
  navigates, lists and renames as that overload.

- **A parser benchmark over a real model, and its Apollo 11 figures on the landing page.**
  `BenchmarkParseModel` in `internal/core/parser` parses every `.sysml` and `.kerml` file under
  the directory `OPENSYSML_BENCH_MODEL` names, with no library, resolution or validation, so the
  parser's own cost is measurable apart from a load's. Its figures for the public Apollo 11 model
  (8 ms to parse, 0.37 s to validate) close the landing page and open the README, and
  `docs/internals/performance.md` records the measurement, the commands to repeat it, and what
  the run reports about the model.

- **An Array, a vector and a vector quantity are runtime values of their own.** A
  `Collections::Array` usage shaped by its `dimensions` and `elements` evaluates to an Array
  value printed `Array(2, 3)[1, 2, 3, 4, 5, 6]`, whose `rank`, `flattenedSize`, `dimensions`
  and `elements` read out and which `CollectionFunctions::'array#'(a, (2, 1))` and
  `a#(2, 1)` index in row-major order, one-based, as the pilot evaluator does; an index count
  other than the rank, an index outside its dimension and an `elements` list that does not
  fill the `dimensions` are typed errors. `VectorFunctions::VectorOf`, `CartesianVectorOf`,
  `CartesianThreeVectorOf` and every vector operation answer a vector printed `⟨1.0, 2.0⟩`,
  distinct from the sequence `[1.0, 2.0]`, so `VectorFunctions::sum`/`sum0` over a sequence
  of vectors and the library feature `cartesianZeroVector` — the 1-, 2- and 3-dimensional
  zero vectors — now evaluate instead of flattening or failing to write a Real into a
  `CartesianVectorValue`. `VectorCalculations::scalarQuantityVectorMult`,
  `vectorScalarQuantityMult` and `vectorScalarQuantityDiv`, and the `*`/`/` operators between
  a scalar quantity and a vector, answer a vector quantity printed `⟨2.0, 4.0⟩ [m]` whose unit
  is composed by the same rule as the scalar quantities'; a vector of no components takes no
  unit, as a quantity's `num` is `Number[1..*]`. `inner`, `norm` and `angle` over vector
  quantities answer the `Number` the library declares — the magnitude over the components, the
  unit dropped by declaration — so a `Number` feature takes them, as the checker already allowed. A vector binds to a feature typed by any
  `NumericalVectorValue` specialization whose fixed dimension and element type it fits — a
  model's own `:> NumericalVectorValue { :>> elements : Integer; }` as much as
  `CartesianThreeVectorValue` — and the refusal names the declaration it fails; an object of
  such a specialization reads as a vector that keeps its own members (`t.tag`), directly, through a
  calc parameter, on a calc's result, and after a `%load` carries the object a run wrote it into
  over into the re-analysis. Each new kind is handled wherever
  the runtime, REPL, traces, solver and gRPC bridge inspect a value's kind, and a test
  enumerates the kinds so a future one cannot be left out. Tensors, coordinate
  transformations and a measurement reference passed as an argument value stay typed
  unevaluable, each with the reason.

- **An `assert` must reference a constraint, and an `assign` must name a feature.** `assert c`,
  `assert not c` and `assert constraint c` now report `assert target must be a constraint usage,
  found partUsage` when `c` is not a constraint usage (a requirement usage counts, and a feature
  chain is judged by its last feature), sharing the referent-kind check `satisfy` already had.
  `assign PD := 1;` where `PD` is a part definition, a package, a datatype or any other
  non-feature now reports `An assignment must have a referent.` followed by what the target is
  declared as, instead of being accepted; an unresolved target keeps its name-resolution error as
  the first and only diagnostic, and only a feature reaches the time-varying rule. Both rules
  are refereed against the pinned pilot, which rejects the same models at the same positions.

- **Composed units render in one canonical form.** A unit an operation composes is a sorted
  product of powers of the units the operands were written in — `3 [m] * 3 [m]`,
  `(3 [m]) ** 2` and `(3 [m] * 3 [m]) / 3 [m]` print `9 [m**2]`, `9 [m**2]` and `3.0 [m]` rather
  than `9.0 [m*m]`, `9 [(m)**2]` and `3.0 [(m*m)/m]`, and `(m/s) * (kg/s)` prints
  `kg*m/s**2` — while a named derived unit stays as written (`2 [N*m]`, `18.0 [km/h**2]`). A
  product of quantities keeps the magnitude kind bare arithmetic gives, so `l1 * l1` and
  `l1 ** 2` agree. The REPL, the trace and the gRPC `unit` field all render the one text, and
  a quantity sent over gRPC composes as one written locally does: `SI::m/SI::s` times `SI::s`
  is `SI::m`. A unit text the model cannot read as a whole is read name by name, so the units
  it does declare stay factors of their own: `SI::s` composed into `metres per second` is
  `'metres per second'*SI::s`, and dividing by `SI::s` after a round trip gives the opaque unit
  back; text that is no unit name is quoted so the product reads back as itself. Only the
  trigonometric functions take a dimension-one quantity for a number;
  `IntegerFunctions::abs(1 [rad])` is a type mismatch, as its declaration says.

- **A control node's successions are validated statically.** The nine SysML v2 constraints on
  `ControlNode`, `ForkNode`, `JoinNode`, `MergeNode` and `DecisionNode` (§8.3.17) are now
  errors at validation time rather than a runtime failure or silence: a fork or decision with
  two incoming successions, a join or merge with two outgoing, a succession end whose written
  multiplicity is not the `1..1` every control node requires (`0..1` into a merge or out of a
  decision), and a control node declared outside an action definition or usage. Successions
  are counted however they are written — `first a then b;`, a member-attached `then b;`, a
  `succession s first a then b;`, a guarded or default branch out of a decision — and include
  those an action inherits from the definition it specializes, with a redefinition replacing
  the succession it redefines; a `connect`, `bind` or `flow` is not a succession and does not
  count. Each diagnostic names the node and the count or multiplicity it found and says what
  the rule requires; the runtime keeps its own structural checks and their timing. The pinned
  pilot implements only the owning-type rule, so the other eight are refereed against the
  specification and recorded as pilot gaps.
  A constraint body now parses the action statements the specification's calculation body
  allows (`assign`, `if`, loops, `send`), so a control node inside one reaches the rule; checking
  or solving a constraint that states such a statement, an action node or a succession refuses —
  `statement in a constraint body is not executed by OpenSysML` — rather than reporting a
  verdict that ignored it. A case's own action steps are its procedure and still translate.

- **A `crosses` clause is validated against the whole of KerML's cross-subsetting rules.** A
  cross subsetting is now an error unless its owner is an end feature of a type that declares
  two or more ends (`Cross subsetting must be owned by one of two or more end features`); the
  crossed feature must be a two-step feature chain that, on a binary association or connection,
  starts at the other end (`Cross subsetting must chain through an opposite end feature`), so
  `end a : A crosses b;` and `end a : A crosses a.x;` are reported where before only a chain
  through a non-end was; a feature may cross at most once (`At most one cross subsetting is
  allowed`, reported on every clause after the first, as the reference implementation's source
  intends where its pinned build only crashes); and an end that redefines another end — by a
  `redefines` clause or by its position in an association that specializes another — must
  cross a feature that specializes what the redefined end crosses (`Cross feature must
  specialized redefined-end cross features`). The rules read the same for KerML `assoc` and
  `connector` ends and for SysML `connection def` and `connection` ends. A cross feature an
  end declares in its own body (`end a : A { member feature x : B; }`) or inline ahead of
  itself (`end x [0..1] feature a : A;`) now implicitly subsets the cross feature of each end
  its owner redefines, as the specification's implied specializations require, so the n-ary
  association examples of the reference stay silent; the inline cross feature is a member of
  its end, so `A::a::x` resolves to it.

- **An invocation that binds one parameter twice is reported at the type tier.** `F(x = 1, x = 2)` is `F binds parameter "x" twice`, judged by the parameter the name resolves to rather than by its spelling: a positional argument followed by a named binding of the same parameter, a short name, an alias, a qualified name or a redefining name of the same parameter counts as the same binding, in KerML function calls, `calc` usages, `action a = A(…)` and `perform action a = A(…)` alike (KerML 1.1 §8.3.4.8 `validateInvocationExpressionNoDuplicateParameterRedefinition`). Overload selection resolves a qualified or aliased named argument against each candidate the same way. The constructor rule (`validateConstructorExpressionNoDuplicateFeatureRedefinition`) now also reaches a constructor written as a feature chain's operand, `send new Sig(p = 1, p = 2).p to r`.

- **Every enumeration definition is a variation, and its enumerated values are its variants.**
  `enum def F :> E;` is now rejected as a variation specializing another variation, as are an
  `enum def` specializing a `variation`, a `variation` specializing or typed by an `enum def`, and
  a member of an `enum def` that is not an enumerated value; the messages name the implicit
  variation and the fix. The reflective `isVariation`/`isVariant` read by element filters, the
  variant queries (`VariantsOf`, `SelectsVariantOf`) behind the runtime and the solver, and the
  RDF export (`sysml:isVariation`, `sysml:isVariant`, `sysml:variant`) all derive the same facts
  for an enumeration definition and its values, and the RDF import reads them back without
  inventing a `variation` or `variant` keyword the enumeration grammar has no place for. A usage
  typed by an enumeration still holds one of its values, as any attribute does, and an enumerated
  value still evaluates to its literal. A declaration an enumeration body cannot own (a nested
  definition, package, import or alias) is reported by `enumeration-body-member`, and every
  `EnumeratedValue` form the grammar admits — typed, anonymous, redefining, with a default or
  initial value, behind visibility or prefix metadata such as `#$::P::M` — parses as an
  enumerated value rather than an attribute.

- **Overriding a binding feature value is an error.** A value written with `=` is a binding,
  fixed for every feature that redefines the valued feature; only a `default =` value may be
  overridden (KerML 8.3.4.10.2 `validateFeatureValueOverriding`). `attribute :>> a = 2;` under an
  `attribute a : Integer = 1;` — on a redefining usage, a specializing definition, or through a
  chain of redefinitions, including the implicit redefinition of a parameter, connector end or
  case subject — now reports `cannot override the binding value of …` at the value, naming the
  bound feature and the fix (`default =` on it, or no value here), at the positions the pinned
  pilot reports. A `:=` initial value over a binding is reported the same way; a `:=` on a
  non-variable feature is a separate rule and unchanged. The parser records the value operator
  (`=`, `:=`, `default =`, `default :=`) on usages, subjects and named `assume`/`require
  constraint` declarations so the rule can tell them apart, and the RDF mapping carries it as
  `sysml:isDefault` and `sysml:isInitial` beside `sysml:value`, so a round trip no longer turns
  an overridable `default = 1` into the binding `= 1`; a graph stating either flag without a
  `sysml:value`, or stating one both true and false, is refused rather than read as one value.
  A named `require constraint c : C = c0;` or `assume constraint a : C` is now a member of its
  requirement: `:>> c` in a specializing requirement resolves, is checked by this rule and by the
  constraint-usage declaration rules, is found by the language server, and is checked at runtime
  by qualified name and through its requirement like any constraint usage, a redefinition owning a
  result expression replacing the one it redefines and one owning none inheriting it. Examples,
  fixtures and guide snippets that overrode a bound attribute now declare the base value as
  `default =`; the solver's `attribute :>> best = <expression>` objective over the library's bound
  `TradeStudies` `best` is reported and recorded as a known gap until that contract is restated.

- **An initial value or `constant` requires a variable feature.** A feature is variable when KerML
  declares it `var` (or `const`), or when a SysML usage may time-vary: owned by an occurrence type,
  not a portion, and not a composite action (KerML 8.3.3.1 `Feature::isVariable`,
  `validateFeatureValueIsInitial`, `validateFeatureConstantIsVariable`). A `:=` initial value on any
  other feature — an attribute of a data type or `attribute def`, a behavior parameter, a timeslice,
  a root usage — now reports `Initialized feature must be variable` at the value, and a `constant`
  prefix on one reports `Only a variable feature can be constant` at the usage, at the positions the
  pinned pilot reports. `var attribute x : Integer := 1;` in an `item def`, `constant attribute c = 1;`
  on a part, and `:=` anywhere inside an occurrence stay silent, as does every model under `examples/`
  and the OMG corpora.
  The rule is element-scoped: an error elsewhere in the document does not silence it, only a
  lower-tier failure in the feature's own declaration or its owner's does.

- **A call selects the overload its arguments fit, and the checker and the runtime select the
  same one.** A name visible as several function or calc declarations — owned, inherited,
  imported or re-exported, a library function among them only where its package is imported — no
  longer resolves to whichever declaration is found first: the candidates are filtered by arity,
  by positional or named binding, and by argument-type conformance (the `ScalarValues` lattice,
  strings, booleans, collections, quantities and declared types), and the most specific fit is
  selected. `ToInteger("7")` is `IntegerFunctions::ToInteger`, `abs(-2)` answers `2` and
  `abs(rect(3.0, 4.0))` answers `5.0` through `ComplexFunctions::abs`, where the unqualified
  name used to be rejected with `expects Real, found String` or `requires a numeric value`. A
  genuine tie is reported as `invocation-ambiguous`, naming the tied candidates, and refused at
  runtime as `ErrAmbiguousInvocation` rather than dispatched silently; a call no candidate fits
  keeps its argument diagnostic and names the declarations considered. The selection is
  memoized in a side table keyed by the invocation node and scope, and the evaluator dispatches
  the declaration recorded there. A model's own `calc` of the name still shadows the library,
  and an argument whose type is statically unknown keeps the previous selection with no new
  diagnostic. `ComplexFunctions::sum` and `product`, which a Real collection now selects where
  `ComplexFunctions` is imported and `RealFunctions` is not, fold Real elements as the library's
  `reduce '+'` does — on the real axis — so `sum((1.0, 2.0))` stays the Real `3.0` rather than
  becoming `3.0 + 0.0i`. A feature typed by a calc (`ref pick : Twice;`) is a candidate beside
  a same-named calc, performing the calc it is typed by, so the call whose argument only its
  signature fits selects and runs it. An explicit empty action call, `action call = tag();`,
  binds nothing: a required input stays unbound and a defaulted one takes its default, where it
  used to read the caller's same-named value as a bare `perform tag;` does.

- **Every Kernel Function Library declaration is dispatchable by name.** All 17 vendored
  packages, and `OpenSysMLMathFunctions`, are gated: each calc or function declaration —
  the operator-named ones included — either computes or names itself as unevaluable with the
  reason, so `RealFunctions::ToReal("1.5")` is `1.5` rather than "calc has no return
  expression" and `NumericalFunctions::sum0((1, 2, 3), 0)` is `6` rather than an unresolved
  `+`. New: the conversions `ToString`, `ToBoolean`, `ToInteger`, `ToNatural`, `ToRational`
  and `ToReal` of every package that declares them (a String that is not a notation of the
  type, a negative given to `ToNatural` and a value outside the Integer range are typed
  errors; `ToString` of a Real is the shortest decimal that reads back as the same Real, so
  `ToReal(ToString(x)) == x`); `RationalFunctions::floor`, `round` and `gcd`
  (`gcd(0, 0)` is `0`, a negative operand is taken by magnitude); `RealFunctions::re`, `im`
  and `arg`; `NumericalFunctions::sum0`/`product1`, which answer the identity they are given
  for an empty collection; `DataFunctions`/`ScalarFunctions::max`/`min` over numbers,
  strings and quantities; every operator as a function — `IntegerFunctions::'+'(1, 2)`,
  `DataFunctions::'=='(1, 1)`, `BaseFunctions::'#'(xs, 2)`, `ScalarFunctions::'..'(1, 3)`,
  `BooleanFunctions::'not'`/`'xor'` — each evaluated by the operator's own code, with each
  package's parameter types imposed (`IntegerFunctions::'=='(2, 2.0)` is a type mismatch where
  `BaseFunctions::'=='` answers `true`) and `NaturalFunctions::'/'` answering the Natural it
  declares (`'/'(6, 3)` is `2`; `'/'(7, 2)` is a domain error, not `3.5`); and
  `ControlFunctions::'if'`, `'and'`, `'or'`, `'implies'` and `'??'`, which evaluate only the
  operand they select and accept an omitted second operand when the first decides. Built-in
  functions bind named arguments (`sum0(zero = 0, collection = xs)`), bind null to every
  `[0..1]` parameter a call leaves out, trailing ones included (`size()` is `0`, `'if'(false)`
  null, `subsequence(seq, 2)` runs to the end), and a model's own calc
  named like a library function — a collection built-in, a conversion, an operator form or
  `sqrt` alike — is no longer answered by the implementation of that name, with or without a
  body of its own. A body
  passed on through an `expr` parameter (`Keep(xs, { in x; x > threshold })` with `Keep`
  doing `xs->select pred`) is applied in the scope it was written in, so it reads its writer's
  `threshold`, and one a control function selects is applied rather than answered as a body;
  a body a calc returns keeps the parameter it closes over after that calc has returned, and
  one that names an output of that calc (`out threshold = n; out pred : expr = { in x; x >
  threshold }; bind result = pred;`, or a usage nested in its body) still works it out from
  the invocation's own parameters once the frame that invocation ran in has been reused.
  A nonzero Real notation too small for a Real (`1e-400`, as a literal or through `ToReal`)
  is an overflow error rather than `0.0`, and only decimal notation is a Real at all (`NaN`,
  `Inf` and a hexadecimal float are invalid notation wherever a Real is read, a compiled
  calculation's command-line arguments included). A `RealFunctions`
  operator form binds an Integer argument as the Real it equals and answers a Real
  (`RealFunctions::'+'(1, 2)` is `3.0`, `RealFunctions::ToRational(2)` is `2.0`; a product too
  large for an Integer stays finite), while
  `RationalFunctions` keep an Integer's kind as their `abs`/`max`/`min` do. A direct
  invocation of a built-in through the runtime API (`InvokeCalc`, `InvokeCalcNamed`) binds
  and computes as the written call does, a body value handed to an `expr` parameter applied
  only when selected.
  `DataFunctions::'=='`/`'==='` take DataValues only: a part or other occurrence is a type
  mismatch, where `BaseFunctions::'=='` compares anything. The equality and identity forms
  hold their `[0..1]` operands to one value: an empty collection is null (`'=='((), null)` is
  `true`, as `() == null` is) and two or more values are a multiplicity violation; `??` in
  either notation falls back over an empty collection, not only over `null`.
  `BaseFunctions::'#'` selects by one index and reports several, which address an Array.
  `RationalFunctions::rat`/`numer`/`denom` (a Rational is a float64 here),
  `CollectionFunctions::'array#'`, `BaseFunctions::'['` and the several-index
  `BaseFunctions::'#'` (no Array value kind),
  `BaseFunctions::all`/`as`/`meta`/`istype`/`hastype`/`'@'`/`'@@'` and `ControlFunctions::'.'`
  (evaluated from their own notation, not as functions), `DataFunctions`/`ScalarFunctions::'~'`
  and every `OccurrenceFunctions` declaration report themselves by name, each holding its
  declared multiplicities (`addNew(occ = o)` omits the `[0..*]` group; a call missing a
  required parameter is an arity error first). An operator-named
  function reports itself as the model writes it (`IntegerFunctions::'+'`).

- **Metadata annotations are checked against the metaclass they name.** A KerML metadata
  feature (`@M`, `@M about …`, `metadata m : M`) must be typed by exactly one metaclass — an
  ordinary class, structure or data type is reported `Must have exactly one metaclass` (KerML
  `validateMetadataFeatureMetadata`) — and a SysML metadata usage by exactly one metadata
  definition (`A metadata usage must be typed by one metadata definition.`,
  `validateMetadataUsageType`), where a part definition was accepted before. The elements an
  annotation may be applied to are now read from the metaclass's own `annotatedElement`
  features — declared, inherited, redefined or subsetted, resolved through the reflective `KerML`
  library rather than a fixed list of kinds — and each annotated element, in the `@M` and the
  `@M about …` forms alike, is reported `Cannot annotate <Metaclass>` when its metaclass does not
  conform (`validateMetadataFeatureAnnotatedElement`). A feature written in an annotation body,
  in KerML as in SysML and at any nesting depth, must redefine a feature of the metaclass or of
  one it specializes: an explicit `:>> g` naming a feature elsewhere is reported `Must redefine an
  owning-type feature` (`validateMetadataFeatureBody`), which previously applied only to the SysML
  `metadata … : M` usage form. Model-level evaluability of a metadata value follows the pinned
  pilot: an unfeatured feature is as evaluable as its own value, a feature of another type is not,
  and a metadata feature always is.

- **Each nested action node performs in a frame of its own.** An action node is a performance
  (`Actions::Action :> Performance`, `subactions :> subperformances`), so the parameters and
  attributes it declares, and those of the action it performs, now live in a frame the node's
  performance holds rather than in the enclosing action's one feature space. Two nodes each
  declaring `out v` no longer overwrite each other; `assign total := p.v + q.v;` reads each
  node's pin, and so does `leg.inner.v` through two levels; `bind add.a = x;` and
  `flow p.v to q.w;` address the pins they name; a typed node's body-local `in a = 3;` seeds its
  own input; `action add = Adder(3, 4)` and `Adder(a = 3, b = 4)` bind the callee's inputs by the
  callee's own parameter order and names, inherited and redefined parameters in their effective
  order — never by what the caller happens to name alike — and the untyped `add` read as a value
  is the callee's `return` parameter, or its `out result`. The callee binds the supplied inputs
  before it evaluates its defaults in declaration order, so `in b : Integer = a * 2;` reads the
  `a` the caller passed; likewise a `bind` at a node's input pin takes precedence over the value
  the node's own declaration states. An invocation's arguments are the enclosing action's
  expressions, so `Adder(a, 1)` reads the caller's `a` even when the node's own pin `a` already
  holds a bound value. A binding at an undirected attribute of a node is kept at both ends: the
  node reads the other end's value as it begins, and what it changed is carried back as it ends,
  to the enclosing attribute or on to a downstream node's pin. A binding end that chains through
  an object, `bind add.sum = holder.inner.mark`, writes the feature of the object the chain
  reaches, typed as an assignment through it is. A performance's bindings and
  defaults are one evaluation of their own: a calc usage two of them read answers once, and the
  next performance of the node evaluates it anew. Two tokens performing one node at once each hold a frame of their own,
  take what flows delivered to its pins oldest first and send their own outputs on. A nested
  body still resolves a name it does not declare lexically to the enclosing action's feature and
  writes it in place, so a grandchild writing `legs` keeps working, and a perform usage on a
  part keeps its occurrence slot. Reading a pin before its node has run, a pin the node does not
  declare, a surplus, missing, unknown or repeated argument, a binding at a non-parameter or
  into a feature no enclosing action holds, and two bindings at one input pin whose other ends
  disagree are typed errors
  (`ErrNodeNotPerformed`, `ErrNodePin`, `ErrActionArity`, `ErrUnboundParameter`,
  `ErrUnknownParameter`, `ErrDuplicateArgument`, `ErrBindingEnd`, `ErrBindingConflict`). `Results()`, the REPL's
  `%continue`/`%tokens` and a gRPC execution response report a node's pins under its path
  (`p.v`); `Data()` stays the action's own performance. Kept for compatibility: a bare typed usage `action call : Callee;`
  still reads an unbound `in` from the same-named enclosing feature — `Callee()` passes nothing
  and lets the callee's defaults apply — and every invocation form still returns its `out`
  values into same-named enclosing features that exist. Not yet: `n.pin` on an untyped
  `action n = Callee(args)` is refused by name resolution, which does not type `n` by the
  invocation; read `n` itself, or type the usage. An action declared in an `if` branch or a
  loop body is a performance of its own like any other node, with the block's locals (a loop
  variable) in reach, so a sibling in the branch reads its pins as `p.v` and `Results()`
  reports them under its path (`iterate.square.s`), the latest iteration's standing for it; a
  `bind` or `flow` written in the branch or body at such a node's pin is applied per
  performance (`bind dbl.a = i` seeds each iteration's node from its loop variable) where
  before it failed as a statement a body cannot run; where both branches declare a `p`, a read
  of `p.v` in a branch is of that branch's node. A typed or invoked node adopts the subactions
  of the action it performed, so `call.inner.v` reads a pin inside the callee through it and
  `Results()` reports it under `call.inner.v`. A node in a state's entry/do/exit or transition
  body performs in a frame of its own as one in an action body does, with the machine's data and
  the enclosing states' attributes in reach, so a sibling reads its pins as `p.v`, two nodes'
  same-named pins keep apart, and a `bind` or `flow` the body states at its pins — from a state
  attribute into a pin, between two nodes' pins, or an output back to a state attribute — is
  applied; before, such a connector was reported as a statement a body cannot run and `p.v`
  found no `p`. A typed action node in a branch or loop of a
  state's entry/do/exit body performs the action it names with the `in` values and arguments
  it states, holds every pin of the callee as it ended (an argument overriding the node's own
  default included) for its body to read, and returns its `out` values to a same-named block
  local, state attribute or state datum that exists — before, the node's `in x = i` and body
  statements were skipped and the callee saw only state data. A pin such a node in a state's or a
  calc's body declares without a value (`out v : Integer;`) is the node's own too: its body's
  `assign v := 1` writes that pin, not a same-named state attribute or calc local — before, the
  write reached the attribute, or was refused in a calc as a name it never declared. In an action
  body as in a state's,
  those `out` values return once the node's own body has run, so a body that rewrites an output
  returns what it wrote rather than what the callee produced. A typed or invoked node a derived
  action inherits resolves its callee where the node was declared, so one visible only to the
  general action is found rather than reported unresolved, and a `bind` or `flow` the general
  action states at that node's pins applies to the derived action's performance of it, in the
  general action's scope — before, only the derived action's own connectors were lowered and the
  inherited node ran with its input unbound. Such a connector follows the node's declaration: a
  node the derived action declares of its own under the inherited node's name does not take it
  (the connector lowers to nothing, and the replacement's same-named pin stays unbound), while
  a node redefining the inherited one (`action add :>> add`) does. A binding between two of the
  general action's nodes holds at both or at neither: when the derived action replaces the node
  at one end (`bind add.a = src.n` with a `src` of its own), the other end no longer reads the
  replacement's pin by name. An action declared in a branch or loop body that
  states a flow of its own (`first`, successions, forks and joins among its nodes) now runs that
  flow to completion in its frame, and its steps spend the action's own token-flow budget
  (`OPENSYSML_MAX_ACTION_STEPS`, `ErrActionStepLimitExceeded`) as the enclosing flow's do;
  before, its `first` was reported as not executable. The
  arguments of `action n = Callee(a = 3) { in a = ...; }` bind the node's pins before its own
  defaults are evaluated, so the default an argument replaces is never evaluated and one the
  node keeps (`in b = a * 10;`) reads the argument — before, the replaced default ran first and
  its failure was reported. A
  `perform` in statement form and a
  state's entry/do/exit action now refuse an `in` without a default that nothing binds
  (`ErrUnboundParameter`) instead of failing later inside the callee. A
  default an action inherits from a generalization (`action def Derived :> Base` with Base's
  `in x : Integer = 3;`) is seeded when the performance starts, as an owned default is, evaluated
  where it was declared and reading a parameter the action redefines as redefined — before the
  fix it was recomputed on every read, so a body that reassigned `x` saw `z = x * 2` change with
  it, and the inherited feature never appeared in `Results()`. A `bind` at a pin two levels
  down, `bind leg.inner.w = x;`, is lowered with the whole path it names, so it seeds `inner`'s
  `w` and not a `w` of `leg`; before, the path collapsed to `leg.w`. A binding between a nested
  pin and a pin of the node around it or of another node under it, `bind leg.inner.v = leg.v` or
  `bind leg.inner.v = leg.rest.n`, holds within the one performance of `leg` the nested node
  runs in: `leg.v` takes `inner`'s value as `inner` ends, and two tokens performing `leg` at
  once each hand their own `inner`'s value to their own `rest` — before, the value went to the
  latest performance of `leg`, or was queued for one yet to come, so `leg.v` ended unvalued and
  concurrent performances swapped values. A debugger breakpoint on a node an `if` branch or a loop body declares now pauses the run before the node performs, once
  per performance, and `%step`/`Step` resumes it; `NodeNames()` lists such nodes, so `%break add`
  on a loop body's node is accepted — before, the name was refused and the node never paused.
  Such a pause ends the step at once: a token forked alongside the paused one takes no step of
  its own in that call, and is stepped first by the next one. A REPL session ended while paused
  there — by `%stop`, another `%action`, or a redeclaration of what it debugs — releases its
  executor (`ActionExecutor.Release()`), ending the paused work rather than holding it suspended.

- **`OccurrenceFunctions` evaluate: `'==='`, `isDuring`, `create`, `destroy`, `addNew` and
  `addNewAt` answer over the runtime's occurrences.** Every object the runtime materializes and
  every action or state performance it runs now has a lifetime, kept in a side table in the
  runtime's own execution order (never wall-clock time), so no library frame member is added
  to an object and `%features` keeps its shape. `a === b` and `OccurrenceFunctions::'==='(a, b)`
  agree: `true` only for one and the same occurrence, so two structurally equal parts are
  `!==` while their attributes are `==`. `isDuring(occ)` is `true` while `occ` is alive at the
  evaluation — an object until it is destroyed, a performed action or exhibited state until it
  completes (a performed action stating no flow takes its inputs and is complete at once, so
  `%features` lists it `completed` rather than `not started`, and a binding it cannot take —
  an `out` parameter or a name it does not declare — fails the performer as it does a flowed
  action). `create(occ)` begins an occurrence the call is the
  first to reach; `destroy(occ)` ends it with the parts it owns, after which `isDuring` is
  `false`, any feature read or write
  is `occurrence was destroyed` rather than a stale value, so is any operation invoked on it or
  behavior performed by it (it sends no message), `%features` prints the destruction
  and `%instances` marks the object `destroyed`; `addNew`/`addNewAt` create and insert into an
  ordered group, an index outside `1..size + 1` being `index out of range`. A data value, an
  empty or several-valued argument, a second `destroy`, or an object whose behavior is still
  performing is a typed error naming the function and the parameter. The execution trace
  records `create:` and `destroy:` events.
- **Known limitation:** `addNew` and `addNewAt` answer the group after insertion rather than
  the declared `occ`, since an expression call cannot write its `inout group` argument back;
  write the result with `assign spares := addNew(spares, spare)`.

- **A member is rejected outside the body kind that owns it.** `subject` and `actor` belong to a
  requirement or case body, `stakeholder` to a requirement body, `objective` to a case body,
  `entry`, `do` and `exit` to a state body and `render` to a view body (SysML v2 `RequirementBody`,
  `CaseBody`, `StateBody` and `ViewBody`). Written anywhere else — a part, an action, a package, a
  nested usage of another kind — the parser now reports an error naming the owning body and the
  fix (`'actor' declares an actor of a requirement or case and is only allowed in a requirement or
  case body; move it into the requirement or case it belongs to`) where it used to accept the
  member silently, or, for `entry action init;`, read it as a plain action. The OMG pilot rejects
  the same models at the same tier. Every legitimate state form is unchanged, including `entry;`,
  `entry; then s;`, inline and braced entry/do/exit actions, transitions and nested or parallel
  states, and a member inside an `include`, `perform`, `exhibit`, `frame` or `satisfy` body is
  judged by that body's kind.

- **The Quantities and Units domain library's calculations compute over quantities.** Every
  `QuantityCalculations` declaration dispatches to the runtime's unit-aware arithmetic:
  `sqrt(9 [m**2])` is `3.0 [m]`, while `sqrt(9 [m])` and `sqrt(9 [rad])` — an angle is
  dimension one, but a named unit all the same — are a typed error (`unit has no root`)
  rather than a magnitude in a fractional unit; `abs`, `floor` and `round` keep the unit;
  `max`/`min` convert to compare and answer the winning operand as written; `sum`/`product`
  fold in the first element's unit; the operator, comparison, predicate and conversion forms
  delegate to the code the operators already use. `import QuantityCalculations::*;` — which the
  ISQ examples do — no longer breaks `(1 [m], 2 [m])->sum()`, which computes `3 [m]` with the
  import and without it (where `sum` is the `NumericalFunctions::sum` the model imports). The
  `TrigFunctions` take an angle quantity (`sin(90 ['°'])`, `cos(0 [rad])`) through its declared
  scale, and only an angle: a bit, a byte, a steradian or `one` is dimension one but no angle,
  and is a type mismatch. `VectorCalculations` over a numeric vector
  compute as the Kernel `VectorFunctions` do; the quantity-scaled vector forms, `outer`, and
  every `MeasurementRefCalculations` and `TensorCalculations` declaration report themselves by
  name with the reason instead of `no result expression`. A parameter these libraries declare
  as `in : Type` binds by the name of the general's parameter it implicitly redefines
  (`VectorCalculations::angle(v = a, w = b)`), and where there is no general it is anonymous:
  it binds by position only, a named call is `ErrUnknownParameter` listing it as `#1`, and the
  registry publishes no name for it. A gate asserts every declaration of the four packages is
  either computed or named, parameters by effective name in declared order.

- **A document-query parameter default is used when the caller leaves it unbound.** A query
  could declare `in root : Element = telescope;` or `in pattern : String default "m";`, but
  the plan recorded only that a default existed and execution refused the unbound parameter
  with `relies on a default not retained in the plan`. The plan now retains each default as a
  compiled expression together with the query that declared it, under the rule `%run-query`
  already applied to bindings: a default naming a model element binds that element, any other
  default is evaluated — once per query execution, before any row is produced, in the scope
  of the query that declared it, within the same visit and invocation budgets as the query
  body. Inherited defaults apply, a redefining parameter's default replaces the inherited one,
  the value passes the same type and multiplicity checks as an explicit binding — what the
  default's text already settles (a literal or named element of the wrong type, a list or
  invocation whose size cannot fit) is refused when the query is planned as
  `document-query-default-type` or `document-query-default-multiplicity`, the rest when the
  default is evaluated — and an explicit binding still overrides. `%run-query`, `-run-query`, `RunDocumentQuery`,
  a document content block that leaves the parameter unbound, and a query invoked by another
  query all take the default through the one executor. A default written in a form the query
  expression language has no operation for is refused when the query is planned
  (`document-query-unsupported-default`, naming the parameter) rather than at execution;
  the `default-unavailable` execution failure no longer exists.

- **A query expression may name a model element wherever a value is expected.** A name in
  a default, a list, or an operation or named-query argument that is not one of the query's
  parameters now binds the element it denotes — `OwnedElements(source = telescope)` starts
  from that part, in a default or in the query body alike — where planning previously refused
  it as an unknown parameter. The element is checked against the receiving parameter's type
  and multiplicity when the query is planned — an element is never a data value, so an
  attribute named where a `String`, a `ScalarValue` or any `attribute def` is due is refused
  rather than passed by name, while an enumeration literal (`Color::red`) is a value of its
  enumeration and binds an `enum def`-typed parameter — with the same typed `argument-type` and
  `argument-multiplicity` failures a mismatched parameter reference gets.

- **`RationalFunctions::rat`, `numer` and `denom` compute.** `rat(n, d)` is the binary64
  quotient the `/` operator computes (`rat(1, 3)` is `0.3333333333333333`, `rat(6, 4)` is
  `1.5`), and `rat(n, 0)` is the division-by-zero error `1 / 0` reports, never an infinity.
  `numer(x)` and `denom(x)` read the exact numerator and positive denominator, in lowest
  terms, of the binary64 a Rational is here — `numer(0.75)` is `3`, `denom(0.75)` is `4`,
  `numer(2)` is `2` over `1` — so `rat(numer(x), denom(x)) == x` holds for every finite `x`
  whose terms are Integers; a term past the Integer range (`denom(0.0001)` is 2^66) is an
  overflow error, an infinity a domain error. Because the value is the double nearest what
  was written, `numer(0.1)` is `3602879701896397` over `2^55` rather than `1` over `10` — the
  same class of artifact as `0.1 + 0.2 != 0.3`, matching the pinned pilot's `double`
  storage; the pilot itself evaluates none of the three, so they are self-assessed
  (`docs/project/exact-rational-evaluation.md`). Before, all three reported themselves as
  unevaluable.

- **A collection-valued property in a Turtle graph is also written as the JSON literal
  Flexo reads it from.** The `flexo-mms-sysmlv2` reader skips a `sysml:` predicate that has
  several objects and takes the property from the literal at
  `urn:sysmlv2:annotation:json:<key>` instead, so every `ownedMember`, `specializes`,
  `argument`, … a graph stated as bare repeated triples was silently dropped on read: the live
  harness measured 0 of 14 multi-valued standard properties delivered. The encoder now keeps the
  typed triples and adds one `json:<key>` literal per collection holding the whole array in the
  shape that service's own commit path stores — `[{"@id":"…"},…]` for references, JSON
  strings, numbers and booleans for literals — in the graph's deterministic order, declared
  under a `json:` prefix (`docs/reference/rdf-mapping.md` § Collections). Reading a graph
  accepts a collection stated by the annotation alone (a Flexo-produced graph), by typed
  triples alone (a graph an earlier release wrote) or by both; two spellings that disagree, or
  an annotation that is not one literal holding a JSON array, are refused with an error naming
  the subject and the key rather than one being picked. `-sync-diff` treats the annotation as
  the restatement it is, and minting ids rewrites the references inside it with the typed ones.
  Re-recorded against the live stack, the graph-load side of the harness delivers 14 of 14
  multi-valued standard properties and 369 of 452 properties overall (was 355 of 424); every
  remaining loss is a `sysx:` property.

- **Metadata annotations convert to RDF structurally, bodies and prefixes included.** The
  encoder used to carry a `#Safety` prefix as the text it was written as and to refuse an
  annotation with a body (`@Safety { level = 2; }`), the most common refusal across the corpus.
  Every annotation — `@M;`, `@M { … }`, `metadata m : M about a, b;` and the prefix
  `#M part def P;` — is now a `sysml:MetadataUsage` owned by the element it is written in or
  ahead of, carrying its type, one `sysml:annotatedElement` per target, `sysx:hasBody`, the sigil
  it was written with as `sysx:declaredKeyword` (`@`, `#`, or none for `metadata`) and its
  body's members as owned members with their `sysml:value` expression trees, so the notation is
  written back from the graph alone, without `sysx:sourceText`. A prefix's type and `about`
  targets link their element IRIs by name resolution as every other typing does, so `#safe` and
  `#$::P::Safety` link `Safety` rather than carrying the spelling as a literal (they come back as
  `#Safety`). `sysx:prefixMetadata` and
  `sysml:annotates` are gone; a `.ttl` file written by 0.4.3 that states either is refused naming
  the property rather than read without the annotation — re-export it from its notation source.
  Of the 19 files refused for this reason, 17 now convert and 13 of those come back as an equal
  graph without `sysx:sourceText`; the other four run into older gaps the refusal had hidden
  (an `isEnd` and an `isNamespaceImport` flag the writer drops, an invocation expression it
  refuses, an n-ary `connect (…)` head with no `sysx:endForm`), and the remaining two fall to
  an older refusal, an unnamed `feature` or `event` declaration. The parser now reads
  `metadata M about x;` as typed by `M` and unnamed, as the grammar's
  `MetadataUsageDeclaration` requires, rather than naming the usage `M` — which had made the
  training corpus's `Metadata Example-1.sysml` a duplicate declaration under conversion
  (`docs/reference/rdf-mapping.md`).
- **An `assume`/`require` member's constraint declaration converts to RDF.** The encoder carried
  only the condition of a requirement's `assume`/`require` members, so
  `assume constraint c : C;` and `require constraint d [1] = true;` came back as
  `assume constraint { }` and `require constraint { }`. The name, specializations, multiplicity
  and value of the constraint usage the member owns are now carried as they are for any usage,
  and a body-less member comes back with its `;` rather than an empty body. The declaration
  form states `sysx:declaredKeyword "constraint"`, which tells its `references C` from the
  reference form `require C;` — `assume constraint c references C;` used to come back as
  `assume C;`, and prefixed (`assume #Safety constraint c references C;`) it was refused.
- **Prefix metadata is written back where the grammar puts it.** A prefixed assertion came back
  as `assert not #Safety constraint c;`, which no grammar production spells, so the notation
  did not parse; it is now written `#Safety assert not constraint c;` (AssertConstraintUsage
  `OccurrenceUsagePrefix 'assert'`), and the parser no longer accepts the prefix after `assert`.
  KerML's `var #Safety feature x;` (FeaturePrefix) now parses, so a variable feature's prefix
  survives the round trip too.
- A graph that puts a prefix annotation on an `assume`/`require` member stating an inline
  condition or a constraint reference (`assume #Safety x > 0`) is refused as unsupported, since
  the grammar gives such a member no prefix position; the notation writer used to emit it unparseable.

- **A body's result expression converts to RDF and back.** `sysml -convert ttl` refused any
  calculation, analysis, verification or case whose body ends in a bare expression
  (`calc def Double { in x : Real; x * 2 }`), which is how the OMG corpora write most of theirs.
  The expression now converts as the metamodel states it: the Expression element itself, at its
  `sysx:memberIndex`, owned through a standard `sysml:ResultExpressionMembership` that states it
  as both its member and its `sysml:ownedResultExpression`, so a graph without `sysx:sourceText`
  still writes the notation back, and any Expression a `ResultExpressionMembership` owns — in a
  graph another tool wrote, with no `sysx:` term on it — is written back as its body's result.
  Expression bodies — `{ in y : Real; y + x }` as a result, nested, or as the body of an `in expr`
  parameter — carry their parameters and result structurally as
  `sysx:bodyParameter` and `sysx:resultExpression`, and a `doc` opening one as a
  `sysml:Documentation` node (the parser now keeps it); any other declaration inside one stays a
  `sysx:BodyMember` with its text, and is refused by name when the text is absent. Across the 345
  example files, none of the 13 refused for this reason is any longer: 12 convert, 11 of them to
  a graph that round-trips equal, and the 13th is refused for an unrelated `feature` declaration;
  80 of the 81 calculation conformance models convert, up from 62. See
  `docs/reference/rdf-mapping.md`.

- **A `#M` prefix on a `subject`, `assume` or `require` member now means what it means on any
  usage.** The prefix was parsed and carried through the RDF mapping but the semantic engine
  never saw it. `MetadataAnnotationsOf` now reports it, a semantic metadata keyword
  (`Metaobjects::SemanticMetadata`) makes the member specialize the keyword's `baseType`, a
  metadata definition that restricts `annotatedElement` is checked against the member
  (`Cannot annotate ConstraintUsage` on `assume #OnDefinitions constraint c : C;`, silence on
  one that admits usages), and an abstract metadata type is reported there as elsewhere.
- **The constraint usage an `assume` or `require` member owns is a declaration in its own
  right.** `assume constraint c : C;` and `require constraint r : C[0..1] { … }` now declare
  `c` and `r` in the requirement: they are found by name, typed by `: C`, bounded by their
  multiplicity, redefine what `:>>` names (and their bodies see names nested under it),
  take a constraint usage's implicit base, and go-to-definition on a `:>> c` lands on the
  member. A value the member binds (`require constraint c = false;`) is its feature value,
  read by `%eval`, materialized on requirement instances and replaced by a redefinition, as
  a constraint usage's `= expr` is. An anonymous `require constraint { … }` still declares
  nothing and its conditions still belong to the requirement.

- **A requirement's `subject`, `assume constraint` and `require constraint` members take a
  short name.** `subject <s> x : T;`, `assume constraint <a> ac : C;`, `require constraint <r> : C;`
  and the other short-name-only forms used to be parse errors, although the grammar allows a
  short name wherever a name is declared. They now parse, with or without a name, a `#M` prefix,
  typing, multiplicity, a value or a body; the short name resolves (`Req::s`, `:>> s`, from an
  expression), is checked for distinguishability against the requirement's other members, is
  exported as `sysml:declaredShortName` and read back into `<s>`, and is what the REPL and the
  editor show, navigate to and rename when the member declares no other name. A malformed short
  name (`subject <> x;`, `subject <s x;`) is a diagnostic.

- **A send's arguments are validated.** The payload, `via` and `to` arguments of a send were
  never typed, so `send Sig() to target` with `attribute def Sig` — an invocation of something
  that is not a behavior, which the pinned OMG pilot rejects with `Must invoke a behavior or a
  behavioral feature` — passed silently. A new type-tier pass infers every send argument, in
  action bodies, state entry/do/exit actions, transition effects and nested forms alike, and
  reports the SysML v2 `SendActionUsage` constraints on them: a state subaction or transition
  effect that sends no payload is an error (`send-payload-missing`), sending `to` a port warns
  that `via` is the routing form (`send-to-port`), and a `via` or `to` argument whose types are
  disjoint from `Occurrence` warns at the argument (`send-sender-not-occurrence`,
  `send-receiver-not-occurrence`). The pass is in the shared registry, so the LSP reports the
  same diagnostics. Two refereed cases join the rejection corpus under
  `cmd/pilot-reject/testdata/negative/semantic/`.
- **`send new Def(args)` constructs the message it sends.** The notation's constructor keeps its
  named arguments (`send new Telemetry(frames = 3.0) via antenna;`) through the AST, the RDF
  export and the runtime, which builds the message from the constructed definition and its
  positional or named arguments. An accept binds the constructed occurrence, whatever the
  argument count: `accept d : Data` of a `send new Data(7)` binds a `Data` whose first feature
  is 7, so read `d.value`, not `d`. An accept subsetting a
  declared event (`accept :> shutDown`) now takes a message sent from that event feature
  (`send shutDown to interrupt`), not only one of its type.
- **A constructor's arguments are checked against the type it instantiates.** `new T(…)` binds
  the type's features — its own first, then the inherited ones — by position or by label, and the
  type tier now reports a positional argument beyond them, a label bound twice, a qualified label
  naming another type's feature, and an argument whose scalar type cannot bind its feature, each at
  the offending argument. A simple label resolves as a feature of the constructed type rather than
  of the surrounding scope, so an unknown one is reported where it is written and renaming the
  feature rewrites its labels. The constructed name must be a type: `new Signals()` on a package
  is `Must have an invoked/instantiated type` at the name, and the runtime refuses an unresolved
  or non-type `new` target at the send instead of posting a message that names nothing.

- **The argument of an `accept after`, `at` or `when` trigger is typed.** An `after` delay must
  be a `DurationValue`, an `at` time a `TimeInstantValue` and a `when` condition a Boolean, as
  the SysML v2 `TriggerInvocationExpression` constraints require and the reference validator
  enforces; `accept after 5` is reported as `trigger-after-duration` with the unit-bearing
  spelling suggested (`after 5 [s]`), and `trigger-at-time-instant` and `trigger-when-boolean`
  name the other two. The judgement is semantic rather than syntactic: a quantity literal is
  typed by the unit it is written in, a feature by its declared type through inheritance,
  redefinition, aliases and feature chains (a feature declared by nothing but a value takes the
  value's type), a call by the result of the overload its arguments select, and arithmetic by
  the dimension of its value — so `after Twice(d) + 5 [s]` is silent and `after Len() * Len()`
  is reported as a value of dimension L². Triggers nested in action, state and transition
  bodies, including the body an action-target succession carries, are checked, and a body
  declared there now gets its own scope. An argument whose type only evaluation determines is
  left to it, and an unresolved name is reported by name resolution alone. A body `{ … }`
  written as a value is the expression itself, not its result, wherever a value is typed: it is
  reported as a trigger argument, bound to a typed feature (`attribute b : Boolean = { true }`),
  passed to a typed parameter or given to an operator, as the reference validator reports it,
  while its members are still checked. The check is gated
  per trigger, so an unresolved name elsewhere in the document does not hide an invalid trigger,
  and a workspace document that redeclares a library type's qualified name does not disable it.

- **A census of the pilot's named validation constraints.** `docs/project/validation-constraints.md`
  lists every `validate*` constraint the pinned pilot validators name (217, re-extracted from the
  pinned jar into `docs/project/validation-constraints-baseline.json`) with what it checks, where
  OpenSysML implements it, how our wording differs, its negative case, and an honest status — a
  constraint without a case or an identifiable pass is recorded as unknown rather than absent. Every
  faithful or approximate row is backed by a probe model `go test ./cmd/validation-census` runs.
  `go run ./cmd/validation-census -check` (in `make docs-counts` and CI) fails when the baseline no
  longer matches the jar, when the table and baseline disagree, or when a quoted figure is edited by
  hand.

### Performance

- **The compiled calc tier takes the bodies analysis models write.** Four constructs join the
  compiled subset, each reproducing the reference evaluator's values, error text and step counts
  exactly: statement bodies of body-local scalar declarations, `return` and `if`/`else`, compiled
  from the lowered statements into further slots of the scalar frame with the evaluator's
  declaration-order and shadowing rules (a local read before its declaration, a local without a
  value, a body that may run off its end, `out` features, loops and assignments stay on the
  evaluator); parameters redefined along the specialization chain (`in :>> x = 3.0;`,
  `in x :>> Base::x : Integer;`), laid out as the effective parameter list the evaluator binds; the
  standard library's scalar functions and constants (`sqrt`, `ln`, `exp`, `abs`, `floor`, `round`,
  `min`, `max`, the trigonometric functions, `deg`, `rad`, `TrigFunctions::pi`, …), dispatched
  through the resolved symbol to the Go implementation the evaluator uses — so an alias or an
  import reaches it and a model's own `sqrt` does not — and `sum`/`product` over a lone scalar;
  and named arguments (`Fib(k = n - 1)`), bound to slots at compile time with the evaluator's
  arity and unknown-name checks ahead of dispatch. Over the repository's fixtures, examples and
  the OMG corpora the tier now compiles 127 of 343 calc definitions (37%) instead of 42 of 263
  (16%); a recursive tree of three-local bodies calling `sqrt` runs 12.6× faster (10.4 ms instead
  of 132 for 131 071 invocations) and `Fib(25)` is unchanged at 21–23 ns per invocation. The
  differential test now also compares Reals bit for bit over ±0.0, ±Inf and NaN, and focused
  fixtures under `internal/core/runtime/testdata/compiled/` run each construct through both tiers.

- **`FeaturesOf` no longer scans the parent scope to find a feature's owning type.** `findOwnerType` reads the owner the scope records and falls back to the scan only for a body re-owned by another declaration. Computing the effective features of a type is 16× faster than 0.4.3; `-instantiate` and `%satisfy` over a large model 13–16× faster.
- **The semantics model memoizes a type's members, its contribution sources and its feature shape.** Action performances, constraint evaluation, instantiation and succession validation each re-derived `MembersOf` of a type per use; the model now caches the view once the closure it depends on has finished resolving. Starting an action performance is 3× faster and evaluating one constraint over many objects 2.8× faster than before the fix. Evaluation also skips the resolver lookup for sub-action names when no action performance is on the stack.
- **Starting a state machine from an exhibiting object is constant in model size again.** The REPL indexes exhibited states per session, rebuilt only when a declaration changes the model, instead of scanning every scope on each `%state` start; 4 000-element start 992 → 9.8 µs.
- **A gRPC request reuses the parsed model's resolver and semantics across requests.** The name resolver and semantic side tables are built once per content hash and shared under a lock; runtime instances stay request-local. `VerifyConstraint` 2.4 s → 12 ms, 29% faster than 0.4.3. `Evaluate` runs under `Resolver.Scratch`, which forgets what the request's own expression nodes memoized — the spellings suggested for their unresolved names included — so the shared resolver does not grow per request.
- **Instantiating an object allocates its feature values in one block and memoizes its redefinition groups and behaving parts per type.** Per-object instantiation is 2× faster than before the fix (35 allocations, was 54).
- **Control-node succession validation asks whether one member is visible instead of enumerating all of them.** `ActionSuccessions` uses `Model.HasMember`, which stops at the first match, and the contributor lists it walks are memoized. Validating a 12 000-element model is 26% faster and allocates 13% less than before the fixes.

- **Resolving a name through a wildcard import costs the matches, not the namespace.** Each
  unqualified name reaching `import ISQ::*` or another large library namespace used to be
  compared against every member the import surfaces, so a model leaning on the quantity and
  unit libraries spent much of its load time in that scan. The index now answers a wildcard
  import's members by name; the public Apollo 11 model (28 files, 7.2k lines) validates in
  about 0.33 s instead of 0.54 s, allocating 168 MiB instead of 218 MiB, with identical
  diagnostics.

### Fixed

- **`%state <machine>` drives the object that exhibits the machine.** Naming an exhibited
  machine alone (`%state lp` after `%instantiate TA::Sys`, or `-state lp` on the command line)
  used to start a detached performance of it, one with no performing object: `%advance` reported
  the timer events it dispatched, but the `do`, `entry` and `effect` writes of that run went to
  the detached run's own frame, so `%features #1` still showed the values `%instantiate` had
  left (`n = 1` for `n = 3`), while `%state #1` over the same object was right. The form now
  attaches to the running machine of the one held object exhibiting it — the same object
  `%instances` and `%features #1` show — so the two forms agree. When no held object exhibits
  the machine, or several do, `%state` refuses with a typed error (`ExhibitorsError`) naming
  the objects and both forms that address one (`%state <object>`, `%state <machine>
  <object>`), rather than guessing or performing the machine detached; with no object yet
  held, it names the types whose objects run the machine, whether they exhibit it inline,
  through usages typed by a shared definition (`exhibit state front : Blink`), or through a
  usage referencing another (`state spare : Blink; exhibit state active ::> spare;`): every
  binding on the way to the body addresses the machine, with the object named or alone. A
  definition one object exhibits as several usages refuses as it does with the object named.
  Held objects are the ones the session has built: a nested part counts once it has been
  reached, by `%features` or a machine that wrote it. A machine no type exhibits (`state def
  Blink` alone) still starts as before, since no object's performance of it exists to attach to.

- **A qualified name through an import evaluates as the checker resolves it.** The evaluator
  used to resolve only the first segment of `Bq::x` through the resolver and walk the rest as
  owned and inherited members, so a segment a `public import` re-exports failed with
  `member x not found in Bq` even though the checker accepted the reference — and the library's
  own façades are built that way, so `ISQ::speed` and `SI::speed` failed with
  `member speed not found`. The whole name now goes through the same `ResolveQualified` the
  checker uses: a wildcard, single-member or recursive import, a façade of a façade, a short
  name and an `alias` (which used to fail with `cannot evaluate element type *ast.Alias`) all
  reach the element the checker reaches, a `private import` stays reachable only from inside
  the importing namespace, and a name the checker rejects fails with the checker's own
  `unresolved reference: Priv::x` or, when several members answer to it, its own
  `ambiguous reference: Twice::t (2 candidates)`. The evaluator reads the name through a new
  `Resolver.ReadQualified`, whose answer (element, segments, ambiguity) is memoized by scope and
  node rather than by node alone, so one parsed expression evaluated in two scopes that each
  hold their own `A::x` answers with each scope's value. A calc usage's outputs, and the "not a
  variant" / "not a literal" reports, are unchanged.

- **A calculation's `return` may specialize without a name or a typing.**
  `return :> ISQ::power = force * speed;`, `return deltaV :> ISQ::speed = v1 - v0;` and
  `return :> ISQ::length[*] = xs;` used to be rejected with `expected '{' or ';' after return
  parameter` followed by a cascade of `expected a body member`; they now declare the result
  parameter with its subsetting, as the pilot implementation reads them. A `return :>` that
  names no target is reported once, without a cascade.
- **A calculation's result may be identified by a short name.** `return <r> result : Real = x;`,
  `return <r> :> ISQ::length = x;`, `return <r> = x;` and `return <r>;` used to be refused as a
  return expression; they now declare the result parameter under its short name, as the pilot
  implementation reads them.
- **A calculation's result may open with a multiplicity, or carry a body right after its name.**
  `return [*] = xs;`, `return [*] :> xs;`, `return r { doc /* … */ }` and `return <r> { … }` used to
  be refused as a return expression; they now declare the result parameter, as the pilot
  implementation reads them. A `return` followed only by a value or a body (`return = e;`,
  `return { … }`) declares nothing and is reported once, without a cascade. A kind-less member
  followed directly by a body (`twice { doc /* … */ }`) in a calculation or constraint body is
  likewise the declaration it is, not a trailing expression.

- **`%run-query` and `-run-query` accept a value of a type the session declares.** The REPL
  reads names from the document's own scope tree while the runtime model reads the index, so
  a parameter typed by a `part def` or `enum def` of the loaded model refused that model's
  own elements and literals (`binding site has type element, expected Site::Telescope`).
  Type conformance now compares symbols as elements, so one declaration reached through
  either tree conforms to itself and to its supertypes whichever tree those came through.

- **A calc specializing a library function keeps its own signature.** A bodiless `calc def
  Renamed :> sqrt { in y :>> x; }` is computed by `sqrt`, but the call was handed to the
  library under the library's parameter names and defaults, so `Renamed(y = 16.0)` was
  refused as naming no parameter and an overridden default was ignored. The written
  arguments now bind to the specialization's effective parameters — renamed, defaulted or
  optional, a default reading the parameters bound before it, each value checked against the
  type and multiplicity the specialization states — before the library function computes
  them, in an expression, a `send` and the `InvokeCalc`/`InvokeCalcNamed` API alike. A call
  in expression context whose name denotes a calc no longer falls back to a same-named
  action when only the action's inputs fit: the arguments are reported against the calc, as
  the runtime, which cannot evaluate an action, would otherwise fail at run time.

- **Every reader of a call selects the overload the checker selects.** Document queries and
  their `Column(...)` projections, and a `send` telling a calc call from a signal, resolved a
  name to the first declaration in view rather than to the overload its arguments fit — a
  query called with the other overload's arguments was refused as binding the wrong type, a
  same-named `Column` elsewhere hid the library's, and a signal and a calc sharing a name
  were told apart by import order. They now share the invocation selection, so a genuine tie
  is reported (`ambiguous-invocation` for a query) instead of picked silently. An expression
  whose name is also an action's selects the calc: an action has no result to evaluate, so
  `attribute v = tag(3);` no longer fails at run time when an `Integer` action beside a `Real`
  calc fits its argument more closely. `action call = tag(3);` and `perform tag(3);` keep
  selecting among actions only. A feature typed by a calc — the model's own or a library
  function such as `ref root : sqrt;` — is called as that calc by an expression and by a
  `send`, which delivers the computed value rather than a message named after the feature.

- **A kind-less feature may be identified by a short name.** `<a> alpha = 1;`, `<b> :> alpha = 2;`
  and `<t> twice = n * 2;` used to be rejected with `expected a namespace member` or `expected a
  body member` in every namespace, definition body, calculation body and nested statement body;
  they now declare the feature under its short name (SysML `DefaultReferenceUsage` over an
  `Identification`), as the pilot implementation reads them. A keyword of the other language is
  an ordinary name there too: `<chains> links = 3;` and `<s> featured = 1;` in a `.sysml` file,
  `<s> part = 1;` and `<attribute> y = 2;` in a `.kerml` file.

- A calc parameter default that re-invoked its own calc reported the recursion limit with one wrapped line per frame, building the message with the square of the depth (over 12 GiB under the race detector for a library specialization) and getting the test suite killed on memory-limited CI. A failing default now counts as a frame of its calc, so the frames collapse into a count as a calc body's already did.

- **The LSP refuses a rename that would collide, capture or shadow.** `textDocument/rename`
  checked only that the new name was spelled like an identifier, then rewrote the declaration
  and its references: renaming `x` to `y` where the owner already declares `y` produced two
  members of one name, and renaming to a name some enclosing, imported or intervening scope
  declares silently rebound every rewritten reference — with no diagnostic, since the name still
  resolves. The rename now runs the batch edit API's check, moved to a package both share
  (`internal/core/rename`): it is refused, with an error the editor shows naming the element the
  new name would mean, when the new long or short name already means something where the element
  is declared (a sibling's long or short name included), or when any reference it rewrites — in
  any open workspace document, as a whole name, a segment of a qualified one or a feature chain's
  member — would read another element afterwards. Each rewritten segment is checked by a trial
  reading of its reference with the new spelling, so a chain member is read in its operand's type
  rather than where the chain is written, a qualifier respelled onto another element is captured
  even where that element lacks the member the rest of the name asks for (the reference would
  otherwise be left unresolved), a segment that would name several members at once is refused as
  leaving the reference ambiguous, and a segment that would write an alias name is captured
  by the alias even when it aliases the renamed element (the rename would leave `alias New for
  New`); the batch edit API gains both checks too, where it previously let `d.x` be renamed onto
  a `y` that `d`'s type declares. A name taken only in an unrelated scope is not a conflict, nor
  is renaming a name to itself, and a shorthand
  redefinition whose declaration and reference share one span still renames. Aliases, short-name
  references and out-of-workspace declarations keep their rules, and
  `textDocument/prepareRename` is unchanged.

- **The LSP renames one of an element's names at a time.** Renaming the long name of
  `part def <O> Old;` rewrote every reference that resolved to it, `part a : O;` included, although
  a reference spelled with the short name still resolves after the rename; the batch edit API
  already left such references alone. `textDocument/rename` now rewrites only the references
  spelled with the name under the cursor, as one whole name or as a segment of a qualified one
  (`P::Old::x` changes, `P::O::x` does not), and renames a short name in its own right: with the
  cursor on `<O>` or on a reference written `O`, `textDocument/prepareRename` offers the short
  name's span and the rename rewrites it and every reference written `O`, leaving the `Old` ones.
  Both hold for definitions, usages, packages and aliases, across the workspace's documents, and
  compose with aliases: renaming `Old` leaves the `O` in `alias X for P::O;` alone.

- **A nested library shape's edges, vertices and per-face dimensions evaluate.** A `Rectangle`
  answers `rect.edges` with four `Line`s, `rect.e1.length`/`rect.e3.length` with its `length`
  and `rect.e2.length`/`rect.e4.length` with its `width`, `rect.vertices` with eight objects
  and `rect.v12`…`rect.v41` with two each; a `Box` answers `box.edges` with the twenty-four
  edges of its six faces and `box.tf.length`/`box.tf.width` with the cuboid's `length` and
  `width`; a `Triangle` values `base`, `e2` and `xoffset`, and `RightTriangle::hypotenuse`
  reports the library's squared length as the dimension mismatch it is. Four general rules do
  this, and each holds for a model of your own: an object listed in a typed collection
  (`item :>> edges : Line = (e1, e2, e3, e4)`) is classified by the collection's type rather
  than refused, keeping its identity and taking on the classifier's bindings, subsettings,
  connections and behaviors — and, where the classifier redefines a feature the object already
  carries, reading that redefinition's default, type and multiplicity — while a value that
  cannot be so classified — an `Integer` into `edges : Line` — stays `type mismatch`, and a
  collection one object of which is refused is refused whole, every object as it was and the
  objects the refused classifications made abandoned; an object a typed feature's value chooses
  by a condition, an index, an invocation or a body is classified as a listed one is, whichever
  feature is read first, and so is the object a selected variant stands for when a typed
  feature holds the variation's value; a classifier renaming a behavior the object already runs
  starts no second one, and the one execution answers to both names; a
  qualified name of an enclosing type's feature read inside a nested usage (`attribute :>> length = Rectangle::length;`) is
  that feature of the enclosing object, and a sibling chain (`e3.length = e1.length`) the
  sibling's; a feature chain valued over a collection (`edges = faces.edges`) collects across
  it, and a required lower bound the named subsetting features fall short of is filled from an
  optional subsetting feature before an anonymous object is made up; and a usage declared with
  no kind keyword (`doubled = span * 2.0;`) is a reference usage that materializes and evaluates
  like any attribute. A binding connector's end multiplicities now reach the runtime, so two
  `[0..*]` bindings of one collection agree or are a `binding conflict`, while a `[0..1]`
  binding that links one unspecified value of each end (`bind [0..1] tf.edges = [0..1] tfe`)
  is reported as the binding end it cannot resolve rather than answered with a witness — so
  `box.vertices` and `box.tfe`…`box.urre` are typed errors naming the binding — and never
  decides a feature also bound whole, whatever order the bindings are declared in. A binding end
  whose path crosses a collection (`bind [0..*] groups.items = [0..*] allItems`) reaches every
  object the collection holds, in order, and binds their values together — the other end is
  their union, counted as one sequence against the end's multiplicity — while each object keeps
  the part it holds on its own and one holding nothing of its own is a typed error naming the
  collection, not a partition the runtime picked; and `bind [m] a = [m] b` now states `m` as the
  first end's multiplicity, as `binding [1] bind [m] a = [m] b` always did, and the RDF mapping
  carries each end's multiplicity as bounds on its end node, so `bind [0..1] a = [0..1] b` and
  `connect [1] a to [0..1] b` come back from the graph without their source text. Only an
  argument a calc's returns pass on is held by the feature its call values, so reading any
  other argument computes neither that feature nor the object the call does return. An optional
  feature holding nothing is the empty sequence on every surface: `%features box` prints
  `shape = []` as `%eval box.shape` does, while a required feature holding nothing is still
  uninitialized and a valueless `Real` still `<unset>`. A read or write naming no feature of
  the object is the typed `object has no such feature` error, which is what
  `box.matingOccurrences` and `box.spaceBoundary` — Kernel frame features, not features of the
  shape — now report; the vertex-mating `assert constraint` bodies of `Path`/`Polygon` and the
  curved `Cylinder`/`Cone` edge graph remain documented limitations with the same typed errors.

- A performed action whose binding declares an `out` parameter with a value
  (`perform action tick { out total : Integer = 7; first start; then done; }`) starts and
  answers that value, rather than failing the performer with `output action parameter given
  as input`: the value of an `out` member is the answer's default, not an argument, so only
  `in`/`inout` members are bound as inputs.

- **`make proto-ts` runs `protoc-gen-es` from `clients/node/node_modules` instead of fetching it through `npx`.** The plugin is now a lockfile-pinned devDependency of the npm client, installed by `npm ci` when absent, so regenerating the TypeScript stubs needs no network and no longer dies with `signal: killed` when npm's registry audit outlasts buf's two-minute plugin timeout, which was failing every Node client CI run at its stub-freshness check.

- **An interface, connection, flow, allocation, binding or succession usage keeps the members
  of its body in RDF.** `sysml -convert ttl` used to fold the whole declaration of a usage whose
  head binds ends (`interface seam connect w.outp to r.inp { attribute coupling : C = C::x; }`)
  into its `sysx:sourceText`, so `coupling` had no node of its own — no `sysml:ownedMember`, no
  `sysml:ownedFeature`, nothing to query — and a graph without the text came back without the
  body. The same held for a `first a then b { … }` or `then b { … }` in an action body. The text
  now carries the head alone, and the body's members convert like those of any part or interface
  definition body: each an element with its name, type, value, `sysx:memberIndex` and membership,
  written back from the structure whether or not the graph carries the text
  (Open-MBEE/OpenSysML#89).

- **A reference reached through an import or an alias links to its element in RDF.**
  `sysml -convert ttl` wrote `sysml:type "BudgetLedger"` for `item b2 : BudgetLedger;` under a
  `public import OtherPkg::*;`, and `sysml:referent "Tempo::operative"` for the value of
  `attribute t : Tempo = Tempo::operative;` — string literals, though the fully qualified
  `OtherPkg::BudgetLedger` a line above linked to `elmt:OtherPkg__BudgetLedger`. The encoder
  looked a name up only along the owner's own namespaces; it now asks name resolution, so a type,
  subsetting, redefinition, relationship end or feature reference spelled through an import, an
  alias or a nested package path links to the same element its fully qualified spelling does.
  A feature chain's member (`w.size`) and a behavior's endpoints (`then idle`) link the same
  way, a chain's member found in its operand's type. A name that genuinely does not resolve is
  still carried as a literal; read back, a linked redefinition target is written by the
  redefined feature's own name where one feature of that name is inherited, qualified where
  several are (Open-MBEE/OpenSysML#90).

- **A literal of the wrong datatype is no longer read as a name.** The Turtle reader took every
  literal by its lexical form, so `sysml:declaredName "3"^^xsd:integer` or a `sysx:bodyParameter`
  stated as `"x"@en` came back as the name `3` or `x`. Every metamodel property the mapping
  reads as text is a `String`; a language-tagged literal, or one whose datatype its property does
  not take, is now refused before anything is read, naming the literal and the subject that
  states it. Plain and `xsd:string` literals read as before, and each property takes the
  datatypes the ontology gives it: `xsd:boolean` for a flag, `xsd:integer` or `xsd:int` for an
  index or a `LiteralInteger`, `xsd:decimal`, `owl:real`, `xsd:double` or `xsd:float` for a
  `LiteralRational`. The text is checked against the datatype's lexical space too, so
  `"false"^^xsd:int` or `"yes"^^xsd:boolean` is refused rather than read as the text it spells.
  A `sysx:` index that is negative or too large for `int` is refused too, where it was read as 0
  and moved its member to the front, and so is a subject stating a single-valued `sysx:`
  property twice (a body with two `sysx:resultExpression`s), where the first was kept and the
  rest dropped. A subject stating several `rdf:type`s is read as the one that is a subclass of all
  the others, whichever is written first, where the first one was read; a set of classes with no
  such member is refused naming the subject. A property a known metaclass does not declare, such
  as `sysml:value` on an `AttributeUsage`, takes the datatypes this mapping writes there rather
  than the union of every metaclass declaring the name, so `"2"^^xsd:integer` is refused as a
  feature's value where it was read as expression text.
- **A result expression rebuilt from its graph is spelled as the grammar requires.** A
  `sysml:LiteralString` whose value holds a quote, a backslash or a line break is written back
  as the escaped string token; a `LiteralRational` value is written as a real token (`"3"^^xsd:decimal`
  comes back as `3.0`) and a `LiteralBoolean` as `true` or `false`; a value no token spells — a
  signed number, `INF`, `NaN` — is refused naming the node instead of becoming a name. A real
  literal with an exponent is now written as `xsd:double`, whose lexical space holds it, where
  `xsd:decimal`'s does not. An empty expression body `{}` states `sysx:hasBody` so it comes
  back without its `sysx:sourceText`, and a named invocation argument whose name is not a basic
  name (`f('the value' = x)`) keeps its quotes.
- A result expression a graph owns without a `sysx:memberIndex` — as a graph written by another
  tool does — is now written last in its body, where the grammar has it, rather than wherever the
  graph happened to list it, which could put it ahead of the parameters it reads.

- **A literal reference must be a name.** A graph stating a reference the graph does not define —
  a metadata usage's `sysml:type`, a specialization, an `about` target — as a literal that is no
  name (a number, a boolean, a language-tagged or expression-typed string, an empty or broken
  qualified name) was written into the notation as it stood, as `@42`. `sysml -convert` now refuses
  it with an error naming the element and the literal; a plain string spelling a qualified name is
  written as before.

- **`sysml:owningNamespace` names a namespace only.** An element a relationship owns — a
  state's entry action, a `#M` prefix on a dependency or a subject — stated the relationship as
  its `sysml:owningNamespace`, outside the property's range. It now states `sysml:owner` and the
  membership wiring alone, as the metamodel does; `sysml:owningNamespace` is written for an
  element a namespace owns, as before.

- **A reference written back from RDF resolves to the element the graph names.** The notation
  writer spelled every reference as the name the source had used, read against the writing scope
  by a walk of its own; where a nearer declaration bore the same name — a redefining attribute
  named after its target, a subsetting part named after the part it subsets, a nested `part def`
  shadowing an outer one — the written name re-resolved to that declaration and the second
  conversion recorded a different graph (`redefines <itself>`, for the pilot `Packets.sysml`). The
  encoder now links each reference through the resolver's own reading of that occurrence
  (redefinition and subsetting targets looked up past the declaring feature, transition ends as
  vertices of the machine, feature-chain members in the operand's scope, `first x` labels and loop
  variables not links at all), and the writer spells each link as the short name only when the
  resolver reads it there as the linked element, else the shortest qualified suffix that it does,
  else the global form, and refuses a reference no spelling reaches. An unnamed usage that
  redefines or references a feature takes that feature's name, so a reference to it is a graph
  link written back by that name rather than a literal. An unnamed transition's effect now has a
  scope of its own, as a named one's does, so a succession between its members links both ends and
  the trigger's parameters reach however deeply the effect nests. A `then` after `first x` now
  sequences from the member `x` names, as the initial node itself does, so the pilot use-case model
  that redefines `start` comes back from its graph. A named multiplicity now owns the members its
  body declares, so a reference made there resolves from the body and is a link rather than a
  literal. The corpus gate writes each file back from the
  source text it carries, so its verdicts do not move; written from the graph alone, `Packets.sysml`
  now comes back as the same graph rather than a different one, and no file regresses.
  `TestRoundTripIsLossless` now also writes every
  fixture back from its graph with the source text removed and requires the same graph again, and
  a fixture reproduces each shadowing
  ([docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md#limitations)).

- **A redefining requirement subject or actor is bound under every name of the feature it
  redefines.** `requirement r : Req { subject renamed :>> truck = loaded; }` used to leave a
  condition inherited from `Req` that reads `truck` (or its short name) unbound, reporting
  `truck subject is unbound` although the redefinition supplies the value. The binding now
  reaches the subject under its own name and short name, the names of every feature it
  redefines — explicitly, by short name, implicitly by role, or through a redefinition of a
  redefinition — and a redefinition that values nothing reads what the redefined feature binds.
  A subject `by` a `satisfy` assertion overrides the declaration under all of those names, and
  the same declaration written without a name (`subject <s> :>> x;`) is one member under `s`
  and `x`, as `part <p> :>> x;` is, so both names resolve, are shown and rename together.

- **`%state <machine> <object>` attaches to an exhibited machine that states its own body
  under the definition's type.** For `exhibit state m : Mission { state extra; }`, `%state Mission
  tank` used to report that the object "exhibits no running machine of this kind" and start a
  second, detached performance of `Mission`, and `%state Mission` alone found no exhibitor. The
  machine an object exhibits is now addressable by every definition its bindings conform to —
  the one typing the usage and the ones that definition specializes — whether the body lives in
  the usage or in the definition, so both forms attach to the running machine as the reference
  already described. The `-state` command-line flag shares the path.

- **A positional `then` sequences past every member that is not a feature.** The parser read
  `action a; doc /* … */ then action b;` as a succession from the documentation, and the same for
  a `comment`, a `rep`, an `import`, an `alias`, a nested definition or `package`, a `multiplicity`
  declaration, a `defer` written before the `then`: the runtime then refused to lower an action body (`succession edge references
  undefined source node`) and a state machine stopped at the state before the `doc`. The rule the
  parser and the RDF writer share is now `ast.IsSuccessionSource` — a feature that is not an edge
  (`ast.UsageKind.IsEdge`) — so a `then` sequences from the nearest feature before it, as the pilot
  implementation resolves it (`UsageUtil.getPreviousFeature`) and as SysML v2 §7.17.4 reads. A
  `then` with only non-feature members before it is diagnosed as having nothing to sequence from,
  whether attached to a member (`then action b;`) or naming its target (`then b;`, `if g then b;`,
  `else b;`), which before reached lowering as an edge with no source.
  The writer folds a succession back into `then` past the same members and refuses a graph whose
  source is one of them; `docs/reference/rdf-mapping.md` records the rule and its two known gaps
  (an end-less `flow`/`message`, and an `alias` of a feature, which the pilot keeps as the source).

- **A positional `then` sequences past a `connect`, `connection`, `interface`, `allocate` or
  `allocation`.** The parser read `action a; connect p to q; then action b;` as a succession
  from the connection, so the runtime refused to lower it (`succession edge references undefined
  source node written as anonymous connection usage`) and the RDF writer folded `then` back only
  past a `flow`, `bind`, `succession` or `transition`. The rule the parser and the writer share,
  `ast.UsageKind.IsEdge`, now covers every connector kind: a `then` sequences from the nearest
  member before it that is not a connector or a transition, as the pilot implementation resolves
  it (`UsageUtil.getPreviousFeature`). `docs/reference/rdf-mapping.md` records the rule, its basis
  and the one known gap — the pilot keeps a `flow` or `message` written with no ends as the
  source, this implementation reads past it.

- **An unrelated error in a package no longer hides its features' variability diagnostics.**
  `Initialized feature must be variable` and `Only a variable feature can be constant` on a feature
  declared directly in a package or namespace used to go unreported whenever any sibling member
  later in that package failed a lower tier (an unresolved typing, say). A package has no typing of
  its own to fail, so it now gates nothing: only the feature's own head, and the head of a
  definition or usage that owns it, silence the rule.

### Changed

- **A conversion from RDF returns the notation as written.** Every element written to `.ttl`
  carries its lines as `sysx:sourceText` — comments, blank lines and keyword synonyms included —
  and an element with members carries the lines closing its body as `sysx:sourceTail`; the text
  is the file's own bytes — tabs, irregular indentation, blank lines inside a head, CRLF line
  endings and the notes after the last root included, never a formatted copy — and the two
  properties are one-line literals with newlines escaped. `sysml model.ttl -convert sysml` now
  writes that text back untouched, so a `.sysml → .ttl → .sysml` round trip is byte for byte for
  any file, where before it came back canonical with its `//` and `/* */` comments dropped. A head
  laid out over several lines or with a comment inside it is recorded in the mapping
  (`sysx:endForm`, `sysx:declaredKeyword`) like one written on a line, since the graph states
  tokens, not layout. The graph stays
  authoritative: the candidate notation is converted back to RDF and compared with the graph, and
  each element whose text no longer states its triples — a flag set, a value changed, a member
  removed or an identity annotation dropped after the export — is written canonically instead,
  with `@IdentityMetadata::ElementId` and `ProjectRef` materialized exactly as for a graph without
  text; text that no longer parses demotes the whole file. A member written on its owner's lines,
  such as an accept's payload, carries no text of its own, so an edit to it rebuilds the owner
  whole rather than splicing a line into it. Each root records the grammar its file was written in
  as `sysx:sourceLanguage`, so KerML text is checked as KerML rather than as the SysML it may also
  read as; a buffer with no extension records none and is checked as such a buffer again. With the
  notation written from its text, the corpus round-trip ratchet moves 101 files to `stable` — every
  `whitespace-only`, `graph-diff` and `unparseable` verdict and all but one `unwritable` — which says
  the text survives, not that the structural predicates alone would (that remains the stripping
  tests' job). A `LiteralString` node's `sysml:value` is now the value
  the notation's escapes read to rather than the text between the quotes, and a value edited in the
  graph is written back as a literal that reads to it, whatever characters it holds. A graph without `sysx:sourceText` — from
  another tool, or stripped — converts as before, and the round-trip tests keep stripping it to
  prove the structural predicates carry the model; each fixture under
  `internal/core/export/testdata/convert` now locks both notations. The
  [saving guide](docs/guide/07-saving-and-rdf.md), the
  [RDF mapping](docs/reference/rdf-mapping.md#source-text) and the round-trip testing skill
  describe the precedence.
- **A calc whose `return` declares a result parameter it never binds says so.** In the notation
  `return` introduces a result *parameter* (SysML.xtext ReturnParameterMember), so `return h;`
  after `attribute h : Real = …;` declares a second member named `h` — the pilot flags the
  duplicate name — and returns nothing; the evaluator's "no result expression" error used to
  stop there. It now names the unbound result parameter and shows the two forms that state a
  computed result: the body's trailing expression `h`, or `return : Real = h;` (the type and
  expression are taken from a same-named member that binds a value, and names are spelled as
  the notation writes them, so `'my result'` keeps its quotes). The grammar and the error's
  type are unchanged.
- **A library function is evaluated by its bare name only where the model imports its
  package, as the checker resolves it.** `wheels->size()` in a model that imports no
  `SequenceFunctions` was reported `unresolved reference: size` by the checker yet evaluated
  to `4` at the prompt, because the runtime answered a bare call from every implementation it
  knew by local name. The runtime now resolves a call where it is written, exactly as the
  checker does, so an expression the checker reports unresolved fails to evaluate with the same
  error and hint — `unresolved reference: size — did you mean SequenceFunctions::size or
  CollectionFunctions::size?` — and importing one of the named packages
  (`private import SequenceFunctions::*;`) makes it both resolve and evaluate. Expressions that
  evaluated before without an import fail until that import is added; the qualified name
  (`SequenceFunctions::size(wheels)`) resolves anywhere, as it always did, and a model's own
  `calc def size` is what a call denotes when the library is imported too. The same rule
  already governed the `OpenSysMLMathFunctions` extension, whose bare `exp(x)` now fails with the
  same unresolved-reference error rather than a separate one. `%builtins` lists each function
  with the package an import must name for its bare name to resolve, and an empty session's
  `%eval` answers a qualified library call rather than refusing every non-literal expression.
  The examples and fixtures that relied on the old fallback now import the packages they call.
- **An OSLC query reports a selected property under the name it was asked for.** `sysml
  -query 'oslc.where=rdf:type="PartUsage"&oslc.select=rdf:type'` and the REPL's `%query`
  used to report the property as `@type=PartUsage`, a Go API name that the query text
  refuses, so a reported row could not be written back into a query; the rows now read
  `rdf:type=PartUsage`, `sysml:name=battery`, `sysml:owner=Robot::Platform`, and a prefix
  rebound by `oslc.prefix` renames the property in the answer as well. The gRPC response
  still keys properties by the query property names the structured `query` field uses.
  A reported value is a bare name, which the grammar wants quoted, so refusing one now
  names the form to write: `sysml:name=battery` answers with `write a name as a quoted
  literal "battery"` instead of only `invalid OSLC value "battery"`. Since a property is
  reported under one name, `oslc.select` naming the same property twice — as
  `sysml:name,sysml:name`, or as two prefixes bound to the SysML namespace — is now refused
  rather than reported twice under whichever name came last.

- **An optional composite feature fills to its lower bound.** `part spare : Wheel[0..1]` used
  to materialize an object, where `part wheels : Wheel[0..*]` materialized none; both now hold
  only the objects the features subsetting them hold, so an optional part reads as the empty
  sequence and an abstract one holds only what subsets it — a required abstract feature nothing
  subsets is a multiplicity violation, not an empty value — and an abstract feature that states
  no multiplicity is bound by what it subsets, so a part's inherited `Action::decisions`,
  `forks` and `joins` (`:> controls[0..*]`) hold nothing rather than demanding one control each.
  A required feature holding nothing is still uninitialized when read. The same governs a connector: `connection c : Link[0..1]
  connect a to b` links nothing of its own until a connector subsetting it does, while a
  required connector still links its ends. What made this visible is the library: `Item::shape` and
  `Item::voids` are optional, and an anonymous object for each would have said the box had a
  void it does not have.

- **The documentation site's landing page describes the four oracles instead of quoting their
  totals.** The band below the hero used to state the differential's agreeing-file count, the Xpect
  suites' declared-diagnostic and scope tallies, the rejection corpus's size and the pilot pin, all
  regenerated by `make docs-counts`. Those figures are a census of the corpora we happen to run,
  not a measure a first-time reader can weigh, so the band now says what each comparison measures
  and links to the record that reports it; the numbers stay in `README.md`,
  `docs/internals/architecture.md` and the conformance records, where `doc-counts` still generates
  and gates them. `overrides/home.html` is no longer a `doc-counts` consumer.
- **`%features` lists an object's behaviors under their own heading instead of as `<unknown>`
  values.** A state or action a type declares holds no value, and the listing used to render each
  as a feature row reading `<unknown>` — `off = <unknown>`, nested state by nested state — while the
  running state was only visible through `%current`. The values are now followed by a `Behaviors:`
  section that says what the object is doing with each: the current active state of a machine it
  exhibits (`modes: exhibited state machine, current state off`, the very state `%current`
  reports, before and after the debugger drives it), the execution state of an action it performs
  (`tick: performed action, completed`), and `not running` for a state or action the type declares
  but the object neither exhibits nor performs. A named transition is listed as the step it
  declares (`toggle: transition, modes.closed → modes.opened`), not as an idle action. The values
  a running behavior owns — the attributes of the machine's own occurrence, an action's parameters
  and outputs — are listed under its row, apart from the performer's own values of the same name.
  A nested object's behaviors are listed under its own row. Nothing is invented: a machine that
  has not started reads `not started`, one that reached its end reads `completed`.

- **A kind-less `x = e;` or `x := e;` in a behavioral body declares a feature; an assignment
  is spelled `assign x := e;`.** In a calculation body, a constraint body, a `while`/`loop`/`for`/`if`
  body, a state's entry/`do`/exit block or a transition effect, `x = e;` used to be OpenSysML's
  own shorthand for assigning `x`. It now reads as the standard notation does — a member of the
  body declared only by its name and value (SysML.xtext `DefaultReferenceUsage`), the reading the
  pilot implementation gives it — so `calc def c { in n : Integer; twice = n * 2; twice + 1 }`
  declares a local `twice` that the trailing expression reads, and
  `assert constraint { flag = true; }` declares `flag` rather than writing it. A model that
  relied on the shorthand must write `assign x := e;`: an `x = e;` in such a body no longer
  updates an output, a local or a state attribute, and a calc whose outputs were written that
  way reports them as never assigned. The bundled fixtures have been migrated.

- **Compliance census counted at docs build.** `docs/project/spec-compliance.md` no longer carries a literal rule census, and `README.md`/`docs/internals/architecture.md` no longer restate the rule total; `scripts/mkdocs_census.py` counts the rows when the documentation site is built. Adding a compliance row no longer rewrites any shared line, and `make docs-counts` regenerates only the oracle-baseline figures.

- **Changelog entries are written as fragments under `changes/unreleased/`.** Every pull request
  used to append to the `## Unreleased` section of `CHANGELOG.md`, so any two open branches
  conflicted there. A change now adds one file, `changes/unreleased/<slug>.<section>.md`, and
  `python3 scripts/changelog.py release X.Y.Z` folds the fragments into a dated entry when a
  release is cut. `make docs-check` and CI validate the fragments.

- **The rejection oracle now names the pilot constraint each case exercises.** A fourth
  corpus source, `cmd/pilot-reject/testdata/negative/semantic/`, adds 43 minimal invalid
  models, one per KerML or shared validation constraint of the pinned pilot that had no case
  before, each header citing the constraint by its `validate*` name. The oracle stands at 163
  cases, 150 rejected by both implementations and 13 the pilot rejects and OpenSysML accepts;
  `docs/project/pilot-rejection.md` adjudicates every gap with the pilot's message and the pass
  that is silent, and lists the constraints the pilot declares but does not enforce and those
  for which no legal violating model exists. No validation rule changes in this entry.

- **Formatting in the editor changes only the lines that need it, and can format a selection.**
  `textDocument/formatting` used to answer with one edit replacing the whole document, so a
  reformat collapsed the undo history into a single step and moved the cursor, selection and
  folds. The server now answers with one small edit per changed region — an indentation fix on
  the one line that needs it, a deletion of the one surplus blank line — and nothing for a
  document that is already formatted, so the editor keeps its cursor, selection, folds and
  undo history across a format. `textDocument/rangeFormatting` is now implemented: a selection
  (widened to whole lines) gets only the edits on those lines, indented from the whole file's
  structure so the result matches its surroundings.

- **Find References and Rename answer in milliseconds on large workspaces.** Each
  `textDocument/references` and `textDocument/rename` request used to re-read every reference
  in every open document and resolve each one afresh, so on a workspace of a hundred files a
  request could take a second or two. The workspace now keeps a reverse reference index —
  every written name, filed under the declaration it denotes and, for an alias, under the alias
  too — rebuilt once on the first such request after an edit and then answered by lookup. The
  results are unchanged: references still list every segment of a qualified name that denotes
  the symbol, alias uses still count for both alias and target, a call tied between overloads
  still names nothing, and rename still edits only the name as written.

- **Release performance comparison against 0.4.3.** `docs/project/performance-release-0.5-vs-0.4.3.md` measures `main` against the `v0.4.3` tag — every Go benchmark under `benchstat`, whole-binary `sysml -validate` scaling, process start and the `examples/` models — and records each regression with its cause, the change responsible, its size and whether it is fixed here or is the quantified price of a rule landed since 0.4.3. After the fixes below, loading is 7–11% slower than 0.4.3 (the validation passes added in the interval, none algorithmic), instantiating one object is +85% (the library features an object now carries), and every other row is on par with or ahead of 0.4.3.

- **`send Def(args)` on an item or attribute definition is an error, as the specification and
  the pinned pilot say.** The runtime used to read that invocation as "send an instance of
  `Def`", the shape the conformance fixtures and the relay-probe demo were written in; KerML's
  `validateInvocationExpressionInstantiatedType` allows an invocation only of a behavior or a
  behavioral feature. Write the constructor instead: `send new Def(args)`. The fixtures, the
  demo and the examples are migrated; invoking a behavioral feature (`send shutDown() to self`
  over an action) is unchanged.

- **The site shows one menu button on a phone.** Below the theme's drawer breakpoint the
  header's own menu button is hidden and its links — Guide, Reference, Roadmap, OpenMBEE and
  the community wiki — appear as a row above the footer, leaving the drawer's button as the
  only one in the header.

- **The solver's design provenance is now credited.** The README's new Acknowledgements section, the solver sections of the guide, the REPL and environment references, the compliance record and the `internal/core/solve` package documentation name the `ConstraintSolverService` of OpenMBEE's [HMF (Hivecore Model Framework)](https://github.com/hivecore-dev/hmf) (Apache 2.0) as the design the constraint-solving capability set follows; the implementation itself remains independent.

- **The SonarCloud findings outside cognitive complexity are cleared.** Duplicated literals are named constants, marker methods state their contract, over-long parameter lists take a struct, identical library conversions share one body, single-method interfaces follow the `-er` convention, and the code generator resolves the `go` command to an absolute path before running it. No behavior changes.

- **Why an unrelated type on a subsetting feature is not a diagnostic is now recorded.** The
  compliance record explains that `feature f : B subsets g;` under `feature g : A;` is
  well-formed because subsetting adds `A` to `f`'s types rather than requiring `B` to conform
  to it (KerML §8.3.3.3.4, §7.3.4.4) — the shape the OMG training corpus's
  `Model Library Example` uses and the reference validator accepts — while a redefinition,
  which replaces the redefined feature, is still held to type conformance.

- **The rejection oracle covers the pilot's SysML validation constraints case by case.** The
  `cmd/pilot-reject/testdata/negative/semantic/` source gains 45 minimal invalid models, one per
  SysML validation constraint of the pinned pilot that had no case before, each header citing the
  constraint by its `validate*` name; seven `grammar/` cases and one `extensions/` case cover the
  requirement, case, state and view body items the pilot rejects as syntax errors outside their
  owning body, and the fourteen existing `xpect/` cases that already covered a constraint now
  name it. The oracle stands at 216 cases, 194 rejected by both implementations and 22 the pilot
  rejects and OpenSysML accepts; `docs/project/pilot-rejection.md` adjudicates every gap, lists
  the constraints the pilot declares but does not enforce and those for which no legal violating
  model exists, and carries a name-by-name census of the 100 SysML constraints. No validation
  rule changes in this entry.

### Fixed

- **`%eval in <part> : <feature>` reads a valueless feature as `<unset>` rather than calling it
  unresolved.** Before an object exists, `%eval in car : wheels` for a multi-valued
  `part wheels : Wheel[4]` — and `wheels.radius`, an attribute with no default, or a multi-valued
  `String[3]` attribute — reported `unresolved reference`, though the name resolved perfectly well
  and its single-valued neighbours evaluated. A feature the declarations give no value to now reads
  `= <unset>`, as it does on an object; `unresolved reference` is reserved for a name nothing
  declares. Only a bare read of the feature is unset: an expression over one (`unsetMass + 1`) or a
  feature whose value depends on one still fails, naming the feature that has no value. The same
  distinction holds throughout the evaluator: a declared name with no value is a typed no-value
  error carrying the name, never an unresolved reference, so a qualified `car::wheels` reports that
  it has no value to evaluate.

### Fixed

- **An expression evaluated after `-instantiate` reads the object that was created and run.**
  `sysml model.sysml -instantiate P::ctx -e "ctx.recv.got"`, the bare line `ctx.recv.got` after
  `%instantiate P::ctx`, and even `%eval in P::ctx : recv.got` — which printed
  `(on P::ctx ID: 1)` — answered `0` while `%features #1` showed `got = 1` for that same object:
  a name in the expression materialized a fresh object of the usage instead of the one
  `%instantiate` created, and a nested part whose machine sends or accepts a signal ran only
  once something read it, so what a read saw depended on the order the parts were first
  inspected in. An instantiated usage now denotes the object created under it, and creating an
  object materializes and runs the nested parts whose types exhibit or perform behaviors with
  it, so the whole runs to quiescence once and every later read — a CLI `-e`, a piped
  expression line, `%eval`, `%eval in` and `%features` — reports the same values.
  `%eval in` also takes an object the way `%features` and `%state` do: by id (`%eval in #1 :
  recv.got`) or by a path under a named object (`%eval in ctx.recv : got`), and its usage line
  lists the forms. (Open-MBEE/OpenSysML#91)

- **Messages cross a binding connector at an assembly's boundary port, in both directions**
  (Open-MBEE/OpenSysML#92). An assembly that binds its boundary port to a port of a part it
  holds (`part def Assembly { port bi : ~PP; part child : Inner; bind bi = child.i; }`) used
  to swallow messages at the boundary: a `send Ping() via o` over a context-level
  `connect env.o to asm.bi` arrived at `asm.bi` and stayed there, so the inner part's
  `accept Ping via i` never fired and its counters stayed at 0, with no diagnostic; and a send
  by the inner part through its own port was reported as reaching no receiving port, although
  the boundary port it is bound to was connected. A binding connector now makes the two ports
  one port for message delivery: an accept on either takes a message that reached the other,
  and a send through either leaves over the connectors joined to the other, through any depth
  of nested assemblies. Bindings chained through several assemblies also keep every bound port
  the same object whichever end is read first — a chain used to split when the outer boundary
  port was materialized before the inner assembly's. A send whose bound boundary port is
  joined to nothing still reports `send reaches no receiving port` where it was written.
  Delivery does not depend on the order connectors, bindings and parts are declared.

- **`satisfy … by config.child` is evaluated on the nested object it names.** A satisfaction
  assertion whose `by` operand is a feature chain used to be read as its last name alone:
  `%satisfy` and `-satisfy` reported `? satisfy r2 by child could not be evaluated — no subject
  to satisfy the requirement: child`, and only the reporter's workaround of binding the
  requirement's `subject` to the chain reached a verdict. The chain is now resolved through
  each feature in turn — the object of `config` is materialized and the one its `child` holds
  is the subject — so the assertion holds or fails on that nested object, at any depth and
  through parts typed by definitions with nested parts of their own. The verdict and every
  diagnostic spell the chain as written (`satisfy r2 by config.child`), and a chain whose
  segment resolves to nothing says so under its full name (`no subject to satisfy the
  requirement: config.nope`). A repeated `%satisfy` is about the same nested object, which
  `%features S::config::child` can then inspect. (Open-MBEE/OpenSysML#94)

- **`%state <machine> <object>` attaches to the machine the object exhibits instead of performing
  it again.** Naming an object's own exhibited machine (`%state Rover::modes rover`, or `-state
  "Rover::modes rover"`) used to start a second performance of it on the same object, so its
  `entry` and `do` actions ran twice against the same feature values — a `log` written once as
  `"W"` read `"WW"`, a `level` raised by 10 read 20. The two-argument form now recognizes that
  machine by its declaration and attaches to the running performance, saying so in a `note:` line
  that names the one-argument form; a machine the object merely performs is still started as a
  detached performance. The attached session follows the object over an unrelated declaration,
  as the one-argument form's does, and stays on the machine it was attached to when the object
  exhibits several. A definition the object exhibits as the body of several usages (`exhibit
  state front : Blink; exhibit state rear : Blink;`) names no one running machine, so `%state
  Blink lamp` refuses and names the usages that would: `object #1 of "lamp" exhibits "Blink" as
  2 machines, so naming the definition attaches to none of them: name the exhibited usage
  instead — Lamp::front or Lamp::rear`.
- **`%state`, `%invoke` and `-state` reach a nested part by path and by id.** The object argument
  accepted only the name of a top-level object, so the machine of a part reached through
  composition could be watched with `%features` but neither debugged nor invoked on. The
  argument now takes the same reference every other command reads — a feature path from a
  top-level object (`driver.r`, `driver.r.motor`, `Fleet::driver::r`), the id the prompt prints
  (`#3`), or an element of a multi-valued part by index (`garage.bays[2]`) — and the CLI's
  `-state "<machine> <object>"` reads it the same way. A segment whose feature value the runtime
  could not materialize keeps the runtime's reason (`spare of Shared::lamp could not be
  materialized: … multiplicity violation …`) rather than being reported as a missing feature, and
  reaches the session status as a failed `%features` would. A qualified path is read as typed —
  `Fleet::driver::r` is the usage's part, reported as `Fleet::driver.r`, even with `Fleet::Driver`,
  where `r` is declared, instantiated too — and an object addressed by id is reported by that id
  alone, so a session attached to it survives an unrelated declaration.
- **An object of the wrong kind is named when a usage is not instantiated.** `-state
  "Rover::modes rover"` after `-instantiate Rover` (the definition, not the usage) reported only
  `no instance of "rover" (use %instantiate first)`. The REPL and the CLI now say that an object
  of the definition exists, not of the usage, and name what to instantiate instead: `no instance
  of the usage "Fleet::rover": object #1 of "Fleet::Rover" is of its definition "Fleet::Rover",
  not of the usage — use %instantiate Fleet::rover to create the usage's object, or name
  Fleet::Rover to address it`. Asking for a definition when only usages typed by it have
  objects names those objects the same way — a nested one by its path (`Fleet::driver.r`), an
  element of a multi-valued part by its index (`Depot::garage.bays[2]`) — and a usage reaches its
  definition through the usages it subsets; with no related object the plain hint stands. The
  hint names only objects the session holds and materializes none to find them.
- **An id reaches an object the session holds, and looking it up builds nothing.** `%features #4`
  used to materialize the features of every named object on the way to finding object #4; an id
  now denotes an object the session holds — one it named, one a materialized feature of such an
  object holds, members of a multi-valued part included however many there are, or one a second
  `%instantiate` of its name displaced — and is found without materializing anything. An id the
  runtime never issued is `no object #9 in this session: nothing materialized has that identity
  (the objects are #1, #2)`. The second `%instantiate` says how the first object goes on being
  reached — `Fleet::rover now denotes this object; object #1 is displaced from that name and stays
  reachable as #1` — and a `%state` or `%action` session over the displaced object keeps running,
  the same notice saying it now follows the object as `#1`; a session over an object another name
  denotes is untouched.

- **A `then` written after a flow, a binding or a standalone succession comes back from Turtle.**
  The parser sequences a positional `then` from the nearest preceding member that is not itself
  an edge — flows, bindings, connectors, successions and transitions are skipped, attributes and
  docs are not — but the Turtle writer took the member written immediately before it, found the
  flow there and refused the whole file as inconsistent. Both sides now share one rule,
  `ast.UsageKind.IsEdge`, and the writer compares the graph's source end against the name the
  skipped-to member answers to (an unnamed `perform x` or `action redefines x` answers to `x`, as
  in the parser), so `first start; then a;` and `then perform run;` fold back too. A graph whose
  `sysx:sourceMember` or `sysml:sourceFeature` names some other member is still refused. Over the
  345-file example corpus the files the writer refused for this reason go from 14 to 0, and their
  round trips reproduce the same graph; the training examples for action shorthand, control
  structures, decisions, merges, terminate actions, messaging and message payloads are among them.

- **The notation the RDF writer spells is read back to the same graph.** Converting a model to
  Turtle, back to notation and to Turtle again lost flags the first graph carried, because the
  writer re-spelled a head in a form the parser read differently: `ref x subsets y;` and
  `composite frontWheel redefines w[2];` lost `ref` and `composite` (the parser only continued a
  modifier-led declaration into a symbolic `:>`, not the keyword spellings), `#derive end r : R;`
  lost `end` and `end ref attribute e : S;` lost `ref` (the `end … kind` path applied only the
  end flag), a nested `private import Pkg1::*;` came back as `Pkg1::**` (the two import suffixes
  were written as exclusive), and a succession end whose name needs quotes was carried as text
  and refused when written. The parser now reads every modifier ahead of the kind keyword, the
  writer spells the modifiers in the grammar's order with the multiplicity beside the clause it
  qualifies, an import writes `::*` and `::**` independently, and a quoted succession end is a
  reference to the element like an unquoted one. Five fixtures under
  `internal/core/export/testdata/convert/` lock this in by re-encoding the notation written
  from the graph alone and comparing the two graphs as triple sets; a relationship's symbolic or
  keyword spelling and a doc body's line endings are documented as normalised. On the corpus
  ratchet, six files move from a differing graph to the same one and six refused for a quoted
  succession end now round-trip; the seventh is written back, but its guarded succession
  (`succession S first A1 if x == 0 then A2;`) is spelled as a `transition` the parser does not
  read, which is a separate writer defect.

## 0.4.3 — 2026-09-02

Release 0.4.3 is where an element gets an identity the notation can carry. The SysML v2 textual
notation deliberately records no element identity, so a model saved as `.sysml` and re-parsed had
fresh ids everywhere and a rename was a delete plus a create. An element may now declare the
repository element it *is* — standard user-defined metadata (`@ElementId`, with a `ProjectRef`
binding a document to its repository once, at the root namespace), shipped as an `IdentityMetadata`
library extension that any conforming tool already parses and preserves. Identity is validated
(id shape, scope binding, uniqueness), survives `notation → RDF → notation`, and
`sysml -sync-diff` computes the change set between a model and a repository graph keyed by that
identity, so a rename or a retype is an update to the same element. The design is
[a project record](docs/project/element-identity-annotations.md), and the notation has been
submitted to OMG for standardization
([the issue text and its status](docs/project/omg-issues.md)).

A solver verdict is now the evaluator's verdict. The SMT translation reasons over exact rationals
while the evaluator computes in IEEE 754 binary64, and the difference is reachable: the exact
encoding holds `0.1 + 0.2 == 0.3` sat, which the evaluator rejects. Every `sat` witness is now
replayed through the evaluator's own arithmetic before it is reported, a query whose conditions the
evaluator rounds is marked and its exact-real `unsat` reported undecided rather than as an
evaluator verdict, and a whole-number quotient divides as an exact ratio rounded once — `5 / 2` is
`2.5`, as the reference evaluates it. The remaining alternative, an exact-rational evaluator value
representation, was adjudicated against the pinned pilot and the specification text and
[declined](docs/project/exact-rational-evaluation.md).

The four non-Go clients and the public Go API were each exercised by worked examples over a fully
capable model — quantities, enumerations, multiplicity, nesting, unvalued features — run by the
test suites so they cannot drift, and the defects that tour surfaced are the client and runtime
fixes below. Each client now has a reference page of its own, the Java client's package moves to
`org.openmbee.opensysml` (the client is unpublished, so no released consumer moves with it), and
the Python client 0.4.0 was published to PyPI. The conformance suite and the pilot differential
now also render their runs as JUnit XML — and the differential as SARIF — so CI shows them as test
results rather than artifacts to download.

A profiling pass across the toolchain removed the costs a September census found rather than the
costs assumed: a multi-file `-validate` batch is indexed once instead of once per file — 6.8 s to
0.30 s over the 100-file training corpus, the quadratic term gone — a calc invocation reuses a
pooled frame instead of allocating ~1.7 KiB per call, a run target resolves from a per-document
name table instead of an O(model) scope walk, the parser's token buffer became a bounded window
(a load allocates 30% fewer objects and holds 16% less live heap), and the `about`-metadata index
is cached when the library index freezes, restoring the empty-session floor the census flagged.
The census and an execution-performance measurement are recorded as project records
([performance census](docs/project/performance-census-2026-09.md),
[execution performance](docs/project/execution-performance-2026-09.md)).

No model that validated under 0.4.2 stops validating and no import path moves.

### Added

- **A complex number crosses the wire as one value.** `Value` gains a `complex` arm carrying the
  real and imaginary parts as two doubles, so a `Complex` feature value, evaluation result, action
  output or calc result arrives as one number rather than the `unsupported` null it was reported
  as, and a complex action input or calc argument is accepted. Every shipped client maps it to one
  native value — Go `opensysml.Complex`, Python `complex`, TypeScript `ComplexValue`, Java
  `Value.ComplexValue`, Rust `Value::Complex` — and prints it in rectangular form. The service
  advertises the `complex_values` capability; without it, a complex in a response is the
  unsupported null as before, and a complex sent in a request is refused with `UNIMPLEMENTED`.
  The Go and Python clients check the capability before sending one, so an older service is a
  capability error rather than an input silently read as null. The Python generator emits
  `complex` for a `Complex` feature (emission schema `4`).

- **Documents render as semantic, styleable HTML.** `-doc-form html` writes a document from the
  compiled document tree itself rather than by converting the Markdown, so the model facts Markdown
  cannot carry reach the markup: `<article>`, nested `<section>` whose heading levels follow the
  nesting, `<table>` with `<caption>`/`<thead>`/`<th scope="col">`, `<figure>`/`<figcaption>`,
  `<nav>` contents and semantic inline runs, hooked by a small `sysml-` class vocabulary and
  `data-` attributes for the content kind and name, the query behind a table or list, the group-by
  column, each row's or item's selected element and element kind, each cell's projected column and
  value kind, and a diagram's view, kind and direction. Identifiers are the Markdown anchors, so a
  `Ref` resolves within a page and, under `-render-documents -doc-form html`, across a linked set
  whose pages share one `sysml-document.css`. The default stylesheet sits in an `@layer opensysml`
  cascade layer and draws every value from `--sysml-*` custom properties, so unlayered reader CSS
  overrides it without `!important` or specificity fights, and no `style` attribute is ever
  emitted; `-html-default-css`, `-html-css` (repeatable; a file is inlined, a URL linked),
  `-html-no-default-css` and `-html-fragment` shape that. Output loads nothing over the network,
  runs no JavaScript of its own — a diagram's Mermaid source sits in `<pre class="mermaid">` — and
  is byte-identical between runs. The title page, contents and numbering options are now
  `-doc-title-page`, `-doc-toc` and `-doc-number-sections`, shared by HTML and PDF, with the
  `-pdf-*` spellings kept as aliases. Markdown is unchanged, and the PDF engines still read their
  own HTML for now.

- **An element declares its repository identity in the notation.** `@ElementId { id = "…"; }`
  annotates the element it is written about, and `@ProjectRef { projectId = "…"; }` on a root
  namespace binds the document to its repository, so element-level ids inherit their scope.
  Identity is opt-in per element: an element without an annotation keeps today's derived,
  latest-wins identity. The two metadata definitions ship as a non-normative library extension
  (`IdentityMetadata`, entering the same gates as the vendored files — the bundled-library check
  now reports 97/97 clean), and a constraint-tier pass validates id shape, scope binding and
  uniqueness across the workspace, including anonymous about-form usages, annotations declared in
  libraries, and targets outside the built roots.

- **Identity survives the RDF round trip.** The writer mints subject IRIs from the effective id —
  the declared one where an annotation exists, the encoded qualified name where none does — marks
  declared ids, writes `ProjectRef` bindings as provenance triples, and refuses colliding ids
  across a mixed-scope workspace rather than silently merging two elements. The reader keys
  subjects on the element id (with the name-encoding fallback for old graphs), reports a dangling
  id as its own error, and re-materializes the annotations on the way back to notation, so
  `notation → RDF → notation` preserves which repository element each declaration is.

- **A model diffs against a repository, keyed by identity.** `sysml model.sysml -sync-diff repo.ttl`
  reports the change set — creates, updates, deletes, and renames seen as updates to the same
  element — and never writes: applying is a separate step. `-sync-base` names the graph at the
  last-seen commit, so repository changes since then surface as conflicts rather than silent
  overwrites; deletes are reported always and confirmed with `-sync-confirm-deletes`;
  `-sync-mint-ids` mints a UUID for each unannotated element being created and `-sync-annotate`
  writes the model back out with each minted id declared, preserving the source text and quoting
  names as the notation requires. The last-seen commit is tool state beside the model
  (`<model>.sync.json`), never written into the notation.

- **The editor mints an element id on request.** `sysml-lsp` offers the `refactor.rewrite` code
  action "Annotate … with a minted element id" on the header of a declaration that carries no
  `ElementId`. Invoking it mints a UUID v4 and writes the annotation as a text edit that leaves
  every other byte of the file alone: inline at the head of a body, standalone about-form at the
  end of the file for a bodiless declaration, names quoted as the notation requires. Where the
  root carries no `ProjectRef`, the same edit binds it with a placeholder `projectId` to fill in,
  and a "Bind … to a project" action does that alone. Nothing mints during analysis, and no
  diagnostic asks for an annotation. A metadata usage in a constraint body (`@Tag { … }`) now
  parses as the member it is, distinct from a classification condition (`@Tag`).

- **The conformance suite and the pilot differential render as CI test results.** The conformance
  runner emits JUnit XML (stored even when the gate fails), and `pilot-diff` writes JUnit XML with
  one suite per corpus root alongside SARIF 2.1.0 with one result per disagreeing diagnostic
  group, located on the compared model file — the same run, in the renderings CI dashboards and
  code-scanning consoles read.

- **Worked examples for every client, run by the tests.** Runnable examples drive the Node, Java,
  Rust and Go clients over one capable model — parsing, diagnostics, symbol navigation, evaluation
  and instantiation — and each client gains a reference page
  ([Java](docs/reference/java-api.md), [Node](docs/reference/node-api.md),
  [Rust](docs/reference/rust-api.md)). The examples were written to find defects and did; the
  fixes are below.

- **The implementation models itself.** [`examples/self-model`](examples/self-model/README.md) is
  the analysis pipeline, surfaces, invariants and views of this implementation written as a SysML
  v2 model across five files whose packages import each other, with a make target rendering its
  diagrams and documents and a test evaluating its invariants and checking its figures against the
  implementation they describe.

- **`-render-document` takes a model of several files.** A document may query elements its sibling
  files declare: `sysml model/*.sysml -render-document Reports::MassReport -o report.md` loads the
  named files as one model.

- **Two design records.** [Exact-rational evaluation](docs/project/exact-rational-evaluation.md)
  adjudicates and declines a `big.Rat`-backed evaluator, with the pinned pilot's verbatim binary64
  answers as evidence and a census showing no marked-rounded query is recoverable by per-term
  narrowing; [the HTML document backend](docs/project/html-document-backend.md) records the agreed
  design for rendering documents as semantic, styleable HTML straight from the document IR —
  proposed, not implemented.

### Performance

- **A multi-file `-validate` batch is indexed once.** Validation loaded each file through a path
  that reopened the session document, reindexed it and re-expanded every wildcard import, so a
  batch of N files paid N full indexes over a growing buffer. The batch is now a single
  submission (`Session.LoadFilesSummary`), with each file's own syntax errors, load notices and
  summary still printed in file order. Over the training corpus: 0.26 s → 0.12 s at 25 files,
  0.59 s → 0.13 s at 50, 6.8 s → 0.30 s at 100 — a fixed floor plus a term that scales with the
  input, no quadratic term. The shared symbol walk also stops rebuilding a visited map that its
  cached, deduplicated symbol list already guarantees.

- **A calc invocation reuses a pooled frame.** Each invocation allocated a fresh parameter map,
  evaluation context, statement host and engine — ~1.7 KiB per call, and GC took half the CPU of
  recursion-heavy evaluations. Returned frames go on a free list and the next invocation runs in
  one; a frame is only ever held by one active invocation, so recursion never aliases. Default
  bindings now run in the invocation's own activation, so a value read while binding is memoized
  per invocation rather than leaking to the next one.

- **A run target resolves from a per-document name table.** `RunCalc`, `RunStateMachine` and
  `InstantiateNamed` re-walked the whole document scope tree per run, so a run cost O(model). The
  session now tabulates its documents' simple names once per scope tree and answers lookups from
  the table, rebuilt when a submission or reset replaces a document. At 4,000 elements a
  state-machine start goes 204 µs → 6.4 µs, a calc 222 µs → 4.5 µs, an instantiation
  217 µs → 3.3 µs, and the figures no longer scale with model size.

- **The parser's token buffer is a bounded window.** The parser buffered every non-trivia token of
  a file up front — ~48 bytes per token before reading any of them; consumed tokens are now
  dropped once no checkpoint can rewind to them, so backtracking still sees what it needs while
  the buffer stays bounded by the lookahead. With the REPL parsing each submitted file once
  rather than twice, a load allocates 30% fewer objects and holds 16% less live heap.

- **The `about`-metadata index is cached at index freeze time.** Building it walked the scope tree
  of every document — the bundled standard library included — once per session. A frozen index is
  immutable, so its `about`-usage symbols are recorded once at freeze and only workspace documents
  are walked per session; the empty-session floor returns to ~0.19 ms and 117 KiB allocated,
  −80% wall and −83% allocation, with the annotations found and their order identical.

- **A repeated REPL command does not re-parse its text.** The session keeps what an argument list
  and a command's name text parsed to, keyed by the exact text, so a repeated
  `%calc`/`%action`/`%state`/`%instantiate` does not rebuild a source, lexer and parser per call.
  Evaluation still runs on every call against the current session state.

- **Two measurement records.** The [September 2026 performance census](docs/project/performance-census-2026-09.md)
  measures week-over-week benchmark movement and whole-binary scaling, and flagged the regressions
  fixed above; the [execution-performance record](docs/project/execution-performance-2026-09.md)
  measures evaluation throughput — ~1.5 µs per calc invocation — and names the optimization gaps
  the profiles show.

- **A process starts in under 20 ms instead of 100.** Every `sysml`, `sysml-lsp` and `sysml-grpc`
  start, and every test that builds a model, first parsed the 97 bundled OMG library files, indexed
  them and expanded their wildcard imports — about 100 ms and 467k allocations before the model was
  looked at. The library's frozen index is now serialized once, at `go generate` time, into
  `internal/core/libs/stdlib.snapshot` — a hand-rolled binary format (varints over a string table,
  a node table per syntax-node type, index references in place of pointers; no `encoding/gob`, no
  reflection) that is embedded in the binary and decoded at start-up, reproducing the object graph a
  fresh load builds. `bin/sysml -memstats -e "2+3"` over a one-part model goes from 95–102 ms,
  53.3 MiB and 466.9k allocations to 17–23 ms, 32.4 MiB and 67.1k; the `sysml` binary grows from
  16.9 to 20.5 MB. The OMG files stay the source of truth: the snapshot records their digest and a
  format version, and a process whose bundled files, `OPENSYSML_LIBRARY_PATH` override or snapshot
  format do not match parses the files as before. `make stdlib-snapshot` regenerates it; a test and
  a CI check fail when the committed snapshot lags the files or the indexing code. The parse path
  itself is also faster — the files are hashed and parsed concurrently and added to the index in
  the same order as before, and wildcard expansion no longer re-sorts namespace children out of a
  map on every enumeration (33 ms → 31 ms over the library).

- **The calc evaluator does less work per invocation.** `runtime.Value` is 64 bytes instead of
  120, so a value returned through the evaluator's nested frames copies half as much; parsed
  literals and resolved invocation targets are memoized per evaluation context, keyed by the
  syntax node an edit replaces; a calc's parameters bind into slot-indexed frames resolved once
  per calc, with a bare name answered from the frames before the general resolution chain; and an
  invocation's arguments and frame stack are borrowed from per-context storage. A recursive
  `Fib(25)` costs 0.65 µs per calc invocation instead of 1.01 µs and allocates about 160 objects
  per evaluation instead of 971 000. Results, errors, traces and step counts are unchanged; the
  measurements are recorded in the
  [execution-performance record](docs/project/execution-performance-2026-09.md).

- **Pure calc bodies compile to a closure fast path.** A calc whose body is one scalar expression
  — Integer, Real and Boolean literals, its own `in` parameters, the arithmetic, comparison,
  equality, identity, logical and conditional operators, and invocations of other such calcs,
  recursion and cycles included — is compiled on its first invocation into a tree of Go closures
  over an unboxed scalar frame: parameters are slot indexes, callees are resolved once, and values
  are boxed only at the invocation boundary. A recursive `Fib(25)` costs 21–22 ns per calc
  invocation instead of 519–532 (CPython 3.12 takes 27 ns for the same function on the same
  machine). Values, errors, error timing and step counts are identical to the reference evaluator's
  — a differential test invokes every eligible calc in the fixture and example trees through both
  tiers on generated edge arguments — and anything outside the subset (calc usages, `out` features,
  feature chains, collections, quantities, strings, locals, non-literal defaults, redeclared
  parameters) stays on the evaluator, as does every traced, named-argument or non-scalar
  invocation. `OPENSYSML_CALC_COMPILE=0` turns the tier off for bisecting.

### Changed

- **A `sat` is a witness the evaluator confirms; an `unsat` is claimed only where the arithmetics
  coincide.** Every satisfying assignment is replayed through the evaluator's float64 arithmetic
  and reported `unknown` with the reason when the replay rejects it; a query whose conditions the
  evaluator rounds is marked, `%check` and `%solve` report its exact-real `unsat` as undecided,
  and `%configure all` and `%optimize` decline the completeness claim outright. Narrower, and
  sound — what is given up is completeness on rounded queries, and the census over the
  repository's solver-facing corpora found none of them recoverable.

- **A whole-number quotient is a Rational.** `5 / 2` answers `2.5` for `Natural` and `Integer`
  operands alike, which is what the reference evaluator answers; the quotient is computed as an
  exact ratio rounded once to float64, so it agrees with the exact SMT encoding even beyond 2^53,
  where rounding each operand first moves the answer. The library's declared `Natural` return is
  recorded as a question for OMG in [omg-issues.md](docs/project/omg-issues.md).

- **The Java client's package is `org.openmbee.opensysml`**, the DNS-verified namespace every
  future Java artifact belongs under, rather than `io.opensysml`. The client has never been
  published, so no released consumer moves with it.

- **A pilot corpus records the pin it was fetched at.** Each corpus directory carries a stamp
  naming the repository and tag it came from: a stale stamp triggers a re-download when the script
  next runs (keeping the old copy until its replacement has been fetched), a current one is left
  alone, and a directory without a stamp is left alone with a warning.

- **The architecture self-model describes the library snapshot.** The standard library stage in
  [`examples/self-model`](examples/self-model/README.md) now carries the embedded snapshot its
  index is decoded from, the `internal/core/pack` and `internal/core/ast/astcodec` units that
  encode it and the generator that writes it; `LoadLibrary` models the load as an action whose two
  decisions — the digest and format match, then the checksum — choose between decoding the snapshot
  and parsing the files; `stdlib-snapshot-check` is an eighth, gating conformance oracle; and a
  ninth invariant, `snapshotIsDerived`, states that the snapshot is checked against the files and
  never the only way to load them. The evaluator now declares itself memoized, since it keeps its
  per-node caches in side tables beside the tree. The self-model test compares each new claim with
  the implementation: the override variable, whether the embedded snapshot decodes for the bundled
  files, the Make targets and the CI step, and the side tables `runtime.Context` keys by syntax
  node. The architecture document gains a section and diagram on loading the library, and the
  pilot differential baseline is re-recorded for the larger model: the eleven new rows are all the
  reference's, of shapes the self-model already drew.

- **The architecture self-model describes the compiled calc tier.** The evaluator now carries a
  `CalcCompiler` part — memoized, switched off by `OPENSYSML_CALC_COMPILE`, falling back to the
  evaluator — and `InvokeCalc` models one invocation as an action whose three decisions (a traced
  run, a pure body, positional scalar arguments) send it to the compiled tier or to the evaluator
  whole. A tenth invariant, `evaluatorIsReference`, states that the compiled tier is an optimization
  of the evaluator and never a second semantics; `CalcDifferential` names the parity and
  differential tests that verify it. The self-model test checks the variable's name against
  `runtime.CalcCompileEnvVar` and the environment reference, that a fresh `runtime.Context`
  compiles calcs until that variable says otherwise, and — invoking the model's own `StepBudget`
  through both tiers — that they agree and that a traced run takes the evaluator. The architecture
  document gains a paragraph and diagram on invoking a calc.

### Fixed

- **An OSLC query's unknown-property diagnostic names properties the query can be written with.**
  `sysml:id="x"` answered with the Go API's own property names (`@id, @type, declaredName, …`), so a
  caller who wrote one back got a second, different error: `@type` and `@id` are not OSLC query text.
  The list is now the OSLC predicates (`rdf:type, sysml:declaredName, …`), derived from the mapping
  the parser reads, and `@type`/`@id` name their OSLC spelling instead — `rdf:type`, and identity,
  which every result reports rather than asks for. The list follows the query's own `oslc.prefix`
  bindings, since a rebound `sysml` or `rdf` changes what the parser accepts. An `oslc.prefix`
  binding whose prefix no prefixed name can be written with (`!s=<…>`) is refused where it is
  bound rather than accepted and never usable, and a prefix of letters outside ASCII
  (`sÿsml=<…>`) now scans as one name in a query instead of ending mid-letter.

- **An anonymous `doc` or `comment` before a kind keyword is kept.** `doc /* … */` followed by
  `attribute a;` in a definition or usage body parsed as an attribute prefixed by `doc`, so the
  documentation vanished silently from the model — absent from validation, printing, the LSP and
  RDF conversion (Open-MBEE/OpenSysML#85). A `doc`, `comment`, `rep` or `locale` annotation ends
  with its comment body and never qualifies the member after it; the bundled standard library
  gains the documentation this had been dropping.

- **A wider-typed expression binds to a narrower feature.** `return : Integer = 7 / 2;` was
  refused statically because the quotient's type is `Rational`, yet an expression's static type
  only bounds its values — `4 / 2` is whole. A binding, argument or index is now refused
  statically only when the two types are disjoint or the value is a literal, whose type is exact;
  everything else is deferred to evaluation, where the value it actually turns out to be is still
  checked. This matches the pinned pilot, which accepts the declaration and evaluates it.

- **Every exported `Session` method holds the session lock.** The run, check/solve, view, document
  and diagnostics entry points did not take it, so a Tab completion racing one of them touched the
  lazily built index, name table and runtime context unsynchronized — 144 races under `-race`,
  now none, with a test running every entry point concurrently.

- **Node client:** restarting the service waits for the previous process to exit before
  reconnecting, and does not wait on one that already exited.

- **Arithmetic outside a type's range is reported, not returned.** A wrapped Integer sum, the
  least Integer's negation and its remainder, an infinite Real from an out-of-range literal, a
  folded infinity, and a quantity magnitude outside the Real range each answered as if computed;
  every one is now a typed error naming the range. An ordinary negated literal evaluates rather
  than being mistaken for a fold, seeded outputs are reported for what they are, and an escaped
  attribute default is decoded before use.

- **An unbound requirement subject is reported as unbound.** A requirement checked with nothing
  supplying its subject read as a modelling mistake in the condition (`no value for feature
  sensor`); the diagnostic now names the subject and the three ways to supply one — bind it, check
  it on an object, or assert satisfaction by an element.

- **A debounced call a later trigger superseded does not run.** A timer firing as the next trigger
  arrived ran the work its successor now owned and deleted the successor's entry; a callback now
  confirms it is still the timer its key waits on.

- **Node client:** a short name is looked up on a model adopted by hash; a missing symbol's error
  names what was looked for rather than repeating the service's text; an RPC failure surfaces as a
  typed client error rather than a raw transport error; an impossible encoding or timeout is
  refused at construction rather than carried into a connection; and a failed handshake carries
  the status it failed with.

- **Java client:** a call that outlives its request timeout is reported as the timeout it is, not
  as the service being unavailable, and a value kind the client cannot read is a refusal rather
  than a value silently dropped from the sequence holding it.

- **Rust client:** an empty `$OPENSYSML_SERVICE` no longer selects an unnamed binary, a response
  above the transport's 10 MB default is read, a qualified value is read outside its declaring
  scope, string escapes decode, Integer overflow and non-finite Real folding are reported, and an
  instance graph iterates in declaration order rather than map order.

- **The toolchain download paths close their quality-gate findings.** The stall watchdog owns a
  thread rather than an executor, the pandoc fetch refuses a plaintext redirect, and the mermaid
  install runs no dependency's lifecycle script and finishes before calling itself present.

- **The pilot reference is pinned to a commit, and a stale copy of it is replaced.** The corpora,
  training examples, Xpect suites and grammars were fetched by release tag alone — a name the
  upstream repository can re-point — and a corpus directory fetched at an earlier release was kept
  with only a warning, so a checkout re-pinned from `2026-05` to `2026-07` still measured the old
  material (98 example files where the baselines record 99) and every provenance test failed at an
  unchanged pin. `scripts/pilot-pin.sh` now names the commit the tag must resolve to and every
  fetch refuses a tag that resolves elsewhere; each fetched directory is stamped with the tag,
  commit and repository, and a copy stamped with another pin or not stamped at all is re-fetched;
  and the committed baselines record the commit they measured next to the tag.

## 0.4.2 — 2026-08-31

Release 0.4.2 is where document generation from a model becomes a working pipeline. 0.4.1 shipped the
planning layer alone — nothing executed and nothing rendered. A document's queries now run (named
invocation under a shared budget, relationship traversal, computed expression columns), an evaluated
document renders as CommonMark Markdown or, through an external converter, as PDF, generated diagrams
and cross-document references are content the document declares, and a whole document set is written
as one atomic replacement. Every surface reaches it: `%run-query` and `%render-document` in the REPL,
`-run-query`, `-render-document`, `-doc-form` and `-render-documents` on the command line,
`RunDocumentQuery` and `RenderDocument` over gRPC and on the public Go API, and
`opensysml/documents`/`opensysml/renderDocument` in the LSP, behind the VS Code extension's
`SysML: Render Document`. The [document-generation manual](docs/manual/README.md) documents the
pipeline with examples that are rendered by the binary the release ships.

The public Go API stops being a subset of the service: behaviour, verification, search, reporting,
conversion and editing are all methods on `opensysml.Client`, and a model spread over several files
is parsed and indexed as one model rather than concatenated. The four non-Go clients now provision
the service the same way — each downloads a `sysml-grpc` release and verifies it against digests
pinned in one shared table, refusing an unverifiable release rather than answering from a cache.

Two example walkthroughs were written to find defects and did: a relay probe across its mission
phases and a bomb-disposal team around the robot. What they found is the runtime and parser half of
this release — a structural `first a then b;` that parsed as an initial node and shadowed the
snapshot it named, sends that could not cross a connector their owner declared or reach a nested
part's identity, signals matched by short name instead of resolved identity, and a bound subject
that did not carry its subject's type.

Configuration unifies on the `OPENSYSML_` prefix, with the `SYSML_` spellings still accepted, and the
load path allocates 15% less on a 12,000-element model. No model that validated under 0.4.1 stops
validating and no import path moves.

### Added

- **A document definition written in the model renders as Markdown.** `%render-document` in the REPL
  and `-render-document` on the command line compile a `part def` specializing
  `DocumentQueries::Document`, run its queries against the loaded model, render its diagram blocks
  through the view engine, and write CommonMark. Sections nest, paragraphs and lists compose
  statically-authored inline runs (`Span` with a `plain`/`emphasis`/`strong`/`code` style, `Link` to a
  URL, `Ref` to another block's anchor) with query-backed values styled through nested `SpanColumn`
  and `LinkColumn` column runs, a table's columns are the query's projected properties and its
  computed `Column` names, and a `groupBy` column writes one subtable per group value. A `Diagram`
  block embeds a declared view — or an element with a stated rendering kind — as a fenced `mermaid`
  block, a table-kind view as a pipe table, with an optional caption and flow direction. Rendering is
  deterministic: the same model and document produce the same bytes.

- **The same document renders as PDF, through a converter chosen at run time.** `-doc-form pdf`
  converts the rendered Markdown with `weasyprint` (default), `pandoc` or `prince`, selected by
  `-pdf-engine`, so the binary links no PDF renderer and Markdown output needs none of them;
  `-pdf-title-page`, `-pdf-toc` and `-pdf-number-sections` are the document-level options. A PDF is a
  binary artifact, so `-doc-form pdf` requires `-o`, and a missing tool stops the run with a typed
  error rather than a partial file.

- **A model's documents render as one linked set.** A `Ref` may target a block in another document,
  and `-render-documents <dir>` renders every document the model declares into a directory, one file
  per document, so those references resolve as on-disk links. The set is committed atomically: the
  rendered files replace their destinations together, a failure restores what was there, and a crash
  cannot leave half a set behind.

- **Document queries execute.** A query definition specializing `DocumentQueries::Query` runs as a
  pipeline over the model — filtering, ordering, projection, named relationship traversal, and
  invocation of another named query with explicit bindings under one shared visit-and-invocation
  budget — and a `Column(name = …, expression = …)` projection is evaluated per row. `%run-query` and
  `-run-query "<name> [<p>=<expr>…]"` report the rows directly, which is how a query is written and
  checked before a document consumes it.

- **The service, the LSP and the editors expose the pipeline.** gRPC gains `RunDocumentQuery` and
  `RenderDocument`; the LSP gains `opensysml/documents` and `opensysml/renderDocument` behind an
  `openSysmlRenderDocument` experimental capability, plus completion and hover for query authoring;
  and the VS Code extension's `SysML: Render Document` renders the model as currently typed into a
  Markdown preview beside the editor.

- **Every client provisions `sysml-grpc` from a verified release.** The Node, Java and Rust clients
  download the service binary for the host platform and verify it against a SHA-256 digest pinned in
  the repository, and the Python client resolves a named or `$PATH` binary the way the others do.
  Downloads are staged per process, bounded, and taken under a shared cache lock, an unverifiable
  release is refused rather than answered from a cache, and the pins themselves are generated into
  every client from one shared table.

- **The public Go API covers every operation the service answers.** `ExecuteAction`,
  `ExecuteState`, `VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`, `EvaluateCalc`,
  `Query`, `QueryOSLC`, `RunDocumentQuery`, `RenderDocument`, `Convert`, `ConvertSource`,
  `ConvertFile` and `ApplyEdits` join parse, lookup, evaluation and instantiation on
  `opensysml.Client`, in-process and over Connect alike, so an embedding program no longer drops
  to the generated protobuf stubs for behaviour, verification, search, reporting, conversion or
  editing. Queries are written with typed conditions (`Equals`, `Greater`, `Less`, `All`, `Any`
  and `Not`, which De Morgans a composite rather than sending a shape the service rejects);
  a verdict that is false or undecided is returned as an answer, while a request that cannot be
  answered at all is a `VerifyError`, and a group of edits that will not apply is an `EditError`
  naming the failure and the elements still referring to the target.

- **A model spread over several files is parsed as one model.** `ParseFiles` and
  `ParseDocuments` on the public Go API — the `ParseSources` RPC and the `parse_sources`
  capability on the service — parse each document on its own and index all of them together, so
  an import between them resolves and every symbol of the set is one lookup, evaluation or
  instantiation away. Nothing is concatenated: a document keeps its own name, a diagnostic
  locates itself in the file it came from, and `Model.Roots` holds each document's root. The two
  operations that write one document's notation back out — conversion from a model hash, and
  editing — refuse a model of several documents rather than picking one.

- **A relay-probe walkthrough for the identity and lifecycle notation:**
  [`examples/relay-probe-demo`](examples/relay-probe-demo/README.md) models one individual probe
  across its mission phases — event occurrences ordered in time, snapshots and a timeslice of one
  individual, occurrences with multiplicity, a calculation reading a feature across two snapshots,
  a requirement whose bound subject is a snapshot, and a beacon inside a timeslice sending
  telemetry through the probe's own antenna. It was written to find defects, and found the three
  below.

- **A second bomb-disposal walkthrough, written for the notation the first one does not reach:**
  [`examples/disposal-team-demo`](examples/disposal-team-demo/README.md) models the team around
  the robot — quantities with units and a payload budget, `select` and `reduce` over the fleet,
  a command crossing the connector the site joins two parts by, a callout occurrence with a
  snapshot and a timeslice, and a requirement, use case, verification case and analysis case over
  the same subject. It was written to find defects, and found the three below.

- **A document-generation manual**, [`docs/manual/`](docs/manual/README.md): the concepts, the
  smallest working document end to end, a query cookbook, document authoring, the output forms and
  their determinism, the CLI/REPL/gRPC/Python interfaces, a worked example and troubleshooting. Every
  snippet in it parses and every rendered output shown was produced by the binary this release ships,
  and the documentation link checker now reads the bracketed and angle-bracketed destinations those
  pages use.

### Changed

- **Configuration is spelled `OPENSYSML_`.** Every variable `sysml`, `sysml-lsp` and `sysml-grpc`
  read — the library path, the six execution budgets and the gRPC index pool — uses that prefix. The
  legacy `SYSML_`-prefixed names remain accepted indefinitely; when both are set and the
  `OPENSYSML_` value is non-empty it wins, and setting only the legacy name prints a one-time
  deprecation warning naming the form to switch to.

### Performance

- **A load allocates 15% less.** Three allocation sources paid once per token or per name parsed — a
  fresh string for every keyword's text, a parts slice for every qualified name, and a
  redefinition-closure map per inherited symbol — now cost one allocation per file or none. On a
  12,000-element model that is 3.69M allocations rather than 4.33M and 474.1 MiB rather than 487.7,
  with wall time unchanged: the win is collector pressure, not bytes. Diagnostics and exit status
  were verified byte-identical against the previous binary.

### Fixed

- **An OSLC `<uri>` value selects what the prefixed name selects.** A term written
  `rdf:type=<https://www.omg.org/spec/SysML#PartUsage>` parsed and then matched nothing, because a
  URI value was compared whole while `rdf:type=sysml:PartUsage` was reduced to the local name the
  model holds. Both forms now reduce alike for the SysML namespace; a URI outside it is still
  compared whole.

- **An OSLC query parameter this implementation does not read is refused, not ignored.** A misspelt
  `oslc.wheree=…` was dropped and the query then selected the whole model — the widest possible
  wrong answer, reported as success. An unknown parameter, a parameter given twice, a parameter
  written with no value (`oslc.where=`, `oslc.select=`, `oslc.orderBy=`, `oslc.prefix=`), and a
  non-wildcard `oslc.properties` (which names `oslc.select` in its message) are now typed
  malformed-query errors.

- **An unquoted model qualified name says what to write instead.** `sysml:qualifiedName=Robot::Platform::battery`
  reported `OSLC prefix "Robot" is unbound`, naming a prefix the caller never wrote; it now names
  the quoted literal form the value needs.

- **A query that matches nothing says so.** Both query surfaces printed nothing at all, which a
  caller could not tell apart from a query that failed to run: `%query` now prints `no elements
  matched`, and `sysml -query` reports it on standard error, so the result rows on standard output
  remain one line per match.

- **A `*` value is refused off the multiplicity bounds, and empty `-query` text is a misuse.**
  `sysml:name=*` was compared as the literal value `*` and reported a successful no-match, while the
  same wildcard elsewhere in a query was a typed refusal; it is now refused on every property but
  `multiplicityLower` and `multiplicityUpper`, where `*` is the model's own infinity value.
  `sysml -query ''` was indistinguishable from an absent flag and started the interactive REPL
  instead of answering.

- **The public Go API holds its contract for a binary that imports it.**
  `ServerInfo.Version` reports the OpenSysML module version the importing program resolved rather
  than that program's own; an in-process call honours its context as the wire does, refusing a
  context already done; every call after `Close` is refused with `CodeUnavailable`, and closing
  twice is not an error; and a `StatusError`, a quantity, an enum literal and an unset value print
  as a caller would write them rather than as Go struct dumps.

- **A caption is no longer confusable with an emphasized paragraph.** The
  Markdown renderer wrote a table or diagram caption and an emphasis-only
  paragraph identically as `*text*`, so the PDF backend styled every such
  paragraph as a caption. The renderer now precedes every caption with a
  `<!-- caption -->` metadata line — invisible in ordinary Markdown
  renderers — and the PDF backend styles only marked lines as captions,
  rendering bare emphasized lines as ordinary paragraphs. A marker without
  a caption line after it is a typed `dangling-caption` error.

- **A structural `first a then b;` is a succession, not an initial node.** Ordering two
  snapshots of an individual in time (`first postSeparation then postFlyby;`) parsed as an
  initial-node member named `postSeparation`, which shadowed the snapshot it named — the first
  portion read as `<unknown>` and everything downstream of it (a calculation's default, a
  requirement's bound subject) failed. A two-ended `first ... then ...` in a structural body now
  parses as a `SuccessionAsUsage` over its members; the one-ended `first a;` stays an initial
  node, and an action-carrying body's `first` still opens its initial-node member.

- **An accepted signal message binds as an occurrence of its signal.** `send Telemetry(frames =
  3.0) via antenna` matched an `accept t : Telemetry` but bound nothing: the message carried its
  arguments, and the accept only understood a single carried value. The accepted name is now
  bound to an occurrence of the signal, its features set from the send's named and positional
  arguments — so a transition effect reads `t.frames`. A message carrying neither a value nor a
  signal is still `ErrNoValue`.

- **A send from inside a nested part finds its port on the enclosing part.** A beacon running in
  a timeslice of the probe sending `via antenna` — the probe's port, not the timeslice's — was
  unroutable: owner routing started at the sender itself, so the probe's connector was never
  consulted. Routing now starts at the object actually holding the resolved `via` port, so the
  enclosing part's connectors carry the message.

- **A connector end through a multi-valued feature fans out.** A send over
  `connect console.command to units.command` where `part units : Unit[2]` was
  `ErrUnroutableSend`: an end reached through a multi-valued feature resolved to no object. Such
  an end now denotes every element the feature holds (KerML 1.0 §7.3.4.6), so one send delivers
  one message per element, each on that element's own identity — of addressing generally, not
  only the owner-level route. The squad site of
  [`examples/disposal-team-demo`](examples/disposal-team-demo/README.md) shows it.

- **Message signals match by semantic identity, not short name.** Two same-named item or signal
  definitions in different packages were conflated, and an accept of a supertype did not take a
  message of a subtype. A message now carries its resolved signal symbol, and an accept takes a
  message whose signal conforms to the type it names — qualified identity plus subtype
  conformance through the semantic model.

- **`send x via p` routes by what `p` resolves to, not the name written.** Connector-end
  matching compared the written name, so a port a behavior declares under the name of one of the
  performer's connected ports did not divert the route. The `via` target is now resolved once at
  lowering with the usual scope-aware shadowing, so the behavior-local port is used and the outer
  connector receives nothing.

- **A part's own connector into a part it holds delegates inward.** `connect command to
  unit.command` delivered the message on the sender's own identity under the port path
  `unit.command`, so the nested part never accepted it. Every receiving end of a route is now
  resolved to the object holding the port it names, so the copy is held to the nested part's
  identity, as it already was for a connector an owner declares between two siblings. An end whose
  part holds no object this run has nothing behind it, so such a send is now `ErrUnroutableSend`
  rather than a message posted to a port path no consumer reads.

- **A send now crosses a connector its owner declared.** A part's port joined by its owner to a
  sibling's port reached nothing: routing consulted only the connections of the behavior and of
  the sending object, so a console commanding a unit over the connector their site declares
  reported `send reaches no receiving port`. Deliveries now also follow the connections of every
  object holding the sender, and arrive on the peer object's own identity.

- **An item object can be sent.** `send cmd via p`, where `cmd` is an `item cmd : Command { … }`,
  reported `message of kind instance has no signal type`: a message took its type from a scalar
  value only, so an object had none. An object's message is typed by the definition it
  materializes, which is the type an accept of it names.

- **A bound subject now carries the subject's type.** `requirement r : Req { subject truck = loaded; }`
  redefines the definition's `subject truck : Truck`, but the redefinition was not among the
  usage's supertypes, so `truck.payload` named no member and `%check` refused the requirement.
  Implicit role redefinitions are direct supertypes, so a subject or objective bound in a usage
  reads the members of the role it redefines.

- **`isReference` and `isComposite` are derived from what a usage declares.** Reflective reads
  reported the flags a declaration carried literally, so a query over metaclass features answered
  from the notation rather than from the reference semantics the usage has; every
  declaration-backed metaclass modifier flag is now reflected the same way, and a metaclass feature
  is read through metaclass conformance.

- **The REPL evaluates a bare expression.** A line that was neither a command nor a declaration was
  echoed rather than evaluated; it is now evaluated, a materialization failure is reported as one,
  qualified suggestions are ranked ahead of unqualified ones, and warnings are printed before the
  load lines they belong to rather than after them.

- **The orthogonal-regions demo terminates.** Its regions completed only on each other's completion,
  so running it livelocked; the demo now uses timed transitions, which is what the notation offers
  for a region that must advance on its own.

- **A rendered document set cannot be lost to a failed write.** Staged documents and their
  directories are synced before the set is committed, a destination that already exists is replaced
  portably rather than removed first, a case-aliased or colliding destination is rejected, and a
  rollback restores a backup even where the failed replacement had removed its destination.

## 0.4.1 — 2026-08-30

Release 0.4.1 is about what the tools *say* about a model. Every surface that names a declaration —
a rendering, the REPL's echo and search, a runtime diagnostic, LSP hover and completion — printed the
classification the implementation keeps internally rather than the notation the file was written in,
so a datatype read as an attribute and a KerML classifier grew a `def` it never had. The written form
now has one source, and those surfaces all read from it; hover and completion documentation render as
Markdown for a client that advertises it, and as plain text for one that does not.

One import path moves: the public Go API is now `github.com/Open-MBEE/OpenSysML/client/opensysml`.
The API itself is unchanged, but Go has no import alias, so **a Go consumer must edit the import
line** — the one change in this release that a user has to make. No model that validated under
0.4.0 stops validating.

Behind those, native document queries gain a compiled planning layer (planning only: nothing
executes or renders yet), the Python and Rust clients move under `clients/` beside the Java and Node
ones, and the SonarCloud gate is measuring the project it is meant to — every language's tests now
count toward coverage, the Java sources are analyzed with types, and the bug and vulnerability
backlog is empty.

### Changed

- **Every language's tests now count toward measured coverage.** The scan read only the Go
  profile, so the Java, Python and TypeScript suites — all of them passing in CI — had every line
  they cover counted as uncovered. Each client job now writes a report (JaCoCo, `pytest-cov`, `c8`)
  and the scan waits on those jobs. The Go profile is written with `-coverpkg`, which credits a
  package for the code it exercises elsewhere: `internal/core/ast/dump.go` measured 21% while the
  parser's golden tests ran 90% of it. `make python-coverage` and `make node-coverage` write the
  reports locally.

- **The public Go API moved to `github.com/Open-MBEE/OpenSysML/client/opensysml`**, from
  `.../pkg/opensysml` — the top-level `client/` directory Go projects conventionally publish a
  client library from. Go has no import alias, so this breaks every consumer's import path; update
  the import, nothing else. The API is unchanged and still ships with the core `v*` tags.

- **The Python and Rust clients live under `clients/`**, beside the Java and Node ones, rather than
  at the repository root. Every path that named them moves with them: the CI jobs and publish
  pipelines, the changed-area filters, the Makefile targets, the buf output paths, the analysis
  inclusions and the documentation. Neither published package changes name, version or contents.

### Added

- **Native document queries now have a compiled planning layer.** Query definitions specialize the
  bundled `DocumentQueries::Query` vocabulary, retain typed parameter/result metadata and source
  provenance, and may invoke other named queries with explicit named bindings. Planning produces an
  immutable dependency-ordered program and reports malformed definitions, unknown operations, bad
  bindings, positional query composition, and complete direct or indirect composition cycles as
  typed validation diagnostics. Execution and document rendering are not part of this release.

- **Hover renders as Markdown when the editor supports it**: the signature is a fenced `sysml`
  block and the doc comment reads as prose. A client that does not advertise Markdown still gets
  the plain text it did before.

### Fixed

- **An element is named the way its notation writes it**, on every surface that names one — a view
  rendering, the REPL's echo and search, a runtime diagnostic, and LSP hover and completion. These
  printed the internal classification of a declaration instead, so a `datatype` read as an attribute
  and a KerML classifier was given a `def` suffix it never had. A short name that is not a valid
  identifier is quoted as the notation requires, and emphasis a doc comment was authored with
  survives into the rendered documentation.

- **Hover keeps what the file says.** Each leading comment's delimiters are stripped on its own, a
  doc comment keeps the line breaks it was written with, a named relationship's prefix stays out of
  its signature, and a relationship is named by the keyword its name follows.

- **Both operands of `?` may be conditional expressions.** The parser accepted one only in the else
  branch, though `KerMLExpressions.xtext` makes both owned expressions and limits only the condition
  to a null-coalescing expression.

- **The Connect server's shutdown no longer runs on an already-cancelled context.** It derived its
  30-second grace period from `context.Background()`, dropping the request context's values; it now
  derives it from a cancellation-free copy of the server's own context.

- **The pilot validators normalize EMF object references with a linearly-scanned pattern.** Theirs
  began with a broad character class, so a message without a reference was rescanned from every
  position. It anchors on the `@` that starts the identity hash instead, and the qualified name
  before it is left in place rather than rewritten unchanged.

- **A failing Java, Node or public-Go-API job now fails the PR gate.** GitHub's `Build and test`
  check exists to give path-filtered jobs one stable required name, but its `needs` listed neither
  client test job nor the `client/opensysml` conformance run, so all three reported green through
  it. It waits on every job now, and `node-test` waits on `build` — the job that uploads the binary
  it downloads — rather than on the gate itself, which had chained it behind the whole workflow.

- **The scan analyzes the Java client with types and the Python client against its own version
  range.** It warned about missing `sonar.java.binaries`/`sonar.java.libraries` and fell back to a
  syntactic analysis of the Java sources, and assumed every Python 3 version, which drops the rules
  that depend on one. `java-test` now persists each module's compiled classes and its dependency
  jars, `sonar-project.properties` names them, and `sonar.python.version` names the `>=3.10`
  through 3.13 range `clients/python/pyproject.toml` declares.

- **The behavioral-bodies demo's state machines can be run.** Each of its four machines declared
  substates but no transition out of its entry action, so `%state PhaseC::Running` (and the other
  three) failed with `no initial state found`; the Boolean features its guards and triggers read had
  no value either. Each machine now names the state it starts in and those features are initialized,
  so all four start and step. No diagnostic moves on either side.

- **The demos are written in standard notation wherever one exists.** `then done;` in place of a
  standalone `done;`, `entry`/`do`/`exit <action>` and named effect actions in state bodies,
  `accept when <event>` triggers, `assert constraint` for an analysis case's own conditions, and the
  objective subject the trade-study library redefines. The pseudostate notation, which no SysML v2
  grammar has a production for, is now written in `examples/pseudostates-demo.sysml` alone and stays
  supported everywhere. Every demo's output is unchanged; the pilot differential baseline and the
  figures quoted from it move.

- **The semantic-layer demo declares its packages with `package`.** Its three `namespace`
  declarations are KerML notation — the SysML grammar has no `namespace` production — so the pinned
  pilot could not parse the file and the non-standard-notation pass warned on each. The file now
  agrees on both sides, which moves the pilot differential baseline and the figures quoted from it.

- **The header's Community Wiki link points at the wiki's landing page**, rather than at the wiki
  root, which lands on whatever page GitHub considers first.

- **A deserialized `ModelException` cannot claim an unbounded diagnostic count**, so a hostile or
  corrupt stream no longer has the Java client allocate for one.

### Project

- **The SonarCloud bug and vulnerability backlog is cleared.** A sort compares by an explicit
  code-unit comparator rather than an implicit collation, the Java client keeps its `Optional` and
  queue returns, workflow permissions are scoped per job, the CI Python installs are pinned, and the
  VS Code extension's webview ignores a message from any other origin. `sonar-project.properties`
  states, with its reason, each rule whose subject in one file is a developer command's documented
  behavior.

- **The maintainability findings behind it are cleared too**, across Go, Java, Python, Rust,
  TypeScript and the shell scripts: parameter lists over seven entries became option structs,
  switches over thirty cases dispatch through kind-scoped helpers or lookup tables, duplicated
  literals are hoisted, and the coverage script validates that the profile path it is given stays
  in the working tree and reports a symlink loop rather than a traceback. Behavior is unchanged
  throughout; the generated Python gRPC stubs and the cognitive complexity of Go test files are
  excluded from analysis, since neither is code a reviewer edits.

## 0.4.0 — 2026-08-28

Release 0.4.0 is about what a model *writes*. An `assign` used to put any value into any feature —
a `String` into a `Real` attribute, a length into a duration — statically and at run time; a write
now answers to the target feature's declared type, multiplicity and, where the target is a quantity,
its dimension, on every path that stores a value. **A model that relied on an unchecked write no
longer validates**, which is why this is a minor rather than a patch.

The runtime also learned the structure it was flattening away: a typed state usage materializes the
content of the definition typing it, a nested action node runs the flow it owns, an assignment target
may be a feature chain, and a performed action's features live on the performance occurrence that
holds them rather than on whatever like-named feature the performer happened to have. A behavior body
now resolves a name where the name is written rather than reaching into the performer for it.

Beyond execution, a view specializing `SequenceView` renders as a sequence diagram, a `Real` prints
as the shortest decimal that reads back as the same value, and the pinned OMG pilot implementation
moves to release `2026-07` (`jupyter-sysml-kernel` 0.61.0) with every oracle baseline re-recorded
against it.

### Changed

- **A write answers to its target feature's declared type and multiplicity.** An `assign` wrote any
  value into any feature: a `Real` attribute accepted a `String`, statically and at run time. KerML's
  FeatureWritePerformance "assigns the values of a feature on an occurrence to the given
  replacementValues", so those values are values of that feature — the rule already applied to an
  initial value. The type pass now walks every body an `assign` may stand in and checks the written
  value with the initial-value rule resolved against the target's declaration, and the runtime checks
  the same before storing, on every write path. A rejected write leaves the feature as it was.
  - **An output a declaration binds is checked too**, so a body-less `out a : Integer = n` no longer
    answers with whatever an untyped input carried: the rule holds however an output is given its
    value.
  - **A written decimal conforms to `Rational`**, as the type tier already read one. The two tiers
    disagreed, so a decimal written to a `Rational` feature validated clean and then failed at run
    time.

- **A bound or written quantity is judged by the dimension its target declares.** A feature declared
  with a quantity value type refuses a quantity measured in another dimension — statically where the
  value's dimension is determined, and at run time on every write path.

- **The pinned OMG pilot implementation is now release `2026-07` (`jupyter-sysml-kernel` 0.61.0)**,
  with the reference validators, the vendored standard library, the pinned corpora, grammars and
  Xpect suites, and every oracle baseline re-recorded at that pin. Two notations the new grammar
  admits now parse: a metadata usage that declares its own name or none at all
  (`@ m : Security;`, `@ : Security;`, `@ m typed by Security;`) and a constraint reference
  carrying a multiplicity (`assume c [0..*];`). The errata overlay drops the redefinition
  correction the published corpus now makes itself.

- **Why a listed view is not drawable is now written under the diagram** rather than only in the
  picker entry's tooltip, so a `geometry` or `textual` view says what it is that cannot be drawn
  without the reason having to be hovered for.

- **A Real prints as the shortest decimal that reads back as the same value**, on every surface
  that renders one — an evaluation result, a feature value listing, a quantity's magnitude, an
  execution trace and the simulation clock. Values were rendered to two decimal places, which
  reported a nonzero magnitude as zero (`0.0001` printed `0.00`) and rounded away precision the
  evaluation had kept (`1.0 / 3.0` printed `0.33`, `123456789.987654` printed `123456789.99`),
  and disagreed between surfaces. A whole Real keeps its `.0` so it is not mistaken for an
  Integer. Arithmetic is unchanged: the stored value was never rounded.

### Added

- **A typed state usage inherits the content of the definition typing it.** The definition's
  substates, initial transition, entry/do/exit behaviors, transitions, deferred events and
  attributes are materialized per usage rather than shared, including inside a parallel body.
  Recursive typing, and content lowering cannot represent, are typed errors rather than silence.

- **A nested action node runs the flow it owns.** A nested node's own members are its
  subperformances: lowering carries them as a subgraph and the executor performs that flow before
  the node completes. An action usage stating no body of its own resolves to the action definition
  typing it, so `%action` on a `perform` usage runs.

- **An assignment target may be a feature chain.** Lowering carries the whole walk and the runtime
  writes the feature on the object the chain reaches, resolved in the statement's own scope.

- **A view specializing `StandardViewDefinitions::SequenceView` (or `sv`) renders as a sequence
  diagram**, at the prompt (`%render`), from the command line (`sysml -render`) and over the LSP, in
  the text form and as a Mermaid `sequenceDiagram`. The occurrences an interaction declares in its
  body are its lifelines, a `message`/`flow` usage is a directed message between the lifelines its
  ends' events belong to, and the successions between those events order the messages — a cycle
  among them is reported and declaration order stands. What a sequence diagram cannot show — an
  exposed element that holds no occurrence, an undirected `connect`, a message stating no ends or
  attaching to something the view does not expose — is reported rather than dropped. A `geometry`
  view remains recognized but not drawn.

- **The `opensysml/views` response now reports supported pseudo-view specs**, so clients can offer
  newly supported rendering kinds without maintaining a second list.

### Fixed

- **A behavior body resolves a name where the name is written**, rather than reaching into the
  object performing it for any like-named feature — which name resolution does not admit. A
  performer feature is read and written only where the name resolves to it, and `this` inside an
  owned performance denotes the owning object.

- **A performed action's features are written to its performance occurrence.** A performed action's
  declared attributes and out parameters are features of the action performance the `perform` usage
  holds, so they are initialized from that occurrence's slots and written through it.

- **A state's attributes reach the occurrence exhibiting it**, so an exhibited state and the
  occurrence behind it no longer hold divergent data.

- **A send nested in a flow is routed by every flow around it.** A send saw only the action's own
  connectors and those of the flow it sat in, so a connector declared by an intermediate flow could
  not route it; each activation now carries the connectors of every enclosing flow.

- **An inherited transition retargets to a redeclared substate**, rather than to the substate the
  supertype declared.

- **A machine's own supertype is no longer read as recursive typing**, which rejected a state
  machine that specialized another.

- **A chained assignment binds no output of a calculation**: the chain's last segment was counted as
  the calculation computing an output of that name.

- **Clicking a sequence participant now reveals its declaration, and the cursor highlights it**
  using Mermaid's participant data attributes.

- **The site's OpenMBEE links point at the host that serves HTTPS.**

### Project

- **The end-to-end example is one model, driven three ways.** A bomb-disposal robot whose structure,
  calculations, fork/join and nested flows, hierarchical state machine, solver cases and views are
  the same model throughout, with a walkthrough of the commands that drive it from the CLI, the REPL
  and the Python client. A nested action node no longer reports the token-flow limitation when a
  view renders it.

- **The dimensional defect in the pilot's `Dynamics.sysml` is a declared erratum**, so the corrected
  copy is what the oracles re-run over and the published corpus is still never edited.

- The documentation site states the Open-MBEE and NumFOCUS affiliation in its footer, and the
  header's wiki link is labelled Community Wiki.

- **The Python client is unchanged in this release**, so no `opensysml` version is published with
  it: the client published as 0.3.2 installs a v0.4.0 `sysml-grpc` by taking the asset's digest from
  the release's signed `SHA256SUMS.txt`, which is what an unpinned release's verification is for.
  Pinning v0.4.0's digests in `python/opensysml/binary.py` and publishing 0.3.3 remains optional,
  and is worth doing only for callers still on `opensysml` 0.3.0, which predates that path and
  installs a release it pins no digest for only under `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`.

## 0.3.1 — 2026-08-27

A performance patch. Loading a model costs less of everything and answers the same: on a
12,000-element synthetic model, 4.62M allocations rather than 7.70M, 503.6 MiB allocated rather
than 744.9 MiB, 272 MiB peak resident rather than 353 MiB, and 0.92s rather than 1.14s. Nothing
about the language, the diagnostics or any API changed — the 895 bundled SysML and KerML files
report byte-identical diagnostics and exit status, and the 749 of them a conversion accepts produce
byte-identical `-convert sysml` and `-convert ttl` output, before and after.

### Performance

- **Whitespace is no longer recorded as trivia.** A node's leading trivia holds the comments and
  notes a consumer reads (doc-comment hover, the REPL's declaration printing) and nothing else,
  which is what dominated allocation on a large parse.
- **A span's text is served from one cached whole-file string** instead of copying the bytes per
  span, so the repeated reads name resolution and validation make cost nothing after the first.
- **A fully qualified name is compared without being built.** Checking whether a symbol *is*
  `Base::DataValue` walks the scope chain against the string rather than constructing the name to
  throw away; constructing one, where a caller genuinely needs the string, sizes its buffer once.
- **The inherited-name conflict pass reads the memoized base-member maps directly** rather than
  merging them per declaration, and merges only where a declaration has more than one base.
- **A scope's children are indexed lazily**, by map only where a scope is large enough for the map
  to pay for itself, and by scan below that.

### Project

- The release-gate counts on `README.md`, [the roadmap](docs/project/roadmap.md),
  [spec compliance](docs/project/spec-compliance.md) and
  [training examples](docs/project/training-examples.md) are recounted together against a real
  `go test -race -count=1 -v ./...` run: 8,361 tests and subtests, 380 execution conformance cases,
  118 golden traces, 256 runtime robustness cases, 146 golden ASTs and 261 negative parser subtests.
  The skip list is stated by what each skip wants, since five tests it named as skipping now pass.

## 0.3.0 — 2026-08-26

Release 0.3.0 spends itself on a single question: what does this implementation accept that the
specification does not? Since 0.2.0 the answer was a warning — thirteen OpenSysML-only spellings were
read, reported as non-standard notation, and executed anyway. That register is now closed. Each of the
thirteen is a parse error, the warning for it is gone, and the standard spelling that replaces it is
documented, migrated throughout this repository's models and guide transcripts, and covered by tests.
**A model relying on any of them no longer parses**, which is the point: an analyzer that quietly
accepts what no production admits teaches its users a notation no other tool reads. The version is a
minor rather than a patch for that reason.

The state machine's completion semantics were re-founded in the process: a machine completes on a
transition to the standard library's `done` rather than on a marker keyword, computed while lowering
and carried in the graph the runtime executes, so completion is a property of the machine rather than
of its syntax. Beyond notation, converted RDF states element ids and ownership as the abstract syntax
does, so a SysML v2 API service can address a graph this tool produced; `sysml-grpc` speaks the
Connect protocol by default, on the same port as gRPC and gRPC-Web; the Python client starts a private
service of its own instead of adopting whichever one is listening; and what a real Flexo MMS stack
does with our Turtle is now a recorded per-property measurement rather than a claim.

The service also stops being reachable from one language only. Four client surfaces ship with this
release — a public Go API that answers in process, and Node/TypeScript, Java and Rust clients that
speak the Connect protocol — and each of the four runs the same language-independent conformance
scenarios through its own public API rather than through generated stubs, so "it speaks to
`sysml-grpc`" is a measured claim in every one of them. None of the four is published yet.

Against the pinned OMG pilot (`2026-05`, jupyter-sysml-kernel 0.60.1), 328 of 355 files agree
diagnostic-by-diagnostic, there is no declared `errors` row in the reference's own suites we are
silent on, and 120 of 120 authored invalid models are rejected by both implementations.

### The notation this release no longer accepts, in one table

Each spelling was an OpenSysML extension warned as `nonstandard-notation`; each is now a parse error,
and its warning is gone. None of the words involved is reserved by the pinned grammars, so
`state final;`, `attribute initial : Boolean;` and `region` as a name keep working. Every row is
described in full below.

| No longer accepted | Write instead |
|---|---|
| `bind result = x * 2.0;` — an expression as a binding's right end | `out result : Real = x * 2.0;`, or bind to a feature holding the result |
| `assert <expr>;` / `assume <expr>;` in a constraint body | the bare condition: `constraint MassBudget { total <= limit }` |
| `assume <expr>;` / `require <expr>;` in a requirement body | a wrapped constraint: `require constraint { power > 0 }` |
| `return <expr>;` in a calculation body | the body's trailing expression: `calc def Add { in x; in y; x + y }` |
| `final <state>;` | `transition first running accept Stop then done;` |
| `initial <state>;` | `entry; then <state>;` |
| `transition <source> to <target>;` | `transition first <source> then <target>;` |
| `region <name> { … }` | mark the owning state's body `parallel`, one `state` substate per region |
| `initial <name>;` as an action node | `first <name> [then <target>];` |
| `final [<name>];` as an action node | `done;`, or `then done;` as a succession target |
| `decision <name>;` | `decide <name>;` |
| `done <name>;` — a named action final node | `done;` |
| `then <source> <target>;`, and a member-leading `<source> then <target>;` | `succession first <source> then <target>;` |

### A name a library supertype already supplies is reported

`state start;` inside a state, or `attribute portions;` inside a part definition, declares a
member indistinguishable from one the declaration inherits from the standard library —
`StatePerformances::StateAction::start`, `Occurrence::portions` — and is now a warning, as the
reference implementation and KerML 8.2.4 have it. Only a name two library supertypes each
supplied was reported before. A member that redefines or subsets the inherited feature is that
feature and stays silent, as do the implicit redefinitions: a positional behavior parameter, the
subject, actors, stakeholders and objective of a case or requirement, and the assignments in a
metadata usage body. Three diagnostics of the reference we did not report become agreements.

### Succession endpoints are checked against their enclosing body

State-machine succession and transition spellings now report a resolved endpoint
that is not a state or pseudostate, using the existing `not-a-vertex` diagnostic.
Action bodies report a resolved endpoint that is not an action node with the
`endpoint-not-a-node` diagnostic before lowering. Positional, implied, unresolved,
and flow endpoints retain their existing behavior. Inherited action nodes are
also collected lazily when named by an endpoint and lowered into executable
edges. The same routing removes a false unresolved-reference diagnostic for
`succession` and `then` ends naming nested or region-local vertices, covered by
`internal/core/parser/testdata/parse/state_history_entry_exit.sysml`,
`internal/core/runtime/testdata/conformance/state_deep_history.sysml`, and
`internal/core/runtime/testdata/conformance/state_shallow_history.sysml`.
Feature-chain endpoints rooted in a part usage are accepted during validation,
while execution still reports the existing lowering error because invoking an
action through that feature chain is not yet supported.

### A Java client, for a JVM host application

`clients/java/` adds `io.github.open-mbee:opensysml-client`, a Java client for the
`sysml-grpc` service, aimed at the JVM tools this ecosystem is full of: an Eclipse-based
tool, a Cameo plugin, a web service. It parses a file or inline content, evaluates an
expression, looks up a symbol, instantiates a definition and negotiates capabilities.

- **A JDK 17 baseline and one compile dependency.** `protobuf-java`, and nothing else by
  default: the transport is `java.net.http.HttpClient` speaking the Connect protocol with
  protobuf bodies, so there is no gRPC, no Netty and no `tcnative` to conflict with a host
  application's own. `Encoding.JSON` is the debugging affordance and needs the optional
  `protobuf-java-util`. 17 is what an Eclipse 2023-03 or Spring Boot 3 host can offer.
- **One private child per classloader**, the JVM analogue of the Python client's one per
  interpreter, so the copy a plugin loaded and the copy a web application loaded each own a
  service and share its parse cache within themselves; `isolatedService(true)` starts one for
  a single connection where a shared cache across tenants is not acceptable. A service the
  client did not start is explicit opt-in and is never stopped.
- **No orphans, by the same mechanism**: the client holds the write end of the child's stdin
  pipe and never writes to it, and the kernel closes that pipe even on `kill -9` of the JVM,
  which a shutdown hook or `ProcessHandle.onExit` does not survive. A test kills a child JVM
  with `SIGKILL` and asserts the service is gone.
- **The binary is not downloaded.** It is resolved from an explicit path,
  `$OPENSYSML_GRPC_BINARY`, `~/.opensysml/bin` or `PATH`, and a caller-pinned SHA-256 is
  verified before it is executed.
- **The conformance suite runs through the public API**, over both Connect encodings: 25 of
  59 scenarios pass and 34 are skipped as RPCs v1 does not cover (the edit API, RDF
  conversion, verification, behaviour execution and queries), with the report in the shape
  `cmd/conformance` writes. `-mutate` corrupts every answer to prove the comparison is not
  vacuous.
- Java stubs are generated by `buf` from a plugin entry in the root `buf.gen.yaml` with the
  version pinned inline, regenerated by `make proto`, and a `java-test` job in both CircleCI and
  the pull-request GitHub Actions workflow runs the tests and the suite, while the Go job in each
  checks the committed stubs are what `buf` generates.

Nothing is published. The build produces a signable, locally-installable artifact with
sources and javadoc jars, and [releasing.md](docs/project/releasing.md) says what a
maintainer must obtain — a verified namespace, a GPG key, Central portal tokens — before a
first upload.

### A public Go API, in process by default

`pkg/opensysml` is the Go surface of this engine: parse, symbol lookup, evaluation and
instantiation from Go code, with the parser, semantic engine and runtime the importing binary
already links. It is not a wrapper around a port.

- **`New` answers in process**, calling the same service implementation the wire transports serve,
  so the semantics are the service's: the same content-addressed parse cache and model hashes, the
  same capability list, the same in-band failures and the same runtime budgets. No child process is
  ever spawned — a Go process wanting the engine in process has no use for one. `Dial` is for a
  service someone else runs, addressed explicitly, over Connect with protobuf bodies.
- **Two failure modes, and the difference is part of the API**: a refused call is a `*StatusError`
  carrying the canonical gRPC status code (`errors.Is(err, opensysml.CodeNotFound)`), an answer that
  reports a failure is a `*FailureError`, and an engine panic arrives as `CodeInternal` rather than
  unwinding into the caller. Syntax errors are neither: parsing broken source succeeds and the
  diagnostics are on the model, in the shape the LSP and the wire report.
- **Everything returned is a copy the caller owns.** In process there is no serialization boundary
  to enforce that, so it is a documented promise with tests behind it: no returned value aliases
  engine state.
- The conformance suite gained `pkg` and `pkg-connect` protocols, so the covered scenarios run
  through this API both in process and against a started service.

### A Rust client, blocking by default

`rust/opensysml` is a blocking Rust client for the service, with no asynchronous runtime anywhere in
its default dependency tree: all fifteen RPCs are unary and the usual consumer talks to a local
child that answers in milliseconds, so a private `tokio::Runtime` inside a library would tax every
consumer for nothing. A test calls the client from inside `Runtime::block_on` to pin that it is safe
to use from an async program anyway. Rust 1.83 is the minimum supported version.

- **The lifecycle is the one the other clients use**: `Connection::private()` starts one child per
  process and shares its parse cache, `Connection::external` and `$OPENSYSML_SERVICE` are explicit
  opt-in and are never stopped by the client, `Drop` cleans up deterministically, and the child's
  stdin pipe is the guarantee that survives `SIGKILL` and `process::exit` — pinned by a test.
- **No binary is downloaded**: the service is resolved from `$OPENSYSML_GRPC_BINARY`,
  `~/.opensysml/bin` or `PATH`, and the client does not pretend to pin a child version.
- **The conformance runner reads answers through the typed API**, not the stubs, and skips only the
  RPCs the v1 surface does not cover plus the one request that surface cannot express; any other
  skip names its missing capability and fails the run. Publishing to crates.io is a maintainer
  action and CI never does it.

### A request for an unavailable capability is refused

`sysml-grpc` answers a request that asks for a capability it does not have with `UNIMPLEMENTED`,
naming the capability, instead of quietly doing something else; capabilities that only describe how
a response is populated omit those fields as before. The conformance gate runs the default service
and a test-only configuration with capabilities withheld, so both halves of the contract are
exercised, and the Python client turns a service-side refusal into `MissingCapabilityError` while
keeping the original error as its cause.

### RDF output states element ids and ownership

A converted graph now carries the two things a SysML v2 API service needs to address it.
Every element states `sysml:elementId` — exactly the id its own IRI ends in, so the triple
and an id derived from the IRI cannot disagree — and containment is stated as the abstract
syntax does: `sysml:owner` and `sysml:owningRelatedElement` as element references, with a
materialized `OwningMembership`, or a `FeatureMembership` where a type owns a feature,
between owner and member. Each membership is an element of its own with a deterministic IRI,
its own `sysml:elementId`, and the member and owner wiring a client walking from a root
follows; a relationship a namespace declares, such as an import, owns its member directly.
Visibility moves to the membership that declares it. A document now has one root instead of
every element reading as one.

The nodes of an expression graph are addressable too: a node's IRI held a `.` and the name
of the position it sits in, which an API service restricting ids to `[a-zA-Z0-9_-]+` refuses
outright, so the position is now joined with `_p` and encoded the way an element id is, and
the node states that id in `sysml:elementId`. A node is still not a model element.

`.ttl` output changes shape accordingly, and every graph regenerates. Reading is
backward compatible: a graph carrying only the previous compact `sysml:owningNamespace`
shape still converts unchanged, and that property is still written.
[reference/rdf-mapping.md](docs/reference/rdf-mapping.md) documents the ownership shape.
Loading a graph into a live triplestore and reading it back through the API is still not
demonstrated, and collection-valued properties still carry no JSON annotation.

### An expression as a binding's right end is no longer accepted

**Breaking change.** `bind <feature> = <expression>;` — a binding whose right side is an
expression rather than a feature, such as `bind result = x * 2.0;` — was an OpenSysML
extension no SysML v2 production admits, warned as `nonstandard-notation`. It is now a parse
error, and the warning for it is gone: a binding relates two connector ends, so each side must
name a feature. Write the expression as the feature's value instead
(`out result : Real = x * 2.0;`), or, where the feature is declared elsewhere, declare a
feature holding the result and bind to it (`attribute b2 = a + 1;` then `binding bind b = b2;`).
Standard bindings — `bind a = b;`, including qualified, chained and indexed ends — are
unchanged.

### A keyworded inline condition is no longer accepted

**Breaking change.** `assert <expression>;` and `assume <expression>;` in a constraint body, and
`assume <expression>;` and `require <expression>;` in a requirement-style body, were an OpenSysML
extension no SysML v2 production admits, warned as `nonstandard-notation`. They are now a parse
error, and the warning for them is gone.

- In a constraint body, write the condition on its own: `constraint MassBudget { total <= limit }`
  instead of `constraint MassBudget { assert total <= limit; }`.
- In a requirement body, which admits no keyword-less condition, wrap it in a constraint:
  `requirement R { require constraint { power > 0 } }` instead of
  `requirement R { require power > 0; }`.
- A negated condition keeps its truth value in the expression: `assert not (x < 0);` becomes
  `not (x < 0)`.
- What the keywords still state is unchanged: `assert [not] <reference>;`,
  `assert constraint { … }`, `assert constraint C;`, `assume`/`require <reference>;` and
  `assume`/`require constraint [name] { … }`. The separate warning for `assume`/`require` used
  outside a requirement body is unchanged too.

### The state final marker is removed; a machine completes on a transition to `done`

**Breaking change** for models that used it. `final <state>;` in a state body was an
OpenSysML-only notation no SysML v2 production admits — `StateBodyItem` has no such member and no
`final` literal appears in the pinned grammars — warned as `nonstandard-notation`. It is now a
parse error, and the warning for it is gone.

What replaces it is the completion the standard library already gives every state: a machine
completes when a transition reaches `done`.

```sysml
state monitor {
    entry; then running;
    state running;
    transition first running accept Stop then done;   // was: final stopped;
}
```

Entering completion runs the exit actions of the states it leaves and reports the machine
completed, exactly as entering a marked state did; an orthogonal machine completes only once every
concurrent region has reached `done` — the machine's own regions and those of a composite state
alike, so a region completing leaves its siblings running. Completion is
now computed while lowering and carried in the state graph the runtime executes, so it is a property
of the machine rather than of its syntax.

Completion is **stated, not inferred**: a state with no outgoing transition does not complete on
its own, because an ancestor or cross-region transition may still leave it. A machine naming a state
of its own `done` reaches that state, unchanged. `final` is reserved by no pinned grammar, so it
still names a state or a feature (`state final;`, `attribute final : Boolean;`); a state body that
writes `final <state>;` is reported as the parse error it now is.

### The state initial marker and the `to` transition spelling are removed

**Breaking change** for models that used them. Both were OpenSysML-only aliases of notation the
pinned grammars already spell, so the aliases and their `nonstandard-notation` warnings are gone
rather than kept:

- `initial <state>;` → declare the state and start the body's entry succession at it:
  `entry; then <state>;` (`SysML.xtext:1766` `EntryActionMember`, followed by `EntryTransitionMember`
  at `SysML.xtext:1796`). Inside a `region`, the entry succession designates that region's start, as
  the marker did. It lowers to the same `StateGraph` initial state, so no execution result moves.
- `transition <source> to <target>;` → `transition first <source> then <target>;`
  (`SysML.xtext:1854` `TransitionUsage`, whose target is introduced by `then`). Naming the
  transition, guards, triggers and effects are unchanged: `transition t first a if g then b;`. The
  source may also be written without `first`, as the grammar allows.

`initial` is not reserved by the pinned grammars, so it still names a feature
(`attribute initial : Boolean;`, `state initial;`). A state body that writes `initial <state>;` or
`transition <source> to <target>;` is reported as the parse error it now is — in particular
`transition a to b;` does not quietly become a transition between other ends.

The `final <state>;` marker is removed too, described below.

### The orthogonal-region member is no longer accepted

**Breaking change.** `region <name> { … }` in a state body was an OpenSysML extension no SysML v2
production admits, warned as `nonstandard-notation`. It is now a parse error, and the warning for it
is gone. Mark the owning state's body `parallel` and write one state substate per region:

```sysml
state working parallel {
    state left  { entry; then building; state building; }
    state right { entry; then checking; state checking; }
}
```

Region order becomes substate declaration order and each region keeps its name, so entry, exit,
event broadcast, history and completion behave as before. A state whose body is `parallel` may still
own directly what a region body could be reached from — its behaviors, `defer`, the pseudostates its
regions branch through and the edges between them — but every *state* substate of a parallel body is
a region, so a body that mixed one region with ordinary sequential substates has to put the region
set in a state of its own. `region` is now an ordinary name.

### Succession shorthand spellings removed

**Breaking change.** OpenSysML no longer accepts named action final nodes (`done <name>;`),
two-name succession shorthand (`then <source> <target>;`), or state-body member-leading
successions (`<source> then <target>;`). Write `done;` for an action final node,
`succession first <source> then <target>;` for an explicit succession, and use the same
standard succession form in state bodies.

### The action nodes spelled `initial`, `final` and `decision` are removed

**Breaking change** for models that used them. Each was an OpenSysML-only alias of a node the
standard notation already spells, so the alias and its `nonstandard-notation` warning are gone
rather than kept:

- `initial <name> [then <target>];` in an action body → write `first <name> [then <target>];`
  (`SysML.xtext:1385`).
- `final [<name>];` as an action node → write `done;` for the anonymous final node, or
  `then done;` when naming the library feature `Actions::Action::done` as a succession target.
- `decision <name>;` → write `decide <name>;` (`SysML.xtext:1672`).

None of the three words is reserved by the pinned grammars, so each still names a feature
(`attribute final : Boolean;`, `action initial;`). An action body that writes `initial <name>;`,
`final <name>;` or `decision <name>;` is reported as the parse error it now is. A bare
`then final;` is a **succession target**, not a node, so it is read as a reference to a member named
`final`: where nothing declares one, `sysml -validate` reports an unresolved reference at the name —
the same as any other undefined succession target (see below). The **state** markers
`final <state>;` and `initial <state>;` are removed as well, both described above. Named
action final syntax `done <name>;`
remains rejected as described above.

### An undefined succession or transition endpoint is reported at validation time

A succession or transition end naming a member nothing declares — `succession first start then zzz;`,
the guarded `succession first a if c then zzz;`, `then zzz;`, a decision's `if c then zzz;` and
`else zzz;`, and `transition first idle then zzz;` — is now an `unresolved` error of the
name-resolution tier, reported at the name itself. It used to analyse clean and fail only when the
action or state machine was executed, so a model could pass `sysml -validate` and still be
unrunnable. The lowering errors remain as the last check.

The endpoints the notation supplies are unaffected: `start` and `done` are the features
`Actions::Action` declares, an end bound to the member beside a member-attached `then` names nothing,
and a declared `done;` final node is reached as before.

### `return <expression>;` is no longer accepted in a calculation body

**Breaking change.** A computed calculation result is written as the body's trailing
expression, with no keyword — `calc def Add { in x; in y; x + y }` — which is what the
standard grammar admits and what OpenSysML already executed. The OpenSysML-only spelling
`return x + y;` had no production of its own; it is now a parse error suggesting the
trailing-expression form, and the `nonstandard-notation` warning that reported it is gone.

`return` itself is unchanged: it still declares the result parameter of a calculation, so
`return r : ScalarValues::Real;`, `return r : ScalarValues::Real = x + 1;` and the bare
`return r;` — a result parameter named `r` — all keep working. A trailing expression must be
the last item of the body, so a body that wrote its `return` before other members needs
those members moved above the expression.

### A Node/TypeScript client, `@opensysml/client`

`clients/node/` is a second first-class client: `load`, `loads`, `eval`, symbol lookup and
`instantiate` over the Connect protocol with protobuf bodies, with `Value`, `Verdict`,
`Quantity` and feature values modelled as discriminated unions a consumer switches on
exhaustively rather than as generated protobuf shapes. Not published yet.

- **The lifecycle is the Python one.** A connection that names no address starts a private
  child (`-port 0 -report-address -exit-with-parent`), learns its address from the child's
  first stdout line, and shares it across the thread's connections; the client holds the
  write end of the child's stdin and the child exits at end of file, which a test proves by
  `SIGKILL`ing the parent. A service someone else runs is explicit opt-in (an address or
  `OPENSYSML_SERVICE`) and is never stopped by closing the connection.
- **No native addon and no postinstall download.** The service binary comes from a
  per-platform optional dependency (`@opensysml/sysml-grpc-<os>-<cpu>`) selected by npm's
  `os`/`cpu`, falling back to `$OPENSYSML_BINARY`, `~/.opensysml/bin`, `$PATH`, or an
  external service.
- **A browser entry point**, explicit-address only: a browser cannot spawn a service. It
  needs the server's CORS origins and TLS, and does not use the `grpc-web-text` variant
  `connect-go` does not implement.
- The TypeScript stubs are generated by `buf` (`make proto-ts`, included in `make proto`)
  and committed. `conformance/scenarios` runs through the client's public API for the five
  RPCs v1 covers: 69 of 177 protocol-scenarios pass, 108 skip with their reason recorded,
  none fail.

### The Python client uses a private service of its own

**Behavior change.** A `Connection` that names no address no longer attaches to whatever
`sysml-grpc` happens to be listening on port 50051. It starts a **private child** of the
interpreter instead: the child binds port 0, is given a port by the kernel and reports the
address on its stdout, so no port is chosen, probed or retried and two interpreters starting
at once cannot collide. One child serves every connection of the interpreter that needs the
same service release, sharing its parse cache, and is stopped when the last of them closes.

- **Connecting to a service you manage is now explicit**, and unchanged otherwise: pass a
  host and port (`connect("localhost", 50051)` or `connect("localhost:50051")`), set
  `OPENSYSML_SERVICE=host:port`, or pass `auto_start=False`. Such a service is never stopped
  or replaced by the client. What is gone is *implicit* adoption — a script that happened to
  find a service listening now starts its own, which is also what makes it reproducible.
- **A private child cannot be orphaned.** The client holds the write end of a pipe on the
  child's stdin and never writes to it; the child exits at end of file. The kernel closes
  that pipe when the owning process goes away, so the child does not survive `SIGKILL`,
  `os._exit`, a fatal interpreter error, or a crash during shutdown — cases an `atexit` hook
  does not cover. A `fork()`ing parent disowns its inherited services in the child, so the
  service stays with the process that started it. `sysml-grpc` gained `-report-address` and
  `-exit-with-parent` for this, and accepts `-port 0`.
- **The ownership records are gone**: no `~/.opensysml/sysml-grpc-<port>.pid`, no process
  start times to authenticate a pid with, no stale-record cleanup, no lockfile, and no
  port-collision retry. The guarantee they protected is kept by construction — the client
  signals only the `Popen` of the child it started, so no pid it did not start, reused or
  otherwise, can be signalled. `OPENSYSML_STATE_DIR` moved those records and nothing else,
  so it no longer has any effect; the binary is cached in `~/.opensysml/bin` as before.
- `filelock` and `psutil` are no longer runtime dependencies of the client.

### What Flexo MMS does with our RDF is now measured, not argued

An opt-in gate loads a model's Turtle into a running Flexo MMS stack through Layer 1's graph
endpoint, reads it back through the SysML v2 API, and compares that against the same model
posted through that service's own commit path. It records a per-element, per-property report
that a human adjudicates, so the mapping's interoperability is a number that moves rather than
a claim: measured before element ids and ownership were stated, every element of the fixture was
listed and 86 of its 142 properties delivered, against 158 of 158 for the service's own
payloads. The reference mapping documents what the difference is made of.

The gate needs Docker and stays out of `go test ./...`: it skips loudly unless `FLEXO_INTEROP`
is set, and with it set an absent stack fails instead of skipping.
`.agents/skills/flexo-interop` documents the stack, the token and the traps. Nothing about the
RDF encoding changed.

### Connect is the default server transport

`sysml-grpc` now serves gRPC, gRPC-Web and the Connect protocol on one port, so a browser or
a plain `curl` reaches the service without a proxy and without generated code. Existing gRPC
clients — the `opensysml` Python client, `grpcurl`, any generated `grpc-go` stub — reach the
new default unchanged; `-transport grpc` still serves the gRPC-only server for anything that
needs exactly the old surface.

`GET /health` answers on the main port. `-health-port` still binds its second listener and
still works, now with a deprecation warning, and `-health-port 0` turns it off; the default
becomes `0` in a later release. Nothing in this repository polls it — the Python readiness
probe is a gRPC call.

Two browser prerequisites are now configurable rather than absent: `-cors-allowed-origins`
takes exact origins and refuses `*` at startup, and `-tls-cert`/`-tls-key` serve every
protocol over HTTPS on the same port. `application/grpc-web-text` remains unimplemented and
answers 415, which affects no `fetch`-based client.

Protobuf is the body encoding to use: a 468 KB `Query` answer costs ~6.5 ms as protobuf and
~40 ms as JSON, from `protojson` CPU rather than the 9.7% extra bytes. JSON is the debugging
affordance, a large JSON response now logs a warning, and
[reference/service-transports.md](docs/reference/service-transports.md) says so where a client
author will read it. The conformance suite runs every scenario once per protocol so the second
surface cannot rot.

`-transport stdio` stays as a prototype behind its flag: not the default, and not a transport
any published client speaks. The transports reference says so, and the binary's wiring of it is
now covered by a test.

### One dependency fewer

`github.com/fsnotify/fsnotify` is no longer a dependency. It was linked for a filesystem
watcher nothing reached: the language server is told which files changed by its client, and no
command watches a directory. Building from source now resolves one module less.

### Conformance figures for this release

Every figure is generated from the committed baselines and gated, so none of it is typed in by hand.
Against the pinned reference (`2026-05`, artifact `0.60.1`):

- **Corpus agreement:** 328 of 355 files agree diagnostic-by-diagnostic; 27 diagnostics are ours
  alone, 58 the reference's alone. Read by root, our diagnostics against the reference's own corpora
  fell while our notation warnings on our own example models rose — the removed spellings reporting
  as the errors they now are.
- **Declared-diagnostic silence:** of the 510 declared `errors` rows in the reference's Xpect suites,
  none is one we are silent on; 230 of 230 declared scope assertions match exactly.
- **Permissiveness gaps:** of 120 invalid models we authored, 120 both reject, two of them only when
  we are asked strictly. We wrote every case, so the denominator measures our corpus's reach and not
  our conformance.
- **Declared errata:** the registry declares three defects in the published reference material, two
  with a specification-derived correction. The figures above are as published and stay the
  conformance statement; the corrected-text run is reported beside them and is diagnostic only.
- The oracle baselines now record their own provenance — the pin, the validator bridge digests and the
  identities of the corpora compared — checked by tests that need no Java, with the Java-backed
  reproduction on a schedule. All oracles remain advisory: they inform judgment, they do not replace
  it.

### Fixed

The conformance comparer no longer compares integral fields within the tolerance it allows a
`Real`. Every numeric field was normalized to a `float64`, so a relative tolerance of 1e-9
reached counts, ids and spans as well — 1,000,000,000,500 matched 1,000,000,000,000 — and a
whole number above 2^53 could not be represented exactly in the first place. Integral values
now carry an integral type and are compared by their digits, expected numbers are read as
literals rather than floats, and the tolerance applies to `Real` alone.

- **Parallel machines and actions lower once and completely**: no duplicate nested regions; parallel
  action edges and state behaviors preserved; explicit action successions and implied endpoints
  lowered; explicit action starts required; an anonymous nested action final targeted correctly.
- **Parser**: a binary operator survives a parenthesis-less arrow invocation, and a keyword binary
  operator after a name reads as a calculation result.
- **Semantics**: interface flow features are paired before conjugate names are required.
- **Python client**: a failed child's log is read under a lock once drained, unset features read as no
  value in typed views, and calls into the private service are serialized across threads.
- **RDF**: an expression node id whose owner segment starts with `p` reverses unambiguously.
- **Conformance harness**: two distinct ports are reserved, and a service that exits early is
  reported instead of waited on.

### Python client (`opensysml` 0.3.2)

- **The client changes in this release reach PyPI as 0.3.2**, once `opensysml-v0.3.2` is tagged:
  the private service of its own, `MissingCapabilityError` for a service-side refusal, and the
  three client fixes listed under Fixed. Installing the v0.3.0 core does not wait on it — the
  published 0.3.1 takes an unpinned release's digest from the signed `SHA256SUMS.txt` it verifies
  with sigstore — so this is the client's own changes reaching users, not a prerequisite for the
  core release.
- The `v0.3.0` digests are pinned after the core release's assets publish, before
  `opensysml-v0.3.2` is tagged, so that release is verified against a committed digest rather
  than against the manifest path alone.

### Project

- **buf is the single source of protobuf codegen**, replacing ad-hoc `protoc` invocations: Go stubs
  through pinned plugins, Python stubs (including `.pyi` and the package-relative gRPC import)
  through a local plugin bridging to `grpcio-tools`, plus `buf lint` and `buf breaking` against `main`
  in the Makefile and in CI. TypeScript and Java stubs are generated the same way for the clients
  that now use them, with a Rust template defined beside them.
- **A language-independent gRPC conformance suite**: scenarios live as protobuf-JSON data under
  `conformance/`, and `cmd/conformance` builds and starts the service itself, so a client in any
  language can prove it speaks to `sysml-grpc` without re-deriving the Python suite. Every scenario
  runs once per protocol, so the second transport surface cannot rot.
- **`google.golang.org/grpc` is test-only for production code.** Service errors are connect-native
  with identical codes and messages, the `-transport grpc` server is isolated in one file, stdio
  dispatch no longer carries incidental grpc-go types, and a CI gate keeps grpc-go out of the
  production packages while the tests and the conformance runner keep using it. A consumer importing
  the public Go API no longer links a gRPC server to get a parser.
- **An oracle figure quoted outside the generated block must name the round it measured**, since only
  the `doc-counts` block states the current totals: `scripts/check-doc-figures.py` fails on an
  undated one across the differential, Xpect, rejection and execution oracles and runs in CI beside
  the document id and link checks. Pull-request checks are parallelized under standardized names,
  aggregated into one required status, and each client's job runs only when the pull request touches
  it.
- A core developer guide; the transport evaluation and the Connect surface documented on the site;
  a page per client, and the release procedure for each written down before anything is published;
  internal engineering records excluded from what is published; and the guide's transcripts and
  examples rewritten in standard notation against the real binary.

## 0.2.1 — 2026-08-24

A conformance patch, a round of performance work, and a supply-chain improvement. The
rejection oracle against the pinned OMG pilot (`2026-05`, jupyter-sysml-kernel 0.60.1) now
stands at 120 of 120 both-reject under default mode — the three reserved-keyword cases that
previously only the pilot rejected are errors by default — and every implementation-side
divergence the conformance records adjudicated as fixable is closed. Reporting findings on a
large model no longer costs its size times its findings, and loading one is faster and holds
less memory; every conformance oracle is byte-identical before and after. Releases now
publish a cosign-signed checksum manifest, and the Python client verifies it. No API was
removed or renamed.

### Reserved keyword names are errors by default

- **A reserved keyword used as a name is rejected in default mode**, not only under
  `-strict`: a keyword as a declaration name (`part if;`), a keyword behind an alias, and a
  SysML keyword used as a name in KerML. Parser recovery still produces a usable AST and the
  diagnostic still lands on the offending name; strict mode is unchanged, and no other
  notation extension was tightened. This closes the last three pilot-only rejection cases:
  120 of 120 negative cases both-reject, none by the pilot alone.

### Diagnostics the reference declares and 0.2.0 missed

- **A member-leading succession shorthand** (`then x;` opening a member) is diagnosed as
  nonstandard notation instead of accepted in silence.
- **Duplicate state and transition member names are reported.** State members and named
  transitions carried no declaration-name span, so the name-distinguishability rule never saw
  them; two states of one name in one body now warn as any other duplicate does.
- **Feature accessibility over behavioral bodies is corrected**: a shared `accept` payload is
  accessible from sibling nodes, references in state and transition bodies resolve in their
  own scopes rather than the enclosing one, and a body's implicit result expression is
  checked like an explicit one.
- **A textual representation accepts a short name**: `rep <ocl> inOCL language "ocl" /* … */`
  parses like the named and anonymous forms; contents remain unevaluated.
- **A transition guard that is not Boolean, and a subsetting whose kinds cannot relate, are
  reported even when the file has an unrelated syntax error.** Both checks now gate
  themselves per element rather than per document, so an error elsewhere in the file no
  longer silences them; an element whose own declaration failed to parse stays quiet.
- **A non-Boolean element filter reports the reference's rule by name**: a filter condition
  whose result cannot be Boolean is diagnosed as `Must have a Boolean result`, matching the
  pinned validator word for word, beside the model-level-evaluability requirement it already
  reported. Constant-folded filters such as `filter 1 + 2 * 3 > 0;` stay accepted.
- **Feature accessibility is checked inside element-filter conditions** — the `filter`
  member and the `[...]` clause of an import — with the candidate element as the featuring
  context: a library-declared referent passes, a feature of a user-declared type is
  reported, matching the pinned validator's boundary on both sides.
- **A chain-shaped filter condition is diagnosed by the rule it breaks**, read off the
  compiled predicate's resolved result type rather than the expression's shape: a condition
  the specification forbids reports `Must have a Boolean result` or `Must be model-level
  evaluable` as the reference does, while a condition only our evaluator cannot compute
  becomes a non-blocking warning — which also lets the accessibility check above speak on
  chains it previously never reached.

### Conformance records match the implementation

- The divergence follow-up rows that lagged earlier fixes are closed against live runs of the
  pinned validators, with pinning tests and fixtures added where coverage was missing — among
  them a cold/warm library-cache test proving a cached library symbol keeps its declaration,
  metamodel type and abstractness. Refreshed baselines: differential 353 files, 324 fully
  agreeing (84 diagnostics only ours, 65 only the pilot's); Xpect 1,295 of 1,323 rows agreeing
  (248 wording-only), 28 disagreeing. All oracles remain advisory.
- The semantic-rule map is re-measured for this release: of 727 tracked rules, 646 are
  faithful, 74 approximate, 1 not implemented and 6 deliberately divergent, each divergence
  named in its row. The roadmap reflects this release's targets and gate counts.

### Validating, loading and rolling back cost what they should

- **A source file's line index is built once and reused.** Locating a finding rebuilt the
  whole-file index on every call, so validating a model that reports something cost the
  file's size times the number of findings: a 16 000-element model warning on every usage
  took 123 s and allocated 258 GiB, and now takes 2.1 s and allocates 1.4 GiB. No diagnostic,
  position or message changed.
- **A failed creation rolls back from a log instead of a snapshot.** Every object
  materialization copied the live-instance set beforehand — 81% of everything allocated
  while instantiating; the context now records each registration and a rollback walks only
  what the failed creation added (156.6 KiB to 2.8 KiB per 250-element instantiation).
- **Loading a model does each traversal once.** Validation shared one ordered symbol
  traversal per analysis instead of re-collecting it per pass, direct-child lookups are
  cached with generation-based invalidation, redefinition closures are computed once and
  reused, and scope members are iterated in place instead of through copied slices. The
  lexer's token buffer is pre-sized from the source length.
- **A scope stores its members flat and indexes them lazily.** Most scopes never hold a
  named member, and most that do hold a handful, so the per-scope map is gone: names and
  symbols live in two declaration-ordered slices, scanned up to twelve names and indexed on
  demand above that. Loading holds ~7% less live heap; a whole-binary validation of a
  12 000-element model runs ~5% faster at lower peak memory.
- **Member iteration is deterministic.** Scope members, FQN registration and duplicate
  declarations now follow declaration order everywhere Go map order previously leaked
  through; for duplicate declarations the first declared wins, pinned by tests. No
  diagnostic, position, message or execution result changed anywhere in this section —
  the differential, Xpect and rejection oracles are byte-identical before and after.

### The release manifest is signed

- **`build-release` signs `dist/SHA256SUMS.txt`** with cosign keyless using the release
  pipeline's OIDC identity and publishes the sigstore bundle beside it as
  `SHA256SUMS.txt.bundle`. Nothing changes for a caller downloading binaries directly.

### Python client (`opensysml` 0.3.1)

- **A core release published after this client can now be installed.** Previously every
  installable core release needed a digest pinned in this client, so a new core release
  required a new PyPI release. Now, for a release with no pin, the client downloads the
  release's `SHA256SUMS.txt.bundle`, verifies it against the release pipeline's identity with
  sigstore, and takes the asset digest from the verified manifest. Pins stay authoritative:
  where one exists it wins, and a verified manifest that disagrees with a pin is reported as
  a mismatch, never used as a downgrade.
- **`sigstore` is an optional dependency, imported on demand.** An install without it refuses
  to verify an unpinned release — exactly the previous behavior — rather than failing to
  import `opensysml`.
- The `v0.2.1` digests are pinned after the core release's assets publish, before
  `opensysml-v0.3.1` is tagged.

### Project

- The documentation site is published at [opensysml.org](https://opensysml.org/); the old
  GitHub Pages address redirects there.
- GitHub issue forms and a pull request template.
- Reader-facing documentation no longer uses internal work-item labels, and
  `scripts/check-doc-ids.py` keeps it that way.

## 0.2.0 — 2026-08-24

Three more advisory oracles now judge this implementation against the pinned OMG pilot
(`2026-05`, jupyter-sysml-kernel 0.60.1), and most of the behavior below is what they found: the
expectations the pilot's own Xpect suites *declare*, the invalid models it rejects, and the part of
its surface that can referee execution at all — which is expression evaluation and nothing else.
Notation and validation rules moved with them: 324 of 353 differential files now agree
diagnostic-by-diagnostic (was 221 of 338), and of the 510 declared `errors` rows in the pilot's own
suites there is none we are silent on. Name resolution now enforces membership visibility, so a
model that named a `private` or `protected` member across a namespace boundary and analyzed clean in
0.1.2 is now reported. A new opt-in strict mode raises this implementation's own notation extensions
to errors; default behavior is unchanged. The standard library is loaded once per process and
shared, which takes a 100-model gRPC cache from 1598.3 MiB of retained heap to 1.1 MiB. The gRPC and
Python surfaces gain fields and calls; nothing was removed or renamed.

### Three new oracles, and one that says what it cannot judge

- **`cmd/pilot-xpect` adjudicates the expectations the pilot declares**, not its observed behavior:
  the 428 `.xt` files it ships in `org.omg.kerml.xpect.tests` (303) and `org.omg.sysml.xpect.tests`
  (125), read through the same `scripts/pilot-pin.sh` pin as every other corpus, over six assertion
  kinds (`errors`, `noErrors`, `linkedName`, `warnings`, `scope`, `exportedObjects`). The committed
  baseline (`docs/project/pilot-xpect-baseline.json`) stands at 1,323 rows from 1,261 assertions:
  1,295 agree — 248 of them wording-only — and 28 disagree, with 0 files unparsed, 0 rows unlocated
  and 0 not adjudicated. Declared scope assertions agree 230 of 230 and declared linked names 194 of
  194.
- **`cmd/pilot-reject` asks the reverse question every other oracle cannot**: does this
  implementation reject what the reference rejects? Both validators run over a negative corpus we
  wrote ourselves, grown from 34 cases to 120. 117 agree, 3 are rejected by the pilot alone, and
  none by us alone; 5 of the 117 agree only when we are asked strictly, and by default 8 of the 120
  are ours-accepted. The denominator is our own authorship, so it measures the reach of the corpus
  rather than conformance, and agreement reached only under strict mode is recorded as the weaker
  evidence it is.
- **`cmd/pilot-exec-diff` maps the pilot's execution surface before comparing anything**: there is
  no interpreter, simulator, scheduler, token or trace in the pinned artifact, so of the four
  behavior areas asked about, model-level expression evaluation is adjudicable and actions, state
  machines and classifier behaviors are out of reach. 125 of the tracked rules therefore have no
  external referee at all, and the figure is now stated wherever behavior compliance is claimed.
- **The differential covers 353 files in seven roots** (the OMG training corpus, the pilot's SysML
  example, SysML validation and KerML example corpora, our testdata, examples and probes): 324 agree
  exactly, 83 diagnostics are ours alone and 66 the reference's alone, from 139 and 122 diagnostics
  respectively. Read by root rather than in total — our diagnostics on the reference's own corpora
  fell while our non-standard-notation warnings on our own example models rose.
- **All four harnesses stay advisory.** Nothing in CI depends on a comparison with the pilot, whose
  Java validators CI does not provision; what CI gates is our own verdicts on the pinned files
  (below).

### An opt-in strict conformance mode

- **`sysml -strict`, `%strict on|off` in the REPL, `strict_conformance` on `ParseFileRequest` and
  `strict_conformance=True` from Python** judge the source as conforming SysML v2: notation this
  implementation accepts that no pinned OMG production admits becomes an error instead of a warning.
  It needed no second grammar and no edit to any model, example or golden — the mode raises the
  severity of the existing `nonstandard-notation` finding and nothing else. Strictness is part of
  the analysis cache key, so the two modes never serve each other's diagnostics, and
  `internal/core/conformance` holds the mode so no pass decides it locally.

### Name resolution follows the specification's two resolution rules

- **Membership visibility is enforced.** A qualified name, a feature chain link and an import all
  consulted a membership without consulting its visibility, so `A::X` resolved even where `X` is
  private in `A`. One predicate now applies the rule on all three routes and to the LSP surface:
  the 70 declared `Couldn't resolve reference to …` rows we were silent on fall to 4.
- **A qualified name's first segment resolves locally and every later segment visibly**, per KerML
  8.2.3.5.3–8.2.3.5.4, which is the distinction the pilot's `VisibilityTests_ProtectedImport_*`
  fixtures separate: a specializing namespace reaches a protected member by simple name, but what
  the *referring* namespace specializes cannot widen what a later segment sees. Generalization
  headers resolve outside the declaration body, qualified redefinition tails walk direct supertypes
  through a speculative probe that emits no diagnostic when it fails, and unqualified lookup
  distinguishes an inherited import from a direct nested one.
- **A supertype's non-private *imported* memberships are inherited** as its owned ones are (KerML
  8.3.3.1 with 8.4.3.2), so redeclaring a name a supertype imported is the distinguishability
  violation the reference declares.
- **Two root namespaces of one name are one global name, not an ambiguity** — resolution in the
  global namespace is single-valued (KerML 8.2.3.5) and distinguishable naming constrains a
  namespace's own members, which the global namespace is not.
- **The reference's name-distinguishability rule is implemented with its own scope and severity** —
  a warning, not an error — over owned, alias, inherited and diamond-inherited conflicts, with an
  explicit `:>>` redefinition suppressing the inherited-name conflict and a plain redeclaration,
  reference or subsetting not.
- **Derived scope paths are bounded** by one re-entry per name, count an inherited import as a
  derivation step, and inherit through a feature's declared type before its implicit base.
- **A redefined name is masked** from member enumeration, and the mask is built once per enumeration
  rather than per member.

### The validation rules the reference declares

- **Thirty-nine new level-scoped passes in `internal/core/passes`** implement rules the pilot's own
  Xpect suites declare and this implementation reported nowhere, each scoped from the constraint in
  `KerMLValidator.xtend` / `SysMLValidator.xtend` rather than from its message string — type
  unioning/intersecting/differencing and feature chaining, multiplicity bound types, reference
  subsetting, top-level import visibility, association end types, occurrence typing, connector
  featuring, flow ends, variability, implicit base and "features must have at least one type",
  conjugated specifics, and a portion that cannot be variable. Of the 510 declared `errors` rows,
  the number we report nothing for is 0; 243 match word for word and the rest differ only in wording
  or location.
- **Rules that fire where the reference warns now warn**: library inherited-name diamonds, short
  names, a user-declared standard library, non-conforming bindings, and a computed return.
- **Validation tier gating asks about the element, not the document.** A blocking error used to skip
  every higher-tier pass for the whole file; `passes.ElementScoped` plus one query on `Context` now
  gates per subject, so a valid declaration is still checked when an unrelated one failed to
  resolve.
- **Twenty-seven differential diagnostics we reported on models the pilot accepts are retired**, two
  of them parse/resolve defects rather than rule defects, with a key-by-key check that no row was
  added and no category re-bucketed. All 137 diagnostics the pilot reports and we did not are
  classified — our defect, adjudicated divergence, or a defect of the pilot — with a named reproducer
  per family.
- **The Step 3 conformance obligations land**: `UsageMayTimeVary` derived from occurrence ownership,
  assignment referents validated in an element-scoped pass, and `BinaryInterface` / `BinaryConnection`
  inferred only for exactly two-ended untyped declarations rather than by a universal arity limit.

### Notation the reference accepts now parses

- **The KerML declaration grammar cluster is closed**: a member with no kind keyword is a
  feature wherever a member is expected, decided by lookahead over the declaration head rather than
  by a keyword table, so `a : Integer;`, `x;`, `p5[1] : Real;` and `composite e1 redefines V::m;`
  parse. The pilot's KerML example corpus now reports no syntax diagnostic of ours at all.
- **Reserved words are read per grammar.** The lexer keeps one token set, but reservation is now the
  parser's decision from the kind of the file being read, so `part chains : T;` is legal in a
  `.sysml` file where `chains` is a word only `KerML.xtext` spells.
- **Index sequences, multiplicity subsetting, `..` recovery and exhibit references parse**, and every
  remaining parser-only divergence in the pilot corpora was probed against the pinned grammars and
  accepted only where a production derives it — three neighboring forms are not derivable and stay
  rejected, with the finding recorded per row. The 28 syntax diagnostics that remain in the
  differential are all this implementation's own registered extension warnings in `examples/`.

### Metadata annotations

- **A metadata annotation body has a scope and its names resolve.** Per KerML 7.4.7 and 8.3.3.3 a
  declaration in the body implicitly redefines a feature of the metadata definition, so in
  `@A { x = ~3; }` the name `x` is `A::x` while the value `~3` resolves in the enclosing scope chain.
  Nested `@Safety` / `@Security` annotations resolve through public namespace and membership
  re-exports.
- **Model-level evaluability is an explicit walk over the expression** (`Model.ModelLevelEvaluable`)
  rather than a by-product of filter compilation: literals, `null`, metadata access, sequences,
  `new T(…)` with evaluable arguments, invocations of the Kernel Function Library functions the model
  itself evaluates, and reads of features reaching an evaluable value are evaluable — being declared
  by the normative library is not the criterion, so `RealFunctions::sqrt(4.0)` is correctly rejected.
- **Metaclass reflection is answered from the element**, and a keyword-first relationship is
  classified by its own metaclass and is a first-class element with ordered ends.

### Execution

- **`send x via p to r` routes instead of reporting unsupported.** Lowering was dropping the
  receiver, so the runtime had nothing to route with; it now carries port, receiver name and sending
  object losslessly, with qualified and shadowed receiver names resolved.
- **A simple state transitioning to itself exits and re-enters.** `transition s to s` ran the effect
  and stayed put — no exit action, no entry action — because the enclosure test answered that the
  transition never left its source.
- **A name denoting one object evaluates to that object**, a vector's elements are its sequence, and
  a merge node's body runs with the traversal that wins rather than on every arrival.
- **Declarations in expression bodies are evaluated and scoped**, calc/constraint operations are
  invoked with the performer context preserved, value type classification is implemented, and
  executor budget errors are typed (`ErrInvalidActionFlow`, `ErrNoEnabledSuccession`,
  `ErrActionDeadlock`) so a malformed flow, an unenabled decision and a deadlock are distinguishable
  rather than one opaque failure.

### The standard library

- **One frozen library index is shared by every model**, with each model's documents in an overlay
  over it. Measured with gRPC at its default `--cache-size 100` over 100 distinct library-backed
  models: retained heap 1598.3 MiB → 1.1 MiB, RSS 2180.3 MiB → 76.5 MiB, library indexes built
  104 → 1. Four REPL sessions go from 122.7 MiB to 17.1 MiB. `Index.Freeze()` makes every write-like
  method fail loudly and `symbols.NewOverlay(base)` refuses a non-frozen or already-stacked base;
  evicting a model tombstones a base-owned document locally instead of deleting what another model
  still reads.
- **A cache hit can no longer produce a poorer semantic state than a miss.** Records held FQN-level
  symbols only, so with a warm cache a library type had no members, declared values or condition
  ASTs — the same commit diverged cold vs warm in user-visible ways (`internal/core/solve` failing
  cold, ~60 inherited library attributes reported instead of 5, unresolved-reference errors on a
  filtered library facade). The library is parsed on every path and the on-disk cache persists only
  derived facts, with a reflective test that fails both ways: a persisted field with no comparator,
  and a comparator for a field no longer persisted. The cache is keyed by build, so a semantics
  change is a miss rather than a stale hit.
- **Library provenance is asked of the index, not inferred.** Four consumers decided a member was
  not the model's own from the accident that a library symbol carries no declaration; they now ask
  `Index.Library`, so a user's own imported library is library content when the index says so and a
  model package named `Occurrences` is not.
- **`ApplyEdits` takes one library index per request**, not one per edit operation plus validation's
  — a 10-operation request built 11.

### Authoring, query and export surfaces

- **Elements can be created and deleted from Python** as source-preserving edits
  (`AddMemberEdit`, `DeleteEdit` over gRPC): every edit is a byte splice into the loaded source, the
  result is reparsed and reanalyzed, and the whole batch is refused if it introduces errors the
  original did not have. Five typed failures are added — `EDIT_FAILURE_OWNER_UNKNOWN`,
  `EDIT_FAILURE_OWNER_NOT_NAMESPACE`, `EDIT_FAILURE_ILLEGAL_KIND`, `EDIT_FAILURE_MEMBER_NAME_TAKEN`,
  `EDIT_FAILURE_DELETE_REFERENCED` — each with a Python exception. `loads()` parses inline content,
  and `ParseFileRequest.language` selects `sysml` or `kerml` for it.
- **Renaming a declaration rewrites the references to it**, and an alias segment is renamed as the
  name it wrote rather than as the element it reaches.
- **OSLC Query 3.0 text is a second query front end** beside the structured SysML v2 API `Query`,
  reachable as `sysml -query 'oslc.where=…'`, `%query` in the REPL and `QueryRequest.oslc_query`.
  Neither surface subsumes the other: structured queries keep `and`/`or` constraint trees, OSLC
  brings `!=`, `<=`, `>=`, `in [...]` and `oslc.orderBy`. The CLI and REPL front ends carry OSLC
  text only; a structured query stays on `QueryRequest.query`. This is not an OSLC server — no query
  capability documents, result containers, service providers or resource shapes.
- **RDF carries expression trees beside the source text** of an expression, states the features a
  binding head relates as structure, keeps a keyword-first relationship's declared visibility, and
  carries a multiplicity's subsetting through conversion.
- **`SymbolInfo.withheld_library_attributes` states what a projection withheld** instead of
  withholding it silently, and two symbol-projection defects are fixed.
- **The REPL parses each snippet as the kind of the file it came from**, defers a load-time notation
  error to the analysis report instead of vetoing the run, and no longer hides what a submission
  declared behind that error. Its prompt scope falls back to the root holding the last declaration.

### Editor surface

- **Contextual keywords are highlighted and completed.** Words the parser reads positionally —
  `chain`, `choice`, `decision`, `deep`, `defer`, `done`, `point`, `region`, `var` and the rest —
  were in neither the TextMate grammars nor LSP completion, because both derived their word list
  from the reserved-word table those words are deliberately absent from. One exported source of
  truth per language now feeds both surfaces, with lexing untouched.
- **The editor is no longer blind inside a metadata annotation body**: hover names the redefined
  feature and its type, go-to-definition jumps to it including an inherited one, completion offers
  the metadata definition's features at a declaration position and the enclosing scope's at a value
  position, and document/workspace symbols and semantic tokens cover the body.
- **Rename and find-references follow the name an alias segment wrote**, so an editor rename
  (<kbd>F2</kbd>) on an alias rewrites its uses, and the same rename on the target no longer
  rewrites a name that was never the target's.

### Declared errata: the published reference material can itself be wrong

- **`internal/errata` is a registry of declared defects in published reference material**, and all
  three corpus oracles now report every census twice — as published, which stays the conformance
  statement, and with the errata applied as a secondary diagnostic. The registry declares 2 defects,
  1 with a specification-derived correction and 1 documented without one because no intended reading
  can be inferred. The published corpus is never edited: a correction is applied to a materialized
  copy under the oracle's own gitignored output directory, and an erratum never reclassifies a
  divergence category.

### Measurement infrastructure

- **The four pinned OMG corpus gates share one mechanism** and keep their two deliberate policies:
  the training corpus is asserted clean, the three pilot corpora are a per-file ratchet whose every
  movement must be adjudicated. The three pilot corpora (212 files) were report-only before this and
  failed nothing in CI; what is gated is our own verdicts on those files, which need no Java
  validator and run in pure Go.
- **The refereed figures in `README.md` and `docs/internals/architecture.md` are generated from the
  committed baselines** by `make docs-counts`, which previously only checked a hand-maintained block.
  No number is typed in by hand, and `cmd/doc-counts -check` makes divergence a build failure.
- **The `~98% of targeted features` claim is gone.** `docs/project/spec-compliance.md` is a census of
  our own row list — 727 tracked semantic rules, 641 faithful, 75 approximate, 5 not implemented, 6
  deliberate divergence — stated as bookkeeping that moves when rows are rewritten and not when an
  oracle does. No percentage of the specification is claimed anywhere.

### What these numbers do not show

The OMG corpora are demonstrations rather than an official conformance suite; the differential is
one-directional; the Xpect suites are the pilot authors' test intent rather than a certification
oracle; the rejection corpus is our own authorship; and the pinned artifact evaluates expressions but
executes neither actions nor state machines, so the 125 behavior rows that carry the action,
state-machine and classifier-behavior semantics are self-assessed. 28 declared Xpect rows and 83
differential diagnostics remain, each adjudicated in `docs/project/pilot-xpect.md` and
`docs/project/pilot-differential.md` rather than left to be discovered.

## 0.1.2 — 2026-08-20

A measurement release. Two advisory harnesses now ask the OMG pilot implementation and the
pinned OMG grammars where this implementation differs from them, and most of the behavior below
is what they found — chiefly in KerML, which the pilot's own KerML validator now judges. A
`.kerml` file is read as KerML rather than as SysML written with other keywords, so a KerML
model 0.1.1 rejected may analyze clean here; notation accepted beyond the grammars now warns
where it used to be silent. Nothing was renamed and no interface changed.

### The pilot implementation judges every corpus, KerML included

- **`cmd/pilot-diff` compares our diagnostics against the pinned OMG pilot implementation**
  (`2026-05`, jupyter-sysml-kernel 0.60.1) file by file, over 349 files in seven roots: the OMG
  training corpus, the pilot's own SysML example and validation corpora, its KerML examples, our
  testdata, our examples and our probes. 254 files agree exactly. It is advisory — a harness that
  finds work, not a gate — so nothing in CI depends on it, and its committed baseline
  (`docs/project/pilot-differential-baseline.json`) makes a rerun a diff rather than a reading.
  On the 100-file training corpus both implementations report nothing at all.
- **The KerML corpus is judged by the pilot's own KerML validator**, not by our reading of the
  KerML specification: `scripts/pilot-kerml-validator/ValidateKerML.java` registers the pilot's
  `KerMLStandaloneSetup`, loads `sysml.library` and the corpus into one EMF `ResourceSet`, and
  asks the pilot's `IResourceValidator` with `CheckMode.ALL`. It contributes no rule of its own,
  so a disagreement it prints is the reference's verdict. Over the pilot's 58 KerML examples, the
  diagnostics only we reported fell from 439 to 98 as the work below landed — the syntax class
  from 360 to 85, and the genuine name-resolution class from 123 to 10 — and 35 of the 58 files
  now agree exactly. Ten of that root's remaining diagnostics are name resolution rather than
  seven: parsing notation that used to be rejected exposes references behind it, so the class
  rose by three as the parser improved.
- **Six diagnostics the pilot reports and we do not are a defect in the pilot**, not a gap here:
  EMF's unpaired-bidirectional-reference check firing on the pilot's own
  `Type::ownedDisjoining` / `Disjoining::owningType` opposite, reproducible from three lines
  (`classifier A; classifier B disjoint from A;`) in a fresh resource set. It is recorded with
  its reproducer rather than absorbed into our own numbers.
- **The harness picks the reference validator per file**, by the file's own language rather than
  per corpus, so a directory holding both `.sysml` and `.kerml` is judged by the SysML validator
  and the KerML one respectively, and a missing KerML validator is reported rather than silently
  skipped.
- **The 373 diagnostics only we reported on the two OMG SysML corpora were adjudicated construct
  by construct**, into ten classes recorded in `docs/project/spec-compliance.md` with the grammar
  production each one cites (`ExtendedUsage`, keyword-less members, node bodies, `return`,
  `binding`/`message`/`event` declarations, requirement members, resolution through imports and
  inheritance, and textual representation). Six of the ten are wholly or partly fixed below,
  taking those two corpora to 204; the rest name the site that rejects the notation, so the work
  is scoped rather than discovered. Where a newly-parsed declaration reaches the type tier for the
  first time it may report there instead, which is why one narrower class rises as the parser
  gains ground — those are unmasked diagnostics, adjudicated per file, not new false positives.
- **`cmd/grammar-coverage` measures the pinned OMG grammars against every corpus we hold**:
  483 of 727 productions and 802 of 807 notational forms have input-presence evidence. The
  number is deliberately an over-approximation — presence of an input a production admits, never
  parser-execution coverage or compliance — so the page's useful reading is the five forms with
  no evidence anywhere, each adjudicated: the `%` remainder operator and prefix metadata on a
  namespace are implemented but exercised by nothing, and the named `disjoining`, `conjugation`
  and `redefinition` relationship declarations are not implemented.

### A `.kerml` file is analyzed as KerML

- **A KerML declaration specializes the library type its keyword implies**, so the features of
  that library type are inherited: `class` reaches `Occurrences::Occurrence`, `struct` reaches
  `Objects::Object`, and so on through `assoc`, `behavior`, `function`, `interaction`,
  `metaclass`, `datatype`, `classifier` and `type`. No library member was inherited before, so
  `portion focusedState : Camera subsets timeSlices;` reported an unresolved reference to a
  feature the library declares. A declared generalization suppresses the implicit base only when
  it already reaches it, so `struct MyWheel specializes Wheel` still reaches `Objects::Object`,
  and a supertype restored from the index cache keeps those edges. A bare `feature` still gets
  no base: the SysML attribute base is not KerML's.
- **SysML's definition-and-usage checks no longer fire on KerML declarations.** `class Person
  specializes Object` was reported as "only a definition may specialize; found a usage" — a
  distinction KerML does not draw. A `.kerml` specialization or typing now requires only that
  its target be a type, from an explicit list of what a type is rather than a guess about what
  isn't, so `metaclass AtomMetadata specializes Metaobject` is accepted while a non-type target
  is still reported. The SysML files keep the kind checks they had.
- **A union conforms through its unioning types** — `classifier MyWheel unions MyWheel1,
  MyWheel2` — without unioning becoming a generalization.
- **A declaration's header sees its own body.** Names in a `featured by`, `crosses` or
  subsetting clause resolve against the members and imports of the body of the same declaration
  before the enclosing scope, and stay reachable afterwards by qualified name and feature chain.
  Resolution of a member inherited through an implicit base no longer recurses.
- **An unknown subsetting target is tolerated** rather than reported as a KerML type error about
  something else.
- **The REPL reads each snippet as the kind of file it came from**, so a `.kerml` snippet gets
  KerML's contextual names and a `.sysml` snippet keeps SysML's reservations, and a session mixing
  the two keeps each snippet's language rather than analyzing everything as SysML.
- **Inline content keeps the language it was submitted with** through parsing, analysis and type
  checking, so a service request that hands over `namespace N;` as KerML is judged by KerML's
  rules for the whole pipeline rather than only at the parse step.

### KerML notation the reference accepts now parses

- **`featured by`, an n-ary connector end list, and a typed or redefining succession parse**:
  `class Owner { member feature inCart : Product featured by Account; }`,
  `connector c : A (a, b, c);` and `succession s : Link [1] first paint then dry;` — whose own
  `[1]` belongs to the succession rather than to its first end — along with the `succession
  redefines s : …` spelling. A missing `by`, a missing target, a missing `then` and a trailing
  comma are still reported.
- **`at`, `while`, `merge` and `decide` are names in a `.kerml` file**, where they are not KerML
  keywords, as `about`, `bind` and the other SysML-only words already were. The remaining KerML
  feature-prefix forms — `abstract var feature x [0..*];` — are still not parsed and are recorded
  as such.

### Imports and visibility

- **A `public` import is re-exported to importers of the importing namespace**, transitively;
  before, re-export stopped after one namespace. A root-level import is visible from a nested
  package, an imported name may prefix a qualified name, and an import cycle terminates instead
  of recursing. A `protected` import is reachable through a specialization of the importing
  namespace and from nowhere else.
- **An import with no visibility indicator warns.** The grammar requires one, so `import Lib::*;`
  now reports `[syntax/import-visibility] import without a visibility indicator: SysML v2
  requires public, private or protected before 'import'`. It is a warning at the syntax tier and
  analysis continues through it; `expose` is exempt, its grammar supplying protected visibility
  implicitly.

### SysML notation the parser was refusing

- **A connector, interface or flow written with shorthand ends may have a body**:
  `connect x.p to r.p { ... }`, `interface b1.p to b2.p { ... }` and `flow s1.x to s2.x { ... }`
  parse with their members. An unclosed body, an interface without `to` and an unterminated flow
  body are still reported.
- **An accept node is an action statement**, so `then action engineStopped accept engineOff :
  EngineOff;` parses and executes. An accept in a loop or branch body remains unsupported by
  lowering and is reported when reached, rather than accepted and silently skipped.
- **`connect` requires its ends and reads a leading multiplicity on each of them**:
  `connect [1] a to [1] b`. `connect;`, `connect { ... }` and a missing target are reported where
  some were accepted and misrepresented.
- **Eleven words that no grammar production reserves are names again**, `on` and `var` among
  them, so `state on;`, `part on : On;` and `attribute var : ScalarValues::Integer;` parse as
  declarations named `on` and `var`. Each word remains a keyword in the position its grammar
  gives it, so `var a : Integer;` without a kind is still reported.
- **A modifier before a kind prefix keeps the kind**, which was dropped from the tree and from
  every diagnostic that read it.
- **Prefix metadata may stand in for a member's keyword**, as the grammar allows, so `#Classified
  connect a to b;` declares the connection, a prefix may follow a modifier (`abstract #Classified
  z;`) or `end`, and it composes with redefinition (`#service :>> sd : PD;`). Some accepted prefix
  forms still reach the type tier with the wrong usage kind and report a kind mismatch, recorded
  as an approximation rather than closed.
- **A member may be written with no keyword at all**: `T1 = 10.0;`, a typing-only member
  (`kpl : D = km;`), an enum value list (`= 60.0; = 70.0;`), a `locale "en_US"` body, and a
  bare result expression as the last member of an analysis or case. An assignment is still an
  assignment rather than a declaration, and an empty assignment, a malformed specialization-only
  member, a malformed enum value and an incomplete `locale` are still reported.
- **A case or analysis body's result expression may begin with a keyword or a word operator**, so
  `if v.m > 1.0 ? v.m else 1.0` and `small and large` are read as the body's result instead of
  being parsed as another member declaration.
- **Every node production's optional body parses, lowers and executes**: a transition, send or
  accept node and every control node may carry `{ ... }`, a transition target may be qualified
  (`then done.stop`), a `for` variable may be typed and a body parameter may redefine. A merge
  node's body runs with the traversal that wins rather than on every arrival. Three corpus forms
  stay rejected and are recorded as such: `exhibit vehicleStates.on { ... }`, a bare
  `ref patient { ... }`, and `send x via p to r`, which parses but is reported as not executable.

### Notation accepted beyond the grammars now says so

- **A construct we accept that no pinned production admits warns** at the syntax tier, from the
  conformance audit: `namespace`, `region`, `choice`, `junction`, an entry/exit/history point,
  `defer`, the `initial`/`final`/`decision` node spellings, and `transition <source> to
  <target>;`. `namespace P { }` in a `.sysml` file reports "`namespace` is KerML notation: the
  SysML v2 grammar has no namespace declaration, so write `package` here or move the declaration
  to a .kerml file"; `featured by` in a `.sysml` file reports the same for the featuring clause.
  The same notation in a `.kerml` file is silent, because there it is standard. These are
  warnings — the models that use this notation still parse, and no higher tier is gated — and the
  REPL warns for its own buffer, which it reads as SysML.

### Analysis corrections

- **An alias is followed through every type relationship** — specialization, typing, subsetting,
  redefinition, multiplicity and invocation inference — so `part def AvionicsLRU :> Box`, where
  `Box` aliases `RectangularCuboid`, no longer reports "part cannot specialize alias (kind
  mismatch)" and inherits what the aliased definition declares. An alias cycle terminates.
- **An invocation of an aliased action or function is type-checked** against its parameters
  instead of going unchecked.
- **An unqualified name resolves through imports and inheritance as a written reference does**, so
  a name a namespace acquired *by* an import is visible to a wildcard import of it, a feature
  reachable by feature chain may be subset, and a redefinition may introduce the type its own
  members are looked up through (`item :>> shape : Box [1] { ... }`). A private import still
  re-exports nothing.
- **A usage may be typed by anything its kind's taxonomy admits** — a part by any occurrence
  definition, a use case by a use-case definition, a succession by what the reference's own rule
  allows — where a narrower table reported a kind mismatch on models the reference accepts.
  `action d : OccurrenceFunctions::destroy` is still rejected: the cached library symbol for a
  KerML `function` is recorded as a calc usage, which is a different layer.
- **A declaration with a short name is listed once** among a document's members, not twice.
- **A part typing check that read an unrelated declaration is gone**, with the diagnostics it
  produced on valid models.

### Source-preserving edits over gRPC

- **`ApplyEdits` adds a member and deletes a declaration**, alongside the value-set and rename it
  already offered, and preserves the source it did not touch — comments, blank lines and layout
  survive, and the result is reparsed and reanalyzed rather than trusted. An edit that would break
  a reference is refused with a typed failure and no content: `EDIT_FAILURE_RENAME_REFERENCED` for
  renaming a referenced element, `EDIT_FAILURE_DELETE_REFERENCED` for deleting one without asking
  for a cascade. The client wrappers for these calls ship on the Python client's own tag.

### Diagrams in the VS Code extension

- **`SysML: Open Diagram` renders the open model in a panel** and keeps it current as the file
  changes, over three new LSP requests — `opensysml/render`, `opensysml/views` and
  `opensysml/renderChanged` — documented in `docs/reference/lsp.md`. The command is gated on the
  server capability `experimental.openSysmlRender`, and the panel is read-only and makes no
  network request.
- **Go-to-definition locates the identifier of a rendered element**, not only its declaration.

### Four pilot rules this release does not implement

Recorded in `docs/project/spec-compliance.md` with the divergence each one produces, rather than
left to be discovered: featuring-type access on a subsetting (`validateSubsettingFeaturingTypes`),
flow-end subsetting (`validateFlowEndSubsetting`, so `flow of Fuel from tank to thruster;` is
accepted here and rejected by the pilot), invocation instantiated type
(`validateInvocationExpressionInstantiatedType`, so `part w = Widget();` on a `part def` is
reported by the pilot and by nothing here), and model-level evaluability of a filter, which
diverges in both directions.

## 0.1.1 — 2026-08-19

A fix release: every change below corrects something 0.1.0 got wrong about a valid model, so a
model that 0.1.0 rejected or misread may analyze differently here. Nothing was renamed and no
interface changed.

### The OMG training corpus reports no errors, and two of its files were never buggy

- **Every definition body inherits the features of the library definition its kind implies.**
  Only behavior definitions had an implicit base, so `snapshot sale = start;` inside a `part def`
  reported `unresolved reference: start` even though `Items::Item` declares `start` and `done` and
  `Parts::Part` redefines both. The verdict recorded against `Time Slice and Snapshot Example` and
  `Individuals and Time Slices` — "bugs in the OMG files" — was wrong; both are clean now, and the
  corpus baseline lists no files.
- **Because those features are now inherited, a member that reuses one of their names is
  reported** where it used to shadow silently: `part def C { part start; }` conflicts with
  `Parts::Part::start`. Redefine it to keep the name — `part start :>> Parts::Part::start;` — which
  is what the model means.
- **A qualified redefinition of an inherited library feature is accepted:**
  `snapshot start :>> Parts::Part::start;` reported "start is not an inherited member of C" because
  a library supertype restored from the index cache carries no scope to compare against.
  Redefining a feature the owner does not inherit is still reported.
- **A metadata usage ends at its own `;` or body:** `@M part def Car;` was read as an annotation
  plus a declaration with no diagnostic. `#M part def Car;` is the prefix spelling.
- **A definition may specialize a definition of a comparable kind:** `individual item def Alice :>
  Person` was refused as a kind mismatch because `Person` is a `part def`. A part definition *is*
  an item definition, so specialization follows the definition taxonomy rather than an exact kind
  match; disjoint kinds — a part definition and an attribute definition — are still refused.
- **A transition may leave the entry action of the state that declares it:** `entry action begin
  { } transition begin then off;` reported the action as "not a state or pseudostate". The entry
  action stands in for a start pseudostate, so the transition designates the state the machine
  starts in rather than an edge between two vertices, and it executes as such. Only that bare
  completion shape designates a start: an ordinary action named as an endpoint, a transition *into*
  an entry action, and a triggered or guarded one out of it are all reported, rather than accepted
  with the trigger, guard or effect dropped. The designation is read from the body the transition
  is written in, so a name reaching another state's entry action, or one naming a junction rather
  than a state, is reported where it used to analyze clean and then fail to execute. A machine
  designated this way renders its initial state in a view that only exposes it.
- **A metadata usage member names a type**, so `@Securty;` reports an unresolved reference the way
  the `#` prefix spelling does instead of going unchecked.
- **A value part accepts every operator the grammar allows** — `= expr`, `:= expr`,
  `default expr`, `default = expr` and `default := expr` — wherever a usage, parameter, result or
  subject binds a value; only some spellings were accepted per position.
- **A metadata usage member (`@M;`) parses in a namespace, a body and a state body**, and RDF
  conversion refuses it with a typed diagnostic instead of writing an annotation on a different
  element.
- **The REPL no longer prints a syntax warning twice**, once from the load that defers the
  analysis and once from the analysis itself.

### The Python client accepts the 0.1.0 service

- **`opensysml` carries the pinned `sysml-grpc` digests for `v0.1.0`**, so
  `OPENSYSML_GRPC_VERSION=v0.1.0` downloads and verifies the service instead of refusing it as
  unpinned. `PINNED_SHA256` stopped at `v0.0.8`.

### A rendering read at a terminal

- **`sysml -render` writes the text form at a terminal**, where a person reads it, and the
  machine-readable form of the kind rendered — Mermaid for a diagram, Markdown for a table — into a
  file or a pipe, where a tool does. `sysml m.sysml -render Views::table` showed a Markdown table on
  screen; `> table.md`, `| tool` and `-o table.md` are unchanged, and `-render-form` still names
  either form whatever the destination.
- **The text form is ASCII**: the rendering header and a connection edge were written with an em
  dash, which a terminal drawing no more than ASCII showed as a replacement character.
- **A text table is written to fit the terminal**, wrapping a cell wider than its column over as
  many lines as it needs rather than truncating it or overflowing the window. Columns are narrowed
  no further than 8 characters, and a table written to a file or a pipe keeps every column as wide
  as its widest cell, so a saved artifact does not depend on the window it was written from.

## 0.1.0 — 2026-08-18

### The project is now OpenSysML

The rename is a clean break with no compatibility aliases: every name below has exactly one
spelling from this release on. Entries for earlier releases keep the old names, because the
artifacts they describe really were called that.

- **Go module path is `github.com/Open-MBEE/OpenSysML`.** `go install
  github.com/Open-MBEE/OpenSysML/cmd/sysml@latest`; the old path resolves only for `v0.0.x`.
- **The binaries are unchanged** — `sysml`, `sysml-lsp` and `sysml-grpc` keep their names.
- **The Python client is `opensysml`**, on PyPI and as the import: `pip install opensysml`,
  `import opensysml`. Its environment variables are `OPENSYSML_*` (`OPENSYSML_GRPC_VERSION`,
  `OPENSYSML_STATE_DIR`, `OPENSYSML_GITHUB_REPO`, `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`,
  `OPENSYSML_REQUIRE_SERVICE`), the base error is `OpenSysMLError`, the generator entry point is
  `opensysml-generate`, the state directory is `~/.opensysml` and the release tag is
  `opensysml-v*`. Nothing reads the `pysysml` names, so a `~/.pysysml` left behind by an older
  install is dead weight and can be deleted. The first release under the new name is 0.3.0,
  carrying on from `pysysml` 0.2.0 rather than restarting, and `pysysml` gets one last version,
  0.2.1, which contains no client: it raises on import naming `opensysml`, so `pip install
  pysysml` reports the rename instead of resolving to the pre-rename 0.2.0. Pin
  `pysysml==0.2.0` to keep that release while migrating; nothing further is published under
  that name.
- **Release archives are `opensysml-<os>-<arch>.tar.gz`** (`.zip` on Windows), and the Homebrew
  formula is `opensysml`: `brew install Open-MBEE/tap/opensysml`. Assets already published under
  `v0.0.x` keep their old names.
- **The RDF extension namespace is `urn:opensysml:sysml:`**, still bound to the `sysx:` prefix. A
  `.ttl` file written before this release carries `urn:systemica:sysml:` properties, and reading one
  is refused rather than silently dropping what those properties said — re-export it from its
  notation source.
- **The non-normative math library is `OpenSysMLMathFunctions`**, in
  `OpenSysML Libraries/OpenSysMLMathFunctions.kerml`. A model that writes
  `import SystemicaMathFunctions::*;` must be updated; the unqualified `exp`, `ln`, `log` and
  `atan2` aliases are unaffected.
- **Environment variables are `OPENSYSML_*`** (`OPENSYSML_SMT`, `OPENSYSML_SMT_TIMEOUT`,
  `OPENSYSML_REQUIRE_SMT`, `OPENSYSML_REQUIRE_TRAINING_CORPUS`, `OPENSYSML_SMT_CORE_BUDGET`,
  `OPENSYSML_SMT_MAX_CONFIGURATIONS`). The
  `SYSTEMICA_*` names are not read.
- **The VS Code extension is `opensysml-sysml`** and its settings are `opensysml.server.path`,
  `opensysml.server.args`, `opensysml.server.enabled` and `opensysml.trace.server`, with the
  command `opensysml.restartServer`. Existing settings must be re-set under the new keys.

### Binding connector runtime semantics

- **Bindings declared in materialized type and usage bodies now propagate values in both
  directions**, including inherited and nested ends, with typed conflict and cycle errors;
  calc result bindings such as `bind result = x` are also evaluated. Package-owned bindings
  remain a documented limitation.

### A named control node is a member, and a chained binding declares none

- **`fork`, `join`, `merge` and `decision` register the name they declare**, the way `first`/`done`
  already do, so `first Jump then Land;` names a control node as source or target instead of
  reporting it unresolved. An unnamed control node declares no name and registers nothing.
 - **A binding's end no longer names the binding.** `bind a.b.c = d;` records `a.b.c` as a reference
   subsetting — the end it binds, not a name the binding answers to — so `%search` and the symbol
   table no longer carry a stray `c` in the binding's owner.

### A view renders

- **`%render <name>` turns a view's exposed set into the rendering its `render` member states**, and
  into a containment tree where it states none. The kinds produced are a tree (the exposed elements
  with their kinds and names, nested views as subtrees), an interconnection diagram (the exposed
  parts and the connections between them, read from the model's own connector and flow ends), a
  state machine (states and transitions), an action flow (nodes and successions) and a table (the
  exposed elements, what they declare and the views nested in the rendered one, as rows). State and
  action renderings read the lowered `StateGraph`/`ActionGraph` the runtime executes, so a rendering
  cannot drift from what runs.
- **`%render <name> <form>` writes the machine-readable form of the rendering**: `mermaid` for the
  graph-shaped kinds — `flowchart TD` for a tree or an action, `flowchart LR` for an
  interconnection, `stateDiagram-v2` for a state machine — and `markdown` for a table, which Mermaid
  has no grammar for. Either pastes into Markdown, a documentation site or an editor as-is, and text
  stays the default at the prompt. A form the kind is not written in is a typed error naming the one
  it is.
- **`sysml model.sysml -render <view>` renders without the prompt**: the artifact on stdout, the
  kind's machine-readable form by default and `-render-form text` for the read form, `-o` writing it
  to a file, and every human notice — what was loaded, an empty rendering, an element the rendering
  cannot represent — on stderr. Rendering decides nothing about the model, so it is not asked for
  together with `-convert` or a check flag.
- **Rendering is a read.** `%render` materializes no object, registers nothing in the session and
  leaves an `%action`/`%state` debugging session stepping the same graph and the same objects, so it
  can be asked between two `%step`s. `%view`'s report is unchanged.
- **The empty and error paths say what happened**: a view exposing nothing renders an empty artifact
  and says so, a rendering kind this build does not produce is a typed error naming the kind and the
  view rather than a substituted rendering, a name that is no view is `semantics.ErrNotAView` as
  `%view` answers, and an exposed element a rendering cannot draw is reported rather than dropped.
- The rendering itself is **tool-defined output**: SysML v2 §10.2 leaves rendering to the tool, so
  the notation is what is supported and the artifact is OpenSysML's own — recorded as such in
  [docs/project/spec-compliance.md](docs/project/spec-compliance.md).
- **An element reached twice is exposed and rendered once.** A wildcard or filtered `expose` walks
  the document's own scope tree and the global index, which build a symbol each for one
  declaration, so `expose P::*` and `expose P::**[@T]` used to show an element as many times as it
  was reached. The declaration a symbol was built from is now its identity, so exposure, filtering,
  rename and reference lookup all agree on when two symbols are one element.

### An object runs the behavior its type exhibits

- **Materializing an object starts the behaviors its type exhibits or performs.** An
  `exhibit state` machine written in a part definition is now that part's own machine: each object
  gets an execution of its own, so two objects of one type hold two current states, two event queues
  and two sets of feature values. Until now the body only parsed, resolved and lowered — running it
  meant `%state` on the state usage itself, detached from any object.
- **Identity carries through the run.** What an entry, `do`, exit or effect body reads and writes is
  the performing object's feature values, and a send addressed to an object reaches that object's
  machine and not a sibling's — a nested object now knows the object that owns it, so
  `send … to sibling` finds the sibling instead of materializing a second one.
- **Startup and quiescence are defined.** Feature values and constant defaults come first, so an
  entry action sees declared initial values; then the behaviors are initialized and run until
  nothing is due at the current time — a machine waiting on a timer is quiescent, and `%step` or
  `%advance` drives it. Cross-object messages are drained collectively, bounded by the state-event
  and do-step budgets, so a machine that never settles reports
  `object behaviors exceeded their budget` rather than hanging.
- **A second `%instantiate` of one name is a second object**, with its own identity and its own
  behaviors, and the command now says so (`note: P now denotes this object; object #1 is no longer
  named, with 1 behavior of its own`) instead of silently replacing the name. `occurrenceOf` is
  still the reuse path for a named occurrence.
- **`%invoke <object> <op> [<p>=<expr>]` runs an operation of the object's type**, performed by that
  object, binding arguments to its `in`/`inout` parameters by name and printing its outputs. Known
  limitation: only an action member is executable this way — an operation written as a `calc` or
  `constraint` is evaluated as an expression rather than performed, and reports that.
- **`%state <object>` attaches to the object's exhibited machine**, so `%step`, `%advance`,
  `%current`, `%events` and `%features` all describe that object. The object's identity and the debug
  session both survive an unrelated declaration.
- **A carried object's behaviors restart in the rebuilt analysis, and it is reported.** An execution
  belongs to the graph, names and message bus of the analysis it started in, so an object carried
  over an unrelated declaration keeps its identity but starts its behaviors again from their initial
  states — dropping what the discarded run wrote — with a `note:` naming what restarted. A `%state`
  session follows the object onto its restarted machine.
- **Rewriting a behavior drops the objects running it.** Re-declaring the machine or action an object
  runs changes what the object is, so the object itself is dropped with a reported reason instead of
  being carried over at all.
- **A feature holds the object it materializes before that object's behaviors start**, so two nested
  objects addressing each other reach one another instead of materializing a fresh copy per message
  until the event budget runs out.
- **A creation that fails leaves nothing naming what it removed**: a feature of a surviving object
  that reached one of the removed objects is read again, and messages addressed to them are dropped
  with them.
- **A `%state` session over a machine an object merely performs stays on that machine** across an
  unrelated declaration; only a session over the object's own exhibited machine follows a restart.
- **`%step` wakes a machine parked on a change condition**, so a condition made true from outside it
  — by `%invoke`, by another object, or by a later declaration — is dispatched instead of the machine
  reporting itself suspended forever.

### `%slots` is now `%features`, the name SysML v2 uses

- **`%features <name>` lists what an object holds for each feature of its type**, which is what
  `%slots` listed. "Slot" is UML/SysML v1 vocabulary (`InstanceSpecification::slot`); the v2/KerML
  pair for this concept is `Feature` and `FeatureValue`, and the listing's heading now reads
  `Features:` to match. `%instantiate` points at the new spelling too.
- **`%slots` is gone**, not kept as an alias: since it never shipped in a release, 0.1.0 takes the
  clean break rather than carrying the v1 spelling forward. Nothing else about the listing changed —
  the nested expansion, its bounds, the error lines and the exit status a non-interactive run takes
  from a feature value that could not be materialized are all as they were.
- **The vocabulary behind the command is v2 too.** What was printed and named as a "slot" is now a
  *feature value* (`FeatureValue`, `KerML.kerml`), and a state's `do` behavior is a *do action*
  (`States.sysml`) rather than a "do activity":
  - Message text changed: `feature value craft.volumes: multiplicity violation: …`,
    `cyclic feature value dependency`, `uninitialized feature value`,
    `no errors in the feature values checked`, and
    `state machine exceeded max do action steps (…)`. The `%budget` label reads `do action steps`.
    `SYSML_MAX_DO_STEPS` and every exit status are unchanged, but a script matching the old text
    needs updating.
  - The gRPC interface carries `Instance.feature_values` (`FeatureValue`) and the `feature_values`
    capability. `Instance.slots` and `SlotValue` are removed; field number 3 and the name `slots`
    stay reserved in `sysml.proto`, so the number is never reused. `opensysml` requires that
    capability before it hands back an object, so a service predating the rename — every published
    release does — is named rather than answering with an object that appears to hold nothing.
  - `opensysml` exposes `Instance.features`, `raw_features`, `get_feature`, `FeatureValueError`, and
    `typed.feature_value`/`optional_feature_value`/`list_feature_value`, which generated modules
    emit (emission schema `3`). The `slots`, `raw_slots`, `get_slot`, `SlotError`, `slot_to_python`
    and `slot` decoder spellings are removed.
  - The Go runtime API is renamed to match (`runtime.FeatureValue`, `Instance.FeatureValues`,
    `GetFeatureValue`, `FeatureValueError`, `ErrFeatureValueMaterialization`, `ErrCyclicFeatureValue`,
    `ErrUninitializedFeatureValue`). It is internal, so nothing outside the module depends on it.

### The prompt prints the model it holds

- **`%print` writes the session's model back as SysML notation at the prompt**, which until now
  needed `%save <file>` and another program to open the file. `%print <name>` prints one element
  and its body instead of the whole buffer, taking the quoted and qualified spellings every other
  command takes (`%print 'My Pkg'::Car`, `%print Top::'My Pkg'::Car`), and tab completes both the
  command and the names after it.
- It is the writer `%save` writes `.sysml` with — `export.SysMLElement` renders one element's
  source through the same `format.Source` path a whole-document save goes through — so comments and
  the text as typed survive, and a print submitted again rebuilds the same model. Notation only: no
  RDF notice follows a print.
- Printing is a read. No object is materialized, `%instances` and the buffer are unchanged, and an
  `%action`/`%state` debugging session keeps running across it. An empty session, a name nothing
  declares, and a symbol this session holds no source of (a library name) each answer in one line.

### An SMT solver decides what a model's conditions permit

The whole path is **experimental** and every surface says so: the vocabulary of the reports may
change, and a solver is optional at runtime — discovered on `PATH` or named by `OPENSYSML_SMT`,
with a build that has none reporting that rather than a verdict.

- **`%check <name>` asks an external SMT solver whether a constraint, requirement or satisfaction
  assertion *can* be satisfied**, and prints an assignment on `sat`. Conditions are translated to
  an SMT-LIB 2 script — one variable per logical feature with injective symbols, quantities in
  named base units, and truncating integer division whose well-definedness guard is hoisted only
  where the division always runs — and `sat`, `unsat` and `unknown` stay three distinct verdicts.
  Satisfiability is not evaluation: `%constraint` and `%satisfy` still answer what holds of an
  object.
- **`%explain <name>` says which conditions conflict** behind an `unsat`: an unsat core reduced to
  a minimal one by dropping a member at a time in fresh solver processes, bounded by member count
  and `OPENSYSML_SMT_CORE_BUDGET`, printed as the role, the condition as written, the declaring
  element and `file:line:col` in the query's assertion order. A declared domain (a `Natural` being
  non-negative) or a division guard can be the conflicting condition, a one-member conflict says it
  is the whole conflict, and a core that was refused, unreadable, empty, repeated or never issued is
  a typed `CoreError` rather than a shorter core presented as minimal. The time reported covers the
  reduction, not just the first verdict.
- **`%solve <name>` synthesises values that satisfy an assertion**, keeping fixed what already is —
  the values an object holds, else the ones the model declares — and reporting what was fixed and
  by whom, the values chosen, and that they are one witness of possibly many. `unsat` there means
  no values exist consistent with what is fixed, and names the fixed values that conflict; an
  object's fixed values survive an unrelated submission.
- **`%configure <name>` answers which variants an assertion permits**: with no argument one
  consistent selection, with `<variation>=<variant>` the named selection checked and the conflict
  named where it is not consistent, and with `all [<count>]` the selections enumerated up to
  `OPENSYSML_SMT_MAX_CONFIGURATIONS`. The report says whether they are all of them or were cut
  short — at the bound, or because the solver stopped deciding or ran out of time, in which case
  the selections found so far are still reported. An element that reads no variation point is an
  error pointing at `%check`.
- **`%optimize <name>` improves the `objective`s an `analysis def` states**, which until now parsed
  and then sat inert: the direction comes from the trade-study definition typing it
  (`TradeStudies::MinimizeObjective` or `MaximizeObjective`), the value from the expression the
  objective states for the library's `best` feature, and feasibility from the case's own conditions
  together with each objective's — all read through the runtime's own surfaces, re-parsing no
  declaration. Several objectives are improved lexicographically in declaration order, with
  `(set-option :opt.priority lex)` written into the script rather than left to a backend default,
  and every optimum is verified by asking whether anything does better, so an attained optimum, an
  unbounded objective, a bound no assignment attains and an answer that could not be verified stay
  four different reports and none of them fabricates a number.
- **What a query needs of a backend is modelled as capabilities**, probed once per executable (or
  declared by the caller) and cached, so a feature the backend lacks is an
  `UnsupportedCapabilityError` naming the backend, the feature and the operation instead of a silent
  degrade or a fabricated verdict. A query emits the narrowest standard SMT-LIB 2.6 logic it needs,
  falling back to the non-standard `ALL` only for datatypes and strings, which the standard logics
  cover with nothing. Optimization is a z3 extension, so cvc5 is refused there rather than answering
  a plain `check-sat` presented as an optimum. "Lacks the feature", "cannot be run" and "did not
  decide" are three distinct reports, an undecided probe settles nothing, and a reply SMT-LIB does
  not define — `maybe` — is a `SolverProcessError` about the executable rather than a capability
  refusal.
- **The path is gated in CI both with a solver and without one.** A differential gate requires the
  solver's `sat`/`unsat` to agree with the evaluator's verdict over the conformance corpus, the
  standard library, the OMG training corpus and deterministic randomized models — it found and
  fixed a real division by zero answered as an infinity, and a redefining variation usage given a
  sort of its own — and a portability harness reports pass/refuse/fail per capability against
  whatever `OPENSYSML_SMT` names, wired to both z3 and cvc5.
- `brew install opensysml` brings z3 along, so the path works out of the box on a Homebrew
  install, and the install guide, troubleshooting page, environment reference and REPL command
  reference each say how to get a solver otherwise.

### A model is edited through the source it was parsed from

- **`ApplyEdits` edits a loaded model by rewriting the bytes of its own source.**
  `internal/core/edit` is a span-level engine that sets a feature's value or renames a declaration
  and leaves every untouched byte identical, so comments and the text as typed survive an edit the
  way they survive a save. The edited source is re-parsed and re-analyzed before it is handed back,
  and the edits of a request are applied all of them or none. The source edited is the one the parse
  read, named by its hash, so a file changed since then is refused rather than edited blind.
- **An edit is judged only by the tiers the original's parse reached.** A model whose parse had
  errors was never analyzed, so its semantic baseline was empty and every pre-existing name or type
  error counted as one the edit introduced — refusing a good edit to a file with a syntax error
  elsewhere. Renaming a referenced element, and creating or deleting an element, are refused with a
  typed error rather than approximated.
- **`opensysml` exposes it as `model.edit()`** — `set_value(target, value)`, `rename(target, name)`
  and `apply()`, whose result saves the way a `Conversion` does — behind the `apply_edits`
  capability, so a service too old to offer it says so instead of failing as an unimplemented
  method.

### Resolution cost

- **Resolving a feature chain is linear in its length**, where a chain's prefixes used to be
  re-resolved per segment, and an operand of a chain that is no reference is preserved rather than
  dropped on the way.

### A model that states behavior converts to RDF

- The behavioral nodes of an action or state body now have metaclasses and the properties their
  notation is rebuilt from, so a model stating steps converts instead of being refused: the
  initial and final node, `perform`, `send`, `accept`, `terminate`, `assign`, the
  fork/join/merge/decision control nodes, `while`/`loop`/`for`, `if`/`else`, and the state
  machine's states, substates, regions, `entry`/`do`/`exit`, `defer`, pseudostates and
  transitions. Each category is covered by a `notation → RDF → notation` round trip asserting the
  body comes back byte-identically. The mapping is tabulated in
  [the RDF mapping](docs/reference/rdf-mapping.md) § Behavior; terms the OMG vocabulary has no
  counterpart for are named under the `sysx:` extension namespace.
- **102 of the 120 models under `examples/` now convert to Turtle, up from 71.** The remaining 18
  are refused with the node named, not partly converted: nine successions that do not name both
  of their ends, three prefix-metadata models, three duplicate declarations, two
  operator-expression members and one anonymous `snapshot`.
- A shorthand relationship no longer collides with the member it names: the `result` of
  `bind result = x;` and the `x` of `first x;` are carried as references to that member rather
  than as a name the element declares, which is what made those models fail as duplicate
  declarations.
- Two things the mapping used to lose quietly now survive it: a metadata annotation
  (`#Safety part def Car;`) is carried as the notation it was written as, and a feature that wrote
  no kind keyword (`in x : Real;`) comes back without one instead of gaining the kind's canonical
  keyword. The two annotation shapes that still cannot be written back — one carrying a body, and
  an `@` annotation the parser records on the declaration ahead of the one it prefixes — are
  reported with the line named.
- Two more shapes the mapping used to change quietly now come back as written: a combined state
  subaction keeps its `do` whatever separates it from its body (`entry do{ … }`), and a kind
  keyword named inside a comment in a declaration's head (`in /* attribute */ x : Real;`) is read
  as the trivia it is rather than as a keyword the author wrote.
- RDF conversion remains **experimental** — the vocabulary may still change, and no round trip
  through a running triplestore has been demonstrated (roadmap D1–D3).
- Every surface now states that status in one wording again: `-help` prints
  `export.ExperimentalNotice` rather than a copy of it, and the Python client's fallback notice,
  the `ConvertResponse.experimental` comment and the guide pages were brought back in step. A test
  pins the Python copy byte-identical to the constant, since Python cannot import it.

### A nested value body governs over an inherited value

- **A redefining declaration whose body values features holds those values.** `part def Ring
  { attribute cost : Cost = template; }` re-opened as `part r : Ring { attribute :>> cost
  { attribute :>> v = 11.0; } }` now reads `r.cost.v` as `11.0`: the more specific declaration of
  the feature governs (KerML 1.0 §7.3.4.5), where the inherited value used to win and the body's
  restatements were dropped with no diagnostic. A feature the body does not value takes its type's
  own default, the inherited value binding nothing there — the body supersedes that value rather
  than merging with it, since a `FeatureValue` binds a feature as a whole.
- Unchanged: a body on the *same* declaration that writes the value still reports
  `ErrValuedFeatureRestated` (two values, neither more specific), and a body that only re-declares
  features (`attribute :>> kept { attribute :>> v; }`) still reads the inherited value — at any
  depth of nesting, a value stated anywhere in the body being what makes it govern.
- **A check made without an object agrees with materializing one.** A condition naming a feature a
  body governs over reads it as uninitialized rather than against the superseded value, so the same
  model no longer passes or fails a check depending on whether an object was built for it, and an
  edit confined to a governing body changes the type's shape, so a carried-over object is
  re-materialized instead of keeping the value the body replaced.

### Names nested inside a `require`/`assume` body are resolved

- **A `require`/`assume` body now resolves to whatever depth it is written to.** Where only its
  direct members resolved, a declaration nested in it (`require Q::r { part p : P { :>> f; } }`)
  had its own body left unwalked, so a typo there produced no diagnostic at all; such a name is
  now resolved and, if it names nothing, reported at its own span.
- The body is a scope of its own: what it declares is visible to what nests inside it but is no
  member of the namespace that declares the member, and the referenced requirement's features stay
  offered to the body's direct members, which is what the reference subsetting inherits them to.
- **Every tier reads the body in that same scope.** Type checking and condition evaluation walked
  such a body in the *enclosing* scope, so a value written there was typed against the wrong set of
  names — silently missing a genuine type error, or judging the name against an unrelated
  declaration outside the body — and a condition stated in the body could not read a name the body
  declares.

### An unimported OpenSysML extension function no longer answers a call it is reported unresolved on

- **`exp`, `ln`, `log` and `atan2` now require the import that declares them.** They are declared by
  the non-normative `OpenSysMLMathFunctions` extension, which no OMG library carries, so a bare
  `exp(x)` is reported `unresolved reference: exp` — and used to be evaluated anyway by dispatch on
  the local name, which meant the diagnostic and the behavior disagreed: ignore the error and the
  model computed, trust it and the model looked broken. Such a call now fails with a typed error
  (`ErrUnimportedExtensionFunction`) naming the function and the `import OpenSysMLMathFunctions::*;`
  that makes it legal.
- **A model that imports the package, or writes the call qualified, is unaffected**, as is a bare
  call to an OMG function library (`sqrt`, `sin`, …), which every model may write whatever it
  imports.
- **`%builtins` still lists them, marked with the import they need.** Dropping the four names from
  the unqualified-dispatch registry also dropped them from the listing and from name completion,
  which made implemented functions look unsupported; the listing is now taken from what the build
  implements, and an extension function is listed with a `(needs import OpenSysMLMathFunctions::*;)`
  marker rather than silently omitted.
- **A root-level import in a document that declares nothing else now surfaces its names**, which is
  what a bare `import OpenSysMLMathFunctions::*;` at the REPL prompt is: the editor's own scope tree
  is identified by the document name stamped on it, and a document with no member had no symbol left
  to carry that name, so its own import was read as another document's private re-export.

### The corpus notation two verdicts were open on is adjudicated legal

- **A conjugated end (`end spacePort : ~CommunicationPort`) and a portion prefixed onto a kind
  keyword (`timeslice item item1`) are legal, and are accepted.** `ConjugatedPortTyping`
  specializes `FeatureTyping`, so any feature typing — a connection or interface end among them —
  may name a conjugated port definition, and `PortionKind` is an attribute of `OccurrenceUsage`,
  which an item usage is. Both are pinned clean over every validation tier by
  `testdata/passes/corpus_notation.golden`, so a regression is caught as the false positive on a
  flagship model that it would be. What the Open-MBEE models still report is other notation: the
  OMG-side `end;` outside an interface body and `'SysML Standard Diagrams'::gv`, and — in
  `DesertKite.sysml`, which lives only on that repository's `InitialDesign` branch — 7 errors that
  are ours: a qualified name refused as a `bind` end and `connection connect … ;` refused inside a
  `requirement` body, both owned by a separate session.

### A binding end may be a qualified name, and a requirement body takes a prefixed connector

- **`bind` now accepts the notation a connector end is written in:** `binding bind R::a = c;` and a
  feature chain whose chaining features are themselves qualified names
  (`bind 'Kite Environment'::'Region Earth Surface'.'Kite System'::'Desert Kite'.'Wall Height' = …`)
  parsed as far as the first `::` and then failed. A connector end names a feature by a
  `QualifiedName` (SysML `ConnectorEndMember` → KerML `OwnedReferenceSubsetting`,
  `OwnedFeatureChaining`), and every segment of the chain is now recorded and resolved, not
  collapsed to its last one.
- **The named form is no longer read as a redefinition:** `binding b1 bind R::a = c;` reported
  "b1 redefines a, but a is not an inherited member of P". `b1` is the binding's name and `R::a` its
  first end, which reference-subsets the feature it names, so it resolves where that feature is
  declared rather than as an inherited member of the binding's owner.
- **A connector, flow or message written with its kind keyword is a member of a requirement-like
  body:** `requirement r { connection connect r to x; }` reported `expected a body member` at the
  closing brace. A `requirement`, `constraint`, `concern`, `objective`, `use case` and `view` body
  admits usage elements, and a connector usage declares no name — its ends are what make it a
  declaration.
- The Open-MBEE `DesertKite.sysml` model parses clean as a result. What it still reports is
  recorded in [spec compliance](docs/project/spec-compliance.md) § Structural, Interface and
  Analysis Notation: the tool-specific `'SysML Standard Diagrams'::gv` namespace, an `OOSEM::MOE`
  reference to a member of `OOSEM::'OOSEM Measures'`, and references to a decision node whose name
  is not registered as a symbol.

### RDF conversion is experimental

- SysML ↔ RDF Turtle conversion is labelled **experimental**, in both directions, on every
  surface that offers it. The mapping covers a model's structure and behavior but not its
  expressions, a model it cannot write is refused with the construct named (the counts are
  above), its vocabulary may change without a compatibility path, and no round trip through a
  running triplestore has been demonstrated (roadmap D1–D3, D6). Saving and converting notation
  (`.sysml`, `.kerml`) is stable and unchanged.
- `sysml -convert` writes the status as a `note:` on **stderr**, so a conversion piped to a file
  or to stdout carries no extra bytes, and a refused conversion is labelled too. `-help` says
  the same.
- `%save model.ttl` prints the status before it writes, including when the model is refused.
- `ConvertResponse` carries `experimental` and `experimental_notice`, set before the conversion
  runs, so a client reads the status off a refusal as well as off a success. The `convert`
  capability is unchanged: the status is per conversion, not per service.
- `opensysml` raises the status as `ExperimentalFeatureWarning` and exposes it as
  `Conversion.experimental`/`.experimental_notice`, plus `opensysml.is_experimental(from, to)`.
  A service too old to send the fields is read from the formats it reports instead, so an RDF
  conversion warns either way. Silence it with
  `warnings.simplefilter("ignore", opensysml.ExperimentalFeatureWarning)` — no stable feature
  warns with that class.
- The wording lives once, in `export.ExperimentalNotice`, so no surface can drift from another.

### Documentation

- The RDF mapping reference opens with a **Status: experimental** section stating what the
  mapping covers, that the vocabulary may change, and that interoperability is unverified.
- The claim that a converted graph "loads into" Flexo MMS's triplestore is withdrawn from the
  reference and from `docs/project/spec-compliance.md`: the vocabulary and element IRIs match
  Flexo's `Namespaces.kt`, which is an addressing claim, not a demonstrated load.
- The README capability table splits notation save (complete) from RDF conversion
  (experimental), and the guide, CLI and REPL references, Python guide and roadmap say the same.
- Two example models come with a walkthrough of the commands that exercise them:
  [`examples/solver-demo.sysml`](examples/solver-demo.sysml) for `%check`, `%explain`, `%solve`,
  `%configure` and `%optimize`, and [`examples/views-demo.sysml`](examples/views-demo.sysml) for
  `%view` and `%render` across the five rendering kinds and the text, Mermaid and Markdown forms.

### Release process

- The CircleCI pipeline that builds release tags downloads the OMG training corpus and runs the
  suite with `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, so the corpus gate can no longer skip
  silently where a tag is cut. 0.0.9 listed that as a known limitation; it is closed.

### Known limitations

- Of what 0.0.9 listed, the untested tag pipeline is closed above and the RDF refusal of a model
  stating behavior is closed by the mapping's behavior coverage. What stands: expressions are not
  emitted as triples and end-binding heads depend on `sysx:sourceText`, so RDF conversion is
  stated as a feature status rather than a footnote; a `that` written inside a nested `action`,
  `constraint` or transition-guard body binds to the innermost enclosing usage; an unqualified
  standard library name requires an import, which is conformant and recorded as won't-do; and a
  port that accepts TCP but never answers gRPC costs the Python client about 9 s rather than the
  nominal 2.5 s `START_TIMEOUT`.
- A package-owned binding connector does not propagate values; a binding declared in a
  materialized type or usage body does.
- Constraint solving is experimental and needs an external solver: `%check`, `%explain`, `%solve`
  and `%configure` want z3 or cvc5, and `%optimize` wants z3, since optimization is a z3 extension
  cvc5 does not implement. A condition the translation has no SMT-LIB form for refuses the whole
  query rather than dropping the condition, and the guide covers the commands by reference rather
  than in a chapter of its own.
- An edit sets a feature's value or renames a declaration; creating or deleting an element, and
  renaming one that is referenced, are refused.
- Only an action member of a type is executable through `%invoke`; an operation written as a
  `calc` or `constraint` is evaluated as an expression and reports that.
- A rendering is tool-defined output (SysML v2 §10.2 leaves rendering to the tool), so what
  `%render` and `sysml -render` produce is OpenSysML's own notation rather than a standard
  interchange form.

## 0.0.9 — 2026-08-17

### Language and semantics

- A transition leaving a composite state fires while a substate is active, where it used to
  never be taken, and a transition between sibling regions exits only its source rather than
  the whole composite state; history is recorded per region.
- Succession and transition endpoints (`a then b`, a transition's source and target) are
  resolved at the name-resolution tier, in the scope they were written in, instead of being
  matched against a flat list of states and silently dropped where no match was found. An
  endpoint naming a vertex of another state machine, or a named `first`/`then` marker, is a
  check-time diagnostic rather than a failure to construct the executor; an endpoint no pass
  reported leaves its own edge out instead of failing the lowering.
- A `send` reaches its target by port direction, conjugation and the performing part, so a
  message declared on a conjugated port arrives where the model says it does and a state
  machine nested in a part reaches that part.
- A block owns its token flow: `for` iterates every collection it is given, an output the
  block's own flow assigns is counted, a result written among its flow nodes is returned, and
  a `for` over a non-collection is reported rather than iterated once.
- Library evaluation covers string operators and `StringFunctions`, `VectorFunctions` and
  `ComplexFunctions`, `@`/`@@` classification, a queryable exposed element set for views,
  library feature values read as names (`TrigFunctions::pi`, deg/rad) and `includingAt`
  insertion. A vector element or inner product beyond the `Real` range, and an argument named
  for no declared parameter, are reported rather than wrapped or ignored.
- An enumeration literal is a value, in the runtime and across the API.
- The subject of a check or evaluation is chosen deterministically and reported: keyed by
  declaration path rather than holder identity, bounded in its search, routed through
  `satisfy`, with the objects of one declaration counted once, an object held by another not
  a subject root, a nested definition's objects among them, and a nested redefinition on an
  object eligible. An ambiguous carrier is named by its definition, and a nested subject is
  named in verdicts, labels and over the wire.
- A calc body with an implicit result is type-checked, and calc recursion evaluates under a
  budget instead of exhausting the stack.
- A multi-valued default is honoured where it conforms and reported where it does not, and a
  default whose multiplicity is not declared is held to the assumed `1..1`.
- An accept node's payload resolves in its action body, a nameless payload no longer masks
  the feature it is named after, and shared payload visibility is limited to action bodies.
- Parser and classification: modifier-driven usage kinds, keyword-named parameters and loop
  variables, a classifier specializing any definition, a KerML datatype classified as a
  definition while `function` stays a calc, recursive `expose` traversal preserved through
  filtered namespaces, and classification judged by element name across index generations.

- A valueless feature of a value type reads as unset rather than as an empty
  object: `attribute d : Real;` reports `d = <unset>` where it used to report
  `d = Instance(ID: 2)` with `(no features)`. What materialization creates is
  unchanged — a `Real` has no features to instantiate, so the object it holds is
  empty — but every surface that reports a value now says so with one spelling:
  `-instantiate`/`-e`, `%slots`, the JSON report, and the wire, where
  `Value.unset` is a new arm the service sends and refuses to accept. A valued
  attribute (`k = 2.00`), an object of a class, and a value type that does
  declare features are unaffected.
- A member chain from `that` resolves: `attribute b : Real = that.a;` in a usage
  body reads `a` off the object featuring the value being written — the innermost
  enclosing usage, whose own and inherited members are both reached — instead of
  reporting `no scope for member lookup in Base::things::that`, since `that` is
  declared `Anything[1]` and owns no members ([KerML, 8.4.2]). A `that` written
  where no usage encloses it stays unresolved rather than resolving to the
  library's declaration.
- A root-level `private import X::*;` serves the document that wrote it. It was
  hidden from that document too, so a file opening with `private import
  ScalarValues::*;` — the spelling OMG's own training files use — reported `Real`
  unresolved. A root-level import still reaches no other document, at any
  visibility ([KerML, 8.2.3.3]).
- The import an editor offers for an unresolved library name is written `private
  import X::*;` explicitly, so applying the fix does not re-export the imported
  names onward.

### Tools

- `sysml-lsp` accepts `--stdio` and implements the `shutdown`/`exit` lifecycle, so the
  shipped VS Code extension can start the shipped server; it used to exit 2 and crash-loop.
- The REPL closes a cluster of usability gaps: unclosed submissions, load diagnostics,
  object-resolved subjects, `%view`, ranked suggestions, quoted qualified names, a pinned
  `%eval` context, reported reset loss, and an unreadable name reported as typed.
- A piped REPL session whose command could not materialize a slot exits 2, so a script can
  detect it, and the CLI reports materialization diagnostics instead of swallowing them.

### gRPC service

- A quantity crosses in both directions with its magnitude, the unit as written and the
  reduced unit term, and is read as a typed Python value. An unreduced unit, a zero unit
  scale and a named unit arriving without its reduction are rejected.
- `Evaluate` is subject-aware behind a capability, attributes are populated by following
  typing edges, and generalization bases are reported.

### Performance

- A cold `ParseFile` is served from a pool of prewarmed standard library indexes, taking
  under 1 ms where it used to rebuild the index per distinct model at ~110 ms. Each cache
  store writes its own temp file and the pool refills serially.

### Tests and documentation

- `cmd/sysml-grpc`, a published artifact that had no tests, is gated on process lifecycle —
  start, one RPC, shutdown, and the failure exits — along with the subtle `resolve` and
  `semantics` rules that were uncovered.
- The gate figures are counted at the first-subtest level and each has exactly one home;
  every other page links to it rather than restating a number that drifts.
- Behavioral semantics cite SysML v2 and KerML rather than UML 2.5.1.

### Python client (`pysysml`)

- `pysysml.UNSET` is what a slot holding no value reads as — falsy, spelled
  `<unset>`, and distinct from `None`, the model's `null`.
- A quantity can be *sent*, not only read: a `pysysml.values.Quantity` is
  accepted wherever a value is — an action input, a calc argument, an element of
  a sequence — and crosses as `Value.quantity` with its magnitude in the kind it
  was written in, the unit as written and the reduced unit term, so a quantity
  read from the service round-trips through an evaluation with both magnitude and
  unit preserved. A unit named without the reduction commensurability is decided
  over is refused before anything is sent, rather than compared by bare
  magnitude.
- A `Connection` that starts the service asks it at once and then backs off (10
  ms, 20 ms, 40 ms … capped at 250 ms) instead of sleeping half a second before
  the first probe, so starting a service that answers in milliseconds costs ~17
  ms rather than ~510 ms. Waiting is bounded by the same ~2.5 s and raises the
  same `ConnectionError`, now as the documented `connection.START_TIMEOUT` and
  covering the probing as well as the sleeping, so a port that accepts without
  ever answering no longer costs a whole probe timeout beyond the bound; a
  service that died is still detected before each probe and ownership,
  stale-service and pid authentication are unchanged.
- The two names that shadowed builtins are renamed: `pysysml.eval` is
  `pysysml.evaluate` and `pysysml.RuntimeError` is `pysysml.ExecutionError`. Each
  old name still resolves to the same object with a `DeprecationWarning` and is
  gone from `__all__`, so existing snippets keep working while a star-import
  shadows neither builtin. The `Model.eval` and `Connection.eval` methods are
  unchanged.
- A release this `pysysml` pins no digest for raises the new
  `UnpinnedReleaseError` instead of `ChecksumMismatchError`, which named the
  wrong cause. It subclasses `ChecksumMismatchError`, so an `except` clause
  written before it existed still catches it, and only it may be answered from a
  cached binary — a contradicted digest still never is.
- `pysysml.__version__` reports the declaration shipped beside the module, so an
  editable install whose checkout bumped `VERSION` after `pip install -e` no
  longer reports the version it had at install time. The version tests locate the
  installed package through the install's own PEP 610 record, which for an
  editable install is the checkout rather than a site-packages path holding no
  `pysysml/`.
- The generated protobuf stubs ship type annotations (`sysml_pb2.pyi`, generated
  by `make python-proto`), so `mypy` no longer reports the message classes and
  enum constants as undefined.

### Known limitations

- Two of 0.0.8's four listed limitations are closed by this release (the nested
  redefinition as a subject, and the unchecked implicit-result `calc` body). The RDF
  limitation stands: expressions are not emitted as triples and a model whose behavior is
  stated as action or state nodes is still reported rather than converted, so the RDF path
  should be read as experimental.
- A `that` written inside a nested `action`, `constraint` or transition-guard body binds to
  the innermost enclosing usage, so `that.k` naming a member of the enclosing part is
  unresolved. This is what the spec text says as written; the outward binding is not
  implemented.
- An unqualified standard library name still requires an import (`private import
  ScalarValues::*;`, the spelling OMG's own training files use). Only the public top-level
  elements of a root namespace are globally visible ([SysML, 7.2] over [KerML, 8.2.3.5]), so
  this is conformant rather than a gap, and is recorded as won't-do.
- A port that accepts TCP but never answers gRPC costs `pysysml` about 9 s of wall clock
  rather than the nominal 2.5 s `START_TIMEOUT`. The wait is bounded and raises a clear
  `ConnectionError`.
- The tag pipeline (`.circleci/config.yml`) does not download the OMG training corpus, so
  the corpus gate does not run there; it was run locally for this release.

### Release process

- `build-release` fails a release whose built artifacts do not report the tag
  they were cut from, before anything is stored or published.
- `python/scripts/pin_release_checksums.py` fails with a typed
  `MissingTokenError` naming `GITHUB_TOKEN`/`GH_TOKEN` when neither is set,
  instead of an opaque rate-limited HTTP 403 from an unauthenticated request; the
  scope it needs is documented in the release runbook.

## 0.0.8 — 2026-08-15

### Language and semantics

- A multi-valued feature that is both typed and given a default holds the
  default's values rather than an instantiation of its type: `attribute xs :
  Real[3] = (1.0, 2.0, 3.0);` materializes those three elements, a
  `part`-typed collection holds the very objects its default names, an
  expression default holds what the expression produced, and a quantity keeps
  its unit. A default whose element count does not conform to the declared
  multiplicity — one value against `[3]`, four against `[3]`, `()` against
  `[1..3]` — is a multiplicity violation, reported statically where the count is
  a literal one and when the slot materializes where only evaluating the
  expression knows it, rather than broadcast, padded or silently dropped. A
  feature whose multiplicity a redefinition does not restate is bound by the one
  it redefines. This was the second known limitation listed for 0.0.4 and 0.0.5.

### Diagnostics

- A comparison or sum of quantities whose dimensions are both statically
  determined and incommensurable (`mass < 1000.0 [m]`) is reported as a
  type-tier warning at validation time, from the stdlib `QuantityDimension`
  power factors, instead of only when the expression is evaluated. Evaluation
  keeps its hard error and a warning changes no exit status; a dimension a
  declaration does not determine stays unknown and is not reported.

### REPL

- A check of a condition declared on a definition is answered about the object
  that carries it, so `%constraint`, `%requirement` and `-constraint` on an
  instantiated model report the object's values rather than the declaration's
  defaults — a violating model used to be answered `✓ passed` with exit 0.
- `%eval` reads the object carrying the feature when the session holds one, so a
  check and an `%eval` in the same session no longer answer about different
  subjects; where several objects carry the feature it refuses to choose.
- A condition whose evaluation could not be carried out is worded as undecided
  (`? … could not be evaluated`) and names why, keeping exit 2, where it used to
  print a failure while exiting 2.
- A submission the parser cannot close — an unterminated body, block comment or
  quoted name, typed or in a loaded file — no longer absorbs the submissions
  after it: it is reported, kept in the buffer for `%list` and `%save`, and
  masked out of the text the session analyzes, so the next declaration parses
  and resolves as it would have before the bad one.
- A loaded file's syntax errors are printed the way a typed submission's are,
  against that file and its own line numbering, and count as errors for
  `HasErrors`, so a non-interactive run over a broken file fails instead of
  reporting nothing.
- An expression whose subject is reached through a declaration is evaluated on
  the object in effect for it, so `%eval Spec::c` honors a redefinition made on
  a nested object; two objects carrying the feature are still refused rather
  than chosen between.
- Two loaded files that open the same package are told apart explicitly: each
  opening stays a declaration of its own, both openings' members resolve
  qualified, and the load says to qualify a reference across them. Re-typing a
  package at the prompt still folds into the package already in the session.
- `%view <name>` is implemented, listing what a view exposes — its own `expose`
  relationships and the protected ones of the views it specializes — and the
  views nested in it; asking it of an element that is no view says so.
- The qualified names offered for an unresolved name are ranked and capped:
  what the session declares before the library, a package's member before a name
  nested in another element, and at most three, where an unresolved `length`
  used to list every same-named library member including function parameters.
- A `%satisfy` verdict quotes the inner names of the assertion it reports, so a
  requirement or subject whose name the notation quotes reads back as written.

### `sysml` command line

- A lone `-` names standard input wherever a model path is taken, `-convert`
  included, and is reported as `<stdin>`; it is read even when stdin is
  `/dev/null`, and stays distinct from a file named `-`.
- `sysml-lsp` parses its command line with the `flag` package, so `-version`
  works and an unreadable flag is a usage error rather than protocol mode.

### Editor support

- `textDocument/semanticTokens/full` and `/range` are implemented, over a new
  `internal/core/highlight` package, and `textDocument/codeAction` answers
  quick fixes carried as structured edits from the layer that reported the
  diagnostic — a located semicolon, a near-miss spelling, an importable
  namespace. Token deltas are not implemented and are not advertised.

### RDF interoperability

- The members that state a condition — a constraint body's conditions, a
  requirement's assumptions and required conditions, a subject and a result —
  have a mapping, so converting a model with a constraint no longer aborts.
  Conditions are carried as `sysx:condition` notation, as every
  expression-valued position in this mapping is.
- Turtle written back as SysML spells the notation: an unrestricted name gets
  its quotes, so a model with a quoted name re-parses.

### Python bindings and `sysml-grpc`

- `sysml-grpc` loads the standard library ahead of the requests that need it
  instead of once per model: the service keeps a small pool of prewarmed library
  indexes, and a model the service has not seen adds its document to one of them
  rather than loading and expanding the library again. A cold `ParseFile` on a
  163-line model measures ~0.5–0.9 ms where it measured ~100–128 ms, which is
  what makes a parameter sweep varying the model text practical. What a model
  resolves against is unchanged: an index carries the same library, an index is
  handed out once so cached models stay independent, and an empty pool builds one
  on the request path, so a result never depends on prewarming. Prewarming runs
  in the background, so startup stays prompt, and `SYSML_GRPC_INDEX_POOL` sizes
  the pool (default 4; 0 keeps the previous per-model behaviour).
- The library record cache writes each store to a temp file of its own, where two
  stores of one key shared a fixed `<key>.idx.tmp` path and could publish a
  truncated record that every later start missed on; `Prune` now also clears the
  temp files a crashed store left behind.
- `sysml-grpc -version` reports the metadata the linker sets, where a released
  binary said `version dev / commit unknown`.
- A cached `~/.pysysml/bin/sysml-grpc` records the release and repository it was
  downloaded from beside it, and a cache from another release is replaced rather
  than served. A failed integrity check is its own `ChecksumMismatchError` and
  is never answered from the cache; a download that fails on the network keeps
  the working binary.
- A service already listening is asked what it is and compared against the
  release and capabilities asked for, raising `StaleServiceError` naming the
  remedy instead of a `MissingCapabilityError` on the first newer call. It is
  stopped only when this client started it and no other client holds it.
- `Model` gained `instantiate`, `execute_action` and `execute_state`, so every
  call taking a model hash is reachable on the model it is about. `pysysml`
  0.2.0 carries these.
- `ChecksumMismatchError` is exported from `pysysml`, where it was reachable
  only as `pysysml.errors.ChecksumMismatchError` while every other documented
  exception was on the package.

### Documentation

- The pages are organized by what a reader is doing rather than by the feature
  that landed: a numbered handbook under `docs/guide/`, looked-up material under
  `docs/reference/`, design and internals under `docs/internals/`, and status
  under `docs/project/`. `QUICKSTART.md` and `RDF_INTEROP.md` are split into the
  chapters they were, the guide content stranded in `examples/*.md` and
  `python/README.md` is folded in, and the paths the released README linked leave
  pointers behind. `scripts/check-doc-links.py` gates every relative link and
  heading anchor in CI.

### Release automation

- Release assets are published with `ghr -replace` rather than `-delete`, which
  is an alias of `-recreate`: it deleted the release *and* its tag ref and
  recreated it empty, wiping hand-written release notes, title and the
  prerelease/latest flags on every re-run of the workflow for a tag.
- The Homebrew tap updates itself from a scheduled workflow in
  `Open-MBEE/homebrew-tap`, reading the latest release's `SHA256SUMS.txt`, with
  `scripts/render-homebrew-formula.sh` left as the manual fallback.

### Known limitations

- Converting a model whose behavior is stated as action or state nodes to RDF
  still reports the node and aborts (initial nodes, `perform`, `send`,
  `terminate`, loop nodes, state regions): 71 of the 120 models under `examples/`
  convert.
- A nested feature redefined on an instantiated object is not yet the subject of
  a check or an `%eval`, so those answer about the declaration while `%slots`
  shows the instantiated value.
- A `calc` body written without `return` is not expression-type-checked, so no
  static dimensional warning is reported inside it.
- Submitting a declaration the debugger depends on ends an active `%action` or
  `%state` session; a submission that changes something else carries it over.

## 0.0.7 — 2026-08-15

0.0.6 was tagged from this section before it was cut, so the changes it carried
are listed here rather than under a heading of their own.

### Language and semantics

- Element filters are evaluated: `filter <expr>;` in a package, definition or
  usage body, `import P::*[@T]` on an import, and a filter written at a
  document's root all gate what the names beside them bring into scope. A
  condition is a boolean predicate over one candidate with the candidate as the
  implicit `self` (KerML 8.2.4), so it is judged against a symbol and the
  metadata annotating it — prefix metadata, a metadata member of the body, and
  `metadata m about X` — with conformance through the candidate's supertypes, so
  `@Safety` matches a metadata type specializing `Safety`. A condition the
  evaluated subset does not cover is reported as such
  (`this filter condition cannot be evaluated, so it selects nothing and is not
  applied`) and one that does not yield a boolean is an error, rather than either
  silently selecting nothing. A root filter applies to its own document only, and
  a namespace's filter does not gate lookups made inside its own body.
- `@Safety` parses as the classification expression it is rather than a feature
  reference to the metadata type, which had lost the classification.
- A KerML `class`/`struct`/`assoc`/`behavior`/`predicate`/`interaction`
  declaration is classified rather than left unclassified, so the type checker
  judges it instead of exempting every unclassified usage — a binding's mismatch
  is still reported.
- A condition starting with an expression keyword (`true`, `null`, `if`) survives
  in a parameterised constraint body, where it used to be read as a nameless
  declaration and dropped.

### Runtime

- A fifth runaway bound, `SYSML_MAX_ELEMENTS` (default 1 000 000), bounds the
  collection elements one evaluation holds rather than the work a run does: an
  element is a 104-byte `Value` living as long as the collection holding it, so
  the default holds ~104 MB of them, in the band the other defaults were sized
  against. Every materializing path is charged — a range, a sequence literal,
  `->collect` and the other collection operations — and exceeding it is
  `ErrElementLimitExceeded` naming the variable, not the step limit: `1..10000000`
  used to conjure ~1 GB before the step budget reported it. A statement releases
  what it materialized, so a loop building a small collection each iteration is
  bounded by what it holds rather than by what it has produced in total.
- An action node's body ends the activation it ran in, so a run stepping the same
  body many times no longer holds what every execution's calc usages computed.
- A calc usage declared in an action's body or among a state machine's members
  binds its inputs from the values the behavior has reached, as one in a calc's
  body does: `calc t : Twice { in k = v; }` after `assign v := 2.0` reads 2.0
  rather than the value `v` was declared with.
- An evaluation outside a body — a decision or transition guard, a change
  condition or duration, an inline node expression, an attribute or slot default,
  an action argument, a constraint check — runs in a scope of its own, so what a
  calc usage answers it and the elements a collection it evaluates materializes
  live no longer than the step. A decision revisited after its body assigned
  reads the usage again over those values instead of the first evaluation's
  result, and a long run whose guard builds a small list is bounded by what it
  holds rather than stopped as a runaway. Reads within one step still share the
  scope, and a read through a part's feature chain belongs to the evaluation
  making it.
- `%budget` prints the five bounds a session runs on with the variable that
  raises each, and a literal expression that spends one is answered with that
  failure instead of "no declarations loaded".
- An action flow ends at a node with no succession, so an action whose last node
  is a plain nested action reaches `Completed` instead of failing the run with
  `nested action b has no successors`.
- `first s1 then s2;` starts the flow at `s1`, the node it names, rather than at
  an initial node of its own whose only edge reached `s2`: `s1` used to be
  skipped, losing what its body assigned, while the run still reported
  `Completed`. Written apart as `first s1; then s1 s2;` it behaves the same. A
  body states one start, so a `first` end naming the body's final node — a flow
  that would end where it starts — is now rejected rather than reported
  `Completed` with the declared node never run.
- A performance holds its values in one feature space its tokens share, because a
  fork duplicates control and not values: concurrent branches are steps of the
  one performance, so both branches' assignments survive where the last token to
  retire used to overwrite the others. Which write decides a feature two branches
  both assign is step order, stated in `docs/project/spec-compliance.md`.
- A runtime failure names SysML kinds and operands rather than Go types, a
  recursion reports a frame count and names the calc it collapsed, and a division
  by zero is reported as one.

### `sysml` command line

- **Breaking:** conversion is spelled `sysml model.sysml -convert ttl`: the model
  is a positional argument as it is in every other mode, and `-convert` names the
  format to convert it to. `-convert <file>` and `-to <format>` are gone — `-to`
  reports the replacement rather than "flag provided but not defined" — and the
  output path no longer chooses the format, so `-o /dev/null` or a FIFO needs
  nothing extra. `-from` still names an input format the extension does not.
- A flag may be written after the model it applies to (`sysml model.sysml -trace`),
  which Go's flag package would otherwise read as two files to load.
- A model is checked from a script or a build step without a prompt:
  `-validate`, `-constraint`, `-requirement`, `-satisfy`, `-instantiate`,
  `-calc`, `-action`, `-state -advance` and `-json`. The verdict comes from the
  runtime rather than from a printed line — one evaluation stands behind both the
  command and the prompt's `%constraint`/`%requirement`/`%satisfy`.
- **Exit status is meaningful on every path**: `0` when the requested operation
  succeeded and every requested check held, `1` when a check answered false, `2`
  when nothing was decided — a model that did not analyse, an expression that
  could not be evaluated, an unreadable input, a misused flag. A check is gated
  on analysis, so a verdict is never reported about a model nobody could read.
- Findings and diagnostics go to **stderr** and requested output to **stdout**,
  under one `sysml: ` prefix, so a pipeline consumes results and a log carries
  failures. Requested help (`-h`) is stdout and exit 0; an unknown flag stays
  stderr and exit 2. The interactive prompt is unchanged.
- A directory or a glob loads as a multi-file project — `sysml <dir>`,
  `sysml 'src/*.sysml'`, `%load <dir>` — expanded, sorted and deduplicated, and
  submitted as one submission, so resolution does not depend on load order.
  Diagnostics are reported against the file they came from at that file's own
  line numbers, and only model files among a glob's matches contribute.
- `-cpuprofile`, `-memprofile` and `-memstats` profile a load or a run.

### REPL

- A session no longer loses state silently. Re-typing a namespace merges into the
  one already in the buffer instead of replacing its body, additions are laid out
  where they belong, and every declaration, instance or debugging session that a
  submission did drop is reported, naming the submission that ended it.
- An instance and an active `%action`/`%state` session survive a submission that
  did not change what they depend on: an object whose declaration identity and
  resolved shape are unchanged is rebound into the new context, keeping its
  identity, its derived values, its connector ends and a selected variant, and
  only genuinely invalidated state is dropped with a notice. Declaring an
  unrelated `part def B;` no longer discards the instance of `A`. A surviving
  debugger keeps the executor it was started with rather than being re-lowered.
- The library is discoverable at the prompt: `%search <substring>` and
  `%builtins`, Tab completion over meta commands, symbols and paths, the nearest
  spelling of a mistyped command or symbol, and history kept outside the
  temporary directory.
- The diagnostic wording agrees with the other surfaces: `%eval`
  reports one parser diagnostic with a position and a caret rather than a cascade,
  an empty session no longer answers a real failure with "no declarations
  loaded", a blocked check names the line the unresolved error sits on and says
  so once, and a caret is drawn only for what was typed, counted in printed cells
  so multi-byte source stays aligned.
- `-satisfy` with no satisfaction assertion in the model is an undecided verdict
  like its siblings, so the command reports
  `sysml: no satisfaction assertion in the session` and exits 2.

### Editor support

- A first-party VS Code extension lives in `editors/vscode`: TextMate
  highlighting for `.sysml` and `.kerml`, comment/bracket configuration, and an
  LSP client that launches `sysml-lsp` from `systemica.server.path`, a
  workspace's `bin/sysml-lsp`, or `PATH` — highlighting still works when no
  server is found. It is built and side-loaded from this repository
  (`make vscode-package`) and is published to no marketplace. The grammars are
  generated from `internal/core/lexer.Keywords()` and a Go test fails when the
  committed ones are stale, so highlighting cannot drift from the lexer.
- LSP completion is typed and context-aware: items carry the kind, detail
  (`partUsage : Vehicle`) and documentation that hover shows, `v.` offers the
  members of `v`'s type — inherited ones included — and nothing else, `Pkg::`
  offers that namespace's members, and the standard library's top-level names
  are offered alongside the ones in scope. Prefix filtering stays on the client.
- `sysml-lsp` serves a session over one reader: it used to start a second read
  loop over its own stdio, so an editor's traffic raced two decoders and the
  server died with corrupted framing ("missing Content-Length header") within
  seconds of typing.
- Completion applies the element filters in force where the name is being
  completed, and resolves a filter condition's own names unfiltered, so the
  editor offers what the document can actually reach.

### Diagnostics

- An unresolved name carries the nearest spelling on **every** surface — command
  line, prompt and editor — where the hint used to exist only at the prompt:
  `unresolved reference: Whel — did you mean Wheel?`. A bare library name is
  offered its qualified spelling (`Integer` → `ScalarValues::Integer`), since the
  base library is not implicitly visible; the shipped examples import what they
  use, and every `examples/*.sysml` and `examples/*.kerml` now analyses cleanly.
- Candidates are ranked by how the reader would reach them, not by edit distance
  alone: the budget scales with the typed name's length (a name of two characters
  is not guessed at), a spelling as typed beats one differing in case, a name in
  scope beats one reachable only by a path, the reader's own declaration beats a
  bundled library one, a dominated candidate is dropped, and at most three are
  offered. A misspelling is not sent to a name nested in another element's body,
  which would take two corrections — so `Whel` beside your own `Wheel` offers
  `Wheel` alone, and with nothing close in the document it offers nothing rather
  than `SysML::Systems::TriggerKind::when`.

### Python bindings and `sysml-grpc`

- `Instantiate` returns every instance reachable from the root, so a Python caller
  expands a composite slot (`inst.engine.power`) instead of holding a bare instance
  id, and a slot the service could not evaluate is reported in `SlotValue.error`
  rather than as a null value. On the client, slot values convert to Python
  scalars, lists and nested `Instance`s, with the raw protobuf still reachable
  through `get_slot()`/`raw_slots`; attribute and item access raise
  `AttributeError`/`KeyError`/`SlotError` rather than returning `None`. (#110)
- `python -m pysysml.generate` emits a Python class per SysML definition —
  properties that carry the static type and perform the runtime delegation, so an
  editor completes `inst.mass` and a type checker rejects `inst.mas`. `GetSymbol`
  reports the type facts this needs (`type_info` with primitive reduction,
  `multiplicity`, all specialization edges), `pysysml` ships `py.typed`, and
  emission is deterministic so the output can be committed. (#111)
- `GetServerInfo` reports the service's build version and the capabilities it
  supports by name, so a client can require a capability instead of comparing
  version strings — versions of source and forked builds are not comparable, and a
  service that predates the RPC answers `UNIMPLEMENTED`, which is itself the
  answer. Typed generation requires the `type_facts` capability and fails naming
  the service in use, where it came from and how to replace it. Against a service
  without it, every generated feature was typed `object`, indistinguishable from a
  feature that is genuinely untyped: the v0.0.5 `sysml-grpc` predates `type_info`,
  so a caller letting `pysysml` download the released binary silently got a
  useless module.
- A generated module records the model source hash and the generator's emission
  schema, and `pysysml.generate --check` regenerates in memory and exits non-zero
  when the committed module is missing or would change, writing nothing — a stale
  module was previously found at attribute access, or never, since a feature
  removed from the model keeps type-checking.
- `TypedObject.from_instance` rejects an instance of another definition, naming
  both types, instead of accepting it and failing later with a confusing
  `TypeMismatchError` on the first slot read. An instance of a definition that
  specializes the expected one is accepted; an instance whose type no generated
  class describes is accepted too, because instantiating a usage reports the
  usage's own FQN, which the client cannot relate to a definition.
  `unchecked(instance)` is the explicit escape hatch.
- `Convert` writes a model back out — SysML/KerML notation or RDF Turtle, from a
  loaded model named by its `model_hash`, a path the service opens, or content
  carried inline — using the same exporter
  `sysml -convert` uses, so a Python caller round-trips a model instead of only
  reading one: `model.to_sysml()`, `model.to_turtle()`, `model.save("m.ttl")` and
  `pysysml.convert(...)`. Reported as the `convert` capability, so an older
  service fails naming the upgrade. A conversion that cannot be written faithfully
  returns the diagnostics that explain it as a `ConversionError` rather than
  partial output; `tolerate_syntax_errors` writes notation anyway and is rejected
  for the graph directions, where an unparsed declaration would vanish silently.
  A `Model` converts by hash, so a file edited between `load` and `save` does not
  change what is written; a model since evicted from the service cache is
  `NOT_FOUND` rather than something else, and `convert(file_path=...)` is how a
  caller asks for the file as it stands now.
- `ParseFile` hits its cache on the source it read — file name and content —
  rather than on the `content_hash` the request carried, which is now ignored:
  `pysysml` never sent one, so re-loading unchanged content re-parsed it and
  reloaded the standard library every time — ~35 ms where the cache costs
  ~0.5 ms — and a hash disagreeing with its content would have served an
  unrelated model.
- `python/scripts/bench_latency.py` reports p50/p95/p99 per client call, and
  `python/README.md` documents the measurements and what they mean for a real-time
  analytics loop.
- `Model.eval(expression, context_symbol_id=...)` evaluates against the model it
  is called on, so evaluation is no longer the one operation making a caller carry
  the hash back to the connection: `model.eval("1+1")` for
  `conn.eval("1+1", model.hash)`. The typed failures are the connection's —
  `ExecutionError` for an expression that cannot be evaluated, `ModelNotFoundError`
  for an evicted model.
- Naming an element of the wrong kind raises `WrongKindError` (an
  `ExecutionError`) from `verify_constraint`, `verify_requirement`,
  `verify_satisfaction` and `calc`, as naming an element that does not exist
  already did: verifying a part def as a constraint used to answer with a verdict
  whose `holds` was false, telling a caller its model does not hold when the
  answer was that it named a part def. The service reports the distinction as a
  typed `FailureReason` on `Verdict`, `VerifySatisfactionResponse` and
  `EvaluateCalcResponse`, so the client classifies it without reading the message
  text.
- A `host:port` address given as the host is read as one, on `connect` and on the
  module-level helpers taking `host`/`port`: `connect("localhost:50123")` reaches
  port 50123 instead of building the target `localhost:50123:50051` and reporting
  a service start timeout for an address nobody asked for. A port named twice with
  two values, and a port that is not a number, raise `ValueError` naming the
  mistake. The `pysysml.generate` and `bench_latency` command lines report a
  host/port disagreement as an `error: …` line and exit 2 rather than as a
  traceback.
- A `Query` RPC evaluates the SysML v2 API & Services query model (scope /
  select / where, primitive and composite constraints) over the symbol index and
  semantic model, with `model.query()` accepting the standard's JSON payloads
  verbatim. The standard's model has no traversal or transitive closure, so this
  is an interop surface for its clients rather than a query language;
  `docs/reference/api.md` and `docs/project/spec-compliance.md` state what is supported. An
  element with no qualified identity — a doc note, an anonymous usage, a
  `connect` — is omitted rather than answered under a non-unique `@id`.

### Performance

- Loading a large model is linear where it was quadratic: three lookups scanned a
  namespace's members or child scopes once per member. Child scopes are indexed
  by the declaration owning them, a namespace's imports are memoized, and a
  member's owner is found through the scope's owner link.
  `docs/internals/performance.md` records the measurements.
- `ParseFile` hits its cache on the source it read, so re-loading unchanged
  content costs ~0.5 ms instead of re-parsing and reloading the standard library
  (~35 ms).

### Removed

- `internal/core/deps` — the `sysml.toml` manifest, lockfile, git fetcher and
  resolver — is deleted. Nothing imported it: no manifest was ever looked for by
  the command line, the prompt or the server, and the README claim it backed is
  gone.

### Documentation

- `README.md`, `docs/guide/` and `docs/reference/cli.md` describe the
  shipped command line, editor and RDF surfaces, including the exit-status
  contract and the streams each finding is written to. The claims that overstated
  what ships — dependency management, and the IDE and Python verification
  caveats — are corrected.

## 0.0.5 — 2026-08-12

### Language and semantics

- A requirement or constraint condition is evaluated against the features of the
  element stating it, so it sees that element's own attributes, the ones it
  inherits from the definition it is typed by, and the values a usage rebinds
  (`attribute :>> maxVerticalSpeed = 1.5;`, `constraint limit : MassLimit { in m = mass; }`).
  This was the first known limitation listed for 0.0.4.
- `require <expr>;` and `assume <expr>;` parse in a requirement definition body,
  not only in a usage, as do the `concern def`, `viewpoint def` and
  `satisfy … by …` bodies that share the member set. A `subject` may redeclare
  the one it inherits (`subject subj : View[1] :>> RequirementCheck::subj;`).
- A condition stated through a nested constraint — `require constraint { <expr> }`,
  `assert constraint [name] { <expr> }` — is evaluated, with every condition of
  that body kept rather than only the last. A requirement carrying no condition
  still has no verdict rather than passing vacuously.
- A violated condition reports which condition failed
  (`Required condition evaluated to false: actualVerticalSpeed <= maxVerticalSpeed`),
  and a feature a condition names but which holds no value is reported as such
  rather than as unresolved.
- A quantity expression (`attribute maxVerticalSpeed = 1.5 [m/s];`) is evaluated.
  A quantity carries its magnitude and the measurement reference it is written
  in, as `Quantities::ScalarQuantityValue` (`num` + `mRef`) does. Units reduce to
  a scale factor over base units through the Quantities and Units library's own
  `unitConversion` and unit-defining expressions, so commensurable units convert
  before a comparison or a sum — `1.5 [m/s] <= 5.4 [km/h]` is true, exactly, at
  its boundary — and an operation whose unit is composed by it keeps that unit
  (`10 [m] / 2 [s]` is `5 [m/s]`, `4 [m] / 2 [m]` is `2`). An operation between
  units that measure different things (`1.5 [m/s] <= 2.0 [s]`) is an error, never
  a comparison of bare magnitudes that would equate `1.5 [m/s]` with
  `1.5 [km/h]`. A cached library record carries the unit reduction of its
  symbols, so its key now covers the digest of the whole library set: the
  reduction follows a prefix or reference unit declared in another file, and a
  key over one file's content alone kept converting with the factors of an
  edited `SYSML_LIBRARY_PATH` library's old definitions. A record no load has
  hit for 30 days is pruned, since a wider key leaves more records that nothing
  will look up again.
- `assert satisfy <requirement> by <part>;` has a verdict of its own: the
  assertion is evaluated as the requirement usage it is, with the requirement's
  subject parameter bound to an object of the part named by `by`, so the
  requirement's conditions — its own and the ones it inherits — read that
  object's values. A requirement feature carrying no value of its own is read
  from that object's feature of the same name, as it is when a requirement is
  evaluated on an instance.
- `%satisfy` evaluates the satisfaction assertions a model states — every one, or
  the ones a named element states — since `assert satisfy … by …` is anonymous
  and could not be named at the prompt before.
- An assertion can be negated: `assert not constraint { <expr> }` and
  `assert not satisfy <requirement> by <part>;` hold exactly when the conditions
  they deny do not, rather than parsing as a declaration named `not`. A negation
  denies the conditions of the constraint it is written on together — `not (a and
  b)`, not `not a and not b` — so it holds as soon as one of them fails.
- The KerML function library's scalar numeric functions are evaluable: `sqrt`,
  `abs`, `floor`, `round`, `max`, `min`, `isZero`, `isUnit`, `sin`, `cos`, `tan`,
  `cot`, `arcsin`, `arccos` and `arctan`. Dispatch is by the declaration's
  qualified name, so a model's own `calc sqrt` is evaluated from its body.
- Exponentiation (`**`, `^`) is evaluated, by one implementation the constant
  folder and the runtime share. Integer operands with a non-negative exponent
  give an Integer, any other numeric pair a Real.
- `exp`, `ln`, `log(x, base)` and `atan2(y, x)` are evaluable. The OMG Kernel
  Function Library declares no signature for any of them, so they are declared in
  a new non-normative Systemica extension library,
  `internal/core/libs/stdlib/Systemica Libraries/SystemicaMathFunctions.kerml`,
  which a model reaches with `import SystemicaMathFunctions::*;`. The vendored OMG
  files are unchanged; the stdlib parse gate is now 95/95 clean.
- A result that is not a finite value of the declared type — `sqrt(-1.0)`,
  `arcsin(2.0)`, `ln(0.0)`, `log(x, 1.0)`, `atan2(0.0, 0.0)`, `0.0 ** -1.0`,
  integer overflow — is reported where the expression is evaluated instead of
  folding to a NaN, an infinity or a wrapped integer.
- A unit written unqualified is resolved through the imports in scope, and a name
  a declaration shadows is reported with the declaration that shadows it and the
  way out: `m resolves to the attributeUsage m declared in SH, shadowing the
  measurement unit SI::metre — write SI::m to name the unit`. The rule and the
  message are the same wherever a quantity is evaluated — a part's attribute, an
  action or state body, a calc invocation, a constraint or requirement condition,
  and an expression typed at the prompt.
- A quantity value renders like the bare Real it measures, magnitude first and
  unit in brackets (`v = -15.20 [m/s]`, `= 5.00 [SI::m/SI::s]`), in results,
  execution traces and slot listings alike, in place of full float precision.
- `%calc` accepts quantity arguments in every argument form it accepts numbers
  in — comma-separated, whitespace-separated, invocation form, and a
  parenthesized subexpression — so `%calc P::Fall 10.0 [m/s], 3.0 [s]` invokes
  the calculation rather than reading the bracket as a sequence index.
- An attribute default written in an action or state body is evaluated in the
  scope that declares it, so a unit or a type an enclosing package imports
  resolves there (`attribute h : LengthValue = 500.0 [m];`), and a body-local
  name is not resolved against the namespace the session happens to be in.
- The loop and conditional statements of an action body execute: `while`, `loop`,
  `for … in …` and `if … { … } else { … }` lower to real decision and merge
  nodes, so a body that iterates reaches its final node with the values it
  computed rather than deadlocking.
- A `then` written as a member of a body (`then loopIt end;`, `then start
  compute;`) is a succession edge in the lowered graph, like the standalone form,
  rather than a member the runtime had no edge for.
- A relationship written with a keyword and one written with its symbol are the
  same relationship end to end — `specializes`/`:>`, `subsets`/`:>`,
  `redefines`/`:>>`, `references`/`::>` — so hover, go-to-definition, completion
  and the index report the same thing whichever spelling a model uses. A feature
  that takes its effective name from what it redefines is read under that name by
  the same paths, including its short name.
- `assert satisfy <requirement> by <part>;` parses with the `by` subject named
  by a qualified name, and an action body that is not braced ends where its
  statement ends rather than swallowing the member that follows it.

### Runtime and tooling

- A parser try-parse that gives up now rewinds: the token buffer is read through
  a cursor rather than re-sliced as tokens are consumed, so a checkpoint restores
  the position it was taken at, along with the diagnostics and warnings the
  abandoned attempt reported. Backtracking previously left the words the attempt
  had consumed behind, which made a condition beginning with a feature named
  `constraint` (`assert constraint x > 0;`) report a missing expression it did
  have, and could exceed the buffer's capacity. A reserved word used as a name
  in an expression still does not resolve; that is a separate gap.
- `redefines <target> = <value>` is read whatever the target's length: the member
  is recognized by parsing the target and rewinding when no `=` follows, in place
  of a scan capped at ten tokens ahead that read
  `redefines outer.middle.inner.leaf.deeper.deepest.last = 1;` as a body member
  it could not parse.
- The evaluation step budget is configurable through `SYSML_MAX_STEPS`, so a
  legitimately long run — a numeric integration in an action body, say — is not
  bounded by a fixed ceiling. A value that is not a positive integer is reported
  at REPL/CLI startup and at gRPC service construction, naming the variable and
  the value, rather than falling back to the default silently.
- The step-limit error reports the budget actually in force and names
  `SYSML_MAX_STEPS`, so the message says how to raise it.
- The three sibling runaway bounds are configurable the same way, each through
  its own variable, since they count incommensurable units: an action run's
  token-flow steps through `SYSML_MAX_ACTION_STEPS`, a state machine run's
  dispatched events through `SYSML_MAX_EVENTS` and its do-activity actions
  through `SYSML_MAX_DO_STEPS`. Each error names the
  variable that raises it, so a long simulation is no longer capped by a bound
  with no way out.
- The defaults are raised to 10 000 000 evaluation steps, 1 000 000 action
  token-flow steps, 1 000 000 events and 5 000 000 do-activity steps (from
  100 000 / 10 000 / 10 000 / 100 000). Execution allocates nothing per step —
  peak RSS is ~34 MB whether a run spends ten thousand steps or fifty million —
  so the sizes are set by how long a runaway takes to report: at ~13.6M
  evaluation steps/s and ~1.9M events/s each reports one within about a second,
  and a fully traced run at all four ceilings holds ~320 MB.
- The evaluation step budget bounds one run rather than a whole session: the
  counter is reset when a run begins - an evaluation, a constraint or
  requirement check, an instantiation, a calc invocation, an action or a state
  machine - so a REPL session of many small evaluations no longer exhausts its
  allowance and starts failing every one. A run started inside another shares
  the outer run's budget, as does every call into a run a caller drives step by
  step (the `%action`/`%state` debuggers), so a runaway cannot escape the bound
  by starting runs of its own.
- The REPL's `%advance` no longer stops after a fixed 10 000 events and
  do-activity actions, which could look like a machine that had settled. It is
  bounded by the session's event and do-activity budgets, and says which one cut
  a drain short.
- A session is written out: `%save <file>` writes the notation (`.sysml`) or the
  RDF graph (`.ttl`) chosen by the extension, atomically, replacing an existing
  file and saying so. A session that does not fully parse still saves as
  notation — the text as typed, re-indented, with the syntax errors reported as
  warnings — so work is never trapped in the REPL; `.ttl` keeps the refusal,
  since a graph built from a partly recovered tree would be quietly missing
  declarations.
- `sysml -convert` converts a model between the notation and RDF Turtle in both
  directions, round-tripping packages, definitions, usages, features, imports,
  connectors, successions (including a `then` written as a body member) and
  satisfy assertions. What the mapping normalizes and what it refuses is
  documented in [docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md); a refused construct
  is reported with its node and position rather than dropped.
- Removing a document unwinds what its wildcard re-exports contributed, so a
  name a removed file re-exported no longer resolves, and the workspace reuses
  the index slot the document held rather than growing one per edit — an editing
  session's memory no longer climbs with the number of reindexes.
- The `sysml-grpc` service binary is published with the release, one per
  platform (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
  windows/amd64), each with a `.sha256` sidecar and covered by
  `SHA256SUMS.txt`. `pysysml` downloads the binary matching the release it is
  told to use, verifies it against its sidecar, caches it under `~/.pysysml/bin`
  and starts it — so a Python caller needs no Go toolchain. (v0.0.4 described
  this; the release assets it publishes do not include the binaries, and this is
  the first release that does.)
- Homebrew installation is live: `brew install Open-MBEE/tap/systemica` installs
  both binaries from the published bundle, verified by checksum, and avoids the
  macOS quarantine prompt that a browser download sets. See
  [packaging/homebrew/README.md](packaging/homebrew/README.md).
- `pysysml` is published to PyPI by CircleCI, on its own `pysysml-v*` tag, from
  one declared version (`python/pysysml/_version.py`) that the packaging
  metadata and `pysysml.__version__` both read — a tag that disagrees with it
  fails the job before anything is uploaded. `python/setup.py` is gone;
  `pyproject.toml` declares the build. See
  [docs/project/releasing.md](docs/project/releasing.md#releasing-opensysml-to-pypi)
  (that section, and the package, are named `opensysml` since the rename).

### Known limitations

- RDF/Turtle conversion has no mapping for a member that states a condition or a
  step, so a model containing one is reported rather than converted: `require`
  and `assume` members, a constraint body's condition, `subject`, a computed
  `return`, `assign`, `if`/`while`/`loop`/`for`, substates, transitions and
  `entry`/`do`/`exit` (`cannot convert the *ast.RequireMember at <file>:<line>`).
  A requirement stating a condition, and any state machine or action body with
  statements, must be saved as `.sysml`. The full list is in
  [docs/reference/rdf-mapping.md](docs/reference/rdf-mapping.md).
- The REPL's prompt evaluates in the *last* namespace the session declared. After
  typing a second package, the first package's members and the units its imports
  brought in are reached by qualified name only (`1.0 [SI::m]`, not `1.0 [m]`).
- Re-typing a declaration whose name the session already holds replaces the
  earlier snippet rather than merging into it, so adding a member to a package by
  re-typing the package drops the members left out of the new text.
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it (as in 0.0.4). *(Fixed after
  this release; see 0.0.8.)*
- An attribute declared with a type but no value (`attribute diameter : Real;`)
  instantiates as an object of that type rather than an unset value, so `%slots`
  shows `diameter = Instance(ID: n)` with `(no features)` under it.
- The macOS and Windows binaries are unsigned, so a browser download is
  quarantined by Gatekeeper or flagged by SmartScreen. Install with Homebrew or
  `curl`; see [docs/project/macos-distribution.md](docs/project/macos-distribution.md).

## 0.0.4 — 2026-08-10

The first tagged release.

### Language and semantics

- Hand-written SysML v2 lexer and recursive-descent parser that never panics:
  malformed input yields error nodes and diagnostics. All 94 official SysML v2
  standard library files parse clean.
- Lazy, memoized name resolution and a type system covering conformance,
  multiplicity, specialization, redefinition and feature chains. An unnamed
  feature takes its effective name from what it redefines or reference-subsets
  (`:>> power = 250.0;` names, and overrides, `power`), and a nested usage that
  reuses an inherited name without redefining it is reported.
- Tiered validation (syntax → name resolution → typing → constraints), where a
  failing tier suppresses the ones above it rather than reporting noise.
- Measured spec compliance, rule by rule, in
  [docs/project/spec-compliance.md](docs/project/spec-compliance.md); 98/100 of the OMG
  training corpus parses and analyzes clean, with the two remaining files
  pinned as upstream source bugs.

### Execution

- Instantiation materializes objects from part definitions: literal defaults
  are folded, defaults written over sibling features are evaluated against the
  object, and a cyclic default is reported as a cycle rather than exhausting
  the step budget.
- Constraints and requirements are evaluated bound to a concrete instance, so a
  verdict is about an object rather than about a declaration. A false condition
  is a verdict, not an internal error.
- Action execution over lowered graphs: tokens, fork/join/decision/merge,
  control-flow keywords, nested invocation, `send`, and `accept` that suspends
  until its message arrives.
- State machine execution: transitions, guards, entry/do/exit behaviors,
  hierarchy, orthogonal regions, pseudostates including shallow and deep
  history, time and change and call triggers, deferred events.
- Every unsupported path returns a typed error; robustness cases cover
  deadlock, dangling transitions, unbound parameters, and budget exhaustion.

### `sysml` REPL

- Declarations, instantiation and inspection: `%instantiate`, `%slots`,
  `%instances`, `%eval`, `%calc`, `%constraint`, `%requirement`. Every command
  accepts a qualified name, so a model inside a `package` is reachable.
- Action debugging (`%action`, `%step`, `%continue`, `%tokens`, `%break`,
  `%stop`) and state machine debugging (`%state`, `%events`, `%current`,
  `%advance <time>`).
- Output modes: `-quiet` reports errors only, `-debug` widens diagnostics to
  the whole session buffer with absolute positions and originating pass, and
  `-trace` prints the execution trace — evaluation steps, calc invocation,
  token flow, transitions — as commands run. `%verbosity` and `%trace` are the
  prompt equivalents.
- A submission's report covers what was just typed: diagnostics are scoped to
  it and line numbers are relative to it.
- `sysml -e '<expr>'` evaluates without entering the prompt; `--version`
  reports the release tag, commit, build time and Go version.

### Tooling

- `sysml-lsp`, a Language Server Protocol server (diagnostics, hover,
  completion, go-to-definition).
- `sysml-grpc` plus Python bindings (`python/`) for driving parse and execution
  from a notebook, including DataFrame output. `Instantiate` reads slots the way
  the REPL does, so a derived attribute comes back evaluated rather than
  unmaterialized. The service binary is published with the release, so `pysysml`
  can fetch and checksum-verify one instead of requiring a Go toolchain:
  `download_binary('latest')`, or set `PYSYSML_GRPC_VERSION` and let
  `pysysml.connect()` start it.
- Releases publish per-binary and bundle archives, and the raw
  `sysml-grpc-<os>-<arch>` binaries with `.sha256` sidecars, for linux/amd64,
  linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64, with
  `SHA256SUMS.txt` over all of them. macOS and Windows binaries are unsigned —
  see [docs/project/macos-distribution.md](docs/project/macos-distribution.md).

### Known limitations

- A parameter bound by a constraint or requirement usage
  (`constraint limit : MassLimit { in m = mass; }`) is not passed into the
  conditions it inherits from its definition. *(Fixed after this release; see
  0.0.5.)*
- A multi-valued feature that is both typed and given a default takes the typed
  instantiation; the default is not merged into it. *(Fixed after this release;
  see 0.0.8.)*
