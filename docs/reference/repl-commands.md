# REPL meta-commands

Every command `sysml` accepts at the prompt. The session model behind them (what a submission
replaces, what it drops) is explained in [guide chapter 4](../guide/04-repl.md).

Every command that takes a `<name>` accepts the quoted spelling the notation uses, including a
quoted segment containing a space and a quoted segment in the middle of a chain:
`%instantiate 'My Pkg'::Car`, `%features Top::'My Pkg'::Car`.

Every command that takes an `<object>` — `%features`, `%invoke`, `%eval in`, and the object
`%action` and `%state` are performed by or attached to — reads one
[object reference](#object-references): the name an object was instantiated under
(`car`, `Demo::car`), its id as `%instantiate` printed it (`#3`), or either followed by a path
into the parts it holds (`car.fl.hub`, `#3.fl`, `car.wheels[2]`).

| Command | Description |
|---------|-------------|
| `%help` | Show help message |
| `%list` | List all declarations in current session |
| `%clear` | Clear session (reset all declarations) |
| `%load <path>...` | Submit the contents of files, directories or globs |
| `%print [name]` | Print the session model as SysML notation at the prompt, or only the named element and its body (`%print 'My Pkg'::Car`). Comments are kept, since the same writer `%save` writes notation with is used, and what is printed can be typed back in. Notation only: nothing about RDF is reported. Reading the model materializes nothing and leaves a debugging session running |
| `%save <file>` | Write the session model to a file: `.sysml` notation (comments preserved) or `.ttl` RDF, which is [experimental](rdf-mapping.md#status-experimental) and reported as such on each save |
| `%query <oslc-query>` | Identify model elements using OSLC Query text |
| `%verbosity [level]` | Show or set output level: `quiet` (errors only), `normal`, `debug` (every diagnostic over the whole buffer) |
| `%trace [on\|off]` | Show or set execution tracing: each evaluation, calc invocation, action step and state transition, and each `choice` the executor made among alternatives the library leaves unordered — several steppable tokens, several holding decision guards, several enabled transitions out of one state for one event, two tokens writing one feature in one step, two executors due at one instant of the clock — naming the alternatives and the one taken, and each `unevaluable guard` it read only to report one and could not evaluate ([Choice points](../guide/06-behavior.md)). `%step`, `%continue` and `%advance` end with a count of the choices they made and of the guards they could not evaluate (`1 choice point; 1 guard not evaluable`) whether or not tracing is on |
| `%schedule [<policy>]` | Show or set the scheduling policy the executors resolve their [choice points](../guide/06-behavior.md) under: `reverse` (the default: reverse token order, first holding guard, first enabled transition), `declared` (spawn and declaration order) or `seed:<n>` (a pseudo-random order the non-negative integer `n` fixes, so the same seed replays the same run). Applies to runs started from then on — `%action`, `%state`, `%analysis`; a calc's body performs nothing, so `%calc` has no choice to make — while a debugging session already under way keeps the policy it started with; every choice point a run reaches is reported and the `took …` of each `choice` line is what the policy took. A spelling naming no policy (an unknown name, `seed` or `seed:` without a number, `seed:-1`, `seed:abc`, a malformed `explore:` option) is refused and the policy is left as it was. `explore[:runs=N,depth=D]` is refused at the prompt too, as a typed error saying why: it replays a behavior from the start once per linearization, which `%action` and `%state`, stepping one run, cannot do — run `sysml -schedule explore -action <name>` (or `-state`, `-analysis`, `-calc`) for the outcome table ([Exploring every linearization](cli.md#exploring-every-linearization)), or send a request with that `schedule` over the wire |
| `%strict [on\|off]` | Show or set strict conformance: report notation no SysML v2 production admits as an error, and reprint the session's diagnostics under the new mode ([Strict conformance](../guide/03-command-line.md#strict-conformance)) |
| `%budget` | Show the five bounds one run may spend, each with the variable that raises it |
| **Library Discovery** | |
| `%search <substring>` | List the declared and library symbols whose qualified name contains the substring, with the kind of each |
| `%builtins` | List the library functions the runtime implements directly (`sqrt`, `abs`, `max`, `floor`, `x->isEmpty()`, `x->sum()` …), each with the package an `import` must name for its bare name to resolve; the qualified name (`RealFunctions::sqrt(2.0)`) resolves anywhere |
| `%view <name>` | Show what a view exposes: its own `expose` relationships plus the protected ones of the views it specializes, the views nested in it (each with its own exposed set), and its conformance to every viewpoint it satisfies. Conformance is a verdict of `conforms`, `violated` or `unevaluable` per viewpoint and per framed concern, with the reason, the exposed element a concern's condition failed for, and `(from <view>)` where the `satisfy` is inherited. Asking about an element that is not a view says so |
| `%render <name> [form]` | Render a view's exposed set in the kind its `render` member states: a containment tree with nested views as subtrees, an interconnection diagram of the exposed parts and the connections between them, a state machine's states and transitions, an action's nodes and successions, or a table of the exposed elements and what they declare. A view with no `render` member renders as a tree. Output is indented text by default, or the machine-readable form of the kind: a [Mermaid](#rendering-a-view) diagram with `mermaid`, a Markdown table with `markdown`. Asking for a form the kind cannot be written in tells you which form it uses. Read-only: it creates no object and leaves a `%action`/`%state` debugging session running. A view that exposes nothing renders empty and says so; a rendering kind this build does not produce is reported by kind and view rather than rendered as something else; an element the rendering cannot represent is reported, not dropped |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create an object of a part definition and start the behaviors its type exhibits or performs. Each object runs its own machine, initialized after its feature values are built and run until it is quiescent. A second `%instantiate` of the same name creates a new object, and the name then refers to that one. A later submission keeps the object's identity but restarts its behaviors from their initial states, and says so |
| `%features <object> [all\|depth <n>] [json]` | Show what an object holds for each feature of its type. The object is named, addressed by id, or reached by a path: `%features car`, `%features #3`, `%features car.fl.hub`, `%features car.wheels[2]`. A feature with no value reads `<unset>`. States and actions hold no value, so they are listed after the values under a `Behaviors:` heading with what the object is doing with each: the current active state of a machine it exhibits (the state `%current` reports), the execution state of an action it performs, `not running` for a state or action it neither exhibits nor performs, or, for a named transition, the step it declares (`toggle: transition, modes.closed → modes.opened`). A behavior a redefinition renamed (`exhibit state fancyModes :>> modes`) is one execution under two names, and both rows report it. The values a running behavior owns — the attributes of the machine's own occurrence, an action's parameters and outputs — are listed under its row (`modes: exhibited state machine, current state running` followed by `count = 1`), apart from the performer's own values, and are bounded like any nested object. Reading a feature value builds the objects it holds, so the listing is bounded by default — 200 lines, nesting 8 deep — and a listing cut short says which form shows the rest. `all` lifts both bounds and reads the whole tree out; `depth <n>` bounds nesting at `n` levels and lifts the size bound, naming what it did not expand (`machine : Machine (not expanded: depth 1)`). `json` writes the object and everything reachable from it as one document in the shape the API's `Instantiate` returns (`instance`, `instances`, `diagnostics`), bounded by default at 1000 objects, with a graph cut short reported as a `warning` diagnostic. `all`/`depth` and `json` combine (`%features ctx all json`); `all` and `depth` together, a missing or negative depth, and an unknown word are errors naming the usage |
| `%instances` | List all created objects: the named ones, and the ones a second `%instantiate` of their name displaced, which stay reachable by id (`#3 (ID: 3, displaced from Demo::car)`) |
| `%eval <expr>` | Evaluate expression, in the last namespace the session declared; a library function is reached by its bare name only where that namespace imports its package, as the checker resolves it, and by its qualified name anywhere |
| `%eval in <name> : <expr>` | Evaluate expression in the named element's own namespace, or, when the name is an [object reference](#object-references) (`car`, `#3`, `car.fl`), on that object, so that a feature reads the value it holds after its behaviors ran (`%eval in #1 : recv.got`, `%eval in ctx.recv : got`). In the declaration's own namespace a feature the declarations give no value to — a multi-valued `part wheels : Wheel[4]` or an attribute with no default, and a chain through one such as `wheels.radius` — reads `<unset>`, as it does on an object; an expression over such a feature (`unsetMass + 1`) fails, naming the feature that has no value, and `unresolved reference` is reserved for a name nothing declares. The separator is the first `:` outside a quoted name that is not part of a `::`, so `%eval in Demo : Vehicle::mass` works |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%analysis <name>[(<args>)] [<object>]` | Run an analysis or verification case — a calculation performed as an action — and print its `out` and `return` values with their units, then the verdict of its `objective` and of each `assert constraint` in its body: `satisfied`, `not satisfied` with the violated condition, or `undecided` with the reason the condition could not be evaluated. An objective typed by a requirement def binds the def's subject as a requirement usage does (`subject = ship;`, `subject s = ship;` or `subject :>> s = ship;`), reading the case's subject, parameters and steps' outputs; one binding none checks the case's result, the library's default for it (`Cases::Case::obj`), and is `undecided` naming the type when that result is not of the subject's type. Arguments in parentheses bind the case's `in` parameters, positionally (`%analysis An::Case(3.0)`) or by name (`%analysis An::Case(limit = 3.0)`), evaluated at the prompt like `%calc`'s; an [object reference](#object-references) after them is the case's `subject` (`%analysis An::CostAnalysis An::barge`). A usage that binds its subject (`subject s = ship;`) needs no object; a definition, or a usage that binds none, is refused by name without one, as `%requirement` refuses an unbound subject. A usage nested in a part runs as a feature of the object the session holds for that part (`%instantiate An::holder`, then `%analysis An::Holder::inner`). The body's `action` steps, sequenced by `then` or by declaration order, run as an action does, later steps reading earlier steps' outputs; a step that fails, a body that deadlocks or exhausts the step budget, a case that runs itself, and an `in` parameter with no argument and no default are errors naming the case; a case recursing without bound through a nested `analysis` step reports the depth limit on one line, its repeated frames collapsed to a count as `%calc`'s are. `%calc` refuses an analysis case and says to run it this way. A `verification def` or `verification` usage runs the same way and reports in addition the `VerdictKind` its body produced: `pass`/`fail` from the library's own `VerificationCases::PassIf` calculation, a `VerdictKind` literal the body bound, `inconclusive` for a body producing no verdict value, or `error` with the reason a body's run could not be carried out; each nested verification step is reported on its own line, marked `(subcase)`, the library stating no roll-up. A `TradeStudies::TradeStudy` runs the same way — the library's own expressions apply the case's `evaluationFunction` to each alternative the subject lists, in subject order, and `selectOne` returns the first scoring the objective's `best` — and the report adds each evaluation the run made of the case's own calc as a value, with `[selected]` on the alternative returned and `[tied]` on a later one scoring the same; an evaluation that fails is listed with its error and leaves the objective `undecided`, a subject listing no alternative or redeclared `[1]` and bound to several is a `multiplicity violation` ([Trade studies](../guide/06-behavior.md#trade-studies)) |
| `%run-query <name> [<p>=<expr>...]` | Execute a document query (a `calc def` specializing `DocumentQueries::Query`) and print its rows and projected cells. A projection lists declared property names and may add computed columns: `Column(name = "<column>", expression = <expr>)` entries evaluated once per row over the row element's features, with arithmetic (`+`, `-`, `*`, `/`), string concatenation and `??` defaults for absent values. A column expression that fails (including a reference that resolves to no value and has no `??` default) fails the query with a typed error rather than producing an empty cell. Each binding is written as `<parameter>=<expression>`; a name binds the element it refers to, anything else is evaluated as an expression. A parameter left unbound takes its declared default (`in root : Element = telescope;`, `in pattern : String default "m";`) under the same rule: a name binds the element it refers to, anything else is evaluated once in the query that declared it, and a redefining parameter's default replaces the inherited one. Named query invocation and relationship traversal (`RelatedElements` over specialization, subsetting, redefinition, typing, connection, allocation, satisfaction and verification edges, outgoing or incoming) are supported. See the [query cookbook](../manual/query-cookbook.md) |
| `%render-document <name>` | Compile a document definition (a `part def` specializing `DocumentQueries::Document`), run its queries against the model and print the rendered Markdown. A document binds its queries' parameters in the model, so the name is the whole invocation. The output is deterministic CommonMark: the title and sections as ATX headings; paragraphs from text runs (`Span` runs with a `plain`/`emphasis`/`strong`/`code` style, `Link` runs to a URL, `Ref` runs linking to another content block's anchor, and query-produced values styled through nested `SpanColumn`/`LinkColumn` column runs); GitHub-flavored pipe tables with the projected column names (one subtable per group value when the table has a `groupBy` column); bullet and numbered lists; diagram blocks as fenced ` ```mermaid ` blocks rendered through the view engine (a table-kind view as a pipe table), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown metacharacters in content are escaped. Markdown is the only form the REPL writes; the CLI's `-doc-form html` renders the same document tree as semantic HTML ([Rendering a document as HTML](cli.md#rendering-a-document-as-html)) and `-doc-form pdf` converts the Markdown to PDF ([Rendering a document as PDF](cli.md#rendering-a-document-as-pdf)). See the [document generation manual](../manual/README.md) |
| `%sweep <name>[(<args>)] [<object>] <parameter>=<from>..<to>[:<step>] ...` | Run an analysis case or a calc once per value of a range and print the runs as a table: one row per run, carrying the values bound for it, the run's `out`/`return` values, the verdict of its `objective` where it has one, and the wall time of that run. The invocation is written as `%analysis`/`%calc` writes it — arguments in parentheses, an [object reference](#object-references) after them as the case's `subject` — and each range that follows names a parameter the case declares and the arguments do not bind. Endpoints and step are expressions evaluated at the prompt, units included (`speed=0.0 [SI::'m/s']..10.0 [SI::'m/s']:2.0 [SI::'m/s']`), converted to the unit `<from>` carries; `<to>` is included where the step lands on it. A range between Integers with no `:<step>` steps by one, up or down as its endpoints direct; a range between Reals with no step is refused. Several ranges run their cartesian product, the first written varying slowest, and rows come out in that order. A run that fails is a row numbering its typed error, printed in full under the table, rather than an abort of the table. A parameter whose name needs the quotes of an unrestricted name is swept under that name, quotes included (`'launch mass'=1..3`). A step of zero, a step whose sign never reaches `<to>`, an endpoint that is no number or is not finite, incompatible units, an undeclared parameter, the case's subject, one the arguments already bind — by name or by holding the position it is bound from — and a plan asking for more runs than `OPENSYSML_MAX_SWEEP_RUNS` allows are errors naming what was asked for. A trade study's rows carry an `evaluations` column, each run's evaluations of its alternatives with the one selected marked, a failed row keeping the ones it made. Same tables as the CLI's [`-sweep`](cli.md#sweeping-a-parameter) |
| `%samples <n> <seed> <name>[(<args>)] [<object>] <parameter>=<from>..<to> ...` | Draw `n` values for each range instead of running every value of it, uniformly over `[<from>, <to>]` between Integers and `[<from>, <to>)` between Reals, and print the rows in draw order with the seed in the table header. The generator is `math/rand/v2`'s `PCG` seeded from `<seed>`, so the same seed draws the same values on every platform; a sampled range needs no step and stating one is refused. Sampling is uniform because the bundled standard library states no probability distribution — a range written as a named distribution (`n=normal(1.0, 0.2)`) is an error naming what is missing rather than an approximation |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%invoke <object> <op> [<p>=<expr>]` | Invoke an operation of an object's type (an action it owns), performed by that object, given as an [object reference](#object-references) (`%invoke car start`, `%invoke #3 start`, `%invoke car.engine start`), with each argument written as `<parameter>=<expression>`. Assignments in the body write that object's feature values; declared outputs are reported. Not yet supported: an operation given as a `calc` or `constraint`, and positional arguments |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor), reporting beside its verdict the `VerdictKind` the body of every verification case verifying it produced |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element, reporting beside each verdict the `VerdictKind` the body of every verification case verifying the requirement produced |
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct. Needs `z3` or `cvc5` on `PATH` (or `OPENSYSML_SMT`; see [installing a solver](../guide/01-install.md#installing-a-solver-optional)) and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` to find out what holds for an object |
| `%explain <name>` | **Experimental.** When `%check` answers `unsat`, ask the solver *which* conditions conflict. Prints an unsat core, reduced to a minimal one, as the role, the condition as written, the declaring element and `file:line:col`, in the query's order. A declared domain (a `Natural` being non-negative) or a division well-definedness guard can be one of the conflicting conditions. On `sat` there is no conflict to explain and `%check` gives the assignment; on `unknown` no explanation is available. Same solver requirements as `%check` ([installing a solver](../guide/01-install.md#installing-a-solver-optional)); `OPENSYSML_SMT_CORE_BUDGET` bounds the reduction |
| `%solve <name>` | **Experimental.** Ask the solver for values that satisfy a constraint, requirement or satisfaction assertion, keeping what is already fixed: the values an object holds, or failing that the ones the model declares, stay fixed and the rest are synthesised. Prints what was fixed (and by what), the values chosen, and a reminder that they are *one* witness of possibly many. `unsat` here means no values exist that are consistent with what is fixed, and names the fixed values that conflict. Same solver requirements as `%check` |
| `%configure <name> [<variation>=<variant>...] [all [<count>]]` | **Experimental.** Ask which variants a constraint, requirement or satisfaction assertion permits. With no argument, one consistent selection is synthesised. With `<variation>=<variant>`, the chosen selection is checked and the conflict is named when it is not consistent. With `all`, the consistent selections are enumerated up to `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a smaller bound), and the report says whether the list is complete or was cut short, either at the bound or because the solver stopped deciding or ran out of time; the selections found so far are still reported. An element that reads no variation point is an error pointing at `%check`. Same solver requirements as `%check` |
| `%optimize <name>` | **Experimental.** Ask the solver for the best values an `analysis def` (or an analysis usage) admits. Each `objective` is improved as the trade-study definition typing it says (`TradeStudies::MinimizeObjective` or `MaximizeObjective`), over the value its redefinition of the library's `eval` calculation returns (`subject :>> selectedAlternative; in calc :>> eval { expression }`), within the conditions the case requires or assumes and the ones the objective states itself. An objective that instead gives the library's bound `best` a value of its own (`attribute :>> best = expression;`, the spelling earlier releases read) is a validation error, and `%optimize` refuses it pointing at the `eval` spelling. Several objectives are improved lexicographically in declaration order, inherited ones first; an objective restating an inherited one (by name or `:>>`) stands in its place with the value stated there. Prints each optimum with its declared unit and the assignment that attains it. An objective that improves without limit, or a bound no assignment attains, is reported as such and never as a number, and every optimum is verified before it is reported. **Needs `z3`**: optimization is a z3 extension, and a backend without it (cvc5) is an error rather than a plain satisfiability check presented as an optimum. Otherwise the same solver requirements as `%check`. `%optimize` answers a different question from running the case: it finds the best *values* the case's conditions admit over a continuous domain, where `%analysis` executes what the model says over the alternatives it lists. A `TradeStudies::TradeStudy` whose objective applies the case's `evaluationFunction` to listed alternatives is therefore refused, pointing at `%analysis`/`-analysis`/`RunAnalysis`, which evaluate every alternative and report the one selected ([Trade studies](../guide/06-behavior.md#trade-studies)) |
| **Action debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%action <name> [<object>]` | Start an action debugging session, optionally performed by an instantiated object, given as an [object reference](#object-references) (`%action tally car`, `%action tally #3`) |
| `%step` | Advance one token step; a token waiting only on the clock (`accept after`, `accept at`) is not stepped, and the report names the `%advance` that would move it |
| `%continue` | Run the action to completion |
| `%tokens` | Show the active tokens |
| `%break <node>` | Set a breakpoint at a node |
| `%stop` | Stop the current debugging session |
| **State machine debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%state <name> [<object>]` | Debug the machine an object exhibits (`%state <object>` after `%instantiate` attaches to that object's own running machine, whether the object is named, `#3`, or `car.controller`), or start a state machine — named, or the object of one the session holds (`%state #2`, `%state monitor.modes`), which exhibits none and so runs afresh — optionally performed by an instantiated object, given as an [object reference](#object-references). `%state <machine> <object>` first looks at what the object already runs: naming a machine it exhibits — one that *is* or is *typed by* `<machine>` (`%state Rover::modes rover`) — attaches to that running machine too, with a note saying so, rather than performing it a second time against the same feature values, so the object never runs two of them; only a machine the object does not exhibit is started as a detached performance, and the report says that too. Naming an exhibited machine alone (`%state Rover::modes`, or its short name `modes`) attaches to the running machine of the one held object exhibiting it — the object `%instances` and `%features` show. Held objects are the ones the session has built: a nested part counts once something has reached it (`%features driver`), and `%state` builds none itself. When no held object exhibits it, or several do, `%state` refuses and names the objects (or, with none held, the types exhibiting it), so that you name one with `%state <object>` or `%state <machine> <object>`; it never guesses, and never performs a machine a type exhibits detached from any object. The machine is addressed by any binding on the way to its body — the exhibited usage, a usage it references, or the definition typing it — so `%state Blink` finds the object exhibiting `spare : Blink`. A machine no type exhibits (a `state def` alone) is started as a detached performance, as there is no object's performance of it to attach to. A definition the object exhibits as the body of several usages names no one machine, so `%state` refuses and names the exhibited usages to name instead. `%step`, `%advance`, `%current` and `%events` then drive that object's machine, and `%features` shows what it wrote |
| `%send <signal>[(<p>=<expr>, ...)] [to <object>]` | Send a signal to an object's machine through the runtime's own message bus, as `send <signal>(...) to <object>` from an action would; the object is an [object reference](#object-references) (`to bulb`, `to #1`, `to rack.lamp`). `<signal>` is a definition the model declares (an `attribute def`, `item def` or other signal-like definition; qualified names allowed), or a bare name an active `accept` matches by name when no declaration types it. Each argument is written `<parameter>=<expression>` as for `%invoke`, evaluated at the prompt, and must name a feature the signal carries with a value that feature admits (its type and multiplicity, checked before anything is sent); a feature left out is left unset. Without `to`, the target is the object whose machine the current `%state` session is debugging, and with no session the command says so rather than guessing. The signal is refused, with the machine's current state, when no machine of the object accepts it there, or when the guard of every transition it triggers is false — decided as the dispatch would decide it, the payload bound, so a guard that reads it is honoured; a guard that cannot be evaluated is an error. A signal the current state defers rather than accepts is sent and reported as deferred: the step dispatching it holds it (`%events` lists it as held) until the machine reaches a state that accepts it, when it is recalled and fires. Otherwise it is in flight (shown by `%events`) until `%step` or `%advance` dispatches it, and the transition it triggers fires as it would for a send from an action; should the state or the data a guard reads change before the dispatch, the step that drops the signal says so. An object running several machines is sent the signal as a whole: `%send` reports each machine that would fire on or defer it, a machine whose guards would drop it leaves it in flight for a sibling that would not, and the report says when the machine being debugged is such a one |
| `%events` | Show the event queue and the signals in flight |
| `%current` | Show the current state and configuration |
| `%advance <time>` | Advance the runtime's simulation clock by `<time>` seconds (`SI::s`), running every state event, action token, change-condition poll and do behavior that comes due, in due order. The clock is the session's, not one debugger's: an `%action` parked at `accept after` and a `%state` machine both move, and the report covers each. Two executors due at the same instant run in the order the scheduling policy picks — the one started last first under the default `reverse` — and the pick is a choice point ([Choice points](../guide/06-behavior.md)) |
| **Control** | |
| `%quit` | Exit the REPL |
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …; a name that needs quoting is offered in quotes, `Q::'the ra` completing to `Q::'the rack'`), object references where a command takes one (`#` offers the ids there are; `car.` offers the object-holding features of `car` — the same ones a path may pass through — a multi-valued one as `car.wheels[1]`, `car.wheels[2]` …; completing reads and materializes nothing, so a part no command has reached yet is offered by type, and only the elements reading it would hold: those the features subsetting it contribute, then anonymous ones up to its lower bound — so an optional part (`spare : Wheel[0..1]`) or an abstract one, which hold only what subsets them, is offered only once something does), the form after `%render <name>`, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |

The five solving commands (`%check`, `%explain`, `%solve`, `%configure`, `%optimize`) follow the
design of the `ConstraintSolverService` in OpenMBEE's [HMF](https://github.com/hivecore-dev/hmf)
(Apache 2.0); see [Acknowledgements](../../README.md#acknowledgements).

## Object references

An object reference names one object the session holds. It is one of:

| Form | Denotes |
|------|---------|
| `car`, `Demo::car` | the object `%instantiate car` created, by the name it was created under (unqualified or qualified, quoted segments included) |
| `#3` | the object whose id `%instantiate` printed as `ID: 3`. Ids count up from 1 and never change: an object keeps its id when a later submission carries it over, and when a second `%instantiate` of its name creates a new object — the name then denotes the new one, and `#3` is how the old one is reached (`%instances` lists it as `#3 (ID: 3, displaced from Demo::car)`) |
| `car.fl`, `#3.fl`, `car.fl.hub` | a path from a named object or an id through the features that hold objects, one nested object per segment: parts, ports, connectors, and structured attributes (an attribute typed by an `attribute def` with attributes of its own, which `%features` shows as `Instance(ID: n)`). An attribute holding a plain value ends a path with an error. `.` and `::` are interchangeable in a path, so `car::fl` and `car.fl` are the same object. They differ only in how the root is found: a segment after `.` is always a feature of the object before it, while `::` may also continue the declared name, so the longest `::`-run naming an object the session holds is the root (`Demo::car::fl` is the object `%instantiate Demo::Car::fl` created, if there is one; `Demo::car.fl` is always car's `fl`). A package before `.` is an error naming the `::` spelling to use |
| `car.wheels[2]` | one element of a multi-valued feature (`part wheels : Wheel[4]`), counted from 1 in the order the feature holds them |

Reading a path materializes the nested objects it passes through, exactly as `%features car` does.

A command reports an object under the reference that reaches it — `Demo::car.fl`, `#3.wheels[2]` —
with every name spelled as the notation writes it, so what is printed can be typed back and reaches
the same object: walked features are written after `.`, which is only ever a feature, even where the
`::` spelling would name a declaration an object was created under. A declared name that only looks
like an id or an index is quoted (`Demo::'#3'`, `car.'hub[2]'`), where a generated id or index is not;
`#3` is always an id and `[2]` always an index. A name holding `::` inside its quotes stays one
quoted segment (`Demo::'left::right'`, `#1.'in::ner'`) rather than flattening into a qualification
that would read back as two names.

Every command reports a bad reference in the same words:

```
sysml> %features #9
error: no object #9 in this session: nothing materialized has that identity (the objects are #1, #2)
sysml> %features car.nope
error: Demo::car has no feature "nope" (its features are fl, mass, wheels, and 13 more the library declares)
sysml> %features car.mass
error: mass of Demo::car holds a value (1500.0), not an object
sysml> %features car.wheels
error: wheels of Demo::car holds 4 objects: pick one by index, wheels[1] to wheels[4]
sysml> %features car.wheels[5]
error: wheels of Demo::car holds 4 objects, so wheels[5] names none (indexes run from 1 to 4)
sysml> %features Wheel
error: no instance of "Demo::Wheel" (use %instantiate first)
sysml> %features car.spare
error: spare of Demo::car could not be materialized: multiplicity violation …
```

The last two are typed errors: a name nothing was instantiated under, and a segment whose feature
value the runtime could not materialize, which keeps the runtime's reason as its cause and is
recorded among the session's materialization failures like any other command's.

A name nothing was instantiated under says what to instantiate when related objects exist. A usage
whose definition alone has an object (`%instantiate Rover` when `%state … rover` wanted the usage) is
reported as `no instance of the usage "Demo::rover": object #1 of "Demo::Rover" is of its definition
"Demo::Rover", not of the usage — use %instantiate Demo::rover to create the usage's object, or name
Demo::Rover to address it`; a definition whose only objects are of usages typed by it names those
objects the same way (`no instance of the definition "Demo::Rover" itself: objects #2 of
"Demo::garage.bays[1]", #3 of "Demo::garage.bays[2]" are typed by it — name Demo::garage.bays[1] or
Demo::garage.bays[2] to address one of them, or use %instantiate Demo::Rover to create an object of
the definition`). A usage reaches its definition through the usages it subsets. Only objects the
session already holds are named, the first five of many (`… (3000 in all)`): the error
materializes nothing to find them.

An id denotes an object the session holds: one it named, one a materialized feature of such an object
holds, an anonymous connector `%features` has shown (`(anonymous connector) = Instance(ID: 4)` — `#4`
is the only way to name it), or one a second `%instantiate` of its name displaced. Instantiating a
name a second time makes a new object and says so — `Demo::car now denotes this object; object #1 is displaced from that name
and stays reachable as #1` — and `#1` goes on reaching the first object on every command, listed by
`%instances` as `#1 (ID: 1, displaced from Demo::car)`. A debugging session over the displaced object
keeps running: the same `%instantiate` notes that it now follows the object as `#1` (or as a path
from that id, `#1.r`, for a nested object), and `%step`, `%advance` and `%continue` go on driving it.
Looking an id up materializes nothing: an id the runtime never issued is `no object #9 in this
session: nothing materialized has that identity (the objects are #1, #2)`, and so is one of an object
made in passing — by `%eval in` on a usage nothing was instantiated under, say — which `%instances`
does not list and `#<id>` completion does not offer. A connector is the one object a submission sets
aside: a declaration that leaves its owner's shape alone keeps the owner and attaches the connector's
ends again when it is next read, and the id `%features` printed for it — anonymous or named — goes on
reaching it and being offered by completion in the meantime, naming it being what reads it — that
one alone, its sibling connectors waiting for their own turn. The id follows the connector's own
declaration, however the owner's connectors are reordered or added to around it; the id of one whose
declaration is gone is gone with it, never handed to another. An end that cannot be attached again is
reported with the id, and the id stays reachable for another attempt: `no object #4 in this session:
the connector that had that identity cannot be materialized again: …`. A connector attached whole is
kept — its ends, its writes, its behaviors — even when an older object's behavior then fails
answering it; that failure is reported as the older object's (`#4 is materialized again, but an older
object's behavior failed: …`), and the next command finds `#4` held.

## Rendering a view

```
sysml> package Demo {
  ...>     private import ScalarValues::*;
  ...>     private import Views::*;
  ...>     part def Wheel {
  ...>         attribute diameter : Real = 16.0;
  ...>     }
  ...>     part def Vehicle {
  ...>         attribute mass : Real;
  ...>         part wheel : Wheel;
  ...>     }
  ...>     part vehicle : Vehicle {
  ...>         attribute :>> mass = 1200.0;
  ...>     }
  ...>     concern def MassBudget {
  ...>         subject s : Vehicle;
  ...>         attribute maxMass : Real = 1000.0;
  ...>         require constraint {
  ...>             s.mass < maxMass
  ...>         }
  ...>     }
  ...>     concern def Modularity {
  ...>         subject s : Vehicle;
  ...>         require constraint {
  ...>             1 < 2
  ...>         }
  ...>     }
  ...>     viewpoint def StructurePerspective {
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     viewpoint structure : StructurePerspective;
  ...>     view def StructureView {
  ...>         satisfy structure;
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     view report : StructureView {
  ...>         expose vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view summary : StructureView {
  ...>         expose Vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view parts {
  ...>         expose Vehicle;
  ...>         render asElementTable;
  ...>     }
  ...> }
✓ package Demo

sysml> %render Demo::summary
Demo::summary - tree rendering (the view states no rendering; a tree is the default)

part def Demo::Vehicle
  attribute mass (Real)
  part wheel (Wheel)
view Demo::summary::detail
  part def Demo::Wheel
    attribute diameter (Real)

sysml> %render Demo::summary mermaid
%% Demo::summary — tree rendering
flowchart TD
  n0["part def Demo::Vehicle"]
  n1["attribute mass (Real)"]
  n0 --- n1
  n2["part wheel (Wheel)"]
  n0 --- n2
  n3["view Demo::summary::detail"]
  n4["part def Demo::Wheel"]
  n5["attribute diameter (Real)"]
  n4 --- n5
  n3 --- n4
```

A view that states `render asElementTable;`, or is typed by `StandardViewDefinitions::GridView`,
renders as rows instead: the exposed elements, the elements declared in them, and the views nested
in the rendered one, as aligned columns at the prompt and as a Markdown table with `markdown`.

```
sysml> %render Demo::parts
Demo::parts - table rendering (render asElementTable)

Element        Kind       Type   Declared in
-------------  ---------  -----  -------------
Demo::Vehicle  part def
mass           attribute  Real   Demo::Vehicle
wheel          part       Wheel  Demo::Vehicle

sysml> %render Demo::parts markdown
<!-- Demo::parts — table rendering (render asElementTable) -->
| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| Demo::Vehicle | part def |  |  |
| mass | attribute | Real | Demo::Vehicle |
| wheel | part | Wheel | Demo::Vehicle |
```

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form for the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has a dedicated state diagram grammar. A table is a Markdown table, since Mermaid has no grammar
for tables. A state rendering reads the lowered state graph and an action rendering reads the
lowered action graph, so what is drawn is what the runtime executes.

To render a view outside the prompt, use [`sysml -render`](cli.md#rendering-a-view).
