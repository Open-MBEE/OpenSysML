# Graphical notation census

How much of the SysML v2 **graphical** notation the view renderers draw. The textual grammar has
its [grammar coverage](grammar-coverage.md) census and the v1→v2 mapping its
[transformation census](sysml-v1-transformation-census.md); this census does the same for clause
§8.2.3 "Graphical Notation" of the pinned SysML v2 specification, production by production, with
a committed baseline and a CI gate that fails when the measurement moves.

<!-- census:begin source -->
Measured over `build/spec-docs/sysml.txt`, rendered with `pdftotext -layout` from the SysML v2 specification PDF of pilot release `2026-09` (SysML-v2-Release commit `b7a6b77000c9136f8f24822fc9d2cb237c76267f`), clause §8.2.3.
<!-- census:end source -->

## Method

### The denominator

`scripts/download-spec-examples.sh` fetches the pinned OMG PDFs and renders them with
`pdftotext -layout` into `build/spec-docs/sysml.txt`. Clause §8.2.3 runs from the heading line
`8.2.3.1 Graphical Notation Overview` to `8.3 Abstract Syntax`; its graphical BNF (the conventions
of Table 30) writes one production statement per line, `name =` for a complete production and
`name =|` for a partial one that adds alternatives to a production stated elsewhere. The extractor
(`tools/census/notation`, `Extract`) reads exactly that: a statement is a lowercase hyphenated
name at the start of a line followed by ` =` or ` =|`, under the most recent `8.2.3.n` heading.
It is deterministic and strict: all 27 sub-clause headings must be found, in order, every
sub-clause after the overview must state at least one production, and a production the verdict
table does not know (or a verdict for a production the clause no longer states) is an error. A
text rendered from another pin fails rather than silently moving the denominator.

Two `pdftotext` artifacts matter. A page starts with a form feed, which may precede a heading or a
statement: the §8.2.3.4 "Annotations Graphical Notation" heading and several statements start a
page, so the extractor accepts an optional form feed before both. And the layout mode interleaves
page headers and footers into long right-hand sides; the census reads only the left-hand `name =`
of each line, so those lines never count as statements.

One census row is one **distinct production name**. The same name is stated more than once where
a later sub-clause extends a list of alternatives with `=|` (`compartment`, `usage-node`,
`definition-node`, …); a production is *partial* when no statement defines it completely. A naive
count of statement lines, without the form feed, finds 503 names; the complete extraction records
every statement and every name, and the summary below states both figures.

### The oracle and the verdicts

The oracle of a production is **its figure** in the specification: the box, compartment, line,
glyph or label the figure shows. Each production is classified as a *node*, a *compartment*, an
*edge*, a *label* (text written on a drawn element), a *view* (a whole diagram kind) or *textual*
(a production that only reuses the textual notation of §8.2.2), and given one verdict:

- **drawn** — a renderer draws it as the figure shows it. The row names the function in
  `internal/ir/view` (or the VS Code canvas) that draws it, the forms it holds in (canvas, DOT,
  PlantUML, Mermaid, D2, draw.io, text, SVG) and the test or golden that pins the drawing.
- **approximate** — drawn, but the medium cannot show the glyph or the writer departs from the
  figure in a stated way (a keyword spelled differently, a compartment headed differently, a
  symbol the form lacks). The reason says what is missing.
- **undrawn** — nothing draws it.
- **not applicable** — a textual production that reuses §8.2.2 and is rendered as text, or a
  list of alternatives (`element-node = part | item | …`) that is adjudicated through the
  alternatives it names.

Evidence is code and tests, not names: every drawn or approximate row cites an implementation
function that `-check` resolves against the Go (or TypeScript) source, and a test function or
golden file that must exist. The verdicts were taken by reading the writers and their goldens and
by rendering fixtures that declare every definition and usage kind, both as nodes and as the
members of a positioned node (whose members become compartments). An unproven row is undrawn.

### The per-form rule for edges

