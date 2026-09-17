# SysML CLI Usage Examples

## OSLC element queries

`-query <oslc-query>` loads the model given on the command line and evaluates OSLC Query text
against it, printing one matched element per line as qualified name and metamodel type,
followed by any selected properties. Like `-convert`, it is a mode that inspects the model
instead of running it.

```bash
sysml -query 'oslc.where=sysml:name="wheel"' model.sysml
```

## Interactive Mode (Default)

Start the REPL with no arguments:

```bash
sysml
```

Load files and enter interactive mode:

```bash
sysml model.sysml
sysml types.sysml instances.sysml
```

## Non-Interactive Mode

Execute expressions and exit without entering interactive mode.

### Basic Evaluation

Evaluate an expression:

```bash
sysml -e "5 + 3"
# Output: ✓ 5 + 3
#           = 8
```

### Load Files and Evaluate

Load a model first, then evaluate:

```bash
sysml -e "someAttribute" model.sysml
```

**Note:** flags may be written before or after the files — `sysml model.sysml -e "x"`
and `sysml -e "x" model.sysml` do the same thing.

An `-e` expression is evaluated at model level: over what the declarations state,
not over an object. A feature the model leaves open — an attribute with no value,
or a `[1..*]` or `[0..2]` feature whose count the model does not fix — reads as the
value `<undetermined>`, and so does every expression whose answer depends on it
(`u + 5`, `u > 3`, `size(rack.gear)`, `isEmpty(rack.loose)`). What the model does
fix still answers: `(u > 3) and false` is `false`, `notEmpty(rack.gear)` is `true`,
`size(rack.slots)` for `part slots[3]` is `3`. `<undetermined>` is a value, so the
run exits `0`; a name nothing declares is still an `unresolved reference` and exits
`2`. To read what an object holds, instantiate it (`-instantiate`), where a valueless
feature shows `<unset>` and multiplicity minimums are materialized:

```bash
$ sysml -e "T::u" -e "(T::u > 3) and false" -e "size(T::rack.gear)" model.sysml
✓ package T
✓ T::u
  = <undetermined>
✓ (T::u > 3) and false
  = false
✓ size(T::rack.gear)
  = <undetermined>
```

### Multiple Evaluations

Evaluate multiple expressions in sequence:

```bash
sysml -e "x" -e "y" -e "z" model.sysml
```

### Multiple Files

Load multiple files before evaluating:

```bash
sysml -e "result" types.sysml instances.sysml
```

## Real-World Examples

### 1. Quick Calculation

```bash
sysml -e "10 * 2 + 5"
# Output: ✓ 10 * 2 + 5
#           = 25
```

### 2. Validate Model and Check Constraint

```bash
sysml -e "speedLimit < 120" vehicle-model.sysml
```

### 3. Extract Calculated Values

```bash
# model.sysml contains: attribute totalCost = partCost + laborCost;
sysml -e "totalCost" model.sysml
# Output: ✓ totalCost
#           = 1500.0
```

### 4. Batch Processing

```bash
#!/bin/bash
for model in models/*.sysml; do
    result=$(sysml -e "result" "$model" 2>&1 | grep "=" | awk '{print $2}')
    echo "$model: $result"
done
```

### 5. CI/CD Integration

