# Outputs

## Markdown

Markdown is the primary output form and the default:

```console
$ sysml report.sysml -render-document Observatory::MassReport            # stdout
$ sysml report.sysml -render-document Observatory::MassReport -o report.md
```

What the renderer emits:

- The document title as a level-1 ATX heading; each section one level deeper,
  saturating at level 6.
- Paragraphs as prose, with inline runs styled (`*emphasis*`, `**strong**`,
  `` `code` ``, and links as `[text]` followed by the angle-bracketed URL in
  parentheses).
- A stable `<a id="..."></a>` anchor before every block that a `Ref` targets;
  the reference renders as a link to it.
- Tables as GitHub-flavored pipe tables; grouped tables as one subtable per
  group key.
- Lists as `-` bullets or `1.` numbered items.
- Diagrams as fenced ` ```mermaid ` blocks (` ```dot ` blocks of Graphviz DOT
  when rendered with `-diagram-form dot`, ` ```plantuml ` blocks with
  `-diagram-form plantuml`, or a pipe table for the `table` kind whichever
  form), with captions in emphasis.
- An object the session holds as its path from the object the query was bound
  through (`car.wheels[2]`), and a verdict as `<assertion> on <path>: <verdict>`
  (`assert constraint powerLow on car.engine: violated`).
- All model-derived text escaped so it cannot break document structure.
- A single trailing newline, no trailing whitespace.

The output is deterministic: the same model — and, for a document that reads
the objects a session holds, the same objects in the same state — produces
byte-identical Markdown on every run, which is why the repository can keep rendered documents as
golden files (this manual does exactly that — see
[the worked example](worked-example.md)).

### Multi-document sets

`-render-documents <dir>` renders every document definition the model
declares into the directory, one Markdown file per document:

```console
$ sysml reports.sysml -render-documents rendered
```

File names are deterministic: the document's fully qualified name with `::`
replaced by `-`, any byte outside ASCII letters, digits and `_` escaped as
`.XX` (uppercase hex), plus `.md`. Cross-document references (see
[the authoring chapter](authoring.md)) therefore resolve as relative links
between the written files, and repeated runs write identical bytes. Rendering
a cross-referencing document on its own still succeeds; its external links
point at the targets' expected file names and dangle until those documents
are rendered into the same directory.

## HTML

`-doc-form html` renders the same document tree as HTML. Nothing is
converted from the Markdown: the renderer reads the compiled document tree,
so the model facts Markdown cannot carry reach the markup.

```console
$ sysml report.sysml -render-document Observatory::MassReport \
    -doc-form html -o report.html
```

The structure is ordinary semantic HTML — `<article>`, nested `<section>`
whose heading levels follow the nesting, `<p>`, `<table>` with `<caption>`,
`<thead>` and `<th scope="col">`, `<ul>`/`<ol>`, `<figure>` with
`<figcaption>`, `<dl>` for definitions, `<nav>` for the contents, and `<em>`, `<strong>`, `<code>`,
`<a>` inline — so a reader, a screen reader and a static-site generator all
see a document rather than a grid of `<div>`s.

The model rides alongside the structure. A small `sysml-` class vocabulary
names each part of the document (`sysml-document`, `sysml-section`,
`sysml-table`, `sysml-row`, `sysml-cell`, `sysml-value`, `sysml-list`,
`sysml-item`, `sysml-definitions`, `sysml-entry`, `sysml-term`,
`sysml-description`, `sysml-diagram`, `sysml-caption`, `sysml-link`,
`sysml-ref` and their kin), and `data-` attributes carry the facts behind
it: the content kind and name, the query behind a table, list or
definitions block, the group-by column, each row's, item's or entry's
selected element with its element kind
(`partUsage`, `requirementDef`, …), each cell's projected column and value
kind (a `quantity` value also carries its `data-magnitude` and `data-unit`
apart), and a diagram's view, kind and flow direction.

```html
<tr class="sysml-row" data-element="Observatory::telescope::mount"
    data-element-kind="partUsage">
