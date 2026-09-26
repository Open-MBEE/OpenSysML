# Changelog

Notable changes per release. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) and, before
1.0, are decided by model compatibility as CONTRIBUTING.md § Versioning states. Cutting a
release is described in [docs/project/releasing.md](docs/project/releasing.md).

## Unreleased

## 0.9.1 — 2026-09-26

### Added

- **Add connection-like usages through `ApplyEdits`.** Python exposes `Editor.add_connection`, `add_allocation` and `add_flow`; Go and Java expose `AddConnection`. The operation requires both the `authoring` and `connection_authoring` capabilities.

- **Extend source-preserving authoring with member modifiers, satisfy usages and requirement constraints.** ApplyEdits now supports grammar-checked modifiers and requirement statements, with dedicated service capabilities and client preflight.

- **A `cameo` drawing style draws the DOT form as Cameo Systems Modeler draws a diagram.** `-render-style cameo` beside `-render-palette` on `-render`, `-render-all`, `-render-document` and `-render-documents`, `%render <view> dot [palette] cameo` and `%render-document <name> dot cameo` in the REPL, `"style": "cameo"` on `opensysml/render` (the styles listed under the new `openSysmlRenderStyles` capability) and `cameo` in the VS Code panel's **Style** list (`opensysml.diagram.style`) all draw a diagram frame with the `stm [State Machine] Owner [ Name ]` header tab, 11 pt Arial, Cameo's gradient fills — pale yellow states, green actions, orange blocks — under thin dark borders, a state's `do / Activity` compartment beneath a rule, a block's «stereotype» line and compartments, rounded composite states with dashed region dividers, the initial dot, final bull's-eye, decision diamond and fork bars, `trigger [guard] / effect` transition labels and notes as folded boxes on a dashed anchor, every colour measured from Cameo's own pages and recorded in the rendering-forms design note. `pilot`, the Pilot visualizer's Standard B&W, stays the default; Mermaid and PlantUML note any other style as not represented.
- **`DiagramLayout::Style` and `DiagramLayout::Note` carry a diagram's colours, fonts and notes.** A `Style` on a member (`fill`, `line`, `text` colours, `font`, `fontSize`, `bold`, `italic`) is drawn over whichever style the DOT form draws in, explicit values winning over the look's defaults; a `Note` on a view (`text`, `x`, `y`, `width`, `height`) about a member is drawn beside it with a dashed anchor. The validation pass checks both as it checks a `Layout`, `opensysml/render` reports a node's or edge's `style`, and `opensysml/applyModelEdit` takes `setStyle` as it takes `setLayout`.
- **A migrated diagram is laid out, coloured and annotated from its own MagicDraw stream.** `sysml Project.mdzip -convert sysml` reads each diagram's `mdOwnedViews` symbols for the frame and every symbol's geometry, each path's breakpoints and end symbols, the fill, pen and text colours and font each symbol was drawn with, and the text of every note anchored to a drawn element, and writes them as `DiagramLayout` `Canvas`, `Layout`, `Route`, `Style` and `Note` metadata on the migrated view, so no MTIP export is needed to keep a Cameo diagram's shape; an MTIP `-layout` record still overrides the stream for every element it places or routes, the stream supplying the rest. The migration report counts the symbols positioned, routed, styled and noted, and the free symbols it dropped — pasted images (their class, geometry and attachment name are exposed for a later change) and unanchored text boxes.
- **A migrated document publishes to PDF with Graphviz figures at Cameo fidelity.** `sysml Project.sysml -render-document 'Project::DesignDescription' -doc-form pdf -diagram-form dot -render-style cameo` runs Graphviz (`OPENSYSML_DOT`) under `neato -n2` for a diagram whose every member is placed and routed, and embeds the SVG; the migration guide's new *Publishing a migrated document with Cameo-style diagrams* section gives the command, the toolchain variables and what the path does not carry.

- Documents gain an `Image` content block (`location` a path relative to the document's file or an http(s)/file URL, optional `caption` and `alt`), rendered as a CommonMark image under its caption in Markdown, a `<figure class="sysml-image">` in HTML and the drawn image in PDF — a missing local file is a `missing-image` error naming the block.
- The SysML v1 migrator writes View Editor image paragraphs — comments stereotyped MagicDraw «AttachedFile», or carrying an `<img>` — as `DocumentQueries::Image` blocks, and copies the attached bytes beside the notation under `images/` when `-o` names a file (stdout or a Flexo target reports the requirement instead of dropping them); an attached file no archive entry holds is refused with the file named. `-image-base-url` resolves a comment body's relative `<img src>` — a path the View Editor serves — to a remote `Image` location (a documentation `<img>` is documentation text and is not shown); without it the paragraph keeps its text with a note naming the flag; a figure whose diagram draws nothing but whose note holds an `<img>` becomes an `Image` block the same way.

- Added thin Julia and MATLAB clients for `sysml-grpc` (`client/julia/OpenSysML` and
  `client/matlab`), each a JSON-over-HTTP client of the Connect-JSON surface with a conformance
  runner driving every scenario — the Julia client from a private child or a named service, the
  MATLAB client from MATLAB R2019b+ or GNU Octave 7+ (an Octave built without Java cannot spawn
  a private child, so Octave uses a named service). `make conformance-julia` and
  `make conformance-matlab` run the suite.

- **Pictures pasted onto a Cameo diagram survive migration.** `sysml Project.mdzip -convert sysml -o Project.sysml` decodes the bytes MagicDraw serializes into each `ImageShape` symbol's `<image>` tag (space-separated hexadecimal octets), writes them beside the notation under `images/` named after the pasted file with the suffix the bytes' content type calls for (the same bytes written once), and draws each where Cameo drew it as `@DiagramLayout::Picture { location; x; y; width; height; alt; above }` on the migrated view — under the element symbols, or `above = true` for a sticker over them. A diagram of pictures alone is a view drawing them, so a document figure of it shows the pictures under the diagram's name instead of being left out; a symbol naming a file the archive does not hold is reported as before, and octets that do not read are an `unmapped` row for the symbol. The report's layout note says what was written per diagram and its summary counts the pasted images.
- **`DiagramLayout::Picture` places an image on a view.** `metadata def Picture { location; x; y; width; height; alt; above }` in the DiagramLayout library draws the file at `location` (relative to the view's file) at stated bounds on any view; the validation pass checks its bounds, that it annotates a view and that `location` is a file's path — a URL, which no drawing tool reads, is refused rather than handed to Graphviz as a file name — the DOT form pins it as an image node (inlined as a data URI when the SVG is embedded in an HTML or PDF document, so the page stays self-contained — only a file whose bytes are a recognised image, an SVG being one well-formed document with an `svg` root; any other file stays a path), and the Mermaid and PlantUML forms count it in their `not represented` notice.
- **`-doc-number-figures` numbers figures and tables.** Captions read `Figure 1. <caption>` for a drawn diagram or an `Image` block and `Table 1. <caption>` for a query table or a table-kind diagram, in document order, alike in the Markdown, HTML (the number in a `sysml-caption-number` span) and PDF forms; off by default, so existing renderings are unchanged.

- **Document queries read a feature through a member nested in the row element.** A `Column` expression may be a feature chain — `'Monte Carlo'.runs`, `stat.runs`, `outer.inner.value` — and a `properties`/`property` string may hold the same `.`-joined path; each segment names a member of the element reached so far, own members before inherited ones. A row lacking a segment makes the path absent on that row alone (an empty cell or a `??` default), a member declaring no value is an empty cell, a multi-valued member fills the cell — more than its multiplicity admits fails the column as a direct feature column does — and `OrderBy` sorts by the path; a path no row reaches stays an unknown-property error. This is what lets a migrated individual's nested usage, such as a Monte Carlo analysis's `runs`, surface as a document column.

- **Queries expose the requirement and satisfying feature of a satisfy usage.** Select `satisfiedRequirement` and `satisfyingFeature` to follow each end of the satisfy relationship.

- **The satellite-network stress model can be generated as a fleet.** `tools/cmd/stress-model -fleet`
  declares each orbital plane as occurrences of one of four spacecraft blocks — `part sats :
  BlockA[400] ordered` — with the as-built values as the block's defaults and stated only on the
  units that diverge, instead of one `part def` per satellite; the spacecraft, ground segment,
  requirements and state machine are unchanged, and the ring, inter-plane and downlink connectors
  are declared once over each collection with `[1]` ends rather than once per satellite pair. `-stats`
  now reports the spacecraft definitions, the units carrying values of their own and the satisfy
  assertions in both forms, so the two can be compared: at 12 800 satellites the fleet declares 12 467 elements against
  2 354 827, and validates in 0.70 s and 184 MB rather than 331 s and 20.3 GB. A guide chapter,
  `docs/guide/modeling-fleets.md`, shows the constellation both ways and what the
  runtime does with 12 800 occurrences, and the stress-test record and performance notes carry the
  measurements. `BenchmarkFleetInstantiate` and `BenchmarkFleetSatisfy` in `tests/stressmodel`
  time the runtime over the fleet form.

- **Tools get a dry run.** `%tool <case|action>[(<args>)] [<object>]` at the prompt and `-tool-dry-run <case|action>` at the CLI show what the external tool a run's first `ToolExecution` names would be given — manifest, executable, argv, environment, working directory, standard input, input file and reply mapping — with the model's current values, without starting the process, then discards everything the preview performed; a manifest fault, an unregistered tool or an input the call does not send reports the same typed error the real run would fail with.
- **Runs record the tools they reached, and `-engines` spells the protocol.** `-record-run`/`%record` write every external tool call a run made into its `AnalysisRecords::RecordedRun` provenance as `tools`, one element per call in call order (`tool version from manifest: executable argv`); the `protocol` column a `-engines`/`%engines`/`ListEngines` listing shows for a tool spells how its manifest entry composes the process — `object` for the one-JSON-object exchange, or `argv+<stdin>` with a `/<reply format>` suffix such as `argv+none/csv` for an `invocation`/`reply` block.
- **A worked example walks the whole loop.** `examples/external-tool-demo/` registers a small Python solver as a tool, previews its invocation with `-tool-dry-run`, records a run — tools included — and renders the recorded run in a document; `docs/manual/running-external-programs.md` tells the same story end to end.

- **`ToolExecution` on a `calc def` or calc usage.** A `calc def` or calc usage annotated `metadata ToolExecution { toolName = "…"; uri = "…"; }` is computed by the named tool wherever a calc is invoked — `sysml -calc`, `%calc`, `EvaluateCalc`, derived attributes and document formulas — its `in` parameters sent by their `ToolVariable` names, its `out` parameters and result parameter (under its `ToolVariable` name, else its declared name, else `result`) bound from the reply. The body never evaluates: a tool unregistered, refusing or failing fails the calculation with the same typed errors a performance gets, and equal inputs answered differently are noted as a divergence.

- **A design note on running programs not written for the tool protocol**
  (`docs/internals/design/bring-your-own-engines.md`, section *Tools: composed invocations and
  structured replies*). Two optional blocks of the `OPENSYSML_TOOLS` entry are specified:
  `invocation`, which composes the command line, environment, working directory, standard input
  and an input file from the values the model binds, through a placeholder grammar with its
  rendering and escaping rules over a minimal base environment; and `reply`, which reads the
  outputs from JSON (by pointer), CSV (by column and row), key–value lines, or the exit status,
  on standard output or in a file the tool wrote, with the typed error each fault raises and
  where it is named. The note also specifies sequence-valued outputs, `ToolExecution` on a
  `calc def`, the `-tool-dry-run`/`%tool` surfaces, the recorded provenance, the security
  invariants and the delivery order. Nothing is implemented; an entry without the two blocks
  keeps today's meaning byte for byte, and the analysis-framework note points to the new section.

- **Tool manifests compose the external tool's command from the model's values.** An optional `invocation` block on an `OPENSYSML_TOOLS` entry renders `args`, `env`, a `cwd`, the standard input (`json`, `none`, `csv` or a template) and an `inputFile` from templates over the declared variables (`{mass}`, `{mass.value}`, `{mass.unit}`), the annotation's `{toolName}` and `{uri}`, and a per-invocation `{inputFile}` and `{outputDir}`, with `{{`/`}}` for literal braces. Each rendered argument is exactly one `argv` entry — no shell, no splitting — the working directory is confined to the manifest's directory as the executable is, and the process environment is `PATH`, `HOME`, `TMPDIR`, `LANG` plus the names `OPENSYSML_TOOL_ENV_PASSTHROUGH` lists and the block's own; the invocation's directory is removed once the reply is read unless `OPENSYSML_TOOL_KEEP=1`. A placeholder naming an undeclared variable is a manifest fault; a declared variable the performance did not send fails it before the process starts. The reply, timeout, size bounds and divergence report are unchanged, an entry without the block runs exactly as before, and `-engines` lists the protocol as `argv+json`, `argv+none`, `argv+csv` or `argv+template` beside `object`.

- **Tool manifests read the external tool's reply in four more formats.** An optional `reply` block on an `OPENSYSML_TOOLS` entry says how a tool answers once its process ran: `json` resolves each output by an RFC 6901 pointer into one JSON document, `csv` reads a cell by column and data row (with `header`, `delimiter`, `errorColumn` and a `unitColumn`), `lines` reads a `key = value` or `key: value` line or an RE2 named group, and `exitcode` reads the process's exit status as a Boolean against `success` or as an Integer. `source` is standard output by default or `file:<template>` under `{outputDir}` for a tool that writes a file — never the invocation's `inputFile`, and symbolic links under `{outputDir}` are not followed out of it; `errorPath`, `errorColumn` and `errorKey` carry the tool's own refusal; `type` reads each output as a number, integer, real, boolean or string, and `unit` or a unit selector sets the unit `ToolCall.Bind` converts as the object protocol does. Sequence-valued outputs (`row: "all"`, a JSON array) are refused for now, a `reply` key spelled twice is a manifest fault, and `-engines` lists the protocol as `csv`, `argv+none/lines` and so on beside `object`.

- **External tools can answer a whole sequence.** `row: "all"` reads every CSV data record in order, a JSON `path` naming an array reads its elements, and the object protocol's `"value": [..]` does the same; `lines` keeps its single-value form. A sequence binds only to a parameter whose multiplicity admits more than one value (`[0..*]`, `[1..*]`, `[0..n]`), with the bounds enforced like any write — a scalar for `Real[0..*]`, a sequence for `Real`, and an out-of-range item count are each `malformed output`. A recorded run writes the sequence as a list literal under an `ordered nonunique [0..*]` member (`attribute :>> temps = (300.0, 310.5, 341.2);`), and a document table cell shows every item. A record definition from an earlier release that held a sequence as a String is refused for a sequence-valued run with the `into` remedy named; records are not rewritten.

- **Source-preserving authoring now supports state transitions and behavior parameters.** Add regular or entry transitions with validated trigger, guard and effect clauses, and create calculations or actions with their parameters in one edit batch.

### Changed

- **A document's diagram form is chosen per diagram when `-diagram-form` is not stated.** A graph-shaped view some `DiagramLayout::Layout` or `Route` positions — a migrated Cameo diagram, a view laid out in an editor — is written as Graphviz DOT and drawn where it states: `-render-document`, `-render-documents`, `%render-document` and `opensysml/renderDocument` inline the SVG Graphviz draws (`OPENSYSML_DOT`, else `dot` on `PATH`) in Markdown and HTML as the PDF backend embeds it, and write a ` ```dot ` fence when no Graphviz can be run for them. Every other graph-shaped view stays Mermaid, so a document mixing both gets each in its own form; a stated `-diagram-form mermaid|dot|plantuml` still applies to every diagram. A positioned view rendered without Graphviz falls back to Mermaid under a visible notice — *drawn as Mermaid, not at its stated positions: Graphviz (dot) is not installed* — in the Markdown, the HTML figure and the PDF, never silently.

- **Every file the command line or `%load` reads is a document of its own.** `sysml -validate`, `-satisfy`, `-e` and the REPL's `%load` used to join the files they were given into one buffer with the typed transcript, so a model split over files was analysed as if it were one file; each file is now a workspace document, indexed with the others and analysed on its own, exactly as the editor and the OMG corpus gates analyse it. Two things a reader will observe: a root-level import in one file (`private import ScalarValues::*;`) no longer serves the other files on the command line or the prompt after `%load` — a KerML root import surfaces its names in its own document's root namespace only, as `docs/project/spec-compliance.md` records — and two files that both declare `package A` are no longer reported as `Duplicate of other owned member name`: they are two root namespaces of one name, and a reference to `A` resolves to the declaration in the file whose name sorts first (the order the editor gives documents, whatever order the files were given in), as the pilot implementation resolves a repeated root name to the first. Root packages stay reachable from every file and from the prompt through the global namespace. A differential test runs every multi-file directory of the fixtures and of the four OMG corpora through the command line and through a workspace and asserts the same diagnostics.
- **A file named `<repl>` is refused by every load.** `<repl>` is the name the REPL keeps the typed transcript under, so `%load <repl>` and `sysml -validate <repl>` report `cannot load <repl>: the name is reserved for the text typed at the prompt` before anything loads and exit as a read failure does; `./<repl>` loads it.

- **`-render-documents` writes every document it can and exits `3` when one could not be rendered.** One document whose query failed stopped the run before anything was written. Each document in the set is now compiled and evaluated on its own: the ones that render are written, a page stating **This document could not be rendered.** and the error stands in for each that does not — so links to it from the other pages resolve rather than dangle — and each failure is reported on stderr as `document <qualified name> could not be rendered: <reason>`. The new exit status `3` says the run was carried out in part; `0` still means every document was written and `2` that nothing was (no documents, a model that did not analyse). Same for `-doc-form html`.

### Fixed

- **A routed edge in the DOT form ends in an arrowhead.** The pinned `pos` spline now names its endpoint (`e,x,y`), which Graphviz needs to draw the head, so a transition migrated with a Cameo route is drawn with an arrow like an auto-routed one, rather than as a bare line.
- **Initial and final pseudo-states are drawn where their `DiagramLayout::Layout` puts them, as the dot and the bull's-eye.** A positioned initial state was dropped from a placed rendering and a positioned final state drawn as a `done` box; both are located and drawn as pseudo-states now, and a synthesized completion node is placed among its neighbours as a final state is.
- **A state's `do / Activity` compartment names the activity.** The state's behaviour line read `do, defers` where the model performed a named activity; it now reads `do / InitializePEAS`, the name of the performed behaviour, or its type's when the usage is unnamed.
- **An edge label on a routed edge is drawn beside its longest segment, not on top of a box.** The label sat at the route's midpoint, which for a Cameo route often falls inside a state; it is offset now along the segment's normal by half the label's extent.
- **A note anchored to a connector or transition is drawn.** A `DiagramLayout::Note` about an edge was written by the migration but never reached the rendering, so the note vanished from every form; it is collected with the edge now and drawn in DOT as a folded box on a dashed anchor to the middle of the edge's longest routed segment, and named on the edge by the text form.
- **A styled symbol keeps its `DiagramLayout::Style` and its anchored notes without a usable geometry.** The migration wrote a stream symbol's colours, font and anchored notes only when it had also positioned or routed the element, so an unsized shape, or a second symbol for an element already placed, lost them; the element's exposure in the view decides now, and its style is written once per element.
- **A migration whose model cannot be saved leaves the images beside the previous model untouched.** `-convert sysml -o model.sysml` from an `.mdzip` stages the model and every image file beside its destination and commits them only once all are written, rather than replacing the image files first and failing on the model.
- **A `DiagramLayout::Style` whose `bold` or `italic` is not a boolean is withheld like one with a malformed colour.** The problem was reported but the style still took effect; every binding now has to read for the style to apply.
- **A migrated image file is written under the suffix its bytes call for.** An attachment named `figure.txt` holding PNG bytes is written as `images/figure.png`, so the PDF engine and browsers read it as an image; a name climbing directories is reduced to its base name, and one with no usable name is written as `images/image.<ext>`.
- **A diagram's frame sizes its `DiagramLayout::Canvas` whatever `-layout` positions.** When the MTIP export placed and routed every symbol the stream draws, the canvas shrank to the bounding box of what was written and lost the frame's extent; the frame counts whenever the diagram's own stream has one.
- **A DocGen view reached by reference before it is composed keeps its nested views in the document.** A view class named by a non-composing property and later composed by another was not entered on the second path, so the paragraphs of its child views fell out of the document as stray; every composing occurrence is entered once now.
- **A figure's note that begins with its title keeps the rest as its caption paragraph.** The paragraph was dropped whenever the note or the title was a prefix of the other; it is dropped now only when the caption says everything the note does.
- **A migration refuses an `-o` one of its image files would land on.** Through a link from `images/` back to the model's directory an image could be committed after the model at the same file; every image destination is now checked against the model's, as against the input and report files.

- **SysML v1 migration reads the 2022x collaborator paragraph tags.** View Editor
  applications serialized by Cameo 2022x place a paragraph by `sectionId` and
  order it by `parentId`, with `viewId` naming the document's top view; every
  such paragraph was reported as a stray `names no view of the document` entry
  and dropped from the migrated document.

- **`-compare-results` over a large migrated model runs within bounded memory at its default parallelism.** Each configuration's verdict is printed as it is decided rather than after every configuration has run; a comparison run's objects and messages are released once its observables are read, its trace alone kept; the index of the model's `about` metadata is built once and shared read-only by every worker; a metadata type's default values are memoized once per type rather than copied into each of its annotations; the incoming successions of an action graph are indexed once rather than rescanned per step; and the runtime's message-queue scan no longer copies the queue per step. The default `-jobs` is now one per CPU bounded by the memory available — fewer where it leaves less than 512 MiB per worker, never fewer than one — and `-jobs`, `OPENSYSML_JOBS` and `%jobs` still set it outright. On the TMT model the reproduction that was killed at over 8 GiB now completes under 3 GiB, and the numbers printed for a fixed seed are unchanged.

- **A migrated DocGen `Image` of a Cameo table embeds the table, not a listing of its view.** An
  `Image` step (or a viewpoint-less view) drawing an instance table, generic table, matrix or
  relation map diagram wrote a `Diagram` of the view rendered `asElementTable`, which a document
  drew as an Element / Kind / Type / Declared-in dump of the view's members. The section now
  holds a `Table` over the same `… Rows` query the table's standalone document uses — written
  once, beside the view — captioned by the step's title, numbered among the tables, and the
  report row says which query it is over. A table whose definition has no query form is refused
  where the figure would be, with the reason, instead of drawn as the listing.
- **A DocGen view conforming to no viewpoint shows what MDK's default behavior shows.** Such a
  view wrote its documentation alone. It now follows `DocumentGenerator.parseView`: a view that
  is itself a diagram shows its own figure; any other shows, after its documentation, each
  diagram it exposes in order — a plain diagram as a figure, a table diagram as its table —
  and nothing for an exposed element that is not a diagram, the report row naming what it drew
  and what it left out. As in MDK, only a missing «Conform» means that: a view whose «Conform»
  names no element is refused with the reason, and the «View» stereotype's `viewpoint` tag
  chooses no method.
- **Collaborator paragraphs stand where their anchors put them.** Every collaborator paragraph
  followed the section's generated content. A paragraph with no `siblingId`/`parentId` now
  precedes it, as Cameo prints it; one anchored to a generated figure
  (`Containment_DiagramMainImage__<id>`) follows the figure, table or refusal the section drew
  for that diagram, with its followers after it — after the section's only figure when the anchor
  names no diagram of the model — and one whose anchor cannot be placed (another anchor kind, a
  tag naming nothing, an ambiguous section) follows the content with the reason in its row.

- **A table column over a multi-valued feature renders its values instead of failing the document.** A `Column` whose expression reads a feature declared `[0..*]` (a migrated DocGen table over `attribute :>> tCalibNB = (69.0, 98.0);`) stopped the whole document with "produced 2 values, expected one". The query planner now carries the column's declared multiplicity into execution: a cell holds as many values as the feature admits, written in order and `, `-joined in Markdown, HTML and PDF alike, and an optional feature with no value is an empty cell rather than an error. A column declared `[1]` still refuses zero or several values, naming the bound it expected, unless `?? null` opts its empty rows into an empty cell; a scalar place — a caption, a `Ref`, a comparison operand — still takes one value.

- **The DOT form no longer writes note anchors or edges to nodes it does not declare.** A nested action's own flow was dressed twice — once on the node drawn for it and once on a root the drawing never held — so a positioned action view with a commented nested action wrote anchor edges to undeclared nodes and `dot -n` refused it; such views render again. A note in a stated box has its label fitted to the box like a node's, so Graphviz no longer warns the size is too small.

- **An interconnection view no longer draws an exposed part or port twice.** Exposing a feature beside the part that already draws it nested drew it a second time as its own root — once under its container and again beside it, with a stub noting it was already shown. It is now drawn once, nested in its container, and connections attach to that one node, in DOT, Mermaid and PlantUML, positioned or not.

- **The language-server lifecycle tests wait for a killed server before reading its stderr.** `TestSilentServerFailsWithinDeadline` read the captured stderr as soon as the server's stdout closed, before `os/exec` had finished copying the goroutine dump the `SIGQUIT` produced, so the dump was sometimes empty and the test failed at random under load.

- **A PDF finds an `Image` block's file beside the document's source.** `DocumentQueries::Image.location` is relative to the document's file, and `-doc-form pdf` now resolves it there as `-doc-form html` does — an output directory other than the model's no longer fails with `missing-image` — falling back to the output's directory only for a document read from standard input. HTML and Markdown write the location relative to the output's directory so the page finds the file too. A location stated in one of several files loaded together resolves against that file, not the joined session buffer, so a `Picture` on a view drawn into a document finds its file.
- **A hidden diagram symbol shows nothing on the migrated view.** A symbol MagicDraw keeps but does not draw (`visible` false), with the symbols nested in it and the parts, regions or compartment rows it lists, no longer exposes its element on the view or lends it a position or style — a hidden compartment row stays off the view though the diagram's element list names it and its owner is drawn; and a symbol's own position and style go to the element it names, not to the last part or region listed under it. Only the image kinds the migration writes (PNG, JPEG, GIF, BMP, WebP, SVG) are recognized as pasted pictures; an icon is reported as no image.

- Migrated «InstanceTable»/TableStructure columns and sorts over MonteCarloAnalysis statistics are written as member-path columns and render their stored values instead of being omitted when an instance the table lists records them; otherwise the column is omitted with the note saying so.

- **Positioned and Cameo-style DOT renderings are faithful to the drawn diagram again.** A `DiagramLayout::Note` stated in a view's body is now drawn in that view alone instead of leaking into every enclosing rendering — a note a nested view states for itself no longer appears in the outer view's drawing, and a note anchored to a node the drawing omits is dropped with a notice rather than anchored to nothing, so `neato -n` no longer places note boxes below the canvas. A view exposing both an element and one of its members no longer draws the member twice (once as a root, once under its owner). Positioned and Cameo-style drawings head each node by the shortest suffix of its qualified name that still distinguishes it among the nodes the drawing declares, as the source diagrams name their scattered elements, and positioned drawings name `layout=neato` so `dot -n` keeps the pinned positions.

- **The positioned DOT tree no longer draws a containment edge to a child boxed inside its owner's box.** Such a child draws as a compartment row of the owner, so the edge painted a connector across the box on top of the row; children boxed outside the owner's box keep their edges, and an unpositioned tree is unchanged.
- **Quoted-name escapes decode in graphical labels.** A `\n` escape in an element's name printed literally in the DOT, Mermaid and PlantUML forms instead of breaking the label line; `\t`, `\'`, `\"` and `\\` decode the same way (the glyph-less control escapes are dropped), and the Cameo frame header decodes too.
- **A note whose stated box encloses a node now paints behind it.** Graphviz paints in file order, so a large note box — a Cameo text box used as a group frame — covered the nodes it framed; enclosing notes are written before the nodes, nested frames outermost first, and captioned at their top; notes enclosing nothing stay on top.

- **A view with `DiagramLayout::Layout` positions draws the same nodes in every graph form.** Only the DOT writer left the members no position placed undrawn; the Mermaid and PlantUML forms drew the whole exposed tree, so a migrated diagram exposing a package it never drew expanded into thousands of nodes and tripped the Mermaid workload bound. One placement decision now serves `dot`, `mermaid` and `plantuml`, through `-render`, `-render-all`, document figures, the LSP and gRPC alike: the placed members and the edges between them by default, every member under `-render-unplaced strip`, and each form accounts for what it left out in its own comment syntax (`%% not represented: …` in Mermaid, `' not represented: …` in PlantUML). The bound itself is unchanged.

- **Queries identify unnamed declarations by their export-compatible position in the gRPC service, the REPL's `%query` and `sysml -query`.** Their IDs match RDF and API-JSON export and can be used as a query scope; declarations without an unambiguous export identity remain omitted.

- **Recording an analysis run into a model with a transition parameter named after its type no longer fails.** Migrated SysML v1 state machines declare transitions such as `transition first a accept s3 : s3 do action { assign s32 := s3; } then b;`. Re-checking the model after the records joined it — or after any reindex — resolved the parameter's own typing from the transition's scope, found the parameter under its name and reported `cannot bind a value of type s3 to a feature typed by s3`, so `-record-run`/`%record` of a `Simulation::MonteCarlo` sample refused with the model left unchanged. A trigger parameter is no longer a candidate for the names its own header writes: its typing names the enclosing scope's definition on a cold resolver as on the document walk, and the runs are recorded.

- **A sequence mixing a quantity with a plain number records as text.** It has no single unit to record under, so it no longer settles to a bare Real that drops the measured element's unit.
- **A repeated sequence value is refused by an existing unique record member.** Recording into a definition that declares the member `[0..*]` without `nonunique` names the `into` remedy instead of generating a record that fails validation; generated definitions, declared `ordered nonunique`, keep admitting repeats.

- **The release job no longer mistakes the platform archives for the Python sdist.** The check that one sdist is staged beside the binaries matched `opensysml-*.tar.gz`, which the `opensysml-<platform>.tar.gz` archives staged in the same directory also satisfy, so the first release to ship the Python distribution from the core tag failed its own verification with "matches 5 files, not one" and published nothing. The check now matches the sdist by its version-led name (`opensysml-[0-9]*.tar.gz`).

- **`-render-documents` writes documents whose file names differ in letter case alone instead of stopping.** Two documents `Reports::Summary` and `Reports::SUMMARY` stopped the run with "render to file names that differ only by letter case"; each is now written under its name tagged with `~` and a hash, as `-render-all` writes such views, and the same planner escapes a stem Windows reads as a device and cuts a name too long for a path component. Cross-document links point at the tagged name where one was needed, in a set and in a single document rendered on its own — by `-render-document`, `%render-document`, the LSP's `opensysml/renderDocument` or the gRPC `RenderDocument`, which all plan the set's file names the same way. Documents sharing a short name in different packages were always written apart, by qualified name, and naming one to `-render-document` by the short name alone is refused with every candidate's qualified name.

- **`OPENSYSML_TOOL_ENV_PASSTHROUGH` refuses malformed entries.** An entry containing `=` or a NUL byte is not an environment variable name; the tool's invocation now fails with an error naming the variable and the entry instead of forwarding it.

- **A migrated DocGen view's own documentation opens its section.** The comment a document view carries as its documentation was written as the `doc` of the migrated `view` but never reached the `DocumentQueries::Section`, so a rendered document showed a heading with the view's prose missing wherever the text was the view's documentation rather than a collaborator paragraph or a method step. The section now opens with that documentation as a `Paragraph`, before the method's content and at every depth of the view tree, as DocGen prints it; a comment that is also shown by one of the view's collaborator paragraphs is written once, in its place, while a malformed collaborator application over it is refused without hiding the documentation.

- **Tool previews do not print passed-through environment values.** `-tool-dry-run` and `%tool` list variables taken from this process (`PATH`, `HOME`, `TMPDIR`, `LANG` and the `OPENSYSML_TOOL_ENV_PASSTHROUGH` names) as `NAME=<from this process>`; only the manifest's own `env` entries show their rendered values.

- **Parser and structural validation now follow the grammar for enumeration prefixes and result parameters.** Enumeration definitions reject definition prefixes, requirement-like bodies reject return parameters, and duplicate returns are reported across function and expression kinds.

### Performance

- **Occurrences of one shape share their derived defaults and their verdicts.** A `=` default
  that one pristine occurrence of a type derives from nothing but declared values under itself
  is now recorded against the occurrence's shape — its type, classifiers and holding feature — in
  a side table of the runtime context, and every other pristine occurrence of that shape reads
  the recorded value instead of deriving it again and materializing the component tree the
  derivation walked; an occurrence that states, writes, binds or classifies anything the
  derivation read derives on its own, and a write under an occurrence invalidates what it took.
  Within one `-satisfy` or `-validate=<object>` report, a check over occurrences of one shape is
  evaluated once per distinct set of inputs and its verdict fanned out to each occurrence, which
  still reports its own verdict, message and path in the same order. Values, verdicts and
  diagnostics are unchanged, as `TestSparseValuesDifferential` asserts with sharing on and off
  (`OPENSYSML_SHARED_DEFAULTS=0` turns it off; a context recording a trace shares nothing, so
  the trace lists every evaluation). On the 12 800-satellite fleet constellation,
  checking its 2 412 satisfaction assertions drops from 8.84 s and 23.4 GiB allocated to 4.59 s
  and 7.2 GiB, and reading one summed attribute over every occurrence from 42.7 s and 141.3 GiB
  to 3.85 s and 2.7 GiB.

- **A batch of files is parsed and analysed on a pool of workers.** `sysml -validate`, `-satisfy`, `-check` and `%load` open the files they are given as one batch: the files are parsed on one worker per CPU, indexed once, wildcard imports are expanded once for the batch instead of once per file, and the documents are analysed in parallel, each with a resolver and semantic model of its own over an index nothing writes meanwhile; the facts the workspace-wide audits (OOSEM, MOSA, identity metadata) need of every document are gathered once for the batch and shared by the workers. The diagnostics are the same at any job count, in command-line order; `-jobs N` or `OPENSYSML_JOBS`, the setting that already bounds how many runs of one check go concurrently, sets the pool. On an eight-CPU machine the 1 600-satellite stress constellation split over 34 files validates in 9.2 s against 22.0 s on one job and 20.3 s as a single file, within a tenth of the single file's peak memory; a 200-satellite split in 1.19 s against 2.63 s. `stress-model -split-planes <dir>` writes the constellation one file per orbital plane, and `docs/project/satellite-network-stress-test.md` records where the split model's remaining time goes.

## 0.9.0 — 2026-09-24

### Added

- **The analysis libraries' conformance is measured, per package, by a runtime test.** `TestAnalysisLibraryCensus` enumerates every public callable declaration of `SampledFunctions`, `TradeStudies`, `StateSpaceRepresentation`, `AnalysisTooling`, `VectorFunctions` and `OccurrenceFunctions`, invokes each through the runtime with a representative model, and records whether the value it produced passed its check, which typed error refused it, or what its value got wrong. The verdicts are committed to `docs/project/analysis-library-census.json`, which the test holds current, and `make docs-counts` renders them as the per-library table in `docs/project/spec-compliance.md`; `go run ./cmd/doc-counts -check` fails when the table and the file disagree. `StateSpaceRepresentation` is recorded as refused by name where its abstract dynamics need a state-space runner, not worked around.

- **Added `examples/analysis-results-demo/`, a worked example of saving analysis runs into the model and reporting them in a document.** The records are `part` usages on the bundled `AnalysisRecords` vocabulary — the same shape `-record-run`/`%record` emits, written by hand to keep pre-edit values — and a generated report groups, filters and lists them, flags the record a later model edit made stale through a derived `drift`/`stale` pair, and contrasts them with `Verdicts` recomputed live at render time.

- **`-convert` and `-from` accept `api-json`, the OMG SysML v2 API element form.** A model written as `api-json` is the JSON array of element objects the SysML v2 API serves — `"@type"`, `"@id"` and the metamodel properties as keys — over the same RDF graph the Turtle mapping builds, so `sysml`, `ttl` and `api-json` convert between each other losslessly; `.json` names the format by extension, and reading or writing it reports the RDF mapping's experimental status as Turtle does.

- **Expressions are written in the KerML abstract-syntax shape.** An `InvocationExpression`, `ConstructorExpression`, `OperatorExpression`, `FeatureChainExpression`, `IndexExpression`, `CollectExpression` or `SelectExpression` now owns one `ParameterMembership` → `Feature` → `FeatureValue` per operand or argument, a `ReturnParameterMembership` → `Feature` for its result, and — for an invocation — a `Membership` whose member is the function it calls (the pilot's serialization of `instantiatedType`); a chained callee is the `FeatureChainExpression` it names, and a body argument is the anonymous `Expression` owned through a `FeatureMembership`, as in the pilot XMI. The collapsed `function`, `operator`, `argument` and `sysx:sourceText` properties stay beside them; the reader accepts either alone and refuses a graph where the two disagree. Graphs earlier releases wrote — a `->` receiver named by `operand` alone, `new` as `sysx:isConstructor` on an `InvocationExpression`, a `return` parameter under a plain `FeatureMembership` — still read.
- **`-convert api-json` writes the pilot's root `Namespace`.** The element form opens with an unnamed `Namespace` (`<root>_ns`, a UUIDv5 under `-id uuid`) whose `OwningMembership`s own the document's top-level packages, as every `.kermlx` of the pilot does; the reader treats the wrapper as transparent, so notation → `api-json` → notation is byte-identical, and the Turtle form is unchanged.
- **Views are written with the SysML v2 view metaclasses.** `ViewDefinition`, `ViewUsage`, `ViewpointDefinition`, `ViewpointUsage`, `RenderingDefinition`, `RenderingUsage`, `Expose`/`NamespaceExpose`/`MembershipExpose`, `ViewRenderingMembership`, `ElementFilterMembership` and `FramedConcernMembership` replace the collapsed view properties, and read back to the `view … { expose …; filter …; render …; }` they came from.
- **Behavior nodes drop this project's extension metaclasses where the metamodel has a form.** `first start`/`then done` are memberships and successions to `Actions::Action::start`/`done`, `if` branches are `ParameterMembership`-owned parameters of the `IfActionUsage`, a state's `entry`/`do`/`exit` is a `StateSubactionMembership` with the normative `kind`, a transition's effect a `TransitionFeatureMembership` of kind `effect`, an alias a `Membership` with `memberName`, and a named multiplicity a `MultiplicityRange`. Graphs written with the older `sysx:InitialNode`, `sysx:FinalNode` and `sysx:IfBranch` still read. `sysx:Pseudostate`, `sysx:DeferMember` and `sysx:ActionExecutionNode` remain, since the metamodel has no element for the notation they carry; `docs/reference/rdf-mapping.md` says why.
- **`-convert sysml -from api-json` reads more of sysml-toolkit's interchange.** A `then` succession whose source is the member before it, a metadata usage that references `ModelingMetadata::Refinement`, the unnamed chain `Feature` (`FeatureChaining`s) an expression reaches or invokes, a membership an expression node owns, the toolkit's `StateSubactionMembership`s and `TransitionFeatureMembership`s, and a full-JSON implied relationship whose derived `relatedFeature`/`chainingFeature` list is ordered differently from its owned ends all decode — the owned structure is authoritative and the derived list is read from it, since a derived property cannot contradict what it is derived from.

- **`-convert api-json` and `-convert ttl` materialize the relationship elements the notation implies.** Typings, specializations, subsettings, redefinitions, multiplicity ranges, conjugated-port definitions, subject/constraint/result/filter memberships and referent memberships are now written as the first-class elements the SysML v2 metamodel defines — `FeatureTyping` (`<S>_ft0`), `Subclassification`, `Subsetting`, `ReferenceSubsetting`, `Redefinition`, `MultiplicityRange` (`<S>_mult`), `ConjugatedPortDefinition`/`PortConjugation` (`<S>_conjugated`, `<S>_pc`), `SubjectMembership`, `RequirementConstraintMembership`, `ResultExpressionMembership`, `ElementFilterMembership` and the `Membership` an expression's `referent`/`targetFeature` is carried by — beside the collapsed properties that already stated them, so element-count comparisons against sysml-toolkit's interchange JSON match and the toolkit's lifter reads the output back to notation. A declared relationship member is owned through an `OwningMembership` like any other member.
- **`-convert sysml` reads sysml-toolkit interchange JSON.** Both `convert --to compact-json` and `convert --to full-json` decode through `ReadAPIJSON`, to byte-identical notation: the toolkit's root `Namespace`+`OwningMembership` wrapper is transparent, `{"@ref": <name>}` and `unresolved:`-derived targets read as the name they spell, stated defaults collapse back off `isImpliedIncluded` elements, and ends the notation cannot place are refused rather than guessed.
- **`-convert` takes `-id uuid`.** Passing `-id uuid` to `-convert ttl` or `-convert api-json` mints name-based uuids the way the SysML v2 library convention does — `uuid5(NamespaceURL, elementIRI(root))` for a root package, `uuid5(pkg, <the qualified-form id>)` for every derived subject under it — while declared and normative ids are never re-derived. Decoding a uuid-form document reproduces the same notation with the ids implied, and only a non-matching `elementId` stays a declared `@ElementId` annotation.

- **A design note on surface parity** (`docs/internals/design/api-surface-parity.md`). It
  inventories what the REPL, the CLI, the editor, the public Go package and the wire each expose,
  sorts every difference as shared already, missing and worth adding, interactive, protocol-bound
  or local-only, and stages the work: one assembly per operation that all four surfaces call; the
  stateless operations the wire lacks (satisfiability, model-checker options, inline replay, view
  rendering, library search, codegen and the XMI migration report through `Convert`); a session
  API for the action and state debuggers designed apart from the stateless calls; and a `repl`
  protocol in the conformance runner so the agreement is tested rather than claimed. The binaries
  keep calling the engine in-process; nothing is implemented, the note exists to be reviewed
  before code is written.

- **Binding and non-message flow usages now materialize as connector objects.** Their ends hold the connected feature values through the same connector path used by `connect` usages; message flows and one-ended bindings remain excluded.

- **The checker warns (`undefined-operator`) on every use of the unary `~` operator.** KerML 1.0 §8.2.5.8.1 leaves `~` abstract and undefined, asking a tool for exactly this warning; the runtime keeps refusing it with a typed error, and the design record `docs/project/bitwise-complement.md` explains why no value is given.

- **A design note for a Cameo Systems Modeler plugin** (`docs/internals/design/cameo-plugin.md`).
  It answers, from the vendor's public documentation and Javadoc, how a plugin for Cameo 2026x
  Refresh1 (bundled JDK 21; 2024x Refresh3 on JDK 17 as the minimum) is declared, loaded and
  distributed; where it contributes browser and
  diagram actions, a docking results panel and progress with cancel; how a selection leaves the
  tool (a saved `.mdzip` today, since no dialog-free OMG XMI 2.5 export was found in the OpenAPI);
  how a Cameo `xmi:id` maps to the SysML v2 name the migration writes through the per-element
  migration report, and what the service must return for that; how verdicts land on elements
  through annotations or a custom table; what the Simulation Toolkit covers and what OpenSysML
  adds; and how the release's own SysML v2 project type, textual import/export and v1-to-v2
  transformation change what OpenSysML parses (none of these was found for 2024x Refresh3). A benchmark
  converts twenty public SysML v1 models — the repository's XMI fixtures, Cameo `.mdzip` projects
  and Papyrus models — and tabulates mapped, approximated, unmapped and skipped elements. It closes
  with the proposed `editors/cameo/` layout and build, the *Run with OpenSysML* sequence, a phased
  plan and the risks, every claim that could not be verified marked as such. Nothing is
  implemented; `editors/README.md` now lists the entry.

- **A Cameo Systems Modeler plugin runs models on OpenSysML** (`editors/cameo/`). Right-clicking
  an element in the containment tree or on a diagram offers an *OpenSysML* group with Instantiate,
  Execute action, Execute state machine, Verify requirement/constraint, Evaluate calc and Run
  analysis. A SysML v2 project (Cameo 2026x) is exported through the textual notation service; a
  SysML v1 project is saved as a `.mdzip` and migrated through `Convert`; either is parsed and run
  on `sysml-grpc` through the Java client, off the event thread with progress and cancel. Outcomes,
  diagnostics, final time and the state schedule appear in a docking *OpenSysML Results* window
  (double-click selects the element in the browser) and as validation annotations on the elements,
  matched by qualified name on both paths. The module compiles against compile-only stubs of the
  OpenAPI so CI needs no licence; `CAMEO_HOME` compiles it against a real installation, and the
  `dist` build stages the digest-pinned service binaries for every platform into a Resource
  Manager zip.

- **A persistent session in the public Go API.** `opensysml.OpenSession` opens a `Session` over
  a model a `New` client parsed: an interactive run that keeps its clock, its scheduling policy
  and the objects it instantiated between calls, where `ExecuteAction` and `ExecuteState` run a
  whole behaviour and return. `SetSchedule` governs the turns from then on, the machines and
  clock already running included (`runtime.Context.Reschedule`), where they stand kept.
  `Instantiate` makes an object and starts the state machines it
  exhibits; `ActiveStates` and `Transitions` say where each machine stands and what could fire
  next, by name; `Accepts` says whether a signal would be taken, read from the machines dispatch would let take it
  — one whose guards all fail yields it to a sibling that would fire on or defer it — whether a
  transition is triggered by it and whether a guard holds now;
  `Send` posts it and `Advance` dispatches it, completion transitions included; `Perform` runs an
  action on the object and reports its outputs, the `ChoicePoint`s the schedule resolved and the
  `Branch` each decision left by, with `TurnedAway()` for an action that declined at its opening
  decision; `Feature`, `SetFeature`, `Evaluate` and `Members` read and write the state the runs
  left. Every answer is a fact copied out of the engine, never one of its graphs or objects, and
  misuse — a closed session, a signal no transition accepts, an unknown action, an exploration
  policy — is a typed refusal. The session is opened from a `Client` but is not part of the
  `Client` interface: a `Dial` client refuses it with `CodeUnimplemented`, because the service
  exposes no RPC for state held between calls, and the parity contract on `Client` is untouched.
  The [Legend of the Red Dragon browser game](https://github.com/Open-MBEE/SysML-LoRD), a
  program on this surface compiled to WebAssembly, is its first client.

- **The SysML v1 migrator writes a composite state's entry and exit points.** A UML `State.connectionPoint` pseudostate, on a nested composite state or on one with orthogonal regions, is a `junction` of the state, reached by path (`then Work::start;`, `first Work::leave then Idle;`), so the runtime runs the state's entry behavior before the entry point's outgoing transition and the transition into the exit point before the state's exit behavior, the order UML and PSSM give connection points. An entry point whose transitions each start a region of an orthogonal state is a `fork`, an exit point its regions reach from each side a `join`, and an entry point no transition leaves is the state's default entry. An entry point leading straight to an exit point of the same state, a route from a connection point on into a history pseudostate, and the other shapes with no faithful form are refused with the shape named in the report; before, every connection point on a state and every transition through it was unmapped. The OMG PSSM test suite's connection points, which the report refused wholesale, migrate under this rule, and the migrated suite validates with no syntax errors.

- **Diagram layout is written into the document that declares what holds it, across the workspace.** `opensysml/applyModelEdit`'s `setLayout`, `setRoute` and `setCanvas` resolve their target through the workspace index rather than the requesting document alone: a view-local `Layout` or `Route` and a `Canvas` go into the view's body in the view's document, an inline `Layout` or `Route` into the element's body in the element's document, whichever document the panel renders. The answer is one `WorkspaceEdit` with a versioned `TextDocumentEdit` per document changed, validated together as rename and delete are; a bundled library file, or a document the index holds without its source, is refused as `referenced-elsewhere` naming the file. A rendering's node or edge that another document declares carries its `fqn`, `owners` or `declaration` as any does, with `origin.uri` naming the document, and a `setLayout` or `setRoute` by declaration takes `declaredIn` to say which document the range is one of. The VS Code diagram panel drags a node another file declares: the annotation lands in the right file, the edit is applied only while every file it names is at the version it was computed against, and one <kbd>Ctrl</kbd>+<kbd>Z</kbd> reverts every file. Server and extension each advertise the contract as `openSysmlCrossDocumentLayout`, and each treats the other's lack of it as the single-document contract before, so an older extension is handed no other file's name to place unpinned and an older server's nodes keep every editing action.

- **`RelatedElements` traverses requirement derivation and refinement.** The relationship kinds `"derivation"` and `"refinement"` join the eight the query engine already walked, in both directions and chained to `maxDepth` like the others. A `derivation` edge runs from an original requirement to each requirement derived from it, read from every notation of a `RequirementDerivation::Derivation`: a connection usage typed by it or written `#derivation`, whose ends state their roles by subsetting `originalRequirements`/`derivedRequirements`, by the `#original`/`#derive` metadata, or by the ends of the definition typing them, an end stating no role taking the one left over (the first such end is the original unless another end is, the rest are derived); and a `connection def` specializing `Derivation` whose ends are typed by requirement definitions, as the v1 migrator writes, which relates those definitions. A `refinement` edge runs from each client to each supplier of a `dependency` annotated `@ModelingMetadata::Refinement` (prefix or body form); a plain connection or dependency states neither kind. The query cookbook gains "Derive relationships" and "Refine relationships" recipes over an extended `cookbook.sysml`.

- **The VS Code diagram is open by default.** A `.sysml` or `.kerml` file shown in an editor gets its diagram beside it without being asked, focus staying in the text; the diagrams of all files share one editor group, so switching files adds a tab there rather than a column. A diagram the user closes stays closed for that file — across editor switches and window reloads — until `SysML: Open Diagram` asks for it again. `opensysml.diagram.autoOpen` (default `true`) turns the automatic opening off.

- **The VS Code diagram moves a declaration when a node is dropped on another with
  <kbd>Shift</kbd> held.** While <kbd>Shift</kbd> is down, the node under the dragged one is
  outlined when its body admits the dragged declaration — the same admission the node menu's
  **Move to…** applies — and the status line says what releasing does; a node that cannot hold
  it is not outlined and releasing there puts the node back with the reason. Releasing writes
  the declaration's new position and its move as one `applyModelEdit` request, so one
  <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes both, and the diagram redraws it under its new owner where
  it was dropped; a move the server refuses — a name already taken, a declaration another file
  refers to — is shown in the status line and the node goes back. A drag without
  <kbd>Shift</kbd>, or released over empty canvas, writes a `Layout` as before.

- **The VS Code diagram panel opens a document that declares several views intelligently, and can show several of them at once.** `SysML: Open Diagram` picks the view whose declaration holds the editor's cursor, else the one last chosen for that document in the workspace, else asks — a quick pick of the drawable views by name and kind, an **All views** entry that opens each in its own panel, and the pseudo-views last; a view the server cannot draw is left out of the list (the panel's own picker still shows it, disabled, with the reason), and cancelling opens nothing. A document may have one panel per view, titled `Diagram: <file> — <view>` while there are several; each redraws on change, highlights the cursor's node, and comes back on its view after a reload. Opening a view a panel already shows reveals it; a panel's picker retargets that panel unless another already draws the view, which is revealed instead. Documents declaring zero or one drawable view open as before, without a prompt. A diagram that opens on its own follows the same choice up to the quick pick and opens nothing rather than ask; closing a document's last panel is what keeps it closed. `opensysml/views` now carries each declared view's optional `range` and `selectionRange`, which the client uses for the cursor step and older servers may omit.

- **The VS Code diagram panel draws in the pilot visualizer's style, and in colour, on request.** A **Style** list in the panel's toolbar, backed by the `opensysml.diagram.style` setting, picks the look of every diagram: `theme` (the default) follows the VS Code colour theme as before; `pilot` is the pilot's Standard B&W that the DOT and PlantUML forms already follow — white canvas, black sans-serif text, thin dark borders, square definitions and rounded usages, heavier packages, dashed regions, bold names over an italic keyword, thick arrowless connections, filled pseudo-states; and each of the eight colourblind-safe palettes (`okabe-ito`, `tol-bright`, `tol-muted`, `tol-light`, `brewer-set2`, `brewer-dark2`, `viridis`, `cividis`) is that look filled by keyword family, a usage a lighter tint of its definition's colour, text black. Changing the list keeps the choice in the settings and redraws every open diagram. The colours come from the server: `opensysml/render` with a `palette` now gives each node its `fill` and `border`, the same hex the DOT and PlantUML forms of that view take, advertised as `openSysmlRenderPalette`; against an older `sysml-lsp` a palette draws as `pilot` and the panel says why.

- **A due `do` step against the dispatch due at the same instant is a recorded choice point.** A state machine ran every due `do` behavior one step before it dispatched the occurrence at the head of its pool, with no choice recorded for `explore` to vary. Under `check`, `replay` and `explore` the machine now runs one unit at a time — one step of one due `do` action of the round under way, or the dispatch — and the draw is a choice point (`choice at t=<instant>: next do <state>, dispatch <event> (unordered; took do <state> first)`; `dispatch change <condition>` for a change trigger risen; a message in flight is delivered behind the events already queued, so it is drawn as `dispatch accept <signal>` once nothing is ahead of it and hidden by a queued event nothing accepts until the round closes) written to the trace and the witness, replayed, refused and rolled back with its move, and enumerated by `explore` and `check`, whose moves offer the due `do` steps and the dispatch together. Only a dispatch that would take its occurrence — fire a transition, or let a `do` behavior parked at an `accept` go on — is drawn; one that would defer or drop it waits for the round to close, and events tied at the head are judged one by one, so a trigger the dispatch would drop hides no tied trigger it would fire. The fixed policies (`declared`, `reverse`, `seed:<n>`) finish the round before they dispatch, so no event order under a fixed policy moved and no existing trace golden changed. The grain is the token move: a due `do` behavior — an inline body, a behavior given as an action, a loop, a nested perform — advances one token, and the draw is made again after every move while a `do` behavior is due, so a dispatch may cut the flow anywhere or wait for it to rest, a body parked at an `accept` offers no move until its occurrence is dispatched, and the fixed policies' whole round then the dispatch is one path of the enumeration — `check` reports *exhaustive* where it has every path and no longer names a run left out. A control node with a body of its own (`fork split { assign x := 1; }`) performs it as the token passes, so it is a move the dispatch is drawn on either side of; only a bodiless control node over unguarded successions routes silently. Seven conformance cases state the admissible sets (`state_do_step_or_dispatch`, `state_do_step_among_completions`, `state_do_step_or_tied_dispatch`, `state_do_step_cuts_typed_do`, `state_do_step_cuts_nested_perform`, `state_do_step_cuts_control_node_body`, `state_do_action_loop_timed_exit`, the last two searched to completion over their five and four outcomes), and the runtime showcase's spacecraft, checked as vehicle and ground station together, reaches the fixed policies' `battery = 39` beside the cut sweeps' `41`. The PSSM referee moves *Behavior 003 A* to `pass`, reaches the do step's admitted places in *Terminate 002* and *Transition 017*, and moves *Exiting 002* to `fail` on a trace the suite registers for the same race one test earlier and not for this one, recorded as the suite's in `docs/project/omg-issues.md` (51 pass, 13 fail, 38 not expressible, 1 differs by design).

- **A DocGen «Image» over an activity or state machine diagram is drawn in the migrated document.** A `Diagram` block whose `source` is a migrated `StandardViewDefinitions::ActionFlowView` or `StateTransitionView` is written and rendered as the graph the view draws, positioned by its MTIP layout when `-layout` was given; drawability is decided by the same rule that chose the view's form, so only a diagram whose view renders as textual notation — a sequence diagram, whose Interaction is written as a scenario rather than as the occurrence parts a `SequenceView` draws — is refused, with that reason.
- **The migrator reads what an `.mdzip` diagram draws, and leaves an empty figure out.** Every diagram is read from the archive entry its `binaryObject` names, MagicDraw's serialization of the diagram's symbols: the elements the symbols stand for are shown and exposed, with the elements of the tool's used-element list a symbol displays without one of their own, by standing for an element they are owned under; a listed element no symbol displays is dropped, and symbols standing for none — a pasted image, a text box, a note — are counted as free content, so a stream whose symbols name no element shows nothing, whatever the list names. An «Image» over a diagram that shows nothing writes no `Diagram`, so the rendered document holds no empty figure; the step is reported with what the diagram draws and its caption stays as a paragraph, as DocGen shows it.
- **DocGen collectors and filters gain their query spellings.** A chain follows the source elements it collects, so `CollectOwnedElements` gathers the diagrams an exposed package or block owns, as DocGen finds its figures; `FilterByDiagramType` keeps the diagrams of the presentation types named; `CollectThingsOnDiagram` names the elements the collected diagrams show, and is refused when any of them names a stream the archive does not hold or holds unreadable, since what it shows beyond the tool's list is then unknown; a fork's branches carry on or end the doubt a step before it leaves, so the rejoined step knows what it draws when every branch names its own targets; `CollectByAssociation` names the types reached through attributes of the aggregation kind to the depth asked; a sort by name or documentation orders the diagrams, and the elements they are collected from, as `OrderBy` orders the rows; `FilterByNames` reads the v1 name as DocGen does, naming the elements it keeps where the v2 name an anonymous element gains would match otherwise, and a step with no query spelling refuses the «Image» after it as it does every other block. A diagram's own comment, its documentation in the tool, is written as its view's `doc`. A requirement's `Id` and `Text` columns are its `shortName` and `documentation`. An «Image» whose chain holds no diagram, and a step over a stereotype with neither a v2 metaclass nor a `metadata def`, a `MonteCarloAnalysis` statistic column, or a chain whose elements are known only when the query runs, are reported with the reason rather than refused blindly.
- **A `uml:Expression` tree that spells nothing is skipped as notation.** A constraint whose specification has no symbol at any node and, as leaves, only `InstanceValue`s naming no instance (Cameo Collaborator's presentation constraints on a «Document») carries nothing to translate and is skipped as notation-only wherever it stands; a tree with a symbol or a leaf naming an instance is translated or refused as before.

- **Three convention themes for `-html-theme`: `nasa`, `ieee` and `acm`.** Each follows a published manuscript convention — the NASA STI Report Series (Times 12pt body, Arial headings, tables and captions, letter page with one-inch margins, roman-numbered front matter), IEEE Transactions (Times 10pt body, 8pt captions and tables, centred small-caps section heads, italic subheads, justified with a one-pica indent, letter page with 0.67in margins) and ACM's `acmart` (Libertine 10pt body falling back to Times, bold sans numbered heads, 9pt captions, letter page) — black on white, with thin horizontal table rules, and sets the same faces and point sizes on screen as on paper so a page and its PDF agree. IEEE and ACM output is single-column; the sources, the values verified against them and the choices made where a convention is silent are recorded in `docs/project/html-document-backend.md`.
- **A theme now governs the PDF page.** A bundled theme may carry a print companion, `themes/<name>.print.css`, which the PDF backend lays over its print stylesheet in a third cascade layer, `opensysml-print-theme`, so the theme's page size and margins, faces, body size, heading scale, caption and table sizes and page-number footer reach paper instead of being overwritten by the print sheet's defaults; the `print` and `report` themes carry one too, so `report`'s Charter/Georgia stack and larger body now print. The order is the default sheet and theme, the print sheet, the theme's companion, then `-html-css` sheets unlayered; `-html-no-default-css` leaves every bundled sheet out, and the pandoc engine still refuses `-html-theme`.

- **Document queries read the objects a session holds.** A `%run-query`/`-run-query` parameter
  written as a usage's name binds the object the session holds under it while it holds one
  (`car` after `%instantiate car`), `#2` binds an object by id and `car.wheels[2]` a nested one by
  path, and the element as before when nothing is held. Every query operation that takes an
  element takes an object and reads what it holds: `OwnedElements` and `Descendants` are the
  objects it holds as parts, each element of a collection under its own path (`wheels[1]`,
  `wheels[2]`), `Ancestors` the objects holding it, `WhereType` tests its types, `WhereName` its
  path, and `WhereFeature`, `Project`, `OrderBy` and `Column` read the values it holds now — after
  a run changed them, not the declared defaults. The new `DocumentQueries::Objects(type = T)`
  enumerates every object the session holds that is of the type; outside a session it is refused
  with an error saying to instantiate an object first. `WhereMetadata` tests the usage an object
  stands for; `RelatedElements` reads the model's relationships and refuses an object row. A document renders over objects too: `-instantiate <name>`
  is now accepted beside `-render-document`/`-render-documents` and creates the objects first, and
  a document parameter bound to a usage's name binds the object held under it. Objects render by
  path in Markdown and PDF; in HTML each carries `data-object="#<id>"` beside the `data-element`
  of the usage it stands for and is a `span.sysml-object`.

- **Document queries report which constraints and requirements hold.** The new
  `DocumentQueries::Verdicts(source, kind = "all")` checks the object behind each row as a whole —
  the object the session holds when the binding is one (`%instantiate car`, `-instantiate`), the
  element's declared object otherwise — and answers one **verdict row** per assertion about it and
  the objects it holds: every `assert constraint`, every requirement carried, every `satisfy` whose
  subject it is, and each verification case verifying such a requirement, a collection's members
  under their own paths (`car.wheels[2]`). A verdict row stands for the assertion (so `name`,
  `WhereName` and `WhereType` read the constraint or requirement) and adds `kind`, `carrier`,
  `path`, `verdict` (`holds`, `violated`, `undecided`), `condition`, `reason` and `verification`,
  which `Project`, `WhereFeature`, `OrderBy` and `Column` read; `kind = "constraint"`
  (`requirement`, `satisfaction`, `verification`) keeps one kind. A row that is no object, and a
  walk the runtime could not complete, are typed errors rather than a table missing rows. Verdicts
  print in `%run-query`/`-run-query` as `<assertion> on <path>: <verdict>`, render in Markdown and
  PDF as that text and in HTML as a `span.sysml-verdict` (`data-verdict`, `data-path`,
  `data-object`), and `RunDocumentQuery` answers them as the new `verdict` arm of `DocumentValue`
  (`DocumentVerdict`), decoded by the Go and Python clients as `DocumentVerdict`; a verdict bound
  as a parameter is refused.

- **PDF output draws Graphviz DOT and PlantUML diagrams.** `-render-document -doc-form pdf -diagram-form dot` runs each diagram block through Graphviz — the `dot` named by `OPENSYSML_DOT`, else the one on `PATH` — as SVG under the layout engine the block's `// layout:` header names (`dot`, `neato`, `neato -n`), so a view the model positions with `DiagramLayout` is drawn where it was placed; `-diagram-form plantuml` runs each block through the PlantUML jar named by `OPENSYSML_PLANTUML_JAR` (`java -jar <jar> -tsvg -pipe`, the `java` from `OPENSYSML_JAVA` or `PATH`). Both tools are optional: without one the block stays in the PDF as source under a notice naming the variable to set, and the render succeeds; a tool that fails is the same typed `tool-failed` error, with its stderr, that a failing Mermaid CLI produces. `scripts/download-doc-pdf-toolchain.sh` provisions a pinned Graphviz and PlantUML jar beside WeasyPrint, Mermaid CLI and KaTeX, and CI's PDF job draws through all of them.
- **The VS Code diagram panel exports every form the server writes.** `SysML: Export Diagram` now asks which form to save — Mermaid (`.mmd`), Graphviz DOT (`.dot`), PlantUML (`.puml`), Markdown (`.md`) or text (`.txt`) — from the list the server advertises under the new `openSysmlRenderForms` capability of `initialize`, sends the pick as the `form` of `opensysml/render`, and opens the save dialog on that form's extension and filter; before, it saved whatever the server defaulted to.

- **A run creates and destroys objects.** `new T(args)` (KerML §7.4.9 instantiation expression) in any expression position — an assignment, a feature value, an argument, `send new Data(…)` — makes a first-class occurrence of the run with an identity of its own, classified by its type, its exhibited and performed behaviors started, so a loop creates one object per iteration and a context holds several objects of one usage. Writing an object into a feature holds it: the feature's type classifies it and a composite feature adopts an ownerless object as a portion of the owner, so `assign cars := (cars, new Car(n))` leaves the fleet owning each car (a write that would make a whole a portion of itself, or give an ended whole a portion live or ended after it, is refused as `ErrOccurrenceLifetime`; an object its home drops moves home to another composite feature still holding it), reached by `all Car`, feature chains and `%features`; the behaviors the feature's type starts on the object run once the feature holds it, so one reading the feature sees the object it started for, and the behaviors a constructor's arguments start run once every argument is stored; a write or constructor rolled back because a behavior it started failed leaves no trace records of that behavior's run. `destroy` ends the occurrence and its portions, terminates the state machine it exhibits and the actions it performs where they stand (it no longer refuses with `ErrOccurrenceLifetime`), releases it from `all T` and drops the messages addressed to it or routed to a port it ends; a feature still naming it keeps the value and reading through it is `ErrOccurrenceDestroyed`, and a `=` value that read the object, `isDuring` of it or `all T` is derived again rather than answering what it derived before, in a context a held image is materialized into as well, where a value derived from `all T` before the image arrived counts the imaged objects too. Creation and destruction are deterministic under `explore` and survive a snapshot.

- **A state's exit behavior reads the data of the transition leaving it.** An `exit action` parameter bound to a transition's accepted payload by the transition's name — `in level : Integer = warn.w ?? alarm.a;` for `transition warn first idle accept w : Warning …`, `in p : Boolean = 'T1.1.2'.p1` for a call trigger's argument — binds when that transition fires, before its effect runs, since the exit is a step of the transition performance that accepted the occurrence (`StatePerformances.kerml`: `accept then transitionLinkSource.exit`). A transition not being taken reads as nothing, so `??` chooses among several leaving transitions; an outer transition's data reaches the exits of the substates it leaves; a completion or data-less transition binds nothing and the parameter keeps its default; a payload of the wrong type, or a read of a transition not taken with no fallback, refuses the firing with a typed error and leaves the state as it was. The lowered transition carries the names its trigger binds (`Transition.Accepted`), and the executor holds the transition being taken from the exit through the entry; a do behavior reads the transition that entered its state for its whole run, whichever draw the entry front makes. The PSSM referee spells this for an exit behavior with parameters, so *Event 017 B*, *Event 019 B* and *Event 019 C* run and pass on exactly their admitted traces (60 pass, 30 not expressible); its reader also leaves out an activity node whose required input pin nothing ever feeds, as UML never executes it, and an exit some leaving paths bind nothing on declares its inputs `[0..1]` and guards only the statements that need them with `if notEmpty(…)`, so the exit still runs on those paths.

- **An exploration runs a behavior on an object nested inside an assembly, named by a path from a declaration.** `-schedule explore`, `-engine check`, `smt`, `sweep` and `-engine all` take a performer, subject or object written as `<declaration>.<usage>[.<usage>…]`, with `[i]` on a multi-valued usage — `-state "Comms::Ground::listen Comms::pair.ground"`, `-analysis "Dyn::Analysis Fleet::fleet.rovers[2]"`. Each run instantiates the declaration the path starts from, its parts and connectors with it, and walks the rest of the path inside that object as `%state` walks it in the session's. The declaration is instantiated once per run however many behaviors name paths under it, so two machines on sibling parts of one `pair` share it and the messages its connector carries between them are what the exploration tables, where naming a part's definition alone ran it deaf to its neighbours. The path is checked against the declarations before any run starts — an unknown usage, an index on a single-valued usage, a step through a value are refused by name — and what only a run can know (a part its recipe left unbuilt) is that run's error, an outcome of the table. A witness the checker writes for a machine on a nested object replays on it.
- **`-instantiate` gives its object to every explored run.** Under `-schedule explore`, `-engine check`, `smt` or `all`, each run creates an object of the `-instantiate`d declaration of its own before its behaviors start: a `-state` or `-action` named alone attaches to the performance the run's one object exhibiting or performing it already runs (several such objects are refused by name), and a path under the declaration (`Comms::pair.ground`) walks into the same object rather than creating another; a declaration `-instantiate`d twice is two objects of every run, as it is two of the session's, the later the one its name denotes. The prompt's `%instantiate` still creates the session's object alone, which no run sees. The `-instantiate`, `-state` and `-action` help and the CLI reference describe the rule under *Objects an exploration runs on*.
- **A service request names the performer of an action or state machine.** `ExecuteActionRequest` and `ExecuteStateRequest` carry `performer_symbol_id`, a declaration or a declaration-rooted path spelled as the CLI spells it, and `subject_symbol_id` of an analysis request takes the same paths; a service that honours it advertises the `performer` capability. The Go client has `opensysml.PerformedBy(path)`, the Python client `performer=` on `execute_action`, `explore_action`, `execute_state` and `explore_state`, and the Node, Java and Rust clients read the capability.

- **A migrated view exposes the edges its Cameo diagram draws.** A control flow, object flow, transition, connector, binding, dependency, satisfy or verify some diagram shows is written as a named member — its v1 name, else a name spelled from its ends, `succession 'start to call' first start then call;`, `transition 'Wait accept Done then Retrieve' first Wait accept Done then Retrieve;`, `binding 'a.p = b.q' bind a.p = b.q;` — unique in its body by the migrator's usual suffixes, and the view `expose`s it. An edge no diagram shows is written anonymously as before, so naming changes nothing in a model without diagrams, and the names never depend on an MTIP `-layout` export. An `include` is exposable too.
- **Activity and state machine diagrams migrate to typed views.** An activity diagram owned by its activity becomes a `StandardViewDefinitions::ActionFlowView` and a state machine diagram owned by its machine or a composite state a `StandardViewDefinitions::StateTransitionView`, each exposing the behavior whose graph it draws, so the renderer draws the successions, flows and transitions and pins their routes.
- **MTIP routes join the named edges, and the rest are itemized.** `-layout` writes a `DiagramLayout::Route` for every connector whose member the view exposes (or whose graph it draws) and whose rendering draws that edge kind. The report and the `results` sidecar (`routesByKind`) count the routes by v1 kind and reason — `written`, `no v2 member` (a generalization, composition, association), `not drawn` (a dependency or satisfy on a tree view, a message step), `not written`, `unnamed`, `not exposed`, `duplicate`, `dangling` — so an unpinned route says why.
- **The interconnection rendering draws bindings.** A `binding` between two features is an edge of its own kind, an undirected line beside the heavy connection in the text, Mermaid, DOT and PlantUML forms, and its route is pinned like a connection's.

- **Feature-owned multiplicities now report invalid featuring types.** A `featuring` relationship that gives a feature's owned multiplicity a featuring type outside the feature's featuring contexts is diagnosed.

- **`-convert` reads and pushes a Flexo MMS project branch.** `sysml <branch-url> -convert sysml` (or `ttl`) reads a branch as its head commit's RDF graph — the URL is `http(s)://host[:port][/base]/projects/{project}/branches/{branch}` or `flexo://{project}/{branch}`, both naming the endpoint `FLEXO_SYSMLV2_URL` configures — and `sysml model.sysml -convert ttl -o <branch-url>` replaces the branch's whole model graph, conditional on the branch's etag so a head the sync state says moved is refused with nothing written. Both sides need the bearer token in `FLEXO_INTEROP_TOKEN` and record the head commit in the sync state (`-sync-state`, `<output>.sync.json` on a read, `<model>.sync.json` on a push).

- **The SysML v1 migrator maps calls to the fUML and Alf standard-library primitives to the SysML v2 library.** A `CallBehaviorAction` whose behavior is an element of `fUML_Library.xmi` or `Alf-Library.xmi` — known by the library document its href names and the fragment within it, whatever date the URI carries and whether or not the model bundles the library; or, as MagicDraw and Cameo reference the library, by an href into the used project `fUML-Library.mdzip` or `Alf-Library.mdzip` whose target — the bundled copy, or the `referentPath` recorded beside the href — sits under the library's own root package, family and name, so a package of the model's own named `fUML_Library` stays its own — is written with its pins, each result pin valued by the v2 library expression over the arguments (`StringFunctions::'+'` for `Concat`, `IntegerFunctions::ToString`, `SequenceFunctions::including`, `SequenceFunctions::size`, …), so the flows out of it carry the computed value and a migrated activity that builds a string or a list runs. Every behavior of both documents is inventoried in the reference: mapped, approximated with the semantic difference in the note (an index outside the sequence fails in v2 where v1 gives no result; `Real` `ToString` writes the shortest text; `ToBoolean` reads lower-case text only), or refused with the reason (`IndexOf`, `ReplacingOne`, the `BitStringFunctions`, `WriteLine`). A scalar parameter the call passes nothing for, or whose pin only flows from something that produces no value, starves the call as in v1, so the action is written empty and carries the token. The report notes which provenance identified each call. Such calls were refused as behaviors with no v2 declaration.

- **The fUML referee's emitter translates classes, objects, signals and active classes.** A fUML `Class` with attributes and generalizations becomes a `part def`, `CreateObjectAction` a `new` occurrence on its result pin, and the structural feature actions (`Read`, `Add`, `Remove`, `Clear`) feature reads and assignments on the object at the `object` pin, positioned as the reference implementation positions them; a `Signal` becomes an `attribute def` specializing its generals, `SendSignalAction` a `send new <Signal>(…) to target` and `AcceptEventAction` an `accept` node whose result pin is the instance received, an instance of a specialized signal satisfying an accept of its general. A class's owned behavior becomes an `action def` nested in its `part def`, its classifier behavior an `action classifierBehavior : <Behavior>;` member no creation starts, `ReadSelfAction` in it `this`, an activity instantiated as an object such a `part def` around its own body, and `StartObjectBehaviorAction` a `perform object.classifierBehavior.start;`; an owned behavior's row is refereed through the executed activities that start an object of its owner, directly or through a call. The referee gives class-typed inputs defaulted objects and compares object outputs by class and feature values; eight activities of the reference suite move from `not-expressible` to `pass` (23 pass, 0 fail, 28 not-expressible, 4 differs-by-design). An edge weight other than 1 and an object-flow cycle through control nodes stay typed translation refusals; no activity of the suite has either.
- **An accepted signal flows on from the accept node's result pin.** The action-graph lowering records an `accept <name> : <Signal>` node's payload as an output feature, and the executor binds the value received on the accept's own performance as well as in the enclosing body, so `flow receiver.msg to consumer.value` carries it to the consumer and `value.level` reads its attributes.
- **A declared behavior starts on an explicit `perform obj.beh.start;`.** An action or state usage a `part def` declares without exhibiting or performing it (`action count : Count;`) is bound to no object at creation — `new T()` and a materialization run nothing of it — and a `perform` naming its `start` gives the object its own execution of it: `this` in the body is the object, its writes land on the object's features, a message sent afterwards wakes an accept it parks at, and the object outlives the behavior's completion. A second start of a running behavior starts nothing more, a start on no one object or of a member that is no behavior of the object is a typed error, and a start that fails is undone whole. The `start` is the shot of the behavior named, told from a feature the type declares under that name: `perform vehicle.start;` where `Vehicle` declares an `action start : Launch;` performs that action.

- **The fUML test models are read and every activity is classified before any is translated.**
  `internal/fuml` reads the pinned Eclipse UML2 XMI — activities, nodes, pins, control and object
  flows with guards and weights, parameters, classes, operations, signals, associations,
  structured nodes, exception handlers and cross-references into the foundational library —
  through the XMI element walker shared with the PSSM referee, which now accepts every OMG XMI
  namespace version. `Classify` files each of the 43 test-model activities and 12
  exception-model activities as expressible, `differs-by-design` (an action the reference
  implementation fired once per object token, which SysML v2 performs once with every delivery)
  or `not-expressible`, with a reason naming the construct and where it occurs; the per-activity
  checklist and counts (24, 4, 15; 12) are pinned by test, and CI downloads the suite and runs
  the reader and classifier gates on their own. See `docs/project/fuml-referee.md`.

- **The fUML reference implementation's activity tests are provisioned as an oracle for the
  action executor.** `./scripts/download-fuml-suite.sh` fetches ModelDriven's pinned
  `fUML-Tests.uml`, `fUML-Exception-Tests.uml`, the foundational library and the `fuml-1.5.0a`
  jar with its Maven runtime dependencies, every one by checksum, into the ignored `build/fuml/`;
  `make fuml-expected` runs the implementation over both models through a small Java driver
  that selects each activity by XMI id and records its outputs, its nested
  `Execute`/`Fire`/`Output`/`Complete` trace with the XMI id of each activity and action node
  the trace names unambiguously, and provenance in `docs/project/fuml-referee-expected.json`.
  A failed activity leaves the committed record unchanged. The record is committed and
  `internal/fuml` reads it back, refusing one whose provenance is not the current pin's, so the
  ordinary test gate never runs Java. See `docs/project/fuml-referee.md`.

- **The fUML test activities referee the action executor.** `cmd/fuml-referee` translates
  every expressible activity of the pinned fUML reference implementation's test model into a
  `fuml::<Activity>` action definition by rule — parameters and their multiplicities, control
  and object flows with an enabling succession beside each flow whose target has no control
  predecessor, forks, joins, merges and guarded decisions, value specifications, nested
  behavior calls with same-named parameters spelled apart, the primitive and list library
  functions KerML has counterparts for — runs it under every schedule the runtime's explorer
  reaches, and requires the values left in its output parameters to be the ones the reference
  implementation recorded, as a multiset where the fUML parameter is unordered; the reference's
  firing sequence being among the reachable ones is reported and never a verdict. Each activity
  is filed as `pass`, `fail`, `not-expressible` or `differs-by-design` (an action the reference
  fires once per object token), the counts are pinned in `docs/project/fuml-referee-baseline.json`
  and checked in CI over the downloaded suite by `go run ./cmd/fuml-referee -check`; an
  activity the emitter does not yet translate (object creation, structural-feature actions,
  accept-event actions, active classes) is `not-expressible` with the construct named. The
  `-json`, `-filter`, `-keep` and `-jobs` flags report, narrow, retain the emitted models and
  parallelize the run; a filtered run never updates the baseline. `docs/project/fuml-referee.md`
  documents the translation rules and the adjudication of every row, the spec-compliance action
  section gains its row, and the precise-semantics alignment note gains row A15 for per-token
  re-firing.

- **A document query over gRPC binds an object the service holds.** `Instantiate` now keeps
  the object it creates, in one runtime per cached model, for as long as the model stays cached;
  instantiating the same usage again denotes the new object and keeps the earlier one by id.
  `RunDocumentQuery` binds a parameter to such an object through the new `object` arm of
  `DocumentValue` — a `DocumentObject` naming it by `instance_id`, by `path` (`car`,
  `Garage::car`, `#2`, `car.wheels[2]`: what `%run-query` accepts) or by both — and runs the
  query in that runtime over the held population, so `DocumentQueries::Objects(type = T)`
  enumerates what the model holds (no rows before the first `Instantiate`) and `Verdicts`
  checks a bound object's current values. A row that is an object, and an object-valued cell,
  is answered as the `object` arm with the object's id, the path it is reached under and the
  usage it stands for; `RenderDocument` renders over the same population, as `-render-document`
  does beside `-instantiate`. A binding while nothing is held or naming an unknown id or usage
  is `NOT_FOUND`; a path that does not reach an object, an out-of-range index, an object bound
  to a non-`Element` parameter or an id its path disagrees with is `INVALID_ARGUMENT`, with the
  REPL's wording. The Go client binds with `opensysml.ObjectByID`/`ObjectByPath` and decodes
  `Object` (`ID`, `Path`, `Element`) as a cell and as `Row.Object`; the Python client binds with
  `ObjectRef(id=…)`/`ObjectRef(path=…)` and decodes `ObjectRef` as a cell and as
  `DocumentRow.object`; the Node, Java and Rust clients carry the regenerated stubs. What one
  model holds is bounded by `OPENSYSML_GRPC_MAX_HELD_OBJECTS` (default `10000`, nested objects
  counted): an `Instantiate`, query or render whose objects would pass it fails whole with
  `RESOURCE_EXHAUSTED`, leaving none of them, until the model leaves the cache, which releases
  its objects; none is evicted behind an id a client holds.

- **`ApplyEdits` edits a model of several documents as one atomic batch.** A model parsed by
  `ParseSources` was refused with `FAILED_PRECONDITION`; its operations are now applied through
  the same cross-document path the LSP uses: a rename or cascade delete follows its references
  into the model's other documents, every document touched is re-parsed and re-analysed together,
  and either all of them are answered or none is. `ApplyEditsResponse.documents` (new field 7)
  lists every document the batch rewrote — one entry for a model of one document — as
  `EditedDocument{name, content}`, `name` being the name the parse request gave it, so a new
  client has one code path for both shapes; `content` (field 1) keeps the edited notation of a
  model of exactly one document and is empty for a model of several, even when only one changed.
  Each `AppliedEdit` names its `document` (new field 7), and a refusal — which still carries no
  content — names each referrer with its document in `referrers` (new field 8, `Referrer{name,
  document}`) beside the textual `referring_elements`. `ApplyEditsRequest.document` (new field 3)
  selects a document other than the model's first for the operations to target; a name that is
  not one of the model's is `INVALID_ARGUMENT`. `ApplyEditsRequest.accept_documents` (new field 4)
  says the client reads `documents`: a model of several is edited only for a request setting it,
  and one leaving it unset — every request a client of the previous schema sends — is refused on
  such a model with `FAILED_PRECONDITION` as before, so a client reading `content` alone is never
  answered an empty one; a model of one document ignores it. The `edit_documents` capability
  advertises all of this: a service without it answers `content` alone — no `documents`,
  `referrers` or applied-edit `document` — refuses a model of several documents with
  `FAILED_PRECONDITION` and a request naming a document with `UNIMPLEMENTED`, so a client reading
  `documents` checks it first. `EDIT_FAILURE_REFERENCED_ELSEWHERE` is appended
  for a rename, delete or move referred to from a document the edit cannot rewrite — a move
  respells references in its own document only, so one referred to from another document of the
  model is refused this way. No existing field changed number, type or meaning, so a generated
  client of the previous schema decodes every answer. `Convert` from a model handle still
  requires a model of one document. The Go client answers `EditResult.Documents`,
  `AppliedEdit.Document` and `EditError.Referrers`, and `ApplyDocumentEdits` names the document
  to edit; the Python client answers `EditResult.documents`, `AppliedEdit.document`,
  `EditError.referrers` and raises `ReferencedElsewhereError`; the Node, Java and Rust clients
  carry the regenerated messages. The conformance suite parses a model of several documents by
  naming `fixtures` rather than one `fixture`, and gains scenarios for a rename and a cascade
  delete crossing documents, an edit answering only the document it touched, and a refusal
  naming a referrer in another document.

- **Imports are followed to the files beside a loaded one.** A file named to `sysml`, `%load` or `sysml -check` that imports a root namespace neither the named files nor the standard library declare now has the `.sysml` and `.kerml` files beside and below it searched for one declaring that name, and each such file is loaded with it, its own imports followed the same way; unrelated siblings and hidden directories stay out. The language server does the same for a document opened from outside every workspace folder, indexing the document's directory so its imports of sibling files resolve instead of being reported unresolved; an open buffer stays authoritative over the file on disk (`project.Dependencies`, `TestLoadingOneFilePullsInTheSiblingsItImports`, `TestOpeningFileOutsideFoldersIndexesItsDirectory`).

- **The Java client wraps every RPC the service offers.** `Connection` gains `parseSources` (a model of several documents), `convert`/`convertFile`, and `listEngines`; `Model` gains `executeAction`/`executeState` and `exploreAction`/`exploreState`, `verifyConstraint`/`verifyRequirement`/`verifySatisfaction`/`validateInstance`, `evaluateCalc`, `runAnalysis`/`exploreAnalysis`, `runSweep`, `applyEdits`, `query`/`queryOslc`, `runDocumentQuery`, `renderDocument`, `convert` and `withEngine`. Each answers an immutable record — `ActionRun`, `StateRun`, `Exploration` of `Outcome`s, `Verification`, `Satisfaction`, `Validation`, `Verdict`, `VerificationVerdict`, `Calculation`, `Analysis`, `CaseEvaluation`, `Standing`, `QueryElement`, `EngineInfo`, `Conversion`, `EditResult`, `Sweep` of `SweepRow`s, `DocumentQueryResult` of `DocumentRow`s, `RenderedDocument` — with no generated protobuf type in the public API. A false verdict is returned as a decided answer rather than thrown; `ModelException.failureReason()` classifies an in-band failure, `AnalysisException.partial()` keeps what a failed analysis computed before it stopped, and `EditException` carries the `EditFailure` kind and the referrers a refused batch named. The Java conformance runner now covers every RPC through the public API — 129 of the 134 scenarios run and pass per protocol, and the five skips are only requests the public API cannot express.

- **A design for scaling to very large models** (`docs/project/large-model-scaling-design.md`).
  Starting from the satellite-network stress test's profiles, it separates the four costs a large
  model pays — per element once, per workspace per edit, per process on one core, and per modeled
  object by construction — and designs one approach against each: a persistent resolver and
  semantic model owned by the workspace and invalidated through a document dependency relation
  rather than cleared on every change; closed documents held as interface records (the facts other
  documents can observe, plus stored diagnostics) that hydrate to a full tree only when opened or
  queried, generalizing the standard library's snapshot and index-record cache; parallel
  per-document analysis over a read-only index; and one definition with many occurrences in the
  runtime, with sparse per-occurrence values. Each names its differential test against the
  unoptimized path, the measurement that decides it, and its place in the sequence. Nothing is
  implemented; the page exists to be reviewed before code is written.

- **Documents typeset LaTeX mathematics, inline and displayed.** A `Span` or `SpanColumn` with
  `style = "math"` is an inline formula, and the new `Formula` content block (required `source`,
  optional `caption`, a `Ref` target when named) a displayed one; the LaTeX reaches every backend
  unescaped while a `$` in ordinary prose is escaped so it never opens a formula. Markdown writes
  `$…$` spans and `$$…$$` blocks; HTML wraps each formula in a `sysml-math` element between MathJax
  delimiters, and `-html-math cdn|<url>` has the page load MathJax, confined to those elements;
  PDF typesets each formula with KaTeX's command line (`katex`, or `OPENSYSML_KATEX`, its stylesheet
  found beside it or named by `OPENSYSML_KATEX_CSS`) and embeds its fonts under every engine, so the
  PDF shows mathematics rather than source. A blank formula, a blank math span or a query row
  supplying no LaTeX to a math column is a typed error; a document without formulas needs no KaTeX.
  `scripts/download-doc-pdf-toolchain.sh` now provisions a pinned KaTeX beside the other tools.

- **The language server runs the behavior a diagram draws.** The new `opensysml/debug/*`
  requests (`start`, `step`, `continue`, `send`, `advance`, `breakpoints`, `stop`), advertised as
  the `openSysmlDebug` capability, execute the state machine or action a `state` or `action` view
  renders — with the executors the REPL's `%state` and `%action` debuggers use, optionally as
  performed by an instantiated part — and answer every request with a snapshot in the IDs of that
  view's `opensysml/render` result: the active states (composite states and regions included, one
  chain per orthogonal region), each action token with the node it sits at, the edge it arrived by
  and the join edges or signal it waits for, the transitions and successions taken since the last
  snapshot, the events queued and the messages pending, the runtime's clock, its notes, a completed
  action's results, and whether the run is `running`, `waiting`, `suspended`, `completed`,
  `failed` or `ended`. Breakpoints are set by render node ID and pause a run as a token reaches the
  node or the state becomes active; a signal the behavior accepts nowhere is refused rather than
  queued to be lost. A session follows the document: an edit that leaves as they were the
  declarations the run reads — the target's, its performer's, and every declaration those name
  and the named name in turn (specialized and typing definitions, invoked actions, accepted
  signals, feature types, values a guard or a `send` names) — keeps it running and reports the
  snapshot in the fresh IDs (a pause reached at a breakpoint moved with them) through the new
  `opensysml/debugChanged` notification — every snapshot numbered by `revision`, so a client keeps
  the newest whatever order the answers and notifications arrive in — while one that
  rewrites or removes any of them, makes the run read one it did not, or rewrites the declared
  view, ends it and says why. To place runtime state on a rendering, the runtime
  now records the transitions a state machine fires (`StateExecutor.FiredTransitions`) and the
  successions each token travels (`ActionExecutor.Traversals`, `Token.Within` for the nested flows
  it runs in) — a driver reading them step by step takes only what a mark it kept has not seen
  (`FiredSince`, `TraversalsSince`, `NotesSince`) — a held image of an object carries its
  debugger's state with it (the breakpoints set, the pause reached, the record so far), a
  `view.StateLocator`/`view.ActionLocator` map lowered vertices, action nodes and edges to render
  IDs by their position within the declaration, and document, they were written in
  (a node or edge inherited from another document is drawn from that document and keeps its place
  as either document is edited), and `model.Workspace.NewRuntime` builds a
  runtime model over a workspace's documents whose `Dependencies` lists the declarations a run
  reads; a lowered `StateGraph` or `ActionGraph` lists the
  declarations it took content from besides its own (`Inherited`), which the root node of a
  `state` or `action` rendering carries as `view.Node.Inherited`. See
  [the LSP reference](docs/reference/lsp.md).

- **The SysML v1 migrator writes every diagram as a `view`.** A `uml:Diagram` serialized in a tool's `xmi:Extension` with its diagram representation (MagicDraw and Cameo's `DiagramRepresentationObject`) is read as a tool-neutral diagram record — name, kind, owner and shown elements — and written as a `view` usage in the body of the v2 element its owner became, exposing every shown element the document writes and rendered by the standard `Views` library's `asTreeDiagram`, `asInterconnectionDiagram`, `asElementTable` or `asTextualNotation` according to the diagram's kind. A diagram whose owner has no v2 body is written in the nearest ancestor that has one, one showing nothing writable is an empty view, and each is reported as mapped or approximated with the reason instead of skipped as tool content; layout stays unmigrated. `render` and the other reference members of a view now accept a globally qualified name, `render $::Views::asTreeDiagram;`, as the grammar allows.

- **The SysML v1 migrator writes a simulation tool's Monte Carlo analysis pattern as an analysis case.** A block generalizing the SysML customization module's `MonteCarloAnalysis` — recognised by the module's provenance alone, so a user's own block of that name stays an ordinary block — keeps its `part def` and gains a sibling `analysis def '<Block> Monte Carlo' :> Simulation::MonteCarlo` whose subject is the part def, whose one run performs the block's classifier behavior, whose `observed` is the value the block binds to `Mean`, and which returns one statistic per bound `Mean`, `Deviation`, `N` or `OutOfSpec` (`return Mean : Real = mean;`, `out Deviation : Real[0..1] = deviation;` …); its result snapshots record the four statistic slots as an `analysis` of that def, and the `-migration-results` sidecar names the analysis def and its declared statistics. The generalization, the binding connectors and the slots move out of the report's unmapped rows; a binding of another statistic, of a non-numeric value, of one statistic twice, or of a statistic of nothing bound to `Mean` is a comment naming the reason.
- **`Simulation::MonteCarlo`, an analysis of repeated runs, joins the OpenSysML library, and `-runs` runs it.** `sysml -analysis "<case> <subject>" -runs <n> -seed <s>` (or `%runs <n> <seed> <case> <subject>`) performs the case's steps on a fresh subject per run, seeded from the seed and the run number, tables what each run observed, and concludes the case once over the sample with `runs`, `mean`, `deviation` (the sample standard deviation, empty under two runs) and `outOfSpec` bound and its own outputs evaluated over them. A case that specializes no `Simulation::MonteCarlo`, a subject named by `#id`, or `-observe` is refused; a single run leaves the statistics unbound rather than passing one run off as many. A quantity-valued `observed` is sampled by magnitude in the first run's unit and `mean` and `deviation` are quantities in it; when every run fails, the table keeps each run's row and error and the case is reported unconcluded. A check decided run by run counts the runs it failed in as `outOfSpec` and is not judged again at the conclusion, which decides only the checks of the statistics; the count of runs is validated against the sweep budget before anything is sized by it; and the `-runs` statistics sum Integer deviations exactly and scale Real deviations before squaring, so a finite sample never overflows. A failed run fails the case whatever comes of the sample; only a table of completed runs is left unresolved when the sample or the conclusion cannot be made.
- **`-compare-results` compares a Monte Carlo analysis statistic by statistic.** Under the observable's table, one row per statistic of the migrated analysis case — its declared returns, then the outputs of `Simulation::MonteCarlo` the tool stored without a return: the tool's pooled `Mean` and `Deviation` against the runs' by the same aggregation with their relative difference (the deviation is newly compared), `N` side by side, and `OutOfSpec` recorded but not compared, being the tool's own criterion.

- **The SysML v1 migrator follows stereotype generalization and writes user profiles as metadata.** An applied stereotype is resolved to its `uml:Stereotype` in the document's profiles and its generalizations followed — through OMG `href`s, MagicDraw `referentPath`s and Papyrus pathmaps, across diamonds and cycles — so a user stereotype specializing «Requirement», «Block», «ValueType», «Satisfy», «Verify», «Refine», «Trace», «DeriveReqt», «Allocate», … takes that standard stereotype's v2 form with its tags (`Id`/`Text` become the short name and `doc`). User profiles become packages of `metadata def`s typed from their tag definitions (`attribute` for String/Integer/Real/Boolean/enumeration tags, `ref` for element references, `:>` between user stereotypes), and their applications `@Profile::Name { tag = value; }` usages instead of comments; the modeling tool's own profiles, known by exact namespace path, are skipped together with the content they mark (specification-dialog customization, UI prototyping mockups, simulation-tool configuration), with a reason naming what it is. A same-named stereotype with no standard general, and an application whose profile the document does not define, keep their previous form.

- **The SysML v1 migrator writes a Cameo/MagicDraw table, dependency matrix or relation map as an executable query and a renderable document.** A diagram carrying «InstanceTable», «DiagramTable» or «RelationMap» from the MagicDraw profile, or «DependencyMatrix» with its «MatrixFilter», is written beside its `view` as a `calc def '<Diagram> Rows' :> DocumentQueries::Query` — the scope as `Descendants` of the named roots, explicit rows in one `Union`, the row type as `WhereType` (and `isIndividual` for an instance table) or `WhereMetadata` for a migrated user stereotype, columns as `Project` and `Column`, sorts as `OrderBy(missing = "last", multiple = "first")`, a matrix criterion as a `RelatedColumn` over the column scope, a relation map as `RelatedElements` — and a `part def '<Diagram> Document' :> DocumentQueries::Document` holding the `Table`, so `-run-query` lists the rows and `-render-document` renders the table. Only the exact profile namespaces define a table; a same-named user stereotype elsewhere is ordinary metadata. A «DeriveReqt» criterion is walked from the original requirement, as the v2 `derivation` runs, and a criterion excluding subtypes of a stereotype the model specializes is approximated with the specializing stereotypes named, since their relationships are written as the same v2 relationship. A criterion no relationship kind spells, a malformed scope, sort, depth or criterion XML, or a table naming no row type is refused with every fault stated and the view kept.
- **The migrator writes an MDK DocGen «Document» as a `DocumentQueries::Document`.** The document's view tree becomes nested `Section`s in declaration order, and each view's viewpoint method activity is lowered from its initial node along control flow: the «Expose» suppliers are the root, `CollectOwnedElements`, `CollectOwners`, `CollectByDirectedRelationshipStereotypes`, `FilterByMetaclasses`, `FilterByStereotypes` (`include = false` as `Except`), `FilterByNames`, `SortByName`, `SortByAttribute`, `Union` forks and nested groups wrap the query, and `TableStructure`, `BulletedList`, `Paragraph`, collaborator paragraphs, `Image` and `Dynamic View` end it as a `Table`, `List`, `Paragraph`, view-backed `Diagram` or nested `Section`. A step with no query spelling (`CollectTypes`, OCL expressions, user scripts…), a recursive dynamic view, a viewpoint that names no method or a collaborator paragraph that names no view is refused with the construct quoted, and the sections around it are still written.
- **`DocumentQueries` gains `Named`, unbounded walks, matrix targets and `isIndividual`.** `Named(qualifiedName = (…))` resolves qualified names to elements, so a query can be rooted at a package or definition; `maxDepth` on `Descendants`, `Ancestors`, `RelatedElements`, `WhereRelated` and `RelatedColumn` may be omitted or `null` for no bound; `RelatedColumn(targets = …)` keeps only the related elements a second query lists, which is a dependency matrix's cell; and `WhereFeature('feature' = "isIndividual", …)` selects individuals. A declared `satisfy`/`verify` assertion typed by a requirement definition now relates its subject to that definition as well as to the assertion usage, so a matrix over requirement definitions finds its satisfiers.
- **The migrator reads MagicDraw's «typeModifier».** `[]` on a property or parameter with no collection multiplicity writes `[0..*] ordered nonunique`, `[n]` writes `[n] ordered nonunique`, and `*` on a part or item property writes it `ref`; a two-dimensional shape, `[]` on an existing collection and `*` on an attribute or parameter stay comments with the reason reported.

- **The v1 migration writes views, viewpoints, use cases, UML Expression trees and interface realizations, which it refused before.** A «View» class is a `view` usage that satisfies the viewpoint its tag or «Conform» relationships name and exposes what its «Expose» dependencies do (`expose P::**;` for a package); a «Viewpoint» class is a `viewpoint` usage with its stakeholders, its concerns framed and its purpose, language and method documented. A UseCase is a `use case def` with its subject, the actors its associations reach, and its inclusions; an Extend is a dependency on the extended case, since v2 has no `extend`. A UML Expression tree — a constraint's specification, a default or a slot value — is lowered to a v2 expression over arithmetic, comparison and Boolean operators (spelled as signs or by name, `Plus`, `Equal`), feature references — a bare symbol, or the ElementValue operands MagicDraw keeps in an `xmi:Extension`, which the XMI reader now reads as operands — and the calls the opaque-language subset already translates. An InterfaceRealization is a port typed by the interface's `port def` on a block, or a specialization on an interface block. Whatever v2 has no form for — an exposed diagram, an extension point, an operator outside the set, an interface outside the document — stays a comment and the report names the reason.

- **The Windows installer now runs a setup wizard.** Double-clicking the MSI used to flash Windows Installer's bare progress window and close with no indication of what had happened. It now walks through Welcome, a *Destination Folder* page (with a folder browser), a *Choose components* tree for the optional gRPC service and bundled Z3 solver, a *Ready to install* confirmation, progress, and a *Completed* page that names the install folder and reminds you to open a new terminal for the updated `PATH`. Running the MSI again offers *Repair* and *Remove*, and a newer MSI proposes the folder chosen last time (recorded under `HKLM\Software\Open-MBEE\OpenSysML`). The dialogs are authored from the standard Windows Installer controls in `packaging/msi/wizard.wxs`, so the MSI carries no custom-action code or binaries; `ADDLOCAL`/`REMOVE` and a new `INSTALLFOLDER` property keep working for silent installs.

- **`-layout <mtip-export.xml>` lays out a SysML v1 migration's views from an MTIP export.** `sysml model.mdzip -convert sysml -layout model_mtip.xml` joins the export's diagram records to the migrated diagrams by element identifier and writes their geometry as `DiagramLayout` metadata — `Layout` per exposed element, `Route` per exposed connector, `@Canvas` sized by what was written — which every rendering honors. What the export shows but the view does not expose, records matching no diagram, malformed records and unsupported presentation properties are counted and reported, never dropped silently; a layout exported from a different project is refused. Without `-layout` the migration is unchanged.

- **Nested action flows in loop and branch bodies now execute with their stated succession, fork, join, and termination semantics.** Body-local attributes are initialized for each iteration.
- **Action-node body execution now covers stated flows in action and state behaviors.** Invalid or unsupported body forms report typed runtime errors.

- **A nightly snapshot of `develop` is published as the prerelease `nightly`.** Every night
  the newest green `develop` commit is built into the same archives, raw `sysml-grpc`
  binaries and signed `SHA256SUMS.txt` a release ships, and published under the moving
  `nightly` tag; a version of the form `nightly-<yyyymmdd>-<commit>` tells a snapshot apart
  from a release. The snapshot is never marked latest, so `releases/latest`, Homebrew and the
  client packages keep following the stable line. The new *Nightly snapshots* page, linked
  from the landing page and the install guide, says where the snapshot is, what it contains,
  how to verify one and what to expect from it.

- **The nightly snapshot ships the VS Code extension.** `opensysml-sysml.vsix`, packaged from the same commit as the binaries, is attached to the `nightly` prerelease and listed in its signed `SHA256SUMS.txt`, so the extension can be installed with `code --install-extension` without a checkout. Its version is the manifest's with the snapshot version as the pre-release part (`0.1.0-nightly-<yyyymmdd>-<commit>`), so a later night installs over an earlier one as an update; `make vscode-package VSIX_VERSION=…` stamps a version the same way.

- **`sysml-lsp` is declared to OpenCode.** A checkout carries an `opencode.json` that starts the language server for `.sysml` and `.kerml` files, so the coding agent reads its diagnostics the way it does `gopls` for Go; the editors chapter of the guide explains the configuration and how to enable it for every project.

- **An operation is invoked with a positional argument list.** `Context.InvokeOperationWith` takes `OperationArguments`, positional or named, and `%invoke <object> <op>` accepts bare expressions (`%invoke rover drive 10 20`) beside its `<parameter>=<expression>` pairs. Positionals bind the operation's `in`/`inout` parameters in signature order, a trailing defaulted parameter may be omitted, and among same-named operations the one the arguments fit is selected as an invocation expression would. A list mixing the two forms is refused (`ErrMixedArguments`), as is a surplus argument (`ErrOperationArity`).

- **A design record on protocol state machines** (`docs/project/protocol-state-machines.md`). It
  establishes that SysML v2 has no counterpart to UML's protocol state machine and needs none for
  the half it can express: the legal order of receptions on a port or part is an ordinary exhibited
  state machine, which the runtime runs on parts with `accept … via`. It also records what the
  runtime does not yet do: a message a model sends that the active state neither accepts nor defers
  is held on the bus and taken by a later state rather than dropped and reported as a directly
  injected event is, and a machine exhibited by a port definition does not take a model's messages
  routed to that port. Post-conditions, conformance between machines, static sequence checking and
  the gating of operation calls by state have no SysML v2 spelling. The record specifies the runtime
  follow-up with its proof fixtures; the roadmap item stays open and the compliance bullet points at
  the record.

- **The PSSM referee spells entry, do and effect behaviors with parameters, bound to the triggering event's data, and behaviors that return a call's result.** The accepting transition's effect stores the accept's signal payload or operation arguments in attributes of the machine and the target state's entry or do action declares its parameters bound to them (`in p : T = trigger_…;`), an effect reads the `accept`'s own parameters, and a behavior producing the operation's result becomes an `action def` with `out` parameters the runtime returns to the caller; the classifier refuses only an exit behavior with parameters, which runs before the leaving transition's effect, the first place the accepted data is readable, and every other refusal reason is unchanged. Same-named operations keep their identity through the call, its carried attributes and the tester's stimulus; a trigger naming one of two overloads with a single `accept` spelling between them is refused. *Event 019 D*, *Event 019 E*, *Deferred 007* and *Standalone 003* move from `not-expressible` to `pass` (56 pass / 13 fail / 33 not-expressible / 1 differs-by-design).

- **A synchronous call of an operation a state machine accepts as a call event returns the operation's outputs to the caller.** `StateExecutor.Call` queues the call event, runs the machine through the run-to-completion step dispatching it — later events that step queued and timers it armed wait for the machine's next run, while a call a state defers holds its caller until the machine recalls it — and releases the caller with the values the behaviors that step fired — the transition's effect, an entry or an exit — returned or assigned to the operation's `out` and result parameters, by name — the parameters the operation declares as a member of the machine's owner when it declares one, an `inout` the step left unwritten going back as passed, every output the step returned otherwise — as PSSM §8.5.9 resumes a synchronous caller after the run-to-completion step; a call the run leaves queued or deferred is reported as `ErrCallNotReturned`, one no transition accepts is discarded. The arguments are checked against the declaration the call selects among same-named operations before the call is queued — an unbound, unknown or wrong-typed one is refused, an omitted input carries its default — and the queued call event carries that declaration, so it fires only the triggers naming it and same-named overloads whose parameter names differ reach their own transitions. A snapshot and a held image capture the call in flight. A nested action's `return` or output assignment reaches the enclosing behavior's parameter of that name on the way. Conformance case `state_call_trigger_results` and `TestRuntimeRobustnessCallResults` cover it.
- **The PSSM referee drives the tester's stimulation in the tester's order and reads a standalone state machine as the class under test.** The driver performs each send, synchronous call and `trace(...)` of the tester's behavior as the tester does, appending a traced value to the target's `log` once the call it embeds has returned, with the suite's test library (`Concat`, `ToString`, `formatParameterValue`) read into the model and evaluated generically; the reader reads a `StateMachine` that is itself the class under test as a target with its attributes, operations and constructor. *Event 019 A* moves from `not-expressible` to `pass` (52 pass / 13 fail / 37 not-expressible / 1 differs-by-design); tests needing an entry, exit or do behavior with parameters, or an effect that returns the call's result, stay `not-expressible` on exactly those reasons until the emitter spells them, and every other test's result and reason is unchanged.

- **Document queries express requirement coverage gaps.** The new
  `DocumentQueries::WhereRelated(source, relationshipKind, direction, maxDepth, exists = true)`
  keeps each row by whether at least one element is reachable from it over a named relationship —
  every kind `RelatedElements` accepts, through the same edge tables, typed errors and visit
  budget — and `exists = false` keeps the rows with none, so "which requirements does nothing
  satisfy or verify" is one filter over incoming `satisfaction` or `verification` edges. The new
  ordered set operations `Except(source, exclude)` and `Union(source, other)` combine query results
  by the identity traversal already deduplicates by (a model element by its declaration, a held
  object by the object, a verdict by its assertion and the object it was checked on), keeping
  source order and projected columns. The query cookbook gains a Coverage section
  (`UnsatisfiedRequirements`, `UnverifiedRequirements`, their union and difference) and a
  Requirement hierarchy recipe that lists nested requirement usages and definitions under a root
  in tree order with `shortName`, `name` and `documentation`, which the requirements example
  renders as a table of its report.

- **Record analysis runs into the model.** `%record <case> [into <package>]` at the REPL and `-record-run <case>` on the command line run an analysis case as `%analysis`/`-analysis` does and write the run into the model as `AnalysisRecords` elements — a record definition per case, one part per run carrying the inputs bound and outputs produced, and `@AnalysisRecords::RecordedRun` provenance metadata. Sweeps (`-sweep`) and Monte Carlo samples (`-runs`/`-seed`) record one part per run, plus one for the sample's conclusion under kind `sample`; records compose with `-convert sysml -o` and `-render-document`, are found by document queries, and an `inout` records the value the run left and a `<name>In` companion for the value it was bound with, and values supplied as Integer and Real alike settle a member to Real and scalar-valued enum literals keep their literal, and a failed run records nothing. A verification case's record carries the verdict its body decided — the `verdict` attribute — and one `VerdictRecord` row apiece for it and each subcase's.

- **The order in which a state's regions are entered, exited and fired across is a recorded choice point.** A composite state's regions, a fork's branches and a history's restored regions were entered in declaration order, exited in declaration order, and the transitions one occurrence selects across regions fired one whole firing at a time, with no choice recorded for `explore` to vary. Each site now runs its regions as the queues of one front, a unit — one state's entry, one state's exit, one segment's effect — at a time, and each draw of which region's next unit runs is a choice point (`choice entering work: next left(entry), right(entry) …`, `exiting <state>`, `fork <name>`, and `on <event>` for the units of the firings) written to the trace and the witness, replayed, refused and rolled back with its move, and enumerated by `explore` and `check`. `declared` and `reverse` take the order the runtime always took, so no event order under a fixed policy moved and existing trace goldens gain only `choice` lines; a unit that performs no behavior is drawn with the performing unit beside it, so exploration counts linearizations of behaviors. Seven conformance cases state the admissible sets (`state_region_entry_order`, `state_region_entry_order_uneven`, `state_region_entry_nested_front`, `state_fork_branch_order`, `state_history_restore_order`, `state_region_exit_order`, `state_firing_units_interleaved`). The PSSM referee moves *Exiting 001*, *Exiting 003*, *Fork 002*, *Terminate 001* and *Deferred 006 C* to `pass` (51 pass, 13 fail, 38 not expressible, 1 differs by design).
- **A design note on the order of orthogonal regions** (`docs/internals/design/region-order-scheduling.md`) records the sites above as implemented, the alignment row deciding that a firing is not atomic across regions (argued from the KerML library's successions), and the designs of two further sites: a due `do` step against the dispatch at the head of the pool, drawn per token move of the `do` flow (landed below), and the firing of a completion a region's entry enables drawn inside the entry front, still open. Until that one lands, *Entering 010*, *Entering 011*, *Junction 005*, *History 001-C*, *History 002-B* and *Terminate 002* stay `fail` in the referee record with that attribution.

- **Run-to-completion redefinitions now execute with their lowered values and scopes.** Entry cascades expose free-dispatch versus held-entry ordering to execution, checking, exploration and replay, while invalid scopes remain typed lowering errors.

- **A satellite-network stress workload and its scaling record.** `go run ./cmd/stress-model -planes P -satellites S -ground-stations G` writes a constellation in which every spacecraft is modeled to its components — seven subsystems, twenty components with unit-bearing attributes, power and data connections, mass and power budgets, requirements with satisfy assertions and a mode machine — plus crosslinks and ground-station downlinks; `-stats` reports the satellites, components, connections, requirements, declared elements and bytes it wrote. `tests/stressmodel` generates the same model in-process and benchmarks loading (with retained heap), satisfaction and an editor keystroke beside the open model. `docs/project/satellite-network-stress-test.md` records how `sysml -validate`, `sysml -satisfy` and per-edit re-analysis scale from 2 to 12 800 satellites (2.4 million elements), where each stops being practical, and what the profiles show.

- **The architecture self-model now shows the runtime from the inside.** A new
  `examples/self-model/execution.sysml` models the instance layer as the six units it is — the
  schema built once per type, the allocator, the lazy reader, the binding propagator, admission
  and the dependency tracker — with the value flows between them, the scheduler with its kinds
  of choice and policy spellings, and one action executed as an interaction from the surface's
  request through lowering, stepping, scheduling, evaluation and every feature read or written.
  `behavior.sysml` gains `ReadFeatureValue`, one feature read whose decision nodes are the cases
  a feature can be in (undeclared, bound, held, a variation, a `default` yielding to
  contributions, a stated value, a connector, a composite). Three views render them —
  `instanceLayer` as an interconnection diagram, `featureReadFlow` as an action flow and
  `actionExecution` as a sequence diagram — and the architecture document embeds all three in
  its validation-and-execution section. The self-model test checks the modelled layer against
  the runtime: the effective feature's field count, the schema's memoization, the refusals the
  reader and admission spell, the scheduler's choice kinds and policy spellings, and every path
  of the feature read.

- **A send's `to` clause accepts any expression and addresses the objects it yields.** `send m to cars#(2)` or `send m via p to cars#(2)` now deliver to the objects the expression evaluates to, where before a receiver that was no name or feature chain silently fell back to the sender itself. A receiver yielding no object, a non-object value, or a destroyed object is a typed error, as is a routed send whose receiver object no connection reaches.

- **Sets and tensor quantities in RDF are the expressions that produce them, resolved by design.** The RDF mapping writes a model, never an evaluation, for every value kind, so a `Set`-, `UniqueCollection`- or `Map`-typed feature and a tensor of any rank need no literal form: they export as standard `OperatorExpression`/`LiteralExpression`/`FeatureReferenceExpression`/`InvocationExpression` trees with no new `sysx:` term, round trip exactly with the source text stripped, and the model read back evaluates to sets equal in whatever order their members were written and to tensors of the same shape and components. New tests pin the contract, including negative controls that remove each structural predicate; the native compilation of both remains open.

- **`ShapeItems` derived geometry evaluates where the library determines it.** A binding connector's own multiplicity is the number of links it declares, so `binding [1] bind [0..*] base.edges = [0..*] be` relates a `Disc`'s edge to some value of `be` instead of binding `be [2]` whole: a `Cylinder`'s or `Cone`'s `faces`, `base.edges` and `af.edges` answer (before, every read through `be` was a multiplicity violation), and so do those of a `Cylinder` nested as a `Box`'s `voids`. A `Box` also answers its per-face `vertices`. What the library leaves open stays a typed error naming why — the `[0..1]`-bound edge and vertex groups (`tfe`, `tflv`, `Box::vertices`), the curved face's edges through `cf : Surface`, and the frame features `matingOccurrences`/`spaceBoundary` — and binding diagnostics quote the connector multiplicity (`binding [1] bind …`). The declared link count is checked too: `binding [2]` identifying one value is a multiplicity violation, and `binding [0]` links nothing.

- The site's header menu (and its footer row on narrow screens) links to lord.opensysml.org.

- **A fifth runtime-showcase model: a spacecraft downlink.** `examples/runtime-showcase/spacecraft-comms.sysml`
  re-spells the OpenSE Cookbook's Spacecraft Example in current SysML v2: a ground station and a
  spacecraft with conjugate ports on a `CommunicationLink` interface, a `parallel` state machine
  whose `dataTransit` region sends frames and drains the battery in a forked do action while its
  `charging` region recharges on a change trigger, a `BatteryLow` signal that interrupts the
  transmission and a change trigger that resumes it. The walkthrough runs both parts on one clock
  with `%advance`, reads values off either object with `%eval in <object> : <expr>`, and shows
  where three timers falling due at the same instant leave `-schedule` a choice — 49 or 50 frames
  before the first interruption — while every schedule reaches the same end: 100 frames received
  at t=241. The REPL tests pin those checkpoints under `reverse`, `declared` and two seeds.

- **A document query reads the state a session's objects are in and the trace their run recorded.** `States(source = <rows>)` answers one row per active leaf state of each object's machine — every orthogonal region, with `machine`, `name`, `statePath`, `region` and the `enclosing` composite states — and `InState(name = "<state>")` the held objects whose machine is in that state, by leaf or enclosing name or dotted path. `Events(source, kind, since, before)` answers the trace as rows in the order the run made them — accepts, sends, transitions, entry, exit and do steps, `choice` draws with their alternatives and the one taken, unevaluable guards — each with its instant on the clock, its object and machine, the states touched, the payload and the line `-trace` prints; `kind` keeps one or several kinds and `[since, before)` an interval inclusive at the start and exclusive at the end, in the clock's unit or a duration. `WhereFeature`, `WhereName`, `WhereType`, `Project`, `OrderBy` and `Column` read the rows as they read object and verdict rows. Each unsupported path is a typed error: no session, no trace recorded, an object exhibiting no state machine, a state no machine declares, a bound that is no instant on the clock, an interval that ends before it starts, or a state or event row where an element or object is asked for. The rows print from `%run-query` and `-run-query` — which now runs after `-state`, `-action` and `-advance`, so it reads the run's end — render in Markdown, HTML (`span.sysml-state`, `span.sysml-event`) and PDF documents, and cross `RunDocumentQuery` as the `state` and `event` arms of `DocumentValue`, which the Go and Python clients decode and refuse to bind; the recipes are in the query cookbook.
- **The trace is a typed record the printer writes from.** `runtime.TraceRecorder` keeps each accept, send, transition, entry, exit, do step, choice and guard as a `TraceRecord` with its instant, object and behavior, and prints `-trace`'s lines from those records, so the printed trace and the `Events` rows cannot disagree; the printed output is unchanged.
- **A manual page tells the query kinds apart.** *Which query is which* distinguishes document queries over elements, objects, verdicts, states and events, the API `Query` over a project, `Evaluate`, `solve`, and the runtime population `all T`: what each returns, what each cannot see, and where each is reached.
- **`States` and `Events` refuse a destroyed object.** A `source` bound to an object the run destroyed fails with a typed `object-destroyed` error naming the object and the activation mark at which it was destroyed, as `%features` reports lifetimes, instead of answering a stale row or a feature-evaluation failure; a terminated machine answers no state rows, while a completed one reports its final state. A destroyed object leaves the population — `Objects`, `InState` and element-derived sources skip it, and `Events` still resolves its label — so a session's other objects keep answering once one is destroyed.

- **A continuous model runs by time-stepping.** An action specializing the analysis library's
  `ContinuousStateSpaceDynamics` or `DiscreteStateSpaceDynamics` is run as a fixed-step
  state-space simulation: the bundled `StateSpaceIntegration` library adds `FixedStepDynamics`
  (`timeStep`, an optional `stopTime`, a `time` the run writes), the integrators `Euler` and `RK4`
  a model binds to `getNextState`'s `integrate` (RK4 when it binds none) and the `ZeroCrossing`
  event; discrete dynamics step by `getDifference`. Each step advances the runtime's shared clock,
  so a state machine exhibited beside the dynamics sees the same time, its `accept after`/`at`
  triggers fire in step order, and a step and a trigger due together are a `due order` choice
  point the scheduling policy decides. An `event occurrence` typed by `ZeroCrossing` posts an
  event of its type when its `guard` changes sign at a step, which a machine's `accept` takes, and
  ends the dynamics when `terminal`. The run records `state: <action> t=<instant> x=<state>
  y=<output>` per step in the execution trace and reports `stateSpace`, `output` and `time` as
  the action's outputs. A shape the runner cannot run — a state, input, derivative or output that
  is not a vector, a protocol calc left abstract, an integrator the runtime does not provide, a
  step that is absent, zero or negative, a state that leaves a step non-finite — is a typed error
  naming the action and the member at fault.

- **A model states its own odds.** Two non-normative OpenSysML libraries add what SysML v2 has no
  notation for: `Stochastic::Probability` weights the successions out of a decision node
  (`first d then fast { @Probability { p = 0.7; } }`), and `RandomFunctions` declares `uniform`,
  `uniformInteger`, `triangular` and `normal`, so `attribute d : Real = uniform(0.0, 1.0);` and
  `accept after uniform(1, 80) [s]` run. The weights are validated at lowering — every succession
  out of a decision weighted or none, each in `[0, 1]`, constant weights summing to one. The flow among an analysis or verification case's steps reads the same weights, and the exported action graph carries each weight beside its edge. Modeled randomness is a
  stream of its own, apart from the token-shuffle stream: `-seed <n>` and `%seed <n>` fix it
  whatever the scheduling policy, `seed:<n>` seeds it too when no model seed is set, an unseeded
  weighted decision under `declared` or `reverse` takes its most probable branch, and an
  unseeded random function is refused naming the flags that seed it. The scheduling choice points
  the spec leaves open — token, write, region and due order — stay unweighted, and `explore` and
  `check` still enumerate and search weighted branches as a set. Every draw is recorded in the
  witness (`draw uniform(0.0, 1.0) = 0.7748…`) beside the weighted pick, so `%replay` and
  `-schedule replay:<file>` reproduce a run exactly and refuse a witness whose draws they cannot
  consume; the trace reports each weighted decision with its weights and its draw.
- **Monte Carlo runs.** `%runs <n> <seed> <action> [<observable>...]` and
  `sysml -action <a> -runs <n> -seed <s> [-observe <f>]` run an action `n` times, each under a
  model seed derived from the seed and the run number, on the sweep machinery, and report the
  table of the observables — every feature the action holds and `clock` when none is named —
  then each numeric observable's min, mean, max, p50, p90 and a compact histogram, as
  `%samples` reports a table. Runs are reproducible for a seed and distinct across seeds.

- **A plain `flow` between action parameters streams.** SysML v2 §7.16 makes parameters streaming unless a flow is designated a `succession flow`, and the runtime now reads them so: each value written to the source pin — in the source's body, or carried back from a node under it — reaches the pin of every ongoing performance of the target at once, so a consumer performing beside its producer reads each value the producer writes; a value written while no performance of the target is under way waits at the pin for its next one, and a further write from the same source performance replaces it, so a target begun after its source reads the pin as the source left it. The lowered `ObjectFlow` carries the kind (`FlowStreaming` or `FlowSuccession`) from the declaration, and a `succession flow` runs as before: the value the pin holds when the source completes moves, and the target begins after. A source that completes without ever writing the pin (`ErrFlowSource`), a write after the target's last performance ended that no later performance takes (`ErrStreamUnreceived`), a stream to a pin the target does not declare (`ErrNodePin`), and streaming flows that lead a value back to the pin it was written to (`ErrStreamCycle`) are typed errors. The outputs of an action a node performs stream from the node as the performance writes them, its declared output values as the performance begins. The checker's footprints count the target pins a node's writes stream to, and the SMT encoding streams writes as the interpreter does, a value arriving after its target's last performance failing the action as it completes.

- **SysML v1 migration is documented on every surface it reaches.** Guide chapter 11 walks one v1 export through `-convert`: reading the report's four verdicts, running a migrated activity under the action debugger, comparing a migrated run configuration with the results its tool stored, and finishing by hand what the mapping reports as unmapped. The gRPC `Convert` contract, the proto comments, the Go client's `FormatXMI` and the Python client's `convert` docstrings say that `xmi`, `uml` and `mdzip` are read and migrated, never written, and that the conversion is reported experimental; the Go client gains a regression test that migrates the vehicle fixture over the wire. The roadmap records what the migration still leaves — units and quantity kinds, the report over gRPC, identity across a re-migration — as its own item.

- **A design note for an Eclipse SysON plugin** (`docs/internals/design/syson-plugin.md`). It
  records, against SysON release `v2026.9.0`, how a jar contributes beans to the SysON backend
  and a React component to its frontend, the textual exporter and its gaps, the SysIDE-based
  importer and where a pre-import check sits, the partial SysML v2 REST API and the fact that
  SysON's standard-library `elementId`s equal the normative UUIDs `internal/semantic/identity`
  derives, how a qualified-name `Symbol.id` maps to an EMF element, how diagnostics reach the
  Validation view, and the verdict of `sysml -validate` on every textual model in the SysON
  repository; then the architecture of a future `editors/syson/`, a four-phase plan and the
  unknowns. `editors/README.md` now introduces each editor integration. Nothing is implemented.

- **The SysON plugin can run OpenSysML from the explorer.** The `runWithOpenSysML` mutation
  supports instantiation, action and state execution or exploration, constraint and requirement
  verification, satisfaction verification, calculation evaluation, analysis and instance
  validation; the “Run with OpenSysML…” entry opens a dialog, maps diagnostics into the Validation
  view, and supports an offline compile-only stub build plus an opt-in real-artifact profile and
  workflow.

- **`terminate` runs in an action.** A `then terminate;` node, a named terminate action usage
  (`action stop terminate;`, whose marker the parser used to drop) reached by a succession, and a
  `terminate;` statement of a nested action node's body (which lowering used to leave out) end the
  performance they are written in with the outputs assigned so far: later nodes do not run, every
  other token of that performance is dropped — a forked branch still running or parked at an
  `accept` included — in an order the trace records, and a nested node's parent continues along the
  node's succession. `terminate <name>;` ends every ongoing performance of the named action node
  of the flow it is in or of a flow around it, the node itself included. A performance that already
  ended and a name that is no action node of an enclosing flow are each a typed error rather
  than a silent no-op; a `terminate` in a calculation is refused as before.

- **`terminate` ends an occurrence, a state's behavior or the state machine.**
  `terminate <occurrence>;` evaluates its target — `this`, a part's feature chain
  (`terminate vehicle.engine;`), a nested action node's own occurrence — and ends that
  occurrence's lifetime: its owned parts, the behaviors it exhibits or performs, and any action
  or state performance running on it stop where they are, keeping their values, and a part of
  it first read afterwards is reached ended too, no behavior of it started; a name that
  denotes no occurrence, one already ended, and one `destroy` emptied are each a typed error.
  A `terminate;` in a state's `entry`, `do` or `exit` body ends that behavior at the statement,
  the state stays active and the machine keeps dispatching. A transition whose target is a
  terminate action (`transition first idle accept Abort then stop; action stop terminate;`) ends
  the state machine's performance as SysML v2 §7.18.3 and the PSSM's terminate pseudostate
  both prescribe: the source exits and the transition's effect run, then no further state is
  exited, running do behaviors are abandoned and no state remains active — reached directly, or
  through a choice, junction or join, from inside a composite state or one region of an
  orthogonal one. Such a run reports `Outcome.Terminated` with no final state, the REPL says
  `State machine terminated` and `Execution state: Terminated`, the LSP debug snapshot's `state`
  is `terminated`, `%instances` lists a terminated object as `ended`, and `explore`/`check`
  count the terminated run as one outcome. The PSSM referee translates the suite's terminate
  pseudostates the same way, so its `terminate-gap` bucket is retired: *Terminate 003* passes,
  and *Terminate 001/002* fail on the order an orthogonal state's regions are entered in, which
  the referee's record already attributes to an open finding.

- **Four graded requirements-traceability examples in the manual.** `docs/manual/traceability-examples.md` walks from three flat requirements and the parts satisfying them (`trace-1-basic.sysml`), through a nested requirement tree with verification verdicts and `Union`/`Except` coverage sets (`trace-2-hierarchy.sysml`) and derivation chains walked one hop and to their ends in both directions (`trace-3-derivation.sysml`), to a multi-package program whose ten-column matrix is grouped by owning team with list, count and any columns side by side (`trace-4-program.sysml`). Each source is committed beside its rendered Markdown, and a test re-renders all four and compares them, so the outputs shown are what the current binary produces.
- **A wide table in a PDF lands on landscape pages.** A table of seven or more columns, with the heading and caption that introduce it, is placed on a landscape page while the surrounding pages stay portrait; a traceability matrix no longer squeezes ten columns into a portrait text width.

- **`RelatedColumn(name, relationshipKind, direction, maxDepth, aggregate)` projects the elements a relationship reaches from each row.** A `Project` column beside `Column(...)`: `aggregate = "list"` (the default) yields the related elements as a multi-valued cell in traversal order, `"count"` an integer and `"any"` a Boolean, over every relationship kind and direction `RelatedElements` accepts, with the same typed errors and visit budget. The values feed `WhereFeature`, `OrderBy` and a document table's `groupBy` — an element-valued cell compares and sorts as its qualified name — so one query now produces a traceability matrix — every requirement with its satisfiers and verifiers — where a table per requirement was needed before. The cookbook gains a "Traceability matrix" recipe and the manual a rendered traceability report (`docs/manual/examples/traceability.sysml`).
- **A satisfaction asserted by an object's type and found again in a validation scope is reported once.** Verdicts deduplicate assertions by the declaration they name, not by scope-tree symbol, so a document scope indexed twice no longer doubles a requirement's satisfaction row.

- **`@Probability` weights state transitions.** The `Stochastic::Probability` notation that
  weights the branches out of a decision node now weights transitions too: of the transitions
  out of a state competing on one trigger (or the completion transitions), of all the branches
  out of a `choice` or `junction`, all carry a weight or none does, each weight lies in
  `[0, 1]`, and the group's weights must sum to one — checked at lowering for constants and at
  dispatch for expressions. A weighted pick is drawn once among the enabled transitions, after
  triggers, guards and innermost-wins have run, and is recorded in the witness, so `%replay`
  reproduces it, `explore` enumerates every weighted alternative, and the trace prints the
  drawn branch with its weight. Transitions sharing a time-trigger spelling fire as one
  occurrence drawn by weight, and a weight may read the trigger's bound arguments.
- **`explore` and `check` report probabilities.** The explore outcome table gains a
  `probability` column — the product of the shares each linearization's picks resolved with (a
  weighted pick its stated weight's share, an unweighted choice the uniform `1/n` a seed takes
  each alternative with), summed over the runs reaching each outcome — and `check` reports each
  violation's probability mass the same way, `(probability 0.3)` on its line and `mass` in the
  JSON report. Both are the model's own probabilities where every choice point is weighted, a
  uniform assumption otherwise; an incomplete exploration prefixes them `≥` and a check that
  hit a bound, revisited a state or left a move out marks them lower bounds
  (`probabilitiesLowerBound` / `massLowerBound` in the JSON, `Outcome.probability` and
  `ExplorationStatus.probabilities_lower_bound` on the wire).

- **The SysML v1 migration writes behaviors that run.** An Activity becomes an `action def`
  the action executor performs — `first start`, nested `action x : Def;` calls with `bind`/`flow`
  for their pins, `fork`/`join`/`decide`/`merge`, `send new Sig() to this.part`, `accept p : Sig`,
  `accept after 2.0 [SI::s]`, `accept when c`, `action x terminate;` for a final node, `if`
  guards where the guard parses and resolves and the guard text as a comment where it does not —
  and a block's classifier behavior is performed by a `perform action` usage of its `part def`.
  A `DurationConstraint` on an action is a wait before it, `accept after lo [SI::s]` for a point
  interval and `accept after RandomFunctions::uniform(lo, hi) [SI::s]` otherwise, with `1s`,
  `80ms`, `2 min` literals scaled to seconds; «Probability» on the edges out of a decision is
  `@Stochastic::Probability { p = … }` when every edge carries one, scaled when they do not sum
  to one. A StateMachine becomes a `state def` the state debugger steps — nested states, the
  regions of an orthogonal state as sub-states of a `parallel` state, a submachine state as a
  `state` usage typed by the referenced machine's `state def`, `entry`/`do`/`exit` behaviors,
  `transition first s accept sig : Sig if g do e then t;` with relative time and change events
  as triggers, a deferrable signal trigger as `defer Sig;` — exhibited by an `exhibit state`
  usage of its block. An Operation is an `action def` owned by the block with its parameters,
  its method as body — the method's parameters standing for the operation's at the same
  position under the operation's names — and its conditions as `assert constraint`s; a
  `CallOperationAction` on an
  object performs it on that object through
  `perform action x ::> target.op;`. An OpaqueBehavior or FunctionBehavior whose body is a v2
  expression is a `calc def`; an Interaction whose messages are all signal sends to parts is a
  scenario `action def` of `send`s; a Reception is a comment naming its signal. Absolute time
  events, internal transitions, entry points and history pseudostates, synchronous interaction
  messages and a tool's time variable have no v2 form and stay comments the report accounts for.
- **A send addressed to a parameter, pin or local of the sending action reaches the object it
  holds.** `send new Go() to recipient` under `in recipient : Worker` is delivered to whatever
  object the caller bound, a chain from it (`team.lead`) walked through that object; a binding
  holding no object is refused with a typed error rather than the message dropped. A target no
  binding leads — `this.part`, a port, a name in scope — is resolved as before.
- **`perform action x ::> part.action;` and `exit part.action;` run on the part.** An action
  usage referencing a feature chain performs the chain's last action on the object the chain
  reaches from the performer, as a state's entry, do or exit behavior does; an empty or
  many-valued receiver, a chain ending in no action and a destroyed receiver are refused with
  typed errors.

- **Monte Carlo summaries come across as statistics.** A result snapshot of a «SimulationConfig»
  whose target specializes MagicDraw's `MonteCarloAnalysis` records `N`, `Mean`, `Deviation` and
  `OutOfSpec` beside the observed values; the `-migration-results` sidecar now writes them as the
  snapshot's `statistics` of the observable the analysis binds its `Mean` to, standing for `N`
  runs, rather than as one more run — `deviation` and `outOfSpec` only when the snapshot records
  them, so a missing deviation is not a zero — and notes a summary that is incomplete, counts
  no runs or more than a count holds, holds a statistic over several slots, binds no observable,
  states an `OutOfSpec` that is no count of its runs, or summarises another configuration's.
- **Every run configuration is compared.** `-compare-results` runs a configuration the tool
  stored no snapshot of and prints its statistics under a `tool (no stored result to compare)`
  row; runs one stating no `numberOfRuns` once, as the tool does, under a note saying so; counts
  the runs a summary stands for and pools raw values with summary means; shows only the
  statistics a summary holds; and notes a summarising snapshot another configuration stores under
  the same name with the same statistics as a likely copy, naming that configuration and its
  result location; and notes summaries of one observable whose means lie more than three
  standard errors apart, which cannot be of runs of one and the same model, so the pooled mean
  they are compared by blends them.
- **A run configuration resolves to an inherited classifier behavior.** The `executionTarget`'s
  classifier behavior is looked up through its generalizations, nearest first, and a test-case
  behavior is performed where its scenario is migrated. A target with no classifier behavior at
  any level whose parts hold constraint properties — a parametric configuration the tool solves
  for values — is reported per constraint property, naming its constraint block and whether the
  block's rule is migrated as a constraint or which call of an opaque rule stops it, in place of
  a blanket refusal.
- **Snapshots of a run on another classifier are set aside.** A result location may hold
  snapshots the tool named after a classifier that is neither the configuration's execution
  target nor a general or special of it; they are of another configuration stored in the same
  package, so they are not read as the configuration's results and the sidecar says so, and the
  sidecar carries the notes saying why a configuration runs no behavior.
- **The clock can tick by a fixed step.** `-clock-step <seconds>` and `%clock-step` make every
  wait of a run — `accept after`, `accept at`, a state's timer, a case's timed step — come due at
  the first multiple of the step not before the instant it ends, as a simulation tool's fixed-step
  clock does; `0`, the default, keeps the continuous clock. The step reaches the run, explore,
  check, sweep and standing engines, the external-engine protocol (`clockStep`) and the gRPC
  handlers as the draw policy does; a witness of a stepped run records `clock steps by <seconds>`
  and replays on it. A migrated «SimulationConfig» stating `startTime` ran on the tool's internal
  clock, so the `-migration-results` sidecar records its `stepSize` in `timeUnit` (`1.0` and the
  millisecond, the tool's defaults, unless stated; `(endTime − startTime) / numberOfSteps` when
  those two stand in for the step) as `clockStep`, in seconds, and
  `-compare-results` runs the configuration on it — a unit of no fixed length, a step of zero or
  less, one of more seconds than a number holds or fewer than it tells from none, and an unstated
  unit are noted. The tool's clock started at `startTime` and a run's starts at 0, so a
  `startTime` other than 0 is noted, in the report, the sidecar and the comparison, as offsetting
  every instant read on the clock.
- **A script's console print is left out.** A `print(…)`, `println(…)` or `System.out.println(…)`
  statement of an opaque body writes to the tool's console and changes nothing of the model, so
  the SysML v1 migration leaves it out of the translation, keeps the other statements of the body,
  and notes each print left out as an approximation; a body of prints alone is an empty action.
  A print whose argument assigns, counts, deletes, constructs or calls anything but a function of the table computing
  a value (a Java `equals` counts only on a receiver known to be a string; any other type's is
  that type's own method) could change the model, so it is refused rather than left out; a call
  not in the table, or a print used as a value, is refused as before.

- **The v1 migration translates opaque JavaScript and English bodies into executable v2.** A bounded subset of a v1 tool's scripting language — assignments and compound assignments, `var x = e`, literals, feature paths, arithmetic, comparisons, `&& || !`, a trivial ternary, `Math.min/max/abs/floor/ceil/sqrt/pow` and `java.util.Collections.max/min` — becomes `assign` statements in an action body, an `if` guard on a succession, an attribute default, a `constraint def` expression or a `calc def` body; a guard in English (`TRUE`, a Boolean property's name, `not X and Y`, `a = b`) becomes the `if` it means. Anything outside the subset is refused whole with the offending token in the report, never translated in part, and a body the translator reads is never re-read as v2 syntax; an English body has no calls, and a script label is read only when every word of it names JavaScript, ECMAScript, JS, Rhino, Nashorn or one version (`JavaScript Expression Language` is another language). A Java body's `/` of two whole numbers is `OpenSysMLMathFunctions::quotient(x, y)`, the exact quotient truncated toward zero; its `Math.floor` and `Math.ceil` answer a double, so a `/` after them stays real division, while its `Math.round` answers a long. A Java body's `a.equals(b)` on strings is `a == b`, and its `==`/`!=` with a string operand is refused, Java comparing strings there by identity. A constraint's specification is checked to yield a Boolean: an integer, real, string or enumeration literal is left as a comment naming the value, a string spelling `true`/`false` written as that Boolean.
- **`OpenSysMLMathFunctions::ceiling(x)` and `quotient(x, y)`** join the non-normative math extension library. `ceiling(x)` is the least Integer not less than `x`, the counterpart of `RealFunctions::floor` (`ceiling(-2.5)` is `-2`; the least Integer is a value, where `-floor(-x)` overflows on its negation), and `ErrArithmeticOverflow` at or beyond 2⁶³ or below −2⁶³. `quotient(x, y)` is the Integer quotient of two Integers truncated toward zero (`quotient(-7, 2)` is `-3`), exact over the whole range where `/` answers a rounded Real, `ErrDivisionByZero` for `y == 0` and `ErrArithmeticOverflow` for the one pair whose quotient is 2⁶³, the least Integer by `-1`.
- **Swimlanes give names their object.** Names in a body or guard resolve first against the object the `ActivityPartition` `represents` — a property of the context block, written `this.tcs.i`, through nested partitions, `this.tank.valve.open`, or the block itself — then against the activity's parameters and locals, then the owning block. A partition that resolved a name is reported *mapped*; one with `represents` unset, dangling, untyped, or naming a classifier the activity does not run in falls back to the activity and says so. A part whose multiplicity is not written in numbers may hold one object or several, so a name read through it, a partition representing it, and a partition whose object is reached through it are refused with the part named, never read as one object.
- **The simulation clock is executable.** A body reading the tool's time variable (`simtime`, or the `SimulationConfig.timeVariableName` the model sets) reads `localClock.currentTime`, the standard library's own form, which the runtime evaluates against the run's clock (`Occurrence::localClock`, `Clock::currentTime`), so `Time_Acq_Total = simtime - Time_Acq_Total` is an attribute a run tables with `-observe this.Time_Acq_Total`. A `DurationObservation` between two nodes of an activity becomes such an attribute (`[0..1]`, no default), stamped at the first and assigned the elapsed clock at the second (an initial node is stamped right after `start`, so an observation from it to the final node spans the run; a flow final or a control node no edge leaves as the token reaches it, before `done`), and left without a value by a run that does not reach both; one whose events are not nodes of the activity, name an element the document does not define, or owned outside one, is a comment whose report line says which. An assignment to a clock's `currentTime`, under that name or a redefinition's (`attribute now :>> currentTime;`), is the typed `ErrClockNotAssignable`, and the redefinition reads the clock as `currentTime` does.
- **Report hygiene for leaf steps and symbolic durations.** A `CallBehaviorAction` calling no behavior whose only content is a `DurationConstraint` is a leaf step reported *mapped*, its wait written; only a call with pins or an unresolved behavior stays *unmapped*, the pins named. A duration bound that names a property (`ditSetup s`) resolves like any other name and is written `accept after this.tcs.ditSetup [SI::s]`.
- **An object typed by a behavior runs no classifier behaviors of its own.** A `perform`/`exhibit` member of an `action def` is a step of the performance that runs it, not a behavior bound to the performance occurrence, so a migrated workflow's sub-activities run once each, as the performer.

- **A run resolves its random draws under a policy.** `-draws random|min|max|average` and
  `%draws` state how every `RandomFunctions` call of a run resolves: `random` (the default) draws
  from the seed as before; `min`, `max` and `average` take each call's least, greatest or mean
  value — `uniform(1, 80)` is `1`, `80` or `40.5`; `uniformInteger(1, 6)` averages to `4`;
  `triangular` to `(lo + mode + hi) / 3`; `normal` averages to its mean and, unless its
  deviation is zero, has no `min` or `max`, which is a typed error naming the call — and need no seed, so a random duration
  becomes a fixed one and `-runs`/`%runs` run without `-seed` under a fixed policy (`%runs <n>
  <action>`, the seed left out). Weighted decisions draw from the seed whatever the policy and
  take their most probable branch unseeded. The policy is a property of the run's context, so
  it reaches the analysis engines, the wire (`"draws":"max"`) and gRPC as the model seed does;
  a witness records it as `draws by max` and `replay:` reproduces the run under it, refusing a
  witness whose draws the recorded policy could not have made; a witness naming a fixed policy
  and recording no draw — an external engine's schedule, which carries choices alone — leaves
  them to the policy. A conformance case pins it with `"draws"`.
- **A «Probability» that names a property is a feature reference.** The SysML v1 migration
  writes `@Probability { p = ProbabilityBTOOP; }` when the tag names a property visible from
  the activity or its context block, by name or id, instead of the property's default, so the
  object the behavior runs on decides the branch weights; a tag naming nothing visible, a
  private property, a non-numeric or a multi-valued property is a report entry with the reason.
  A feature-valued weight is type-checked against `Probability::p` where it is written, and the
  checks lowering makes of constants — each in `[0, 1]`, the set summing to one — are made of
  the values read when the decision is reached, each a typed `ErrBranchWeights`.
- **Simulation run configurations migrate.** A MagicDraw «SimulationConfig», recognised by its
  profile's provenance, becomes an `action def` holding its `executionTarget` individual as
  `part target` and performing the target's classifier behavior on it, annotated
  `@Simulation::Configuration { runs = …; draws = DrawPolicy::…; timeVariable = …; startTime = …;
  stepSize = …; timeUnit = …; parallelForks = …; }` — a new non-normative library beside
  `Stochastic` that records how the tool ran the behavior and applies none of it — with every
  setting of no v2 meaning kept in a comment. The configuration, its target and result
  instances and the probability edges it reads are mapped rather than unmapped; a target the
  migration did not write, one that is no part, a state machine or a classifier with no
  behavior is reported with the reason. The outcome of an action spells `<part>.<attribute>`
  for the one object each of its own parts denotes, so `-observe target.duration` reads
  the target's attribute after the run.
- **The tool's results come across, and a harness compares them.** `-migration-results
  <file>` writes a JSON sidecar indexing the tool's result-snapshot instances — typed, or
  classifier-less under a configuration's `resultLocation` with slots of features of one lineage
  of blocks the target is of, which types them; a classifier-less instance anywhere else stays
  unmapped — per configuration and observable; `sysml <migrated>.sysml -compare-results <file> [-action
  <configuration>...] [-runs <n>] [-draws <policy>] [-seed <s>] [-observe <stored>[=<feature>]...]`
  runs each configuration with its recorded run count and policy, or the ones given, and
  reports the tool's and OpenSysML's min, mean, p50, p90 and max of each observable with the
  relative difference; a configuration with no stored snapshots, a stored observable no run
  holds and a non-numeric one are reported, never left out, and a run that fails fails the
  comparison with its error under the table; an `-action` no configuration bears fails the
  check beside the ones compared. The numbers are reported as run.

- **The SysML v1 migration writes interactions, receptions and the rest of a state machine
  executably.** An Interaction owned by a block is a scenario `action def` of every message
  kind: a signal send, a `synchCall`/`asynchCall` of an operation as a typed perform on the
  lifeline's object — `perform action spin : Motor::Spin ::> drive.motor.spin { in rpm = 30.0; }`,
  the arguments bound to the operation's `in` and `inout` parameters by name or position — an
  unnamed argument taking the next parameter no named one claims — each
  with the parameter's direction so an `inout` value is written back — and a `reply`
  as the assignment of the call's result to the caller lifeline's attribute; a lifeline is
  resolved to the feature path through the block's parts, ports and references or to an `in`
  parameter, and `alt`/`opt`/`loop`/`par` fragments are `if`/`for`/`while`/`fork` structures
  when their guards parse and resolve, a reply answering the latest open call of its operation
  between its lifelines, never one another alternative or a concurrent operand made; a duration
  constraint between two messages with steps
  between them is a wait forked after the earlier step and joined before the later, so those
  steps count toward the interval, and one whose interval is open on one side (a min with no
  max, a max of `*`) is reported with the bound it lacks rather than written as a wait at the
  bound it has, the min beside an expressionless max of a MagicDraw document — its encoding of
  a one-valued `{60s}` — excepted. A lifeline or guard that does not resolve, a create or
  delete message and a message-less timing trace are refused with the reason, as is a call
  or signal message leaving an `in` parameter or signal attribute — inherited ones included —
  with no default and a lower bound above zero unbound; two parts of
  one type are two paths, so a lifeline standing for a part of that type is ambiguous. A Reception is
  an `action def` of the block that accepts its signal, runs its method with the signal's
  attributes bound to the method's parameters of the same name and accepts again, performed by
  every object of the block from creation — a method that is also the method of an operation runs
  as that operation's `action def` — so a signal sent to the object at any time runs the
  method against the object; where the signal arrives at ports of the block, the accept is forked
  into one loop per port, `accept … via <port>`, beside the one from the object; where the method requires a value no attribute supplies,
  or a same-named attribute does not fit its parameter's type or multiplicity, the reception only
  accepts the signal and says so. State machines gain transitions across regions and
  nesting levels named by path, `junction`/`choice`/`fork`/`join`/`history`/`deep history`
  pseudostates, entry and exit points of a submachine as states of its `state def` addressed
  by path, internal transitions as self transitions where re-entry is not observable (written
  with no target, as some tools do, they stay in their source; one targeting another vertex or
  leaving a pseudostate is refused), and
  absolute time events as `accept at <instant>` over a `Time::TimeInstantValue` attribute of
  the behavior. A `CallOperationAction` over a port performs the operation on the part a
  connector of the caller's block joins to that port, the way connector paths resolve.
- **The migration report counts what nothing refers to apart from gaps.** An event no trigger
  names is skipped as a model element nothing refers to, counted apart from profile and library
  content in the summary line, rather than reported as unmapped.
- **A typed action usage that references a feature chain performs the chain on the object it
  reaches.** `perform action x : Def ::> part.action { in p = v; }` runs the part's action with
  the part as performer and binds the callee's inputs from its body, and an accept payload is
  visible from the body of a typed usage in the same action body, so a nested typed action can
  read the accepted message (`in level = msg.level`).
- **A migrated state values its entry and do parameters from the signal that enters it.** When
  every transition into a state accepts the same signal and its attributes fit the behavior's
  parameters in order, type and multiplicity — the attributes the signal inherits from its
  generals counted with its own — the `state def` keeps the signal in an item
  (`item setPoint : SetPoint;`) each transition assigns and the parameters read
  (`in target : ScalarValues::Real = setPoint.level;`); a state entered without a signal or with
  one that does not fit is reported with the transition or attribute that is the reason. A
  region holding no vertex is skipped when the machine's states are named as when they are
  written, so a machine whose other region is populated is written inline and a transition
  across nesting levels names its far end by a path that exists. A
  trigger naming no port is also written accepting via each port of the owner its signal arrives
  at — one the document's connectors and delegations carry a send of it to, one an item flow
  conveys it to, or one whose type (generals and realized interfaces included) declares an inward
  flow property or a reception of it — with the ports that declare nothing and receive no send
  reported as left unrouted, and an activity whose required input pin only
  parameters nothing values flow into is reported as never firing instead of written to wait.
- **A `via` path can start at a bound reference, and delegated, redefined and untyped ports
  route.** `send … via ctx.p` from a behavior whose `ctx` is bound to another object leaves that
  object's port even when the performer owns a feature of the same name, the binding shadowing
  it as it does in every other expression, while `via this.ctx.p` stays the performer's own,
  and a state transition's `accept … via ctx.p` resolves its path the same way through the
  machine's parameters; the behavior's own connectors are read the same way, so a
  `connect ctx.p to snk.local` between two bound references carries that send beside the
  connections of the object holding the port, and an addressed `send … via ctx.p to m`
  names a machine of the object the path is re-rooted to;
  a part's port is known to the connectors its type inherits under the name the
  part was declared with before redefinition; a `ref` usage holds what is bound to it rather than
  an object of its own; and an untyped `port` materializes as a `Ports::Port`, so a binding
  connector can join it and a signal sent inward over it reaches the bound part's machine.
- **A migrated call or send that v1 fires without a required value keeps its place and performs
  nothing.** A call passing no argument for a parameter that must hold a value — a parameter of
  the operation's action def, which declares the operation's parameters and then those its method
  adds, so a call binds and is checked against exactly what is declared — or a call or
  signal send passing none for a signal attribute that must, one passing a pin of a type the attribute cannot take, or one whose pin is fed only by flows no value travels — from a parameter nothing values,
  an unmigrated opaque or value specification action, a callee whose own activity gives that
  `out` parameter no value, judged through any depth of nesting, or a call's result pin past
  the callee's `out` parameters, which stands for none — is written as an empty action carrying the token,
  with the reason in its comment and report line, and the object flow is kept as a comment
  rather than written from a feature that will hold nothing. Control and buffer nodes only
  object flows lead to route their values from source to pin, a control node no edge leaves
  ends the token as `done` does, and an action fed by an object flow from outside its control
  path waits for the value only when the producer runs on every pass of the surrounding loop.

- **An object is validated as a whole.** `%validate <object>` at the prompt, `-validate=<object>`
  on the command line and the `ValidateInstance` RPC evaluate every assertion about an object and
  the objects it holds — each `assert constraint` the carrier's type declares or inherits, each
  requirement usage it carries, and each `satisfy` assertion whose subject is in the tree — against
  the concrete object carrying it, nested parts and every element of a collection included, and
  report one verdict per assertion per object, labelled by the path from the object validated
  (`car.wheels[2]`), then one verdict about the object itself. A condition that evaluated false is
  violated; one that could not be evaluated is undecided with the reason, and leaves the object
  not shown valid rather than valid; a walk cut short by an object graph without end is reported
  bounded and not valid; an object no assertion is about decides nothing and is not shown valid
  either. A constraint declared without `assert` is not swept, and a symbol with no object to
  validate — a package, an attribute — is refused as the wrong kind. The object is named
  as every prompt command names one: by the name it was instantiated under, by id, or by a path
  into what it holds. `-validate` without an object still checks only that the model analyses
  cleanly. Over the wire the response carries each verdict's `instance_path` and a `summary`
  verdict of kind `object`; the Go client answers `Client.ValidateInstance` with a `Validation`
  (`Valid()`, `Violated()`, `Bounded`) and the Python client `Model.validate_instance` with a
  `Validation` (`valid`, `violated`, `undecided`, `bounded`), each verdict carrying its
  `instance_path`.

- **A worked example of verdict queries.** `examples/verdicts-demo/` holds a rover whose
  constraints, requirement, `satisfy` and verification case are read as a `Verdicts(...)` table:
  twelve rows over the declared object, then the same queries over the object a session holds
  after a drive, with a violated satisfaction beside a passed verification and an undecided
  constraint beside a violated one. The walkthrough spells out what the table checks that
  evaluating one expression does not.

### Changed

- **The RDF graph a model converts to states its relationships as elements.** Turtle and `api-json` output now carry the metamodel's relationship elements alongside the collapsed properties — graphs written by earlier releases still read, but a re-conversion of the same notation states more subjects (the materialized elements), and the same holds for the goldens the convert fixtures pin. Keyword-headed usages that wrote the plain metaclass now write the usage's own — `perform action` is `PerformActionUsage`, `exhibit state` is `ExhibitStateUsage`, `include use case` is `IncludeUseCaseUsage`, and a bare or metadata-body member that takes no kind keyword is a `ReferenceUsage` rather than the kind's usage.

- **A braced `entry { … }`, `do { … }`, `exit { … }` or transition `do { … }` block is one anonymous action.** The block parses as the one action usage SysML.xtext reads it as, the same tree as `entry action { … }`, rather than as one action per statement. A declaration inside the block (`entry { attribute k : Integer = 2; assign log := k; }`) is local to the block and shadows the state's, and is not visible outside it; a `terminate;` in the block resolves to the block's own performance and ends the whole block; a `do` block runs one statement a round as an inline do body does, so orthogonal regions still interleave statement by statement and a transition out of the state drops the rest of the block. `-trace` output changes shape: the block's statements are indented under one `stmt action body` line and the per-statement behavior lines and the `terminate …: ended before it began` lines of the statements after a `terminate` are gone. In the RDF mapping a braced block is the nested anonymous `ActionUsage` — a state subaction's one member, a transition effect's under `sysx:hasEffect` — no longer its statements under the membership with `sysx:hasBody` or `sysx:bracedEffect`; a Turtle graph written in that older shape is refused as unsupported rather than read back as something else. The formatter and `sysml -convert sysml` keep the spelling as written.

- The Cameo v1 export is now written under `~/.opensysml/cameo-exports` instead of the system temp directory.

- **The Python, Node, Java and Rust clients moved from `clients/` to `client/`, beside the Go client.** `client/java`, `client/node`, `client/python`, `client/rust` and the shared `client/release-digests.json` replace their `clients/` counterparts; package names, Maven coordinates, crate names and the Go import path `client/opensysml` are unchanged. Build targets, protobuf generation outputs, CI jobs and the changed-area detection follow the new paths.

- **A completion enabled by a region's entry stays a run-to-completion step of its own; the
  design that would have drawn its firing inside the entry front is closed without code.**
  Enumerated against the PSSM traces the five tests held on it admit
  (`docs/internals/design/region-order-scheduling.md`), that rule reaches every admitted set
  only by also reaching orders the suite refuses, and the sets turn out to want three
  different things: *Entering 010*, *Entering 011* and *Junction 005* want a UML initial
  transition's *effect* run as part of the region's default entry, which SysML v2 can spell
  only as the effect of a completion transition out of a start state — a translation limit,
  not a scheduling one; folding the effect into the target state's entry action was run
  against the three and refused (it runs the effect on every entry of the state, which
  *Entering 010* refuses, merges two behaviors into one unit, and has no target where the
  initial transition ends at a junction); *History 001-C* and *History 002-B* register
  incompatible orders for structurally identical halves, each contradicting the
  specification's own account of the test (`docs/project/omg-issues.md` gains the entry);
  *Terminate 002*'s remaining trace is a do step against a sibling's entry unit. The one
  runtime gap the enumeration confirms — under `reverse`, `seed:<n>` and `explore`, the
  completion events two regions' entries generate are dispatched in region declaration order
  rather than the order the regions were entered — is recorded for a runtime change of its
  own, since it moves no test alone. The alignment note and the referee record carry the
  adjudication; no bucket, golden or baseline moves (51 pass, 13 fail, 38 not expressible,
  1 differs by design).

- **`Diagnostic` and `Severity` moved from `internal/core/passes` to the leaf package `internal/core/diag`.** Every layer that reports a finding — the parser's warnings, the validation passes, the runtime's informational guard and tool notes, the editor and the service — now spells `diag.Diagnostic` and `diag.SeverityError`/`Warning`/`Info`/`Hint`; `passes` keeps no alias. The runtime no longer imports the validation suite for its diagnostic type.

- **The PDF document backend sits beside the renderer it drives.** `internal/docpdf` is
  `internal/core/docpdf`, next to `internal/core/docrender`, whose Markdown it converts; nothing
  changes for `sysml -render-document -doc-form pdf`.

- The design record for exception handlers, `docs/project/exception-handlers.md`, closes the compliance mapping's last UML-referenced action item as not a SysML v2 construct: the language spells no raise, handler or propagation, and a failure is handled with a result routed by `decide` or a failure signal accepted beside the work and a `terminate`. The compliance mapping, the roadmap and the behavior guide say so, and no runtime work follows.

- The design record for concurrent per-element performance ("expansion regions"), `docs/project/expansion-regions.md`, closes the roadmap item: the iterative form is `for`; the parallel form is not SysML v2. The compliance mapping's bullet and the roadmap say so, and no executor work follows.

- **The refusal of a session object under exploration says what to name instead.** `"#2.ground" names an object of this session, which an exploration does not run on: each explored run creates its own objects, so name a declaration to instantiate, or a path from one to an object it holds (Assembly::part.nested)` replaces the message that refused every dotted name; a machine named alone with no run object exhibiting it is refused naming the run's objects, not the session's (`no object of the explored run exhibits …`).
- **A check's outcome carries the attributes the performing objects hold.** The outcome table and the `-check-diverge` comparison of a behavior run on an object include that object's attributes (`received = 2`) beside the behavior's own, as `this.<feature>` names them, so two schedules that leave a nested part's counter apart are two outcomes; a `-check-diverge` that selects another feature of the object (`this.mode`, an item) tells them apart the same way. The service's explored `ExecuteAction` and `ExecuteState` outcomes carry the performer's attributes in `outputs` under the same names.
- **An action executed on an object performing it runs that one performance.** `ExecuteAction` with a performer whose declaration performs the action (`perform action fill`) answers with the performance the object started when it was created, its results and the object's attributes as that run left them, where it started a second run of the action on the object and wrote it twice. Inputs for such an action are refused (`inputs for a performed action`), as its declaration binds its arguments; an object performing the action under several usages is refused naming them (`ambiguous action: the object performs Fill as morning and evening`), as a machine exhibited twice over is.

- **Conversions are driven from `internal/core/convert`, not from the RDF mapping.** The format names, `Convert`, `ConvertTolerant`, `SysMLElement`, `Migrate`, `SysMLToRDF` and `SyntaxError` moved out of `internal/core/export` into a conversion entry point that parses notation and runs the SysML v1 migration before handing the tree or graph to the mapping; `internal/core/export` keeps `ToRDF` and `ToSysML` and no longer imports the migration. `sysml -convert`, `%save`, `%print` and the service's `Convert` behave as before, and a hygiene test keeps the mapping free of the migration.

- **An edge is labelled by its name only when it has no text of its own.** A transition's trigger, guard and effect, a succession's guard and a flow's payload take the label, whatever the edge is named; a plain named succession, connection, binding or completion transition is labelled by its name, as the graphical notation draws them.
- **A comment on a state or a named transition says `about` it.** A state, the parallel state standing for an orthogonal region, and a transition written under its v1 name are members a qualified name reaches, so a comment annotating one is written `comment about <member>` rather than unanchored, and the report records it `mapped`.

- **The conformance mode lives with the diagnostics it grades.** `internal/core/conformance` is folded into `internal/core/diag`: `conformance.Mode`, `ModeDefault`, `ModeStrict`, `ModeOf` and `ParseMode` are now `diag.ConformanceMode`, `diag.ConformanceDefault`, `diag.ConformanceStrict`, `diag.ConformanceModeOf` and `diag.ParseConformanceMode`. The mode's spellings, `default` and `strict`, are unchanged.

- **Atomic file replacement lives with the other file-level primitives.** The one-function `internal/fsutil` package is folded into `internal/core/source`: `fsutil.Replace` is now `source.ReplaceFile`, with the same rename-over-target semantics.

- **Normative library ids are minted in `identity` itself.** `internal/core/identity/normative` is folded into `internal/core/identity`: `normative.Language`, `KerML`, `SysML`, `ElementID`, `OwningMembershipID`, `EscapeName` and `NamespaceURL` are now `identity.Language`, `identity.KerML`, and so on. No behavior changes.

- **A symbol's source origin is read off the symbol.** `internal/core/provenance` is folded into `internal/core/symbols`: `provenance.Origin` is `symbols.Origin`, `provenance.Symbol(sym)` is `sym.Origin()`, and `provenance.Node` and `provenance.At` are `symbols.NodeOrigin` and `symbols.OriginAt`. No behavior changes.

- **Quick-fix edits live with the diagnostics that carry them.** `internal/core/quickfix` is folded into `internal/core/diag`: `quickfix.Fix`, `Edit`, `Insert`, `InsertLine` and `Replace` are now `diag.Fix`, `diag.Edit`, `diag.Insert`, `diag.InsertLine` and `diag.Replace`. No behavior changes.

- **Rename conflict checking lives with the rename edit.** `internal/core/rename` is folded into `internal/core/edit`: `rename.Occurrence`, `rename.Conflict` and `rename.Check` are now `edit.RenameOccurrence`, `edit.RenameConflict` and `edit.CheckRename`. No behavior changes.

- **The fUML referee driver source lives under `scripts/fuml-driver/io/opensysml/fuml/`.** The path now matches the `io.opensysml.fuml` package the source declares; `scripts/fuml-expected.sh` compiles it from there and its output is unchanged.

- **The test-suite figures are counted from the tree when the documentation site is built, never committed.** The conformance-case, golden-AST, golden-trace, negative-parser, robustness, gRPC and `Test`-function counts in the compliance map's test inventory are counted by `cmd/doc-counts` the way the gates enumerate them — the conformance cases through `tests/fixtures`, which the runtime and gRPC conformance tests read too — and rendered by the site build (`scripts/mkdocs_suite_figures.py`), which now republishes on every push to `main` so a release's figures follow its tree; in git each block names what is counted, and `go run ./cmd/doc-counts -check` refuses a figure typed into one. A branch adding a test or a fixture therefore no longer rewrites `README.md` or the compliance map, so concurrent branches stop conflicting on them. The README's status table names the gates without restating their counts; the only suite figure still committed is whether every conformance case passes, which moves with `known_failures.txt` alone. The hand-typed figures the generated ones replaced lagged the tree (889 conformance cases where it carried 904); the tests-and-subtests total of a run, which only a run can state, is not quoted.

- **The RDF exporter no longer carries the engine model forms.** The `graphs:1` form of the lowered action and state graphs, `GraphsVersion`, the `sources` form and the form refusals live in `internal/core/analysis/modelform`, in the execution layer beside the analysis framework that hands them to external engines; `internal/core/analysis` no longer imports `internal/core/export`. The bytes of the `graphs:1` form are unchanged and now pinned by goldens.

- **`sysml -help` and the manual page list the options by task.** The 84 flags come under headings — General, Evaluating, Checking a model, Running behaviors, Analysis engines, Checking every schedule, Converting and migrating, Compiling natively, Rendering views, Rendering documents, Styling HTML documents, Syncing against a repository, Diagnostics and profiling, Deprecated — instead of in one alphabetical list. Each is spelled once with its shorthand folded in (`-e, -eval <expr>`, `-o, -output <file>`) and its argument named (`<format>`, `<duration>`, `<policy>`) rather than typed (`string`, `value`); a description is a wrapped line or two, with the detail it carried moved into the section on that mode (`Checking a model`, `Running actions and state machines`, `Checking every schedule`, ...), and the three `-pdf-*` spellings sit under *Deprecated* pointing at their `-doc-*` names. The examples come first. A flag added without a heading fails a test.
- **A mistyped flag prints the error, the synopsis and `Run 'sysml -help' for the options.`** — three lines, rather than the whole help after the error.
- **`%help` groups every REPL command under a heading and wraps its description.** The commands that had none come under *Session*, *Settings*, *Analysis engines* and *Checking every schedule*; a description wraps into the column after its command, and a signature too wide for that column takes a line of its own. `%exit` is noted beside `%quit` rather than listed as a command.

- **The Java client's `Condition.equal` is renamed `Condition.equalTo`.** The old name collided with `Object.equals` on every use; `Condition.equalTo(property, values)` is a drop-in rename.

- **The simple name a qualified name ends in is computed in one place.** `symbols.LastSegment` is exported and the identical copy in `suggest` is gone; `ast.QualifiedNameOf` builds a qualified name from its segments, replacing the copies in `solve` and the resolver tests. No behavior changes.

- **Moved the validation packages to `internal/check`.** `passes` (with `kit`, `behavior`, `document`, `diagram` and `identity`) and `edit` now live under `internal/check/`.

- **Moved the document packages to `internal/doc`.** `queryexec`, `docir`, `docrender` and `docpdf` now live under `internal/doc/`.

- **Moved the execution packages to `internal/exec`.** `runtime`, `solve`, `smt`, `analysis` (with `enginewire` and `modelform`), `engines` and `objref` now live under `internal/exec/`.

- **Moved the frontend packages to `internal/frontend`.** `protoconv`, `repl`, `lsp`, `grpc`, `stdiorpc` and `usage` now live under `internal/frontend/`.

- **Moved the semantic IR packages to `internal/ir`.** `lower`, `queryplan`, `docplan` and `view` now live under `internal/ir/`.

- **Moved the semantic packages to `internal/semantic`.** `symbols`, `suggest`, `resolve`, `semantics`, `identity`, `highlight` and `query` now live under `internal/semantic/`; `query` declares the namespace IRIs it needs rather than importing the RDF vocabulary.

- **Moved the foundation and syntax packages to `internal/syntax`.** `source`, `ast` (with `astcodec`), `pack`, `diag`, `lexer`, `parser` and `format` now live under `internal/syntax/`.

- **Moved the translation packages to `internal/translate`.** `rdf` (with `ontology`), `export`, `migrate`, `convert`, `xmi` (with `sysmlv1`), `codegen` and `interop` (`flexo`, `reposync`) now live under `internal/translate/`.

- **Moved the workspace packages to `internal/workspace`.** `model`, `libs` (with `errata`), `project` and `envvar` now live under `internal/workspace/`.

- **The SysML v1 migrator writes a call behavior action that names no behavior yet owns pins as a declared stub action.** `action x { in a : T; out r : U[0..1]; }` takes its parameters from the pins, typed and bounded as they are, keeps its place in the activity's successions, and is reported as approximated, computing nothing: each output is declared admitting no value. The flows into and out of its pins and the «Allocate» relations to it are written by the existing paths instead of being dropped as ends of an unmigrated action, an allocation to a node in an operation's method naming it under the operation. A pin a called behavior has no parameter for is still refused, the reason now naming the behavior and its parameters of that direction, while the call itself is kept. At run time a flow out of an unassigned output declared admitting no value carries nothing, so the target reads the parameter empty rather than the run failing; a required output or input left without a value is still a typed error.

- **The notation-text helpers moved from `internal/core/lexer` to `internal/core/source`.** `NameText`, `QualifiedNameText`, `StringValue`, `StringText`, `UnrestrictedNameText`, `CommentBody`, the keyword sets (`Keywords`, `IsKeyword`, `IsKeywordIn`) and `IsIdentifier` now live beside the source files they read and write, so the type system, the runtime and the exporter no longer import the scanner to spell a name or read a comment body; `internal/core/semantics` imports `lexer` in no file. A layering test in `tests/hygiene` assigns every package to a layer of the package-layering plan and permits exactly the cross-layer imports that exist today, so a removed edge cannot return unnoticed.

- **The migration and referee XMI readers now share one parser.** The generic element tree in `internal/core/xmi` feeds both the SysML v1 migration reader and the fUML/PSSM referees, and the referees now accept the schema.omg.org and unversioned XMI namespace forms.

- **PDF output is laid out from the HTML document backend's page.** `-doc-form pdf` hands
  WeasyPrint and Prince the same semantic HTML `-doc-form html` writes, under the backend's
  default stylesheet and a print stylesheet layered after it, instead of a page reconstructed from
  the rendered Markdown; pandoc keeps reading the Markdown. `-html-theme`, `-html-no-default-css`
  and `-html-css` now reach a WeasyPrint or Prince PDF exactly as they reach HTML — a user
  stylesheet is unlayered, so it overrides the default and print layers without `!important` — and
  are refused with a typed error for pandoc, whose page is its own. A sheet's relative `url()`
  and `@import` references resolve against the PDF's directory under every engine, as a page's
  resolve against the page's. Tables, figures, formulas and cross-references carry their
  `sysml-*` classes and `data-*` attributes into the PDF's HTML.
- **Markdown output no longer carries a caption marker.** The `<!-- caption -->` comment the
  Markdown backend wrote ahead of a table's or diagram's emphasized caption is gone; the caption
  stays an emphasized paragraph. Pandoc recognizes captions by matching those paragraphs against
  the document's captions in order, so an emphasized paragraph elsewhere stays prose. A caption
  is written without its surrounding blanks, which CommonMark would otherwise read as literal
  asterisks or as indented code; a blank caption writes no paragraph.

- **The SysML v1 migration no longer writes MagicDraw's property-kind markers as comments.** «ValueProperty», «PartProperty», «SharedProperty», «ReferenceProperty» and «ConstraintProperty» applied from MagicDraw's SysML customization profile to a UML Property, carrying no tag, say only what the usage's keyword (`attribute`, `part`, `ref part`, `constraint`, `in …`) already says, so the `/* applied stereotype «ValueProperty» */` body they forced on every such property is gone and the declaration is one line. The marker is recognised by the provenance of its profile, as «ConstraintParameter» already was: a same-named stereotype from a user profile is still written as a metadata usage or comment, a marker that carries a tag is still written with it, and a marker on a property written as another kind is kept. Report verdicts are unchanged.

- **The conversion between runtime values and the API's protobuf messages is its own
  package, `internal/protoconv`.** The instance-graph serialization (`InstanceGraphToProto`,
  `GraphBounds`) and the value conversions in both directions moved there out of the gRPC
  service, so the REPL's `%features … json` and the Go client's value marshalling no longer
  link the service or its Connect transport; `sysml` reaches gRPC-Go only through the
  generated service stubs that share the `api/proto` package with the messages. The
  `features` JSON is unchanged.

- **Every failing PSSM test is attributed.** The fourteen failures the referee's record tabled
  as unadjudicated each cite either a *differs, v2 silent* row of the alignment note or an open
  finding against the runtime: *Junction 004* and *Join003* report on SM32 (a junction or join
  with no way through), *Join001* and *Transition 019* on SM34 (a join — where its owner is left
  and in what order its segments fire, now recorded there), nine cite the new finding 9 (the
  order in which orthogonal regions are entered, exited and stepped against a dispatch is not a
  recorded choice point, so `explore` never reaches the other interleavings) and *Junction 005*
  the new finding 10 (a segment leaving a junction inside a composite state runs its effect
  before the composite is entered). No bucket count moves; the record tables the twenty
  failures the first baseline left unadjudicated by root cause and where each landed.

- **The PSSM referee's refusal of a guard that acts on the model is settled, not provisional.**
  *Choice 005* traces its four guards to show when a junction's and a choice's are read.
  Recording the runtime's own guard reads as the referee's observable was tried and refused:
  the suite reads the junction on the entered state's default entry before the incoming
  transition's effect and the state's entry, where the runtime and `StatePerformances.kerml`
  read a transition inside a state after its entry, and the runtime's trace keeps only the
  first read at each vertex, the others rolled back with the probe that made them. A `calc def`
  with a side effect is refused too: a v2 expression is pure, and UML 2.5.1 §14.5.11 calls a
  guard with a side effect ill formed. The alignment note tables the three candidates against
  the admitted trace, and no bucket, reason or trace moves.

- **The PSSM referee classifies a guard whose behavior acts on the model as having no
  translation.** A SysML v2 guard is a Boolean expression; a UML guard whose activity calls
  `trace(...)` before returning its value has no spelling, and the translation used to carry the
  value alone and silently drop the call. The classifier now names the construct (*guard side
  effect*), the emitter refuses it, and *Choice 005* moves from `fail` to `not-expressible`
  (17 `fail`, 40 `not-expressible`), adjudicated in `docs/project/pssm-referee.md`.

- **The Python client is released with the core, from the same `v*` tag and at the same version.** `v0.9.0` publishes `opensysml` 0.9.0 to PyPI and puts the wheel and sdist on the GitHub release, listed in the signed `SHA256SUMS.txt` beside the binaries, so `pip install opensysml==0.9.0` with `OPENSYSML_GRPC_VERSION=v0.9.0` is the package and the `sysml-grpc` that were tested together. The release workflow fails before building anything when `client/python/opensysml/_version.py` does not declare the tag's version (compared as versions, so the SemVer tag `v0.9.0-rc1` names the PEP 440 `0.9.0rc1`); the separate `opensysml-v*` tag and its `release-python` workflow are gone. The PyPI upload runs only after the GitHub release is published, so a package version never exists without its release. Re-running a published tag still replaces the GitHub assets, while the PyPI upload — which cannot be repeated — fails by design for a version the index already has.

- **Qualified-name rendering and declaration-body lookup each have one home.** `ast.QualifiedName` gained a `Text` method ("A::B::C", empty for a nil name) and `ast` a `DeclMembers` function returning the body of a definition or usage; the per-package copies of both helpers in the lowering, validation, runtime, query-plan, document-plan and symbol packages are gone. No behavior changes.

- **Expression roots and operands now use standard ownership vocabulary in RDF.** Roots are carried by `OwningMembership`/`FeatureValue` and operands by `ParameterMembership` input Features and FeatureValues, while legacy positional graphs remain importable.
- **Connector ends now use standard `connectorEnd` ownership.** EndFeatureMembership and ReferenceUsage nodes carry end references and chains, with binary source/target features and transition source/target predicates.
- **Legacy RDF shapes remain compatible.** Earlier argument, positional end, and transition endpoint predicates still import, while conflicting old and new representations are refused.
- **Connector-end references use `ReferenceSubsetting`.** End targets are emitted through standard relationship ownership; interim `sysml:references` remains accepted on import.

- **A state's inline `do` body is interrupted between its statements.** `do action { s1; s2; s3; }` runs one statement per do round — each statement of a `for` or `while` iteration, of a nested block or branch its own, one step of a flow the body states (each of its tokens one node) — so a transition out of the state triggered after `s1` leaves `s2` and `s3` unrun, as §7.18.3 has the source state's do action interrupted "if it is still being performed"; the `exit` behavior runs as before. The inline bodies of orthogonal regions interleave statement by statement, as the braced `do { … }` form does, the order within a round the same do-round choice point. A body paused mid-loop when its state is left drops the rest of the iteration and the iterations after it with nothing kept on the clock, and a non-terminating inline body still ends with the do-step budget.

- **Runtime and gRPC robustness cases are registered per feature, not in one shared function.** A feature's failure-mode subtests live in their own `robustness_<feature>_test.go` under a `TestRuntimeRobustness<Feature>` (or `TestGRPCRobustness<Feature>`) function, and the documentation counters sum the first-level subtests across every `TestRuntimeRobustness*` and `TestGRPCRobustness*` function, so two branches adding cases no longer edit the same lines of `robustness_test.go`. `go test ./...` runs every case as before.

- **Buf configuration now lives in `api/proto/`, and the manual pages now live in `packaging/man/man1/`.**

- **The runtime no longer imports the parser.** The notation text a run reads — a witness file's input values and the unit a tool answers in — is parsed through the `runtime.ExpressionParser` the frontend installs on the model (`Model.SetExpressionParser`, `parser.ParseOneExpression`); reaching either with none installed is the typed `runtime.ErrNoExpressionParser`. `internal/core/runtime` no longer depends on `internal/core/parser`, and a hygiene test keeps it so.

- **The runtime selects call overloads through the semantic model, with the argument typing its caller installed.** `runtime.NewModel` no longer installs the checker's argument typing itself; every path that builds a runtime model constructs its semantic model with `passes.NewTypedModel`, and a call selected on a model carrying no typing fails with `runtime.ErrNoArgumentTyper` instead of selecting by arity alone. The invocation AST helpers `InvocationArgs` and `ChainCallee` moved from `passes` to `semantics`.

- **The SonarCloud scan fits its container and retries transient failures.** The scan job's memory budget is now split between the analysis JVM (5.5 GB), the JS/TS sensor's Node process (1 GB) and the launcher (256 MB) so the 8 GB `large` container is not OOM-killed — a silent `EXECUTION FAILURE` exit 3 at the JS/TS sensor — and a `when: always` step prints the cgroup memory counters so an OOM kill names itself. The scan runs inline instead of through the orb, retrying once on transient SonarCloud API or network errors and storing each attempt's log as an artifact, and it no longer waits on the Go race run, so a race-test failure no longer hides the analysis.

- **Sorted map keys come from the standard library.** The seven private `sortedKeys` helpers in the symbols, validation, SMT, view, runtime and interop packages are replaced by `slices.Sorted(maps.Keys(m))`. No behavior changes.

- **The declared errata overlay now covers the bundled standard library.** `internal/errata`
  accepts entries under `internal/core/libs/stdlib` beside the example corpora, and the nine
  dimension defects the expression type checker reports in the published `SI.sysml` and
  `USCustomaryUnits.sysml` are its entries, each with a citation, a derivation and the published
  line it must still match. Three have one reading with the declared dimension and carry a
  correction (`eV*m^-2/kg` → `eV*m^2/kg`, `m^3/C*m^3*s^-1*A^-1` → `m^3/C`, `229835/900 [K]` →
  `(229835/900) [K]`); the other six are documented without one. The library a process loads
  (`libs.BundledSource`, `libs.DefaultSource` and the generated `stdlib.snapshot`) is the
  published text with the corrected lines substituted on read — the vendored bytes are never
  edited, `libs.EmbeddedSource` still serves them as published, and a directory named by
  `OPENSYSML_LIBRARY_PATH` is read as it stands. A library read fails rather than serve the file
  uncorrected when a declared line no longer matches, and two entries naming one line are refused.
  `TestExprTypeCheckPublishedStdlibDefects` pins all nine findings over the published text;
  `TestExprTypeCheckNoStdlibFalsePositives` pins exactly the six uncorrected ones over the bundled
  library. Derivations are in `docs/project/omg-issues.md` ("Defects in the vendored quantity
  libraries"); nothing is filed upstream.

- **The built binaries are about a quarter smaller.** `make build` and the release builds now link with `-s -w`, dropping the symbol table and DWARF debug data that a shipped binary never reads; on Linux `sysml` goes from 50 MB to 37 MB. The version stamps (`sysml --version`), the embedded build info the standard-library cache keys on, and panic stack traces are unchanged.

- **A symbol's declaring element is read through one method.** `symbols.Symbol` gained `Owner()` (nil at a document's root); the identical `ownerOf` copies in the resolver, semantics, validation, code generation, views and editing packages are gone, as are four private "FQN or name" helpers that were `symbols.FQNOf` under another name, since `FQNOf` always ends in the symbol's own name. No behavior changes.

- **Test-support packages live under `tests/`.** `internal/fixtures` (conformance-case and library-census fixture readers) and `internal/stressmodel` (the satellite-network generator) are now `tests/fixtures` and `tests/stressmodel`; the runtime, gRPC and model tests and the `tools` module import them from there, and the layering test fails any `internal/` or `cmd/` package that reaches into `tests/`. No shipped binary imported either. No behavior changes.

- **The OMG corpus gates and the RDF round-trip ratchet move to `tests/corpus`.** `TestTrainingExamplesSemanticErrors`, `TestPilotCorporaDiagnostics` and `TestCorpusGatesCacheStateIndependent` leave `internal/core/model`, `TestCorpusRoundTrip` leaves `internal/core/export`, and their expectation files move to `tests/corpus/testdata/` (`go test -count=1 ./tests/corpus -run 'TestTrainingExamples|TestPilotCorpora|TestCorpusRoundTrip'`). The policies are unchanged: the training corpus is still asserted clean and `-update-training` still refuses to record a per-file count, while the pilot roots and the round trip remain per-file ratchets with `-update-pilot-corpora` and `-update-corpus-roundtrip`. The download scripts, the `OPENSYSML_REQUIRE_*` variables and the CI steps that set them now run the gates from the new package.

- **The gRPC conformance suite and the external-package tests move under `tests/`.** `TestGRPCConformance` and its fixtures leave `internal/grpc` for `tests/grpc` (`go test -count=1 ./tests/grpc`), driving the service through `grpc.NewService` and the RPC surface alone; the package-local gRPC tests, including the `TestGRPCRobustness*` census, stay put. The `package x_test` files that exercised `export`, `resolve`, `semantics`, `migrate`, `suggest`, `identity`, `queryplan`, `model`, `rdf/ontology` and `interop/reposync` through their exported surface move to the matching `tests/<package>` directory together with the `convert`, `superseded` and `xmi` fixture trees they read; the pilot library identity gate now runs as `go test -count=1 ./tests/identity -run TestPilotLibraryXMI`. The LSP and REPL suites stay beside their packages: they share package-local helpers and reach unexported server state.

- **The parser's black-box suites move to `tests/parser`.** `TestGolden` (with its `-update` flag), the `TestNegative` table and `TestNegativeKerML` now drive the parser through its exported API from `tests/parser`, and the golden fixtures move with them from `internal/core/parser/testdata/parse` to `tests/parser/testdata/parse` (`go test -run TestGolden ./tests/parser`). The parser's white-box tests stay beside the package; the documentation census counts negative subtests across both directories, and the grammar coverage's `parser-fixtures` root follows the fixtures.

- **The repository root gains a `tests/` tree for black-box test code and fixtures.** The module-wide hygiene check moves from `internal/hygiene` to `tests/hygiene`, the benchmark harness from `internal/perfbench` to `tests/perf` (`go test ./tests/perf -run '^$' -bench .`), the `gobuild` and `graphcmp` test-support packages from `internal/testutil` to `tests/testutil`, and the shared `.sysml`/`.kerml` fixtures from the top-level `testdata/` to `tests/testdata/`. `scripts/pgo-profile.sh` and the pilot differential's and grammar coverage's `testdata` root follow the fixtures; nothing shipped in a binary changes.

- **The counting gates and the remaining unreleased programs live in the tools module.** The
  validation-constraint census (`cmd/validation-census`), the grammar-coverage harness
  (`cmd/grammar-coverage`) and the documentation-figure gate (`cmd/doc-counts` with
  `internal/doccounts`) are `tools/census/{validation,grammar,doccounts}`, the conformance runner
  (`cmd/conformance`) and the stress-model writer (`cmd/stress-model`) are `tools/cmd/conformance`
  and `tools/cmd/stress-model`, and the baseline provenance and JUnit writers they share
  (`internal/baseline`, `internal/junit`) are `tools/oracle/baseline` and `tools/oracle/junit`;
  every one runs with `go run -C tools ./cmd/<name>`. The analysis-library census schema that the
  runtime's own test writes is `tests/fixtures`, which the doc-counts gate reads from there;
  the model tests load the stress model from `tests/stressmodel`. No figure moved.

- **The development tools are a nested Go module, `tools/`.** `tools/go.mod` requires the
  product module through `replace … => ../`, so the tools build against the working tree while
  `go build ./...` and `go test ./...` at the root stay product-only; `make test` and `make lint`
  run both modules. The library snapshot generator and the ontology table generator are its first
  residents, at `tools/gen/snapshot` and `tools/gen/ontology`, invoked with
  `go run -C tools ./gen/<name>` (the `go:generate` directives and `make stdlib-snapshot-check`
  follow); both resolve the repository root through `tools/oracle/repo` rather than the working
  directory.
- **The errata overlay is split from the registry.** The entry type, the overlay applied to the
  bundled standard library on read and the library's own entries are the product's
  `internal/core/libs/errata`; `internal/errata` keeps the registry the oracles read — the corpus
  entries, the published roots and the corrected copy of a corpus root — and builds on it.

- **The referees live in the tools module.** The fUML and PSSM referees (`internal/fuml`,
  `internal/pssm`) are `tools/referee/fuml` and `tools/referee/pssm`; the pilot differential,
  rejection, Xpect and execution oracles (`cmd/pilot-diff`, `cmd/pilot-reject`, `cmd/pilot-xpect`,
  `cmd/pilot-exec-diff`) are `tools/referee/{diff,reject,xpect,exec}` with one thin `main` each under
  `tools/cmd/`, so every referee is run with `go run -C tools ./cmd/<name>`. What they share moved
  beside them: the XMI reader is `internal/core/xmi`, the corpus errata registry is
  `tools/oracle/errata`, the repository-root and develop-commit lookups every tool carried a copy of
  are `tools/oracle/repo`, and the report files and verdict buckets are `tools/oracle/report`. The
  committed baselines record the new corpus paths; no figure moved.

- **PDF headings and captions stay with what they introduce.** A heading, a caption, or the paragraph directly before a table is no longer left at the foot of one page with its table or figure at the head of the next, and a column header is no longer broken inside a word to fit; a page break can move by a paragraph in an existing document, but page counts do not change.

- **A call whose required input receives no value is performed.** The SysML v1 migration
  wrote a call passing no argument, or a pin no value reaches, for a parameter with no default
  and a lower bound above zero as a placeholder performing nothing; v1 runs the callee with the
  parameter unset, so the parameter or pin is now declared admitting no value (`[0..upper]`),
  the call is written and performed, the absence propagates through the pins, nested activity
  outputs and method parameters it feeds, and a write of a feature requiring a value from one
  that may be absent is guarded (`if x->SequenceFunctions::notEmpty() { assign … }`). The report
  says on each parameter and pin why a value may fail to reach it. A send of a signal whose
  required attribute gets no value still stands in for itself, since v2 admits no such send. A
  call behavior action that names no behavior yet has pins stays unresolved with its pins and
  the reason; an «Allocate» from the action to a part is named in it as saying where the action
  runs, not what it does, and no behavior or value is made up for it.
- **«Probability» is read by provenance.** The SysML v1 migration weights a decision's
  branches only by the OMG SysML profile's «Probability», recognised by the namespace its
  application is serialised under as every standard stereotype is; a same-named stereotype from
  another profile weights nothing, and the report says which profile it comes from.
- **A duration with no unit is in milliseconds.** The SysML v1 migration read a duration
  constraint, time event or `SimulationConfig` step written as a bare number — `200`, `t = 1500`,
  an expression naming no unit — in seconds; the simulation toolkit's default unit is the
  millisecond, so such a duration is now scaled from milliseconds (`accept after 0.2 [SI::s]`,
  `this.settle * 0.001`) and the report notes the reading. A duration with a unit is read as
  before, and `m`, `wk`, `millisec`, `microsec` and `nsec` are read as the toolkit spells them.
- **A run configuration is refused by name.** `-compare-results` refuses a configuration whose
  behavior was not migrated, or whose `durationSimulationMode` is no draw policy, or whose run
  fails, naming the configuration in the refusal, so the refusals of several configurations
  printed together tell which is which.

- **The diagram panel lays unplaced nodes out in layers and routes edges around boxes.** Nodes the model does not place took slots in a square grid and every edge ran straight between centres, across whatever lay between; they are now laid out by the ELK layered algorithm, edges orthogonal and routed around the boxes. A node the model places, or one dragged, is drawn where stated as before. Renderings of more than 600 nodes keep the grid.

- **A workspace copy of a library file rooted at the library's packages is the library, as it
  is for the RDF mapping.** The language server treated such a copy as the user's file — derived
  ids, a minting action on every declaration — where `sysml -convert` recognised the same bytes
  as the bundled file. The workspace now applies the one recognition
  (`identity.Catalog.DocumentRootedAt`, moved out of `internal/core/export`): a document whose
  every root is a top-level package of one bundled library file (a package two library files
  declare at their top names neither), stating that package's
  normative id or declared as the library declares it, and in the file's language (the text of a
  `.kerml` file under a `.sysml` name was parsed as SysML), stands in for the bundled file. Its
  declarations are what the library's names resolve to, its elements keep their normative ids
  (hover states `(normative, KerML)`) and get no minting action, and it is not warned for its
  `standard library` keyword. Editing a root so it no longer qualifies, or closing a version
  whose on-disk text is the user's, puts the bundled file back. The library identity the
  runtime names library types by (`symbols.Index.LibraryIdentity`) digests each library
  document's language, tier and text, no longer its name, so an unchanged version standing in
  leaves it — and the objects carried across a re-analysis — as they were. A workspace over a
  caller-built index (`model.NewWorkspaceWithIndex`) treats the library files that index marks
  the same way, a file `MarkLibrary` marks at the generic tier included: a document may stand in
  for one or take its name, and closing it puts the file back. The library is what the index
  shows, a file the overlay shadows under a frozen base's name included. An edit's temporary
  index keeps the documents such an index holds beyond its frozen base, marked or not, and
  resolves against a shadowing file rather than the one it shadows, and does not bring back a
  base document the overlay removed.
- **A file opened under a bundled library's own name no longer removes that library from the
  workspace when it is closed.** Closing it put nothing back, so every later document was checked
  against a library missing that file; the standard-library expression gate
  (`TestExprTypeCheckNoStdlibFalsePositives`) passed on an incomplete library for that reason.
  Over the whole library it now reports nine dimension defects in the published `SI.sysml` and
  `USCustomaryUnits.sysml`, pinned as an exact set and recorded in `docs/project/omg-issues.md`
  ("Defects in the vendored quantity libraries"); the library bytes are unchanged.

- **The XMI element walker the PSSM referee reads its suite with is now its own package,
  `internal/xmi`**, so other UML-based test suites can be read with it. `internal/pssm`
  behaves exactly as before.

### Fixed

- **The API element form spells a multi-valued property as an array even with one member.** The writer decided array-vs-object by member count — an array only where the `json:` annotation of two or more members stated one — so `ownedRelationship` or `ownedMember` holding a single element went out as a bare `{"@id": …}` object, which conforming readers such as sysml-toolkit drop silently. The shape now follows the metamodel's upper multiplicity, read from `SysML.ecore` into the generated `internal/translate/rdf/ontology` table (`Property.Many`, resolved per metaclass by `PropertyOf`): an unbounded property is always an array and a single-valued one an object or scalar.
- **The element-form reader accepts `{"@ref": <name>}`.** sysml-toolkit writes an unresolved reference target as `{"@ref": "<name>"}` rather than `{"@id": …}`; the value now reads as the name literal the mapping already uses for a name-valued reference, and the refusal message for any other object spelling names both forms.

- **Completion events are queued in the order their states were entered.** When the entries of two orthogonal regions each left a state that completes at once — a start state, a state with a completion transition and nothing to perform — the runtime queued the completion events once the move had settled, region by region in declaration order, whatever order the entry draw had entered them in; PSSM §8.5.9 puts a completion event behind those already in the pool, so under `reverse`, `seed:<n>`, `check` and `explore` the region drawn second could have its completion dispatched first. A state's completion is now queued as its entry unit is performed, on every way in — a composite's default entry, a fork's branches, a shallow or deep history's restore, the target of a transition — so the pool dispatches the completions in the entry draw's order; time triggers are still scheduled once the move settles, and a completion a running do behavior or a nested composite's regions hold back keeps its timing. An entry that performs nothing but generates a completion is drawn as an alternative of the entry front rather than riding with the neighboring performing unit as a silent unit, since the pool's order makes it observable, so a `choice entering <state>` line appears where two such entries meet; `declared` takes the same order as before and no default outcome moved, while `check` and `explore` reach the runs in which the regions were entered the other way round — `state_concurrent_do` and its kin reach eight values of `seq` where they reached four, `state_do_step_among_completions` six logs where it reached three (`state_completion_pool_entry_order`, `state_completion_pool_history_order`, `state_completion_pool_deep_history_order`, `state_completion_pool_fork_order`, `state_region_completes_at_own_done`; `TestRuntimeRobustnessCompletionOrder`). On the PSSM suite no bucket moved: *History 001-C* and *Entering 011* each reach one more admitted trace, *History 002-B* one more and the two §8.5.9 gives that the suite does not register, and *Transition 017* reaches six of its eight, the two it misses being the suite's own defect.

- **Several completion transitions out of one state are one choice.** A state's completion queued one event per enabled completion transition and dispatched them in declaration order, so the first always fired, the rest went stale, and no choice point was reported: `explore` could never reach the run in which another of them fires. The completion is now one occurrence: when it is dispatched, the queued completion transitions of the state are read again and one is drawn by the scheduling policy — declaration order by default — and recorded as a `transition` choice point that `explore` enumerates and `check` compares, the others leaving the queue (`state_explore_completion_choice`).

- **A compound transition through a pseudostate inside a composite state exits and runs its effects segment by segment.** A transition into a junction of a composite state, continued by the junction's outgoing transition out of the state, now exits the source, runs the first effect, exits the composite state, then runs the second effect and enters the target, as UML 2.5.1 §14.2.3.8.4 orders the segments; before, every exit ran before any effect. A join of the state its orthogonal regions leave through runs each region's exit and effect, then the state's exit, then the outgoing transition; a route ending at a terminate action or a history pseudostate keeps its effects in that order.

- **A connector whose first end is missing is reported at the `to`/`then`, with
  one diagnostic.** `connection c connect  to ;` read the `to` as the first
  end's name and then wanted a second `to`, producing the misleading `expected
  'to' between connector ends` at the semicolon, and `connect to a;` produced a
  second spurious `expected '{' or ';' after declaration`. The clause now reports
  `expected a connector end before 'to'` (or `'then'` for successions) at the
  keyword and still parses the end after it, so recovery adds no further error;
  `connect to to b;`, where the first end is genuinely named `to`, is unchanged.

- **A diagram panel left open across an extension update restored blank.** VS Code restores a webview panel with the options it was created with, whose resource roots named the directory of the extension version that created it; after installing another version the bundled script lived elsewhere and was blocked, so the restored panel drew nothing and listed no views until closed and reopened. A restored panel now sets its resource roots from the installed version before its page is set.

- **A due do step is drawn against the entry units left in the move: the sibling regions' and the state's own substates'.** A state's do behavior is started by its entry and, in KerML's `StatePerformance`, ordered after that entry and against nothing a sibling region or a substate performs (`succession entry then middle`; PSSM §8.5.5 has the do activity run concurrently with the behaviors that follow it in entering the state, the substates' entries among them), but under `check`, `replay` and `explore` its first token move waited for the whole entry move to settle and was drawn only against the dispatch, so the run in which a do activity's first action precedes a sibling region's entry, or a composite's own do activity precedes its substate's entry, was never reached. A do behavior now begins as its state's entry unit ends, before the state's regions or serial body are entered, and each due token move of the do behaviors the move began is a drawn unit, `do <state>`, against the entry units still ahead: on a front, under the front's own `entering <state>` (or `fork <name>`, or a firing's `on <event>`) choice while another queue has a unit left; down a serial body, against each entry unit on the way under the owner's `entering <state>`; then against the dispatch after the move settles as before. A do step is never silent, and a do behavior that ends while its state's body is still ahead of the move completes nothing — the body's `done` completes the state. The fixed policies (`declared`, `reverse`, `seed:<n>`) enter every state whole and run the do round after, as they always did, and no default, `declared` or seeded trace moved (`state_do_step_before_sibling_entry`, `state_do_step_before_sibling_entries`, `state_do_step_before_nested_entries`, `state_do_step_nested_before_outer_entry`, `state_do_step_before_fork_branch`, `state_do_step_typed_before_sibling_entry`, `state_do_step_before_history_restore`, `state_do_step_cut_by_sibling_completion`, `state_do_step_cut_by_sibling_terminate`, `state_do_step_before_own_substate_entries`, `state_do_step_before_own_body_entry`, `state_do_step_machine_before_top_entries`, `state_do_step_way_down_before_fork_branch`; `TestRuntimeRobustnessDoStepEntryFront`). On the PSSM suite *Terminate 002* reaches its fifth admitted trace, the do activity's first segment before the sibling region's entry, and passes (57 pass / 12 fail); *Deferred 006 C* and *Transition 017* explore more linearizations and reach the same traces as before.

- **A PDF draws every state and tree figure a migrated document holds.** A state transition's label is written with its colons as entities, so a trigger or guard naming a qualified element (`accept Signals::Go`) no longer reads as a Mermaid class marker and fails the chart; and each Mermaid chart is drawn under a configuration sized to it — as is an HTML page that loads Mermaid with `-html-mermaid`, whose script is configured for its largest chart — so a tree or flowchart of more than 500 edges or 50 000 characters is drawn rather than refused by Mermaid's defaults. The configuration stops at twenty times those defaults, and a chart past 1 000 000 characters or 10 000 edges is refused with a typed `oversized-diagram` error naming it and its size — to be drawn as `dot` or `plantuml` — so no model asks a browser or `mmdc` for unbounded work. A figure taller than the page is scaled onto one page with its caption instead of running off the page's foot with the caption on the next.

- **The SysML v1 migrator writes a transition effect with no body as `do action effect { }`.** An effect activity with no nodes was written `do action effect;`, which ended the transition clause before its `then`, a syntax error in the migrated notation; the braces are now kept so the `then` still belongs to the transition.

- **An `exhibit state` usage naming nothing exhibits itself.** Per SysML v2
  §8.3.17 (`ExhibitStateUsage::exhibitedState` redefines `performedAction` — the
  reference feature of the owned reference subsetting, or the usage itself when
  there is none), `exhibit state modes { in cmd = port.cmd; … }` and
  `exhibit state idle;` are state usages with their own (possibly empty) body,
  not references to one held elsewhere; `exhibit state modes;` no longer fails
  instantiation with `classifier behavior names no body`, and a body declaring
  no initial state fails with `ErrNoInitialState` at initialization like any
  machine stating an empty body. An `exhibit` declaration that does name an
  element — the `exhibit m;` reference form, a `references`/`::>` clause, or a
  typing — that resolves to no behavior body is still reported.

- **`explore` varies the first run's choice points earliest first.** The runs took the untried alternatives deepest first, so a choice met early with a long tail of closed choices behind it — the `do round` at t=79 of the runtime showcase's spacecraft, after two hundred token orders — was varied only after every order of the tail, and `explore:runs=300` tabled one of the two outcomes `-engine check` finds. The prefixes an exploration leaves now run in plan order: every prefix departing from the first run at one choice before any departing at two, earliest choice first, so a `runs` budget of one more than the first run's choice points varies each of them at least once, and the table is the same at any `-jobs`. A choice point past the `depth` budget is still never varied, and every choice point a run met is in its witness, which sizes `depth`: the spacecraft's run to t=80 meets 277, so `explore:runs=300,depth=512` tables both outcomes where the default depth of 64 cannot (`action_explore_early_race_long_tail`, `TestExploreTablesTheSpacecraftRaceWithinItsBudget`). The run a witness names moves with the order: the three-writer race's `x = 1` is now reached by run 5 rather than run 4.

- **A fork may enter orthogonal regions that have no initial state.** The lowerer used to refuse
  every `parallel` region without an `entry; then <state>;`, even when a `fork`'s outgoing
  transitions were the only way in, which UML allows. Each fork's branches are now read into a
  plan — one target state per orthogonal region of one composite state, at least two branches,
  none guarded or triggered — and a region a fork enters needs no entry transition of its own; a
  region with neither is still refused with the same diagnostic, and so is a machine with
  another way into the composite state — a transition to the state itself, to another of its
  regions or to its history, the machine's entry naming it, or another fork passing through it
  on the way to a state nested deeper — since that way would start the region by default and it
  has no default start. Entering through a fork leaves the source configuration down to
  the ancestor the source and the composite share, as a move to a single state does, so an
  active ancestor is neither exited nor entered again — a fork reached from inside the
  composite's own regions leaves every one of them, in declaration order, ending their do
  behaviors, while the composite stays active; then the first branch runs its effect
  and enters the states still on the way down to the composite, and every region enters in
  declaration order, each branch's effect before its target — which may lie below a
  region's own substates, the branch entering every state on the way — so a branch's effect
  precedes the composite's `entry` when the fork sits outside it, even when a region no branch
  names is declared first, and such a region starts at its own initial state. A branch's effect reads and writes the attributes of the
  state declaring the fork, as a transition leaving one of its substates does. A composite state entered on the way down does not start the
  region the branches pass through at its own initial state — only its other regions start as
  usual — and a do behavior in a region the fork leaves untouched still takes the occurrence
  that fired it. Branches that end at `done` complete the composite state, or the machine, as an
  ordinary entry does, and the checker's footprint of a fork covers the regions it leaves to
  start by default, so their entry behaviors' reads and writes count as the dispatch's. The PSSM
  referee's classifier stops filing
  *Fork 002* and *Join 001* as not expressible; both translate and run, and the baseline moves
  from 39 to 37 `not-expressible` and 18 to 20 `fail`, adjudicated in
  `docs/project/pssm-referee.md`.

- **Graphical renderings head a root by its name within the view, not its whole qualified name.** The interconnection, state, action and tree forms labelled every exposed root `TMT::'01 TMT PO'::'System Model'::…::tcs : TMT::…::TCS` while its nested members read `pump : Pump`, so a migrated diagram's fixed-size boxes were overrun by their own labels and the picture was unreadable. The Mermaid, DOT and PlantUML writers now drop the namespace every root shares from the roots' names (`Plant::Loop` heads `Loop`; `Systems::Radio` beside `Systems::Braking::Brake` heads `Radio` beside `Braking::Brake`; roots from unrelated packages keep their whole names) and name a type by the name each reference ends in, `~` kept (`~Ports::FuelPort` is `~FuelPort`). The name and type a node carries — `name` and `type` in the rendering JSON, the text form's declarations — are unchanged, and the DOT writer sizes a box from the label it emits.

- **A transition guard reads the accepted payload by the transition's name.** `transition raise first idle accept l : Level if raise.l > 5 then high;` failed with `eval guard of transition raise: no value for feature raise`, as did `if raise.d.level > 5` and a guard on a segment out of a choice or junction reading the accepting segment's payload, `transition up first pick if raise.l > 5 then high;` — the bare `l` worked, and so did `raise.l` in the state's exit and the transition's effect, since only those were evaluated within the transition's firing. A guard, and a probability on a route out of a choice, is now evaluated within the firing of the transition it belongs to: the candidate transition's own, with the payload its trigger just bound, for a plain guard, and the compound transition's, the accepting segment included, for a segment past a choice or junction, an occurrence released from a deferral included. A guard naming a transition that is not being taken reads null, so comparing it is the operator's type error (`type mismatch: operator '<' is not defined for null and an Integer`) rather than a silent false, and a guard the read leaves non-Boolean is `type mismatch: guard of transition raise must be boolean, got an Integer`. Time-trigger durations, change conditions, entry guards and run-to-completion values are evaluated as before, outside any firing (`state_choice_guard_reads_accepting_segment`, `state_junction_guard_reads_call_argument`, `state_guard_reads_own_payload_member`, `state_guard_names_transition_not_taken`, `state_guard_reads_deferred_payload`; `TestRuntimeRobustnessGuardPayload`). The PSSM referee's every row is unchanged.

- **A transition into a history pseudostate restores the configuration it is leaving.** The
  record a history restores is written when its owning composite state is exited, but a
  transition whose source is that owner — its self-transition or its completion transition into
  its own history — used to read the record before its exits ran, so it restored the previous
  visit's configuration (or, on the first visit, performed a default entry) instead of the one
  being left. The record is now read after the transition's exits and effects; a history's
  default transition is taken from inside the owner once it is entered, so the owner's `entry`
  runs before the default transition's effect and the region's initial transition does not run
  beside it. A `history` declared in a state machine's own body restores the machine's top-level
  configuration instead of being refused as a history outside any composite state.
- **The PSSM referee's translation no longer folds an initial transition's effect into the entry
  action of the state or region it starts.** The effect ran before the state's own `entry` and
  again on every re-entry, a history restore included; the initial transition now enters an
  empty helper state whose completion transition carries the effect. With the history fix, the
  baseline moves from 36 to 41 `pass` and 23 to 18 `fail` (History 001-A, 001-B, 001-D, 002-A
  and 002-D), adjudicated in `docs/project/pssm-referee.md`.

- **A join runs the effect of every transition into it.** The transitions into a join and the one
  out of it are segments of one compound transition, but firing the join ran only the effect of
  the incoming transition that completed last and dropped the others'. Every incoming segment now
  fires — its source exited, then its effect — before the state owning the join is exited and the
  outgoing segment's effect runs; the order among the incoming segments is a region-order choice
  the scheduling policy draws (`choice join <name>` in traces, source declaration order by
  default), and a failing effect on any incoming segment fails the step. The static footprint of a
  transition into a join folds in the other incoming effects too.

- **A join's segments fire with their own trigger's arguments, and a refused replay undoes them
  all.** A transition into a join that accepts a payload or a call had its effect run without the
  arguments its trigger names bound, so it read the previous values or failed; each segment now
  binds what its own trigger takes from the occurrence being dispatched before its effect runs. A
  join two of whose incoming transitions leave the same region is refused when lowered — UML has
  the segments originate in different orthogonal regions, so none is an alternative to another
  — and a replay refused at a later draw among the segments undoes the segments already fired
  with the rest of the move rather than leaving some sources exited. A segment fired by a timer
  or a change condition's rise now holds the join, as one fired by a signal does, until the same
  occurrence enables every other segment into it; and a join whose sources all lie nested below
  the states of the owner's regions exits those wrappers and the owner, where before it found
  no owner and left them active. A segment leaving a composite state whose substate is active
  now holds and fires the join as the occurrence reaches that state from within, exiting the
  substate first, where before the join never fired. A join of the machine's own regions whose
  segment leaves a state nested in an orthogonal state of a region now records that region, so
  the segment exits the nested state and its wrappers once, where before the innermost region
  was recorded and the orthogonal state was exited a second time when the regions were left. A
  segment drawn among several transitions out of its source, whose join an earlier region's
  effect disarms before its turn, fires nothing and records no choice, where before the draw
  stood among the run's choices as though the segment had fired. A timer's expiry selects the
  segment it fires as a signal dispatch does — its guard holding and the join it leads into
  ready — before the route out of the join is resolved, so an expiry that does not fire the join
  reads no guard beyond it, where before a junction beyond the join with no guard holding aborted
  the run. Two time-triggered segments into a join whose timers are due at one instant fire the
  join, whichever expiry is dispatched first, where before each expiry found the other segment's
  timer to be a different occurrence and the join never fired; timers due at different instants
  still never fire it. A signal or call dispatched at the instant a segment's timer is due does
  not stand in for that expiry, so it enables no time-triggered segment; nor does a completion
  event queued at that instant, so a source completing when a sibling segment's timer is due
  leaves the join to that timer's own expiry. The checker no longer stops a machine whose closed
  do round finds no dispatch due short of its timers: it rests, and the clock's advance to the
  next expiry resumes it, as a run outside the checker does. A change condition's
  rise selects the segment it fires as a signal dispatch does, the join it leads into ready,
  before the route out of the join is resolved, so a rise that does not fire the join reads no
  guard beyond it. The draw among a source's transitions that selects a segment into a join is
  recorded within the join's move, so a replay refused at a later draw among the segments undoes
  that record too, where before it stood among the run's choices after the move was undone. The
  check oracle's snapshot of a run's draws is copied rather than aliased, so a run restored to an
  earlier point no longer trims a snapshot taken after it. A rise that enables only a segment
  whose join is not ready is dispatched as a signal nothing takes is — consumed, and counted
  as one occurrence — so a run stepped under the check policy takes the dispatch it was offered,
  where before the checker offered a dispatch the poll then refused as nothing to do; and a
  segment drawn among several that fires nothing is reported as firing nothing rather than as
  a transition taken. A segment into a join that has no trigger
  is enabled by another segment's occurrence only once its source has completed — no do behavior
  of it running and, where one runs, its body done — so the join no longer fires and abandons
  that behavior; it waits for the next occurrence after the source completes. The footprint of a
  transition into a join now covers what firing the join reads and writes: every other segment's
  source, trigger and guard, the exits of every source up to the owner and of every region the
  owner (or the machine, joining its own regions) has, and the effects of every segment — so the
  checker's reduction no longer treats a step writing what a sibling segment's guard or exit
  touches as independent of the join.

- **A junction with several enabled outgoing branches draws one of them as a choice point.**
  The guards of a junction's outgoing transitions are still read before the incoming transition
  fires, against the data as it then stands, but where several hold the runtime used to take the
  first in declaration order; it now draws and records the transition choice point at the junction,
  as it does at a choice, so a `seed` policy replays its draw, `explore` enumerates every branch, and
  the trace and the `choice` note name the junction. The draw is made only as the transition fires
  — after the order among several regions' transitions is drawn and the transition's own guard is
  read again — so a witness lists the region order before the junction's draw and a transition
  another region's effect disarms draws nothing, no guard beyond the junction is read again (the
  route on from each enabled branch, through any further junction, is settled with the
  transition, and a branch beyond which no guard holds fails only the run that draws it, with
  the guards it noted on its way), and a
  replay refused at a choice beyond the junction undoes the draw with the rest
  of the move; a history's default transition through such
  a junction records its draw the same way. The unguarded branches remain the default
  when no guard holds, and a junction with no enabled branch still leaves the compound transition
  unenabled. The PSSM referee's baseline moves from 44 to 45 `pass` and 16 to 15 `fail`
  (Junction 003), adjudicated in `docs/project/pssm-referee.md`.

- **A DOT rendering positions a node from the routes that meet it, so a migrated diagram is written for `neato -n2` throughout.** The `start` node and the initial and final pseudo-states of a migrated activity or state machine have no member a `DiagramLayout::Layout` could name, so every view drawing one had an unpositioned node, the header fell back to plain `neato` and the written `Route`s were redrawn rather than kept. A node with no `Layout` now takes its box from the first waypoint of a route leaving it or the last of one reaching it, sized as the writer already sizes it; a stated `Layout` still wins, a one-point route places nothing, and a node with neither stays unpositioned so the engine degrades as before. A cluster with no `Layout` is boxed round its positioned members and its anchor pinned there, which `neato -n2` requires.
- **The SysML v1 migrator writes a `Layout` for the control, buffer and final nodes it declares.** A fork, join, decision, merge, buffer or activity final node written as a named member of the migrated action (`fork 'fork';`, `action final terminate;`) was counted "not exposed" by the diagram join and left without a placement; it is now exposed and positioned like an action, while an initial or flow final node, which the migrator spells as the inherited `start` and `done`, is still counted not exposed and positioned by the renderer from its routes.
- **`-render-all` writes a view whose name is not a bare filename instead of stopping the run.** A qualified view name containing `/`, `\`, `:`, `%`, `.`, a control character or a character Windows reserves stopped the whole run with "does not form a safe rendering filename"; the unsafe bytes are now percent-encoded (`%2F` for `/`), a Windows device-name stem has its first byte encoded, and the encoding reverses to the view name. A name past the 255 bytes a path component holds is cut and tagged `~` and a hash of the whole, and a save writes its temporary file under a name that fits beside a destination that long. Two views meeting in the same path, letter case aside, are refused together.

- Editing a document no longer recomputes workspace-wide gathers inside the edit; they are recomputed on the first diagnostics or query after it, so an edit itself is as cheap as before those gathers existed.

- **Untyped usages now subset the standard-library base feature for their kind and derive their type from that feature, preserving inherited members through recorded library specialization edges.**
- **Parameters of a step or calc typed by a standard-library behavior are redefined by position whether the library is parsed, restored from the on-disk cache, or loaded from the embedded snapshot; this is now covered by tests.**

- **`Session.LoadFile` follows imports to sibling model files.** Loading one file through the session's exported single-file API now pulls in the `.sysml`/`.kerml` files beside and below it that declare an imported root namespace, as `%load`, `-check`, `-compile` and `-render` already did, and submits them together so a reference into a sibling package resolves.

- **Loop and branch bodies now preserve member-attached `then` successions.** These flows are retained when parsed and exported instead of silently losing their positional edge.

- **An edit no longer re-derives every wildcard import in the workspace.** An import whose target did not resolve — an ambiguous or unknown package — was bookkept as reading from the document root, so a keystroke anywhere purged and rebuilt the re-exports of every importing file before the language server could answer again. Diagnostics, completion and semantic highlighting now catch up in milliseconds after an edit rather than seconds.
- **Completion keeps its name table across edits.** The table of simple names the language server completes and suggests corrections from was rebuilt from the whole index after every change; it is now refiled for the names that changed, so the first completion after an edit answers as fast as the next.
- **Semantic tokens index the text they were computed from.** The tokens and the document content were read separately, so an edit landing between the two could encode one revision's tokens against another's lines; both are now taken in one read.

- The language server no longer scans the filesystem root for sibling files when a document at the root is opened outside every workspace folder; the walk could take minutes.

- **Diagrams and diagnostics of a large document no longer stall the language server.** Byte offsets are mapped to editor positions through the document's line index instead of a scan from the start of the text on every span, so `opensysml/render` on a multi-megabyte model answers in seconds rather than minutes.

- **Name lookup no longer resolves a membership import's target when the import cannot surface the name.** A non-recursive `import P::x` (or `expose x`) only surfaces `x` or the target's short name, so an unqualified lookup of any other name skips it; previously each import's target resolution re-walked the scope's sibling imports, making lookups in a namespace with many `expose` or `import` members factorial in their number.

- SysML v1 migration no longer copies opaque expressions whose cast multiplicity bounds or body-local connection/flow endpoints name members the model lacks, and treats declared connector-end names as in scope within their connector bodies.

- **The v1 migration writes a transition whose effect has an empty body in a form the parser accepts.** An effect that is an Activity with no nodes was written `do action effect;` before the transition's `then <target>`, which the parser rejects, and one such transition made the whole migrated file unwritable. The effect is now `do action effect { }`; a state's `entry`, `do` and `exit` actions, which stand alone, keep `entry action x;`. A transition whose `effect` refers to a behavior owned elsewhere, which was dropped without a word, now runs it: `do action : Def`, with no `;` before `then`. The report says when an action is empty, and an effect or state action whose every node is refused, or whose opaque body is in a language the mapping cannot write, is reported approximated with the reason. The migrated OMG PSSM test suite, which this made unwritable, is gated in `tests/corpus` (`TestPSSMSuiteMigration`): its notation must parse and its report totals ratchet; see `docs/project/pssm-migration.md`.

- **Validating deeply nested calls no longer takes exponential time.** Typing a call read its arguments again for every reader of the enclosing call — the argument check, the result type, the held element type — so each level of nesting doubled the work and ten nested `calc` invocations took seconds while a generated query nesting twenty took longer than anyone waits. The checker now types every call once per scope and answers its silent re-readers from that memo, keyed by the call and its scope, with a call still being typed never recorded as its provisional unknown type; the reporting checker types as before, so each diagnostic is still reported once at the place it arose. The semantic model likewise answers a memoized call selection without retyping its arguments.

- **A body inside a nested definition no longer reaches the enclosing definition's features
  by their bare names.** `part def P { attribute n = 1; calc def E { n + 1 } }` — and the same
  shape with a `constraint def`, an `action def`'s `assign`/`if`, or a `state def`'s transition
  guard — now reports `Must be an accessible feature (use dot notation for nesting)`, as the
  reference implementation does: a nested *definition* is a new type with no featuring
  relationship to the one that owns it, so `n` is a feature of `P`, not of `E`. The featuring
  contexts of a definition were being derived from its owner as if it were a feature. A nested
  *usage* (`calc e { n + 1 }`) is featured by `P` and still reaches `n`, and a nested definition
  still reaches its own, inherited and redefined features and every package-level feature.

- **The nightly snapshot signs again.** The cosign installer action was pinned at a v3 release that fetches a detached `.sig` for the requested cosign, but cosign v3.0.1+ ships `.sigstore.json` bundles instead, so the install step failed with a 404 before signing. The workflow now pins cosign-installer v4, which verifies those bundles.

- The nightly snapshot's release notes no longer break mid-sentence: GitHub renders a release body with hard line breaks, so each paragraph is written as one line.

- The nightly snapshot no longer falls back past the commit it was last built from when `develop`'s head is red: the walk for the newest green commit ends at the published snapshot, so a night with nothing newer green leaves the previous snapshot standing instead of publishing an older commit — or failing on one that predates `scripts/build-release-artifacts.sh`, which is now skipped.

- **Feature chains are now written in the normative interchange shape.** A chain (`connect a.b.c to d`, `:>> a.b`, `references a.b`) converts to a chain `Feature` that owns one `FeatureChaining` relationship per link — owned by the `ReferenceSubsetting`/`Redefinition`/`Subsetting` that states it, or an `OwningMembership` for an invocation's chain — beside the derived `chainingFeature` list, so other tools' graphs read and a re-conversion states the normative elements. Unresolved links are `{"@ref": "<name>"}` rather than bare strings; reading accepts the normative form, the derived list, or both, and refuses a graph where the two disagree. Interface ends are `PortUsage` (the grammar's `InterfaceEnd`), `then` successions state their two ends in `EndFeatureMembership`s, and `perform`/`exhibit`/`include`/`assert`/`satisfy` usages spell their qualifier from the metaclass alone — named forms like `perform action pa : A`, unnamed reference forms like `perform sub.sa :>> a2` and `assert c1` — so graphs carrying no `sysx:` annotations still write the qualified notation.

- **A region-owning state is not re-entered by a transition inside its region.** In a parallel state, a transition between two substates of one region (`state left { entry action …; state prep; state work; transition first prep when Go then work; }`) ran the entry behavior of `left` again and restarted its do behavior, although `left` never became inactive; a counter its entry incremented read 2 where KerML `StatePerformance` runs `entry` once per activation. The move now keeps the region owner active, whether it is the region of a top-level parallel machine or of a parallel state nested deeper, so only the substates below it are exited and entered; a transition out of the owner still exits it, and a transition into it still enters it afresh (`state_parallel_owner_entry_once_intra_region`, `state_nested_parallel_owner_entry_once_intra_region`).

- **A PDF's default faces are Times, Arial and Courier, not whatever the generic family resolves to.** The print stylesheet asked for bare `serif`, `sans-serif` and `monospace`, which fontconfig resolves to DejaVu on most Linux machines — a face some 15 % wider and taller than Times at the same nominal size, so an 11pt page read like 13pt — while the metric-compatible Liberation faces installed beside it were never chosen. The default body, heading, code and page-number stacks now name the conventional families first, their free metric-compatible equivalents next (`"Times New Roman", Times, "Liberation Serif", "Nimbus Roman", serif`; `Arial, Helvetica, "Liberation Sans", "Nimbus Sans", sans-serif`; `"Courier New", Courier, "Liberation Mono", "Nimbus Mono PS", monospace`) and the generic family last, for the pandoc engine's page as for WeasyPrint's and Prince's. Page size, margins and every point size are unchanged, and the HTML page keeps its system face.

- **The PDF toolchain CI job now reads its rendered PDFs back.** The job installs `poppler-utils`, and the integration tests treat an absent `pdftotext`/`pdfimages` like an absent converter: a skip locally, a failure under `OPENSYSML_REQUIRE_PDF_TOOLCHAIN`. Before, the text and image assertions (headings, captions, formulas, diagram source kept off the page) silently skipped in CI for want of `pdftotext`.

- **A `perform action` usage naming nothing performs itself.** Per SysML v2
  §8.3.16 (`EventOccurrenceUsage::eventOccurrence` — the reference feature of the
  owned reference subsetting, or the usage itself when there is none) and §8.3.17,
  `perform action boost { in amount = level; }` and `perform action idle;` are
  action usages with their own (possibly empty) body, not references to one held
  elsewhere; instantiation no longer fails with `classifier behavior names no
  body` on them, and the body's `in` members bind the performance's parameters.
  A `perform` declaration that does name an element — the `perform a;` reference
  form, a `references`/`::>` clause, or a typing — that resolves to no behavior
  body is still reported.

- **The pilot corpus downloader refuses to report success over an empty corpus.**
  `pilot_fetch_subtrees` in `scripts/pilot-pin.sh` now fails, installing nothing, when a subtree
  of the pinned release holds no file of the kinds asked for, and re-fetches a destination that
  is stamped at the current pin but holds no such file instead of reporting it present; before,
  either left a stamped, empty directory that a required corpus gate would then fail on with no
  hint of why. `scripts/pilot-pin-test.sh` checks the downloader against a throwaway release
  repository and runs in CI before any corpus is fetched. The contributor docs now list all
  three download scripts beside the `OPENSYSML_REQUIRE_*` variables that make their gates
  mandatory.

- **A positioned node's label fits the box its `DiagramLayout::Layout` states; the box is never grown to the label.** The DOT writer word-wraps the head at the stated width and draws it at the largest font size from 14 pt down to 8 pt at which the wrapped lines fit the height, keeps the `«keyword»` and detail lines only while height remains, and cuts and ellipsizes a head that overruns even at 8 pt, using the same glyph estimate the unsized boxes are fitted with; the box's `margin=0` gives the whole of it to the label, as the fit assumes. A migrated Cameo diagram, whose boxes were sized for the name alone, reads as it did: a 449×14 px attribute row holds its one line, `call : doTracking` no longer spills out of its action box, and Graphviz's `size too small for label` warnings on such a model drop to none. A stated box that holds other stated boxes — a part drawn round its members, a definition over its compartment rows — sets its title in the strip above the topmost of them, fitted to that strip, so the title is read as the frame's header rather than covered by the members, which stay where the Layout put them; a box drawn as a cluster round its children is fitted the same way. A stated box, or the strip its members leave it, too short for one 8 pt line or too narrow for one glyph holds no text, and sets its head beside the box instead. A node without a stated size keeps its label-fitted box.
- **A control node or port in a stated box is drawn as its notation symbol, with no text inside.** A decision, merge or choice is a diamond, a fork or join the filled bar, an initial node the filled dot, a final node or terminate action the double ring, and a port its small square; the node's name is set beside the symbol as an `xlabel`, and left out when the view IR marks it as one the model did not give (`Node.NameSynthesized`). Without a stated box these kinds keep their labelled shapes, except that an action's `start` and `done` — the language's names, not the body's — are now the filled dot and the double ring in every graphical form, where they were a large labelled circle.
- **A name the SysML v1 migration made up is not drawn.** The migrator now records every name it spells for an element its source left unnamed — `'start to call'`, `fork2`, `decide`, an `unnamed` ref — once per body, as `metadata MigrationMetadata::SynthesizedName about …;` from the new bundled `MigrationMetadata` library. The renderings read the marker from the model: such a node is drawn as its source drew it — a control node as its bare symbol, a typed usage as `: Type` alone — and an edge whose only text would be such a name carries none, while a triggered transition or a guarded succession keeps its trigger and guard. Names the source gave, however spelled, are never marked; the migration report and results are unchanged.
- **A member drawn under its owner is headed by its name below that owner.** A nested node, or an exposed element whose owner is drawn in the same rendering, no longer repeats the owner's qualified path: `'K-Mirror Offset'::'interpolation Error' : 'Interpolation Error'` inside the `'K-Mirror Offset'` box reads `'interpolation Error' : 'Interpolation Error'`, as a diagram frame shows it. Only the graphical forms' heads change; the text and JSON forms and the LSP keep the qualified name.
- **A positioned DOT drawing leaves the nodes no `Layout` places undrawn, so none lands on a placed box.** When some nodes of a view are positioned and others are not, the DOT writer left the others to `neato`, which set them wherever it found room — over the positioned boxes, in a migrated diagram whose source never drew them. They are now left out, with the edges at them, under a `// not represented:` notice that counts them, so the drawing shows what the source diagram showed; every node drawn is pinned, so the `// layout:` header names `neato -n` or `neato -n2` and never plain `neato`. The new `-render-unplaced strip` (`Options.Unplaced` in the view API) keeps them instead, boxed and packed in rows below the canvas or the positioned boxes' extent, clear of them and of one another; it applies to `-render`, `-render-all` and the `dot` diagrams of `-render-document` and `-render-documents`, and an unknown placement is refused with the two there are. A view with no positioned node is laid out by `dot` as before.
- **A view's layout annotations and `render` members are not drawn as nodes.** The member walk every rendering kind shares leaves out `DiagramLayout::Canvas`, `Layout` and `Route` annotations, wherever they are owned, the `MigrationMetadata::SynthesizedName` markers a migration leaves in a body, and the `render` members a view holds, so a tree over a package of migrated views no longer fills with `metadata`, `x`, `y`, `width`, `height` and `asTreeDiagram` nodes. Every other metadata usage, and a rendering usage outside a view, is drawn as before.
- **A parallel state may carry a metadata usage in its body.** Lowering a `state … parallel` body treated a `metadata` member as unsupported content and refused the whole state machine, so its state rendering came out empty; the annotation is now the state's own, like an attribute or a port, and its substates alone are the regions.

- **`make proto-breaking` reads only `api/proto` from the baseline.** The baseline archive is
  taken from the `api/proto` subtree of `BUF_BREAKING_REF` rather than from the whole commit with
  a pathspec, which walked the whole tree and, from a blobless checkout, lazily fetched every blob
  the commit does not share with the checkout — a fetch CircleCI's checkout cannot always make, so
  the check failed with `could not fetch … from promisor remote` on a merge that touched no
  protobuf file. The cvc5 download in the same pipeline retries a failed transfer instead of
  failing the job on one bad response from the release host.

- **A pseudostate can carry a quoted name.** `fork 'spread 2';`, `join`, `junction`, `choice`, `history` and `deep history` read their name as every other declaration does, an unrestricted name `'…'` included, where only an identifier or keyword parsed before; a transition then reaches it by the same quoted name.

- **The PSSM referee's translation carries the values a test's constructor writes.** A test
  class whose `<Class>$factory` activity assigns a literal to an attribute of the new instance
  (*Join003*'s `value = 15`, read by the guard of the join's outgoing transition) lost the
  assignment: the attribute was declared without a value and the run failed with `no value for
  feature value` before reaching the guard. The literal is now the attribute's initial value; a
  constructor that does anything else — writes a feature the class does not own, writes
  something other than the new instance, or computes a value — is refused as untranslatable
  rather than dropped. No bucket count moves: *Join003* still fails, now at the join itself
  (`docs/project/pssm-referee.md`).

- Checking a constraint or writing a feature no longer rescans every object's behaviors when nothing has changed since the last scan found them all idle, so a batch of checks over many instantiated objects is linear again.

- **The RDF mapping links references instead of naming them.** A property the SysML v2 API defines as a reference — `sysml:type`, `sysml:importedNamespace`, `sysml:importedMembership`, a succession's `sourceFeature`, a feature chain's `targetFeature`, a feature reference's `referent`, an invocation's `function` — is now the IRI of the element the name resolves to: an element of the graph by its own id, and a standard library element by its normative id whether or not the library is in the graph (`attribute mass : MassValue` links `<urn:sysmlv2:element:9cd0e404-efee-50e5-a59b-681065bd188c>`). Only a name that resolves to nothing the model declares stays a literal, and an element declaring the id the norm fixes for a library element the graph links is refused rather than merged with it. Every metaclass written is concrete, as every element the API returns is: an import is `sysml:NamespaceImport` or `sysml:MembershipImport` and an `expose` is `sysml:NamespaceExpose` or `sysml:MembershipExpose` rather than abstract `sysml:Import`, an import written through an alias links the alias's owning membership, and a KerML `connector` is `sysml:Connector` rather than `sysml:ConnectorAsUsage`. Graphs written by earlier releases still read: the decoder accepts a literal where a link now stands, and the two abstract classes, and writes the current form on the next hop. Name resolution now also finds the `start` an implied `first start then a` names when the action inherits it from the library, a feature chain's target from its featuring usage, and the names in a `dependency` body, so a metadata usage annotating a dependency links its type and reads back from the graph alone.

- **Relationship queries no longer spend their visit budget building edge tables.** `RelatedElements`, `WhereRelated` and relationship-derived columns build the edge table of a relationship kind by scanning every declaration in the workspace, and each declaration scanned was charged to the query's visit budget — so on a model past ~100,000 declarations every relationship query failed with `visit-budget` before traversing anything (a migrated TMT requirements-mapping document, 101,014 declarations, needed about 200 visits for its rows). The scan is a fixed cost of the model, not of the query: it is now memoized per model in `queryexec.Context.Related`, shared by every query a document (or a linked set of documents) evaluates, and left uncharged; the budget still bounds the traversal itself, paying one visit per element reached, and `visit-budget` is still the typed failure when that is exceeded.

- **`-render-all` writes two views whose names differ in letter case alone instead of stopping.** A model naming two views `Report` and `report` (or two migrated Cameo diagrams named `iRIS …` and `IRIS …` in one package) stopped the run with "have the same rendering path", because a filesystem that ignores case would hand both one file. Each such view is now written under its name tagged with `~` and a hash of the encoded name, as a name too long for a path component already is, so the rest of the model still renders; a view whose name meets no other keeps its plain filename.

- **A redefinition is masked along an alias path too.** The workspace built two scope trees for each of its documents — one the index resolved and analyzed, one it enumerated visible names and positions from — while the resolver memoizes a reference's answer by its syntax node, shared by both. Once a document had been analyzed, an alias resolved from the enumeration's tree came back as the index's copy of its target, whose members the redefinition mask (keyed by the enumeration's symbols) did not recognize: at `feature B redefines A` inside `A`, the redefinition being written reappeared as `test.A.A.B` through `alias A for A1`, though not on any direct path. A workspace document now hands its own scope tree to the index (`Index.AddDocumentScope`), so every route reaches one symbol and the pilot Xpect suite is back at its recorded 1296 agreeing expectations.

- **A segment leaving a junction or choice declared inside a composite state runs its effect after
  that state's entry.** A compound transition used to run every effect of its route after its
  exits and before any state on the way down to the target was entered, so a segment out of a
  pseudostate declared in a composite state — the composite being entered on the way to the
  pseudostate — logged its effect before the composite's `entry`. Each effect now runs once the
  states down to the one declaring the pseudostate it leaves are entered: the composite's `entry`,
  then the segment's effect, then the entry of the target below, at every depth of nesting, for a
  junction as for a choice (whose guards are read once the state declaring it is entered and the
  effects into it have run, so a guard testing what that state's `entry` wrote reads the new
  value), for a pseudostate in one region of a parallel state (the parallel state entered first,
  the other regions starting as usual) and for a history's default transition through such a
  pseudostate. A choice whose branches end in different states enters only the states every branch
  enters before its guards are read. The PSSM referee's counts do not move: *Junction 005* now
  reaches an admitted trace and misses only the interleavings of the other region's entry, so it
  stays `fail` on the region-order gap alone, adjudicated in `docs/project/pssm-referee.md`.

- **Fixed a send receiver expression leaking what it built.** A `send … to <expr>` whose expression constructed objects — `send new Ping() via out to new Car()` — left the constructed object and its behaviors behind when the send then failed to deliver; the payload and receiver are now abandoned together. A `to` expression yielding a selected variant — such as a selection over `engine` where `engine::electric` is the chosen variant — now addresses the object the variant materializes instead of reporting that it holds no object.

- **The npm platform packages and the nightly pipeline build statically linked binaries too.** `CGO_ENABLED=0` now governs the npm platform packages as well as `make build`, and `make static-check` runs in the nightly and npm pipelines beside the release and pull-request ones, so a dynamically linked binary fails those builds instead of shipping.

- **Two flows out of one pin deliver twice.** A value a streaming flow carries to a target not yet under way waits at the target's pin, and a later write from the same source performance replaces it; the waiting place was keyed by the source performance and pin alone, so two `flow` declarations out of one pin into one target pin — the two routes of a fork duplicating a token, or a pin named by its inherited and its redefining name — collapsed into one delivery, and a second performance of a nested action definition with a required input ran with the input unbound (`unbound parameter`). Each `flow` declaration is a transfer of its own: the runtime (and the `smt` engine's encoding) keys the place by source performance, pin and flow, so the two flows each stage the write and two performances of the target each take one, while a later write along the same flow still replaces its earlier one (`action_flow_streaming_two_flows_one_pin_to_call`, `action_flow_streaming_aliased_source_pin`; on the fUML suite `ForkMergeData` reaches its recorded `0, 0` again).

- **SysON plugin diagnostics land on the element they are about.** A parser diagnostic inside a nested element now maps to the innermost named element enclosing its line rather than to nothing; a serializer warning that names an element by id is attributed to that element (or its nearest named owner) rather than to the run target; and an anonymous satisfy or constraint usage can be the target of `verifySatisfaction` and `validateInstance`, which run against its enclosing named element.

- **`terminate;` in a braced `entry { … }`, `do { … }`, `exit { … }` or transition `do { … }` block ends the whole block.** A `terminate` among the block's statements ended only the statement it was written in, so the assignments after it still ran — `entry { assign e := 1; terminate; assign e := 9; }` left `e` at 9 where the same body as `entry action a { … }` left it at 1. The block is now the action the `terminate` ends, in a state's and in the machine's own `entry`/`do`/`exit`, in a transition's `do` effect and in the blocks a definition's usages inherit: the statements after it do not run, a `do` block's later `accept` never parks, while a named action beside the block (`entry action first { … }`), the sibling regions' blocks, the state's `do` after its `entry` and the transition's completion after its `exit` or effect run as before (`state_terminate_braced_entry_do_exit_effect`, `state_terminate_braced_do_after_accept`, `state_terminate_braced_entry_among_named`, `state_terminate_braced_inherited_by_two_usages`, `state_terminate_braced_do_in_one_region`; `TestRuntimeRobustnessTerminateBlock`).

- **A document binding may name a nested usage in dot notation.** `in req = specification.mission.range;` in a document's content block resolves the chain to the nested requirement; it was rejected as an unsupported binding before, so a query parameter could only be bound to a top-level usage or a qualified name.
- **A verification case is matched to a requirement by the declaration it names.** `Verdicts(...)` compared the requirement a verification case verifies by scope-tree symbol, so a requirement nested inside a part that the document indexed under a second scope root had its verification cases dropped; the match now uses element identity, and the case's verdict row appears.
- **The pandoc PDF engine no longer receives a `document-css` variable.** Passing `document-css=false` did not turn pandoc's built-in stylesheet off; pandoc read the variable as set, and its screen layout narrowed the page beside the print stylesheet. Only the print stylesheet is passed now.

- **A wide table in a PDF document stays within the page.** The PDF stylesheet let a table grow past the text width when its cells held long unbreakable tokens such as qualified names, so the rightmost columns were cut off at the page edge; tables now take the text width and cells wrap anywhere they must.

- **The SysML v1 migrator reads a transition guard serialized as a reference.** A UML `Transition.guard` some exporters write as a `guard="…"` reference to an owned rule of the transition, rather than as a `guard` child, is now found and written as the `if` clause, reported and kept in a comment when it has no v2 form; before, such a transition was written unguarded and its constraint left out of the report. A `LiteralBoolean` guard whose `value` the file omits is read as `false`, the UML default, where a transition guard is concerned; before, it was taken for `true` and dropped.

- A transition from a substate into the composite state enclosing it no longer restarts that state's default substate: the composite is already active, so it is not re-entered, and the body the substate left completes — a composite with no other region completes and its completion transition fires, a parallel owner waits for its other regions. The PSSM referee's *Transition 011 C* moves `fail` → `pass` (`docs/project/pssm-referee.md`).

- **A case's timed steps run on the clock.** A case body's action flow waited on the clock for
  its own `accept after`, but the clock did not list its waits, so a timed step in an analysis
  or verification case deadlocked; the flow is now on the clock for the run. A case an action
  body performs as a step pauses that body, whose executor lists the case's waits among its own
  and resumes the step when the instant comes, so a wait for a message nothing posts is the
  typed `ErrAcceptDeadlock` of the performing action, and an expression reading a case's output
  while it waits is the typed `ErrCaseReadWaits` naming the wait (`analysis_steps_wait_on_clock`,
  `action_case_step_waits_on_clock`).
- **A state behavior of no content executes as nothing.** A state's entry, do or exit behavior,
  or a transition's effect, written as an action usage with neither a body nor an action
  performed (`entry action hello;`, `do action log`) was refused at run time as performing no
  action; it now executes as nothing, as a bodyless nested action of an action body does
  (`state_behavior_action_of_no_content`).
- **A binding end at a performed action's node reads the body's names.** A `bind` written at a
  node of a `perform action` resolved a simple name to the performing part's feature before the
  enclosing action's same-named parameter, so a parameter given no value read the part's value
  instead of being empty; the name now resolves in the body's scope first, as an expression of
  the body does. A pin valued by its own name (`inout log = log`) reads the feature it masks
  around the usage owning the pin rather than itself, which was refused as a cyclic feature
  value (`performed_action_binding_end_names_parameter`).

- **A parallel region stood for by a stateless state needs no initial.** A parallel state or machine whose direct substate declares no substates of its own is one region that starts in that state and stays there; lowering demanded an `entry; then <state>;` of it and refused the machine with "region … has no initial state", whether the region was a parallel state's or the machine's own. Both now lower and run, and a state's entry, do and exit behaviors, transitions and deferred events do not make it composite (`state_parallel_stateless_region`, `state_parallel_stateless_top_region`, `state_parallel_stateless_region_with_behaviors`).

- **A `via` path that is a bare bound port reference leaves the bound port.** `send … via p` and `accept … via p` under an `in ref port p` that the caller binds to another object's port were rooted at the performer, so the message left or was awaited at the performer's same-named port, or was refused as unconnected; the bound port's owner now sends and receives it, and an action's own connector may end at such a reference by name. A binding that holds an object which is no port is refused as `ErrSendViaNotPort` rather than falling back to the performer.

- **A private v1 property shown on a diagram in another namespace is no longer written `private`.** The view's `expose` — and any layout annotation naming it — must be able to refer to the feature, and v2 hides a private member from every qualified path; the migration report notes "private visibility is not written: view … exposes it".

- **Transition triggers are labelled by the name their signal or operation ends in.** A rendered state or action view wrote an `accept` trigger as its source text, so a migrated transition accepting `TMT::'02 JPL'::…::Control::'Post-Segment Exchange Alignment'` carried that whole path across the drawing. The DOT, Mermaid, PlantUML and text forms now head the trigger by its end name — `accept 'Post-Segment Exchange Alignment'`, `accept msg : Halt`, `accept setSpeed(value)` — the way a node's type is headed; time and change events keep their written text.

- **`.sysml` and `.kerml` files are recognized in VS Code Restricted Mode.** The extension now declares limited untrusted-workspace support, so files are no longer opened as Plain Text when the folder is untrusted; looking up `bin/sysml-lsp` inside the workspace requires a trusted workspace.

- **The diagram panel shows an element table as a table.** A table-kind view or the `#table` pseudo-view was written into the panel as its Markdown source; it is now drawn as a table from the rendering's rows, and clicking a row opens its element in the editor.

- **Every witness `check` writes replays.** A replay past its witness's last choice line went on as `reverse`, a sweep giving every token its turn in one step, where the checker that wrote the witness made one move a step to the end; a `do` body looping through timed waits kept stepping after the last order the checker had to record, so the replay left another trace and `-engine check` reported `replay disagrees with the witness` — the runtime showcase's spacecraft, checked on `SpacecraftComms::mission.spacecraftVehicle`, wrote a witness of `battery = 39` it could not reproduce. A replay now stays one token a step past the witness, picking as `reverse` would, and the run it re-makes is the checker's (`state_do_action_loop_timed_exit`). An action performed inline in another's flow steps its own tokens within the performer's step, and the checker records the inner branches' order at the performer's step number; replay followed that line against the performer's frame, where the performer's token alone is able to act, and refused it — it now follows an order over the tokens of the flow holding them all, so the line is resolved in the inner flow, and an order naming a token no flow holds is refused once the step is past (`TestCheckWitnessesOfAnInlinePerformanceReplay`, `TestRuntimeRobustnessReplay`).

- **XMI metadata attributes on stereotype applications are no longer emitted as stereotype tags.** Migration now ignores tool metadata such as `xmi:uuid` instead of reporting it as an unsupported tag.

### Performance

- Validating a model rich in membership imports no longer re-scans the names registered under a segment on every lookup: `ShortNamed` is memoized per index generation, undoing a ~25% whole-model validate slowdown introduced with the import-prune fix.

- **A workspace keeps its semantic model between edits and invalidates it per document.**
  `model.Workspace` owns one `resolve.Resolver` and one `semantics.Model` for its lifetime and
  hands them to every analysis it runs; the resolver keeps a frame per document owning what was
  memoized while that document was analyzed and records which documents it read (a namespace it
  imports that another contributes to, a namespace both contribute to, a symbol of another that a
  resolution returned). Replacing a document drops its frame and, transitively, its dependents' —
  their memo entries, cached diagnostics and reverse references — and nothing else, where every
  edit used to clear the whole workspace. The OOSEM, MOSA and identity-metadata audits and the
  coherent-quantity ranking gather each document's facts once into the workspace and judge each
  analyzed document over the union, where they gathered every document once per document
  analyzed. `TestIncrementalEqualsFresh` replays scripted and random edit sequences over the
  fixtures and the OMG corpora and compares diagnostics, resolutions and references with a fresh
  workspace after every step. On the satellite-network stress test, editing a two-line file beside
  512 satellites goes from 861 ms and 327 MiB per edit to 8.7 ms and 2.0 MiB; editing the library
  every file of the split network imports costs one analysis of the model (8.8 s to 5.3 s at 512
  satellites), and loading the 1 600-satellite network split into 34 files through one workspace
  goes from 126 s to 18 s. A loaded workspace holds about twice the heap (254 MiB to 478 MiB at
  512 satellites), the memo tables that were allocated and discarded on every analysis, and a
  thousand edits grow it by 4.5%. A one-shot `sysml -validate` pays the dependency recording it
  never uses: about a sixth more wall time (1.9 s to 2.2 s at 200 satellites) and 4% more
  allocation. Figures and the machine they were taken on are in `docs/internals/performance.md`
  and `docs/project/satellite-network-stress-test.md`.

- **A references query made right after an edit is slower than in 0.8.1, in exchange for
  incremental invalidation.** The resolver now records which documents and names each
  resolution read, so an edit invalidates only what depended on it: a rename or an edit beside
  a large document is several times faster than before. The recording is paid on the first
  query after an edit that walks a long wildcard-import chain, where cold references measure
  about 40% slower on the LSP benchmark; the query itself returns the same locations.

- **Resolving a name through a scope no longer rescans every anonymous member of that scope for implicit parameters.** The resolver used to walk all anonymous members and test each for an implied redefinition on every unqualified lookup, so a definition with many anonymous interface usages cost more per name the larger it grew. The candidates are now collected once per scope and journaled with the other per-scope caches; validating a 1 600-satellite constellation of fully modeled spacecraft drops from 30 s to 19 s.
- **Checking `n` satisfy assertions no longer walks the model `n` times for verification cases.** The runtime collected every verification case beneath the model root on each satisfaction check; the walk is now memoized per scope for the life of the runtime model, so `sysml -satisfy` over 600 assertions on a 200-satellite constellation drops from 5.9 s and 2.2 GiB allocated to 3.9 s and 1.5 GiB.

- The `~` undefined-operator warning now reads the operator sites the parser records instead of walking every node of every document, removing about 8% from load and validation time.

- **A state machine's poll of the signals in flight is memoized.** A run holding many active
  objects probed every state machine's transitions against every queued message at each
  scheduling step; the probe's answer is now kept until a queue, a write, a nested call, a
  rollback or another executor changes what it could see, which makes a long stochastic run of
  a model with many active parts several times faster with the same trace.

## 0.8.1 — 2026-09-16

### Added

- **The VS Code diagram opens with a keystroke or a click.** `SysML: Open Diagram` is bound to <kbd>Alt</kbd>+<kbd>D</kbd> (<kbd>Option</kbd>+<kbd>D</kbd> on macOS) and <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd> (<kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>V</kbd>), active only in a `.sysml` or `.kerml` editor or in the diagram panel itself, where the same key returns to the source. The command also sits in the editor's title bar and right-click menu and in the Explorer's menu for model files, which opens the file and its diagram side by side; `SysML: Export Diagram` joins it in the editor menus. Without a running server, or one too old to draw, the commands say so instead of greying out.

### Fixed

- **The SysML v1 migration reads an opaque body expression's own names as its own.** An opaque expression written in v2 syntax whose body expression declares parameters or members — `{ in v; v > limit }` — no longer fails to copy because `v` is not a feature of the surrounding block; only the names the body reaches for beyond its own are checked for visibility, including those in a nested body, a nested constraint or a multiplicity bound (`in v { attribute y[limit]; }`). A body declaring a member kind the check does not read, such as an import, is left unmapped rather than copied unchecked. A feature chain on one of the body's own names is checked against the type the name is declared with — `{ in v : Pt; v.missing }` is unmapped rather than copied, and `{ in v; v.x }` is unmapped because `v` declares no type to check `x` against. A qualified name starting in a standard-library package, `ScalarValues::Integer` or `ISQ::mass`, is checked against the bundled library, which a v2 model reaches without importing it; a bare `Integer` is still not visible without an import. What the expression is followed by must be nothing, so `c > 0.0; attribute k = c` is still not one expression.
- **The SysML v1 migration keeps an empty slot of a required feature as a comment.** A slot holding no value for a feature of multiplicity 1 or more contradicts the feature just as too many values do, and is now left unmapped with that note instead of being written as a redefinition bound to nothing. An empty slot of an optional feature is still written.
- **The SysML v1 migration does not redefine a feature of a classifier the individual leaves out.** An instance classified by both a block and a constraint block is written as an `individual part def` of the block alone; a slot for a feature of the constraint block is now left as a comment rather than redefining a feature the individual does not inherit.
- **The SysML v1 migration keeps a typed-in number of any size.** A string default spelling a whole number or a real for a numeric feature is now checked exactly, so `"9223372036854775808"` for an `Integer` and `"1e400"` for a `Real` are written as those values instead of being left as comments for exceeding a machine word or a double.
- **`-check-timeout` (`%check-bounds timeout=`) is the `smt` engine's solver clock as well as
  the plan's.** Each solver query of a check runs under the check's timeout in place of
  `OPENSYSML_SMT_TIMEOUT`, so a check told it may run for `2m` is no longer left *not covered*
  by a query the solver's own 10 s default cut short; the `solver` bound the result names is
  the clock the query ran under. Without a timeout the queries keep `OPENSYSML_SMT_TIMEOUT`.

- **A library copy whose root package states only a short name is read as the library.** The
  notation-side library check looked the root up by its long name alone, so a copy opening with
  `standard library package <Occurrences> {` fell through to user-document analysis and its
  elements took derived ids. The check now uses the name the symbol table registers the package
  under — the long name, else the short name — as the graph-side check already did.

- **Every Mermaid flowchart `subgraph` now states the flowchart's `direction`.** Mermaid lays
  out a subgraph that states no direction without regard to the flowchart's, so an action
  rendering declared `flowchart TD` drew its container's contents left to right, and an
  interconnection or action rendering with `direction BT` or `RL` lost the direction inside
  every container. Each `subgraph`, nested ones included, now opens on `direction <flow>` —
  `TD`, `LR` for an interconnection, or the direction the view or the caller asked for — so
  the drawing follows the declared direction throughout. A tree draws containment as edges
  rather than subgraphs and is unchanged.

- **The landing page's four refereed-comparison cards no longer wrap three and one.** The grid
  now lays them out in one row of four on wide screens, two rows of two below the width at which
  four fit, and a single column on phones, instead of letting the fourth card fall alone onto a
  second row.

- **Every build of `sysml`, `sysml-lsp` and `sysml-grpc` is now statically linked, not only the release job's.** `CGO_ENABLED=0` moved from the release scripts into the Makefile's build and install targets, so `make build`, `make install` and the pull-request build no longer link the builder's glibc either; a Linux `sysml-grpc` built that way needed glibc 2.34 where the release binary did not. `make static-check` (`scripts/check-static-binaries.sh`) verifies the Linux binaries, and the release and pull-request pipelines run it, so a dynamically linked binary now fails the build instead of shipping.

- **A transition's `accept` trigger payload is a member of the transition.** The parameter an
  accept trigger declares (`transition t first a accept p : Payload then b;`) was catalogued in a
  scope of its own, so it had no owner, no qualified name and — for the one such parameter in the
  standard library, `Actions::AcceptAction::aState::aTransition::apayload` — no normative id, the
  last named library element whose id differed from the pilot's XMI. The symbol index now defines
  it in the transition's own scope beside the effect and body members, so `t::p` names it, the
  normative catalog derives its id under the transition, and `TestPilotLibraryXMI` lists no
  pilot-only element. The guard, effect and body still reach it as before; any other reference
  to it reports `Must be an accessible feature`, as the pilot does.

- **The SysML v1 migration reads past the UML metaclass where the tool's own encoding hides the
  v2 form.** A Signal is an `item def`, and properties typed by one are `item` / `ref item`, not
  `attribute def` and `attribute`. A constraint block's parameters are public `in attribute`s —
  `in ref part`s when typed by a block — whether the tool stores them as UML Properties or, as
  MagicDraw does under a «ConstraintParameter» marker, as UML Ports, and a `private` parameter loses its visibility so
  the block's binding connectors can reach it — as does any private feature a connector, slot,
  redefinition or subset reaches from outside, the report naming what reached it; a connector
  or slot that is itself left as a comment reaches nothing. A private packaged element (a
  block, value type or enumeration) is written public, with a note, since v2 would put it out
  of reach of the packages importing it. A type
  referenced by href into the SysML or UML primitive library resolves to `ScalarValues::Real` /
  `Integer` / `Boolean` / `String` from a plain (`PrimitiveTypes.xmi#Real`) or dotted
  (`SysML.xmi#SysML_dataType.Real`) fragment, or from the qualified name MagicDraw records
  beside an opaque id (`referentPath`) into a module named for that library; the tool library's `float`, `double`, `int`, `long`,
  `short`, `byte` and `boolean` are written as the matching scalar and reported as
  approximations. A nested connector end's `propertyPath` given as one whitespace-separated
  attribute is split into its ids rather than failing to resolve. An opaque expression is copied
  only when it parses as v2 and every name it uses is a written element visible where it is
  written, so a JavaScript body, a bare enumeration literal or a call to an operation stays a
  comment, and a private inherited feature an expression names is exposed like one a connector
  reaches. An instance of a value type is
  an `attribute` typed by it rather than an `individual def` that cannot specialize an attribute
  def; a slot contradicting its feature — more values than the multiplicity allows, a repeated
  value of a unique feature, a feature of a classifier the instance is not written to
  specialize — is left as a comment; a real
  literal on an `Integer` feature and a numeric string on a scalar feature take the feature's
  scalar, a literal on a value type or enumeration with no scalar base is not bound, and a
  default naming an instance of a block types the usage by that individual — its only type when
  the property is untyped, and not at all when the usage is a port, of another kind than the
  individual, or typed by a block the individual is not an instance of — instead of being written
  as a value. An instance of a block is an
  `individual part def` and of a constraint block an `individual constraint def`, and a slot of a
  part, item or constraint property is written too: one instance redefines the property as an
  `individual part :>> x : 'the instance';`, several each subset it under a redefinition
  counting them, while a slot whose instance is not of the property's type, or differs from the
  individual its default types it by, is left as a comment. An undirected part or item property of an interface block is a `ref`,
  since a port owns no composite parts, and a specializing block's property named like an
  inherited one redefines it when both are the same kind of usage, and is reported when they
  are not. Migrating the current TMT observatory model now yields notation
  with no analysis errors, down from a hundred, and keeps the structure of its instance trees.

- **The VS Code extension's test runner finds `src/` on Windows.** `tools/test.mjs` derived its
  `src/` and `out/` directories from a file URL's `pathname`, which on Windows carries a leading
  slash before the drive letter, so `path.resolve` prefixed the current drive again and
  `npm test` / `npm run package` failed with `ENOENT … scandir 'C:\C:\…\src'`. The paths now come
  from `fileURLToPath`, which yields a native path on every platform.

- **A diagram drawn from an older `sysml-lsp` no longer labels a typed usage `undefined`.** A
  server predating the node `type` field in `opensysml/render` sends no such field, and the
  VS Code panel wrote the missing value into the label as `engine : undefined`. The extension
  now fills in what an older server omits — a node's `type`, `name` and `detail`, an edge's
  `label`, absent lists — before drawing, so such a usage reads `engine`, «part», `Engine`, as
  that server's own client drew it, and an unnamed element leads with its kind alone.

## 0.8.0 — 2026-09-14

### Added

- **Every verdict now states its standing, and the analysis engines can be listed and chosen.**
  Each verdict the CLI, the REPL and `-json` report is followed by a `standing:` line — the
  claim, the strength of the evidence behind it (*not covered*, *observed*, *witnessed*,
  *bounded*, *proved*) and what earned it, such as `holds (observed: 1 run under reverse)` or
  `outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))`; a budget the
  engine reached is named and lowers the strength, never a proof. `sysml -engines` and `%engines`
  table the engines of the build with the authority each carries, the questions it answers and
  whether its process was found (`solve` reports the solver it discovered); `-engine
  <name>|auto|all` and `%engine` select the engine every check is put to: `auto` (the default)
  is the dispatch every check had, a name puts the question to that engine alone with its refusal
  as the verdict, and `all` puts it to every covering engine in name order and composes their
  answers — a witnessed violation stands over any universal claim, a universal claim an execution
  refutes is a disagreement resolved in the interpreter's favor with the refuted result demoted to
  *not covered*, and an engine cancelled by the plan's deadline is kept in the plan with the bound
  it reached. `-engine explore` is `-schedule explore`. `-json` checks gain `plan` (the selection,
  the composed standing, each engine consulted and its status, the disagreements) and
  `results[]` (one entry per engine that answered: `engine`, `claim`, `strength`, `bounds`,
  `witness`, `standing`) beside the keys they always carried. The service adds `ListEngines`, an
  `engine` field on the verification, calculation, analysis and sweep requests (unset meaning
  `auto`) and `engine`, `strength` and `bounds` on their responses and on every `Verdict`,
  advertised as the `engines` capability; the Python client takes `engine=` on its verification
  and analysis calls, reads `Verdict.engine`, `.strength` and `.bounds`, and lists engines with
  `Connection.list_engines()`; the Go client takes `WithEngine`/`Engine`, gains `Calculate` — `EvaluateCalc` with
  options — taking `CalcArguments` and `CalcEngine`, reads the `Standing` of
  every verdict, calculation and analysis, and lists engines with `Client.ListEngines`. No
  existing flag, command, RPC, field or key changed its meaning.

- **External analysis engines over standard input.** A directory named by `OPENSYSML_ENGINES`
  holds one JSON file per engine — `kind: engine`, its `name`, `version`, `command`, the
  question kinds it `answers`, the declaration kinds it takes as `subjects`, the `model` forms
  it reads, its `bounds`, the `witness` kind it gives, its `authority` and whether it is
  `concurrent` — and each registers under its name beside `run`, `explore`, `check`, `sweep`
  and `solve`. A plan that reaches the engine starts its command once, hands it the model as
  `sources` (the documents) or `graphs:1` (a new versioned export of the lowered action and
  state graphs — guards, triggers, effects, pseudostate edges, footprints — byte-stable across
  runs and `-jobs` counts), and speaks JSON-RPC lines to it: `describe`, checked against the
  manifest field by field; `covers`; `run`; `cancel`; and `progress` notifications the CLI and
  REPL print to standard error, coalesced to a few a second. The message set is published as
  `docs/reference/engine-protocol.schema.json`. Nothing an engine claims is trusted: a
  `violated` stands as *witnessed* only when its schedule replays under `replay:` and the
  condition is false at the move it names, `sensitive` needs two replaying schedules that end
  the named feature differently, `satisfiable` needs an assignment the evaluator confirms, a
  universal `holds` is *observed* over the `executions` that replay and otherwise *not covered*
  with the claim kept in the reason; `admit` is refused until referee records exist. Every
  failure — a program that does not start, a `describe` that disagrees with the manifest, a
  broken protocol line, an error the engine reports, an exit mid-run, a cancel unanswered by the
  deadline — is a typed *not covered* answer naming it, with the engine's standard error, and
  `auto` advances past it. `sysml -engines` and `%engines` list external engines with their
  kind, protocol and status without starting them; `-engines -probe` and `%engines probe` start
  each once to check its `describe`; `-engine <name>` and `-engine all` reach them like any
  engine. `ListEngines` gains `kind`, `protocol`, `source`, `command`, `version` and `served`,
  and `sysml-grpc` lists external engines but refuses to run them (`failed_precondition`) until
  started with `-serve-external-engines <names|all>`, which advertises `engines_external`.
  `policy`, `sampler` and `module` entries, the `grpc` transport and the `rdf` model form are
  parsed and listed `unavailable` with a reason naming the stage that serves them.
- **`OPENSYSML_TOOL_MAX_OUTPUT`** (default `64M`) bounds what one external process may write
  before it is cut off — a tool's one reply and its standard error, an external engine's one
  protocol line and its standard error — and a tool's relative `executable` path is confined to
  its manifest directory as an engine's `command` is.

- **A design note for the analysis framework**
  (`docs/internals/design/analysis-framework.md`). It proposes one engine contract that the
  interpreter, the `explore` scheduling policy, the parameter sweep, the SMT constraint solver,
  the proposed model checkers and external analysis tools (`AnalysisTooling::ToolExecution`)
  register against; one scale for the strength of an answer — proved, bounded, witnessed,
  observed, not covered — under which a faster engine's observation never outranks a slower
  engine's proof; dispatch by question with explicit fallback and a referee mode that runs every
  covering engine; and runs isolated over shared immutable model state so sweep rows, explored
  linearizations and solver queries can run in parallel with results identical to the sequential
  ones. Every existing flag, command, RPC and field keeps its meaning. Nothing is implemented;
  the note exists to be reviewed before code is written.

- **A sweep over a held object runs on the object as the session holds it, behaviors included.**
  `%sweep` on an object from `%instantiate`, `-instantiate` followed by `-sweep`, and the gRPC
  sweep over such a subject now run when the object's type exhibits a state machine or performs
  an action, where they were refused before. The runtime records whether each execution has
  moved since its start — a token stepped, a body statement run, an accept or wait consumed, a
  transition fired, a timer or change trigger taken, a feature written — and a held object whose
  executions are all unmoved sweeps from its declaration in every row's context, printing the
  table the sequential form printed. A held object that has moved, been written, been sent a
  signal not yet dispatched, waits on a clock that has moved, runs a behavior while a signal
  open to any taker is in flight, or is named by `#<id>` is swept from one image of it and everything
  it holds, taken when the sweep begins and made afresh in each row's context under the same
  identities, with the executors' state and the posted signals: every row starts where the held
  object stands, no row sees another's writes, and the held object is byte for byte as it was
  afterwards. An object the image cannot carry — a destroyed one, a body paused mid-statement, a
  debugger inside a step, a value bound to the run that made it — is refused naming the reason
  before any row runs, never swept on the session's state.

- **Analysis runs in parallel under `-jobs`, with the same answer whatever the count.**
  `sysml -jobs <n>`, `%jobs <n>` in the REPL and `OPENSYSML_JOBS` (default one per CPU; the
  flag overrides the environment) set how many runs of one check go at once, each on a worker
  of its own — a resolver and semantic model per job over the one loaded model, so no run sees
  another's memo. `-schedule explore` runs its linearizations on a work queue of prefixes
  ordered as the sequential exploration would take them, and reports the outcome table, each
  outcome's witness, the run count and the budget hit `-jobs 1` reports, byte for byte: a
  `runs` budget is a cut in that order, at most `n` runs beyond it are ever started (so an
  exploration performs at most `runs + n` executions), and a run that fails is an outcome of
  the table, as under one job.
  `-engine all` puts the question to its covering engines at once and composes their answers in
  name order; a fault or deadline stops the engines after it, each kept in the plan with the
  bound it reached, and a run serving a universal claim is not cancelled by a witness. A count
  below one, or one that is no integer, is refused before anything runs. `-json` checks carry
  `workers` and `warming` (the milliseconds spent building them) under `plan`; the
  human-readable report does not print them. The gRPC service takes its count from
  `OPENSYSML_JOBS`; no request field changed.

- **Actions annotated `ToolExecution` run through an external tool.** A directory named by
  `OPENSYSML_TOOLS` holds one JSON file per tool — its `toolName`, `version`, `executable` and the
  `variables` it accepts — and each registers a `tool:<name>` engine that `sysml -engines`,
  `%engines` and `ListEngines` list with its process status like `solve`. A performance of an
  action carrying `AnalysisTooling::ToolExecution` starts the tool once, writes one JSON request
  (`toolName`, `uri`, `inputs` keyed by `ToolVariable` name with value and unit) to its standard
  input, reads one JSON reply from its standard output and binds the `outputs` to the action's
  parameters converted to their declared units; the body is never run. A tool with no entry is
  refused with `tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS`; a non-zero exit,
  malformed or missing output, an unknown output, a unit the model does not declare or the
  timeout `OPENSYSML_TOOL_TIMEOUT` (default `10s`) fails the performance with a typed error, and
  no value is ever invented. A tool's answer stands at strength *observed*; equal inputs
  answered differently are noted as a divergence of the run.

- **Non-conforming operator and invocation arguments warn `Bound features should have conforming
  types`, as the reference does.** Each argument of an operator or invocation expression is bound to
  the parameter it fills (KerML 1.1 §8.3.4.8.3), and the type checker now judges that implied binding
  with the rule an explicit `bind` gets: an argument whose static type conforms neither to nor from
  the selected function's parameter type — `rearWheel + 1` with `rearWheel : Wheel`, `sum(robots.mass)`
  passing `MassValue`s to `RealFunctions::sum` — draws the warning, at the argument of an invocation
  and at the whole operator expression, where the reference puts it. Positional, named and receiver
  arguments are judged alike, through feature chains and nested invocations. Nothing is reported where
  either side is unknown — an unresolved or ambiguous callee, an untyped, unbound or collection-valued
  argument, a parameter typed by a `Collection` or `Element` — or where a precise type error already
  covers the argument, and a conforming argument (`Integer` into `Real`, `MassValue` into
  `ScalarQuantityValue`) stays silent. The pilot Xpect `warnings` kind agrees 113 of 113, and the two
  reference-only rows on `examples/disposal-team-demo/team.sysml:29` are agreed.

- **A design note for bringing your own engine**
  (`docs/internals/design/bring-your-own-engines.md`). It proposes how a user adds to the analysis
  framework without changing OpenSysML: an engine of their own that answers questions about a
  model, a scheduling policy or sampler of their own inside a built-in engine, or a tool of their
  own, registered from a manifest the environment names and run as a process speaking a JSON-RPC
  message set on standard input, as a WebAssembly module, or as Go over the public
  `client/opensysml` package. Every external witness is checked by the interpreter before it
  counts — a schedule replayed and the claim evaluated at the move it names, an assignment
  evaluated; an external universal claim is *observed* only over executions the interpreter
  replayed and is otherwise *not covered* with
  the claim kept, until the site admits a strength against a referee record earned on a corpus
  under `-engine all`; and nothing a model, a workspace or a remote client can cause an
  executable to run. Nothing is implemented; the note exists to
  be reviewed before code is written.

- **`-engine check` searches every schedule of an action.** The `check` engine is an
  explicit-state model checker over the action `-action`/`%action` names: it takes the run one
  move at a time — one token advancing one node, a body being one move — snapshots the executor
  before each choice and backtracks to take every other, so every schedule the library admits is
  visited and no other, and reports what it finds: a constraint or requirement named with
  `-check-property <name>` (`%check-property`) that is `false` at a stable state, a deadlock or
  a typed error a body raises, each as a *violation* on the schedule that reaches it; a feature
  that ends with different final values on different schedules as *divergent* with every value
  it takes (`-check-diverge <feature>`, `%check-diverge`; by default every attribute of the
  action and of its performing object, an action run without one on its own attributes);
  otherwise `no violation, exhaustive` when the search finished, or `no violation within bounds`
  naming every bound it reached — `-check-depth` (moves along one schedule, 10 000),
  `-check-states` (distinct states, 1 000 000), `-check-timeout` (the plan's clock; a search it
  stops is reported `incomplete: time`, not as a verdict) and the executor's own budgets, set
  in the REPL with `%check-bounds`. Two moves whose statically computed footprints are
  independent are searched in one order only, and a state already visited is not searched
  again. `-check-witness <dir>` writes one file per violation and divergent value, named for
  the action and the object performing it — the schedule's choice lines, a blank line, then
  the run's trace — and each is reported
  *witnessed* only after the interpreter replayed it to the state it claims; `-schedule
  replay:<file>` and `%replay <witness>` step that run under the ordinary debugger. The CLI
  exits `1` on a violation or a divergence, `0` on an exhaustive clean search and `2` on a
  bounded, cancelled or refused one; `-json` carries a `check` object (`verdict`, `states`,
  `moves`, `depth`, `boundsHit`, `violations[]`, `divergent[]`, `outcomes[]` and the witness
  paths) on the engine's `results[]` entry. The engine is listed by `-engines` and `%engines`
  at authority *bounded*, `auto` never picks it over `explore`, and it refuses with a typed
  reason a `-check-*` flag without `-engine check` or the engine without a behavior; the
  search itself is single-threaded, `-jobs` dividing only the replay of its witnesses.

- **Function values compile natively.** `sysml -compile` (C and Go) now accepts a calc that
  takes an `in calc` parameter, a calc def, a calc usage with an unsupplied input or a compiled
  library function (`RealFunctions::sqrt`, `floor`) passed for one, `f(a)` and `f(v = a)` in the
  body — positionally, by name, passed on to another calc and through recursion — and
  `SampledFunctions::Sample(f, xs)` bound to a `SampledFunction` attribute or read by `Domain` and
  `Range`. The compiler fixes each function value at compile time and compiles the callee once
  per distinct binding, so `f(a)` is a direct call and computes, prints and fails exactly as the
  interpreter's invocation does (`Apply(Recip, 0.0)` divides by zero, `Sample` reports its first
  failing element, a null domain — a literal `null` included — samples to `[]`, and a parameter
  typed by a calc, `in calc f : Sq`, takes only a function value whose calc conforms to `Sq`).
  What a compile-time value cannot express keeps
  a typed refusal naming the construct: a function value returned, stored, compared, chosen by an
  `if` at run time or handed to a value parameter, a calc owned by a part or declared in a
  behavior body (its value closes over that object or run), an entry calc's own `in calc`
  parameter (a program cannot take one on its command line), a control operation such as
  `collect` passed as a function, and a `SampledFunction` used as anything but the operand of
  `Domain` or `Range`. `docs/project/native-compilation.md` states the representation and its
  trade-off.

- **The VS Code diagram panel edits the model.** An **Add…** menu on the panel and a right-click
  menu on every node the file declares add a member (`part`, `port`, `state`, `action`, a `def`, …),
  add a connection, flow, succession or transition between two nodes, rename a declaration, or
  delete one — with a confirmation, and a second one before a delete cascades to the declarations
  that refer to it. The kinds offered follow the diagram: an interconnection diagram offers parts,
  ports and connections, a state diagram states and transitions, an action or sequence diagram
  actions, control nodes and successions, a tree every kind the language has — `subject`, `actor`
  and `stakeholder` only on a requirement or case, `objective` only on a case. Every action is a
  source-preserving edit of the `.sysml` or `.kerml` file, applied to the editor's buffer like typed
  text: <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it, comments and layout outside the edited lines are
  untouched, and the diagram redraws from what the file now says. An edit that would leave the file
  with an error it did not have is refused and the message names the diagnostic. Layout is not
  persisted and nodes are not dragged.
- **`opensysml/applyModelEdit` LSP request.** Turns a list of model operations (`setValue`,
  `rename`, `addMember`, `addConnection`, `delete`) on a document at a stated version into a
  versioned `WorkspaceEdit` the client applies itself, so the change lands in the editor's own undo
  history and the server learns of it through `textDocument/didChange`. A version that no longer
  matches is answered `stale`; an edit the re-analysis refuses is answered with the operation at
  fault, a stable failure name, the diagnostics the edited text would have had and the declarations
  still referring to a target. A delete or rename that another document of the workspace refers to
  is refused as `referenced-elsewhere` rather than applied to the one document the edit rewrites. The server advertises it as `experimental.openSysmlApplyModelEdit`.
  `opensysml/render` now gives each node its qualified name (`fqn`) for the request to target, the
  namespaces declaring it (`owners`) so a connection between nodes a view draws apart still goes into
  the declaration they share, and a `palette` naming the member and connection kinds a diagram of that kind offers and, for a member
  only some bodies declare, the nodes it may go into.
- **Source-preserving connection edits.** `internal/core/edit` gains `OpAddConnection`, which writes
  a `connection`, `interface`, `allocation`, `binding`, `flow`, `succession` or `transition` (KerML:
  `connector`, `binding`, `flow`, `succession`) into an owner's body with its ends spelled as they
  resolve from that scope, and refuses a kind the language does not have, a type on a kind that takes
  none, an end that does not resolve, or a name already taken.

- **A model can say where a view draws its elements, and every rendering carries it.** The
  bundled `DiagramLayout` library declares `Layout` (`x`, `y`, optional `width`, `height`,
  `collapsed`), `Route` (an edge's waypoints, flattened `x0, y0, x1, y1, …`) and `Canvas` (a view's
  `unit`, `width`, `height`), in pixels from the top-left corner. A `metadata Layout about <element>
  { … }` stated in a view's body places the element in that view; an `@Layout { … }` inside the
  element's own body is what every view that does not place it falls back to. The geometry rides
  the rendering tree (`Node.Geometry`, `Edge.Route`, `Rendering.Canvas`) in the tree,
  interconnection, state and action renderings; Mermaid keeps it as `%% layout:`, `%% route:` and
  `%% canvas:` comments after the header, the text form appends `at (x, y)`, `size w×h`, `collapsed`
  and `via (x, y) …`, and `opensysml/render` adds optional `x`, `y`, `width`, `height`, `collapsed`,
  `route` and `canvas` fields. A model without layout annotations renders byte-for-byte as before.
- **Validation of layout annotations.** `-validate` and the REPL warn about a `Layout` or `Route`
  on an element the rendering draws no node or edge for, and about a second position for one
  element in one view (the first applies); a `Route` with an odd number of values, a non-constant
  binding, a `Canvas` binding only one of `width` and `height`, and a `Canvas` stated outside the
  body of the view it sizes are errors.

- **DOT diagrams can be filled from a named colourblind-safe palette.** `-render-palette <name>`
  on `-render` and `-render-all`, `%render <name> dot <name>` (completed with <kbd>Tab</kbd>), a
  `palette` field on `opensysml/render`, and a `palette` attribute on a document's `Diagram` block
  fill the DOT form's nodes by keyword family — a `part def` and its `part` usages share a hue, a
  `port` takes the next, and so on through item, port, attribute, action, state, requirement,
  constraint, connection, interface, use case, case, allocation, analysis, verification, enum,
  occurrence and flow. The palettes are `okabe-ito` (Okabe & Ito), `tol-bright`, `tol-muted` and
  `tol-light` (Paul Tol), `brewer-set2` and `brewer-dark2` (ColorBrewer), and the sequential
  `viridis` and `cividis` (matplotlib), which are sampled evenly across the families a diagram
  draws. A definition is filled with the family colour and a usage with a lighter tint of it, both
  bordered in the colour; every fill is lightened until black text on it reads at the WCAG 2 AA
  ratio of 4.5:1, and pseudo-states, control nodes and cluster borders stay black and white. The
  palette applies to the DOT form alone: a Mermaid artifact notes it as `%% not represented:`,
  the text and Markdown forms ignore it, and an HTML document figure carries it as `data-palette`.
  A name that is no palette is refused on every surface with the names there are (`invalid-palette`
  in a document plan, `unsupported-palette` on a table or sequence diagram). The default, with no
  palette named, is the black-and-white style.

- **The DOT render form draws in the SysML v2 Pilot visualizer's Standard B&W style.** A
  `-render-form dot`, `%render <name> dot`, `opensysml/render` or document DOT artifact is now
  drawn as the OMG SysML v2 Pilot Implementation's PlantUML visualizer draws it, after the
  `sysmlbw` PlantUML skin by Hisashi Miyashita (Mgnite Inc.) shipped with the Pilot: Helvetica
  text, white fills, thin `#181818` lines (`penwidth=0.5` on nodes, `1` on edges, edge text at
  13 pt), a definition square and a usage `rounded`, the name in bold over the `«keyword»` line in
  italics, clusters unfilled with black borders (`1.5` for a package, `0.5` for an element or a
  region), a connection at `penwidth=3`, and an unnamed initial or final pseudo-state as the
  filled black UML dot. The `graph`, `node` and `edge` defaults open every digraph and a node
  lists only what it deviates in. Nothing else moved: node IDs, label text and escaping, `pos`
  splines, cluster anchors, `lhead`/`ltail`, the header comments and the order of nodes and edges
  are as before, so a diagram laid out from the old text lays out the same. Producing DOT still
  needs no Graphviz installation. The translation, what the skin says that Graphviz cannot draw,
  and the provenance are recorded in `docs/project/view-rendering-forms.md`.

- **A view renders as Graphviz DOT with the new `dot` form.** `sysml -render <view> -render-form
  dot` (and `.dot` files under `-render-all -render-form dot`), `%render <view> dot` at the
  prompt, and `"form": "dot"` on the editor's `opensysml/render` request write a tree,
  interconnection, state or action rendering as a `digraph` for the `dot` engine — containment
  as `subgraph "cluster_…"`, the flow direction as `rankdir`, states as rounded boxes with
  `point`/`doublecircle` pseudo-states and trigger/guard/effect labels, and edges styled as the
  Mermaid form styles them — with `// view:`, `// kind:` and `// layout: dot` header comments and
  one `// not represented:` line per notice. Producing DOT needs no Graphviz installation; it is
  written for Graphviz toolchains and for layouts of graphs larger than Mermaid draws. Mermaid
  stays the default machine-readable form. A `sequence` or `table` view has no DOT form and is
  refused the way a wrong form always was.
- **A document renders its diagrams as DOT on request.** `sysml -render-document … -diagram-form
  dot` (and `-render-documents`), `%render-document <name> dot` at the prompt, and
  `"diagramForm": "dot"` on `opensysml/renderDocument` write every graph-shaped `Diagram` block
  of the document as a fenced ` ```dot ` block in Markdown and as `<pre class="dot">` in HTML;
  the PDF backend keeps the source under a notice that it does not draw DOT, looking for no
  Graphviz tool. Mermaid stays the default, and a table-kind block is a table either way. The
  form is a choice of the render, not of the model: a `Diagram` block states what is drawn, and
  no attribute names the notation. An unknown form, or `dot` on a document holding a `sequence`
  diagram, is a typed error.
- **The DOT form draws a view where the model places it.** A rendering's `DiagramLayout` geometry
  is written as Graphviz reads it: a positioned node is pinned at the centre of its box
  (`pos="x,y!"`, `pin=true`, one pixel to one point with y measured up from the canvas's bottom
  edge), a stated size is `width`/`height` in inches with `fixedsize=true` and an unstated one is
  fitted to the label so the box's corner stays put, a collapsed node keeps `comment="collapsed"`,
  a positioned cluster states its `bb` and pins its anchor at the centre, a `Route` is the edge's
  `pos` spline through its waypoints (a route of one waypoint is noticed, not drawn), and a sized
  `Canvas` is a `// canvas:` header line and an invisible point pinned at each corner, so the
  drawing's bounding box is the canvas. The `// layout:` header names the engine that honours
  the file: `neato -n2` when every node is placed and any edge routed, `neato -n` when every
  node is placed and none routed, `neato` when some nodes are, `dot` when none — so `neato -n2
  -Tsvg view.dot` draws the view as laid out, and a route written under an engine that redraws
  it is noticed. A model without layout annotations writes the same DOT as before, and no
  Graphviz binary is run to produce it.

- **A design note for an embedded target at NASA's highest software class**
  (`docs/internals/design/embedded-target.md`). It proposes how a state machine, an action and
  the calcs they reach compile to C that runs under an RTOS in a form Class A software assurance
  can accept: a closed, serializable behavior IR whose written semantics, not the interpreter,
  is the requirement basis; a freestanding ISO C profile over static tables — no allocation, no
  recursion, a static bound on every loop, no compiler extensions or non-local exits, one
  decision per branch so structural coverage is measurable — with a small prelude verified once;
  a static refusal of every model with an admissible scheduling choice or an unbounded resource;
  the IR, trace map, resource report and budget file as reproducible configuration items; a
  fixed-step host interface designed with the planned C ABI and proved under Zephyr on QEMU and
  as an F´ component; and the evidence a tool qualification argument consumes. Nothing is
  implemented; the note exists to be reviewed before code is written.

- **The extent operator `all T` evaluates.** `all T` (KerML `ExtentExpression`,
  `BaseFunctions::'all'`) answers the instances of the named type as an ordered sequence in
  declaration order, and the typer gives it the static type `T[0..*]`, judging it element by
  element as it does any collection where a collection binds (`attribute xs : Boolean[*] = all
  Flags;`), while a condition `all T` is refused for any `T`, Boolean-typed included, as the
  sequence it is rather than the one Boolean a condition needs — as is an extent given to a Boolean
  operator (`not all Flags`, `all Flags and true`) — and `all Car as String` warns
  that the cast selects nothing. Because objects materialize
  lazily, the extent is the run's: for a variation definition or usage it is the variants it
  declares (`all engineChoice` in the trade-off pilot model now yields the engine alternatives,
  and its trade study proceeds to evaluation instead of stopping at the operator); for an ordinary
  definition it is every object the run has materialized or the current context reaches that the
  definition classifies, nested usages included; for an enumeration it is the declared literals.
  Any other data type, scalar (`all Integer`, `all String`) or structured (`all Point`), is
  refused with the typed `ErrUnboundedExtent`, since a run creates no data values to enumerate; an
  operand that is not a type (a package, a relationship, a comment, or no name at all) with
  `ErrTypeMismatch`, and a name that resolves to nothing with `ErrUnresolvedType`. Only nested
  usages whose type may hold an object of the type are materialized for its extent, and of those
  every one but a usage that would create another object of a declaration already on the path,
  settled by its value's possible types where they agree and else by what reading it makes, a
  read making one undone (so a composition recursing through one declaration ends, while each
  object a run linked to another of its declaration still has its own nested usages read and a
  value choosing at run time between recursing and not contributes what it chose); one the extent
  cannot materialize ends it with that usage's error rather than an extent short of it, and one
  making an object on the path together with one that may lead to a `T` is undone and refused
  with `ErrExtentUnavailable` naming it.
  An extent that a package-level port may contribute to is refused with the typed
  `ErrExtentUnavailable` naming the usage, since the runtime denotes no object of such a usage,
  rather than answered without it. A namespace-level object usage given a value (`ref part car : Car = new Car();`,
  `part fleet = new Truck();`, `ref part alias : Car = spare;`) is bound to that value for the run —
  a feature value binds its feature to its expression's result (KerML 1.0 §7.4.11 Feature Values,
  §8.4.4.11) — so `all Car` reaches the object it denotes before any read of it, and the usage
  denotes that one object on every read; it used to be evaluated anew on each read, so `car` read
  twice was two `Car`s. A `default` or initial (`:=`) value at namespace level is held for the run
  the same way, there being no other individual for it to be realized on; a `default` nested in a
  definition is still what each object built from it reads at construction. A usage bound to an
  extent of its own type (`ref part cars : Car[*] = all Car;`) stands for no object while it is
  being bound, and one whose value depends on it for none yet, so the extent binds to the objects
  there are; a value reaching back to its own usage is refused as a cyclic feature value. A binding
  refused after constructing objects (`ref part car : Car = new Boat();`) leaves none of them
  behind, however often the usage is read. `all T`
  is never model-level evaluable, so a `filter` or metadata value built on it is diagnosed. The
  native compiler keeps refusing `all` with a typed `UnsupportedError` (`operator
  'all'`), since a compiled program has no run whose extent it could report.

- **The `meta` cast evaluates.** `x meta T` (KerML 1.0 §7.4.9.2 MetaCastExpression) is now a
  value rather than the refusal `unsupported operator: 'meta'`: as the shorthand for
  `x.metadata as T` it answers, of the element `x` names, the metadata annotations whose type
  conforms to `T` in model order, then the element's reflective metaobject when its own metaclass
  conforms to `T` (§8.3.4.8.15), and `()` when neither does (`seatBelt meta SysML::PartDefinition`
  for a part usage). The metaobject is a value of its own kind: the element together with the
  reflective metaclass that classifies it, equal to and identical with every other metaobject of
  the same element whatever it was cast to, and rendered `meta(Pkg::x : SysML::Systems::PartUsage)`
  in the REPL and in traces. The metaclass's features read off it through ordinary member access —
  `declaredName`, `name`, `qualifiedName`, `shortName`, `documentation`, `isAbstract`, `isComposite`,
  `isDerived`, `isEnd`, `isOrdered`, `isUnique`, `isVariable`, `isConstant`, `isPortion`,
  `isSufficient`, the element-valued `owner`, `ownedMember`, `ownedFeature`, `type` and
  `documentation` (metaobjects in turn, `definition` for a SysML usage; a `doc` comment's own
  `body` and `locale` are its strings), `direction` (a `FeatureDirectionKind` literal), a
  dependency's `client` and `supplier`, and a textual representation's `language`, `body` and
  `representedElement` (its owner) — each shaped by the feature's declared multiplicity; a feature the metaclass declares but the
  runtime does not derive (`ownedRelationship`, …) is a typed error naming the feature and the
  element, and a name the metaclass does not declare is the ordinary missing-member error.
  `sysml -compile` refuses a `meta` cast by name, a metaobject having no native representation;
  `@@`, `@` and the static reading of `meta` in a `SemanticMetadata::baseType` are unchanged.
- **`x.metadata` ends with the element's reflective metaobject.** A MetadataAccessExpression
  yields the referenced element's metadata annotations followed by one metaobject of the
  element's own metaclass (KerML 1.0 §8.3.4.8.15), so `seatBelt.metadata->size()` on a part
  annotated once is `2` and the metadata of an element nothing annotates is that metaobject
  alone rather than `()`. The pilot evaluator answers the same counts; the two conformance
  fixtures and the pilot referee record moved with it.
- **Metaobjects cross the gRPC boundary.** `Value.metaobject` carries `element_id`, the
  qualified name of the element (its identity), and `metaclass_id`, the qualified name of the
  metaclass that classifies it, in both directions of `Evaluate`, `EvaluateCalc`, `ExecuteAction`
  and `RunAnalysis`; a metaobject sent as an argument is rebound to the named model's element,
  `metaclass_id` resolved when omitted and refused in band when it names another metaclass, and
  one naming no element is refused in band rather than read as null. The service advertises the
  `metaobject_values` capability; without it a metaobject in a response is an unsupported null and
  one in a request is `UNIMPLEMENTED`. The Go, Python, Node, Java and Rust clients read the arm as
  a typed value equal by element (`opensysml.Metaobject`, `{ kind: "metaobject" }`,
  `Value.MetaobjectValue`, `Value::Metaobject`), reject an empty `element_id`, and refuse to send
  one to a service lacking the capability; the conformance suite pins the arm over gRPC, Connect
  protobuf and Connect JSON.

- **A `MOSA` library for the Modular Open Systems Approach** (`OpenSysML Libraries/MOSA.sysml`, non-normative), named as 10 U.S.C. § 4401 names its concepts: `MajorSystemPlatform`, `MajorSystemComponent`, `ModularSystem` and `ModularSystemInterface` with their `#majorSystemPlatform`, `#majorSystemComponent`, `#modularSystem` and `#modularSystemInterface` keywords (`#keyInterface` is the practitioner's alias, a specialization of the statutory keyword); `Standard`/`ConsensusStandard` with a `#conformance` connection whose ends are `#conformant` and `#conformsTo`; `DataRights`, `Proprietary` and `InterfaceControl` metadata; `MOSAObjective`, `ModularityRequirement` and `InterfaceRequirement`; `MOSAConformance` over the guidebook's assessment criteria and `MOSAPackage` over its five pillars; viewpoints and view definitions for the modular decomposition, the modular interfaces, the standards, the proprietary elements, the data rights, the requirements and the conformance; and document queries (`InterfaceRegister`, `StandardsRegister`, `DataRightsRegister`, `ProprietaryRegister`, each walking `depth` ownership levels below `root`, sixteen by default) with an `InterfaceControlDocument` to specialize. `ParametersOfInterestMetadata`, `RequirementDerivation` and `ModelingMetadata` are re-exported. The worked example is `examples/mosa-demo/`; the design is `docs/project/mosa-library.md`.
- **MOSA openness checks** (`mosa-interface-no-standard`, `mosa-interface-no-control`, `mosa-interface-not-traced`, `mosa-component-no-data-rights`, `mosa-proprietary-no-rationale`, `mosa-boundary-not-designated`): constraint-tier warnings that a modular system interface conforms to a standard or is marked proprietary, is controlled, and satisfies an interface requirement, that a major system component or modular system records its data rights, that a proprietary element states its rationale, and that a connector between two distinct components is designated a modular system interface — each rule waits until the model states the kind of fact it looks for, and a model that never touches `MOSA` gets no finding.

- **A namespace-level object usage of several occurrences denotes its objects.** `part wheels :
  Wheel[2];`, `item links : Link[3];` or `part hubs : Hub[1..*];` declared in a package with no
  value now denotes its lower bound of objects for the run (KerML 1.0 §7.3.4.3 Multiplicities — a
  feature of multiplicity `[2]` has exactly two values), created once in declaration order and
  reused on every read, through the same materializer that fills a collection nested in an object;
  a `[0..*]` usage still denotes none. `all Wheel` counts them beside the `[1]` usages, a usage of
  exact count reads as the sequence (or set) of those objects, and `wheels#(1)`, `wheels.radius`
  and `size(wheels)` read them, in the REPL and over gRPC alike, while a usage of open count read
  directly (`hubs`, `size(hubs)`) stays undetermined of that count, as a nested collection of open
  count does, and one whose bound the model does not evaluate (`part wheels : Wheel[2..n];`)
  fixes no count, so it denotes nothing and an extent that may reach it is refused with
  `ErrExtentUnavailable` naming the usage and its bounds; the trace names each member after its
  usage (`wheels #1`, `wheels #2`). It used to denote nothing, so `all Wheel` was refused with
  `ErrExtentUnavailable` and `wheels.radius` was undetermined. A valued usage (`part wheels :
  Wheel[2] = (new Wheel(), new Wheel());`) is bound to its value instead, and a value whose count
  breaks the declared multiplicity is refused with `ErrMultiplicityViolation` naming the usage. A
  member that cannot be constructed, a lower bound over the materialized-collection cap
  (`part many : Wheel[10000];`) or over the element budget is that typed error naming the usage
  and leaves no object, behavior or record behind. A write chained from the usage (`assign
  wheels.radius := 2.0;`) reaches several objects and is refused with `ErrTypeMismatch`, as a write
  through a nested collection is, leaving the objects as they were. A constraint or requirement the
  definition declares (`constraint small` in `part def Wheel`) takes the members as one declaration
  when it looks for its subject, as it takes the members of a nested collection, so it is decided
  once rather than refused as ambiguous between them; a second usage of the definition remains a
  distinct carrier. A port at namespace level still denotes no object and keeps its
  `ErrExtentUnavailable` refusal.

- **Named standard-library elements carry the normative element ids the KerML and SysML
  specifications fix for them.** `ScalarValues::Real` is `14c0aa22-5489-59b5-b438-ded26e83ba31`
  here as it is in the pilot implementation and on every conforming SysML v2 API server, instead
  of the encoded name `ScalarValues__Real`, so a graph, a Flexo project or an element-by-element
  comparison that names a library element names the same element on both sides. The id is the
  name-based UUID (`uuid5`) the norm prescribes: the library package's over the OMG specification
  URL and its name, a named member's over the package id and its qualified name, and the owning
  membership's over the same with `/owningMembership` appended. The RDF mapping writes the
  normative id as the IRI tail, `sysml:elementId` and owning-membership IRI of every named element
  of the bundled library and reads it back without inventing an `@IdentityMetadata::ElementId`
  for it; the Flexo sync treats it as neither declared nor mintable; the LSP hover states it with
  its source and language (`Element id `14c0aa22-…` (normative, KerML)`), and the code action that
  mints an id is no longer offered on a library element. A declared `@ElementId` still wins, and a
  user element without one keeps its encoded name. Unnamed, aliased and shadowed library elements
  keep derived ids, since the norm gives them none that agrees across implementations.
  `TestPilotLibraryXMI` asserts every derived id and owning membership against the pilot's
  `sysml.library.xmi` at the pinned release (`./scripts/download-pilot-library-xmi.sh`), and CI
  requires the download.

- **Sweep rows run in parallel under `-jobs`, each in a context of its own, with the same table
  whatever the count.** `-sweep`/`-samples`, `%sweep`/`%samples` and the `RunSweep` RPC run
  their rows `-jobs` (`%jobs`, `OPENSYSML_JOBS`) at a time. Every row is a run of its own: it
  instantiates the case's subject and the `self` of a nested case in a fresh context over the
  plan's worker and carries the arguments' values in — evaluated once at the prompt, so a held
  feature a run wrote reads as written, an argument naming a held object bound to the object
  the row makes for it — so a case that writes a feature of its subject writes its own row's
  object and no row sees another's; the table comes out in range order whatever order the rows
  finish in, so the rows, their outputs, verdicts, evaluations and errors are those of
  `-jobs 1`, only each row's `time` and the report's `workers`/`warming` varying with the
  count; the RPC's response numbers every row's objects apart in its `instances` table. A
  `%sweep` on an object the session holds runs each row on a fresh object of the held object's
  declaration (one reached through a feature of another on the like of its root, walked along
  the same path) and leaves the held object as it was, which stands for it while it is as the
  declaration made it; an object named by `#<id>`, one a run has written a feature of or
  destroyed, or one running a behavior its type exhibits or performs — one fresh from
  `%instantiate` included, for now — is refused naming the reason rather than swept on the
  session's context, as is an argument naming such an object or holding a value bound to the
  run that made it, and the session's readers are no longer held up while the rows run. A
  deadline met mid-sweep starts no further row, discards the rows in flight and reports the
  deadline and no table, as one job does when it meets the deadline between two rows.

- **A view renders as PlantUML with the new `plantuml` form.** `sysml -render <view> -render-form
  plantuml` (and `.puml` files under `-render-all -render-form plantuml`), `%render <view>
  plantuml [palette]` at the prompt, and `"form": "plantuml"` on the editor's `opensysml/render`
  request write a tree, interconnection, state, action or sequence rendering as an `@startuml` …
  `@enduml` file in the Standard B&W style of the OMG SysML v2 Pilot Implementation's visualizer,
  the style carried inline as a `<style>` block so the file stands alone: a tree as a class diagram,
  an interconnection as nested `rectangle` blocks with the Pilot's heavy undirected connectors and
  dashed flows, a state or action rendering as the `state` grammar with composite states, `[*]`
  starts and PlantUML's pseudostate stereotypes, and a sequence — the one kind DOT does not write —
  as participants and messages. Labels lead with the name in bold over an italic `«keyword»` line,
  the keyword doubling as a stereotype the style selects definitions and usages by; the header,
  notices and DiagramLayout geometry are `'` comments, with a notice that PlantUML pins no position
  (the `dot` form does). The named palettes fill the nodes with the same colours the DOT form
  uses, a sequence's participants included. Producing PlantUML needs no Java and no PlantUML jar;
  nothing runs one. Mermaid stays the default machine-readable form, and a `table` view has no
  PlantUML form.
- **A document renders its diagrams as PlantUML on request.** `sysml -render-document …
  -diagram-form plantuml` (and `-render-documents`), `%render-document <name> plantuml` at the
  prompt, and `"diagramForm": "plantuml"` on `opensysml/renderDocument` write every graph-shaped
  `Diagram` block as a fenced ` ```plantuml ` block in Markdown and as `<pre class="plantuml">` in
  HTML; the PDF backend keeps the source under a notice that it does not draw PlantUML, looking
  for no PlantUML tool. A table-kind block is a table whichever form. A `Diagram` block's `palette`
  is now accepted on a sequence diagram too, since the PlantUML form fills its participants; a
  palette on a table remains a typed error.

- **The OMG PSSM state-machine test suite runs as an advisory referee.**
  `cmd/pssm-referee` downloads nothing itself: `./scripts/download-pssm-suite.sh` fetches the
  pinned `PSSM_TestSuite.xmi` (OMG `ptc/18-11-06`, 103 tests) over HTTPS, checks its sha256 and
  places it under the ignored `build/pssm/`; nothing from the suite is committed. The referee
  reads the suite's XMI, classifies every test — standard notation, this project's `fork`/`join`/
  history/`defer` extensions, the terminate gap, or no SysML v2 spelling — translates each
  expressible test into textual notation in memory by the construct table of the precise-semantics
  alignment note (every `trace("…")` appends to a `String` attribute `log`), runs it under the
  runtime's own state-machine driver with the `explore` schedule, and compares the set of `log`
  values reachable against the suite's expected traces in both directions. Each test is filed as
  `pass`, `fail`, `not-expressible`, `terminate-gap` or `differs-by-design` (the last only through
  a committed table mapping the test to a note row that differs by design, never inferred from a
  failure); a typed runtime error or an exhausted budget is a `fail` that names it. `-jobs`
  explores in parallel with byte-identical output, `-filter` selects tests, `-keep` writes the
  translated models for debugging, `-json` prints the full report, `-update` records the baseline
  and `-check` fails when the bucket counts move. `docs/project/pssm-referee.md` records the pin,
  the licence reading, the construct mapping as implemented, the counts with the date and
  `develop` commit they were measured on, and every test's bucket and reason; the pull-request
  workflow provisions the suite and gates on the counts. A pass checks that the runtime reproduces
  UML behavior where the model has a defensible SysML v2 mapping and is never evidence of SysML v2
  conformance. The state-machine driver the conformance tests used (`PerformState`, `Explore`,
  `QueuedEvent`) is now exported by `internal/core/runtime` so both harnesses share it; no
  runtime behavior changed.

- **A runtime showcase under `examples/runtime-showcase/` shows what only running a model
  answers.** Four small models — a recursive mass rollup over a materialized launch vehicle, a
  delta-v budget whose rocket equation carries units through to an analysis objective and a
  requirement, a reliability product asserted of two missions, and a mission action branching
  on its remaining budget beside a clocked state machine — each pair a construct that validates
  clean with one that fails at runtime with the reason named: an inherited attribute no usage
  values, a result computed in the wrong dimension, a name that resolves to a valueless library
  quantity, an action with three steps and no succession between them. The README reproduces
  every run command by command, and does the same for the published Apollo 11 model at a pinned
  revision, where the same four defects stop the model's own calculations and the instantiation
  of its mission. The project README and the landing page lead with the runtime — what it
  materializes, computes, performs and decides — and open on that showcase; the landing page's
  terminal gains an *Analysis* pane and a *Found at runtime* pane ahead of the REPL one.

- **The SMT model checker decides properties over free inputs, and is reachable as `-engine
  smt`.** A feature the model leaves unbound — an `in` parameter with no argument, an attribute
  with no default — is now a free variable of the initial state, ranging over its declared type
  (`Boolean`, `Integer`, `Natural` as `>= 0`, `Real` and a quantity over one, an enumeration's or
  variation point's constructors), and over its absence too when its multiplicity admits none
  (`x : Integer[0..1]`, reported *free … or absent*, a witness spelling it `input x = null`); a
  feature the model binds stays pinned as before, and a
  declared type the encoding cannot narrow (`String`, a collection, an object-valued feature) is
  *not covered* naming it, before any query. `-check-input <feature>` (`%check-input`) leaves a
  bound feature free in its domain; `-check-assume <constraint>` (`%check-assume`) asserts a
  constraint or requirement over the initial state, one no initial state satisfies being *not
  covered: assumptions admit no initial state*, never *proved*. A proof now reads *proved over
  schedules and inputs: inputs free in their domains* and lists each free input and assumption; a
  violation's witness names the values the solver chose (`inputs chosen from their domains`), its
  file opens with `input <feature> = <value>` lines ahead of the choice lines, and `-schedule
  replay:<file>` fixes them as the action starts before following the moves — a file without
  input lines replays as before, and one naming a feature the action lacks is refused naming it.
  `-json` results gain `inputs` (name, type, domain, `free`, `optional`, value) and `assumptions`, and a
  witness its `inputs`. `-check-unroll <n>` (`%check-bounds unroll=`, `Budget.Unroll`) bounds the
  loop unrolling (default 4); `-check-depth` is the move bound under `smt` too (default 40). The
  `smt` engine joins the build's registry at authority *proved*, listed by `-engines`,
  `%engines` and `ListEngines` with its solver as its status; `-engine smt` and `%engine smt`
  reach it, `-engine all` with `-check-input` shows `check` refusing the free inputs beside its
  answer, and since no check is asked as `holds` under `auto`, no existing verdict, plan line or
  golden changes.

- **An SMT model checker for actions on concrete inputs.** `internal/core/smt` encodes the
  lowered action graph — straight-line bodies, fork, join, merge, decisions, body loops unrolled
  to a bound, pins and object flows — as a transition relation over `k` moves and asks an SMT
  solver whether any schedule violates a requirement or constraint, or deadlocks. `unsat` is
  *proved* when every schedule finishes within the bounds and *bounded* otherwise; a `sat` model
  is decoded to the interpreter's choice lines and replayed through it, and is *violated* only
  when the replay reaches the state the solver described; `unknown`, a timeout, a body the
  encoding cannot express (clock, messages, nested flows, object-valued assignments, calc
  invocation, nonlinear arithmetic) and a replay that disagrees are *not covered* with the
  reason. The `smt` engine implements the analysis `Engine` contract over the solver
  `OPENSYSML_SMT`, `z3` or `cvc5` names, with `k` from `Budget.Depth`, the solver time from
  `Budget.Solver` and an unroll bound of its own (4), and is refereed against `explore` over the
  conformance corpus: outcome sets must agree, every witness must replay, and a proof must agree
  with exhaustive exploration. It is not yet registered with the default engines, so no verdict,
  golden or exploration changes; `-engine smt` follows.
- **`replay:<file>` scheduling policy.** A file of choice lines — as `explore`'s witness column
  spells them, one per line up to the first blank line, or `no choice points` alone for a run
  that met none — is followed move for move, then the run continues as `reverse`. A move the run cannot make (a pick not offered, a step already passed,
  a line left over at the end) fails the run with `replay refused: move <n> (<choice>): <what
  the run faced>` rather than running another linearization. Accepted by `sysml -schedule`,
  `%schedule` and a conformance case's `schedule` pin; over the wire, where a request carries no
  file of the caller's, the spelling is `INVALID_ARGUMENT`.

- **`-engine check` searches every schedule of a state machine, and of actions and machines
  on one clock.** The `check` engine takes an *invocation* — every `-action` and `-state`
  named (`%action`, `%state`, `RunFor` in the REPL), started on one context and run to the
  `-advance` horizon, together with the machines of the objects they materialize — and searches
  it as one run, one move at a time: a token advancing one node, an event dispatched, a `do`
  behavior stepped. Beside the action choices it explored before, the search draws every
  transition enabled for one event, every order of the regions reacting to it, every branch of
  a choice pseudostate, every order of the events the library leaves unordered at one instant
  (two timers firing together, a timer beside a signal of the same timestamp; completion
  events still go first and signals arrive in the order sent) and every order of the executors
  due at one instant — an action's `accept after` and a machine's timer falling due together
  being the case a run under one policy silently decides. A machine resting where nothing will
  wake it is a complete schedule, not a deadlock; an action left incomplete is one as before; a
  wait past the horizon is left unreached and the verdict reads `exhaustive up to t=<horizon>`.
  `finalState` is an observable beside the attributes — `-check-diverge finalState`, and
  `<behavior>.<feature>` or `<behavior> finalState` when several behaviors are checked
  together — so a machine that rests in different states on different schedules is *divergent*
  with a witness per state, and every witness of a joint run replays through `-schedule
  replay:` and `%replay` onto `%action`, `%state` and `%advance` on one object. Moves of
  independent footprints — a dispatch's being the guards, triggers, effects and state
  activities of the transitions the event can select — are searched in one order only, as an
  action's already were. Without `-advance` each behavior named is its own search; under
  `-engine smt` a `-state` or `-advance` is refused, as the solver does not encode a machine.
- **A body paused mid-statement is state a snapshot captures.** A token suspended at a
  breakpoint or on the clock inside a block or loop, a performed action waiting on its callee,
  and a state's `do` behavior waiting at an `accept` or a timed wait are held as explicit
  continuations — the statement cursor, the block, loop and flow-node frames, the nested
  performance and the wait — in place of a suspended coroutine, so `Snapshot` and `Restore`
  round-trip them and the checker searches the cases that pause a body like any other. A
  portable image (`HeldImage`, what a sweep of a held object copies) still refuses one with
  `ErrSnapshotPausedBody`, since its continuation points into the model's lowered statements.
  Every existing run, trace and choice point is unchanged.

- **The VS Code diagram panel is an editable canvas, and a dragged node's position is written
  into the model.** The panel draws its own SVG from the server's rendering: a node the model
  places with a `DiagramLayout::Layout` annotation is drawn exactly there, at the size it
  states, every other node takes a deterministic slot in a grid under its owner, and an edge
  follows its `DiagramLayout::Route` waypoints. Dragging a node writes `metadata Layout about
  … { x = …; y = …; }` — into the view's body when a view is drawn, into the element's own
  when the document is — as one source-preserving edit of the file when the pointer is
  released, so <kbd>Ctrl</kbd>+<kbd>Z</kbd> puts it back; an annotation already stated is
  updated in place, and the children the model places move with their owner. Dragging the
  handle on an edge bends it through a `Route` waypoint, dragging a waypoint moves it, and
  double-clicking one removes it. The palette, node menu, click-to-source and cursor
  highlight work on the canvas as before; a sequence diagram is drawn as lifelines and a
  table as Markdown, neither dragged. `SysML: Export Diagram` saves the Mermaid (or a table's
  Markdown) the server writes, positions included, to a file. Behind it, `opensysml/applyModelEdit`
  gains the `setLayout`, `setRoute` and `setCanvas` operations, which set, update or clear the
  three `DiagramLayout` annotations and refuse a target outside the document, a view that is
  none (`not-a-view`), a view-local placement of an element the view does not expose
  (`not-exposed`), an element no rendering draws (`not-drawn`) and a clearing with nothing to
  clear (`not-annotated`); a model without annotations keeps exactly its bytes. A node or edge
  no qualified name reaches — an unnamed transition, a connection in an unnamed part — is
  reported with the range of its `declaration` in place of an `fqn`, which `setLayout` and
  `setRoute` take as the target of an inline annotation. Not built:
  moving a node into another owner, placing an element in another document's view, and the
  `geometry` view kind.

- **A declaration can be moved into another namespace of its document.** `internal/core/edit`
  gains `OpMove{Target, NewOwner}`: the declaration — its body, the comment block above it, a
  line comment after it and its own lines, the span a delete removes — is taken out of its owner
  and written at the end of the new owner's body where an added member would go, re-indented to
  its neighbors, with a body added to an owner declared without one and the empty owner meaning
  the document itself. Every reference the move would break is respelled to the shortest
  qualified name that still reaches the declaration, references inside the moved declaration
  included; an import the move leaves redundant or dangling is dropped or respelled. The move is
  one operation, so a refusal leaves neither half applied, and a batch it is part of is applied
  all-or-nothing as before. Refused, with the stable names `owner-inside-target`,
  `illegal-kind`, `member-name-taken`, `move-referenced` and `referenced-elsewhere`: moving a
  declaration into itself or into what it declares, into a body that does not admit its kind,
  beside a declaration of the same name, a reference no qualified name can respell, and a target
  another document of the workspace refers to. The service's `ApplyEdits` takes it as `MoveEdit`
  (`target`, `owner`), under the `authoring` capability like `add_member` and `delete`; the
  Python client as `Editor.move(target, owner)` and the Go client as `edit.Move`. The language
  server's `opensysml/applyModelEdit` takes it as the `move` operation (`target`, `owner`), and
  `opensysml/render` now gives each declared node the `notation` it was written with and lists,
  under that notation in the palette's `owners`, the nodes that admit it, so a client can offer
  the right destinations.
- **The VS Code diagram panel moves declarations.** A node's right-click menu gains **Move to…**,
  which lists the drawn declarations whose body may hold the node's kind — not the node itself,
  its present owner or anything it declares — and the document's top level, and moves the
  declaration into the one picked. The edit lands in the editor's buffer like typed text, so
  <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it and the diagram redraws from the file.

- **The `smt` engine decides whether the schedule decides a feature, under the one flag both
  checkers read.** `-check-diverge <feature>` (repeatable) and `%check-diverge` now make the
  checked action's question one of *sensitivity* for whichever engine answers: `check` searches
  the schedules for two that end the feature differently as before, `smt` asks a solver for the
  two at once — two copies of the action's unrolled relation sharing the initial state and the
  free inputs, both completing within the depth, the feature's final values differing — and
  `-engine all` puts the one question to both and composes their answers. A *sensitive* verdict
  names the two final values, the first step at which the two schedules part and the move each
  took there, and comes with both schedules as witnesses (`witness A:`/`witness B:`, `-json`'s
  `witness` and `contrast`), each replayed through the interpreter before it is claimed and each
  written by `-check-witness` as a `-A`/`-B` file that `-schedule replay:` and `%replay` follow.
  A feature every schedule ends alike is *holds*, *proved* when no schedule was cut by the
  depth, unroll or slot bounds and `no sensitivity found within k moves` (*bounded*) otherwise,
  with the live schedule or the cut loop as the reason; a deadlock or typed error every schedule
  reaches is reported as that violation instead. The refusal of `-check-diverge` under `-engine
  smt` alone is gone; `-check-states` under `smt` alone stays refused. Two expectations of the
  `check` engine moved with the shared question: a clean exhaustive search under
  `-check-diverge` now stands as `holds (bounded over schedules …)`, the same claim `smt` spells
  for its bounded negative, where it stood as `outcomes (bounded …)`; the `✓ … no violation,
  exhaustive` line is unchanged. A feature of the performing object and an action with the
  clock or a paused nested flow are refused by `smt` naming the construct until later stages
  encode them.

### Changed

- **Every analysis question now goes through one framework.** `internal/core/analysis` holds
  the engine contract the analysis framework design note fixes — `Question`, `Engine`, `Result`,
  `Claim`, `Strength`, `Bounds`, `Budget` — a registry whose duplicate registrations and missing
  engines are typed errors, and `auto` dispatch that consults the engines covering a question
  strongest first, records each refusal and each run-time *not covered* answer in its plan, and
  stops on a run's error. The interpreter (`run`), `explore`, the parameter sweep (`sweep`) and
  the SMT solver (`solve`) register as engines over the code that already existed, and the REPL
  session and the gRPC service put every constraint, requirement and satisfaction check, calc,
  analysis and verification run, action and state execution, exploration, sweep and `%check`,
  `%explain`, `%solve`, `%configure` and `%optimize` query to their registry. No flag, command,
  RPC, field or line of output changed; the framework's own surface — engine listing and
  selection, the standing line on verdicts, parallel runs — is still to come. `runtime.Explore`
  takes the caller's `context.Context` as `RunSweep` does, so a `RunAnalysis` request cancelled
  mid-exploration ends it before the next linearization instead of running every remaining one.

- **Each analysis plan runs on a resolver and semantic model of its own.** The analysis
  framework builds one `analysis.Worker` per plan over the shared frozen index and every
  context a run owns on it, so two plans on one model in one process share nothing that
  memoizes; the result carries how many workers a plan built and how long that took, printed
  nowhere yet. A run-owned context takes the budget's `Steps` as `OPENSYSML_MAX_STEPS` and
  `Memory` as `OPENSYSML_MAX_ELEMENTS` at construction and leaves every other bound at the
  context's own; a zero field is the context's own too. The gRPC service builds a worker per
  request instead of serializing every runtime request on a model behind one shared pair, and
  the REPL session answers completion and its getters while an exploration runs on contexts of
  its own — a second command still waits. No flag, command, RPC, field or line of output
  changed.

- **The bounded model checker's independence relation counts message order.** The design note
  (`docs/internals/design/bounded-model-checking.md`) now declares any two sends, and any two
  accepts, dependent whatever their receivers: the bus is one context-wide list in arrival
  order and an accept takes the oldest match, so neither pair reaches the same captured state
  in both orders.

- **gRPC: a scalar-valued enumeration literal is an `enum_literal` on the wire.** `Level::high`
  from `enum def Level :> Integer { low = 1; high = 3; }`, a feature `l : Level = Level::high`
  and a successful `3 as Level` used to go out as `int_value: 3`, indistinguishable from a bare
  `3`; they now go out as `enum_literal`, like a plain enumeration's literal. The `EnumLiteral`
  message gains an optional `value` field (additive, so existing decoders keep working) that
  carries the scalar the literal equals, unset for a literal that is only its identity. The
  Go, Python, Node, Java and Rust clients expose it as an optional `value` on their enumeration
  literal type, keeping the literal's identity in `literal_id` alone. A bare `3` that no
  enumeration value holds stays `int_value`.

- **The documentation describes action execution in KerML's terms.** The action executor is a succession-ordered scheduler over the lowered `ActionGraph`; "token" names its bookkeeping for where each performance is, not a Petri-net or fUML semantics, and the API reference, architecture, roadmap and self-model say so. The composite-state conflict rule (the innermost enabled transition wins) is cited to KerML `StatePerformances::StatePerformance::acceptable` rather than to UML.

- **The landing page's hero terminal is a carousel of what the toolchain does.** Where the page showed one REPL transcript, it now cycles through six panes — the REPL, rendering a document from the model, checking a model, running a state machine, the model as RDF with a query, and the Python client — each with a link into the guide or manual. The panes are tabs of one fixed size: each prints itself as it comes up — commands typed, output by the line, the pane following the tail until the reader scrolls it — then holds for a few seconds before the next. Wide or tall transcripts scroll behind hidden scrollbars and the link floats over the pane. Rotation pauses while hovered or focused, honours `prefers-reduced-motion` (no printing, no rotation), and panes can be chosen with the mouse or the arrow, Home and End keys.

- **The extent `all T` is taken over the whole model, not the namespaces enclosing the
  expression.** The roots of `all T` are every object the run holds and every namespace-level
  object usage of every document of the loaded model that may hold a `T` — an imported package's,
  an unrelated un-imported package's and the standard library's alike, so `size(all Car)` in a
  package importing `Gaps` counts `Gaps::car` as `Gaps` itself does, and `all Clock` answers
  `Time::universalClock` — materialized as the extent is taken, in document-name then declaration
  order, the order `.metadata` gives cross-file annotations. The extent is of the type (KerML 1.0
  §7.3.2.1, §7.4.9.2), not of what the expression's namespace sees, so what it used to answer from
  a sibling package's expression is now what it answers from anywhere. A usage nested in a
  definition nothing instantiates still contributes no object, the library's `[0..*]` collections
  (`Parts::parts`, `Items::items`) neither count nor refuse, a namespace-level collection or port
  that may hold a `T` still refuses the extent with `ErrExtentUnavailable` naming it, and a usage of
  another document that cannot be read ends the extent with that usage's error naming it. The
  model's namespace usages are enumerated once per model and judged once per run and type, and a
  binding to an extent is read again when a re-analysis adds or removes such a usage in any
  document.

- **Diagram node labels lead with the element's name.** Every Mermaid and DOT node written by
  the view engine — `-render`, `-render-all`, `%render`, `opensysml/render` and a document's
  diagram blocks — now heads its label the way the graphical notation heads a compartment: the
  name on the first line, with ` : Type` after it for a typed usage (`pump : Pump`), the kind on
  the next line in guillemets (`«part»`, `«part def»`), and any note (`initial`, `already shown`,
  `own flow`) after that; an anonymous element leads with its kind alone. Mermaid breaks the
  lines with `<br>` in the flowchart, `stateDiagram-v2` and `sequenceDiagram` grammars alike, and
  DOT writes an HTML-like label (`label=<<b>pump : Pump</b><br/><font point-size="10">«part»</font>>`)
  with the name in bold and the keyword line smaller, for clusters as for nodes, escaping `&`,
  `<`, `>` and `"` in a name. The text form keeps the keyword first, as the notation declares it,
  but writes the type after a colon — `part pump : Pump` instead of `part pump (Pump)`. Edge
  labels, sequence messages, geometry comments and notices are unchanged. The `opensysml/render`
  result carries a node's declared type in its own `type` field; `detail` holds the notes alone.
  A declared type is spelled as written: a conjugated port type keeps its `~`, a global name its
  `$::`, a name that is not a basic one its quotes, and a usage typed by several types lists them
  all (`base : Mount, Cart`). A Mermaid flowchart reserves one line of height for a `subgraph`
  title, so an interconnection or action rendering whose cluster title spans more opens on a YAML
  frontmatter block (`config: flowchart: subGraphTitleMargin: bottom: <n>`, 24px per extra line)
  that keeps the title clear of the first child; a flowchart without such a cluster, a tree, a
  state and a sequence diagram carry none.

- **SysML v1 migration reads the open model formats, tool-neutrally.** `-convert` from `xmi`
  takes OMG XMI 2.5.1 with UML 2.5 and the OMG SysML 1.x profile, Eclipse UML2 `.uml` files as
  Papyrus writes them (`uml` is the new `-from` synonym; `.uml` is recognized by extension), and
  a zip archive holding the XMI, a `.mdzip` project among them. Only the OMG and Papyrus
  namespaces of the SysML profile classify elements — matched by host and path, so a lookalike
  namespace elsewhere does not — and a tool's own customization stereotypes over SysML are
  preserved as applied-stereotype comments like any other custom profile rather than
  special-cased. The canonical fixture and its goldens are a vendor-neutral XMI export.

- **The pinned OMG pilot implementation is now release `2026-08` (`jupyter-sysml-kernel` 0.62.0)**,
  with the reference validators, the vendored standard library, the pinned corpora, grammars and
  Xpect suites, and every oracle baseline re-recorded at that pin. The grammars, the standard
  library and the validation-constraint set are unchanged from `2026-07`; what moved is the
  pilot's own behavior: it now reports a type's `disjoint` clauses (the six `kerml-examples`
  diagnostics it alone used to raise are gone), and its new Xpect assertion that a feature may
  not own two `crosses` clauses is met. The validator build passes the pin to Maven, so the
  wrapper no longer has to be re-pinned for a pilot release it does not yet default to, and it
  stamps `build/pilot-validator` with the pin it was built from so a later re-pin rebuilds a
  stale or incomplete validator instead of reporting it already built. The rebuild is staged
  beside the installed copy and swapped in only once complete, with the previous copy kept until
  the new one is in place, so a failed or interrupted run keeps the previous validator usable;
  the SysML, KerML and evaluator builds always go through that check before compiling against
  the jar.
- **The Xpect scope oracle narrows a scope to the inherited members only for a redefinition.**
  A `subsets` clause whose target is spelled differently from the declaring feature's own name
  was being treated as a redefinition, which restricted the names the scope check expected at
  that target to the inherited members alone.

- **The runtime's `Context` is split into a shared `Model` and a run's own state, and a run
  can be snapshotted and restored.** `runtime.Model` holds what the runtime derives from the
  model alone — the semantic model, the resolver, the memoized calc shapes, write and
  invocation targets, literal caches, compiled calc closures and effective features — and is
  built once per analysis worker; `runtime.NewContext(model, maxSteps)` allocates only what a
  run mutates (objects, lifetimes, variants, the message bus, the clock, the scheduler, the
  trace), so a fresh context rebuilds none of the model's tables. `Context.Snapshot`, and the
  action and state executors' `Snapshot`, mark the run's journal between steps and capture the
  executors' tokens, frame tree, configuration, event queue, timers and `do` progress;
  `Restore` rolls the run back to the mark as often as asked, keeping every object's identity,
  until `Release`. A snapshot asked for inside a step is the typed `ErrSnapshotMidRun`; a body
  paused mid-statement is captured where it paused, and only a portable image (`HeldImage`)
  refuses it (`ErrSnapshotPausedBody`). The conformance suite proves the round
  trip on every case at every step, and `OPENSYSML_SCHEDULE_SEEDS` widens its scheduling sweep
  to further seeds. No flag, command, RPC, field or line of output changed.

- **The architecture self-model now describes the analysis framework.** `examples/self-model`
  gains `AnalysisFramework` — the seven question kinds and three freedoms, the five-step
  evidence scale and ten claims, the per-owner engine registry, the dispatcher with its `auto`,
  `all` and named selections, the seven-field plan budget with `OPENSYSML_JOBS`, the worker
  fleet, and the four engines this build registers with what each answers, what bounds it and
  the strongest evidence it can produce — wired into `AnalysisPipeline` and asked from the REPL,
  the service and the command line through the `%engine`/`%jobs`/`%engines` commands, the
  `engine` field, `ListEngines` and the `engines` capability, and the `-engine`/`-jobs`/`-engines`
  flags. The runtime's split of model-derived from run-derived state is modelled with the snapshot
  store and the exploration queue it makes possible. Five behaviors join the model — one question
  answered, a behavior's outcomes explored over a fleet of workers, the evidence ladder, a worker's
  life in a plan and a snapshot as a mark between steps — with four invariants over them
  (`questionsHaveOneContract`, `evidenceIsHonest`, `runsAreIsolated`, `snapshotsAreRunState`), the
  gates that verify them, eight views and an architecture-document section with an engine table
  generated from the model. `go test ./examples/` holds every new fact to `internal/core/analysis`
  and the runtime — engine names and descriptions, kinds, strengths, claims, budget fields, the jobs
  variable and its parsing, the selections, the exploration defaults, the snapshot refusals and the
  surfaces' commands, flags, field, RPC and capability — and exercises worker isolation: two jobs of
  one plan get runtime models of their own, two runs on one worker get contexts of their own.

- **A deferred event outranks a transition in an enclosing state or a sibling region.** While a
  state that defers an event is active, the event is held back from every enabled transition
  except one whose source is that state or a state nested in it; a transition in an enclosing
  state or in a sibling orthogonal region waits until the deferring state is exited, and the
  event is then dispatched, ahead of later arrivals, to the configuration that exit leaves. Only
  a nested transition overrides the deferral and consumes the event, and with two regions each
  deferring it, the event fires only if every deferring state has such a nested transition.
  Before, a sibling region's transition fired on the event and the deferral was never
  consulted. The outcome is determined by the configuration and is not a choice point.
- **A history with nothing to restore performs the owning state's ordinary entry.** A `history`
  or `deep history` that has no recorded configuration and no outgoing default transition now
  enters the owning state as a first entry would, through its `entry; then …;`, instead of
  failing the run; a region left through `done` records no history and is re-entered the same
  way. An owner with neither a default transition nor an entry transition fails with the typed
  `ErrHistoryWithoutEntry` naming the history.
- **A composite state's completion fires the composite's own completion transitions; the machine
  ends only when its top-level regions complete.** `then done;` inside a composite state's body
  now ends that state: once its do behavior and every region have ended, its transitions with no
  trigger are queued as completion events at the current instant, ordered as a plain state's are,
  so `state outer { … then done; } transition first outer then next;` enters `next`. Before, a
  nested `done` completed the whole machine and the composite's completion transition never
  fired. A completed composite with no enabled completion transition stays active and the
  machine runs on until its own top-level regions reach `done`.
- **A `choice` reads its guards after the incoming transition's effect; a `junction` before.**
  A route through a choice is now resolved on arrival: the states every branch leaves are
  exited, the effects into the choice run, and only then are its guards read, so `transition
  first idle do assign x := 1 then pick; transition first pick if x == 1 then seen;` reaches
  `seen`. Several enabled branches are the existing transition choice point, which `explore`
  enumerates and `seed:<n>` replays; no enabled branch fails the run with the typed
  `ErrChoiceWithoutBranch` naming the choice. A junction's guards are still read before the
  transition fires, so a junction with no holding guard leaves the transition not enabled, and on
  a chain each pseudostate follows its own rule at the point the route reaches it.

- **An unnamed transition is an anonymous member of its state.** `transition first off then
  on { … }` had a scope for its body but no symbol, and one with no body had neither, so nothing
  that walks a state's members by declaration — a `Route` annotation in the body among them —
  could find it, while the same transition named `t` was found. Every transition is now
  registered as a named one is, an unnamed one under no name, so it resolves nothing new and is
  listed and annotated as the state's own feature (SysML v2 §7.19.2).

- **An untyped parameter of a collection-operation body is typed by the collection's elements.** A body applied by `collect`, `select`, `reject`, `selectOne`, `forAll`, `exists`, `reduce`, `minimize` or `maximize` — in the `xs.{…}`, `xs.?{…}`, `xs->f {…}`, `f(xs, {…})` and `f(mapper = {…}, collection = xs)` notations alike — binds its parameter to each element of the collection (KerML 1.1 §8.3.4.8), so a parameter declaring no type now takes the element type(s) the collection is statically known to hold, as do a reducer's second parameter and the parameter of a body nested in another's. A reducer's first parameter holds the reducer's own result from the second fold on, so it takes the element type(s) only where that result, typed under the assumption, conforms to them (`cs->reduce {in a; in b; a}`); `cs->reduce {in a; in b; a.mass}` leaves `a` untyped as before, its `a.mass` refused rather than passed to fail at run time. `xs->collect {in x; x.mass}` and `xs.{in x; x.mass}` are typed `MassValue` as their `in x : C` forms are, `x.mass` is checked against `C`, and `x.nosuch` is reported as an unresolved member where it was passed over before. A body over a collection whose elements cannot be typed keeps its parameter untyped and its result the library's `Anything`. Runtime results are unchanged.

- **A rename or delete from the diagram panel follows references into the other documents of
  the workspace.** `opensysml/applyModelEdit` answers a `WorkspaceEdit` with one versioned
  `TextDocumentEdit` per document it rewrites, the requested document first: a rename respells
  every reference that writes the name wherever the workspace declares it, the way the editor's
  rename does; a cascade delete removes the referring declarations in whichever documents make
  them, recursively; a delete without cascade is refused as `delete-referenced` naming the
  referrers, each with its document, in `referring` and the new `referrers`. Every document is
  snapshotted, rewritten and re-analyzed together under one lock, so a new error in any of them
  refuses the whole request. The `referenced-elsewhere` refusal now applies only to a reference
  the edit cannot follow: one from a bundled library file, or from a document the index holds
  without the workspace holding its source. The VS Code extension applies the edit as one
  `WorkspaceEdit`, so one <kbd>Ctrl</kbd>+<kbd>Z</kbd> reverts every file, lists the referrers
  of a refused delete by file, opens a file the server read from disk before editing it so the
  edit lands on a versioned buffer, and leaves an edit unapplied when another document it names
  was typed into while it was computed. An edit within one document is unchanged.

- **A copy of a library file rooted at the library's packages converts as the library.** The
  encoder read only the byte-identical bundled file as the standard library, so the notation
  rebuilt from a source-free graph — not the bundled bytes — came back as a user file: its
  normative ids were written as `@IdentityMetadata::ElementId` annotations, and its second
  Turtle gained `sysx:declaredId`, `sysx:hasBody` and derived `_om` owning-membership IRIs;
  a `standard library package` sitting beside the bundled one also resolved a few inherited
  names (`portionOf` in a respaced `Occurrences.kerml`) against the bundled package. A
  document whose roots are the top-level packages of one bundled library file — same
  qualified names and normative ids, or the library's own package modifiers — is now
  analyzed in that file's place on the encoder side as the decoder already did: its elements
  carry normative element and owning-membership ids, no annotation and no `sysx:declaredId`,
  and its names resolve to its own declarations. An annotation restating a normative id
  declares nothing. `succession all a then b` no longer gains `first` when rebuilt. Every
  bundled library file now converts `notation → Turtle → strip source text → notation →
  Turtle` with the second Turtle equal to the first, source predicates aside, from the first
  hop. A user `package Actions { … }` keeps its encoded ids, an element carrying a library
  UUID under another name keeps it as declared, and a workspace copy of a library file in
  the editor is still the user's file.

- **The SMT model-checking note reads timed accepts without a horizon, and the encoding numbers
  flows by frame.** `docs/internals/design/smt-model-checking.md` now says, in one place, how
  `now` is to advance under `-engine smt`: only when no token is enabled, to the earliest due
  time, with `k` moves alone bounding the run and `-advance` refused as `check` refuses it — the
  library's `Clocks` and `Occurrences` fix when a timed accept becomes enabled and name no
  horizon, so none is added; the coverage row that said timed accepts needed `-advance` is
  corrected. In `internal/core/smt`, `Flow` numbers the root graph and every flow a node states
  of its own as frames, each with its own node range, labels and slot count, and records the
  `send` and `accept` sites it meets; `State` declares parked, due, clock and bus variables only
  for a flow that has accepts, timed accepts or sends, and lists its variables as a named vector.
  No transition reads them yet: `send`, `accept`, `accept after`/`accept at` and a node stating
  a flow of its own are refused before any query exactly as before, and no `-engine smt` output
  changes.

### Removed

- **The `entry point <name>;` and `exit point <name>;` pseudostates are gone.** They were UML's entry and exit points, an OpenSysML extension with no SysML v2 production and no KerML performance, and they added nothing a conforming model cannot write: a transition may target a nested state directly. `entry <action>` and `exit <action>` keep their OMG meaning, `point` is an ordinary name everywhere, and a former `entry point x;` is now diagnosed as an entry action followed by a stray name. The `history`, `shallow history` and `deep history` pseudostates stay, warned as non-standard notation as before.

### Fixed

- **The analysis `Budget`'s `Runs` bounds a sweep's rows and a solver's queries, as it does an
  exploration's runs.** `Context.RunSweep` takes the row limit it runs under (the context's
  `OPENSYSML_MAX_SWEEP_RUNS` when none is stated), the `sweep` engine passes the budget's, and the
  `solve` engine asks no more queries than the budget's runs, leaving the set *not covered* with
  the `runs` bound reached when some went unasked. `BudgetOf` fills `Runs` in the unit of the
  question's kind, so a sweep or solver query under an exploring schedule is no longer handed
  the schedule's exploration runs as its bound. `Registry.Answer` bounds a plan's context by the
  budget's `Deadline`, so an engine that meets it returns `context.DeadlineExceeded` and stops the
  plan on that step. No flag, command, RPC, field or line of output changed.

- **A declaration local to a behavior body answers to its declared type, multiplicity and
  uniqueness when its initial value is bound**, as a parameter, a `return` and a namespace-level
  declaration already do (KerML 1.0 §7.3.4, "the values of a feature are instances of its types").
  With `enum def Level :> Integer { low = 1; high = 3; }`,
  `calc def BodyOnly { in n : Integer; attribute l : Level = n; return : Integer = l + 0; }`
  answered `BodyOnly(2)` with `2`; it is now the write's `type mismatch: cannot write 2 (an Integer)
  to a feature typed by Level`, and `BodyOnly(3)` holds `l` as `Level::high`. A local stating a
  multiplicity (`attribute xs : Integer[2] = (n, n + 1, n + 2);`) is a `multiplicity violation`
  where its value's count falls outside it, a unique multi-valued local (`Integer[*] = (n, n + 1, n)`)
  a `uniqueness violation`; one stating no multiplicity keeps the count it is given and one declaring
  no type holds anything. The rule holds in a calc, action or constraint body, an `if` or loop block
  and a collection-expression body, and on the compiled tier, which checks a scalar or collection
  local the same way and declines to compile an enumeration-typed one rather than answer differently.

- **A replay refused at a choice pseudostate leaves the machine as the occurrence found it.**
  A choice's branch is drawn after the compound transition has left the states every branch
  leaves and run the incoming segments' effects, so a replay whose witness named a branch the
  choice does not enable was refused with those exits and effects standing. The refusal now
  undoes the move whole: the exits, their effects, the incoming effects, the trace and the
  run's notes are restored and a do behavior an undone exit abandoned stays paused where it
  was — no branch is taken and no choice recorded, as a refused transition or decision move
  changes nothing. An exploration's witness over such a choice replays to its outcome.

- **The runtime builds again, and a choice pseudostate follows the scheduling policy.** A
  choice with several enabled branches resolved its pick through two scheduler methods the
  replay policy had replaced, so `internal/core/runtime` no longer compiled. The choice now
  goes through the scheduler's one `choose` entry like every other choice point, so a seeded,
  explored or replayed run resolves a choice pseudostate the same way it resolves a state's
  competing transitions — and a witness naming a branch the choice's guards do not enable is
  refused as `replay refused: ... is not enabled` instead of silently taking the first branch.

- **A connector end that redefines an end by name also redefines the end at its own position.**
  `connection def Link :> BinaryConnection { end source : Foo :>> BinaryLinkObject::source; end
  target : Foo :>> BinaryLinkObject::target; }` has two ends again, not four: an explicit `:>>` on
  an end adds to the positional redefinition of each general connector's end (KerML 7.4.6,
  SysML v2 7.13.2) instead of suppressing it, as it does in the pilot implementation. Such a
  declaration, and the usages typed by it, are no longer reported as specializing a binary link
  with more than two ends; a genuine third end still is.

- **A constraint body whose condition is a bare feature reference comes back from its RDF
  graph alone.** `require constraint { ready }`, `assert constraint { ready }`, a bare
  `constraint { ready }`, a named `require constraint <'R-1'> ok { ready }` and KerML's
  `inv { ready }` are carried as a `sysml:FeatureReferenceExpression` with its `sysml:referent`,
  but the notation written from a graph with no `sysx:sourceText` closed the condition with a
  `;` — and `ready;` declares a feature named `ready` rather than referring to one, so
  `sysml -convert sysml -from ttl` refused the model (`the notation written for it does not
  read back as a reference`). The condition that closes a constraint body is now written bare,
  as a calculation's result expression is, for every condition shape (`not x`, `a and b`, a
  comparison); a condition that others follow keeps its `;`. The rebuilt notation validates
  as the original does and a second Turtle hop states the same triples, so
  `examples/phase-c-behavioral-bodies.sysml` now converts from its structure alone. The
  `.canonical.golden.sysml` fixtures under `internal/core/export/testdata/convert/` lose the `;`
  after their trailing conditions accordingly.

- **Classification against an enumeration is decided by its enumerated values.** An
  enumeration's enumerated values are the only instances it has (SysML v2 §8.3.7
  EnumerationDefinition), so with `enum def Level :> Integer { low = 1; high = 3; }` the shared
  classification rule behind `istype`, `@`, `as` and feature writes now answers `3 istype Level`
  `true`, `2 istype Level` `false`, `3 as Level` the value `Level::high`, `2 as Level` `()`, admits
  `attribute l : Level = 3;` and refuses `= 2` — statically where the value is constant
  (`cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::low = 1,
  Level::high = 3`), with the write-conformance error at run time otherwise — where every one of
  them was `ErrUndecidedClassification` before. `hastype` still reads the value's own type alone
  (KerML 1.0 §7.4.9.2): a bare `3` is an `Integer` and no `Level`, while a `Level` literal —
  written, held by a `Level` feature or produced by `3 as Level` — is a `Level` and not directly
  an `Integer`; a scalar-valued literal keeps that identity on the scalar it evaluates to, so
  `Level::high hastype Level` is `true` (it was `false`), `Level::high == 3` still holds, and the
  literal's own features and metadata are read from it (`Level::high.n`, `Level::high @ Hot`)
  where a chain through it used to fail as a chain through a constant, while `===` tells it from
  the bare `3` and from another enumeration's literal of that value; a
  literal of another enumeration cast or written to `Level` takes the equal `Level` literal's
  identity, as does a scalar bound to an enumeration-typed calculation parameter or result
  (`calc def asLevel { in n : Integer; return : Level = n; }`, so `asLevel(3) hastype Level`).
  Unnamed enumerated values (`enum def Size :> Real { = 60.0; }`) count, and an
  enumeration with none refuses every constant. A
  plain `enum def Color { red; green; blue; }` classifies by identity with its literals, and a
  user-defined subtype such as `Even :> Integer` stays undecided against a bare `5`.

- **`Model.find` on a model with errors keeps preferring the outermost symbol of a shared short
  name.** A feature whose name comes from an unresolved redefinition has no effective name for the
  service's `Query` to see, so the index alone could answer with a deeper symbol of the same name
  when such an outer one existed. The tree is now walked breadth-first, down to the depth of the
  index's best answer, before that answer is accepted; a model without errors is still one query
  and one fetch.

- **Prefix alternatives the grammar makes exclusive are syntax errors.** `composite portion` or a repeated `composite` in KerML (`BasicFeaturePrefix`), `abstract variation` in either order on a SysML definition (`BasicDefinitionPrefix`) or usage (`RefPrefix`), a repeated direction (`in out`; `FeatureDirection` is read once and `inout` is the single keyword) and a usage-only prefix on a definition (`ref`, `constant`, `const`, `derived` or a direction: `DefinitionPrefix` and KerML `TypePrefix` admit only `abstract`, or `variation` in SysML) each draw one diagnostic on the offending keyword, naming the alternative already written; a single-occurrence prefix written twice (`ref ref`, `constant constant`, `derived derived`, `ordered ordered`, `nonunique nonunique`, `end end`) reports the repeat the same way; a cross feature's prefix (`end in out x : T item e;`) is checked the same way, and `variation` in a `.kerml` file is a syntax error naming the SysML spelling. The KerML spelling of the constant prefix is `const`; `constant feature n;` in a `.kerml` file is now a syntax error naming the spelling, like the other SysML-only keywords, instead of the constraint-tier `Must be owned by an occurrence type`.
- **A string or unrestricted-name escape outside the terminal's set is a lexical error.** `"bad \q escape"` reports `invalid escape '\q'` spanning the backslash and the character after it (`KerMLExpressions.xtext STRING_VALUE` admits `\b \t \n \f \r \" \' \\` only); the token still reads so the declaration around it parses.
- **A bare `#` is a syntax error.** `# part def D;` and `# namespace N;` report `expected a metadata feature name after '#'` at the `#` (`PrefixMetadataAnnotation`, `PrefixMetadataMember` name a metaclass) instead of reading the keyword as the name and reporting an unresolved reference later; `#$:: part def D;` reports the same at `#$::`, since the global prefix also needs a name after it.
- **A signed multiplicity bound is a syntax error.** `[-1]` reports at the `-` that a bound is a literal or a feature name (`MultiplicityExpressionMember`), the verdict the pinned pilot gives; arithmetic bounds such as `[0..n+1]` remain an accepted extension.
- **Every state-body extension warns by default.** `defer`, `history`, `choice` and `junction` members in a state body, none of them a `StateBodyItem`, draw the `nonstandard-notation` warning in default mode and an error under `-strict`, consistently across the four keywords.
- **Body-context legality is a syntax-tier finding.** A `transition` in a part, package or other non-state body is a parser error naming the state body that admits it (`TransitionUsageMember` is a `StateBodyItem` only); `expose` outside a view body is a parser error, and in a `view def` body — the extension OpenSysML resolves — a `nonstandard-notation` warning; `require` outside a requirement body warns in default mode and is an error under `-strict`; a `variant` outside a variation is reported by an element-scoped constraint pass, since the grammar admits `VariantUsageMember` in any definition body and only `validateVariationMembershipOwningNamespace` rejects it. An unrelated syntax error elsewhere in the file no longer hides any of these.
- **The parser reports its own grammar findings.** An `import` without a visibility indicator and a non-enumeration member in an `enum def` body join a reserved word written as a name (`part def part;`, `alias part for D;`) as parser warnings of `parser.New(sf).ParseFile()` itself — recoverable findings the analysis escalates to syntax-tier errors without gating later tiers — so a direct parser consumer and the LSP see them without running any pass; `constant` in a `.kerml` file is a parser error.
- **`inv false` parses and is negated.** `inv false v { … }` and `inv true v { … }` read as KerML `Invariant` (`'inv' ( 'true' | isNegated ?= 'false' )?`), `false` recording the negation the runtime evaluates and the RDF export round-trips, where the truth keyword used to be rejected as a reserved word in name position.
- **A missing `;` is reported where it belongs.** When the next token starts a later line or closes the body, `missing ';' at end of declaration` sits on the declaration's last token, with the insertion quick fix; on the same line the unexpected token is still named.
- **`individual part : 'Gus Grissom' :> crew;` no longer warns.** A modifier followed by a kind keyword and no name declares an anonymous usage of that kind, the form the Apollo 11 model uses throughout; the `ambiguous-modifier-kind` warning that misread it was a false positive and is removed.

- **An `individual` definition keeps its modifier through the RDF mapping.** The definition
  exporter now writes `sysml:isIndividual` for an `individual part def`, `individual item def`,
  `individual occurrence def` and every other definition kind the modifier may prefix — the same
  property a usage already carried — and the importer writes the modifier back from it, so a
  `.ttl` stripped of its `sysx:sourceText` no longer comes back as a plain `part def` with
  `An individual must be typed by one individual definition.` on each usage typed by it. An
  `individual def` carries the flag too and reads back by its keyword alone; a definition without
  the modifier still carries no flag.

- **A library graph stripped of its source text converts back to notation whole.** Reading
  a `.ttl` of a bundled library file without `sysx:sourceText` failed on `Actions.sysml` at
  `aState.aTransition.accepter.acceptedMessage`: `accepter` is a feature every transition
  inherits from `Actions::TransitionAction`, and the reader checked the spelling against the
  bundled library rather than the notation it was rebuilding, so no spelling reached the
  graph's element. A graph whose roots are library packages under their normative ids is now
  read in that library file's place — its own declarations stand in for the bundled ones and
  a `.kerml` library is read in KerML's grammar when the roots record none — so the chain
  resolves and fifteen more KerML library files (`Clocks`, `Performances`,
  `ControlFunctions`, …) read back without source text. The target still has to be the
  graph's exact element.

- **A literal's direct type is its `ScalarValues` definition, read from the library and not from
  the evaluating scope.** `directValueType` answers an integer with `ScalarValues::Integer`, a
  finite real with `ScalarValues::Rational` (KerML 1.0 §8.4.4.9.2), a Boolean, string or complex
  with its library type, found by qualified name the way `*` is found as `Positive`, where it
  resolved the bare simple name in scope — undetermined in a scope importing nothing from
  `ScalarValues`, and a model's own `attribute def Integer` where one was declared. A written
  `istype Integer` still resolves to the type the scope sees, so `2 istype Integer` beside such a
  declaration is `false` and `2 istype ScalarValues::Integer` `true`, as the pilot answers. A model
  built with no library document at all keeps its same-named types as the stand-in; with any
  library present, a scalar whose `ScalarValues` definition is missing has no determinable type.

- **Arithmetic, comparison and aggregation over a quantity on a measurement scale agree with
  `ConvertQuantity`.** A magnitude on an `IntervalScale` such as `SI::'°C_abs'` or `Time::UTC`
  is a point on an affine scale, and the operators treated the scale as a ratio unit: `300.0 [K]
  == ConvertQuantity(300.0 [K], SI::'°C_abs')`, `300.0 [K] < 30.0 [SI::'°C_abs']`, `warm + 10.0
  [SI::'°C']` and `5.0 [Time::UTC] + 3.0 [s]` were `incommensurable units`, while `2 * warm`,
  `warm * 2.0 [s]` (a `['°C_abs'*s]` unit) and `5.0 [Time::UTC] + 3.0 [Time::UTC]` computed
  meaningless points. A point now moves by a difference in a commensurable ratio unit
  (`36.85 ['°C_abs']`, `8.0 [UTC]`), two points subtract to a difference in the scale's declared
  `unit` (`warm - 10.0 [SI::'°C_abs']` is `16.85 ['°C']`, `5.0 [Time::UTC] - 3.0 [Time::UTC]` is
  `2.0 [s]`), `==`, `!=`, `<`, `<=`, `>`, `>=`, `min` and `max` carry the right operand onto the
  left operand's reference through the scale's anchor before comparing, and set membership and
  collection equality equate `293.15 [K]` with `20.0 [SI::'°C_abs']`.
- **Operations a point on a scale does not define are typed errors.** `point + point`,
  `k * point`, `point * q`, `point / q`, `q / point`, `magnitude - point`, `point ** n`,
  `sqrt(point)`, `-point` and `sum`/`product` over points are `ErrScalePoint` naming the scale
  and the operation, and a point on an `OrdinalScale`, `CyclicRatioScale` or
  `LogarithmicScale` refuses interval arithmetic instead of behaving as a ratio unit. A
  measurement scale is no longer accepted as a factor of a unit term, on the wire included, and
  the static dimension check warns about a refused operation a literal makes certain while
  accepting `point + difference`.

- **`elem.metadata` answers annotations in textual order.** An inline `@` annotation and a `metadata … about elem` usage declared elsewhere now take their places by source position (across files, in document order) instead of every inline annotation preceding every `about` one, so `elem.metadata#(1)` is the annotation written first. Element filters are unaffected: they never depended on the order.

- **An ordering operator over a value the Kernel Function Library declares no ordering for is
  reported as a type mismatch, not as "operands must be constants".** `Color::red < Color::blue`
  over `enum def Color { red; green; blue; }` was refused with `comparison operands must be
  constants, got enumeration literal and enumeration literal`, though both operands are constants;
  the refusal stands — `DataFunctions::'<'` and `ScalarFunctions::'<'` are abstract and only the
  numeric libraries and `StringFunctions` declare an ordering — but it now says so: `type mismatch:
  operator '<' is not defined for the enumeration literal Color::red and the enumeration literal
  Color::blue; DataFunctions::'<' is abstract and no library function declares '<' for the
  enumeration Color, which is no ScalarValue`. Every ordering operator (`<`, `>`, `<=`, `>=`) over a
  Boolean, a part or metadata instance, a function value, a sequence, a set, null or a measurement
  reference reports the same way, naming the operator, both operand types and the library function
  that would have to declare it, on the operator, its `'<'(x, y)` library form, `->minimize`/
  `->maximize`, and in the compiled calc tier, which no longer orders a Boolean as a number. An
  enumeration specializing `Integer`, Strings and quantities order as before.

- **A computed quantity is reported in the coherent unit of its dimension, not in the expression it was composed by.** A product, quotient, power or root of quantities kept the units it was built from — `9.80665 [SI::'m⋅s⁻²'] * 311 [SI::s]` read `3049.86815 [SI::'m⋅s⁻²'*SI::s]`, `10 [SI::N] / 2 [SI::kg]` read `5.0 [SI::N/SI::kg]`, and `(mu / 6563 [SI::km]) ^ (1/2)` (`mu` in a model-declared `Orbit::'m³⋅s⁻²'`) read `246443.54… [Orbit::'m³⋅s⁻²'**0.5/SI::km**0.5]`. The result is now reduced through the library's `MeasurementReferences` data — a unit's `unitPowerFactors`, its `unitConversion` and the prefixes, so a `DerivedUnit` a model declares reduces like `SI`'s own — to a power vector over the base units and a scale, the scale is folded into the magnitude by the exact factor, and the unit is spelt by the measurement unit the library declares for that dimension: `3049.86815 [SI::'m/s']`, `5.0 [SI::'m⋅s⁻²']`, `7793.229127559948 [SI::'m/s']` — the same value the expression over `6563000 [SI::m]` gives. The library's coherent unit is preferred to a synonym the model declares, the declared type of the feature or parameter the value is bound to chooses between same-dimension units that measure different kinds (`EnergyValue` takes `SI::J`, `TorqueValue` `SI::'N⋅m'`), and a product no library unit measures stays over its base units. A quantity written as one named unit is kept as written, a dimension-one result is still a number, a product holding a dimension-one unit such as `rad` keeps it, and a power whose exponent is not a number is still the error it was. The rule lives once in the quantity layer, so `-calc`, `-eval`, `-analysis`, `-requirement`, `-satisfy`, `-instantiate`, the REPL, traces, queries, aggregates, vector and tensor scaling, `QuantityCalculations` and the gRPC `unit` field all report the one spelling; a value written to a feature typed by a quantity kind is judged by its reduced dimension as before, so `[SI::N/SI::kg]` is admitted to an `AccelerationValue` and a speed written to one is refused with the same `type mismatch`.

- **An unresolved name that is the unquoted start of a declared name says so, on every
  surface.** Names such as `'SA-506'` or `'HLR-R001'` hold characters no basic name can, so typed
  bare they read as an identifier followed by something else — `T::SA-506` is `T::SA` and `-506`,
  the subtraction `T::SA - 506` where an expression is expected. The failure used to stop at
  `unresolved reference: T::SA`, or at `"-506" cannot follow SA` for an object reference; it now
  offers the quoted declaration and states the rule: `unresolved reference: T::SA — did you mean
  T::'SA-506'? Names containing '-' must be quoted.` The offer comes from the declarations in
  scope (and, at the prompt, of the kinds the command acts on) whose unquoted spelling starts with
  the identifier read, never from the rest of the text; the characters named are the ones those
  declarations hold. The hint reaches the analysis diagnostics and their quick fixes, expression
  evaluation (`-e`, `%eval`, `%calc` arguments), every meta-command and command-line flag that
  resolves a name (`%instantiate`, `-instantiate`, `%state`, `-requirement`, …) and the object
  references `%features` and its kin read. Parsing is unchanged: `SA-506` is still a subtraction
  wherever an expression is valid.
- **`%instantiate` echoes a spelling `%features` can read.** Its `Use %features … to inspect` line
  used to repeat the name as typed, which for a bare `T::SA-506` names nothing; it now writes the
  declaration's own notation (`T::'SA-506'`) whenever the typed spelling would not read as an
  object reference.

- **A redefinition's target is resolved from the owning type's generals and then the enclosing
  namespace, never from the owning type's own scope (KerML 8.2.3.5.2).** `attribute z :>> x;`
  used to bind a sibling `x`, a member the owning type imported (`private import Lib::*;`) or an
  alias it declared (`alias y for x;`), and `:>> C::nope` or `:>> w.x` started at a sibling `C` or
  `w`; the pinned OMG pilot leaves all of these unresolved, and so does OpenSysML now, so the
  downstream reports those bindings drew (a featuring-type conflict, a conformance error) no
  longer appear. A target the generals lack is still found in the enclosing namespace and
  outward, as before.
  The general-type search itself is completed so that no case the pilot accepts moved: a
  member a general acquires through its `public import`, the `source`/`target` ends of a
  redefined flow or transfer, a namesake reached through the type of a redefined feature, a
  nested metadata body at any depth, and a qualified chain through an inherited feature all
  resolve from the generals. The standard library snapshot is regenerated: a flow definition
  now specializes `Flows::MessageAction` (`Flows::Message` when it declares two ends) rather
  than `Flows::Flow`, `Flows::Flow::source`/`target` no longer list themselves as their own
  generals, and `ShapeItems::CuboidOrTriangularPrism::ff`/`rf`'s `faces::edges` binds through
  `Polygon`'s inherited `faces`, as the pilot binds it.

- **`sysml -render <view>` accepts several model files, loaded as one model.** It used to stop
  at the second file with `-render renders a view of one model; unexpected extra argument`, so
  a view that exposed elements a sibling file declares could only be rendered with `-render-all`
  or from the REPL. Every file named on the command line is now loaded together, as `-render-all`
  and `-render-document` already loaded theirs; `-render-form` and `-o` apply as before, and the
  `#tree` pseudo-view renders every file loaded. A single file renders exactly as it did.

- **A witness `explore` writes for a step the clock retries replays under `replay:`.** When a
  step leaves every remaining token waiting on the clock, the executor advances it and retries
  the step, and an exploring run draws its token order at the retry, where the tokens are due
  together. Replay consumed the order's move on the first pass, found the token not yet due
  unable to act and refused the exploration's own witness (`step 3: 3@direct is not able to
  act (able to act: 2@performed)`). A token-order move is now kept for the retry while at most
  one token is able to act and every alternative it names is a token present in the step,
  parked or held; the one able to act moves, and the move is taken at the retry. A move naming
  a token absent from the step is refused at once as before, and one kept for a retry that
  never comes — the step ending in progress, a message wait or a deadlock — is refused with the
  same message. The `-schedule replay:` flag, `%replay` and the checker's replay of a witness
  share the fix; what `explore` records, the witness format and the step numbering are unchanged.

- **A state machine redefining `isRunToCompletion` or `runToCompletionScope` away from the
  Kernel Semantic Library default is refused instead of run under the default.** The runtime
  implements only the defaults (`isRunToCompletion = true`, the whole machine as the scope) and
  never read a model's redefinition, so `attribute :>> isRunToCompletion = false` or a
  `runToCompletionScope` narrowed to a substate executed silently as if the default held.
  Lowering now refuses such a machine with the typed `lower.RunToCompletionRedefinition`, an
  `ErrUnsupportedStateContent` naming the feature, the declaring body and the value written —
  on the executed machine, on a state definition it specializes, on a substate and on an
  orthogonal region's substate alike — and refuses a value it cannot read as the default, saying
  the default cannot be verified. A redefinition restating the default (`= true`, `= self` on the
  machine) runs unchanged, and so does a machine restating it over the redefinition it inherits
  from a specialized definition: only the redefinition a body makes effective is judged. The
  target is resolved as a symbol, so an alias of the library feature is refused too, while a
  model's own feature declared under the library's name is an ordinary attribute. Every
  surface that starts a machine — the REPL's `%state`, an object exhibiting it, the analysis
  engines — reports the refusal through the lowering error it already shows, and a state
  rendering reports it as a machine that does not lower. Neither a non-run-to-completion
  scheduling nor a narrowed scope is implemented.

- **`x as T`, `x istype T`, `x hastype T`, `x @ T` and a feature's type judge a scalar by one rule.** A value is of the type its representation states and of that type's supertypes, never of a narrower one by the number it holds: `4.0 istype Integer` is `false` and `4.0 as Integer` is now empty (it kept `4.0`), `6 / 3` is a `Rational` and no `Integer`, and `hastype` names the value's own type alone. A feature write follows the same rule, so `attribute whole : Integer = 4 / 2` and an `Integer` parameter fed a whole `Real` are refused — the quotient by the checker where it is written, the `Real` at evaluation — where they were held by magnitude before; convert with `RationalFunctions::ToInteger`, `RealFunctions::ToInteger` or `IntegerFunctions::ToNatural`, or declare the feature `Rational` or `Real`. `Natural` and `Positive` mark no evaluated value, so `7 as Natural` on a bare integer is reported as undecided like `5 as Even`, while their bounds still refuse `-1` and `0` and a value read from a feature declared `Natural` is kept. A Complex is a `Complex` on the real axis or off it, so `rect(2.0, 0.0)` is no longer held by a `Real` or `Integer` feature; `re` yields the `Real`.

- **The self-model's question and exploration flows follow the dispatcher and the explorer.**
  `AnswerQuestion` gives `all` a branch of its own that consults the engines declaring the
  question's kind in name order, as `Registry.Answer` does, where before it shared `auto`'s
  authority ranking; the three selections join before the coverage check, and the plan advances
  only while candidates remain — the engines declaring the kind (`candidates`, one per kind in the
  default registry, where every engine declares a kind of its own), not the whole registry, with
  every engine consulted — answering, refusing or faulting — landing in the plan as a step
  (`steps`), and a refusal (`refusing`, the candidates asked first that do not cover the
  question) moving on to the next candidate whatever the selection, as `Registry.answer` does.
  `ExploreOutcomes` charges a run to the budget for every linearization it commits and works the
  queue it drains — the empty prefix to start, then the prefixes each run leaves unexplored: the
  choice tree is modelled as a chain of choices (`choicesAhead`) of so many alternatives each
  (`alternatives`), the next below one alternative of the one above (`below`, the first by
  default), and the prefix in hand as the choice it ends at and the alternative it takes there
  (`choice`, `taken`); a run takes the first alternative of every choice it meets on its way down
  and leaves the second of each, and the next alternative of the choice its prefix ended at — one
  prefix per choice it owns, as `exploreRun.unexplored` does, not every alternative at once — and
  the next run takes the prefix queued deepest, so two binary choices met by the first run take
  three runs, not four, and a later run advances a four-way choice to its third alternative rather
  than finding three prefixes queued; what a run leaves goes on the queue only up to the runs left,
  the plan never outgrowing the run budget, the rest dropped and the runs bound marked hit
  (`runsHit`), as `exploreQueue.insert` does; a choice met beyond the depth bound (`depth`, 64 by
  default) takes its first alternative and marks that bound hit — so the queue always drains, a
  finite choice tree within the bounds proves, a tree either bound cut observes, and neither flow
  depends on a fixed decision any more. Both flows now run under
  `go test ./examples/`, through several candidate counts, selections, faults and choice trees.
  The pilot differential baseline is re-recorded from one validator run over the current
  `examples` tree — 370 files, 337 fully agreeing, 671 pilot-only, 707 pilot diagnostics — so its
  per-file rows and provenance digest measure the same inputs again; the record and the generated
  figures follow.

- **A semantic metadata definition inherits its supertype's `baseType`.** `metadata def <k> K :> M;` where `M` is a `SemanticMetadata` binding `baseType` now classifies a `#k` element as `M` does — the annotated usage subsets `M`'s base and a `#k` definition specializes its type — where before only a definition's own binding was read and `#k` added nothing. An own `:>> baseType = …` still takes precedence, including when it names nothing resolvable or its conditional branch is not taken: the inherited binding is replaced, not fallen back on.

- **A rendering quotes a name holding `::` whole.** A view drew a declaration whose unrestricted
  name holds the qualified-name separator by splitting the joined qualified name at `::`: `port
  'fuel::out'` was labeled `'out'` and a top-level `part 'x::y'` was written `x::y`, as if it were
  `y` in `x`. The tree, interconnection, sequence and table renderings, the REPL's `%render` and
  `%view`, the `opensysml/render` LSP result and the VS Code diagram now spell each segment from the
  declaration's owner chain, so `'x::y'` and `'fuel::out'` are one name each, and a connection
  added from the diagram between such ports names the port that is there. The qualified name a
  rendering hands a client to edit by, and the one `opensysml/applyModelEdit` reads, quote each name
  on its own too, so a top-level `part def 'x::y'` and a `part def y` in `package x` are two
  targets rather than one.

- A `[0..*]` feature holding one value now denotes that value wherever one value is
  taken — an operator operand, a `[1]` parameter of a user calc or library function, a
  cast or `istype`/`hastype` operand — since a feature's values are a sequence its
  multiplicity constrains (KerML §7.3.4.1, §7.4.12). `SampledFunctions::SamplePair`
  arithmetic and `interpolateLinear` on the library's own examples evaluate, as does
  `q.zs + 1.0` with `zs : Real[0..*] = (2.5)`; a collection of several values is refused
  as before, and multiplicity checks are unchanged.

- **An inline `entry`, `do` or `exit action` body of a state that states successions now
  executes.** A body such as `do action ops { first start; then action a : A; then action b : B;
  then done; }` or `do action ops { action a : A; action b : B; first a then b; }` was rejected
  at instantiation with `statement not executable: … *ast.InitialNode in a body is not
  executable`, while the same body as a standalone `action def` ran. The body is now lowered to
  the same token flow a standalone action's body is and runs through the action executor, so
  successions, `first … then …`, guards, forks, joins, `then done` and action nodes with a flow
  of their own behave as they do in an action, and the attributes the body declares are the
  performance's own; a dangling or unstartable succession, or the body or a node of it declaring `return`, is
  reported as a typed error before any node runs. A body stating no flow still runs its
  statements in declaration order.

- **A state's `do` behavior may be a full action: one that waits on the clock or for a
  signal, and a typed usage with pin bindings.** `do action poll { action wait accept after
  3 [SI::s]; then action count assign ticks := ticks + 1; }` was refused at instantiation with
  `statement not executable: … action usage "wait" in a body is not executable`, and `do action
  poll : Poll { inout n = ticks; }` with `performing an action and stating a body of its own in a
  body is not executable`, although both validate clean. A do body's flow now starts at its one
  node no succession leads to where no `first` says (two such nodes or a cycle are still reported,
  naming them), and the body runs as one performance that may pause: an `accept after`/`accept at`
  parks it on the shared clock — `%advance`/`-advance` move it and list it under `Waiting on the
  clock` — and an `accept Sig` parks it until a matching signal is sent (`%send Sig` takes it,
  reporting `the do behavior of state <s> goes on from its accept`, rather than refusing it because
  no transition fires; a signal sent from a sibling object wakes it too), while the machine's
  transitions and its other regions' do behaviors go on around it. `%send`'s preview and the
  dispatch select the signal's takers by one rule: the transition chosen for a state is drawn once,
  before the takers are settled, and where it leaves the state whose do behavior is parked for the
  signal it is the only taker there, while a transition between that state's own substates, or
  one in a sibling region, shares the one dispatch with the do behavior; the step reports `Event dispatched,
  letting the do behavior of state <s> go on from its accept`, and such a signal is neither deferred
  nor counted as dropped. A nested action node stating its flow in declaration order starts at its one
  unpreceded node as the body does, rather than being reported as a flow without a start. Leaving the state ends the
  performance: its wait leaves the clock, nothing after the wait runs, and a signal sent later
  wakes nothing. A typed usage whose body declares only the pins of the action it performs
  performs that action, an `inout` pin bound to a feature (`inout n = ticks`) writing back when the
  performance ends and not when the state's exit abandons it, and one valued by an enumeration
  literal or another name no enclosing feature answers (`inout mode = Mode::idle`) starting from
  that value and writing nowhere, rather than being refused as an output bound to no feature; an
  `in` pin nothing binds, or one
  bound to a feature the state does not declare, is a typed error naming the pin. Which of two
  regions' do behaviors due at one instant acts first in a round is a choice point (`choice do
  round at t=2.0: states lwork, rwork react`), explored and seeded as the other choice points are.
  An `entry` or `exit` body, or a transition effect, whose flow waits on the clock is refused with
  `state behavior waits for the clock` — those behaviors are performed whole at the instant they
  are triggered.

- **A bare feature reference or feature chain is typed statically by the feature it names.**
  The checker reads a name's effective scalar type — declared, given by its value (a
  non-default one beside no generalization, as KerML 1.1 §8.3.3.3 has a value type a feature;
  a `default =` fixes none), or reached through the features it redefines or subsets and the
  types it inherits, an alias followed to its target — so `while total { … }` over `total : Integer`, `-s` over `s : String` and
  `s == 1` are judged before execution as a literal of that type would be; the executor's own
  check now stands only for a condition whose type is genuinely unknown. The pilot corpora
  move by four diagnostics, all on `kerml-examples/Simple Tests/Expressions.kerml` and each
  adjudicated in `docs/project/pilot-differential.md`.
- **A computed value is judged against a scalar-typed feature.** An invocation's result and an
  operator expression's are checked against the feature they are bound to by the same
  classification the runtime's write conformance applies, so `attribute s : String = GetReal()`
  and `attribute s : String = a + 1.0` are reported statically and the two verdicts cannot
  disagree; numeric results still conform along the lattice, and a behavior declaring no result
  leaves the binding to the runtime. A call's result is typed by its declaration whether or not
  its input signature can be determined, so a result-only or parameterless behavior types its
  value too.
- **An argument of statically unknown type keeps every overload applicable.** A call such an
  argument leaves open selects only where one candidate remains or the known arguments prove a
  unique winner; otherwise the `invocation-ambiguous` warning `call of abs is undetermined
  between …` names the candidates left open, and the runtime settles the call — a calc's or a
  nested action's — by the values it is given: a scalar as the literal spelling it would be, so
  a positive value selects a `Natural` overload as `pick(1)` does, an object by every type it is
  classified by, an argument a candidate takes as an `expr` left unevaluated; it reports
  `ambiguous invocation` when they tie still. A tie no value could break — the tied candidates
  type the unknown argument alike — stays the `ambiguous` error. A settled action node holds
  the pins of the action performed alone and is read as a value by its result, not by a pin
  another candidate declares. The first visible candidate is no longer chosen silently. A
  called name is resolved as KerML 1.1 §8.2.3.5 resolves any name — an owned declaration hides
  an imported one, a nested namespace's import stands ahead of an enclosing declaration — with
  no rule of its own for library functions.

- **The Turtle a model converts to no longer depends on the order its heads spell their clauses.** The RDF mapping wrote a declaration's `sysml:type`, `sysml:subsets`, `sysml:redefines` and the other relationship properties in the order the clauses appeared in the notation, so converting a model to Turtle, back to notation (which writes the clauses in one order) and to Turtle again gave the same triples in a different order, and a `.ttl` kept under version control churned with the spelling of a head. Every head's relationships are now written in one canonical order (`type`, `specializes`, `subsets`, `redefines`, `references`, …; see `docs/reference/rdf-mapping.md`), the same order the notation is written back in, so the second hop is byte-identical once source text is stripped.

- **A model-level expression over a feature the model leaves open is `<undetermined>`, not an
  error and not a made-up number.** Evaluated at model level (`-e`, `%eval`, gRPC `Evaluate`),
  a reference to an attribute with no value (`attribute u;`) failed the whole expression with
  `no value for feature u`, even where the other operand fixed the answer, and a feature whose
  count the model does not fix was counted at its minimum, so `size(gear)` for a `part gear[1..*]`
  was `1` and `isEmpty(loose)` for a `part loose[0..2]` was `true`. Such a read is now the value
  `<undetermined>`, which every expression over it carries (`u + 5`, `u > 3`, `-u`,
  `if u > 0 ? 1 else 2`, `(10, 20, 30)#(u)`, `size(gear)`, `isEmpty(loose)`, `gear#(7)`), while
  what the model fixes still answers: KerML's conditional `and`, `or` and `implies`
  (`ControlFunctions`) decide on a constant second operand as they already did on the first, so
  `(u > 3) and false` is `false`, `(u == 1) or true` and `(u == 1) implies true` are `true`;
  `notEmpty(gear)` is `true` since `[1..*]` guarantees an element; `size(slots)` of a `part
  slots[3]` is `3`; `includes((1, u + 1), 1)` is `true`; a one-valued feature and an enum literal
  keep their definite counts. The result is a first-class value: the CLI and REPL print
  `<undetermined>`, gRPC `Evaluate` carries it as the new `Value.undetermined` arm (its `reason`
  and count bounds) under the `undetermined_value` capability, falling back to an unsupported
  null for a client that does not know the arm, and the Go, Python (`opensysml.Undetermined`,
  whose `bool()` raises), Node, Rust and Java clients decode it. An undetermined constraint or
  requirement condition is still not a verdict: it is reported as `no value`, naming the open
  feature. Nothing changes on an object: a feature it holds nothing for reads `<unset>`, a
  required value that is missing is still `no value for feature`, and `%instantiate` still
  materializes multiplicity minimums. An unresolved name is still `unresolved reference`.
  A model-level read never builds an open collection up to its lower bound: `part many[10001..*]`
  reads `<undetermined>` rather than failing with a multiplicity violation, the values that
  features subsetting the collection contribute are what it certainly holds (`includes(gear,
  fixed)` is `true` for `part fixed :> gear`), and `forAll`, `exists`, `allTrue` and `anyTrue`
  decide from those certain elements (`(1, u)->exists{in x; x == 1}` is `true`,
  `(1, u)->forAll{in x; x > 2}` is `false`) before answering `<undetermined>`. `select`, `reject`,
  `selectOne` and `collect` likewise apply their body to the certain elements and keep what
  results (`includes((1, u)->select{in x; x == 1}, 1)` is `true`), `includes` and `excludes`
  decide from the certain elements of both sequences (`includes((1), (2, u))` and
  `excludes((1), (1, u))` are `false`), and indexing a certainly empty sequence by an open index
  (`()#(u)`) is the index error `()#(1)` is. Such a read holds nothing, so it is not charged to
  the element budget. An undetermined value keeps the count the model fixes for it: `collect`
  over an open collection counts what its body yields per element the collection may hold, so
  `size((1, u)->collect{in x; x + 1})` is `2`; a conditional over an open test holds the count
  its branches declare, so `size(if u > 0 ? 1 else 2)` is `1` and `if b ? (1, 2) else 3` holds
  `[1..2]` values; and a feature declared `[0]` reads as the empty sequence, its count being
  fixed. `??` over an open operand that may be empty holds either that operand, then nonempty,
  or the fallback, so `rack.loose ?? 3` holds `[1..2]` values and `size(u ?? 3)` is `1`. A test
  that does not depend on the element decides a quantifier over a collection certainly holding
  one, so `gear->exists{in x; true}` is `true` and `gear->forAll{in x; false}` is `false`. A
  determined operand that alone fails an operation still fails it: `u / 0` and `u % 0` are
  `division by zero`, in the operator and the `RealFunctions::'/'` forms alike, and a determined
  position no value of an open operand admits fails `Substring`, `includingAt`, `subsequence`
  and `excludingAt` (`Substring(s, 0, 2)`, `excludingAt(xs, 5)` for `xs : Real[2..4]`) as
  `index out of range`, where a position every value admits leaves the result `<undetermined>`.
  A sequence fixes each position up to its first element of open count, so `(10, u, 30)#(1)`
  is `10` and `#(3)` `30` while `#(2)` stays `<undetermined>` and `#(4)` is `index out of range`,
  as `head`, `last`, `tail`, `subsequence`, `excludingAt` and `includingAt` read them; an
  insertion index is checked against the upper bound plus one without overflowing it, so
  `includingAt` at a bound of `9223372036854775807` is `<undetermined>`, not rejected.
  A multiplicity bound the model does not evaluate (`a : Real[n]` over a valueless `n`) fixes no
  count either: such a read is `<undetermined>` of the bounds the declaration does fix rather
  than the `cannot materialize … with unknown multiplicity` error, which stays the object-level
  answer. An open value is judged by the type its feature declares, so an operator or library
  function that admits no value of that type is the `type mismatch` it is for a determined
  value (`s - 1`, `not s`, `s > 1`, `if s ? 1 else 2`, `(10, 20, 30)#(s)`,
  `RealFunctions::sqrt(s)` for `s : String`; `b - 1` for `b : Boolean`;
  `StringFunctions::Length(r)` for `r : Real`), while one it admits stays `<undetermined>`
  (`s + "a"`, `r - 1`, `not b`) and a constant operand still folds (`false and s`). A feature
  chain through an open collection reads the members of the values it certainly holds, so
  `includes(gear.tag, "x")` is `true` for a `part tagged :> gear { attribute :>> tag = "x"; }`
  and `gear.tag->exists{in x; x == "x"}` decides on it. An open operand that certainly holds
  several values (`xs : Real[2..4]`) is no scalar: `xs + 1`, `xs > 1`, `-xs` and `(10, 20)#(xs)`
  are the `type mismatch` a sequence is, and a one-valued library or calc parameter
  (`RealFunctions::abs(xs)`, `StringFunctions::Length(ss)`) the `multiplicity violation`, while
  one that may hold a single value (`os : Real[0..4]`) stays `<undetermined>`; the named
  `ControlFunctions::'if'` checks its open test as the operator form does, so `'if'(r, 1, 2)`
  for `r : Real` is a `type mismatch`. A subsetter whose own count is open is read as the
  collection it fills is, never built up to its lower bound: `part sub[10001..*] :> base` leaves
  `base` `<undetermined>` holding at least `10001` values and no made-up object.
- **A calc-typed parameter whose value names a calc (`in calc f = twice;`) applies that calc when
  called by its qualified name.** `Apply::f(3.0)` outside a run of `Apply` failed with `calc
  Apply::f has no return expression`; it now applies `twice` as the bare `f(3.0)` does, and a run
  that bound `f` still applies the binding.

- **A multi-valued feature not declared `nonunique` refuses a repeated value.** KerML defaults `isUnique` to true, so `Collections::OrderedSet`/`OrderedMap` elements, `attribute xs : Integer[*] ordered` and a plain `Integer[*]` alike hold no two equal values; `OrderedSet { :>> elements = (1, 1, 2); }` was read as three elements. A repeat the checker can decide from a literal is a `type.expr` error (`1 (an Integer) is written at positions 1 and 2 of a unique feature`); one that appears only when the model runs — through a feature, a body-local declaration's initializer, an `assign`, a binding, a calc argument or result, a compiled calc in Go or C — is the typed `ErrUniquenessViolation`, reported after multiplicity and type, leaving the feature's prior value in place. Nothing is deduplicated. Equality is the runtime's own, so `2 [kg]` repeats `2000 [g]` and an enumeration literal repeats itself. `nonunique` opts out; a feature held as a `Collections::Set` or `Map` keeps set semantics. A redefinition stating neither `ordered` nor `nonunique` takes the uniqueness of what it redefines, so `:>> num = (0, 0, 1)` and `:>> mRefs = (mm, mm, mm)` over the library's `nonunique` tensor features stay legal; the `Collections` redefinitions whose notes say "unique by default" are read as unique, recorded as a library erratum. A `nonunique` redefinition is judged against its target's effective uniqueness, so redefining a feature that itself inherits `nonunique` is legal.

- **An unset quantity attribute no longer fails `-instantiate`.** A part whose quantity
  attribute nothing values (`attribute mass :> ISQ::mass;`) made the materialization check
  evaluate the derived `dimensions` of the `MassValue` it does not hold and report
  `feature value MassValue.dimensions: no value for feature mRef` with exit status `2`, once
  per unset quantity of any ISQ kind. The check now descends only into objects a feature value
  holds, not into the object standing for an unset one, so such a model is reported clean and
  exits `0` while `%features` lists the attribute `<unset>` as before. A bound quantity still
  derives its `dimensions`, and a default that genuinely fails beside or beneath an unset
  quantity is still reported.

- **The `first` end of an action body's `first a then b;` is a reference to the succession's
  source, not a name the node declares.** The symbol table no longer registers a label under `a`
  for the two-ended form in an action body, so `a` resolves like any other succession end: a
  source nothing declares is reported as `unresolved reference` where it is written instead of
  being taken for a member the node declares, and `a` is collected as a reference for
  find-references and rename. A one-ended `first a;` still marks the start of the flow under `a`'s
  name, and a state machine's `first start then off;` still declares its `start` for transitions
  to name. The AST keeps the name after `first` as a qualified name (`InitialNode.First`), so the
  RDF mapping, the REPL and the language server read the source through the one binding; no
  notation, output or graph changed.

- **A two-ended `first a then b;` in an action body is the succession a → b, not a second start.**
  Lowering used to collect every `first` member as the action's initial node, so a body writing
  `first start;` beside one or more `first a then b;` successions — as OMG's
  `3a-Function-based Behavior-2` and the Annex A vehicle models do — validated clean and then
  failed at `%action` with `action has multiple initial nodes`. A two-ended `first`, with or
  without a body, now lowers to the same edge `succession first a then b;` does, any number of
  times and beside `first start;`, `then` chains, forks, joins, decisions and merges; only the
  one-ended `first start;` and `first a;` mark where the flow starts, two of them are still
  rejected, and a body writing neither still starts at its one node no succession leads to. The
  validation pass checks both ends of a two-ended `first`, and the RDF mapping is unchanged.

- **Every bundled library file converts back from its graph without source text.** Reading a
  `.ttl` stripped of `sysx:sourceText` refused seven library files. A cross feature an `end`
  declares in its head (`end happensWhile subsets timeCoincidentOccurrences feature …`) was
  refused whenever its id had to be written, since the head holds no annotation: the reader
  now writes the id as `metadata : IdentityMetadata::ElementId about <end> { … }` in the end's
  body, and a normative library id, which the rebuilt notation implies on its own, is not
  written at all; a graph that gives the cross feature a body or members of its own is still
  reported. A declaration inside an expression body (`{ in x; attribute k = 2; x * k }`) was
  carried only as its notation and refused without it: it is now a `sysx:bodyMember` node of
  its own, typed by its metaclass and carrying what a namespace member carries, nested bodies
  included, and is written back from that structure. An invocation of a named function
  (`f(a, b)`, `x->f()`, `new T()`) was refused: it is now spelled from its `sysml:function`
  link by the same rule as every other reference, so a function the graph does not define, or
  that no spelling reaches, is still refused rather than misspelled. The inherited target of a
  cross feature's `subsets` is resolved in the end that owns it, so the spelling written back
  is the one the library wrote.

- **The Rust client's private-service startup error carries the exit status.** When the service closed its stdout without serving an address, the client read the exit status before the process had been reaped and reported `code None`; it now waits briefly for the exit and reports the real code, so a binary that exits immediately is described as `code Some(0)` rather than as still running.

- **`%send` reaches an object whose performed action is waiting at an `accept`.** The REPL used to
  refuse a signal to any object that exhibited no state machine (`runs no state machine, so
  nothing there accepts a signal`), so the only way to wake a lone object's performed action was a
  sibling's machine sending to it. `%send <signal> to <object>` now asks every behavior the object
  runs — the machines it exhibits and the actions it performs, an accept nested in a performed
  action included — and posts the signal on the runtime's message bus when one of them is parked
  for it, reporting the accept that takes it (`Accepted by performed action "main" waiting at
  accept g`); the action goes on from its accept at the next `%advance`. An object none of whose
  behaviors accepts the signal is refused with each behavior's standing, and one running no
  behavior at all with a hint to start one. With an `%action <name> <object>` session open, a bare
  `%send <signal>` goes to that object, as it goes to the `%state` session's; an `%action` session
  performing on behalf of no object still has none to send to, and says so.

- **A state body's two-ended `first a then b;` is the succession `succession first a then b;`
  spells with its keyword: a completion transition from `a` to `b`.** It was read as an initial
  node and lowered as an entry transition into `b`, so a machine with no `entry` marker silently
  started in `b`, and one written `entry; then a; first a then b; first b then c;` never left `a`.
  The keyword-less spelling now parses as the same `SuccessionAsUsage`, resolves nested, qualified,
  region-local and pseudostate ends as a `succession` does, and designates no initial state; a
  machine whose only edges are such successions is reported as having no initial state, as any
  exhibited machine without one is. `first start then off;`, leaving the `start` every state
  inherits, still designates where the machine starts. A one-ended `first a;` in a state body
  orders nothing and is reported (`first-names-no-target`) rather than ignored. Validation and the
  RDF mapping follow: the two-ended form is checked and exported as a succession, with no
  `sysx:InitialNode` written for it.
- **A succession end written as a feature chain (`succession first b then c.c1;`, `first b then
  c.c1;`) names the same nested vertex as `c::c1`.** The chained end was dropped, so the edge into
  `c1` was never lowered; it is now resolved through the endpoint lookup as the qualified spelling
  is, at either end of the edge, its first segment reaching a vertex nested anywhere in the machine
  as a qualified end's does, and a chain whose member or operand is not a state or pseudostate is
  reported.

### Performance

- **The Python client answers `Model.find`, `Model.get`, `model[name]` and `name in model` from
  the service's index instead of walking the symbol tree one RPC at a time.** A qualified name
  is one `GetSymbol` call; a short name is one `Query` on the effective `name` followed by one
  fetch, so a lookup costs the same on a model of ten symbols and one of ten thousand, where the
  walk took tens of milliseconds. What is found is unchanged: the outermost symbol of a shared
  short name, and among those the one declared first. A library symbol is now reachable by its
  qualified name too (`model.get("ISQBase::mass")`), with a name declared in the model winning over
  a library package of the same spelling. A service without the `query` capability is still
  walked. `import opensysml` no longer loads the HTTP stack, which only a release download
  needs; `opensysml.binary.fetch` imports it when one happens.

- **The gRPC service answers `Evaluate`, `Instantiate`, `VerifyConstraint` and `Sweep` on a
  model it holds without rebuilding its resolver and semantic model per request.** A held model
  keeps a pool of idle analysis workers; a request takes one, builds one only when the pool is
  empty, and returns it with the diagnostics it added trimmed away, on the success, error and
  cancellation paths alike. Two concurrent requests still work on workers of their own, and the
  pool keeps at most as many workers as the machine can run at once, so a burst of requests does
  not leave the model holding a worker per request. An
  `Evaluate` that had grown from 10 µs to 3.5 ms is 8 µs, `VerifyConstraint` on a held model is
  ten times faster than in 0.7.0, and the Python client's `Instantiate` on a held model is a
  third faster than 0.7.0 instead of three to eighteen times slower.
- **Writing a scalar to a typed feature no longer walks the type's difference closure or formats
  its own refusal message on every write.** Whether a type's closure subtracts anything is
  memoized per type, a write's description is built only when the write is refused, a timed
  wait formats its holder and subject only when a caller asks, and the library symbol a
  qualified name denotes is looked up once. The messages a refused write or a wait report are
  unchanged. A feature write is 11 times faster than it had become and a fifth faster than in
  0.7.0; the REPL's interpreted `SumTo(1000000)` and `Collatz(27)` calcs, action loops and
  assignment loops are back at 0.7.0's speed.
- **Lowering an action no longer computes every node's static read/write footprint up front.**
  The footprints exist for the model checker's independence relation, which is their only
  reader; they are projected once on first use and share one scan of the graph's declared
  features. Lowering a 1 000-step action chain takes 8 ms instead of 23 ms, as in 0.7.0.
- **Loading and validating a model asks the semantic model fewer repeated questions.** The
  library base a declaration's kind implies, the index order of a document's annotations and
  the engines an analysis kind dispatches to are computed once; probing whether an operand
  names a unit builds no diagnosis for the operands that do not; a scope indexes its members by
  declaration as well as by name; and the did-you-mean table takes the index's registered names
  directly. Diagnostics are unchanged. Loading a 4 000-element model in the REPL is within a
  tenth of 0.7.0's time instead of a quarter slower, and a whole-file `Analyze` is within noise
  of 0.7.0 instead of half again slower.

- Expanding a model's wildcard imports of large library packages (`import ISQ::*`,
  `import SI::*`) is about 30% faster and allocates about a quarter less: the symbol
  index keeps its re-export and hidden marks and its per-segment name table as small
  sorted slices rather than one map per name, reuses a re-export claim's writable
  record instead of looking it up again, passes an unfiltered import's inherited routes
  on without copying them, and no longer notes a parent namespace's change twice per
  re-exported member. Semantics are unchanged; the embedded standard-library snapshot
  is regenerated for the new table layout. `BenchmarkExpandModelImports` in
  `internal/core/libs` measures the cost.

## 0.7.0 — 2026-09-10

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
  region declaration order. Exploring the whole suite found one case pinning a scheduling
  artefact — two accepts on one port, addressed by two sends, binding one `value` whose last
  writer is open — and it is restated as the two outcomes the oracle derives. Cases without
  `outcomes` are not explored, and nothing changes under the default schedule.

- **A `Collections::Set` holds a set.** Where the Kernel Data Type Library declares a collection's `elements` unique and unordered — `Set`, `UniqueCollection`, `Map` — the runtime now holds them as a set value: each member once, `size` counting members, equality that ignores the order the members were written in, and `contains`/`containsAll` as membership. A set consumed by an ordered operation (`collect`, `head`, `#`, a comparison against a sequence, a trace, a write into an `ordered` or `nonunique` feature) enumerates in one canonical order — Booleans, then numbers ascending, strings, quantities, enumeration literals, objects — so equal sets behave alike. What the library declares ordered or nonunique (`Bag`, `List`, `Array`, `OrderedSet`, `OrderedMap`, every `SequenceFunctions` result) is unchanged.
- **Tensor quantities of any rank.** A `TensorMeasurementReference` with three or more `dimensions` builds a rank-three-or-higher tensor whose `#` takes one index per dimension; the wrong number of indexes, an index out of its dimension's range, a non-Integer index, a component count off the flattened size and arithmetic between two shapes are each a typed error naming what was wrong, and the shape survives `+`, `-` and the scalar multiplications.
- **Sets and tensor quantities cross gRPC whole.** `Value` gains a `set` arm (the members as `Value`s, in canonical order, readable in any order, a repeated member refused by the service and by every client) and a `tensor_quantity` arm (`dimensions` and one `Quantity` per row-major component, at any rank), advertised as the `set_values` and `tensor_values` capabilities. The Go, Python, Node, Rust and Java clients decode both to native types that check their own invariants, send them as calc arguments and refuse them to a service that does not advertise the capability; a service withholding one reports the unsupported null it always did. Neither value has an RDF literal form — the mapping writes the model's expressions, which round trip exactly — and neither compiles natively: `sysml -compile` refuses a calc that uses one with a typed error naming the type.

- **One simulation clock, owned by the runtime context, that actions and state machines wait
  on together.** Simulation time is a property of the runtime a behavior runs in, no longer of
  one state machine's executor: every state machine and action a context runs — the ones
  `%action`/`%state` debug, the ones an object exhibits, the ones a body performs — reads the same
  clock (`Context.Clock()`, in `SI::s`), so two machines materialized in one context share time,
  and a nested performance runs on the enclosing clock rather than a copy. An action body now
  waits on it: `accept after <duration>` parks the token until the clock has moved that far from
  the moment the accept was reached, `accept at <instant>` until the clock reads the instant (one
  already passed is due at once), with the unit conversion and the `DurationValue`/`TimeInstantValue`
  refusal a transition's trigger gets; an action running on its own moves the clock to each wait
  as it reaches it. Advancing moves everything: `Context.Advance(duration)` runs every state
  event, action token, change-condition poll and do round due up to the new instant, instant by
  instant, within the event, do-step and step budgets, and returns what it moved. `-advance` no
  longer needs `-state`: it runs the invocation's `-action` and `-state` behaviors together on the
  one clock (an action sending a signal a machine accepts after a delay), and an action still
  parked on the clock when the time is up is reported as undecided with the instant it waits for;
  `%advance` moves the clock of the session's runtime, so an `%action` debugger parked at
  `accept after 5 [SI::s]` and a `%state` debugger both move and the report covers each, and
  `%step` on a token waiting only on time says so and names the `%advance` that would move it.
  Which executor runs first when several are due at one instant — an action token and a state
  transition, two machines, two actions, two executors polling a change condition once the
  definite work at the instant has settled — is a new choice point, `due order`, drawn by the
  scheduling policy (`choice at t=5.0: due action watcher, state machine blinking of object #1
  (unordered; ran state machine blinking of object #1 first)`): the executor started last runs
  first under the default `reverse`, the first started under `declared`, a draw under `seed:<n>`;
  one executor alone due is no choice and is not reported, so every existing single-behavior
  result and trace is unchanged; `explore` enumerates every due order like every other choice
  point, and `sysml -schedule explore -action … -state … -advance <time>` tables the joint outcome
  of the behaviors run on one clock, each behavior's observables under its name, once per order the
  due executors can run in. `ExecuteActionResponse` and `ExecuteStateResponse` report
  `final_time`, the clock's reading when the run ended, advertised as the `final_time` capability
  (`CAPABILITY_FINAL_TIME`, `Capabilities.FINAL_TIME`) and read by the Python client's
  `execute_state` result and by the generated response types of the Node, Java and Rust clients. The
  robustness case that pinned the old refusal of a time trigger in an action body
  (`action_accept_time_trigger`) is replaced by `action_accept_time_waits` and `clock_advance`,
  which pin the waits firing, a negative, infinite or not-a-number duration and one leading past
  the last instant the clock can hold (each refused, the clock unmoved), a duration of another
  dimension, an instant already past, a wait beyond the advance, an advance of zero, one with
  nothing waiting, and a wait met in a flow a body runs or in an action a node performs, which
  pauses the token's work until the clock reaches it rather than moving the clock past a bounded
  advance's deadline.

- **A trade study runs as the library writes it.** Running a `TradeStudies::TradeStudy` definition or usage through `-analysis`, `%analysis`, `RunAnalysis` or a sweep over them executes the library's own expressions rather than a special case: the subject binds `studyAlternatives`, the case's `evaluationFunction` binds `tradeStudyObjective.eval` as a function value, `MinimizeObjective`/`MaximizeObjective` compute `best` with `->minimize {in x; eval(x)}`/`->maximize`, the inherited `require constraint { eval(selectedAlternative) == best }` is checked as the objective's own condition, and `selectedAlternative` is the first alternative `->selectOne` finds it holding for. The general rules this needed: a domain library's calc executes from its text as a model's does; an inherited expression reads a feature through the running case's redefinition of it (`studyAlternatives` is the case's `subject`); a requirement usage applies as a predicate with its subject as its first parameter (`tradeStudyObjective(selectedAlternative = a)`); and a redefinition stating no multiplicity inherits the redefined feature's, so `subject : Engine = (a, b);` is `[1..*]`. Every application of the case's calc is reported with the run, in subject order, with its result or error and marked `[selected]` for the one `selectOne` picked and the case returned (never for a result that merely equals an argument) and `[tied]` for a later one scoring the same; an alternative whose evaluation fails, an `evaluationFunction` left without a body and a subject listing no alternative or redeclared `[1]` are typed errors that leave the objective undecided, never a fabricated pick. A run one of whose outputs fails keeps the outputs and evaluations it reached but leaves every objective and assertion undecided for the failure, and each evaluation is reported once per distinct binding of the calc's parameters, its arguments spelled by parameter position. The trace shows each alternative scored and the pick in that order.
- **Case evaluations cross the API.** `RunAnalysisResponse.evaluations` and each `SweepRow.evaluations` carry one `CaseEvaluation` per application — the calc's id, its arguments (an alternative as an `instance_id` resolving in `instances`), the result or an `error`, `selected` and `tied` — under the new `case_evaluations` capability, kept on a failed run beside the outputs and verdicts it reached; `-json` writes the same `evaluations` on each check and sweep row. The Go package exposes them as `Analysis.Evaluations`/`Selected()` (`Evaluation`, its alternatives resolved by `Analysis.Instance`) and a failed run as `*AnalysisError`, whose `Partial` keeps what the run computed; the Python client as `AnalysisResult.evaluations`/`.selected` and `SweepRow.evaluations` (`CaseEvaluation` with `explain()`, and `AnalysisRunError.result` on a failed run); the Node, Java and Rust clients read them from the wire messages, the Rust client through the new `Connection::call`. `%optimize` refuses an objective whose `eval` is bound to the case's own calc — a choice among listed alternatives, not an optimum over a continuous domain — and points at the analysis run.

- **A verification case runs and reports the verdict of its body.** `sysml -analysis`, `%analysis` and the `RunAnalysis` RPC accept a `verification def` or `verification` usage and run it the way they run an analysis case — the same lowering, subject and input binding, and step execution. The `VerdictKind` the body produced is reported: `pass` or `fail` as the library's own `VerificationCases::PassIf` calculation computes it, a `VerdictKind` literal the body binds as it stands, `inconclusive` for a body that produced no verdict value, and `error`, carrying the message, for a body whose run failed.
- **The body verdict is reported beside requirement satisfaction, not instead of it.** `-requirement`, `-satisfy`, `%requirement`, `%satisfy` and the `VerifyRequirement`/`VerifySatisfaction` RPCs add one line per verification case verifying the requirement; what the requirement engine decided, and the exit status, are unchanged. A case performed as a step of another is reported on its own, marked as a subcase, since the library states no roll-up. Over gRPC the verdicts are added fields (`verification_verdicts` on `VerifyRequirementResponse`, `VerifySatisfactionResponse` and `RunAnalysisResponse`), advertised as the `verification_verdicts` capability, and `sysml -json` reports them under `verifications`. Each carries the `requirement_id` it was reported for, matching the `requirement_id` a `satisfy` verdict carries, so a satisfaction response covering several requirements is read per requirement. The Go and Python clients report the verdicts as `Verifications`/`verifications`, giving each satisfaction verdict the cases of its own requirement.

- **A worked walkthrough of analysis cases, `examples/analysis-demo`.** One lander model asked every way the tool answers: an analysis whose action steps feed each other and whose objective is a requirement, run bound, with arguments and on an object; a verification case whose body verdict is reported beside its objective; a parameter sweep and a seeded sample; two trade studies choosing among three landers, one swept over its cost parameter; and an action and a state machine due at the same instant of the shared clock, run under the default and `declared` scheduling policies and under `explore`. Each command is shown with its output and what to read in it, with `-trace` and `-json`, the REPL forms and the same questions asked through the Python client in `lander_demo.py`.

- **A worked example of the expression forms.** `examples/expressions-demo.sysml` and its walkthrough `examples/EXPRESSIONS-DEMO.md` take `as` casts, the unbounded value `*`, `.metadata`, function values, `Collections::Set`, rank-three tensor quantities and typed collection bodies through one payload model, and the guide's expressions chapter (`docs/guide/05-checking.md`) gains sections on each with REPL transcripts.

- **A design note for SMT bounded model checking of behaviors**
  (`docs/internals/design/smt-model-checking.md`). It proposes unrolling an action's token flow
  to a bounded number of moves and asking the SMT solver whether any schedule, for any value of
  the inputs the model leaves unbound, violates a requirement, deadlocks or leaves a feature's
  final value depending on the order of two moves. It fixes what a verdict may claim — proved
  within a stated bound, violated with a witness the interpreter replays, sensitive with the two
  schedules, or not covered with the reason — a per-construct coverage table, the referee gate
  against `-schedule explore`, and the stages. Nothing is implemented; the note exists to be
  reviewed before code is written.

### Changed

- **What may differ from 0.6.0.** A model that mixes actions and state machines can take another
  interleaving under the default schedule than 0.6.0 took, one the library leaves equally open:
  actions and state machines now wait on one simulation clock owned by the runtime context — a
  nested performance runs on the enclosing clock rather than a copy, and two machines materialized
  in one context share time. A single behavior run alone is unchanged. The golden execution traces
  changed too: they gained a `choice` line at every pick the library leaves unordered and a step
  boundary at synchronized joins, which renumbers the steps after it, so a 0.6.0 checkout
  re-running `-update-traces` sees those lines and boundaries and the corrected traces of the
  fixes below; no `.expected.json` schema and no `-json` report shape changed. Every wire change
  since v0.6.0 is additive — `make proto-breaking BUF_BREAKING_REF=v0.6.0` passes — and no CLI
  flag, REPL command, RPC or wire field was removed or renamed: `sysml` gains `-schedule`,
  `-sweep`, `-samples` and `-seed`, the REPL gains `%schedule`, `%sweep` and `%samples`, and the
  service gains `RunSweep` and fields advertised under new capabilities. The other results that
  change are corrections of results the Kernel Semantic Library derives otherwise, listed under
  *Fixed*: a join, or a node several successions reach, fires once per succession; a loop through
  a merge re-enters it; a breakpoint on a synchronized node pauses once; and a via-less `accept`
  no longer takes a transfer addressed to a port. A collection body whose result type does not
  fit the receiving feature (`accept when counts.{in n : Integer; n}`, `attribute i : Integer =
  xs.{ in x : C; 1.5 }`) is now refused where 0.6.0 let an ill-typed model through.

- **Pull requests run one CI, GitHub Actions; CircleCI runs on `main` and tags.** The CircleCI `build-test` workflow is filtered to `main`, so a pull request no longer runs the suite twice, and the checks that only CircleCI carried moved into the pull-request workflow: the protobuf lint and wire-compatibility check (against the branch the pull request merges into) join `Go static and integrity checks`, the documentation hygiene checks (`make docs-check`, `make man-check` and the census check) join `Documentation site`, the release-digest check and the check that the committed stubs are what buf generates join each client's job (the Python and Java stub checks were CircleCI-only), and `make conformance` with the `-transport grpc` run join the renamed `Conformance suite` job. Every stub check, in both configs, now also fails when a committed stub was deleted and regeneration brings it back.

- **The coverage the SonarCloud scan reads now measures every test suite the checks already run.** The command-line binaries the tests build and run (`sysml`, `sysml-grpc`, `sysml-lsp`) are built instrumented under `make coverage`, and the counters each run writes are folded into `coverage.txt`; the conformance suite runs in-process under `go test` as well as over the wire; the repository scripts and the Python release scripts run under coverage (`make scripts-coverage`, read as a second Python report); the Node conformance runner and the example runner are tested under c8; the Java conformance runner's command line is tested in-process under JaCoCo; and the ontology-table and stdlib-snapshot generators have tests over their `run` functions. The `sysml -compile -source` path is tested for each target. Build tooling no test suite executes (mkdocs hooks, the buf plugin, benchmarks, the release-fixture recorder, the VS Code extension host glue and its build steps) is excluded from coverage only, each with its justification in `sonar-project.properties`. No product behavior changes.

- **Before 1.0, the version segment a release bumps is decided by model compatibility.** A release that still accepts every model the previous one accepted, with the same diagnostics and results, and removes or renames no flag, command, RPC or wire field, is a patch release even when it adds features; a release that refuses a previously accepted construct, changes a correctly derived result, removes or renames an interface, or changes a fixture or output format so that existing artifacts fail, is a minor release. CONTRIBUTING.md § Versioning states the rule and the Release Checklist asks for the chosen segment to be justified against it.

- **The documentation now states which orders a behavior leaves open and how each is checked.** Every open ordering in `docs/project/behavior-semantic-oracle.md` names the `outcomes` or `.trace.order` file that encodes it and the run and outcome counts `sysml -schedule explore` reaches — or the single-outcome fixture that pins an ordering nothing observable depends on — and `docs/project/spec-compliance.md` grades choice-point reporting of every kind, `guard-unevaluable` tolerance, the harness's unreachable- and unlisted-outcome checks, the exploration budget and per-driven-run state, with the scheduling-policy row now faithful rather than approximate.
- **The behavior guide has a section on models with more than one valid run.** It walks the three-writers fixture through `%trace on`, `seed:<n>`, `explore` and its `incomplete` status, and writing `outcomes`, `admissible` and `.trace.order` for a conformance case, with the output each command prints; the client chapter spells `schedule=`, `WithSchedule`, `explore_action` and `ExploreAction` per client and says which clients carry no execution surface at all.

- **The SonarCloud findings outside cognitive complexity are cleared again.** Duplicated literals are named constants, same-typed parameters share a declaration, `encodeMember` takes its member head as a struct, marker methods state their contract, unnecessary locals are inlined, the release-gate script reports errors on stderr, the MSI script names its positional parameters, the Java transport catches connect timeouts in their own block, and the Java and Python tests hold one call per exception assertion. The exhaustive switches of the AST codec and the planners' error kinds, and the sealed code-generation IR, are documented exclusions. No behavior changes.

- Development moved to a `develop` integration branch; `main` now carries releases only.
  Feature and fix pull requests target `develop`, releases reach `main` through
  `release/x.y.z` pull requests and are tagged there, and CircleCI builds and tests both
  branches. `make proto-breaking` compares against `origin/develop` by default.

- **The contributor documentation now covers how a run resolves what the library leaves unordered.** A new design note, `docs/internals/design/scheduling.md`, describes the six kinds of choice point and how each is recorded without altering the run, the `reverse`, `declared`, `seed:<n>` and `explore[:runs=N,depth=D]` policies and what each draws, replay-based exploration from a fresh context per run with its witnesses, outcome grouping, budgets and `incomplete` verdict, and the conformance contract behind it — plural `outcomes` with their `admissible` citations, `.trace.order` partial-order constraints, per-policy trace goldens and the whole-suite sweep under `declared` and `seed:1`. The architecture and testing overviews and the orthogonal-regions note point to it, the latter now describing how sibling regions' reactions to one event or one change are dispatched through the scheduler and reported as a `region order` choice. The `--trace`, `%trace` and `SchedulePolicy` descriptions list region order among the choice kinds they report, and the REPL guide's command table gains a row for `%schedule`.

### Fixed

- **A transition's `accept` with no `via` no longer takes a transfer addressed to a port.** An accept naming no port receives as the performer of the machine (SysML v2 §7.16.7), and a port is a sub-occurrence of its part, not the part, so `send new Ping() to alpha.inPort` — or a send routed to `inPort` over a connector — is now taken only by `accept Ping via inPort`; a via-less `accept Ping` on the same state is not enabled by it and no choice point is reported between the two. A transfer addressed to the part itself, `send new Ping() to alpha`, is still taken by the via-less accept and not by the `via` one. The state executor now judges every message by the same rule its dispatch check and the action executor already applied, so the two agree on what a machine can react to; call and change triggers are unaffected.

- **A breakpoint on a node several successions reach pauses once, before its one performance.** A join, or a plain action node two fork branches converge on, stopped a `%continue` once per arriving token and let a resumed run pause at it again. The run now stops there once, when every arrival is in, and resumes past the node; a node a loop re-enters still stops before each pass. A step of the executor now moves each token at most once — a token a step created or moved, a fork's branch, the one a synchronized node performs with or one sent into a nested flow, takes its first step in the next — so traces of forks and joins gain a step boundary between the last arrival and the node's performance, with the same statements in the same order.

- **A collection operation's static type follows what its declaration hands through, not the element type of the collection.** `xs->collect { in x : C; x.mass }` and `xs.{ in x : C; x.mass }` are typed by the body's result (`MassValue`), a nested collect by its innermost body, a body answering a sequence by every element type and `xs->collect f` by the named function's result; `select`, `reject` and `selectOne` keep the elements of `xs`, `reduce` follows its reducer's result — and the element a one-element collection hands back unreduced, unless the collection is known to hold two or more, by its own multiplicity, one it inherits by redefinition, or a chain through such features; a collection holding one at most is never reduced, so its element alone is the result — and `forAll`/`exists` stay `Boolean`, each with the multiplicity the Kernel Function Library declares. A body whose result cannot be typed keeps the library's `Anything`. Value conformance, a feature's bound value, invocation arguments, trigger arguments and enumerated values are judged by the specialized type, so `accept when counts.{in n : Integer; n}` is refused where it was silent, and `when counts.{in n; n > 3}` is accepted where it was refused. A scalar literal a body writes out is as exact as one bound directly: `attribute i : Integer = xs.{ in x : C; 1.5 }` is refused, and a quantity it writes out is measured against the target's dimension: `attribute t : DurationValue = xs.{ in x : C; 5 [m] }` is refused. `xs.?{…}` binds and is typed as `xs->select {…}` is — `(v1, v2).?{ in v : Vehicle; true }` is a `Vehicle` collection, not the `Anything` the sequence is; elements of sibling types share their nearest common supertype, so `(truck, car).?{ in v : Vehicle; true }` is a `Vehicle` collection too — and an element that is itself a collection value binds by the elements it holds, so `attribute i : Integer = xs.{ in x : C; xs.{ in y : C; 1.5 } }` is refused. A collection over `()` or a feature admitting no value keeps its declared type — `()->collect { in a : Integer; "s" }` is a `String` collection — but holds no element, so none is judged and its `reduce` takes nothing from an element it would hand back unreduced; so does one mapping every element to a `[0]` feature or function result — the multiplicity read through an alias, or from the feature or result a redefinition inherits it from — or any operation over such a collection, and an argument holding nothing is judged against no parameter type. An argument or constructor value that is a collection binds each element it holds on its own, so `Sail(vs.{ in v : Vehicle; (v, boat) })` and `new Fleet(vs.{ in v : Vehicle; 1.5 })` are refused by the element that does not bind where they were silent. A collection value binds as many values as it is known to hold, counted against the feature's multiplicity — `part b : Boat[1] = pair.items.{ in v : Vehicle; boat }` binds two — a `reduce` counting the one element it hands back unreduced or what its reducer yields over two or more, so `pair.items->reduce { in a : Vehicle; in b : Vehicle; (a, b) }` binds two and one over `()` none. A body reading the feature it values terminates as a self-referential argument does.

- **A paused debugger run resumes with its own state, not another run's.** An `%action` or `%state` session paused between steps while another run happened on the same session — an `%eval`, a `%calc`, an `%invoke`, a run to completion — resumed with what that run left behind: its step and element budget replaced the paused run's (so a step that fit the budget before could fail with the other run's spending, or a run could spend past its bound), its choice points and unevaluable guards stood in for the paused run's, and the calc usage evaluations of an activation paused at a breakpoint were dropped, so the next read ran the body again. Each run now keeps a state of its own — budget spent, notes, scheduler and calc usage memo — that every call into a driven executor installs, so the paused run continues where it left off and the run in between spends and notes on its own; the executor reports its own notes and the REPL's choice summary counts from them. The scheduler was already the run's own; the other three fields join it.

- **A node several successions reach is performed once, after one token has arrived over each of them.** A token now records the succession it travelled (`Token.Via`, the lowered `ActionEdge` with its `Source`), and a join — or a plain action node two or more successions reach — fires when every incoming succession has delivered one token, the arrivals collapsing into the one token that performs it; a second token over an already-delivered succession waits for the next firing instead of standing in for another succession, and a join one of whose successions no token can travel deadlocks (`ErrActionDeadlock`) rather than firing on a token count. A plain node in a loop or behind a decision still re-performs once per pass: it awaits a succession only while some token can still reach its source before the node performs. `action_join_one_token_per_incoming_succession` (`log = 12`) and `action_node_with_two_incoming_successions_runs_once` (`hits = 1`) leave the known failures; the REPL's `%tokens` says which succession a held token arrived over and which it awaits.

- **The order orthogonal regions react to one event in is reported as a choice point under every scheduling policy, and `seed:<n>` varies it.** When one event enables transitions in two or more regions the library leaves their order open, but only `explore` recorded the choice; `reverse`, `declared` and `seed:<n>` took declaration order and said nothing, so a seed could not reproduce a region order `explore` found. Every dispatch among two or more regions is now drawn from the run's scheduler like every other pick and reported as `choice on <trigger>: states <a>, <b> react (unordered; took <a> first)` — a trace line, a `choice-point` diagnostic of kind `region order` over gRPC and Connect, and one more choice in the REPL summary. `reverse` and `declared` still take declaration order, so no output, state visit or final state moves under the default; `seed:<n>` now draws the order and replays it, and `explore` is unchanged. A change occurrence that raises the conditions of transitions in several regions at once is dispatched through the same draw, so its region order is reported and varied too, where before every policy took declaration order silently. The conformance cases whose regions react to one event list both orders as admissible outcomes.

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

- **`explore` advances one token per step, so `complete` covers every interleaving.** Exploration
  used to permute the tokens of one lockstep step, in which every steppable token moved once, so a
  branch of two nodes could never both run before a concurrent branch's one node: a fork of
  `left1 { x := 1 } → left2 { y := x }` against `right { x := 2 }` reported `complete` with two
  outcomes and missed `x = 2, y = 1`. Under `explore` a step is now one token advancing one node,
  the tokens able to act are picked among afresh after each move, and each pick is its own
  `step N:` choice point in the witness; the fixed policies (`reverse`, `declared`, `seed:<n>`)
  keep their sweep, so no default trace changed. Run counts grow with the finer granularity
  (`action_merge_fork_branch_and_loop` needs `explore:runs=10000` to complete) and the semantic
  oracle's figures are re-derived; `action_explore_write_between_branch_nodes` pins the case.
  A performed action paused on the clock is among the tokens an exploring step picks from once
  its wait has ended, so a sibling accept due at the same instant no longer always runs first:
  `action_explore_performed_and_accept_due_together` reaches both writes, six linearizations.

- **Sweep and sample values are typed by the parameter they bind, not by the literals of the range.** `-sweep`, `-samples`, `%sweep`, `%samples` and `RunSweep` resolve each range against the parameter's declaration: a `Real` or `Rational` parameter swept over `1..4:1` is bound to `1.0`, `2.0`, `3.0`, `4.0` and the table shows them so, and sampled over `1..4` draws reals in `[1, 4)` rather than the four Integers; an `Integer`, `Natural` or `Positive` parameter — or an `attribute def` specializing one — swept over `1.0..3.0:1.0` is bound to the Integers `1`, `2`, `3`, and sampled over `1.0..4.0` draws Integers inclusively. A fractional endpoint or step over an Integer parameter (`1.0..3.0:0.5`, `1.5..3`), a value below what a `Natural` or `Positive` holds, and a range over a `Boolean`, `String`, enumeration or non-scalar parameter are refused before any run, naming the parameter and its type, where before half the rows failed one by one. A quantity-typed parameter is typed through its `num` — refusing a magnitude a `num : Natural` or `num : Positive` cannot hold, and any range where the `num` holds no number — and keeps the first endpoint's unit; a `Number`-typed parameter and one declaring no type take the range as written, the untyped one noted under the table. A range between whole numbers needs no step whatever the parameter's type — `0.0..1.0` over a `Real` steps by one — while a range with a fractional endpoint still needs `:<step>`. A range read as reals takes an Integer endpoint or step only where a Real holds it without rounding, and steps only where the reals tell its rows apart, so a `Real` parameter swept from 2⁶⁰ to 2⁶⁰+3 is refused rather than collapsed onto one row.

- **A transition written without a source (`accept … then`, `if … then`, `then`) now leaves the state declared before it in the same body, as SysML v2 §7.18.3 specifies and the OMG pilot implements.** It used to take the state whose body contained it as the source, and to refuse the form at a state machine's top level at instantiation, so a nested `accept after 5 [SI::s] then decelerating;` fired from every substate and re-armed its timer forever, and the top-level form failed with `sourceless transition at top level has no containing state`. The shorthand is now a member of the body that declares the state it leaves, written after that state (the pinned pilot rejects it inside the state's own body), several in a row all leave the same state, and one written first in its body or after a member that is not a state — an attribute, a `do` action, a succession, a `choice` or `join` pseudostate, an orthogonal region — is reported by validation with the member named; a pseudostate is left by `transition first <pseudostate> … then …;` only.
- **The guarded entry transition (`entry; if cold then heating; if not cold then idle;`, SysML v2 §7.18.3 `EntryTransitionMember`) now chooses the state a body starts in.** The transitions out of a body's entry action are lowered in declaration order and tried in that order each time the body is entered — when the machine starts and whenever a transition enters the composite state whose body it is — after the entry action itself has run; the first whose guard holds is entered, an unguarded `then s;` among them is taken when reached, and when none holds the machine reports `no entry transition holds` rather than starting somewhere. A body's entry transitions are tried inside a composite state, an orthogonal region and an exhibited state alike, and a transition into a composite state now starts that state's body by its entry transition rather than leaving it without an active substate. A state usage typed by a definition that writes entry transitions of its own starts by those alone, replacing the inherited ones as its own entry behavior does, instead of starting where the definition's came first. An entry transition written with a trigger or an effect, or reaching something other than a state, is reported by validation and by lowering, as the OMG pilot rejects those shapes.
- **The state rendering now draws a body's entry transitions and marks only an unconditional start as `initial`.** For `entry; if cold then heating; if not cold then idle;` the text, Mermaid and LSP renderings labelled both `heating` and `idle` `(initial)` and the diagram drew a start arrow into each, though the machine enters only the first alternative whose guard holds, and the guards appeared nowhere. Each body that says where it starts — the machine, a composite state, an orthogonal region — now has a `start` node whose edges are its entry transitions in the order the guards are tried in, carrying the guard as `[cold]` like any transition edge (`[*] --> heating : [cold]` in Mermaid); the `initial` detail is kept for the target of a body's first entry transition when that one is unguarded, so a state reached only through a guarded alternative is no longer `initial`.
- **A state machine's own `entry`, `do` and `exit` behaviors now run for every machine.** They were skipped unless the machine had orthogonal regions of its own, so `state def M { entry assign started := true; then idle; … }` never assigned; the entry behavior now runs before the start state is chosen, and the exit behavior once a transition to `done` has completed the machine.

- **A verification case's objective checks the case's subject, not its verdict.** The library binds it so — `VerificationCases::VerificationCase::obj` redefines `Cases::Case::obj` with `subject subj = VerificationCase::subj` — but the runtime applied the redefined objective's `default Case::result` to every case kind, so an objective typed by a requirement with a typed subject (`subject lander : Lander`) was `undecided` with `type mismatch: VerdictKind::pass (enumeration literal) is not a Lander`, and a sweep over such a case printed `undecided` in every row. An unbound objective subject now holds the value the library states for it, read through the redefinition chain: an analysis case's objective still defaults to the result, a verification case's evaluates the requirement against the verification subject whatever the requirement names it, and the objective is decided on `-analysis`, `-requirement`, `-satisfy`, the REPL and gRPC alike. A requirement subject the verification subject cannot be is `undecided` naming both types, and a usage restating the library's `=` binding is still refused.

### Performance

- **Loading a model no longer merges the library's visible member set once per declaration.** The inherited-name conflict rule looks each name up in the memoized member maps of a declaration's library bases and passed-through types instead of copying them into a fresh map per part, attribute, action and state; its diagnostics are unchanged. The OOSEM method rule memoizes a type's classification, so an attribute type shared by many features is conformance-checked once. Loading and validating a 4 000-element model is 15% faster and allocates a third fewer bytes than before; `sysml -validate` on 3 000–12 000-element models is now at or ahead of release 0.4.2. `docs/project/performance-release-0.6-vs-0.4.2.md` records the comparison, the remaining costs of the validation rules added since 0.4.2, and how to repeat it. The Apollo 11 load figure on the landing page and in `docs/internals/performance.md` is re-measured at 0.43 s: the earlier 0.37 s was taken while the model's three calculation-arity findings were still errors, before the higher validation tiers ran.

- **A long action run no longer grows the memory of a race-instrumented binary step by step.** Each step of a token used to run on a coroutine of its own so that a breakpoint or a wait on the clock inside it could pause the token, and Go's race detector keeps a coroutine's state after it ends, so a run of a million steps under `go test -race` took gigabytes and could be killed for memory. The steps of a run now share one coroutine, which only a paused step keeps for as long as it is paused; breakpoints, clock waits, `Release` and the deadlock reported for an abandoned pause behave as before. The step-budget tests of the runtime package now peak at a few hundred megabytes under the race detector rather than several gigabytes.

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
  recorded commit ([the report](internal/translate/interop/flexo/testdata/identity_apply_expected.txt)).
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
