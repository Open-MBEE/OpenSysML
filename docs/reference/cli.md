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
[Exit status](#exit-status)):

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
| `--from <format>` | | Input format for `--convert`: the `--convert` formats, or `xmi`/`mdzip` for a SysML v1 model to migrate (default: from the input's extension; `.xmi` and `.mdzip` are recognized) — see [SysML v1 migration](sysml-v1-migration.md) |
| `--migration-report <file>` | | With `--convert` from `xmi`: write the element-by-element migration report to this file, JSON when it ends in `.json`, text otherwise. Without it the one-line summary goes to stderr |
| `--render <view>` | | Render this view of the model (every file named, loaded as one) instead of running it, in the form its `render` member states (see [Rendering a view](#rendering-a-view)) |
| `--render-all <dir>` | | Render every declared view into the directory, one artifact per view |
| `--render-form <form>` | | Form `--render` or `--render-all` writes: `text`, `mermaid` or `markdown` (default: destination-dependent for `--render`, each kind's machine-readable form for `--render-all`) |
| `--render-document <name>` | | Compile a document definition (a `part def` specializing `DocumentQueries::Document`), run its queries against the model, render its diagram blocks through the view engine and write the result as CommonMark Markdown, as `%render-document` does. Paragraphs may hold inline runs (`Span` with a `plain`/`emphasis`/`strong`/`code` style, `Link` to a URL, `Ref` linking to another content block's anchor); a query-backed paragraph or list styles its projected values through nested `SpanColumn`/`LinkColumn` column runs; a table with a `groupBy` column writes one subtable per group value, with the query's projected properties and computed `Column` names as its columns. A `Diagram` block embeds a declared view, or an element with a stated rendering kind, as a fenced ` ```mermaid ` block (a table-kind view as a pipe table), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown is the default form; `-doc-form html` renders the same document tree as semantic HTML (see [Rendering a document as HTML](#rendering-a-document-as-html)) and `-doc-form pdf` converts the Markdown (see [Rendering a document as PDF](#rendering-a-document-as-pdf)). `-json` does not apply. See the [document generation manual](../manual/README.md) |
| `--doc-form <form>` | | Form `--render-document` writes: `markdown` (default), `html`, rendered from the document tree itself (see [Rendering a document as HTML](#rendering-a-document-as-html)), or `pdf`, which drives an external converter |
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
| `-constraint <name>` | One constraint, as `%constraint` does |
| `-requirement <name>` | One requirement, as `%requirement` does, with [the verdict of every verification case](#verification-case-verdicts) verifying it beside its own |
| `-satisfy` | Every satisfaction assertion the model states, with [the verdict of every verification case](#verification-case-verdicts) verifying the requirement beside each |
| `-satisfy=<name>` | Only the assertions the named element states (`-satisfy=false` asks for none) |
| `-instantiate <name>` | Creates an object first, so the verdicts are about it |
| `-calc "<name>(<args>)"` | Invokes a calculation and reports what it computed |
| `-analysis "<name>[(<args>)] [object]"` | Runs an analysis or [verification](#verification-case-verdicts) case — a [trade study](#trade-studies) included — and reports its `out` and `return` values with their units, then the verdict of its `objective` — `satisfied`, `not satisfied` with the violated condition, or `undecided` with the reason — as `%analysis` does. An objective typed by a requirement def binds the def's subject as a requirement usage does (`subject = ship;`, `subject s = ship;` or `subject :>> s = ship;`); one binding none checks the case's result, the library's default for it, and is `undecided` naming the type when that result is not of the subject's type. Arguments bind the case's `in` parameters, positionally (`Pkg::Case(3.0)`) or by name (`Pkg::Case(limit = 3.0)`); the object, one `-instantiate` created and named as `-state` names its performer, is the case's `subject`. A usage that binds its subject (`subject s = ship;`) needs no object; a definition, or a usage that binds none, is refused by name without one. A verification case runs the same way and reports beside those verdicts the `VerdictKind` its body produced. Repeatable |
| `-run-query "<name> [<p>=<expr>...]"` | Executes a document query and reports its rows, as `%run-query` does — including any computed `Column(name = "<column>", expression = <expr>)` projections evaluated per row. Each binding is written as `<parameter>=<expression>` |
| `-action "<name> [object]"` | Runs an action to completion and reports its outputs |
| `-state "<name> [object]"` | Runs a state machine and reports where it settled. The object is one `-instantiate` created, named as `%state` names it: a usage's name, a feature path to a part it holds (`Fleet::driver.r`), or the id the report prints (`#2`). Naming the machine the object exhibits attaches to its running machine rather than performing it again (a definition exhibited as several usages is refused with the usages to name instead); naming a usage whose definition alone was instantiated says which usage to `-instantiate` |
| `-advance <time>` | Simulated time (seconds, `SI::s`) the invocation's `-action` and `-state` behaviors run for, on the one clock they share: every state event, action `accept after`/`accept at` and do behavior due within it runs, in due order, and two behaviors due at the same instant run in the order `-schedule` picks (the one started last first by default), reported as a choice point. A state machine takes only its initial transition without it; an action runs to completion on its own without it and, with it, only as far as that much time takes it, so one still waiting on the clock is reported as undecided with the instant it waits for. Refused without an `-action` or `-state` to run |
| `-sweep <param>=<from>..<to>[:<step>]` | Runs the `-analysis` case or `-calc` once per value of the range, rather than once, and reports the runs as a table. `<from>`, `<to>` and `<step>` are written as an argument is, units included (`0.0 [SI::m]..10.0 [SI::m]:2.0 [SI::m]`); the parameter is one the case or calc declares and the arguments do not bind, and the values are produced in its declared type (`1..4:1` over a `Real` binds `1.0`, `2.0`, …). Repeatable: several ranges run their cartesian product, the first flag given varying slowest. See [Sweeping a parameter](#sweeping-a-parameter) |
| `-samples <n>` | Draws `n` values for each `-sweep` range instead of running every value of it, uniformly over the range from the seed `-seed` names — Integers inclusively for a parameter taking Integers, reals in `[<from>, <to>)` for one taking reals |
| `-seed <s>` | The seed `-samples` draws from, required with it: the same seed draws the same values on every platform |
| `-schedule <policy>` | The scheduling policy every run this invocation starts — `-action`, `-state`, `-analysis`; a calc's body performs nothing, so `-calc` has no choice to make — resolves its [choice points](../guide/06-behavior.md) under: `reverse` (the default: reverse token order, first holding guard, first enabled transition), `declared` (spawn and declaration order), `seed:<n>` (a pseudo-random order the non-negative integer `n` fixes, the same on every platform) or `explore[:runs=N,depth=D]` (every linearization within the budget, tabled by distinct outcome — see [Exploring every linearization](#exploring-every-linearization)). Every choice point the run reaches is reported and the `took …` in each is what the policy took; another policy's run may reach other choice points, so their count is not fixed across policies. A spelling naming no policy — an unknown name, `seed` or `seed:` without a number, `seed:-1`, `seed:abc`, `explore:` with nothing after the colon, `explore:runs=0`, `explore:depth=-1`, an option named twice — is refused before anything runs |
| `-json` | Reports the checks as one JSON document rather than as lines |

**Arguments:**
- `[file...]` - SysML files to load (loaded in order)

**Usage pattern:**
```
sysml [options] [file...]
```

Flags may be written before or after the files. `--` ends the flags, so a file whose
name looks like a flag can be given after it: `sysml -trace -- -m.sysml`.

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
text form uses `.txt` and unbounded width.

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
`-render-document`, `-render`, `-render-all`, `-o`, `-convert`, a query flag, or a check flag.
Rendering a single document with `-render-document` still succeeds when it has cross-document
references: the links point at the targets' expected file names and dangle until those documents
are rendered into the same directory.

`-render-document` takes as many model files as the document needs, loaded as one model, so a
document can query elements declared in sibling files:

```bash
sysml model/*.sysml -render-document Reports::MassReport -o report.md
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
without diagrams needs no diagram tool.

Inline runs keep their meaning in PDF: emphasis, strong and code styling, links, and `Ref`
cross-references as clickable internal links to their targets' invisible anchors, in every engine
(`weasyprint` and `prince` through the prepared HTML, `pandoc` through the Markdown itself). A
grouped table's group key renders in bold above each subtable.

A PDF is a binary artifact, so `-doc-form pdf` requires `-o`. A missing tool stops the run with
status 2 and a message naming the tool, its override variable and the other engines; a converter
that fails reports its own output. `scripts/download-doc-pdf-toolchain.sh` installs pinned copies
of WeasyPrint, pandoc and mermaid-cli under `build/doc-pdf/` and prints the variables to export
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
its name (`Demo::Beacon::blinking finalState = "shining"; Demo::watcher.sawLit = true`), and the
witness names which executor ran first (`t=5.0: state machine blinking of object #1 first of action
watcher, state machine blinking of object #1`); an action still waiting on the clock when the time
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

So `2> errors.log` collects everything a script would otherwise have to pick out
of the results, plus the `wrote …` note of a successful `-convert -o` and any
warning the model raised. Neither of those changes the status, so a non-empty
log is not by itself a failure; the exit status is what to check.

## Exit status

The exit status contract is the same whatever the run was asked to do. This is
the one place it is written down; [the guide](../guide/) links here.

| Status | Means |
|--------|-------|
| `0` | What was asked for was done: every file loaded and analysed cleanly, every `-e` expression produced a value, every check held, a conversion was written. Warnings leave the status `0`. |
| `1` | The model answered false: a constraint, requirement or satisfaction assertion the model decided did not hold. Only a verdict reports this status. |
| `2` | What was asked for could not be done, so the model answered nothing: a file that could not be read, a model that did not analyse cleanly, an object whose feature values did not materialize, an unresolved name, a check that could not be made, an exploration that hit its budget before every linearization was tried, a conversion that could not be written because the RDF graph cannot rebuild a source construct, a misused flag or an invalid `OPENSYSML_MAX_*` value. |

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