The 31 relationship productions are adjudicated per form once, in the
[notation table](view-rendering-forms.md#notation-table) of the rendering-forms page, which
`TestNotationTableMatchesTheWriters` holds the writers to. This census reads that table rather
than restating it: a production whose row is *conformant*, *differs from pilot* (the specification
wins over the Pilot) or states its glyph is **drawn**; a row marked *approximate* is
**approximate** with the table's note as its reason.

### Running it

```sh
./scripts/download-spec-examples.sh                    # renders build/spec-docs/sysml.txt
go run -C tools ./cmd/notation-census -measure          # rewrites the baseline, this page's blocks and build/graphical-notation-census/rows.md
go run -C tools ./cmd/notation-census -check            # the CI gate
```

`-measure` writes `docs/project/graphical-notation-census-baseline.json` (the source pin, the
totals, the per-clause and per-category counts, and every row that is not drawn with its reason
and work package) and the generated blocks of this page; the full table with every row's evidence
goes to `build/graphical-notation-census/rows.md`, which is not committed. `-check` verifies the
baseline is consistent, records the pinned release, matches this page's blocks, that every
citation resolves, and — where `build/spec-docs/sysml.txt` is present — that a fresh measurement
equals the baseline. With `-require-text` or `OPENSYSML_REQUIRE_SPEC_EXAMPLES` set, an absent text
fails instead of skipping, so the CI job that provisions the text cannot pass without comparing.

## Totals

<!-- census:begin summary -->
620 production statements (97 of them partial `=|` statements) name 523 distinct productions: **293 drawn**, **34 approximate**, **24 undrawn**, 172 not applicable.

| Category | Productions |
| --- | --- |
| node | 159 |
| compartment | 147 |
| edge | 59 |
| label | 24 |
| view | 10 |
| textual | 124 |
<!-- census:end summary -->

### By clause

<!-- census:begin clauses -->
| Clause | Statements | Productions | Drawn | Approximate | Undrawn | Not applicable |
| --- | --- | --- | --- | --- | --- | --- |
| §8.2.3.1 Graphical Notation Overview | 0 | 0 | 0 | 0 | 0 | 0 |
| §8.2.3.2 Elements and Relationships Graphical Notation | 9 | 9 | 3 | 1 | 1 | 4 |
| §8.2.3.3 Dependencies Graphical Notation | 9 | 8 | 6 | 0 | 0 | 2 |
| §8.2.3.4 Annotations Graphical Notation | 12 | 11 | 7 | 0 | 0 | 4 |
| §8.2.3.5 Namespaces and Packages Graphical Notation | 25 | 24 | 6 | 3 | 6 | 9 |
| §8.2.3.6 Definition and Usage Graphical Notation | 28 | 24 | 7 | 7 | 1 | 9 |
| §8.2.3.7 Attributes Graphical Notation | 10 | 9 | 5 | 0 | 0 | 4 |
| §8.2.3.8 Enumerations Graphical Notation | 9 | 6 | 5 | 0 | 0 | 1 |
| §8.2.3.9 Occurrences Graphical Notation | 48 | 43 | 24 | 2 | 1 | 16 |
| §8.2.3.10 Items Graphical Notation | 12 | 9 | 6 | 0 | 0 | 3 |
| §8.2.3.11 Parts Graphical Notation | 18 | 13 | 7 | 0 | 2 | 4 |
| §8.2.3.12 Ports Graphical Notation | 20 | 16 | 6 | 6 | 2 | 2 |
| §8.2.3.13 Connections Graphical Notation | 35 | 31 | 19 | 0 | 2 | 10 |
| §8.2.3.14 Interfaces Graphical Notation | 17 | 12 | 7 | 1 | 0 | 4 |
| §8.2.3.15 Allocations Graphical Notation | 14 | 9 | 6 | 0 | 0 | 3 |
| §8.2.3.16 Flows Graphical Notation | 30 | 22 | 16 | 2 | 0 | 4 |
| §8.2.3.17 Actions Graphical Notation | 72 | 68 | 44 | 3 | 3 | 18 |
| §8.2.3.18 States Graphical Notation | 45 | 41 | 25 | 2 | 0 | 14 |
| §8.2.3.19 Calculations Graphical Notation | 14 | 9 | 6 | 0 | 0 | 3 |
| §8.2.3.20 Constraints Graphical Notation | 24 | 19 | 13 | 0 | 0 | 6 |
| §8.2.3.21 Requirements Graphical Notation | 63 | 57 | 34 | 2 | 1 | 20 |
| §8.2.3.22 Cases Graphical Notation | 4 | 3 | 0 | 1 | 0 | 2 |
| §8.2.3.23 Analysis Cases Graphical Notation | 12 | 7 | 5 | 0 | 0 | 2 |
| §8.2.3.24 Verification Cases Graphical Notation | 22 | 16 | 6 | 2 | 2 | 6 |
| §8.2.3.25 Use Cases Graphical Notation | 22 | 16 | 9 | 1 | 0 | 6 |
| §8.2.3.26 Views and Viewpoints Graphical Notation | 38 | 34 | 19 | 1 | 2 | 12 |
| §8.2.3.27 Metadata Graphical Notation | 8 | 7 | 2 | 0 | 1 | 4 |
| **Total** | 620 | 523 | 293 | 34 | 24 | 172 |
<!-- census:end clauses -->

## Gaps

The undrawn and approximate productions, grouped by clause. Each row is assigned to a work
package a follow-up can take one at a time (see below).

<!-- census:begin gaps -->
#### §8.2.3.2 Elements and Relationships Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `element-inside-textual-compartment` | node | approximate | E | a row a folded edge or note ends at is greyed in DOT and PlantUML; Mermaid's markdown labels carry no colour, so the row is italic there, and D2, draw.io and the text form list it plain |
| `general-compartment` | compartment | undrawn | F | a nested general view inside a node's compartment is not drawn; nested nodes are drawn as a tree |

#### §8.2.3.5 Namespaces and Packages Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `imported-package-with-name-in-tab` | node | undrawn | F | an imported package is drawn as an ordinary package node at the end of an import edge |
| `imported-package-with-name-inside` | node | undrawn | F | an imported package is drawn as an ordinary package node at the end of an import edge |
| `members-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `owned-membership` | edge | approximate | F | view-rendering-forms.md § Notation table: Circle-plus at the namespace: Graphviz has no such arrow, so a `+` tail label is set on its hollow circle; D2 puts a `+` arrowhead label beside its circle; Mermaid draws a plain line, a tree's nesting showing the ownership. A mixed rendering draws it from an owner to every nested member that is no feature — a nested package or definition — as above. |
| `package-node` | node | approximate | F | a «package» box without the tab; PlantUML's `package` folder is the nearest form |
| `package-with-name-in-tab` | node | undrawn | F | the name-in-tab form is not drawn; packages are keyword boxes |
| `package-with-name-inside` | node | approximate | F | a «package» box without the tab; PlantUML's `package` folder is the nearest form |
| `packages-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `relationships-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |

#### §8.2.3.6 Definition and Usage Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `composite-feature-membership` | edge | approximate | F | view-rendering-forms.md § Notation table: Filled diamond at the owner; Mermaid has no diamond and leads the edge text with `◆`. A case or mixed diagram writes the DOT attributes as `arrowtail=diamond, dir=back`. A mixed rendering draws it from an owner to every composite usage nested in its node — a part, action or state usage, a view usage — where PlantUML and the text form show it and DOT and Mermaid leave it to the nesting ([case and mixed diagrams](view-rendering-forms.md#case-and-mixed-diagrams)). |
| `definition` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid hollow triangle at the definition with two dots beside its base, one each side of the line, as PlantUML's `--:\|>` sets them: no Graphviz arrowhead sets a dot beside the line, so the DOT edge is the triangle under a class and the dots are drawn into the SVG of every drawing this project has Graphviz make ([the definition glyph](view-rendering-forms.md#the-definition-glyph)), D2 and draw.io have no dots and draw the plain triangle, Mermaid writes `typing` as the edge text. |
| `features-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all (a KerML `feature` member never folds into a SysML node's compartments) |
| `noncomposite-feature-membership` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow diamond at the owner; Mermaid leads the edge text with `◇`. A mixed rendering draws it to every referential usage nested in its owner's node — a `ref` part, an attribute, a port — as above. |
| `redefinition` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow triangle with a bar across the line at its base, as Graphviz's `onormaltee` sets it; PlantUML sets the bar a little behind the triangle, D2 and draw.io have no bar and draw the plain triangle, Mermaid writes `redefinition` as the edge text. |
| `reference-subsetting` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow triangle with four dots behind its base, from a usage to the one it `::>` references, labelled `references` as `subsets` and `redefines` are: Graphviz sets one dot behind the triangle, PlantUML, D2 and draw.io draw the plain triangle, Mermaid has none; the canvas draws the four dots. The Pilot writes `--\|>`. |
| `subclassification` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid hollow triangle; Mermaid has no triangle head and names the kind as the edge text instead. |
| `subsetting` | edge | approximate | F | view-rendering-forms.md § Notation table: The same hollow triangle, no text; Mermaid writes `subsetting` as the edge text, having no triangle. |

#### §8.2.3.9 Occurrences Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `portion-relationship` | edge | approximate | D | view-rendering-forms.md § Notation table: Solid line with a filled three-quarter circle at the occurrence, from the occurrence to a `timeslice` or `snapshot` it owns, in the definition graph and the tree: Graphviz and D2 draw a filled circle, PlantUML a filled square, Mermaid a plain line with `◕` leading the text; the canvas draws the three-quarter circle, which no Graphviz, PlantUML, D2 or draw.io arrowhead is. The Pilot writes `*--`. |
| `sequence-compartment` | compartment | undrawn | D | holds a sequence view; no box form nests a diagram, and the canvas lays a sequence out as a diagram of its own |
| `sq-succession` | edge | approximate | D | dashed with an open head in PlantUML (`-->>`), Mermaid (`--)`) and the canvas; D2's dashed arrow carries a filled head |

#### §8.2.3.11 Parts Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `directed-features-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `interconnection-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |

#### §8.2.3.12 Ports Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `pdh` | label | undrawn | F | no form draws the direction arrow inside a port or parameter square |
| `pdv` | label | undrawn | F | no form draws the direction arrow inside a port or parameter square |
| `port-b` | node | approximate | F | the port square is placed on the owner's boundary but the direction arrow (pdh/pdv) inside it is not drawn |
| `port-l` | node | approximate | F | the port square is placed on the owner's boundary but the direction arrow (pdh/pdv) inside it is not drawn |
| `port-r` | node | approximate | F | the port square is placed on the owner's boundary but the direction arrow (pdh/pdv) inside it is not drawn |
| `port-t` | node | approximate | F | the port square is placed on the owner's boundary but the direction arrow (pdh/pdv) inside it is not drawn |
| `proxy-h` | node | approximate | F | a conjugated port is written as `~Type` text (view.go typingTexts), not as the proxy glyph |
| `proxy-v` | node | approximate | F | a conjugated port is written as `~Type` text (view.go typingTexts), not as the proxy glyph |

#### §8.2.3.13 Connections Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `connection-definition-elaboration` | view | undrawn | F | a connection's own interconnection view nested in its node is not drawn |
| `connection-usage-elaboration` | view | undrawn | F | a connection's own interconnection view nested in its node is not drawn |

#### §8.2.3.14 Interfaces Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `interface-connection` | edge | approximate | F | drawn as a connection edge between the ports; the «interface» keyword is not written |

#### §8.2.3.16 Flows Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `flow` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid with a filled triangle head; PlantUML's `-->` head is a filled vee, not a triangle (as the Pilot writes it). |
| `message` | edge | approximate | D | solid with an open head in PlantUML (`->>`), Mermaid (`-)`) and the canvas; D2 has only a filled head |

#### §8.2.3.17 Actions Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `accept-action-node` | node | approximate | B | a rounded box with the keyword compartment in every form; the send and accept corner glyphs are drawn in DOT, draw.io and the canvas, written as ▷ and ◁ in PlantUML and Mermaid, and not drawn in D2 |
| `perform-actions-swimlanes` | compartment | undrawn | B | swimlanes are not drawn |
| `performed-by-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `send-action-node` | node | approximate | B | a rounded box with the keyword compartment in every form; the send and accept corner glyphs are drawn in DOT, draw.io and the canvas, written as ▷ and ◁ in PlantUML and Mermaid, and not drawn in D2 |
| `swimlane` | compartment | undrawn | B | swimlanes are not drawn |
| `terminate-node` | node | approximate | B | the figure's X in a circle in DOT, Mermaid (cross-circ), draw.io (umlDestroy) and the canvas; PlantUML has no terminate pseudostate and D2 no crossed circle, both draw a plain end |

#### §8.2.3.18 States Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `st-succession` | edge | approximate | B | view-rendering-forms.md § Notation table: Solid with an open head and the transition label (figures of §8.2.3.18), as DOT, PlantUML, draw.io and the canvas draw it; Mermaid and D2 have only a filled head. |
| `transition` | edge | approximate | B | view-rendering-forms.md § Notation table: Solid with an open head and the transition label (figures of §8.2.3.18), as DOT, PlantUML, draw.io and the canvas draw it; Mermaid and D2 have only a filled head. |

#### §8.2.3.21 Requirements Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `actor` | node | approximate | F | a box headed «actor» in DOT, Mermaid and the text form; PlantUML (`actor`, plantuml.go writeCaseMixedNode) and draw.io (plainStyle umlActor) draw the stick figure |
| `concern-stakeholder-link` | edge | undrawn | F | the line from a concern to its stakeholder is not drawn |
| `satisfy-requirements-compartment` | compartment | approximate | F | headed `satisfies` and listing the satisfy requirement usages as rows; the figure heads it `satisfy requirements` (its `satify` is a typo) over a text block |

#### §8.2.3.22 Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `objective-compartment` | compartment | approximate | F | headed `objective` in a positioned tree, the row the objective's name and type; in a case rendering the objective is a note anchored to the case (case.go objectiveNode) |

#### §8.2.3.24 Verification Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `verification-methods-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `verifies-compartment` | compartment | undrawn | F | a verify requirement usage is a member of the case's objective and lists under its row, not in a compartment of the case |
| `verify-requirement-name-compartment` | compartment | approximate | F | a «verify requirement» node in a tree rendering; in a case rendering the objective that owns it is drawn as a note (case.go objectiveNode) |
| `verify-requirement-node` | node | approximate | F | a «verify requirement» node in a tree rendering; in a case rendering the objective that owns it is drawn as a note (case.go objectiveNode) |

#### §8.2.3.25 Use Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `includes-compartment` | compartment | approximate | F | the included use cases list under `include use cases` (the clause's other compartment for them), not under `includes` |

#### §8.2.3.26 Views and Viewpoints Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `exposes-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `filters-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `rendering-compartment` | compartment | approximate | F | headed `rendering` for a rendering usage listed in a node; a view's own `render` members describe its picture and are not shown (tree.go contentKind) |

#### §8.2.3.27 Metadata Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `metadata-feature-annotation-node` | node | undrawn | F | a metadata usage annotating an element is not drawn as a «metadata» note |
<!-- census:end gaps -->

### Work packages

<!-- census:begin packages -->
| Package | Scope | Undrawn | Approximate | Productions |
| --- | --- | --- | --- | --- |
| A | §8.2.3.26 view frame and §8.2.3.4 annotation nodes | 0 | 0 |  |
| B | §8.2.3.17–.18 action and state node forms | 3 | 5 | `accept-action-node`, `perform-actions-swimlanes`, `performed-by-compartment`, `send-action-node`, `swimlane`, `terminate-node`, `st-succession`, `transition` |
| C | §8.2.3.3/.13 n-ary dots and §8.2.3.16 flow-on-connection | 0 | 0 |  |
| D | §8.2.3.9 sequence glyphs | 1 | 3 | `portion-relationship`, `sequence-compartment`, `sq-succession`, `message` |
| E | §8.2.3.2 textual-compartment greying | 0 | 1 | `element-inside-textual-compartment` |
| F | everything else | 20 | 25 | `general-compartment`, `imported-package-with-name-in-tab`, `imported-package-with-name-inside`, `members-compartment`, `owned-membership`, `package-node`, `package-with-name-in-tab`, `package-with-name-inside`, `packages-compartment`, `relationships-compartment`, `composite-feature-membership`, `definition`, `features-compartment`, `noncomposite-feature-membership`, `redefinition`, `reference-subsetting`, `subclassification`, `subsetting`, `directed-features-compartment`, `interconnection-compartment`, `pdh`, `pdv`, `port-b`, `port-l`, `port-r`, `port-t`, `proxy-h`, `proxy-v`, `connection-definition-elaboration`, `connection-usage-elaboration`, `interface-connection`, `flow`, `actor`, `concern-stakeholder-link`, `satisfy-requirements-compartment`, `objective-compartment`, `verification-methods-compartment`, `verifies-compartment`, `verify-requirement-name-compartment`, `verify-requirement-node`, `includes-compartment`, `exposes-compartment`, `filters-compartment`, `rendering-compartment`, `metadata-feature-annotation-node` |
<!-- census:end packages -->

## Decisions

Calls the specification's figures do not settle on their own, with the evidence behind them:

- **A connector usage is a node in a tree and a line in an interconnection view.** `connection`,
  `interface`, `allocation` and `flow-node` describe the usage's own box; the tree form draws a
  connection, interface, allocation or flow usage as a keyword box like any usage, while the
  interconnection form draws it as the line between its ends. The box productions are therefore
  *drawn*, and the edge productions (`connection-graphical`, `interface-connection`,
  `allocate-relationship`, `flow`) carry the verdicts for the lines.
- **Keyword boxes are drawn per kind, with the figure's keyword.** `label.go` `keyword` writes
  `«kind»` for every definition and usage kind, so `item-def`, `concern`, `viewpoint`, … are
  *drawn*; where the figure writes more than the textual keyword, `view.go` `figureKeyword` and
  `declKind` write it in full — `«satisfy requirement»`, `«frame concern»`, `«event occurrence»`,
  `«#command part def»` — so those nodes and name compartments are *drawn* too.
  `verify-requirement-node` is *approximate*: a verify usage is drawn as a `«verify requirement»`
  box in a tree rendering, while in a case rendering the objective that owns it is a note.
- **Compartments follow `compartmentName`.** Every usage kind under a positioned node gets a
  compartment headed by the plural of its keyword, which matches the figure for most clauses
  (`parts`, `perform actions`, `exhibit states`, `require constraints`, …). Where the plural is
  not the figure's heading the compartment is *approximate*: `enumerations` for `enums`,
  `calculations` for `calcs`, `renderings` for `rendering`, `subjects` for `subject`, and
  `satisfys` for `satisfies` (the specification's own `satify requirements` heading is a typo of
  `satisfy requirements`). Compartments no renderer heads (`members`, `features`, `variants`,
  `ends`, `flows`, `performed by`, `state actions`, `result`, `exposes`, `filters`, …) are
  *undrawn*.
- **The port and parameter squares are approximate.** `port-l/r/t/b` and `param-l/r/t/b` place
  a square on the owner's boundary and label it, but the direction arrow inside the square
  (`pdh`, `pdv`) is drawn by no form, so the squares are *approximate* and the arrows *undrawn*.
- **`«parallel»` is not written.** A state's name compartment admits an optional `«parallel»`
  marker that no renderer writes, so `state-def-name-compartment` and `state-name-compartment`
  are *approximate* while the state nodes themselves are *drawn*.
- **Control-flow node forms.** `send`, `accept`, `assign`, `if`, `while` and `for` action nodes
  are drawn as a plain `«action»` box (`behavior.go` `actionNodeKind`), so the nodes are
  *approximate* and their keyword compartments and condition/body compartments *undrawn*;
  `terminate-node` is *approximate* because only draw.io draws the figure's X-in-a-circle.
- **State entry/do/exit actions** are written as text lines inside the state box, not as the
  figure's labelled compartments: *approximate*; the separate entry/do/exit action nodes are
  *undrawn*.
- **The view frame** (`root-view`, `framed-view`, `view-frame-name-compartment`) is drawn only by
  the Cameo-style DOT frame (`dot.go` `openFrame`), whose header does not write the `«view»`
  keyword: *approximate*, DOT only; the info compartments are *undrawn*.
- **Annotations** (comments, documentation, textual representations) are filtered out of trees by
  `tree.go` `containedKind`; only a `DiagramLayout::Note` and a case objective's documentation
  are drawn, so the annotation nodes are *undrawn* and `annotation-link` *approximate*.
- **Labels are drawn when their text is written on a drawn element** (end role names,
  multiplicities, edge labels, flow payloads, message labels, triggers and guards); a label of an
  undrawn element (`succession-label`, `proxy-label`, `cdot-label`) is *undrawn*.
- **Figures counted twice.** A production stated in more than one sub-clause is one row under the
  clause of its first statement; the baseline records how many statements name it.

## Not applicable

The productions excluded from the drawn/undrawn question, grouped by reason:

<!-- census:begin exclusions -->
- **a list of alternatives; adjudicated through the alternatives it names** (46): `compartment` (§8.2.3.2), `element` (§8.2.3.2), `element-node` (§8.2.3.2), `general-element` (§8.2.3.2), `dependencies-and-annotations-element` (§8.2.3.3), `dependencies-element` (§8.2.3.3), `annotated-element` (§8.2.3.4), `annotation-node` (§8.2.3.4), `general-node` (§8.2.3.5), `general-relationship` (§8.2.3.5), `namespace-node` (§8.2.3.5), `type-node` (§8.2.3.6), `type-relationship` (§8.2.3.6), `definition-node` (§8.2.3.7), `usage-node` (§8.2.3.7), `eventer` (§8.2.3.9), `sq-graphical-element` (§8.2.3.9), `sq-graphical-node` (§8.2.3.9), `sq-graphical-relationship` (§8.2.3.9), `timeslice-or-snapshot-node` (§8.2.3.9), `interconnection-element` (§8.2.3.10), `connection-end` (§8.2.3.13), `connection-relationship` (§8.2.3.13), `usage-edge` (§8.2.3.13), `allocation-node` (§8.2.3.15), `flow-end-node` (§8.2.3.16), `msg-end-node` (§8.2.3.16), `action-flow-element` (§8.2.3.17), `action-flow-node` (§8.2.3.17), `action-flow-relationship` (§8.2.3.17), `parameter` (§8.2.3.17), `performer-node` (§8.2.3.17), `exhibitor` (§8.2.3.18), `state-source` (§8.2.3.18), `state-transition-element` (§8.2.3.18), `state-transition-node` (§8.2.3.18), `assertor` (§8.2.3.20), `assumer` (§8.2.3.20), `distinguished-parameter` (§8.2.3.21), `framer` (§8.2.3.21), `require-constraint-element` (§8.2.3.21), `requirer` (§8.2.3.21), `satisfier` (§8.2.3.21), `subject-actors-stakeholders-node` (§8.2.3.21), `expose-relationship` (§8.2.3.26), `frameless-view` (§8.2.3.26)
- **textual notation of §8.2.2 reused as the text of a drawn element** (124): `language-string` (§8.2.3.4), `text-block` (§8.2.3.4), `members-compartment-contents` (§8.2.3.5), `members-compartment-element` (§8.2.3.5), `packages-compartment-contents` (§8.2.3.5), `packages-compartment-element` (§8.2.3.5), `relationships-compartment-contents` (§8.2.3.5), `relationships-compartment-element` (§8.2.3.5), `definition-name-with-alias` (§8.2.3.6), `el-prefix` (§8.2.3.6), `features-compartment-contents` (§8.2.3.6), `features-compartment-element` (§8.2.3.6), `usage-cp` (§8.2.3.6), `usage-name-with-alias` (§8.2.3.6), `variants-compartment-contents` (§8.2.3.6), `attributes-compartment-contents` (§8.2.3.7), `attributes-compartment-element` (§8.2.3.7), `enums-compartment-element` (§8.2.3.8), `individuals-compartment-contents` (§8.2.3.9), `individuals-compartment-element` (§8.2.3.9), `occurrences-compartment-contents` (§8.2.3.9), `occurrences-compartment-element` (§8.2.3.9), `snapshots-compartment-contents` (§8.2.3.9), `snapshots-compartment-element` (§8.2.3.9), `sq-port-label` (§8.2.3.9), `timeslices-compartment-contents` (§8.2.3.9), `timeslices-compartment-element` (§8.2.3.9), `items-compartment-contents` (§8.2.3.10), `items-compartment-element` (§8.2.3.10), `directed-features-compartment-contents` (§8.2.3.11), `directed-features-compartment-element` (§8.2.3.11), `parts-compartment-contents` (§8.2.3.11), `parts-compartment-element` (§8.2.3.11), `ports-compartment-contents` (§8.2.3.12), `ports-compartment-element` (§8.2.3.12), `a-direction` (§8.2.3.13), `a-property` (§8.2.3.13), `a-redefinition` (§8.2.3.13), `a-subsetting` (§8.2.3.13), `c-adornment` (§8.2.3.13), `connections-compartment-contents` (§8.2.3.13), `connections-compartment-element` (§8.2.3.13), `ends-compartment-contents` (§8.2.3.14), `ends-compartment-element` (§8.2.3.14), `interfaces-compartment-contents` (§8.2.3.14), `interfaces-compartment-element` (§8.2.3.14), `allocations-compartment-contents` (§8.2.3.15), `allocations-compartment-element` (§8.2.3.15), `flows-compartment-contents` (§8.2.3.16), `flows-compartment-element` (§8.2.3.16), `accept-action-expression` (§8.2.3.17), `action-body` (§8.2.3.17), `action-body-textual` (§8.2.3.17), `actions-compartment-contents` (§8.2.3.17), `actions-compartment-element` (§8.2.3.17), `condition-expression` (§8.2.3.17), `iteration-expression` (§8.2.3.17), `parameters-compartment-contents` (§8.2.3.17), `parameters-compartment-element` (§8.2.3.17), `perform-actions-compartment-contents` (§8.2.3.17), `perform-actions-compartment-element` (§8.2.3.17), `performed-by-compartment-contents` (§8.2.3.17), `send-action-expression` (§8.2.3.17), `exhibit-states-compartment-contents` (§8.2.3.18), `exhibit-states-compartment-element-compartment` (§8.2.3.18), `state-actions-compartment-contents` (§8.2.3.18), `state-actions-compartment-element` (§8.2.3.18), `state-subaction-body` (§8.2.3.18), `state-subaction-body-textual` (§8.2.3.18), `state-subaction-declaration` (§8.2.3.18), `states-compartment-contents` (§8.2.3.18), `states-compartment-element` (§8.2.3.18), `successions-compartment-contents` (§8.2.3.18), `calcs-compartment-contents` (§8.2.3.19), `calcs-compartment-element` (§8.2.3.19), `result-compartment-contents` (§8.2.3.19), `assert-constraints-compartment-contents` (§8.2.3.20), `assert-constraints-compartment-element` (§8.2.3.20), `constraints-compartment-contents` (§8.2.3.20), `constraints-usage-compartment-element` (§8.2.3.20), `actors-compartment-contents` (§8.2.3.21), `actors-compartment-element` (§8.2.3.21), `assume-constraints-compartment-contents` (§8.2.3.21), `frames-compartment-contents` (§8.2.3.21), `frames-compartment-element` (§8.2.3.21), `require-constraints-compartment-contents` (§8.2.3.21), `requirements-compartment-contents` (§8.2.3.21), `requirements-compartment-element` (§8.2.3.21), `satisfies-compartment-contents` (§8.2.3.21), `satisfy-requirements-compartment-contents` (§8.2.3.21), `stakeholders-compartment-contents` (§8.2.3.21), `stakeholders-compartment-element` (§8.2.3.21), `subject-compartment-contents` (§8.2.3.21), `subject-compartment-element` (§8.2.3.21), `objective-compartment-contents` (§8.2.3.22), `objective-compartment-element` (§8.2.3.22), `analyses-compartment-contents` (§8.2.3.23), `analyses-compartment-element` (§8.2.3.23), `verification-methods-compartment-contents` (§8.2.3.24), `verification-methods-compartment-element` (§8.2.3.24), `verifications-compartment-contents` (§8.2.3.24), `verifications-compartment-element` (§8.2.3.24), `verifies-compartment-contents` (§8.2.3.24), `verifies-compartment-element` (§8.2.3.24), `include-use-cases-compartment-contents` (§8.2.3.25), `include-use-cases-compartment-element` (§8.2.3.25), `includes-compartment-contents` (§8.2.3.25), `includes-compartment-element` (§8.2.3.25), `use-cases-compartment-contents` (§8.2.3.25), `use-cases-compartment-element` (§8.2.3.25), `exposes-compartment-contents` (§8.2.3.26), `exposes-compartment-element` (§8.2.3.26), `filters-compartment-contents` (§8.2.3.26), `filters-compartment-element` (§8.2.3.26), `rendering-compartment-contents` (§8.2.3.26), `view-frame-info-contents` (§8.2.3.26), `viewpoints-compartment-contents` (§8.2.3.26), `viewpoints-compartment-element` (§8.2.3.26), `views-compartment-contents` (§8.2.3.26), `views-compartment-element` (§8.2.3.26), `expression-text` (§8.2.3.27), `metadata-feature-decl` (§8.2.3.27), `metadata-feature-name` (§8.2.3.27), `metadata-feature-name-value-list` (§8.2.3.27)
- **the textual grammar declares no event occurrence definition (`event` is a usage prefix), so nothing can be declared for the figure** (2): `event-occurrence-def` (§8.2.3.9), `event-occurrence-def-name-compartment` (§8.2.3.9)
<!-- census:end exclusions -->

## Related

- [View rendering forms](view-rendering-forms.md) — the forms, their notation table and the
  optional notation elements.
- [Spec compliance](spec-compliance.md) — the rows citing §8.2.3.
- [Grammar coverage](grammar-coverage.md) and the
  [transformation census](sysml-v1-transformation-census.md) — the sibling censuses.
