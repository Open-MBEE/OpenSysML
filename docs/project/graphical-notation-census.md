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
620 production statements (97 of them partial `=|` statements) name 523 distinct productions: **199 drawn**, **55 approximate**, **97 undrawn**, 172 not applicable.

| Category | Productions |
| --- | --- |
| node | 165 |
| compartment | 141 |
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
| §8.2.3.2 Elements and Relationships Graphical Notation | 9 | 9 | 2 | 0 | 3 | 4 |
| §8.2.3.3 Dependencies Graphical Notation | 9 | 8 | 1 | 0 | 5 | 2 |
| §8.2.3.4 Annotations Graphical Notation | 12 | 11 | 0 | 1 | 6 | 4 |
| §8.2.3.5 Namespaces and Packages Graphical Notation | 25 | 24 | 6 | 3 | 6 | 9 |
| §8.2.3.6 Definition and Usage Graphical Notation | 28 | 24 | 1 | 7 | 7 | 9 |
| §8.2.3.7 Attributes Graphical Notation | 10 | 9 | 5 | 0 | 0 | 4 |
| §8.2.3.8 Enumerations Graphical Notation | 9 | 6 | 4 | 1 | 0 | 1 |
| §8.2.3.9 Occurrences Graphical Notation | 48 | 43 | 17 | 3 | 7 | 16 |
| §8.2.3.10 Items Graphical Notation | 12 | 9 | 6 | 0 | 0 | 3 |
| §8.2.3.11 Parts Graphical Notation | 18 | 13 | 7 | 0 | 2 | 4 |
| §8.2.3.12 Ports Graphical Notation | 20 | 16 | 6 | 6 | 2 | 2 |
| §8.2.3.13 Connections Graphical Notation | 35 | 31 | 12 | 0 | 9 | 10 |
| §8.2.3.14 Interfaces Graphical Notation | 17 | 12 | 6 | 1 | 1 | 4 |
| §8.2.3.15 Allocations Graphical Notation | 14 | 9 | 6 | 0 | 0 | 3 |
| §8.2.3.16 Flows Graphical Notation | 30 | 22 | 8 | 1 | 9 | 4 |
| §8.2.3.17 Actions Graphical Notation | 72 | 68 | 21 | 11 | 18 | 18 |
| §8.2.3.18 States Graphical Notation | 45 | 41 | 11 | 7 | 9 | 14 |
| §8.2.3.19 Calculations Graphical Notation | 14 | 9 | 4 | 1 | 1 | 3 |
| §8.2.3.20 Constraints Graphical Notation | 24 | 19 | 13 | 0 | 0 | 6 |
| §8.2.3.21 Requirements Graphical Notation | 63 | 57 | 28 | 8 | 1 | 20 |
| §8.2.3.22 Cases Graphical Notation | 4 | 3 | 0 | 1 | 0 | 2 |
| §8.2.3.23 Analysis Cases Graphical Notation | 12 | 7 | 5 | 0 | 0 | 2 |
| §8.2.3.24 Verification Cases Graphical Notation | 22 | 16 | 6 | 0 | 4 | 6 |
| §8.2.3.25 Use Cases Graphical Notation | 22 | 16 | 9 | 0 | 1 | 6 |
| §8.2.3.26 Views and Viewpoints Graphical Notation | 38 | 34 | 13 | 4 | 5 | 12 |
| §8.2.3.27 Metadata Graphical Notation | 8 | 7 | 2 | 0 | 1 | 4 |
| **Total** | 620 | 523 | 199 | 55 | 97 | 172 |
<!-- census:end clauses -->

## Gaps

The undrawn and approximate productions, grouped by clause. Each row is assigned to a work
package a follow-up can take one at a time (see below).

<!-- census:begin gaps -->
#### §8.2.3.2 Elements and Relationships Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `element-inside-textual-compartment` | node | undrawn | E | the greyed element inside a textual compartment is not drawn; compartment rows are plain text |
| `ellipsis-at-lower-left-corner` | label | undrawn | E | hidden compartment rows are dropped without the '…' mark (compartments.go hidden rows) |
| `general-compartment` | compartment | undrawn | F | a nested general view inside a node's compartment is not drawn; nested nodes are drawn as a tree |

