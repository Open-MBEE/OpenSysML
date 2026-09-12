# Document Authoring

A document definition is a `part def` specializing
`DocumentQueries::Document`. Its nested parts are its content, rendered in
declaration order. This chapter covers each block; the snippets are excerpts
of models that render with the current binary (most are drawn from the
[worked example](worked-example.md)).

## Document and sections

```sysml
part def MassReport :> Document {
	attribute redefines title = "Telescope Mass Report";

	part breakdown : Section {
		attribute redefines title = "Subsystem Masses";

		part heavy : Section {
			attribute redefines title = "Heavy Subsystems";
			// ...
		}
	}
}
```

- The document's `title` is required and renders as the level-1 heading.
- A `Section` requires a `title` too and renders as a heading one level
  deeper than its parent, saturating at Markdown's level 6.
- Sections nest arbitrarily; anything a document can contain, a section can.
- A document nested inside a document is a planning error.

## Paragraphs

A `Paragraph` carries exactly one of three things:

**Static text** — a literal `text` attribute:

```sysml
part intro : Paragraph {
	attribute redefines text = "Mass rollup for the telescope assembly.";
}
```

**A query** — each projected value becomes one plain text run, joined by
spaces (nested column runs restyle this — see
[Styled query text](#styled-query-text-column-runs)):

```sysml
part summary : Paragraph {
	calc names : HeavySubsystemNames {
		in root = telescope;
		in threshold = "10";
	}
}
```

renders as `mount segmentControl`.

**Inline runs** — nested `Span`, `Link` and `Ref` parts, composed in
declaration order and joined by single spaces (next section).

Giving a paragraph both text and a query, or runs alongside either, is a
typed planning error rather than a guess about your intent.

## Inline runs, links and cross-references

```sysml
part guide : Paragraph {
	part lead : Span {
		attribute redefines text = "This report is";
	}
	part generated : Span {
		attribute redefines text = "generated";
		attribute redefines style = "emphasis";
	}
	part tool : Span {
		attribute redefines text = "sysml -render-document";
		attribute redefines style = "code";
	}
	part docsLink : Link {
		attribute redefines text = "(OpenSysML)";
		attribute redefines target = "https://opensysml.org/";
	}
	part massesRef : Ref {
		ref redefines target = breakdown;
	}
}
```

renders as:

```markdown
This report is *generated* `sysml -render-document` [(OpenSysML)](<https://opensysml.org/>) [Subsystem Masses](#breakdown)
```

**`Span`** carries required `text` and an optional `style`: `"plain"` (the
default), `"emphasis"` (`*text*`), `"strong"` (`**text**`) or `"code"`
(`` `text` ``). Content is still escaped inside the styling — a `*` in an
emphasis span cannot break out of it, and a code span grows its backtick
fence past any backticks in the text.

**`Link`** carries required `text` and a required `target` URL, rendering as
an inline Markdown link with the destination in pointy brackets (so
parentheses and spaces in URLs survive).

**`Ref`** cross-references a named content block of this document — or of
another document — by name: `ref redefines target = <block>`. The renderer
gives the referenced block a stable HTML anchor derived from its named path —
`<a id="breakdown"></a>` before the section above — and the `Ref` renders as
a link to it. `text` is optional; it defaults to the target's title (for a
section), caption (for a table or diagram) or name. A target that is neither
a content block nor a document, or one without a stable name, is a typed
planning error.

### Cross-document references

To reference another document, declare a usage typed by the target document
definition, then name it (for the document's root) or reach into it with dot
notation (for one of its content blocks):

```sysml
ref appendix : 'Mass Appendix';

part def SystemReport :> Document {
	attribute redefines title = "System Report";
	part intro : Paragraph {
		part see : Ref {
			ref redefines target = appendix.tables.masses;
		}
		part whole : Ref {
			ref redefines target = appendix;
		}
	}
}
```

Targets resolve at planning time against the loaded workspace. A content
target renders as a relative link into the target document's generated file
with the block's stable anchor — a destination like
`Observatory-Mass.20Appendix.md#tables-masses` behind the link text;
a root target links to the file alone. The file name is deterministic: the
target document's fully qualified name with `::` replaced by `-` and any
byte outside ASCII letters, digits and `_` escaped as `.XX` (uppercase hex),
plus `.md`. Render the whole set with `-render-documents <dir>` so the links
resolve on disk. Rendering a single document that references another still
succeeds — the link points at the expected file name of the unrendered
target, and it dangles until that document is rendered into the same
directory. An unknown target is a typed planning error, and a target usage
typed by more than one document definition is an ambiguous-target error;
both carry the reference's source location.

## Styled query text (column runs)

A query-backed paragraph or list may nest **column runs** — `SpanColumn` and
`LinkColumn` parts — that map projected columns to styled runs. Each result
row renders one run per column run, in declaration order:

```sysml
part styledSummary : Paragraph {
	calc names : StyledHeavyNames {
		in root = telescope;
		in threshold = "10";
	}
	part styledName : SpanColumn {
		attribute redefines column = "name";
		attribute redefines styleColumn = "style";
	}
	part linkedName : LinkColumn {
		attribute redefines column = "name";
		attribute redefines targetColumn = "url";
	}
}
```

```markdown
**mount** [mount](<https://example.com/parts#mount>) **segmentControl** [segmentControl](<https://example.com/parts#segmentControl>)
```

**`SpanColumn`** carries a required `column` naming the projected column its
text comes from, plus at most one of:

- `style` — a fixed `"plain"` (the default), `"emphasis"`, `"strong"` or
  `"code"` applied to every row;
- `styleColumn` — a projected column supplying each row's style. Each row
  must supply exactly one string value among the four styles; anything else
  is a typed evaluation error naming the query, column and row.

**`LinkColumn`** carries a required `column` for the link text and a required
`targetColumn` naming a projected column that supplies each row's one
non-empty link destination.

Computed columns (`Column(name, expression)`) feed column runs like any
projected property, so a query can compute both the text and the style or
target it renders with — the `styleColumn`/`targetColumn` example above uses
computed `style` and `url` columns.

Column names are checked against the query's statically-known projection at
planning time; a projection only known at evaluation (e.g. a parameter-driven
`properties`) is checked when the query runs. Column runs alongside static
text or inline runs, on a paragraph without a query, or anywhere other than a
query-backed paragraph or list, are typed planning errors.

## Tables

A `Table` requires a query and may carry a `caption` (rendered in emphasis
above the table):

```sysml
part masses : Table {
	attribute redefines caption = "All subsystems by mass";
	calc rows : SubsystemTable {
		in root = telescope;
	}
}
```

```markdown
<!-- caption -->
*All subsystems by mass*

| name | mass |
| --- | --- |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |
```

The header row is the query's projected column names; a query that projects
no columns gets a single `element` column holding each element's qualified
name. An empty result still renders the header and delimiter rows, so the
document shows *that* the table is empty rather than omitting it. Cell values
render faithfully: strings unquoted, integers in base 10, reals in shortest
notation, booleans as `true`/`false`, unbounded multiplicity as `*`, elements
by qualified name, and quantities as the magnitude followed by the unit the
model spelt in brackets — `2290000 [kg]`, escaped in Markdown as
`2290000 \[kg\]` so the brackets read as text. An attribute whose value is
not a constant (`mass = dryMass + propellantMass`) is a typed error naming
the query, the property and the row, not an empty cell; see the [query
cookbook](query-cookbook.md#quantity-cells).

### Grouped tables

A `groupBy` attribute names one projected column; rows partition into one
subtable per distinct value of that column, in order of first appearance,
with the group key above each in strong emphasis:

```sysml
part zones : Table {
	attribute redefines caption = "Subsystems grouped by zone";
	attribute redefines groupBy = "zone";
	calc rows : ZonedSubsystems {
		in root = telescope;
	}
}
```

```markdown
<!-- caption -->
*Subsystems grouped by zone*

**zone: support**

| zone | name | mass |
| --- | --- | --- |
| support | mount | 15 |

**zone: payload**

| zone | name | mass |
| --- | --- | --- |
| payload | optics | 8.5 |
| payload | segmentControl | 20 |
```

The group column must be one the query statically projects — an unknown name
is a typed error at planning time, not an empty rendering. Row order within
each group is the query's order.

## Lists

A `List` requires a query and renders each result value as one item. `style`
is `"bullet"` (the default) or `"number"`; anything else is a typed planning
error. Nested column runs restyle each item's runs — see
[Styled query text](#styled-query-text-column-runs).

```sysml
part heavyItems : List {
	attribute redefines style = "number";
	calc items : HeavySubsystemNames {
		in root = telescope;
		in threshold = "10";
	}
}
```

```markdown
1. mount
2. segmentControl
```

An empty result renders as nothing — unlike a table, an empty list leaves no
trace.

## Definitions — query rows as prose

A `Definitions` block renders each query row as one prose entry: a **term**
followed by its **description**, both taken from projected columns the block
names. It is how model-authored text — a requirement's short name and its
`doc` body, a part's identifier and its description — reads as sentences
rather than as cells. `term` and `description` are required and name
projected columns of the bound query; either may be a property (`shortName`,
`name`, `documentation`, …) or a computed `Column`:

```sysml
calc def Reqs :> Query {
	in root : Element;
	Project(
		source = RelatedElements(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "RequirementUsage"),
			relationshipKind = "typing", direction = "outgoing", maxDepth = 1
		),
		properties = ("shortName", "name", "documentation")
	)
}

part requirementText : Definitions {
	attribute redefines term = "shortName";
	attribute redefines description = "documentation";
	calc rows : Reqs {
		in root = spec;
	}
}
```

```markdown
**HLR-R001** — The mission shall safely return all three crew members to Earth.

**HLR-R002** — The mission shall achieve a soft landing on the lunar surface.
```

Markdown writes one paragraph per row: the term in strong emphasis, an em
dash, then the description. HTML writes a description list — `<dl>` with one
`<dt>`/`<dd>` group per row, each group carrying the row's element — so a
theme can style terms and descriptions apart; PDF follows the Markdown form.
A cell with several values (an element with two `doc` comments) joins them
with spaces, as a list item does. A row whose term is absent renders its
description alone and vice versa; a row with neither writes nothing in
Markdown and an empty group in HTML. An empty result follows a list's: it
leaves no trace in Markdown, while HTML keeps the empty `<dl>` — as it keeps
an empty `<ul>` — so the block stays addressable by name and query.

Both column names are checked against the query's statically-known
projection at planning time; a name the query does not project is a typed
error there, or at evaluation when the projection is only known then. A
`Definitions` block without a query, or one nesting other content, is a typed
planning error.

The complete model is [`examples/requirements.sysml`](examples/requirements.sysml)
and its rendered output [`examples/requirements.md`](examples/requirements.md):

```console
$ sysml docs/manual/examples/requirements.sysml -run-query "Requirements::Reqs root=Requirements::spec"
$ sysml docs/manual/examples/requirements.sysml -render-document Requirements::RequirementsReport
$ sysml docs/manual/examples/requirements.sysml -render-document Requirements::RequirementsReport -doc-form html
```

## Diagrams

A `Diagram` embeds a rendering of a model element, drawn by the same view
engine that serves the editor's diagram panel. Its `source` names either a
declared `view` or a plain element:

```sysml
view interconnectView {
	expose imagingChain;
	render asInterconnectionDiagram;
}

part imaging : Diagram {
	attribute redefines caption = "Imaging chain interconnection";
	ref redefines source = interconnectView;
}

part structure : Diagram {
	attribute redefines caption = "Telescope part tree, left to right";
	attribute redefines kind = "tree";
	attribute redefines direction = "LR";
	ref redefines source = telescope;
}
```

- A **view** source carries its own rendering kind from its `render` clause;
  stating a `kind` on the diagram too is a conflict error.
- A **plain element** source requires a `kind`: `"tree"`,
  `"interconnection"`, `"state"`, `"action"`, `"table"` or `"sequence"`.
- `caption` is optional and renders in emphasis above the diagram.
- `direction` — `"TB"`, `"LR"`, `"RL"` or `"BT"` — is accepted only by kinds
  drawn as directed graphs; it becomes the Mermaid flowchart direction or a
  `stateDiagram-v2` `direction` statement, or the Graphviz `rankdir` when the
  document is rendered with DOT diagrams. Stating one on a sequence diagram is
  a typed error.

A diagram block states *what* is drawn, not the notation it is written in:
that is a choice made when the document is rendered. By default most kinds
render as a fenced ` ```mermaid ` block:

```markdown
*Imaging chain interconnection*

```mermaid
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["part Observatory::imagingChain"]
    n1["part camera (Camera)"]
    n2["part recorder (Recorder)"]
  end
  n1 ---|"link"| n2
```
```

Rendered with `-diagram-form dot` (`%render-document <name> dot` in the REPL,
`diagramForm: "dot"` over the LSP), every graph-shaped diagram of the
document — a `tree`, `interconnection`, `state` or `action` rendering — is a
fenced ` ```dot ` block of Graphviz DOT instead, for a toolchain that lays
diagrams out with Graphviz. No Graphviz installation is needed to write it:

```markdown
*Imaging chain interconnection*

```dot
// view: Observatory::interconnectView
// kind: interconnection
// stated: render asInterconnectionDiagram
// layout: dot
digraph "Observatory::interconnectView" {
  graph [rankdir=LR];
  node [shape=box];
  subgraph "cluster_n0" {
    label="part Observatory::imagingChain";
    "n1" [label="part camera\nCamera"];
    "n2" [label="part recorder\nRecorder"];
  }
  "n1" -> "n2" [label="link", arrowhead=none];
}
```
```

The HTML backend embeds the source in `<pre class="dot">`, and the PDF backend
keeps it as source under a notice rather than drawing it. A `sequence` kind
has no DOT form, so a document holding one cannot be rendered with DOT
diagrams; the failure is a typed error naming the block.

The `table` kind is the exception — it renders as a pipe table of the
element's structure (Element / Kind / Type / Declared in) rather than a
Mermaid or DOT block, whichever diagram form the document is rendered with.

## Binding queries to blocks

Every query-carrying block uses the same form:

```sysml
calc rows : SubsystemTable {
	in root = telescope;
	in threshold = "10";
}
```

Bindings are validated against the query's compiled signature at planning
time: an unknown parameter, a duplicate, a missing one without a usable
default, or a type or multiplicity mismatch is a typed error before anything
runs. A binding's value is an element name or a literal; the engine does not
evaluate arbitrary default expressions.

## Escaping — write content freely

Model text cannot corrupt document structure. The renderer backslash-escapes
Markdown metacharacters (`|`, `*`, `_`, `#`, backticks, backslashes,
brackets, HTML-sensitive characters), folds newlines to spaces in prose and
`<br>` in table cells, and escapes a leading quote, bullet or list marker.
A part named `baffle|shroud *tricky*` renders as exactly that text in a
table cell, not as a broken row.