<td class="sysml-cell" data-column="mass" data-value-kind="real">
<span class="sysml-value" data-value-kind="real">15</span></td>
</tr>
```

A row over an object the session holds ([Objects the session holds](query-cookbook.md#objects-the-session-holds))
adds `data-object="#<id>"`, the id the instantiation report printed, beside
the usage the object stands for, and an object-valued cell is a
`span.sysml-object` whose text is the object's path. A row a `Verdicts` query
answered ([Which constraints and requirements hold](query-cookbook.md#which-constraints-and-requirements-hold))
is the assertion checked, and its row or list item carries `data-verdict`
(`holds`, `violated` or `undecided`), `data-path` (the object checked) and,
over a held object, its `data-object`; a verdict-valued cell is a
`span.sysml-verdict` with the same three attributes, whose text is the line
Markdown prints, `<assertion> on <path>: <verdict>`. So a stylesheet colours
what is violated with
`[data-verdict="violated"] { … }` and a script reads which objects it is
about without parsing the text.

```html
<tr class="sysml-row" data-object="#2" data-verdict="violated" data-path="car.engine"
    data-element="Garage::Engine::powerLow" data-element-kind="constraintUsage">
<td class="sysml-cell" data-column="path" data-value-kind="string">
<span class="sysml-value" data-value-kind="string">car.engine</span></td>
<td class="sysml-cell" data-column="verdict" data-value-kind="string">
<span class="sysml-value" data-value-kind="string">violated</span></td>
</tr>
```

Identifiers are the same anchors the Markdown writes, so a `Ref` resolves
within the page; in a `-render-documents` set it resolves across pages, whose
file names are the Markdown names with `.html` instead of `.md`. Diagram
blocks embed their Mermaid source in `<pre class="mermaid">`, which a page
that loads Mermaid renders as a diagram and any other page shows as source.
By default the output loads nothing over the network, runs no JavaScript of
its own, and is byte-identical between runs. To have a browser draw the
diagrams, `-html-mermaid cdn` adds a `<script>` loading a pinned Mermaid
release from jsDelivr, and `-html-mermaid <url>` loads it from a URL of your
own; a second `<script>` configures it to draw the page's largest chart, which
Mermaid's default size limits (50 000 characters, 500 edges) would refuse. The
configuration never exceeds twenty times those defaults, so no page asks a
browser for unbounded work: a chart past 1 000 000 characters or 10 000 edges
is refused before it is written, naming the chart and its size, and is drawn
with `-diagram-form dot` or `plantuml` instead. The page keeps the source, so it
still reads where the script cannot load. A fragment has no page shell for the
script, so a page embedding one loads Mermaid itself. Rendered with
`-diagram-form dot` or `-diagram-form plantuml`, every graph-shaped diagram
embeds its Graphviz DOT source in
`<pre class="dot">` or its PlantUML source in `<pre class="plantuml">` instead;
the page never draws it, and `-html-mermaid` leaves it alone.

Formulas work the same way. A math span becomes `<span class="sysml-math">`
and a `Formula` block a `<figure class="sysml-formula">` with its caption,
each holding the LaTeX between MathJax's `\(…\)` or `\[…\]` delimiters.
By default the page loads nothing and shows the source; `-html-math cdn`
adds a `<script>` loading a pinned MathJax release from jsDelivr, and
`-html-math <url>` loads it from a URL of your own. The script is configured
to typeset `.sysml-math` elements alone, so a `$` or `\(` in ordinary prose
is never mistaken for a formula. `-html-math` and `-html-mermaid` combine, and
neither combines with `-html-fragment` — the embedding page loads the
typesetter itself. See [Mathematics](#mathematics) for the PDF side.

### Styling it

The default stylesheet is inlined in a standalone page and declared in a
cascade layer:

```css
@layer opensysml;
@layer opensysml { /* the defaults */ }
```

Your own CSS is unlayered, so it wins on cascade origin rather than
specificity — overriding a default needs neither `!important` nor a matching
selector. Every default value comes from a `--sysml-*` custom property on
`.sysml-document`, so retheming can be a handful of properties, and the
renderer emits no `style` attributes to compete with.

| Flag | Effect |
|---|---|
| `-html-theme <name>` | Layer a bundled theme over the default sheet: `default`, `modern`, `print` or `report` |
| `-html-default-css` | Write the default sheet and exit, to copy from; with `-html-theme`, the theme's whole sheet |
| `-html-css <file\|url>` | Add a sheet after the default one: a file is inlined, a URL is linked (repeatable, applied in order) |
| `-html-no-default-css` | Leave the default sheet out |
| `-html-fragment` | Write the `<article>` alone, with no page shell and no stylesheet |

The bundled themes are written against the same tokens, inside the same
layer, right after the default sheet — so a theme changes the look while your
unlayered CSS still wins over both:

| Theme | Look |
|---|---|
| `default` | The default sheet alone: system sans-serif, one navy accent, boxed tables |
| `modern` | Clean corporate sans-serif: filled table headers, zebra rows, rounded surfaces for code and contents |
| `report` | Formal technical report: serif body, wider measure, open tables ruled top and bottom, captions above |
| `print` | Monochrome and compact for paper: black rules, no fills, tables and figures kept whole across page breaks, external links spelled out |

A theme needs the default sheet under it, so it is refused with
`-html-no-default-css`, and a fragment has no page to style, so it is refused
with `-html-fragment`.

A `-render-documents -doc-form html` set writes its stylesheets as files
beside the pages — `sysml-document.css` (default sheet, with the theme when
one is named) and each `-html-css` file, under its
own base name, escaped and shortened where a name is not a portable file name
and distinguished where two sheets share one — and every page links them in
order, so the styling of a whole set is edited in one place. A `-html-css` URL
stays a link.

A title page, a table of contents and section numbering are
[deliverable options](#deliverable-options) shared with PDF.

## PDF

`-doc-form pdf` renders the same document tree to PDF. It requires `-o`
(PDFs are not written to stdout):

```console
$ sysml report.sysml -render-document Observatory::MassReport \
    -doc-form pdf -o report.pdf
```

Internally the PDF backend reads the compiled document tree, renders any
Mermaid diagrams to SVG with Mermaid CLI (`mmdc`) — each under a
configuration sized to the chart, within the same ceiling the
[HTML backend](#html) draws under, and the same five-minute limit every
converter runs under — typesets any formulas
with KaTeX (`katex`), and hands an external converter the document in the
form it reads: an HTML-to-PDF engine gets the [HTML backend's](#html) page
with the drawn diagrams and typeset formulas in place of their source and a
print stylesheet over the default sheet; pandoc gets the
[Markdown](#markdown) with a filter swapping the artwork in on its own
syntax tree. Rendered with `-diagram-form dot`, the diagrams are drawn by
Graphviz — the `dot` named by `OPENSYSML_DOT`, else the one on `PATH`, writing
SVG under the layout engine the block's `// layout:` header names, so a view
the model positions is drawn where its `Layout` annotations put it — and with
`-diagram-form plantuml` by the PlantUML jar named by `OPENSYSML_PLANTUML_JAR`,
run by the `java` named by `OPENSYSML_JAVA` or found on `PATH`. Both are
optional where Mermaid CLI is required: without the tool, the PDF keeps the
block's DOT or PlantUML source under a notice naming the variable to set, and
the render still succeeds. A tool that is present and fails stops the render
with a typed `tool-failed` error carrying its output, as a failing `mmdc`
does.

### Styling a PDF

An engine reading HTML lays out the same markup the HTML form writes, so a
PDF is styled the way an HTML page is. The print stylesheet — page size and
margins, the page-number footer, print faces and sizes, page breaks kept out
of tables and figures, the title page and contents on pages of their own —
is declared in a second cascade layer after the default sheet:

```css
@layer opensysml;        /* the default sheet, or the theme over it */
@layer opensysml-print;  /* the PDF backend's print sheet */
```

Both layers draw their values from the same `--sysml-*` tokens and write no
`style` attributes, so `-html-theme` rethemes a PDF as it does a page,
`-html-css` sheets apply unlayered after both layers and win on cascade
origin, and `-html-no-default-css` leaves both layers out so that only your
sheets — `@page` rules included — style the PDF. A sheet's relative `url()`
and `@import` references resolve against the PDF's directory, as a page's
resolve against the page's. The pandoc engine reads Markdown and writes its
own HTML, so `-html-theme` and `-html-no-default-css` are refused for it,
while `-html-css` sheets are attached in pandoc's page after its own.
`-html-fragment`, `-html-mermaid` and `-html-math` shape a browser page and
are refused for PDF.

### Engines

`-pdf-engine` selects the converter (default `weasyprint`):

| Engine | Tool invoked | Notes |
|---|---|---|
| `weasyprint` | `weasyprint`, an HTML-to-PDF paged-media engine | Default; open source |
| `pandoc` | `pandoc` reading the Markdown itself, with WeasyPrint as its PDF engine | |
| `prince` | `prince`, a commercial HTML-to-PDF engine | License required |

The converters are external tools, not bundled with the binary. If the
selected tool is not on `PATH`, the render fails with a typed `tool-missing`
error naming it. Environment variables override discovery:
`OPENSYSML_WEASYPRINT`, `OPENSYSML_PANDOC`, `OPENSYSML_PRINCE`,
`OPENSYSML_MMDC`, `OPENSYSML_MMDC_PUPPETEER` (extra Puppeteer
configuration for Mermaid CLI), `OPENSYSML_KATEX` and `OPENSYSML_KATEX_CSS`
(the KaTeX stylesheet, when it is not installed beside the `katex` command),
`OPENSYSML_DOT` (Graphviz), `OPENSYSML_PLANTUML_JAR` and `OPENSYSML_JAVA`
(PlantUML). The repository's `scripts/download-doc-pdf-toolchain.sh` fetches
a pinned WeasyPrint, pandoc, Mermaid CLI, KaTeX, Graphviz and PlantUML jar and
prints the exports to use them; the jar still needs a Java runtime of your
own.

### Deliverable options

| Flag | Effect |
|---|---|
| `-doc-title-page` | A separate title page before the content |
| `-doc-toc` | A table of contents built from the section headings |
| `-doc-number-sections` | Hierarchical section numbers (1, 1.1, ...) |

All three are off by default and shape HTML and PDF alike; `-pdf-title-page`,
`-pdf-toc` and `-pdf-number-sections` are aliases of them.

### Wide tables

The PDF stylesheet keeps every table within the text width: cells wrap
wherever they must, so a long qualified name breaks rather than pushing the
rightmost columns off the page. A table of seven or more columns — a
traceability matrix, say — is placed on landscape pages, together with the
heading and caption that introduce it, while the surrounding pages stay
portrait. The rules use the CSS `:has()` selector, which WeasyPrint — and so
`weasyprint` and `pandoc` — supports; an engine without it keeps the whole
document portrait.

### PDF rendering of inline runs and anchors

Inline runs and cross-reference anchors keep their meaning in PDF. A
paragraph built from `Span`/`Link`/`Ref` runs renders with emphasis, strong
and code styling and working links; a `Ref` anchor becomes an invisible
PDF-native anchor, so an in-document `Ref` is a clickable internal link; and
a grouped table's group key heads each group. All three engines support
internal links: `weasyprint` and `prince` from the HTML's element ids and
fragment hrefs, and `pandoc` from the Markdown itself, whose CommonMark
reader keeps the anchor's raw HTML. Captions are `<caption>` and
`<figcaption>` elements in the HTML the first two read; the pandoc engine
tells a caption from an emphasized paragraph by matching the paragraph ahead
of each table, diagram and formula block against the document's captions in
order, and styles it small.

### Mathematics

Formulas are typeset in the PDF, not printed as LaTeX. The backend lists the
document's distinct formulas — math spans in paragraphs, list items and
definitions, and `Formula` blocks — and runs each once through the KaTeX
command line (`katex`, found on `PATH` or named by `OPENSYSML_KATEX`), which
typesets it as HTML that needs no JavaScript. KaTeX's stylesheet and fonts
are copied beside the page, so the finished PDF embeds the KaTeX faces and
shows the formula as a formula:

- `weasyprint` and `prince` lay out the typeset HTML in the HTML backend's
  own places, an inline formula in its `<span class="sysml-math">` and a
  displayed one in the `<div class="sysml-math">` of its
  `<figure class="sysml-formula">`, centered under its caption;
- `pandoc` reads the Markdown, and its filter replaces each formula pandoc
  parses with the typeset HTML as a raw block or span, linking the same
  stylesheet.

A document without formulas needs no KaTeX, as one without diagrams needs no
Mermaid. An escaped `\$` in prose stays a dollar sign, and a `$` inside a
fenced code or diagram block is never read as math. A missing `katex` stops
the run with the usual `tool-missing` message naming `OPENSYSML_KATEX`; a
missing stylesheet names `OPENSYSML_KATEX_CSS`; and LaTeX KaTeX cannot parse
fails the run with KaTeX's own message quoting the formula, rather than a PDF
with a hole in it.

## Determinism

**Markdown** is fully deterministic: byte-identical output for the same
model, the same held objects and binary. Query results preserve declaration
order unless ordered explicitly, ordering policies are explicit parameters, and rendering
introduces no timestamps, random identifiers or map-order dependence.

**PDF** is deterministic *for a pinned toolchain*. The engine does its
part (it invokes converters with `SOURCE_DATE_EPOCH=0` so they embed a fixed
creation date), but the bytes also depend on the converter version and the
fonts installed on the machine. Two runs on the same machine with the same
toolchain produce identical PDFs; two machines with different WeasyPrint
versions or fonts generally do not. For reproducible PDFs, pin the toolchain
(the download script above is how CI does it).