#### §8.2.3.3 Dependencies Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `n-ary-association-dot` | node | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-dependency` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-dependency-client-link` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-dependency-client-or-supplier-link` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-dependency-supplier-link` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |

#### §8.2.3.4 Annotations Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `annotation-link` | edge | approximate | A | only the anchor of a DiagramLayout::Note or a case objective is drawn (EdgeAnchor); comments are filtered out |
| `comment-node` | node | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |
| `comment-with-keyword` | node | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |
| `comment-without-keyword` | node | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |
| `documentation-compartment` | compartment | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |
| `documentation-node` | node | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |
| `textual-representation-node` | node | undrawn | A | annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn |

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
| `definition` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid hollow triangle at the definition with two dots at its base: Graphviz sets the dots in line behind the triangle, D2 has no dots and draws the plain triangle, Mermaid writes `typing` as the edge text. |
| `extended-def` | node | undrawn | F | a user-defined keyword (metadata-based definition) is not written; label.go keyword writes the element's kind |
| `extended-def-name-compartment` | node | undrawn | F | a user-defined keyword (metadata-based definition) is not written; label.go keyword writes the element's kind |
| `extended-usage` | node | undrawn | F | a user-defined keyword (metadata-based definition) is not written; label.go keyword writes the element's kind |
| `extended-usage-name-compartment` | node | undrawn | F | a user-defined keyword (metadata-based definition) is not written; label.go keyword writes the element's kind |
| `features-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `noncomposite-feature-membership` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow diamond at the owner; Mermaid leads the edge text with `◇`. A mixed rendering draws it to every referential usage nested in its owner's node — a `ref` part, an attribute, a port — as above. |
| `redefinition` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow triangle with a bar across the line: Graphviz sets the tee behind the triangle, D2 has no bar and draws the plain triangle, Mermaid writes `redefinition` as the edge text. |
| `reference-subsetting` | edge | approximate | F | view-rendering-forms.md § Notation table: Hollow triangle with four dots behind its base, from a usage to the one it `::>` references, labelled `references` as `subsets` and `redefines` are: Graphviz sets one dot behind the triangle, PlantUML and D2 draw the plain triangle, Mermaid has none; the canvas draws the four dots. The Pilot writes `--\|>`. |
| `subclassification` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid hollow triangle; Mermaid has no triangle head and names the kind as the edge text instead. |
| `subsetting` | edge | approximate | F | view-rendering-forms.md § Notation table: The same hollow triangle, no text; Mermaid writes `subsetting` as the edge text, having no triangle. |
| `variant-elementusages-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `variants-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |

#### §8.2.3.8 Enumerations Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `enums-compartment` | compartment | approximate | F | headed `enumerations`; the figure heads it `enums` |

#### §8.2.3.9 Occurrences Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `event-occurrence` | node | approximate | D | an event occurrence is drawn as a plain «occurrence» box |
| `event-occurrence-name-compartment` | compartment | approximate | D | written as «occurrence»; the `event` prefix is not part of the keyword |
| `individuals-compartment` | compartment | undrawn | D | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `portion-relationship` | edge | approximate | D | view-rendering-forms.md § Notation table: Solid line with a filled three-quarter circle at the occurrence, from the occurrence to a `timeslice` or `snapshot` it owns, in the definition graph and the tree: Graphviz and D2 draw a filled circle, PlantUML a filled square, Mermaid a plain line with `◕` leading the text; the canvas draws the three-quarter circle. The Pilot writes `*--`. |
| `proxy-label` | label | undrawn | D | labels the undrawn sq-proxy |
| `sequence-compartment` | compartment | undrawn | D | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `sq-port` | node | undrawn | D | a port is not given a lifeline; sequence.go lifelinesOf collects parts and occurrences |
| `sq-proxy` | node | undrawn | D | a feature-chain proxy lifeline is not drawn |
| `sq-succession` | edge | undrawn | D | documented as undrawn in view-rendering-forms.md § Notation table: the sequence form draws its own arrow |
| `succession-label` | label | undrawn | D | labels the undrawn sq-succession |

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
| `cdot-label` | label | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `connection-definition-elaboration` | view | undrawn | F | a connection's own interconnection view nested in its node is not drawn |
| `connection-usage-elaboration` | view | undrawn | F | a connection's own interconnection view nested in its node is not drawn |
| `n-ary-connection` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-connection-def` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-connection-dot` | node | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-def-connection-dot` | node | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-def-segment` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |
| `n-ary-segment` | edge | undrawn | C | n-ary dots are documented as undrawn in view-rendering-forms.md § Notation table |

#### §8.2.3.14 Interfaces Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `ends-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `interface-connection` | edge | approximate | F | drawn as a connection edge between the ports; the «interface» keyword is not written |

