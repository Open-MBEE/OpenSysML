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
- Diagrams as fenced ` ```mermaid ` blocks (` ```dot ` for a block stating
  `form = "dot"`, or a pipe table for the `table` kind), with captions in
  emphasis.
- Every table and diagram caption preceded by a `<!-- caption -->` marker
  line, so the caption is distinguishable from an emphasis-only paragraph.
  The marker is metadata of OpenSysML's Markdown dialect: ordinary Markdown
  renderers treat it as a comment and display nothing.
- All model-derived text escaped so it cannot break document structure.
- A single trailing newline, no trailing whitespace.

The output is deterministic: the same model produces byte-identical Markdown
on every run, which is why the repository can keep rendered documents as
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

Identifiers are the same anchors the Markdown writes, so a `Ref` resolves
within the page; in a `-render-documents` set it resolves across pages, whose
file names are the Markdown names with `.html` instead of `.md`. Diagram
blocks embed their Mermaid source in `<pre class="mermaid">`, which a page
that loads Mermaid renders as a diagram and any other page shows as source.
By default the output loads nothing over the network, runs no JavaScript of
its own, and is byte-identical between runs. To have a browser draw the
diagrams, `-html-mermaid cdn` adds a `<script>` loading a pinned Mermaid
release from jsDelivr, and `-html-mermaid <url>` loads it from a URL of your
own; the page keeps the source, so it still reads where the script cannot
load. A fragment has no page shell for the script, so a page embedding one
loads Mermaid itself. A diagram block stating `form = "dot"` embeds its
Graphviz DOT source in `<pre class="dot">` instead; the page never draws it,
and `-html-mermaid` leaves it alone.

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

Internally the engine renders Markdown, converts it to styled HTML, renders
any Mermaid diagrams to SVG with Mermaid CLI (`mmdc`), and hands the result
to an external HTML-to-PDF converter. A diagram block stating `form = "dot"`
is not drawn: the PDF keeps its DOT source under a notice saying so, and no
Graphviz tool is looked for, so a document whose diagrams are all DOT needs
no diagram tool at all.

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
`OPENSYSML_MMDC`, and `OPENSYSML_MMDC_PUPPETEER` (extra Puppeteer
configuration for Mermaid CLI). The repository's
`scripts/download-doc-pdf-toolchain.sh` fetches a pinned WeasyPrint, pandoc
and Mermaid CLI and prints the exports to use them.

### Deliverable options

| Flag | Effect |
|---|---|
| `-doc-title-page` | A separate title page before the content |
| `-doc-toc` | A table of contents built from the section headings |
| `-doc-number-sections` | Hierarchical section numbers (1, 1.1, ...) |

All three are off by default and shape HTML and PDF alike; `-pdf-title-page`,
`-pdf-toc` and `-pdf-number-sections` are aliases of them.

### PDF rendering of inline runs and anchors

Inline runs and cross-reference anchors keep their meaning in PDF. A
paragraph built from `Span`/`Link`/`Ref` runs renders with emphasis, strong
and code styling and working links; a `Ref` anchor becomes an invisible
PDF-native anchor, so an in-document `Ref` is a clickable internal link; and
a grouped table's group key renders in bold above each subtable. All
three engines support internal links: `weasyprint` and `prince` from the
prepared HTML's element ids and fragment hrefs, and `pandoc` from the
Markdown itself, whose CommonMark reader keeps the anchor's raw HTML.

## Determinism

**Markdown** is fully deterministic: byte-identical output for the same
model and binary. Query results preserve declaration order unless ordered
explicitly, ordering policies are explicit parameters, and rendering
introduces no timestamps, random identifiers or map-order dependence.

**PDF** is deterministic *for a pinned toolchain*. The engine does its
part (it invokes converters with `SOURCE_DATE_EPOCH=0` so they embed a fixed
creation date), but the bytes also depend on the converter version and the
fonts installed on the machine. Two runs on the same machine with the same
toolchain produce identical PDFs; two machines with different WeasyPrint
versions or fonts generally do not. For reproducible PDFs, pin the toolchain
(the download script above is how CI does it).