A pipeline can gate on the exit status: an expression that could not be evaluated
exits `2`, so anything left on stdout is a value you can compare (see
[Exit status](#exit-status)). Compare it literally: a feature the model leaves
open prints the value `<undetermined>` with status `0`, which is not the number
you expected either:

```bash
# Check that a calculated value matches what is expected
expected=42
actual=$(sysml -e "designParameter" design.sysml | awk '/^ *=/ {print $2}') || exit $?
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual" >&2
    exit 1
fi
```

Constraint, requirement and satisfy verdicts gate the same way, and report
themselves:

```bash
sysml -satisfy -constraint MassBudget design.sysml   # 0 held, 1 answered false, 2 undecided
```

So does an object validated as a whole — every assertion it and the parts it
holds carry, in one exit status. Over a `Car` that asserts `massOk`, carries the
requirement `light` and holds two `wheels : Wheel[2]` whose `Wheel` asserts
`pressureOk`:

```bash
$ sysml fleet.sysml -instantiate Fleet::car -validate=Fleet::car; echo $?
✓ package Fleet
✓ Created instance of Fleet::car
  ID: 1
  Use %features Fleet::car to inspect
✓ assert constraint massOk holds (on Fleet::car ID: 1)
✗ requirement light fails (on Fleet::car ID: 1)
  Required condition evaluated to false: mass < 1000.0
✗ assert constraint pressureOk fails (on Fleet::car.wheels[1] ID: 2)
  Assertion evaluated to false: pressure >= 30.0
✗ assert constraint pressureOk fails (on Fleet::car.wheels[2] ID: 3)
  Assertion evaluated to false: pressure >= 30.0
✗ Fleet::car is not valid: 3 of 4 assertions fail
  standing: violated (witnessed: 1 run under reverse)
1
```

Under `-json` each verdict is a check of its own, `subject` naming the
assertion and the object (`assert constraint pressureOk on Fleet::car.wheels[2]`),
and the last check is the object's, carrying the `plan` and `results` of the
engine that answered.

### 6. Use REPL Meta Commands

Load a file and use meta commands:

```bash
echo "%load model.sysml
%instantiate Vehicle
%features Vehicle
%eval speedLimit" | sysml
```

A whole run is read out the same way. `%features <name>` is bounded, since reading a
feature value builds the objects it holds; `all` lifts the bound and `json` writes the
graph in the shape the API's `Instantiate` returns, so a piped session is how a script
gets the complete state of a large object tree:

```bash
printf '%%instantiate Plant::Context\n%%features Plant::Context all\n' | sysml model.sysml
printf '%%instantiate Plant::Context\n%%features Plant::Context all json\n' | sysml model.sysml \
  | sed -n '/^{/,$p' | jq '.instances | length'
```

The JSON document is written on the listing's own lines, after whatever the load
reported, so a script that reads it takes the output from the first `{`.

## Command Reference

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--eval <expr>` | `-e` | Evaluate expression and exit (repeatable) |
| `--debug` | | Report every diagnostic over the whole session buffer, with the pass that produced it |
| `--quiet` | | Report errors only, suppressing warnings |
| `--strict` | | Judge the model as conforming SysML v2: notation no pinned production admits is an error, not a warning (see [Strict conformance](../guide/03-command-line.md#strict-conformance)) |
| `--trace` | | Report each execution step: expression evaluation, calc invocation, action tokens, state transitions, each `choice` the executor made among alternatives the library leaves unordered, naming the alternatives and the one taken, and each `unevaluable guard` it read only to report one and could not evaluate ([Choice points](../guide/06-behavior.md)). Under `-schedule explore` the table is printed first, then the trace of one witness run per distinct outcome, each under a `trace of outcome <n>'s witness (run <r>):` heading ([Exploring every linearization](#exploring-every-linearization)) |
| `--convert <format>` | | Convert the model instead of running it: `sysml`, `kerml`, `ttl`, `turtle` or `rdf`. RDF is [experimental](rdf-mapping.md#status-experimental) and every run that converts it says so on stderr (see [the RDF mapping](rdf-mapping.md)) |
| `--from <format>` | | Input format for `--convert`: the `--convert` formats, or `xmi`/`uml`/`mdzip` for a SysML v1 model to migrate (experimental; default: from the input's extension; `.xmi`, `.uml` and `.mdzip` are recognized) — see [SysML v1 migration](sysml-v1-migration.md) |
| `--migration-report <file>` | | With `--convert` from `xmi`: write the element-by-element migration report to this file, JSON when it ends in `.json`, text otherwise. Without it the one-line summary goes to stderr |
| `--render <view>` | | Render this view of the model (every file named, loaded as one) instead of running it, in the form its `render` member states (see [Rendering a view](#rendering-a-view)) |
| `--render-all <dir>` | | Render every declared view into the directory, one artifact per view |
| `--render-form <form>` | | Form `--render` or `--render-all` writes: `text`, `mermaid`, `markdown`, `dot` or `plantuml` (default: destination-dependent for `--render`, each kind's machine-readable form for `--render-all`) |
| `--render-palette <name>` | | Palette the `dot` or `plantuml` form of `--render` or `--render-all` fills nodes with, by keyword family: `okabe-ito`, `tol-bright`, `tol-muted`, `tol-light`, `brewer-set2`, `brewer-dark2`, `viridis` or `cividis`; black and white when absent. Mermaid notes it as not represented; text and Markdown ignore it. An unknown name is refused with the names there are (see [Rendering a view](#rendering-a-view)) |
| `--render-document <name>` | | Compile a document definition (a `part def` specializing `DocumentQueries::Document`), run its queries against the model, render its diagram blocks through the view engine and write the result as CommonMark Markdown, as `%render-document` does. Paragraphs may hold inline runs (`Span` with a `plain`/`emphasis`/`strong`/`code` style, `Link` to a URL, `Ref` linking to another content block's anchor); a query-backed paragraph or list styles its projected values through nested `SpanColumn`/`LinkColumn` column runs; a table with a `groupBy` column writes one subtable per group value, with the query's projected properties and computed `Column` names as its columns. A `Diagram` block embeds a declared view, or an element with a stated rendering kind, as a fenced ` ```mermaid ` block (a fenced ` ```dot ` block of Graphviz DOT under `-diagram-form dot`, a ` ```plantuml ` block under `-diagram-form plantuml`; a table-kind view as a pipe table whichever form), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown is the default form; `-doc-form html` renders the same document tree as semantic HTML (see [Rendering a document as HTML](#rendering-a-document-as-html)) and `-doc-form pdf` converts the Markdown (see [Rendering a document as PDF](#rendering-a-document-as-pdf)). Combined with `--instantiate`, the document's queries run over the objects created (see [Rendering a document over objects](#rendering-a-document-over-objects)). `-json` does not apply. See the [document generation manual](../manual/README.md) |
| `--doc-form <form>` | | Form `--render-document` writes: `markdown` (default), `html`, rendered from the document tree itself (see [Rendering a document as HTML](#rendering-a-document-as-html)), or `pdf`, which drives an external converter |
| `--diagram-form <form>` | | Form the graph-shaped diagram blocks of `--render-document` and `--render-documents` are written in: `mermaid` (default), `dot`, Graphviz DOT for a toolchain that lays diagrams out with Graphviz, produced without Graphviz installed, or `plantuml`, PlantUML in the Pilot visualizer's B&W style, produced without a PlantUML jar. Applies to every diagram of the document in every `--doc-form`; a table-kind view is a table whichever form, and a `sequence` diagram, which has no DOT form, is refused under `dot` |
| `--render-documents <dir>` | | Render every document definition the model declares as a linked set into the directory, one file per document, so cross-document references resolve on disk. `--doc-form html` writes the set as HTML pages linking shared stylesheet files written beside them |
| `--doc-title-page` | | Put the document title on a page of its own (`--doc-form html` or `pdf`) |
| `--doc-toc` | | Write a table of contents ahead of the content (`--doc-form html` or `pdf`) |
| `--doc-number-sections` | | Number the section headings hierarchically (`--doc-form html` or `pdf`) |
| `--html-theme <name>` | | Style the HTML page with a bundled theme layered over the default stylesheet: `default`, `modern`, `print` or `report` (default: the default stylesheet alone) |
| `--html-css <file\|url>` | | Style the HTML with this stylesheet: a file is inlined in a single page and written beside a set's pages, a URL is linked. Repeatable, applied in order after the default sheet (`--doc-form html`) |
| `--html-no-default-css` | | Leave the default stylesheet out, so only `--html-css` sheets style the document |
| `--html-default-css` | | Write the default document stylesheet and exit, as a starting point for your own; with `--html-theme`, the theme's whole sheet |
| `--html-fragment` | | Write the document element alone, without the page shell or a stylesheet, to embed in a page of your own |
| `--html-mermaid <cdn\|url>` | | Have the HTML page load Mermaid to draw its diagrams: `cdn` loads a pinned release from jsDelivr, a URL loads the script it names (default: diagrams stay Mermaid source) |
| `--html-math <cdn\|url>` | | Have the HTML page load MathJax to typeset its formulas: `cdn` loads a pinned release from jsDelivr, a URL loads the script it names (default: formulas stay LaTeX source) |
| `--pdf-engine <engine>` | | Converter `--doc-form pdf` drives: `weasyprint` (default), `pandoc` or `prince` |
| `--pdf-title-page` | | Alias of `--doc-title-page` |
| `--pdf-toc` | | Alias of `--doc-toc` |
| `--pdf-number-sections` | | Alias of `--doc-number-sections` |
| `--output <file>` | `-o` | Write the conversion, the rendering or the rendered document to a file instead of stdout |
| `--version` | `-v` | Show version information |
| `--help` | `-h` | Show usage information |
| `--man` | | Write this command's manual page, in roff, to stdout (see [Installing](../guide/01-install.md)) |

Check flags, each repeatable. `-instantiate` runs first whatever order the flags are
written in, so the verdicts are about that object:

| Flag | Checks |
|------|--------|
| `-validate` | Only that the model analyses cleanly and that the objects `-instantiate` asked for could be built; it says nothing about the model's constraints |
| `-validate=<object>` | Every assertion about an object `-instantiate` created and the objects it holds, as `%validate` does: each `assert constraint` the carrier's type declares or inherits, each requirement usage it carries and each `satisfy` assertion whose subject is in the tree, one verdict per assertion per object, root first and then each held object as the walk reaches it (`Fleet::car.wheels[2]`), then one verdict about the object as a whole — valid only when every assertion holds and every held object was reached, so an assertion that could not be evaluated or a walk cut short by an object graph without end leaves it undecided rather than valid, as does an object no assertion is about (`states no assertion to validate`, exit status 2). The object is named as `%validate` names it: the usage's name, a feature path to a part it holds (`Fleet::car.engine`), or the id the report prints (`#2`). A constraint declared without `assert` is not swept; name it with `-constraint`. Repeatable; `-validate=false` asks for nothing and withdraws a bare `-validate` written before it, as `-satisfy=false` does |
| `-constraint <name>` | One constraint, as `%constraint` does |
| `-requirement <name>` | One requirement, as `%requirement` does, with [the verdict of every verification case](#verification-case-verdicts) verifying it beside its own |
| `-satisfy` | Every satisfaction assertion the model states, with [the verdict of every verification case](#verification-case-verdicts) verifying the requirement beside each |
| `-satisfy=<name>` | Only the assertions the named element states (`-satisfy=false` asks for none) |
| `-instantiate <name>` | Creates an object first, so the verdicts are about it; with `-run-query` or `-render-document`, so the query reads it ([Objects the session holds](../manual/query-cookbook.md#objects-the-session-holds)). Under `-schedule explore`, `-engine check`, `smt` or `all` each run creates an object of the declaration of its own before its behaviors start, one per `-instantiate` as the session holds one per `-instantiate`, which a `-state` or `-action` named alone attaches to and a path such as `Mission::mission.vehicle` walks into ([Objects an exploration runs on](#objects-an-exploration-runs-on)) |
| `-calc "<name>(<args>)"` | Invokes a calculation and reports what it computed |
| `-analysis "<name>[(<args>)] [object]"` | Runs an analysis or [verification](#verification-case-verdicts) case — a [trade study](#trade-studies) included — and reports its `out` and `return` values with their units, then the verdict of its `objective` — `satisfied`, `not satisfied` with the violated condition, or `undecided` with the reason — as `%analysis` does. An objective typed by a requirement def binds the def's subject as a requirement usage does (`subject = ship;`, `subject s = ship;` or `subject :>> s = ship;`); one binding none checks the case's result, the library's default for it, and is `undecided` naming the type when that result is not of the subject's type. Arguments bind the case's `in` parameters, positionally (`Pkg::Case(3.0)`) or by name (`Pkg::Case(limit = 3.0)`); the object, one `-instantiate` created and named as `-state` names its performer, is the case's `subject`. A usage that binds its subject (`subject s = ship;`) needs no object; a definition, or a usage that binds none, is refused by name without one. A verification case runs the same way and reports beside those verdicts the `VerdictKind` its body produced. Repeatable |
| `-run-query "<name> [<p>=<expr>...]"` | Executes a document query and reports its rows, as `%run-query` does — including any computed `Column(name = "<column>", expression = <expr>)` projections evaluated per row. Each binding is written as `<parameter>=<expression>`; a name binds the object `-instantiate` created under it while the run holds one (`#2` and `car.wheels[2]` bind an object by id and by path), and the element otherwise. A query over `Verdicts` reports each row as `<assertion> on <path>: <verdict>` ([Which constraints and requirements hold](../manual/query-cookbook.md#which-constraints-and-requirements-hold)). The queries run after `-state`, `-action` and `-advance` have run, so `States`, `InState` and `Events` read where the run left the objects and, with `-trace`, what it recorded — a state row as `<object>.<machine> in <statePath>`, an event row as `t=<instant> <object>.<machine>: <text>` ([Where the objects stand and what they did](../manual/query-cookbook.md#where-the-objects-stand-and-what-they-did)) |
| `-action "<name> [object]"` | Runs an action to completion and reports its outputs, on the object named as `-state` names its performer when one is; under `-schedule explore` each run performs it on an object of its own ([Objects an exploration runs on](#objects-an-exploration-runs-on)) |
| `-state "<name> [object]"` | Runs a state machine and reports where it settled. The object is one `-instantiate` created, named as `%state` names it: a usage's name, a feature path to a part it holds (`Fleet::driver.r`), or the id the report prints (`#2`). Naming the machine the object exhibits attaches to its running machine rather than performing it again (a definition exhibited as several usages is refused with the usages to name instead); naming a usage whose definition alone was instantiated says which usage to `-instantiate`. Under `-schedule explore` the object is one each run creates of its own: a definition or usage to instantiate, a path from one into a part it holds (`Mission::mission.vehicle`, `Fleet::fleet.rovers[2]`) or, named alone, the run's one `-instantiate` object exhibiting the machine ([Objects an exploration runs on](#objects-an-exploration-runs-on)) |
| `-advance <time>` | Simulated time (seconds, `SI::s`) the invocation's `-action` and `-state` behaviors run for, on the one clock they share: every state event, action `accept after`/`accept at` and do behavior due within it runs, in due order — a state's do behavior parked at an `accept after` of its own action body among them — and two behaviors due at the same instant run in the order `-schedule` picks (the one started last first by default), reported as a choice point. A state machine takes only its initial transition without it; an action runs to completion on its own without it and, with it, only as far as that much time takes it, so one still waiting on the clock is reported as undecided with the instant it waits for. Refused without an `-action` or `-state` to run |
| `-sweep <param>=<from>..<to>[:<step>]` | Runs the `-analysis` case or `-calc` once per value of the range, rather than once, and reports the runs as a table. `<from>`, `<to>` and `<step>` are written as an argument is, units included (`0.0 [SI::m]..10.0 [SI::m]:2.0 [SI::m]`); the parameter is one the case or calc declares and the arguments do not bind, and the values are produced in its declared type (`1..4:1` over a `Real` binds `1.0`, `2.0`, …). Repeatable: several ranges run their cartesian product, the first flag given varying slowest. See [Sweeping a parameter](#sweeping-a-parameter) |
| `-samples <n>` | Draws `n` values for each `-sweep` range instead of running every value of it, uniformly over the range from the seed `-seed` names — Integers inclusively for a parameter taking Integers, reals in `[<from>, <to>)` for one taking reals |
| `-seed <s>` | The seed the model's own draws come from in every run the invocation makes, whatever `-schedule` — the branch a `@Probability`-weighted decision takes, the value a `RandomFunctions` call returns — and the seed `-samples` and `-runs` draw from, required with those two: the same seed draws the same run or table on every platform. Without it a run that must draw is refused naming the call and the flag, and a weighted decision takes its most probable branch. See [Running an action many times](#running-an-action-many-times) |
| `-runs <n>` | Runs the one `-action` to completion `n` times, each on a fresh context with a model seed of its own derived from `-seed` and the run number, and tables what each run's `-observe` features came to with a distribution of each; needs `-seed` and exactly one `-action`, and is refused with `-sweep`, `-samples`, `-advance`, `-state` or the checker's flags. See [Running an action many times](#running-an-action-many-times) |
| `-observe <feature>` | A feature of the `-runs` action to table, or `clock` for the simulation time each run completed at (the clock's name, never a feature's); repeatable; default every feature the action holds and the clock. A name the action does not hold, or one named twice, is refused; the flag without `-runs` is refused |
| `-schedule <policy>` | The scheduling policy every run this invocation starts — `-action`, `-state`, `-analysis`; a calc's body performs nothing, so `-calc` has no choice to make — resolves its [choice points](../guide/06-behavior.md) under: `reverse` (the default: reverse token order, first holding guard, first enabled transition), `declared` (spawn and declaration order), `seed:<n>` (a pseudo-random order the non-negative integer `n` fixes, the same on every platform) `explore[:runs=N,depth=D]` (every linearization within the budget, tabled by distinct outcome — see [Exploring every linearization](#exploring-every-linearization)) or `replay:<file>` (the `input <feature> = <value>` lines of a witness, which pin those features before the run starts, then its choice lines, one per line up to the first blank line, followed move for move and then `reverse`'s picks one token a step — a header of `no choice points`, as the checker writes for a run that met none, follows the one run there is; a move the run cannot make — a pick not offered, a step already passed, a line left over at the end — is `replay refused: move <n> (<the choice>): <what the run faced>`, an input line naming a feature the action does not have is refused naming it, and the check is *not covered*; see [Running one witness again](../guide/06-behavior.md#running-one-witness-again)). Every choice point the run reaches is reported and the `took …` in each is what the policy took; another policy's run may reach other choice points, so their count is not fixed across policies. A spelling naming no policy — an unknown name, `seed` or `seed:` without a number, `seed:-1`, `seed:abc`, `explore:` with nothing after the colon, `explore:runs=0`, `explore:depth=-1`, an option named twice, `replay` or `replay:` without a file, a replay file that cannot be read, is empty or has a line spelling no choice — is refused before anything runs |
| `-check-property <name>` | With `-engine check` or `-engine all`: a constraint or requirement the checker evaluates at every stable state of the invocation's behaviors, on the performing object where there is one, reporting a schedule at which it is false; repeatable. See [Checking every schedule of an action or a state machine](#checking-every-schedule-of-an-action-or-a-state-machine) |
| `-check-diverge <feature>` | With `-engine check`, `-engine smt` or `-engine all`: a feature whose final value is compared across schedules, so the question put to the engine is whether it is *sensitive* to the schedule — `x` for the action's attribute, `step.out` for an output of a node it performs, `this.level` for the performing object's, `finalState` for a machine's resting state, `<behavior>.<feature>` and `<behavior> finalState` for one of several behaviors checked together; repeatable; a name nothing holds is refused. Under `check` a feature a schedule leaves unset ends as `<unset>`, and absent the flag every attribute of the behaviors and of the performing object and a machine's `finalState` are compared (an action run without an object has its own attributes only); under `smt` the feature is an action's alone, decided by a two-copy query, and the performing object's features are *not covered* until they are encoded |
| `-check-input <feature>` | With `-engine smt` or `-engine all`: a feature of the action the solver leaves free in its declared type's domain although the model binds it — a default, a value the performing object holds — as `-check-input inletTemp`; repeatable. A name that is not a feature the action reads is refused naming it. Without the flag every input the model leaves unbound is free and every bound one is pinned at its value. See [Deciding a property over the inputs](#deciding-a-property-over-the-inputs) |
| `-check-assume <name>` | With `-engine smt` or `-engine all`: a constraint or requirement asserted over the initial state of the action, as `-check-assume Plant::EnvelopeLimits`; repeatable. One the translator cannot encode is refused naming the construct; a set no initial state satisfies is reported *not covered*, never *proved* |
| `-check-witness <dir>` | With `-engine check`, `-engine smt` or `-engine all`: write a witness file into this directory for each violation and each divergent value — the inputs the solver chose (`input <feature> = <value>`, one per line), the schedule's choice lines, a blank line, then the run's trace, and for a deadlock or a failure a blank line and `fails: <the error>` last — which `-schedule replay:<file>` and `%replay` follow. The directory is created if absent |
| `-check-depth <n>` | With `-engine check`, `-engine smt` or `-engine all`: the most moves one schedule may make before the search backtracks (default 10 000), or the moves the `smt` engine unrolls the action to (default 40), named as the `depth` bound when it is hit; a positive integer |
| `-check-unroll <n>` | With `-engine smt` or `-engine all`: the most iterations of one loop the `smt` engine unrolls before it stops (default 4), named as the `unroll` bound when it is hit; a positive integer |
| `-check-states <n>` | With `-engine check` or `-engine all`: the most distinct states the search may visit (default 1 000 000), named as the `states` bound when it is hit; a positive integer. It is the shared `runs` budget in the checker's unit, so under `-engine all` the one figure is also an exploration's linearizations |
| `-check-timeout <duration>` | With `-engine check`, `-engine smt` or `-engine all`: the wall clock the check's plan may run for, as `30s` or `2m`, and the time each of the `smt` engine's solver queries may take, in place of `OPENSYSML_SMT_TIMEOUT`; a search the clock stops is reported `incomplete: time` with the states and depth it reached, not as a verdict, and exits 2. Unbounded by default, the solver's queries at `OPENSYSML_SMT_TIMEOUT` |
| `-engines` | Lists the analysis engines this build knows — name, kind, protocol, authority, the question kinds each answers and its status — and exits, without a model and without starting a process: the external engines of `OPENSYSML_ENGINES` and the tools of `OPENSYSML_TOOLS` are listed from their manifests alone, each followed by a line naming its file and command. See [Analysis engines](#analysis-engines) |
| `-probe` | With `-engines`, also start each external engine once, check its `describe` against its manifest entry field by field and report the outcome as its status (`ready (…; describe agrees)`, or the first field that disagrees). See [External engines](external-engines.md) |
| `-engine <name>\|auto\|all` | The analysis engine every check of the invocation is put to. `auto` (the default) picks the engine of highest authority covering the question and advances past one that refuses or answers *not covered*, reaching an external engine only after every built-in one has; a name (`run`, `explore`, `check`, `smt`, `sweep`, `solve`, or an external engine's) puts the question to that engine alone, and its refusal is the answer; `all` puts it to every engine covering it, one after another in name order, and composes their answers. A name no engine is registered under is refused before anything runs. `-engine explore` explores as `-schedule explore` does; `-engine check` searches every schedule of each `-action` for a violation, a deadlock, a failure or a divergence ([Checking every schedule of an action](#checking-every-schedule-of-an-action-or-a-state-machine)); `-engine smt` decides a `-check-property` over every schedule and every value of the free inputs with an SMT solver ([Deciding a property over the inputs](#deciding-a-property-over-the-inputs)). See [Analysis engines](#analysis-engines) |
| `-jobs <n>` | Runs of one check that may go concurrently — the linearizations of an exploration, the rows of a `-sweep`/`-samples`, the engines `-engine all` consults — each on a worker of its own over the shared model. `n` is a positive integer; the default is `OPENSYSML_JOBS`, else the number of CPUs. The result of a check is the same at any count: the outcome table, the witness, the run count and the cut a violation makes are those of the runs taken one at a time in plan order. See [Running in parallel](#running-in-parallel) |
| `-json` | Reports the checks as one JSON document rather than as lines. Each check carries its `plan` and `results[]` beside the fields it always carried ([Analysis engines](#analysis-engines)) |

Other modes, each described in full by `sysml -help` and the manual page:

| Flag | Does |
|------|------|
| `-query <oslc-query>` | Evaluates [OSLC Query text](oslc-query.md) against the model instead of running the REPL |
| `-compile <name>` | Compiles the named `calc def` to a native executable named by `-o`; `-target c` (default) or `go` picks the backend, and `-source` writes the generated source to `-o` instead of building it. What else compiles is in [Native compilation](../project/native-compilation.md) |
| `-sync-diff <repo>` | Shows the change set between the model and a repository — a graph file (`.ttl`) or a SysML v2 API endpoint URL — keyed by effective element id, and never writes. `-sync-base <file>` names the repository graph at the last-seen commit so repository changes since then surface as conflicts; `-sync-confirm-deletes` confirms repository-side deletes, which the diff otherwise reports but refuses to apply; `-sync-mint-ids` mints a UUID for each unannotated element being created, and `-sync-annotate <file>` writes the model with each minted id declared as an `@ElementId` annotation |
| `-sync-apply <url>` | Applies the change set to the model's project branch at the SysML v2 API endpoint, refusing one the dry run would have flagged, then records the commit in the sync state (`-sync-state <file>`, default `<model>.sync.json` beside the model). The token comes from `FLEXO_INTEROP_TOKEN` |
| `-memstats`, `-cpuprofile <file>`, `-memprofile <file>` | Report on stderr what the run cost — wall time, memory allocated, memory taken from the OS — or write a CPU or heap profile for `go tool pprof` |
| `-to <format>` | Replaced by `-convert`, which names the output format |

**Arguments:**
- `[file...]` - SysML files to load (loaded in order)

**Usage pattern:**
```
sysml [options] [file...]
```

Flags may be written before or after the files. `--` ends the flags, so a file whose
name looks like a flag can be given after it: `sysml -trace -- -m.sysml`.

A `<name>` is written as the notation writes it, quotes included where the declaration
needs them: `-instantiate "T::'SA-506'"`, `-requirement "Reqs::'HLR-R001'"` (the shell's
double quotes keep the single quotes). A flag that reads its argument as a name alone also
finds the declaration under the bare spelling, but wherever the text is an expression — `-e`,
the arguments of `-calc` and `-analysis`, a `-sweep` endpoint — `T::SA-506` reads as the
subtraction `T::SA - 506`. When the identifier that was read names nothing but starts a declared
name that does need quotes, the failure offers that name and states the rule; the offer is drawn
from the declarations in scope, never from the rest of the text:

```
$ sysml -e T::SA-506 model.sysml
✓ package T
sysml: evaluation failed: unresolved reference: T::SA — did you mean T::'SA-506'? Names containing '-' must be quoted.
$ sysml -instantiate T::SA model.sysml
✓ package T
sysml: unresolved reference: T::SA — did you mean T::'SA-506'? Names containing '-' must be quoted.
```

### Verification case verdicts

A `verification def` or `verification` usage runs the way an analysis case does —
the same subject and `in` bindings, the same body of `action`, `perform` and
nested case steps — and reports in addition the `VerdictKind` its body produced:

```bash
$ sysml -analysis Landing::checkSlow model.sysml
✓ package Landing
✓ Landing::checkSlow
  result = VerdictKind::pass
  objective obj: satisfied
  ✓ Verification Landing::checkSlow verdict: pass
```

The body verdict is beside the objective's, not instead of it: an objective
stating no condition to check — `objective { verify touchdown; }` with no
`require constraint` of its own — stays `undecided`, and the case is reported
unresolved however its body came out.

The verdict is what running the body answers, not a separate judgement of the
model: a body whose result is a `VerificationCases::PassIf(...)` call is `pass`
or `fail` as that library calculation computes it, a body binding `verdict` to a
`VerdictKind` literal reports that literal, a body producing no verdict value is
`inconclusive`, and a body whose run could not be carried out is `error` carrying
the reason. Each nested `verification` step is reported on its own line, marked
`(subcase)` and carrying `"subcase": true` in the JSON report: the library
states no roll-up of a subcase's verdict into its parent's.

`-requirement` and `-satisfy` report the same verdicts beside their own. The
requirement verdict stays what the requirement engine decided — a failing
verification body does not turn a satisfied requirement into a violated one:

```bash
$ sysml -requirement Landing::touchdown model.sysml
✓ package Landing
✓ Requirement Landing::touchdown satisfied
✓ Verification Landing::checkSlow verdict: pass
✗ Verification Landing::checkFast verdict: fail
```

With `-json` each check carries them as a `verifications` array of `case`, `kind`
and, for a verdict that decided nothing, `detail`.

### Trade studies

A `TradeStudies::TradeStudy` runs through `-analysis` as any case does: the library's own
expressions apply the case's `evaluationFunction` to each alternative the subject lists, in
subject order, and `selectOne` returns the first whose score is the objective's `best`. Beside
the outputs and the objective's verdict the report lists each evaluation the run made of the
case's own calc as a value — the alternative, what it computed, `[selected]` on the one the case
returned and `[tied]` on any later one that scored the same:

```bash
$ sysml -analysis Trade::lightest trade.sysml
✓ package Trade
✓ Trade::lightest
  selectedAlternative = Trade::b (object #2)
  objective tradeStudyObjective: satisfied
  evaluationFunction(Trade::a (object #1)) = 30.0
  evaluationFunction(Trade::b (object #2)) = 10.0 [selected]
  evaluationFunction(Trade::c (object #3)) = 10.0 [tied]
```

An evaluation that fails for one alternative is listed with its error, the earlier ones keep
their values, nothing is `[selected]`, the objective is `undecided` naming the failure and the
run fails with status 2 — as does an `evaluationFunction` with no body, at the first alternative,
naming the calc that has no return expression. A subject listing no alternative, or redeclared
`[1]` and bound to several, is a `multiplicity violation` before any is evaluated.

With `-json` each check carries them as an `evaluations` array of `function`, `arguments`,
`result` or `error`, and `selected`/`tied` where true. A [swept](#sweeping-a-parameter) trade
study carries the same per row: an `evaluations` column in the table, and an `evaluations` array
in each JSON row, a failed row keeping the evaluations it made. See
[Trade studies](../guide/06-behavior.md#trade-studies) in the guide.

## Examples

```bash
# Interactive REPL
sysml

# Load file and start REPL
sysml model.sysml

# Evaluate and exit
sysml -e "5 + 3"

# Load file, evaluate, and exit
sysml -e "expr" file.sysml

# Multiple evaluations
sysml -e "x" -e "y" file.sysml

# Multiple files
sysml -e "result" file1.sysml file2.sysml
```

## Rendering a view

`-render <view>` renders one view of the model and exits. Every file named on the command line is
loaded as one model, as `-render-all` and `-render-document` load theirs, so the view may expose
elements a sibling file declares. The rendering kind comes from the view's `render` member, or is a
containment tree if the view does not state one. This build can produce a tree, an interconnection
diagram, a state machine, an action flow, a sequence diagram and a table. A geometry view is
recognized but not drawn. Pseudo-views let you render without declaring a view: `#tree` renders
every file `-render` loaded (or every document loaded in the REPL), while `#tree:<name>`,
`#interconnection:<name>`, `#state:<name>`, `#action:<name>`, `#sequence:<name>` and `#table:<name>`
render the named element directly. Only the kinds this build produces are offered; newly supported
kinds become pseudo-views automatically.

```bash
# The ASCII text form a person reads, written to fit the terminal
sysml model.sysml -render Views::vehicleView

# The machine-readable form of the kind: piped, redirected or written to a file
sysml model.sysml -render Views::vehicleView | tee view.mmd
sysml model.sysml -render Views::partsTable > parts.md
sysml model.sysml -render Views::vehicleView -o view.mmd

# Either form, whatever the destination
sysml model.sysml -render Views::partsTable -render-form markdown
sysml model.sysml -render Views::vehicleView -render-form text

# Graphviz DOT for a graph-shaped kind, to lay out with dot(1) or any Graphviz-reading tool
sysml model.sysml -render Views::vehicleView -render-form dot -o view.dot
sysml model.sysml -render Views::vehicleView -render-form dot -render-palette okabe-ito -o view.dot

# PlantUML in the Pilot visualizer's B&W style, for a PlantUML toolchain; no jar is run
sysml model.sysml -render Views::vehicleView -render-form plantuml -o view.puml
sysml model.sysml -render Views::handshake -render-form plantuml -render-palette tol-bright -o handshake.puml

# A view over several files, loaded as one model
sysml types.sysml model.sysml -render Views::vehicleView
sysml model/*.sysml -render Views::partsTable -render-form markdown -o parts.md

# Render a named element, or the whole model directly, without declaring a view
sysml model.sysml -render '#state:Vehicle::controller'
sysml model.sysml -render '#tree'
```

When `-render-form` is not given, the form depends on the destination: the text form when stdout is
a terminal, where a person reads it, and the machine-readable form of the kind when stdout is a file
or a pipe, where a tool does. The text form is ASCII, and its table is laid out to fit the terminal,
wrapping a cell wider than its column rather than truncating it. Into a file or a pipe, every column
is as wide as its widest cell, so a saved artifact does not depend on the window it was written from.

The rendering is the run's result, so it is the only thing on stdout. Load reports, analysis
diagnostics, a note that the rendering is empty, and any element the rendering cannot represent all
go to stderr, and `-o` writes the rendering only. A view that exposes nothing renders an empty
artifact and says so. A name that is not a view, a rendering kind this build does not produce, a
form the kind cannot be written in, and a model that did not analyse cleanly each stop the run with
status 2. Rendering decides nothing about the model, so it cannot be combined with a check flag or
with `-convert`.

`-render-all <dir>` writes every declared view of all loaded files, in document and declaration
order. Each qualified view name becomes a file name with `::` replaced by `.`. With no
`-render-form`, graph-shaped kinds use Mermaid (`.mmd`) and tables use Markdown (`.md`); a forced
text form uses `.txt` and unbounded width, a forced `dot` form uses `.dot`, and a forced `plantuml`
form uses `.puml`, PlantUML's conventional extension.

```bash
sysml types.sysml model.sysml -render-all rendered
sysml model.sysml -render-all rendered-text -render-form text
```

The directory is created if needed. Written paths, load reports, and notices (prefixed by the view
they concern) go to stderr; stdout stays empty. An unsupported rendering kind, or a forced form the
kind cannot be written in, is reported and skipped without failing the run. A model with no declared
views, or an analysis error, stops the run with status 2. `-render-all` cannot be combined with
`-render`, `-o`, `-convert`, or a check flag.

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form for the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has dedicated state diagram and sequence diagram grammars. A table is written as a Markdown table,
since Mermaid has no grammar for tables, so `-render-form mermaid` on a table produces Markdown
rather than a diagram of rows.

The forms a kind can be written in:

| Form | Kinds | What it is |
| --- | --- | --- |
| `text` | every kind | ASCII a person reads; the default at a terminal |
| `mermaid` | `tree`, `interconnection`, `state`, `action`, `sequence` | The machine-readable form of the graph-shaped kinds; a table falls back to Markdown |
| `markdown` | `table` | A pipe table, the machine-readable form of a table |
| `dot` | `tree`, `interconnection`, `state`, `action` | Graphviz DOT, an alternative to Mermaid for Graphviz toolchains and layouts of large graphs |
| `plantuml` | `tree`, `interconnection`, `state`, `action`, `sequence` | PlantUML in the Pilot visualizer's B&W style, for PlantUML toolchains; the one alternative form with a sequence grammar |

A node's label follows the graphical notation's header: the element's name leads, with ` : Type`
after it for a typed usage, the kind follows on its own line in guillemets, and any note (`initial`,
`already shown`, `own flow`) comes after that. An anonymous element leads with its kind and has no
keyword line. The text form keeps the notation's keyword-leading declaration order instead. One node
in each form:

| Form | `part pump : Pump` |
| --- | --- |
| `text` | `part pump : Pump` (a note in parentheses after it: `part sensor : Pump (already shown)`) |
| `mermaid` | `n1["pump : Pump<br>«part»"]` — a flowchart node, a `state "…" as n1` and a `participant n1 as …` all break at `<br>` |
| `dot` | `"n1" [label=<<b>pump : Pump</b><br/><font point-size="10">«part»</font>>];` — an HTML-like label, the name in bold and the keyword line at 10pt |
| `plantuml` | `rectangle "**pump : Pump**\n<size:10>//«part»//</size>" as n1 <<part>> <<usage>>` — a creole label, the name in bold and the keyword line italic at 10pt; the stereotypes drive the style and are hidden |

Every `subgraph` of a Mermaid flowchart opens on a `direction` statement restating the
flowchart's, because Mermaid lays out a subgraph that states none without regard to the
flowchart's; a tree draws containment as edges, not subgraphs, so it carries none.

A Mermaid flowchart reserves the height of one line for a `subgraph` title, so a flowchart
whose cluster title spans more — an interconnection or action rendering with a container —
opens on a YAML frontmatter block that claims the rest as the title's bottom margin, 24px per
extra line:

```
---
config:
  flowchart:
    subGraphTitleMargin:
      bottom: 24
---
%% Plant::loopView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["Plant::Loop<br>«part def»"]
    direction LR
  …
```

The block travels with the text into every consumer (`-render`, `-render-all`, `%render`,
`opensysml/render`, the Mermaid fences of a document in Markdown, HTML and PDF), and Mermaid
10.5 and later reads it. A flowchart with no such cluster, a `tree` rendering (its containment
is edges), a `state` and a `sequence` diagram have no frontmatter.

`dot` writes a `digraph` with one `// view:`, `// kind:` and `// layout:` header comment line and
one `// not represented:` line per notice, the same header Mermaid writes as `%%` comments.
Node and cluster labels are HTML-like strings (`label=<…>`) with `&`, `<`, `>` and `"` in a name
written as entities; edge labels, identifiers and geometry stay double-quoted strings.
Containment becomes a `subgraph "cluster_…"`, the flow direction becomes `rankdir`, and edges keep
Mermaid's semantics: a connection is undirected (`arrowhead=none`), a flow is dashed, a transition
or succession is a solid arrow drawn at the Pilot visualizer's thickness (`penwidth=3` for a
connection). The drawing is in the Standard B&W style of the OMG SysML v2 Pilot Implementation's
visualizer, after Hisashi Miyashita's `sysmlbw` PlantUML skin: Helvetica text, white fills, thin
`#181818` lines, square definitions and rounded usages, a bold name over an italic `«keyword»`
line, unfilled black-bordered clusters, and unnamed initial and final pseudo-states as the filled
UML dot ([the translation](../project/view-rendering-forms.md#style)). Producing DOT needs no
Graphviz installation; laying it out does, with the engine the `// layout:` header names
(`dot -Tsvg view.dot`, or `neato -Tsvg view.dot` when the view states positions — below). A
`sequence` view has no DOT counterpart and, like a `table`, is refused with status 2 when `dot`
is forced.

`plantuml` writes an `@startuml` … `@enduml` file for a PlantUML toolchain — the OMG Pilot's own
visualizer draws with PlantUML — with the same header as `'` comments (`' <view> — <kind> rendering`,
one `' not represented:` line per notice), the Pilot's B&W style inline as a `<style>` block plus
`skinparam wrapWidth 300`, and one grammar per kind: a tree is a class diagram (`hide circle`,
`hide empty members`, containment as `parent -- child` edges as the other forms draw it), an
interconnection nested `rectangle` blocks with the Pilot's heavy `-[thickness=3]-` connectors and
dashed `-[dashed]->` flows, a state rendering the `state` grammar with composite states, `[*] -->`
starts and PlantUML's pseudostate stereotypes, an action rendering the state grammar too (PlantUML's
activity syntax is procedural and cannot hold an arbitrary graph of successions and flows), and a
sequence `participant`s and `->` messages one for one with the Mermaid form. Each node's keyword is
a stereotype (`<<part def>>`, `<<state>>`) that the style selects on, hidden so the label's `«keyword»`
line is the only one printed. `TB`/`LR` become `top to bottom direction`/`left to right direction`;
PlantUML has no reversed direction, so `BT`/`RL` take the nearest forward one under a
`' not represented:` notice. PlantUML pins no position either, so DiagramLayout geometry is kept as
`' canvas:`, `' layout:` and `' route:` comments and noticed — `-render-form dot` is the form that
honours it ([the PlantUML section](../project/view-rendering-forms.md#plantuml)). Producing PlantUML
needs no Java and no PlantUML jar; drawing the file does (`java -jar plantuml.jar -tsvg view.puml`).

`-render-palette <name>` fills the DOT and PlantUML nodes with a colourblind-safe palette by **keyword
family** — a `part def` and a `part` share a hue, a `port` takes the next, and so on through
item, port, attribute, action, state, requirement, constraint, connection, interface, use case,
case, allocation, analysis, verification, enum, occurrence and flow. A definition is filled with
the family colour, a usage with a lighter tint of it, both bordered in the colour; every fill is
lightened until black text on it reads at the WCAG AA ratio of 4.5:1, and pseudo-states, control
nodes and cluster borders stay black and white. The palettes are `okabe-ito` (Okabe & Ito),
`tol-bright`, `tol-muted` and `tol-light` (Paul Tol), `brewer-set2` and `brewer-dark2`
(ColorBrewer), and the sequential `viridis` and `cividis` (matplotlib), which are sampled evenly
across the families the view draws, darkest first
([the palettes and their sources](../project/view-rendering-forms.md#palettes)). The palette
applies to the `dot` and `plantuml` forms alone, which fill each node with the same hex
(`#hex;line:hex` on a PlantUML element; a sequence participant takes the fill by keyword family):
`-render-form mermaid` writes a `%% not represented:` comment naming it, and the text and Markdown
forms ignore it. A name that is no palette is refused with
status 2 and the names there are; `-render-palette` without `-render` or `-render-all` is refused
likewise.

A rendering is laid out by whatever draws it, unless the model says where things go. The
`DiagramLayout` library (bundled, imported like any other) states that in notation: a
`metadata Layout about <element> { x = …; y = …; width = …; height = …; collapsed = true; }` in a
view's body positions the element in that view, an `@Layout { … }` inside an element's own body is
the position every view that does not place it falls back to, a `Route about <connection> {
points = (x0, y0, x1, y1, …); }` gives an edge its waypoints, and an `@Canvas { unit = "px"; width
= …; height = …; }` in the view body sizes its drawing surface. Coordinates are pixels from the
top-left corner, y downward. Mermaid cannot place a node, so the machine-readable form keeps the
geometry as comments after the header (`%% canvas: unit=px w=1200 h=800`, `%% layout: n1 x=120
y=80 w=200 h=90`, `%% route: n1->n2 320,125 400,125`) and the text form appends `at (120, 80)`,
`size 200×90` and `via (320, 125) (400, 125)` to the nodes and edges concerned. `-render-form dot`
honours it: a positioned node is pinned at the centre of its box (`pos="220,675!", pin=true`, in
points with y measured up from the canvas's bottom edge — negated when no canvas height is
stated — one pixel to one point under `inputscale=72`), a stated size is `width`/`height` in inches
with `fixedsize=true` (an unstated one is fitted to the label, so the box's corner stays put), a
positioned cluster states its `bb`, a route is the edge's `pos` spline (a route of one waypoint
draws no line and is noticed), the canvas is echoed as `// canvas:` and held by an invisible
point pinned at each corner so the drawing's bounding box is the canvas, and the
`// layout:` header names the command that honours it — `neato -n2` when every node is placed
and any edge routed, `neato -n` when every node is placed and none routed, `neato` when only
some nodes are — so `neato -n2 -Tsvg view.dot` draws the view where the model put it. `neato`
redraws every edge, so under it a written route is noticed as redrawn.
A model with no layout annotations renders exactly as before. `-validate` reports a `Layout` or
`Route` on an element the rendering does not draw as a node or an edge, a `Route` with an odd
number of values, a `Canvas` outside a view, and two positions for one element in one view (the
first applies). See
[Diagram layout annotations](../project/diagram-layout-annotations.md).

```bash
sysml model.sysml -render Views::vehicleView -render-form text
# part engine : Engine at (120, 80) size 200×90
```

`-render-documents <dir>` renders every document definition the loaded model declares into the
directory, one Markdown file per document, in fully-qualified-name order. Each file name is the
document's fully qualified name with `::` replaced by `-`, any byte outside ASCII letters,
digits and `_` escaped as `.XX` (uppercase hex), plus `.md`. The names are deterministic, so
cross-document references (see [the authoring chapter](../manual/authoring.md)) resolve as relative
links between the written files, and repeated runs write identical bytes.

```bash
sysml model.sysml -render-documents rendered
```

The directory is created if needed; written paths go to stderr and stdout stays empty. A
model that declares no documents, declares two documents with the same name, or does not analyse
cleanly stops the run with status 2. `-render-documents` cannot be combined with
`-render-document`, `-render`, `-render-all`, `-o`, `-convert`, a query flag, or a check flag
other than `-instantiate` (see [Rendering a document over objects](#rendering-a-document-over-objects)).
Rendering a single document with `-render-document` still succeeds when it has cross-document
references: the links point at the targets' expected file names and dangle until those documents
are rendered into the same directory.

`-render-document` takes as many model files as the document needs, loaded as one model, so a
document can query elements declared in sibling files:

```bash
sysml model/*.sysml -render-document Reports::MassReport -o report.md
```

## Rendering a document over objects

A document's queries read the model — the elements and what they declare. With `-instantiate`,
the one check flag a render run takes, they read the **objects** the run holds as well: each
`-instantiate <name>` creates its object first, as it does before a check, and the document is
then rendered over a session holding them. A query parameter the document binds to a usage's name
(`in root = car;`) binds the object the run holds under that name while it holds one, and the
element otherwise, so one document renders the declared model in one run and the objects in the
next; `Objects(type = "<type>")` enumerates every object held that is of the type. Over an
object, `OwnedElements`, `Descendants` and `Ancestors` walk the objects it holds and is held by,
`WhereType` tests its types, and `WhereFeature`, `Project` and `OrderBy` read the values it holds
now. See [Objects the session holds](../manual/query-cookbook.md#objects-the-session-holds).

```bash
sysml model.sysml -instantiate Garage::car -render-document Reports::CarReport -o report.md
```

The instantiation report goes to stderr, the document to stdout or `-o`, in every `-doc-form`.
An object renders by its path from the object it was bound through: a row's `name` is
`wheels[2]` for the second wheel of a collection, its `qualifiedName` the whole path
(`car.wheels[2]`), and an object-valued cell (a part's `engine`) is the path of the object held.
In HTML each row or list item over an object carries `data-object="#<id>"`, the id the
instantiation report printed, beside the `data-element` and `data-element-kind` of the usage the
object stands for, and an object-valued value is a `span.sysml-object`. An object that could not
be materialized stops the run with status 2 and the materialization errors; a render run still
takes no other check flag (`-validate`, `-constraint`, `-satisfy`, …): the verdicts about the
objects are a run of their own, or a table of the document itself.

A document lists what holds and what does not through `Verdicts(source = <rows>)`, which runs
the check `-validate=<object>` runs over the object behind each row — the object the run holds
when the binding is one, the element's declared object otherwise — and answers one row per
assertion about it and the objects it holds. Each row is a verdict: it stands for the
constraint, requirement, `satisfy` or verification case checked (so `name` and `WhereType` read
the assertion) and carries `path` (the object checked, `car.wheels[2]`), `kind`, `verdict`
(`holds`, `violated`, `undecided`), `condition`, `reason` and `verification`, which `Project`,
`WhereFeature` and `OrderBy` read. A verdict cell renders as `<assertion> on <path>: <verdict>`
in Markdown and PDF; in HTML it is a `span.sysml-verdict` carrying `data-verdict`, `data-path`
and, over a held object, `data-object`. See
[Which constraints and requirements hold](../manual/query-cookbook.md#which-constraints-and-requirements-hold).

```bash
sysml model.sysml -instantiate Garage::car -run-query "Reports::Violated root=car"
sysml model.sysml -instantiate Garage::car -render-document Reports::CarChecks -o checks.md
```

## Rendering a document as HTML

`-render-document <name> -doc-form html` writes the document as HTML rendered from the compiled
document tree itself, not by converting the Markdown: the model facts Markdown cannot carry survive
into the markup, so a stylesheet, a static-site generator, an accessibility tool or a downstream
processor can address them.

```bash
sysml model.sysml -render-document Reports::MassReport -doc-form html -o report.html
sysml model.sysml -render-documents site -doc-form html
sysml model.sysml -render-document Reports::MassReport -doc-form html \
    -doc-title-page -doc-toc -doc-number-sections -html-css theme.css -o report.html
```

The structure is ordinary semantic HTML — `<article>`, nested `<section>` whose heading levels
follow the nesting, `<p>`, `<table>` with `<caption>`, `<thead>` and `<th scope="col">`,
`<ul>`/`<ol>`, `<dl>` with `<dt>`/`<dd>`, `<figure>` with `<figcaption>`, `<nav>` for the
contents, and `<em>`, `<strong>`, `<code>`, `<a>` inline. Styling hooks are a small `sysml-` class
vocabulary (`sysml-document`, `sysml-section`, `sysml-table`, `sysml-row`, `sysml-cell`,
`sysml-value`, `sysml-list`, `sysml-item`, `sysml-definitions`, `sysml-entry`, `sysml-term`,
`sysml-description`, `sysml-diagram`, `sysml-caption`, `sysml-link`, `sysml-ref` and their kin),
and the model facts ride alongside on `data-` attributes: the content kind and name, the query
behind a table, list or definitions block, the group-by column, each row's, item's or entry's
selected element and its element kind
(`partUsage`, `requirementDef`, …), each cell's projected column and value kind, and a diagram's
view, kind and flow direction. Identifiers are anchors only, matching the Markdown anchors, so a
`Ref` resolves within a page and across a rendered set.

Diagram blocks embed their Mermaid source in `<pre class="mermaid">`, which a page that loads
Mermaid renders as a diagram and any other page shows as source. By default the output loads
nothing over the network, runs no JavaScript of its own, and is byte-identical between runs.
`-html-mermaid cdn` adds one `<script>` before `</body>` that loads a pinned Mermaid release from
jsDelivr so a browser with network access draws the diagrams; `-html-mermaid <url>` loads the
script from a URL of your own instead, such as a copy served beside the pages. The page still
carries only the source, so it degrades to source wherever the script cannot load. The option
does not combine with `-html-fragment`: a fragment has no page shell to hold the script, so the
embedding page loads Mermaid itself.

Formulas follow the same rule. A math span is a `<span class="sysml-math">` and a `Formula` block a
`<figure class="sysml-formula">`, each holding its LaTeX between MathJax's `\(…\)` or `\[…\]`
delimiters; `-html-math cdn` adds one `<script>` loading a pinned MathJax release from jsDelivr,
configured to typeset `.sysml-math` elements alone, and `-html-math <url>` loads the script from a
URL of your own. Without the option the page shows the LaTeX source, and the option does not
combine with `-html-fragment`.

### Styling the HTML

The default stylesheet is inlined in a standalone page and declared in a cascade layer:

```css
@layer opensysml;
@layer opensysml { /* the defaults */ }
```

Your own CSS is unlayered, so it wins on cascade origin rather than specificity — overriding a
default needs neither `!important` nor a matching selector. Every default value comes from a
`--sysml-*` custom property on `.sysml-document`, so retheming can be a handful of properties, and
the renderer emits no `style` attributes to compete with. `-html-theme modern|print|report` layers
a bundled theme over the default sheet, in the same layer, so your CSS still wins over both.
`-html-default-css` writes that sheet to copy from (the theme's whole sheet with `-html-theme`),
`-html-css` adds sheets after it (a file is inlined in a single page and written beside a set's pages, a URL is linked), and
`-html-no-default-css` drops it entirely. A `-render-documents` set writes one shared
`sysml-document.css` that every page links, so the styling is edited in one place, and
`-html-fragment` writes the `<article>` alone, with no page shell and no stylesheet, for embedding
in a page that brings its own.

## Rendering a document as PDF

`-render-document <name> -doc-form pdf -o report.pdf` converts the rendered Markdown to PDF. The
conversion never runs inside the `sysml` binary: it drives an external converter as a subprocess,
chosen with `-pdf-engine`, so the binary links no PDF renderer and Markdown output needs none of
these tools.

```bash
sysml model.sysml -render-document Reports::MassReport -doc-form pdf -o report.pdf
sysml model.sysml -render-document Reports::MassReport -doc-form pdf \
    -pdf-engine pandoc -doc-title-page -doc-toc -doc-number-sections -o report.pdf
```

The engines. Each is found on `PATH` by its default name unless an environment variable points
at a specific executable:

| Engine | Tools it drives | Override |
|--------|-----------------|----------|
| `weasyprint` (default) | `weasyprint`, an HTML-to-PDF paged-media engine | `OPENSYSML_WEASYPRINT` |
| `pandoc` | `pandoc` reading the Markdown itself, with WeasyPrint as its PDF engine | `OPENSYSML_PANDOC` (and `OPENSYSML_WEASYPRINT`) |
| `prince` | `prince`, a commercial HTML-to-PDF engine | `OPENSYSML_PRINCE` |

The title page, table of contents and section numbering belong to this output step alone. They
are flags of the run, never attributes of the document model, so the same document renders to
Markdown unchanged.

Diagram blocks are pre-rendered to SVG with [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)
(`mmdc`; override with `OPENSYSML_MMDC`. `OPENSYSML_MMDC_PUPPETEER` names a puppeteer configuration
file for a browser that needs launch flags, such as `--no-sandbox` in a container). A document
without Mermaid diagrams needs no diagram tool. Under `-diagram-form dot` or `-diagram-form
plantuml` no diagram is drawn: the PDF keeps each one's DOT or PlantUML source under a notice
saying so, and neither `mmdc` nor a Graphviz or PlantUML tool is looked for.

Formulas — `$…$` spans and `$$…$$` blocks, wherever the Markdown carries them — are typeset with
[KaTeX](https://katex.org)'s command line (`katex`; override with `OPENSYSML_KATEX`, and name its
stylesheet with `OPENSYSML_KATEX_CSS` when it is not installed beside the command), whose HTML and
fonts every engine embeds, so the PDF shows typeset mathematics rather than LaTeX. A document
without formulas needs no KaTeX; LaTeX KaTeX rejects fails the run with its parse error.

Inline runs keep their meaning in PDF: emphasis, strong and code styling, links, and `Ref`
cross-references as clickable internal links to their targets' invisible anchors, in every engine
(`weasyprint` and `prince` through the prepared HTML, `pandoc` through the Markdown itself). A
grouped table's group key renders in bold above each subtable.

A PDF is a binary artifact, so `-doc-form pdf` requires `-o`. A missing tool stops the run with
status 2 and a message naming the tool, its override variable and the other engines; a converter
that fails reports its own output. `scripts/download-doc-pdf-toolchain.sh` installs pinned copies
of WeasyPrint, pandoc, mermaid-cli and KaTeX under `build/doc-pdf/` and prints the variables to export
(Prince is commercial and installed separately). Every tool runs with `SOURCE_DATE_EPOCH=0`, so
an engine that embeds a creation date embeds the same one every run, and the artifact is
reproducible for a given toolchain.

## Sweeping a parameter

`-sweep` runs the analysis case or calc named by `-analysis`/`-calc` once per value of a range,
and reports the runs as a table. Each run is the ordinary run that flag makes on its own, with the
swept parameter bound to that run's value and every other argument as written on the command line,
so nothing about how a case executes changes:

```bash
$ sysml -calc "Dyn::Speed(mass = 1000.0 [SI::kg])" \
    -sweep "power=1000.0 [SI::W]..3000.0 [SI::W]:1000.0 [SI::W]" model.sysml
sweep Dyn::Speed — 3 run(s)
power             | result           | time
------------------+------------------+--------
1000.0 [SI::W]    | 1.0 [SI::'m/s']  | 0.412ms
2000.0 [SI::W]    | 2.0 [SI::'m/s']  | 0.221ms
3000.0 [SI::W]    | 3.0 [SI::'m/s']  | 0.219ms
```

The columns are the swept parameters, the run's `return` or `out` values, the verdict of the
case's `objective` where it has one, the wall time of that run, and an `error` column present only
when a run failed. A failed run keeps its place in the table and numbers its typed error, which is
printed in full under the table — the table continues, and the check as a whole fails:

```bash
$ sysml -instantiate Sub::car -analysis "Dyn::DynamicsAnalysis(deltaT = 1.0 [SI::s]) Sub::car" \
    -sweep "initialSpeed=0.0 [SI::'m/s']..1.0 [SI::'m/s']:1.0 [SI::'m/s']" model.sysml subject.sysml
sweep Dyn::DynamicsAnalysis — 2 run(s)
initialSpeed    | accelerationProfile | time    | error
----------------+---------------------+---------+------
0.0 [SI::'m/s'] |                     | 0.617ms | 1
1.0 [SI::'m/s'] | [1.0 …, 0.5 …]      | 0.469ms |
error 1: analysis Dyn::DynamicsAnalysis: … calc Dyn::Acceleration: division by zero
```

**Rows.** Each row is a run in a context of its own: the `-instantiate`d subject is instantiated
afresh for it and the arguments, evaluated once, are carried in — one naming an `-instantiate`d
object binds the row's own — so a case that writes a feature of its subject writes its own row's
object and no row sees another's. Rows run [`-jobs`](#running-in-parallel) at a time and the table
is in range order whatever order they finish in; the `time` column is each row's own wall time. An
`-instantiate`d subject whose type exhibits or performs a behavior sweeps as any other: its
execution, fresh from `-instantiate`, is as its start left it, and each row's subject starts so.

**Ranges.** `<from>`, `<to>` and `<step>` carry the literal syntax an argument carries, units
included; a quantity range's endpoints and step must be compatible, and the values are converted
to the unit `<from>` is written in. `<to>` is included when the step lands on it. A range between
whole numbers with no `:<step>` steps by one, up or down as the endpoints direct; a range with a
fractional endpoint and no step is refused, because no step is the obviously intended one. A range
read as reals takes an Integer endpoint or step only where a Real holds it without rounding, and
steps only where the reals tell its rows apart, so a range no two rows of which would differ is
refused rather than run. A step
of zero, a step whose sign never reaches `<to>`, an endpoint that is no number or is not finite, a
parameter the case or calc does not declare, a case's subject — which an `-instantiate`d object
binds, not a range — a parameter the arguments already bind, by name or by holding the position it
is bound from, and a `-sweep`/`-samples` without an `-analysis` or `-calc` are each refused with
status 2 before any run is made.

**Types.** The values a range produces are typed by the parameter it sweeps, not by how its
endpoints are spelled. A `Real` or `Rational` parameter swept over `1..4:1` is bound to the reals
`1.0`, `2.0`, `3.0`, `4.0`, and the table shows them so; an `Integer`, `Natural` or `Positive`
parameter swept over `1.0..3.0:1.0` is bound to the Integers `1`, `2`, `3`, and an endpoint or
step of it that is no Integer (`1.0..3.0:0.5`, `1.5..3`) — or below what a `Natural` or
`Positive` holds — is refused naming the parameter and its type, before any run, rather than
failing row by row. An `attribute def` specializing a scalar takes that scalar's values. A
quantity-typed parameter (`ISQ::LengthValue`) is typed through its `num`: the library declares
`Number`, which Integers and reals both are, so its magnitudes are read as written, while a
quantity redefining `num : Integer` takes Integers, one redefining it `Natural` or `Positive`
refuses a magnitude below what that holds, and one whose `num` holds no number refuses the range;
the range's unit is the one `<from>` carries, not one the parameter names. A `Number`-typed
parameter, and one declaring no type, take the range as written — Integers between Integer
literals, reals otherwise — and the table notes an untyped one under its rows. A range over a
`Boolean`, `String`, enumeration or non-scalar parameter is refused naming that type.

**Order.** Rows come out in the order the ranges are written: the first `-sweep` flag varies
slowest, the last fastest, each range from `<from>` towards `<to>`. Two runs of one plan produce
the same rows in the same order.

**Samples.** `-samples <n> -seed <s>` draws `n` values for each range instead of running every
value of it, uniformly over `[<from>, <to>]` for a parameter taking Integers and `[<from>, <to>)`
for one taking reals, in draw order — the parameter's type decides, so an `Integer` parameter
sampled over `1.0..4.0` draws the Integers 1 to 4 inclusive and a `Real` parameter sampled over
`1..4` draws reals in `[1, 4)`. A sampled range needs no step, and stating one is refused.
Sampling is uniform because the bundled standard library states no probability distribution: a
range written as a named distribution (`n=normal(1.0, 0.2)`) is refused naming what is missing,
rather than approximated. The generator is `math/rand/v2`'s `PCG` seeded from `<s>`, so the same
seed draws the same table on every platform; `-seed` is required with `-samples` (there is no
wall-clock default) and is echoed in the table header:

```bash
$ sysml -calc "Dyn::Speed(mass = 1000.0 [SI::kg])" \
    -sweep "power=0.0 [SI::W]..3000.0 [SI::W]" -samples 3 -seed 42 model.sysml
samples Dyn::Speed — 3 run(s), seed 42
```

**Budget.** A plan is bounded by `OPENSYSML_MAX_SWEEP_RUNS` (default 1000, see
[Environment variables](environment.md)) and one asking for more runs than that is refused with
the count it asks for, rather than started.

With `-json` the runs are the `rows` array of the check they belong to, inside the same document
`-json` prints without them (`null` where the check made no sweep):

```json
{"checks": [{"kind": "calc", "subject": "Dyn::Speed", "status": "passes",
  "rows": [{"inputs": [{"name": "power", "value": "1000.0 [SI::W]"}],
            "outputs": [{"name": "result", "value": "1.0 [SI::'m/s']"}],
            "verdicts": [], "milliseconds": 0.412, "error": ""}]}]}
```

A row of a [trade study](#trade-studies) carries in addition the `evaluations` the run made, as
the check itself does.

The REPL runs the same tables through [`%sweep` and `%samples`](repl-commands.md), and a service
client through the [`RunSweep` RPC](api.md).

## Running an action many times

A model that states its own odds — a decision whose successions carry
`@Probability { p = … }`, a duration or a value drawn by `uniform`, `uniformInteger`,
`triangular` or `normal` from the `RandomFunctions` library (see
[When a model states its own odds](../guide/06-behavior.md#when-a-model-states-its-own-odds)) —
is a question about a distribution. `-runs <n>` with `-seed <s>` runs the one `-action` to
completion `n` times, each run on a fresh context whose model seed is derived from `<s>` and the
run's number, so run 3 of seed 7 is the same run on every platform and can be made alone with
that run's seed. The table has one row per run, numbered, with each `-observe` feature of the
action and `clock`, the simulation time the run completed at; without `-observe` every feature
the action holds and the clock are tabled. Below the table each numeric observable is summarised
over the runs that completed — minimum, mean, maximum, the nearest-rank p50 and p90, and a
histogram — and a non-numeric one is counted by value:

```bash
$ sysml -action MC::route -runs 8 -seed 7 -observe taken -observe clock mc.sysml
✓ package MC
runs MC::route — 8 run(s), seed 7
run | taken | clock                  | time
----+-------+------------------------+--------
1   | 1     | 45.771104597451966 [s] | 5.056ms
2   | 1     | 18.029229676573745 [s] | 6.089ms
…
taken: 8 run(s), min 1, mean 1.25, max 2, p50 1, p90 2
  1 ###############      6
  2 #####                2
clock: 8 run(s), min 18.029229676573745 [s], mean 43.490366063934395 [s], max 75.88725335563454 [s], p50 35.38279454977086 [s], p90 75.88725335563454 [s]
  18.03..25.26 [s] ###                  1
  …
  standing: table (observed: 8 rows)
```

The runs are the rows of a [sweep](#sweeping-a-parameter) plan with no range: they run `-jobs`
at a time, a run that fails is a numbered row with its error under the table, the plan is
bounded by `OPENSYSML_MAX_SWEEP_RUNS`, and with `-json` they are the check's `rows`, each run's
number its one input, `run`. `-schedule` is the second, independent knob: it resolves the
concurrency choices — which carry no probability — in every run alike, and `replay:<file>`,
which is one run, is refused with `-runs`. `-runs` needs exactly one `-action` and `-seed`, and
is refused with `-sweep`, `-samples`, `-advance`, `-state`, `-check-property`, `-check-diverge`
or `-check-input`.

`-seed` alone seeds the one run an invocation makes: `sysml -action MC::route -seed 7` draws the
model's values from `7` whatever `-schedule` shuffles the tokens with, so `-schedule declared
-seed 7` and `-schedule seed:3 -seed 7` make the same draws in two token orders. Without any
seed a run that must draw a value is refused —
`modeled randomness needs a seed: uniform(0.0, 10.0) draws a random value; seed the run, as
-seed <n> or %seed <n>, or schedule it under seed:<n>` — while a weighted decision takes its most
probable branch, so an unseeded run stays deterministic; `-schedule seed:<n>` with no `-seed`
draws the model's values from `n` too, on a stream of its own. Every draw is recorded in the
witness the checker writes, as `draw <call> = <value>` lines, and `-schedule replay:<file>`
consumes them instead of drawing again.

## Exploring every linearization

Where a behavior has [choice points](../guide/06-behavior.md) — several steppable tokens in one
step, several holding guards at a decision, several enabled transitions out of one state for one
event, several regions of one parallel state reacting to one event, two tokens writing one
feature in one step, two executors due at one instant of the clock — one run shows one
linearization.
`-schedule explore` runs them all: the first run records the alternative taken at each choice
point, and every later run replays the recorded prefix and takes the next untried alternative at
the frontier, depth-first, until no alternative is left untried or a budget is hit. Every run
starts from a fresh executor on the same loaded model: no object, message, clock, calc memo or
note of one run is seen by the next. Under `explore` an action step is one token advancing one
node, where the fixed policies move every steppable token once per step, so the tokens able to act
are picked among afresh after each move and every interleaving of the nodes the library leaves
unordered is a distinct linearization; a body's statements still run without interruption. The
policy applies to `-action`, `-state`, `-analysis` and `-calc` alike; a body with no choice point
explores in exactly one run. With `-advance`, every
`-action` and `-state` behavior named is started on one clock in each run and the clock advanced
once, as it is under any policy, so the order of executors due at one instant is explored like any
other choice point: several behaviors come to one *joint* outcome, each behavior's observables under
its name and what the object performed on holds under `this.` (`Demo::Beacon::blinking finalState =
"shining"; Demo::watcher.sawLit = true; this.lit = true`), and the witness names which executor
ran first (`t=5.0: state machine blinking of object #1 first of state machine blinking of object
#1, action watcher` — the `-instantiate`d beacon's machine, created first in each run, is listed
first); an action still waiting on the clock when the time
is up is the run's error, as it is undecided under one policy.

Runs that agree on what the harness compares — an action's outputs; a state machine's final state,
the states it visited and its context's values; an analysis case's outputs and verdicts — are one
*outcome*. The report is one row per distinct outcome, sorted by the outcome's rendering, with the
number of linearizations that reached it and the choice sequence of one witness run, then a status
line:

```bash
$ sysml -schedule explore -action test::race three-writers.sysml
✓ package test
✓ explored test::race: 3 outcomes
outcome                                      | linearizations | witness
---------------------------------------------+----------------+------------------------------------------------------------------
aRan = true; bRan = true; cRan = true; x = 1 | 2              | step 3: 3@b first of 2@a, 3@b, 4@c; step 4: 4@c first of 2@a, 4@c
aRan = true; bRan = true; cRan = true; x = 2 | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 4@c first of 3@b, 4@c
aRan = true; bRan = true; cRan = true; x = 3 | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 3@b first of 3@b, 4@c
complete (6 runs)
```

A run that fails — a guard that divides by zero on one path, say — is an outcome of its own,
rendered as `error: <message>`, not the end of the exploration; a witness of `no choice points`
marks the one outcome of a behavior with none. The rendering is canonical: the same model tables
the same rows in the same order every time. An object a run holds is spelled by its type and
feature values — `lead = test::Rover#1{id = 2}`, a repeat of the same object `#1` — never by the
id the run gave it, so two runs binding the same object are one outcome even when their ids differ,
and two runs binding different objects under one id are two.

**Budget.** `explore` alone runs at most 1024 runs and resolves at most 64 choice points per run;
`explore:runs=N`, `explore:depth=D` and `explore:runs=N,depth=D` (in either order) set them. `N`
is a decimal integer of at least 1 and `D` of at least 0. Hitting either budget is never silent:
the status line becomes `incomplete: <budget> budget <limit> hit after N runs` (naming both, `runs`
then `depth`, when both were hit), the outcomes reached so far are still tabled, the check is
reported `?` rather than `✓`, and the exit status is `2` — the exploration could not answer whether
other outcomes exist. `explore:depth=0` therefore explores a behavior with a choice point in one
run and reports `incomplete: depth budget 0 hit after 1 runs`.

With `-trace`, the table and status come first and the trace of each outcome's witness run follows,
under `trace of outcome <n>'s witness (run <r>):`, so every `choice` line a witness took is
readable beside the row it produced; the other runs' traces are not printed, since a witness per
outcome is what distinguishes the outcomes and the full set would repeat every prefix once per
replay.

With `-json` the check carries the rows as `outcomes` — each with its `values`, `linearizations`,
`witness` (one choice per entry, in run order) and, for a failed run, its `error` — and how the
exploration ended as `exploration` (`complete`, `runs`, `budgetsHit`):

```json
{"checks": [{"subject": "Mission::race", "status": "unresolved",
  "outcomes": [{"values": [{"name": "x", "value": "2"}], "linearizations": 1,
                "witness": ["step 3: 2@left first of 2@left, 3@right"]}],
  "exploration": {"complete": false, "runs": 1, "budgetsHit": ["runs"]}}]}
```

The REPL's `%schedule` refuses `explore`, since its `%action` and `%state` debuggers step one run
([`%schedule`](repl-commands.md)); a service client explores through the same `schedule` field
and reads the outcomes off the response ([API](api.md), [wire contract](wire-contract.md)).

### Objects an exploration runs on

Every explored run creates its objects afresh, so the object a `-state`, `-action` or `-analysis`
names is not one the session holds but a *recipe* each run follows. Three spellings name one:

- **A declaration** — a `part`/`item` usage or definition, as `-state "Fleet::Rover::modes
  Fleet::rover"`: each run instantiates it and runs the machine on that object.
- **A path from a declaration** into a part it holds — `<declaration>.<usage>[.<usage>…]`, with
  `[i]` on a multi-valued usage: `-state "Comms::Craft::modes Comms::pair.craft"`,
  `-analysis "Dyn::Analysis Fleet::fleet.rovers[2]"`. The run instantiates the declaration the path
  starts from, its parts and connectors with it, then walks the rest of the path inside that
  object exactly as `%state` walks `pair.craft` in the session's. The declaration is created **once
  per run** however many behaviors name paths under it, so `-state "Comms::Ground::listen
  Comms::pair.ground" -state "Comms::Craft::modes Comms::pair.craft"` runs both machines on the
  parts of one `pair` and the messages the pair's connector carries between them are what the
  exploration tables — the point of exploring an assembly rather than a part on its own.
- **An `-instantiate`d declaration**, given to every run: `-instantiate Comms::pair` makes each run
  create its own `pair` before its behaviors start. A machine or action named alone (`-state
  Comms::Ground::listen`, `-action Tank::Tank::fill`) then attaches to the performance the run's one
  object exhibiting or performing it already runs, so the outcome is that object's; several objects
  running it are refused by name, as `%state` refuses the session's. A path under the declaration
  (`Comms::pair.ground`) walks into the same object rather than creating another. A declaration
  `-instantiate`d twice gives each run two objects of it, as the session holds two: the name and a
  path from it denote the later, the earlier is the run's by id alone (`#3.ground`), and a machine
  named alone that both run is ambiguous between them.

The path is planned once, under the session's lock, before any run starts: an unknown usage
(`Comms::pair.tower`), an index on a usage of one value (`Comms::pair.ground[2]`) or a step through
a value that is no object are refused by name with nothing run. What only a run can know — a part
its recipe left unbuilt, an index past what the run created — is that run's error, an outcome of its
own in the table. The id the report prints (`#2`) and a path rooted at it (`#2.ground`) name an
object of *this* session, which no run sees, and are refused saying so: name the declaration
instead. The prompt's `%instantiate` likewise creates the session's object alone, which no run
sees; only the CLI's `-instantiate` is given to the runs. The same spellings name the performer and
subject of a service request ([wire contract](wire-contract.md)), and `-engine check`, `smt` and
`all` plan their runs' objects by the same rules, so a witness the checker writes for a machine on
`Comms::pair.ground` replays on it.

### Running in parallel

`-jobs <n>` (default `OPENSYSML_JOBS`, else one per CPU) lets `n` runs of one exploration go at
once, each on a worker of its own — a resolver and semantic model per worker over the one loaded
model, so no run sees another's memo or object. The prefixes explore discovers form a work queue
ordered as the sequential exploration would take them, and the report is assembled in that order:
the outcome table, each outcome's witness (the least prefix reaching it), the run count and the
budget hit are the ones `-jobs 1` reports, byte for byte, whatever `n` is. A `runs` budget is a
cut in that order — runs past it are discarded and charged to nothing. At most `n` runs beyond
the cut are ever started, so an exploration performs at most `runs + n` executions. A run that
fails is an outcome of the table, as it is under `-jobs 1`; no run is cancelled for it, because
the table is the answer to a question about every linearization. A count below one, or one
that is no integer, is refused before anything runs. `-engine all` consults its covering engines
on the same count, `n` at most at once with the count shared out among them. A sweep runs its
rows `n` at a time, each in a context of its own — the subject and arguments instantiated there,
so no row sees another's writes — and prints them in range order whatever order they finish in:
the rows, their outputs, verdicts, evaluations and errors are those of `-jobs 1`, only each row's
`time` (its own wall time) and the report's `workers`/`warming` varying with `n`.

With `-json` the check's `plan` carries `workers`, how many workers the plan built, and
`warming`, the milliseconds spent building them; the human-readable report does not print them.

## Analysis engines

Every check is a question put to an analysis engine, and every verdict line is followed by its
**standing**: the claim, the strength of the evidence behind it, and what earned that strength.

```bash
$ sysml -constraint Rover::MassBudget -constraint Rover::Overweight model.sysml
✓ package Rover
✓ Constraint Rover::MassBudget passed
  standing: holds (observed: 1 run under reverse)
✗ Constraint Rover::Overweight failed
  Assertion evaluated to false: 250.0 <= 200.0
  standing: violated (witnessed: 1 run under reverse)
```

The strengths, weakest first: *not covered* (no claim is made, and the standing says why —
the engine's refusal, a solver's `unknown`, a witness that did not replay), *observed* (one
execution, or an exploration that stopped at its budget), *witnessed* (an existential claim
exhibited by an execution the interpreter replayed — a violation, a satisfying assignment),
*bounded* (every case within a stated budget) and *proved* (every case). A universal claim states
what it ranges over — `holds (proved over schedules: 6 linearizations, inputs as written)` —
and a budget the engine reached is named in the standing and lowers the strength:
`outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))`. A budget reached is
never a proof.

`-engines` tables the engines of the build, in name order, with the kind of each (`built-in`,
or the manifest entry kind that registered it), the protocol it is spoken by (`-` for one built
in, `object` for a tool, `stdio/1` for an external engine), the authority it carries (the
strongest strength it may claim for a universal answer), the question kinds it answers and its
status — `ready`, `ready (z3 at /usr/bin/z3)` for one whose process was found, or
`unavailable: <why>`:

```bash
$ sysml -engines
engine   kind      protocol  authority  answers          status
check    built-in  -         bounded    outcomes, holds  ready
explore  built-in  -         proved     outcomes         ready
run      built-in  -         observed   evaluate         ready
smt      built-in  -         proved     holds            ready (z3 at /usr/bin/z3)
solve    built-in  -         proved     satisfiable      ready (z3 at /usr/bin/z3)
sweep    built-in  -         observed   sweep            ready
```

Every tool the manifest directory `OPENSYSML_TOOLS` names adds a `tool:<name>` engine, listed
the same way with its executable's status (`tool:ModelCenter  tool  object  observed  compute
ready (ModelCenter 14.1 at /opt/modelcenter/bin/mc-batch)`); it answers the `compute` a
performance of an action annotated `ToolExecution` asks, and nothing else does, so a tool that
is unregistered or fails stops that performance rather than falling back to the action's body
([External tools](environment.md#external-tools)). Every engine the manifest directory
`OPENSYSML_ENGINES` names is listed under its own name, with the manifest's authority and
answers and the status its file can tell; a `policy`, `sampler` or `module` entry is listed as
`unavailable: … is not served in this build: …` naming the stage that serves it. After the
table, one line per manifest entry names its file and the command it runs, and `not admitted`
for an engine, since no engine is admitted by this build. Nothing is started:

```bash
$ OPENSYSML_ENGINES=/etc/opensysml/engines sysml -engines
engine       kind      protocol  authority    answers          status
check        built-in  -         bounded      outcomes, holds  ready
explore      built-in  -         proved       outcomes         ready
priority     policy    stdio/1   not covered                   unavailable: policy "priority" is not served in this build: scheduling policies are the strategies stage
run          built-in  -         observed     evaluate         ready
solve        built-in  -         proved       satisfiable      ready (z3 at /usr/bin/z3)
spin-bridge  engine    stdio/1   bounded      holds, outcomes  ready (spin-bridge 1.4.0 at /opt/spin-bridge/bin/spin-bridge)
sweep        built-in  -         observed   sweep            ready
priority 0.3: policy from /etc/opensysml/engines/priority.json, runs /opt/priority/bin/priority-policy
spin-bridge 1.4.0: engine from /etc/opensysml/engines/spin-bridge.json, runs /opt/spin-bridge/bin/spin-bridge, not admitted
```

`-engines -probe` also starts each external engine once, asks it to `describe` itself, checks
the answer against the manifest field by field and ends it; the status becomes `ready (…;
describe agrees)` or names the first disagreement (`engine "spin-bridge" describes its version
as "1.5.0"; the manifest says "1.4.0"`), a process that does not start or answer in
`OPENSYSML_TOOL_TIMEOUT` being reported as such. The manifest, the protocol, what an external
answer is worth and every way one fails are on [External engines](external-engines.md).

`-engine` selects. `auto`, the default, is the dispatch every check has always had: the engine
of highest authority that covers the question answers it, and one that refuses or answers *not
covered* is passed over for the next, each kept in the plan with its reason. `-engine <name>`
puts the question to that engine alone, and its refusal is the verdict — nothing answers in its
place, so `-engine explore -constraint C` reports the constraint as not evaluated with
`explore does not answer evaluate questions` and exits 2:

```bash
$ sysml -engine explore -constraint Rover::MassBudget model.sysml
✓ package Rover
? Constraint Rover::MassBudget could not be evaluated
  Error: explore does not answer evaluate questions
  standing: not covered (explore refused: explore does not answer evaluate questions)
```

`-engine explore -action <name>` explores every linearization exactly as `-schedule explore`
does, and a budget spelled on `-schedule explore:runs=N,depth=D` bounds it. `-engine
spin-bridge -action <name>` puts the action to that external engine alone, and its answer
stands at the strength the interpreter's replay of its witness earns, never at the one it
claimed ([External engines](external-engines.md#the-standing-of-an-answer)); under
`auto` an external engine is reached only once every built-in engine has refused or answered
*not covered*, whatever authority its manifest declares. `-engine all` puts the question to
every engine that covers it, one after another in name order, and composes what they answered: a witnessed violation stands over any universal claim, agreeing universal claims
stand at the strongest strength any of them earned (never promoted past it), differing
observed values become a witnessed sensitivity, and a universal claim an execution refutes is a
**disagreement** — the witness stands, the refuted result is demoted to *not covered* with the
disagreement as its reason, and the plan records both. The standing under `all` lists each
engine's part after the composed result:

```
  standing: holds (observed: 1 run under reverse); all: run holds (observed)
```

An engine `all` stopped before it answered — the plan's deadline was met — is kept in the plan
as cancelled with the bound it reached, and the composed result is what the engines that
finished earned. A name no engine is registered under is refused before anything runs:

```bash
$ sysml -engine bogus -constraint Rover::MassBudget model.sysml
invalid value "bogus" for flag -engine: analysis: no engine named "bogus"; the engines are check, explore, run, smt, solve, sweep, or auto, or all
```

With `-json` each check carries how it was answered beside the fields it always carried. `plan`
holds the selection (`engine`: `auto`, `all` or the name), the composed `standing`, one `steps[]`
entry per engine consulted — its `engine` and `status` (`answered`, `refused`, `failed`,
`cancelled`), the `detail` of a refusal or fault, the `bounds` a cancelled engine reached — and
the `disagreements[]` the composition under `all` resolved (`stands`, `demoted`, the demoted
`claim` and `strength`, `reason`). `results[]` holds one entry per engine that answered:
`engine`, `claim`, `strength`, `bounds` (every bound the engine took, each with `name`, `limit`
and whether it was `reached`), `witness` (the replayable execution behind a witnessed claim —
its `schedule`, its `choices` and, when the run drew modeled randomness, its `draws` — or
`null`), the `reason` of a result claiming nothing, and its
`standing`. The verdict's `lines` end with the standing line. `results` is `[]` when no engine
answered (every one refused), and a check decided before any engine was asked — a subject that
did not resolve — carries neither key.

```json
{"checks": [{"subject": "Rover::Overweight", "status": "fails",
  "lines": ["✗ Constraint Rover::Overweight failed",
            "  Assertion evaluated to false: 250.0 <= 200.0",
            "  standing: violated (witnessed: 1 run under reverse)"],
  "plan": {"engine": "auto", "standing": "violated (witnessed: 1 run under reverse)",
           "steps": [{"engine": "run", "status": "answered"}]},
  "results": [{"engine": "run", "claim": "violated", "strength": "witnessed",
               "bounds": [{"name": "steps", "limit": 10000000, "reached": false},
                          {"name": "elements", "limit": 1000000, "reached": false}],
               "witness": {"schedule": "reverse", "choices": []},
               "reason": "constraint Overweight: assertion evaluated to false: 250.0 <= 200.0",
               "standing": "violated (witnessed: 1 run under reverse)"}]}]}
```

The REPL selects with [`%engine`](repl-commands.md) and lists with `%engines` (`%engines
probe` probing as `-engines -probe` does); a service client sends the same selection in the
`engine` field of a request and reads `engine`, `strength` and `bounds` off the response
([wire contract](wire-contract.md)). The service lists external engines but runs none until
started with `-serve-external-engines` ([sysml-grpc](service-transports.md)).

## Checking every schedule of an action or a state machine

`-engine check` puts the behaviors of the invocation — each `-action` and each `-state` — to
the `check` engine, an explicit-state model checker over the interpreter: instead of running
the behavior once, or once per linearization as `-schedule explore` does, it searches the
schedules the library leaves open one move at a time — one token advancing one node, one
event dispatched, one `do` behavior stepped — taking a snapshot of the run before each choice
and restoring it to try the next, and reports the first state on any schedule where a property
is false, the run deadlocks or a body raises a typed error, or, with none, whether a feature
ends differently on different schedules. Two moves that touch disjoint features, messages and
control nodes reach the same state in either order, so the search explores one order of each
such pair (a static partial-order reduction over the reads, writes, sends, accepts and joins
each node's body names and over the guards, triggers and effects of the transitions an event
can select, computed once when the behavior is lowered), and a state it has visited — the same
tokens at the same nodes, the same configuration, the same values, the same messages and
clock, whatever ids the run handed out — is not searched again. The design is in
[bounded model checking](../internals/design/bounded-model-checking.md).

```bash
$ sysml -engine check -action Mission::race -check-witness witnesses race.sysml; echo $?
✓ package Mission
✗ Action Mission::race: divergent (11 states, 10 moves, depth 6)
  divergent: x ends as 1 or 2
    x = 1 (witness witnesses/Mission.race-x-1.witness)
    x = 2 (witness witnesses/Mission.race-x-2.witness)
  outcome: leftRan = true; rightRan = true; x = 1
  outcome: leftRan = true; rightRan = true; x = 2
  standing: sensitive (witnessed: 11 states, 10 moves searched, witness of 1 choice replayed)
1
```

The verdict line names the search: the distinct states visited, the moves made and the deepest
schedule. The verdicts:

| Verdict | Means | Status |
|---------|-------|--------|
| `no violation, exhaustive` | every schedule ended complete, no bound was hit and no property was false. The one verdict that is a proof — relative to the atomic step and the properties named — and the standing is *bounded over schedules*, never *proved*, because the checker's atomic step is coarser than the interpreter's | `0` |
| `no violation within bounds` | no violation on the schedules searched, but a bound cut some of them, named after `bounds hit:`; the standing is *bounded* with the bound marked `(reached)` | `2` |
| `violation` | a `-check-property` false at a reached state, a deadlock (`ErrActionDeadlock`, `ErrAcceptDeadlock` on that schedule), or a typed error a body raised — an unbound parameter, a dangling succession, a division by zero, an `accept` whose `via` port does not resolve — each with the schedule that reaches it; a budget the executor exhausts is a bound, not a violation | `1` |
| `divergent` | no violation, and a feature `-check-diverge` names (or, absent one, an attribute of a behavior or its performing object, a machine's `finalState`) ends with different values on different schedules; each value with one witness. The library admits the divergence; the model depends on a tool's choice | `1` |
| `incomplete: time` | `-check-timeout` ended the plan before the search did; the states and depth it reached are named and the result is *not covered* | `2` |

The final values every complete schedule reaches are listed as `outcome:` lines, as
`-schedule explore` tables them, and the two agree: where an exploration completes, the set of
outcomes the checker reaches is the set the exploration tabled, and `-engine all` composes the
two with `explore` as the referee: any `-check-*` flag under `-engine all` puts the invocation
to both engines, `-check-depth` and `-check-states` being the one figure each bounds in its own
unit (else the exploration's own, 64 deep and 1024), and the `standing:` line names what each
found.

Properties are named by `-check-property`, not by `-requirement`/`-constraint` — those ask an
`evaluate` question of the object, which `run` answers once and `check` refuses by name — and
are evaluated at every stable state of the action (no body mid-statement) and at its
completion, on the performing object when the action is one an `-instantiate`d object performs:

```bash
$ sysml -engine check -instantiate Fleet::truck -action "Fleet::Truck::dispatch truck" \
    -check-property Fleet::NeverOverloaded -check-diverge this.load model.sysml
```

A **witness** is the schedule that reaches a violation or a divergent value: its choice lines,
as `-trace` prints a `choice` under `-schedule explore` (`step 3: 3@right first of 2@left,
3@right`), then a blank line, then the trace of the run under that schedule, so it reads like
any `-trace` and is reviewed the same way; a witness of a deadlock or a failure ends, after a
blank line, in `fails: <the error>` as the run raises it, since the failing move may leave no
trace of its own. `-check-witness <dir>` writes one file per witness,
named for the behavior, the object performing it when `-action` or `-state` names one, and
what it witnesses (`Mission.race-x-1.witness`, `Mission.race.violation-1.witness`,
`Plant.Tank.fill@Plant.tank-this.level-1.witness`; behaviors checked together on one clock are
joined by `+` and a feature of one of them spelled under its name,
`Shine.Lamp.peek+Shine.Lamp.glow@Shine.Lamp-Shine.Lamp.peek.saw-1.witness`; a character of a
name that is no letter, digit or `_` is spelled `%XX`, so two features spelled apart never share
a file, and one behavior checked on two objects writes two sets), and the verdict names each
path. Every witness is
**replayed** before it is reported: the interpreter re-runs the action under
`-schedule replay:<file>`, the schedule policy that follows a witness file's choice lines, and
the standing is *witnessed* only when that run reaches the state the witness claims with the
same trace — failing as it claims, or going on where it claims a state; one that does not is
reported *not covered* with the disagreement as its reason.
The same file replays by hand, with the trace showing every choice taken as the witness fixed
it:

```bash
$ sysml -schedule replay:witnesses/Mission.race-x-1.witness -action Mission::race -trace race.sysml
✓ package Mission
[trace] step 1: token 1@f
[trace] step 2: token 2@left, token 3@right
…
[trace] choice step 3: tokens 2@left, 3@right (unordered; took 3@right first)
…
```

**Bounds.** `-check-depth` cuts a schedule after that many moves (a merge loop that never
completes is cut here rather than searched forever), `-check-states` cuts the search after that
many distinct states, `-check-timeout` ends the plan on the wall clock, and the executor's own
budgets (`OPENSYSML_MAX_ACTION_STEPS` and its kin) cut a schedule as they cut any run; each is
named in the verdict when hit, and every bound the search ran under is listed under `bounds`
with `-json`. A search that hits one never claims exhaustiveness:

```bash
$ sysml -engine check -action Mission::race -check-depth 3 race.sysml; echo $?
✓ package Mission
? Action Mission::race: no violation within bounds (5 states, 4 moves, depth 3; bounds hit: depth)
  standing: outcomes (bounded over schedules: 5 states, 4 moves searched, depth=3 (reached))
2
```

**State machines and the clock.** A `-state` is searched as an `-action` is: one move is one
dispatch — the event at the head of the queue taken, the transitions it enables selected and
fired, entries and effects run — or one step of a `do` behavior that is due, and the choice
points are the machine's (which transition of those enabled, which region reacts first, which
of the events due at one instant dispatches first, and, for a `do` action, which of the states
with one due acts first). Without `-advance` the search runs the clock until nothing more is
due; a machine whose timer re-arms forever is searched until a bound cuts it, so give it a
horizon. With `-advance D` the behaviors named are **one invocation on one clock**, answered by
one verdict named for them all (`Behaviors Shine::Lamp::peek, Shine::Lamp::glow`), the machines
of the objects they materialize on that clock beside them: an action's wait and a machine's
timer due at one instant are a choice the search draws, so what the action reads of the machine
may diverge. The search runs the clock to the horizon as `-advance` alone does, so a property
is evaluated there — the verdict reads `exhaustive up to t=D`, a wait past it left unreached —
and a machine resting where nothing wakes it is a complete schedule,
not a deadlock; its `outcome:` line names its `finalState` and the states it visited under its
name. Without `-advance` each behavior named is its own search. A body paused mid-statement — an
action performing another that waits inside its body, a `do` behavior waiting at an `accept` —
is a state the search holds and resumes like any other.

The checker searches one invocation under a single executor, so `-jobs` does not divide a
search; it runs the replay of each witness on the plan's workers.

Misuse is refused before anything runs: a `-check-*` flag without `-engine check` or `-engine
all`, a `-check-*` flag without an `-action` or `-state` to check (`-engine check` alone, like
any `-engine`, is the prompt's selection), `-advance` or a `-state` under `-engine smt` (whose
search is an action's alone), and a bound that is no positive integer (`-check-depth 0`,
`-check-states x`) or no duration.

With `-json` the check's `results[]` entry for the `check` engine carries, beside `claim`,
`strength`, `bounds` and `witness`, a `check` object: `verdict`, `states`, `moves`, `depth`,
`boundsHit[]`, `violations[]` (each with its `kind`, `detail`, `witness` choices, the `draws`
the run made when it drew, and file `path`), `divergent[]` (each `feature` with its `values[]`,
each with `value`, `witness`, `draws` and `path`) and `outcomes[]`.

### Deciding a property over the inputs

The `smt` engine answers the same `-check-property` question symbolically: it encodes the
action's schedules as a relation over its states, hands the property to an SMT solver (z3 or
cvc5, found on `PATH`) and decides it for **every schedule and every value of the free
inputs**, not for the one value each input was written with. A feature the model binds — an
attribute with a default, a value the performing object holds, an argument the invocation
passes — is pinned at that value, as `check` and `explore` run it; a feature the model leaves
unbound, or one `-check-input` names, ranges over the domain of its declared type: `Boolean`;
`Integer`, within the interpreter's 64-bit range; `Natural` as an integer that is not negative;
`Real`, `Rational` and a quantity type over them as the solver's reals; an enumeration or a
variation as its constructors. A declared type the encoding cannot narrow to a domain —
`String`, a collection, an object-valued feature, a type with no translation — is reported *not
covered* naming the feature and the type before the solver is asked, never left as a silent
unconstrained variable. The design is in [SMT model
checking](../internals/design/smt-model-checking.md).

```bash
$ sysml -engine smt -action Gate::open -check-property Gate::open::positive gate.sysml
✓ package Gate
✓ Action Gate::open: holds
  inputs: n = 1, limit = 5
  standing: holds (proved over schedules: inputs as written)
$ sysml -engine smt -action Gate::open -check-property Gate::open::positive \
    -check-input limit -check-witness witnesses gate.sysml; echo $?
✓ package Gate
✗ Action Gate::open: at step 0: requirement positive: require condition evaluated to false: n + limit > 0
  inputs: n = 1, limit = -1
  witness: witnesses/Gate.open.violation-1.witness
  standing: violated (witnessed: witness of 1 input replayed, inputs chosen from their domains: limit = -1)
1
```

The `inputs:` line lists every feature the encoding pinned or freed — `limit = 5` pinned,
`limit : Integer free` ranging — and the standing says which claim was made: *proved over
schedules: inputs as written* when every input was pinned, *proved over schedules and inputs:
inputs free in their domains: …* when some ranged, *inputs chosen from their domains* on a
violation, naming the values the solver picked. `-engine check` and `-engine explore` without
the release still find no violation, since they run the inputs as written, and their standing
says so; under `-engine all` the plan shows `check refused (check cannot leave the inputs
free)` beside `smt`'s answer whenever an input is free. Any of `-check-input`, `-check-assume`
and `-check-unroll` alone, with no `-check-property`, makes the question one of what holds, so
under `-engine all` it reaches `smt` (and `check`, which refuses the first two) rather than the
exploration.

`-check-assume <name>` asserts a constraint or requirement over the initial state, so the
claim is made only for the inputs it admits; each assumption is listed on an `assumed:` line
and in the standing. A set no initial state satisfies is reported *not covered* with
`assumptions admit no initial state` as its reason — there is nothing to prove over — never
*proved*:

```bash
$ sysml -engine smt -action Gate::open -check-property Gate::open::positive \
    -check-input limit -check-assume Gate::open::wide gate.sysml
✓ package Gate
✓ Action Gate::open: holds
  inputs: n = 1, limit : Integer free
  assumed: constraint wide
  standing: holds (proved over schedules and inputs: inputs free in their domains: limit : Integer free, assumed constraint wide)
```

A **witness** of the solver's opens with the input values it chose, one `input <feature> =
<value>` line per free input — an enumeration value spelled as its qualified constructor, a
real as the exact rational the solver returned, `null` for a feature declared `[0..1]` the
solver left without a value — ahead of the choice lines `check` writes, so a witness without
inputs is the format it always was:

```
input limit = -1
no choice points

…
```

`-schedule replay:<file>` pins those features before the run starts, as an argument the
invocation passes is pinned, then follows the moves; every solver witness is replayed this way
before it is reported, and one whose inputs cannot be set, or that does not reach the state it
claims, is *not covered* in the interpreter's favor. An input line naming a feature the action
does not have is refused naming it.

**Bounds.** `-check-depth` is the number of moves the action is unrolled to (default 40 under
`smt`), `-check-unroll` the iterations of one loop unrolled within it (default 4) and
`-check-timeout` the solver's clock per query, `OPENSYSML_SMT_TIMEOUT` (default 10 s) without
it; a bound hit is named in the standing and the claim is *bounded*, not *proved*.
`-check-input`, `-check-assume` or `-check-unroll` without `-engine smt`
or `-engine all` is refused before anything runs, naming the flag as the `smt` engine's, as
`-check-states` under `-engine smt` alone is refused as the `check` engine's — a flag only the
engine left out would read is never dropped silently;
`-check-input` naming no feature the action reads, and `-check-assume` naming a constraint the translator cannot encode, are refused naming
it before the solver is asked, and the check is *not covered*.

With `-json` the `smt` engine's `results[]` entry carries `inputs[]` — each with its `name`,
`type`, `domain`, whether it was `free`, whether it is `optional` (declared `[0..1]`, so the
solver ranged over its absence too) and, when pinned or chosen, its `value` — and `assumptions[]`,
and its `witness` carries `inputs[]` (each `feature` and `value`) beside `schedule` and
`choices`, with the `path` of the file `-check-witness` wrote.

The `smt` engine is registered at authority *proved*, so `-engines` lists it with its solver
(`ready (z3 at /usr/bin/z3)`) or the solver's absence, and a check put to it without a solver is
*not covered: no solver*. `auto` never asks a `holds` question today — a property is checked
only under `-engine check`, `-engine smt` or `-engine all` — so no plan line of an invocation
without `-engine` goes through the solver.

### Deciding whether the schedule decides a feature

`-check-diverge <feature>` asks whether the feature's final value depends on the order the
schedule takes, and both engines answer it: `check` by the divergence search above, `smt` by a
**two-copy query** — two copies of the action's schedules from one initial state, both complete
within `-check-depth` moves, ending with different values of the feature. Naming a feature makes
the question one of *sensitivity* rather than of what holds, so under `-engine all` one
question reaches both engines and the standing compares their answers. A `-check-property`
beside it is still checked first: a violation, a deadlock or a typed error on any schedule is
the verdict, the feature having no final value there.

```bash
$ sysml -engine smt -action Mission::race -check-diverge x -check-witness witnesses race.sysml; echo $?
✓ package Mission
✗ Action Mission::race: sensitive: x ends as 2 or 1; the schedules part at step 3: step 3: 2@left first of 2@left, 3@right against step 3: 3@right first of 2@left, 3@right
  inputs: x = 0, leftRan = false, rightRan = false
  witness A: witnesses/Mission.race-x-A.witness
  witness B: witnesses/Mission.race-x-B.witness
  standing: sensitive (witnessed: witness of 1 choice replayed, inputs as written)
1
$ sysml -engine smt -action Mission::race -check-diverge leftRan race.sysml; echo $?
✓ package Mission
✓ Action Mission::race: holds
  inputs: x = 0, leftRan = false, rightRan = false
  standing: holds (proved over schedules: inputs as written)
0
$ sysml -engine smt -action Mission::race -check-diverge x -check-depth 3 race.sysml; echo $?
✓ package Mission
? Action Mission::race: holds (bounded)
  no sensitivity found within 3 moves: a schedule is still live after move 3 (step 3: 3@right first of 2@left, 3@right)
  inputs: x = 0, leftRan = false, rightRan = false
  standing: holds (bounded over schedules: inputs as written, moves=3 (reached))
2
```

A *sensitive* verdict names the two final values, the earliest move at which the two schedules
part and the move each takes there, and is *witnessed* only once **both** schedules were
replayed by the interpreter to the values the solver claimed; a schedule that replays to another
value is *not covered* with the disagreement, in the interpreter's favor. `-check-witness`
writes the pair as `<checked>-<feature>-A.witness` and `-B.witness`, each a file
`-schedule replay:<file>` and `%replay` follow. A feature the schedule does not decide is
*holds* at *proved*: every schedule completes within the bound and all of them end with one
value, for the inputs as written or free in their domains. A bound that cuts some schedule
short — the depth, a loop's unrolling — leaves the question open on that schedule, so the verdict
is *holds* at *bounded* with `no sensitivity found within k moves` and the still-live schedule
or the cut loop as its reason, never *proved*; and a question the solver cannot decide
(`unknown`, a timeout) is *not covered*. Where every schedule deadlocks or raises a typed
error, that violation is the verdict. The performing object's own features (`this.level`) are
not encoded by the `smt` engine, so asking about one is *not covered* naming it; `check`
answers it beside `smt` under `-engine all`.

With `-json` the `smt` engine's `results[]` entry carries the pair as `witness` and `contrast`,
each with its `schedule`, `choices`, `inputs[]` and the `path` `-check-witness` wrote, and the
verdict's `reason` names the two values and the parting move.

## Output Format

All evaluations include checkmark and result:

```bash
$ sysml -e "10 * 2"
✓ 10 * 2
  = 20
```

For file loads, declarations are summarized:

```bash
$ sysml demo.sysml
package Demo
  part def Vehicle {
    attribute speed : Real;
  }
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

## Error Handling

**On every non-interactive run, results go to stdout and findings go to stderr.**
Evaluated values, conversion output, verdict lines and the `✓` echoes of what a
load declared are results. Model diagnostics (errors and warnings alike) and
anything that stopped the run are findings, and so is the `wrote <file> …` note a
successful `-convert -o` prints, which stays off stdout so a conversion can be
piped.

A file that cannot be read ends the run:

```bash
$ sysml missing.sysml
sysml: cannot read missing.sysml: no such file or directory
$ echo $?
2
```

A model that does not analyse cleanly answers nothing, so its diagnostics end the
run rather than reporting an evaluation against a model nobody could read:

```bash
$ sysml -e "1+1" bad.sysml 2>/dev/null
$ echo $?
2
$ sysml -e "1+1" bad.sysml 2>&1 >/dev/null
bad.sysml:2:39: error: expected an expression
  part def Vehicle { attribute mass = ; }
                                      ^
sysml: bad.sysml did not analyse cleanly
```

An expression that cannot be evaluated is reported the same way, leaving what the
load declared on stdout:

```bash
$ sysml -e "Demo::Vehicle::nope" model.sysml
✓ package Demo
sysml: unresolved reference: Demo::Vehicle::nope
$ echo $?
2
```

A name that needs quoting and was written without them is such an expression, and its
failure names the quoted declaration ([writing names](#command-reference)):

```bash
$ sysml -e "T::SA-506" model.sysml
✓ package T
sysml: evaluation failed: unresolved reference: T::SA — did you mean T::'SA-506'? Names containing '-' must be quoted.
```

So `2> errors.log` collects everything a script would otherwise have to pick out
of the results, plus the `wrote …` note of a successful `-convert -o` and any
warning the model raised. Neither of those changes the status, so a non-empty
log is not by itself a failure; the exit status is what to check.

## Exit status

The exit status contract is the same whatever the run was asked to do. This is
the one place it is written down; [the guide](../guide/) links here.

| Status | Means |
|--------|-------|
| `0` | What was asked for was done: every file loaded and analysed cleanly, every `-e` expression produced a value (`<undetermined>`, the model-level value of an expression over a feature the model leaves open, is one), every check held, a conversion was written. Warnings leave the status `0`. |
| `1` | The model answered false: a constraint, requirement or satisfaction assertion the model decided did not hold. Only a verdict reports this status. |
| `2` | What was asked for could not be done, so the model answered nothing: a file that could not be read, a model that did not analyse cleanly, an object whose feature values did not materialize, an unresolved name, a check that could not be made (including a condition that is `<undetermined>`: it is reported as `no value`, naming the feature the model leaves open, never as a verdict), an exploration that hit its budget before every linearization was tried, a conversion that could not be written because the RDF graph cannot rebuild a source construct, a misused flag or an invalid `OPENSYSML_MAX_*` value. |

```bash
$ printf '%s\n' 'constraint MassBudget { 1 > 2 }' > model.sysml
$ sysml -constraint MassBudget model.sysml; echo $?      # a verdict the model decided
✓ constraint MassBudget
✗ Constraint MassBudget failed
  Assertion evaluated to false: 1 > 2
1

$ sysml -debug -quiet model.sysml; echo $?
sysml: -debug and -quiet are mutually exclusive
2

$ OPENSYSML_MAX_STEPS=abc sysml -e "1+1"; echo $?
sysml: OPENSYSML_MAX_STEPS="abc" is not an integer: set it to a positive number of evaluation steps (default 10000000)
2

$ sysml examples/parser_features_demo_advanced_bodies.kerml -convert ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
sysml: cannot convert the operator expr at examples/parser_features_demo_advanced_bodies.kerml:87:9: save to .sysml or .kerml instead, which writes the source exactly; see docs/reference/rdf-mapping.md § Limitations
2

$ sysml examples/state-machine-demo.sysml -convert ttl -o /tmp/state-machine.ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/state-machine.ttl (ttl, 2078 bytes)
0
```

Instantiating an object is part of the run, so what it finds is a diagnostic
about the model. `-instantiate` reports every feature value it could not build, such
as a default whose number of values does not fit the feature's multiplicity (which is
`1..1` for a feature that declares none), and `-validate` reports `no errors` only
when the run found none. The REPL follows the same rule: a command that showed a
feature value it could not build (a `%features` listing containing `<error: …>`, or an
`%eval` of such a value, with or without a context via `%eval in <name> : <expr>`)
has answered nothing about it, so a session driven from a pipe exits `2` rather than
reporting success, whatever analysis found. Asking for a name that is not a feature of
the object is a mistake in the request, not a feature value that failed to build, and
does not change the status.

```bash
$ cat > model.sysml <<'EOF'
package test {
  part def Sub;
  part def Craft {
    part left : Sub;
    part right : Sub;
    part volumes : Sub = (left, right);
  }
  part craft : Craft;
}
EOF
$ sysml model.sysml -instantiate test::craft -validate; echo $?
✓ package test
✓ Created instance of test::craft
  ID: 1
  Use %features test::craft to inspect
error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1
sysml: model.sysml did not materialize cleanly
2

$ printf '%%instantiate test::craft\n%%features test::craft\n' | sysml model.sysml; echo $?
✓ package test
Instance: test::craft (ID: 1)
Features:
  left = Instance(ID: 2)
    (no features)
  right = Instance(ID: 3)
    (no features)
  volumes: <error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1>
2
```

Nesting multiplies, and reading a feature value builds the objects it holds, so the
check is bounded, just as the `%features` listing is. A model wide enough to use up that
budget, deeper than the walk descends, or with a part that contains its own kind is
reported as partly checked (`warning: … materialization is bounded; not every
feature value was checked`, and `no errors in the feature values checked`) rather than
read to the end. That is not a model error, so the status stays `0`.

The interactive prompt is the exception: a line it could not carry out is reported
and the session goes on, and `%quit` or Ctrl-D exits `0`. `sysml model.sysml` at a
terminal loads the model, reports what analysis found, and opens the prompt with
status `0`, because the prompt is where the model gets fixed. The same command with
its lines coming from a pipe or a file does gate: it exits `2` for a model that did
not analyse cleanly, and for one whose feature values a command could not build.

## Use Cases

1. **Quick Calculations**: Use as a calculator with `-e`
2. **Automated Testing**: Run model evaluations in CI/CD pipelines
3. **Scripting**: Extract calculated values for other tools
4. **Validation**: Check model properties without manual interaction
5. **Batch Processing**: Process multiple models programmatically
6. **Interactive Development**: Load files and explore in REPL

## Tips

- Use `%help` in REPL to see all meta commands
- Combine multiple `-e` flags to evaluate related expressions
- Load common definitions before custom models
- Read [Exit status](#exit-status) before gating a pipeline on it: `0` means the
  model answered what was asked, `1` that it answered false, `2` that it answered
  nothing
- Results go to **stdout** and findings (diagnostics, warnings, whatever
  stopped the run) to **stderr**, so `> model.ttl` and `2> errors.log` separate
  them
- Use shell pipes for REPL automation: `echo "%load file.sysml" | sysml`