#### §8.2.3.16 Flows Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `flow` | edge | approximate | F | view-rendering-forms.md § Notation table: Solid with a filled head; PlantUML has no filled head, its open `-->` stands in (as the Pilot writes it). |
| `flow-node-l` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `flow-node-r` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `flow-on-connection` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `flows-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `message` | edge | undrawn | D | documented as undrawn in view-rendering-forms.md § Notation table: the sequence form draws its own arrow |
| `message-node-l` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `message-node-r` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `sflow-node-l` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |
| `sflow-node-r` | node | undrawn | C | the item box on a connection line (flow-on-connection) is documented as undrawn in view-rendering-forms.md § Notation table |

#### §8.2.3.17 Actions Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `accept-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `accept-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |
| `action-flow-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `assign-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `assign-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |
| `else-body` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `else-branch` | edge | undrawn | B | the else branch of a decision is drawn as a plain succession without the `else` guard |
| `for-loop-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `for-loop-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |
| `if-condition` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `if-else-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |
| `ifelse-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `iteration` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `loop-body` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `param-b` | node | approximate | B | the pin is placed on the side its direction names but the direction arrow (pdh/pdv) inside it is not drawn |
| `param-l` | node | approximate | B | the pin is placed on the side its direction names but the direction arrow (pdh/pdv) inside it is not drawn |
| `param-r` | node | approximate | B | the pin is placed on the side its direction names but the direction arrow (pdh/pdv) inside it is not drawn |
| `param-t` | node | approximate | B | the pin is placed on the side its direction names but the direction arrow (pdh/pdv) inside it is not drawn |
| `perform-actions-swimlanes` | compartment | undrawn | B | swimlanes are not drawn |
| `performed-by-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `send-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `send-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |
| `swimlane` | compartment | undrawn | B | swimlanes are not drawn |
| `terminate-node` | node | approximate | B | DOT and Mermaid draw a double circle, the figure's X-in-a-circle only draw.io (umlDestroy) does |
| `then-body` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `until-condition` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `while-condition` | compartment | undrawn | B | the control node's condition and body compartments are not drawn; the body's actions are drawn as nodes |
| `while-loop-action-name-compartment` | compartment | undrawn | B | the box is drawn but its keyword is not written (behavior.go actionNodeKind) |
| `while-loop-action-node` | node | approximate | B | behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written |

#### §8.2.3.18 States Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `do-action` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `do-action-name-comp` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `entry-action` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `entry-action-name-comp` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `exit-action` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `exit-action-name-comp` | node | undrawn | B | entry/do/exit action nodes are not drawn; the state box lists them as text |
| `st-succession` | edge | approximate | B | view-rendering-forms.md § Notation table: Solid with an open head; Mermaid and D2 have only a filled head. |
| `state-actions-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `state-def-name-compartment` | compartment | approximate | B | the keyword and name are written, the optional «parallel» marker is not |
| `state-do-action` | compartment | approximate | B | entry/do/exit actions are written as text lines in the state box, not as labelled compartments |
| `state-entry-action` | compartment | approximate | B | entry/do/exit actions are written as text lines in the state box, not as labelled compartments |
| `state-exit-action` | compartment | approximate | B | entry/do/exit actions are written as text lines in the state box, not as labelled compartments |
| `state-name-compartment` | compartment | approximate | B | the keyword and name are written, the optional «parallel» marker is not |
| `state-transition-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `successions-compartment` | compartment | undrawn | B | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `transition` | edge | approximate | B | view-rendering-forms.md § Notation table: Solid with an open head; Mermaid and D2 have only a filled head. |

#### §8.2.3.19 Calculations Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `calcs-compartment` | compartment | approximate | F | headed `calculations`; the figure heads it `calcs` |
| `result-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |

#### §8.2.3.21 Requirements Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `actor` | node | approximate | F | a box headed «actor» in DOT, PlantUML, Mermaid and text; only draw.io draws the stick figure (drawio.go plainStyle umlActor) |
| `concern-stakeholder-link` | edge | undrawn | F | the line from a concern to its stakeholder is not drawn |
| `frame-concern-name-compartment` | compartment | approximate | F | written as «frame», the figure's keyword is «frame concern» |
| `frame-concern-node` | node | approximate | F | written as «frame», the figure's keyword is «frame concern» |
| `satisfies-compartment` | compartment | approximate | F | headed `satisfys` by compartments.go compartmentName's plural; the figure heads it `satisfies` |
| `satisfy-requirement-name-compartment` | compartment | approximate | F | written as «satisfy», the figure's keyword is «satisfy requirement» |
| `satisfy-requirement-usage` | node | approximate | F | written as «satisfy», the figure's keyword is «satisfy requirement» |
| `satisfy-requirements-compartment` | compartment | approximate | F | headed `satisfys` by compartments.go compartmentName's plural; the figure heads it `satisfies` |
| `subject-compartment` | compartment | approximate | F | headed `subjects`; the figure heads it `subject` |

#### §8.2.3.22 Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `objective-compartment` | compartment | approximate | F | the objective is drawn as a note anchored to the case (case.go objectiveNode), not as a compartment |

#### §8.2.3.24 Verification Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `verification-methods-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `verifies-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `verify-requirement-name-compartment` | node | undrawn | F | a verify requirement usage lives in a verification case's objective, which is drawn as a note (case.go objectiveNode) |
| `verify-requirement-node` | node | undrawn | F | a verify requirement usage lives in a verification case's objective, which is drawn as a note (case.go objectiveNode) |

#### §8.2.3.25 Use Cases Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `includes-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |

#### §8.2.3.26 Views and Viewpoints Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `exposes-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `filters-compartment` | compartment | undrawn | F | no renderer writes this compartment; the members are drawn as nodes or edges, or not at all |
| `framed-view` | view | approximate | A | only the Cameo-style DOT frame draws it (dot.go openFrame); the default style draws no frame |
| `rendering-compartment` | compartment | approximate | F | headed `renderings`; the figure heads it `rendering` |
| `root-view` | view | approximate | A | only the Cameo-style DOT frame draws it (dot.go openFrame); the default style draws no frame |
| `view-frame-info-compartment-bl` | compartment | undrawn | A | no form draws the frame's info compartments |
| `view-frame-info-compartment-br` | compartment | undrawn | A | no form draws the frame's info compartments |
| `view-frame-info-compartment-tr` | compartment | undrawn | A | no form draws the frame's info compartments |
| `view-frame-name-compartment` | compartment | approximate | A | only the Cameo-style DOT frame draws it (dot.go openFrame); the default style draws no frame; its header reads the view's name without the «view» keyword |

#### §8.2.3.27 Metadata Graphical Notation

| Production | Category | Verdict | Package | Reason |
| --- | --- | --- | --- | --- |
| `metadata-feature-annotation-node` | node | undrawn | F | a metadata usage annotating an element is not drawn as a «metadata» note |
<!-- census:end gaps -->

### Work packages

<!-- census:begin packages -->
| Package | Scope | Undrawn | Approximate | Productions |
| --- | --- | --- | --- | --- |
| A | §8.2.3.26 view frame and §8.2.3.4 annotation nodes | 9 | 4 | `annotation-link`, `comment-node`, `comment-with-keyword`, `comment-without-keyword`, `documentation-compartment`, `documentation-node`, `textual-representation-node`, `framed-view`, `root-view`, `view-frame-info-compartment-bl`, `view-frame-info-compartment-br`, `view-frame-info-compartment-tr`, `view-frame-name-compartment` |
| B | §8.2.3.17–.18 action and state node forms | 27 | 18 | `accept-action-name-compartment`, `accept-action-node`, `action-flow-compartment`, `assign-action-name-compartment`, `assign-action-node`, `else-body`, `else-branch`, `for-loop-action-name-compartment`, `for-loop-action-node`, `if-condition`, `if-else-action-node`, `ifelse-action-name-compartment`, `iteration`, `loop-body`, `param-b`, `param-l`, `param-r`, `param-t`, `perform-actions-swimlanes`, `performed-by-compartment`, `send-action-name-compartment`, `send-action-node`, `swimlane`, `terminate-node`, `then-body`, `until-condition`, `while-condition`, `while-loop-action-name-compartment`, `while-loop-action-node`, `do-action`, `do-action-name-comp`, `entry-action`, `entry-action-name-comp`, `exit-action`, `exit-action-name-comp`, `st-succession`, `state-actions-compartment`, `state-def-name-compartment`, `state-do-action`, `state-entry-action`, `state-exit-action`, `state-name-compartment`, `state-transition-compartment`, `successions-compartment`, `transition` |
| C | §8.2.3.3/.13 n-ary dots and §8.2.3.16 flow-on-connection | 19 | 0 | `n-ary-association-dot`, `n-ary-dependency`, `n-ary-dependency-client-link`, `n-ary-dependency-client-or-supplier-link`, `n-ary-dependency-supplier-link`, `cdot-label`, `n-ary-connection`, `n-ary-connection-def`, `n-ary-connection-dot`, `n-ary-def-connection-dot`, `n-ary-def-segment`, `n-ary-segment`, `flow-node-l`, `flow-node-r`, `flow-on-connection`, `message-node-l`, `message-node-r`, `sflow-node-l`, `sflow-node-r` |
| D | §8.2.3.9 sequence glyphs | 8 | 3 | `event-occurrence`, `event-occurrence-name-compartment`, `individuals-compartment`, `portion-relationship`, `proxy-label`, `sequence-compartment`, `sq-port`, `sq-proxy`, `sq-succession`, `succession-label`, `message` |
| E | §8.2.3.2 textual-compartment greying | 2 | 0 | `element-inside-textual-compartment`, `ellipsis-at-lower-left-corner` |
| F | everything else | 32 | 30 | `general-compartment`, `imported-package-with-name-in-tab`, `imported-package-with-name-inside`, `members-compartment`, `owned-membership`, `package-node`, `package-with-name-in-tab`, `package-with-name-inside`, `packages-compartment`, `relationships-compartment`, `composite-feature-membership`, `definition`, `extended-def`, `extended-def-name-compartment`, `extended-usage`, `extended-usage-name-compartment`, `features-compartment`, `noncomposite-feature-membership`, `redefinition`, `reference-subsetting`, `subclassification`, `subsetting`, `variant-elementusages-compartment`, `variants-compartment`, `enums-compartment`, `directed-features-compartment`, `interconnection-compartment`, `pdh`, `pdv`, `port-b`, `port-l`, `port-r`, `port-t`, `proxy-h`, `proxy-v`, `connection-definition-elaboration`, `connection-usage-elaboration`, `ends-compartment`, `interface-connection`, `flow`, `flows-compartment`, `calcs-compartment`, `result-compartment`, `actor`, `concern-stakeholder-link`, `frame-concern-name-compartment`, `frame-concern-node`, `satisfies-compartment`, `satisfy-requirement-name-compartment`, `satisfy-requirement-usage`, `satisfy-requirements-compartment`, `subject-compartment`, `objective-compartment`, `verification-methods-compartment`, `verifies-compartment`, `verify-requirement-name-compartment`, `verify-requirement-node`, `includes-compartment`, `exposes-compartment`, `filters-compartment`, `rendering-compartment`, `metadata-feature-annotation-node` |
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
  *drawn*; where the written keyword differs from the figure's — `«satisfy»` for
  `«satisfy requirement»`, `«frame»` for `«frame concern»`, `«occurrence»` for an event occurrence
  — the node and its name compartment are *approximate*. `verify-requirement-node` is *undrawn*: a
  verify usage lives in a verification case's objective, which is drawn as a note.
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
