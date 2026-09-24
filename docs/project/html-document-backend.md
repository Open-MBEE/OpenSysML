# HTML document backend — design

Status: **implemented** — `docrender.HTML`, `-doc-form html`, the stylesheet options and linked
HTML sets ship, and the PDF engines that read HTML are handed this backend's page with the PDF
backend's print stylesheet (see [Rendering a document as HTML](../reference/cli.md#rendering-a-document-as-html)
and [Rendering a document as PDF](../reference/cli.md#rendering-a-document-as-pdf) for the
user-facing surface). This page records the design agreed for rendering documents as HTML
directly from the document IR, the class and attribute vocabulary that makes the output styleable,
and what the change did to the PDF backend.

## The problem

The problem this page set out to solve, as it stood before either step landed: `-doc-form` wrote Markdown or PDF, and there
was no HTML form at all. HTML did exist inside
the toolchain, but only as an intermediate for the PDF converters that read HTML — WeasyPrint
and Prince — and it was built the long way round: `internal/doc/docpdf/markdown.go` re-parsed
docrender's Markdown back into flat presentation blocks (heading, paragraph, caption, table,
list, mermaid, anchor) and `internal/doc/docpdf/html.go` wrote those blocks as a page with an
inline print stylesheet. That intermediate was a deliberate choice — it kept the PDF layer
independent of the document IR — and it had two consequences.

The first is that the markup carries no model information. Its only hooks are `title-page`,
`nav.toc` with a flat `toc-2`/`toc-3` depth class, `p.caption`, `span.section-number`,
`figure`, headings with a positional `id="sec-1-2"`, and the `<a id>` anchors that survive as
raw HTML in the Markdown. A stylesheet cannot ask for requirement tables, part rows, or
reference links, because nothing in the output says which is which. Everything the model knew
— the content node's kind and declared name, the query behind a table, the element and element
kind behind a row, whether a link points at a URL or at another content node — is discarded at
the Markdown boundary. Markdown is a lossy encoding of the IR, so no amount of work inside
`docpdf` can recover it.

The second was that the intermediate had to reconstruct what it lost, and that showed. A caption
is a paragraph that happens to be one emphasis run, so docrender wrote an HTML comment marker
(`<!-- caption -->`) ahead of it and docpdf recognized the marker; the pandoc path rewrote the
marked line as `[…]{.caption}` so the Markdown reader styled it the same way. Table cells fold
newlines to a literal `<br>` that the block parser had to split on and preserve, distinguishing
it from the escaped metacharacters around it. None of this was wrong, but all of it was a
consequence of going through Markdown twice.

## What the IR already carries

Nothing new has to be computed. `internal/doc/docir` is a backend-agnostic tree with
provenance on every node, and it already holds everything a stylesheet would want to select on:

| Available on the IR | Where |
|---|---|
| Content kind: section, paragraph, table, list, diagram | `Content.Kind` |
| Declared name of the content node, its title, its caption | `Content.Name`, `Title`, `Caption` |
| Stable anchor of a referenced node | `Content.Anchor`, `docir.AnchorFor` |
| The query behind a query-backed node, and that query's declaration | `Content.Query`, `QueryOrigin` |
| Projected column names, with the expression that projected each | `queryexec.Column` |
| The selected element of every table row and list item | `queryexec.Row.Element` → `*symbols.Symbol` |
| The kind of every element: `partDef`, `requirementUsage`, … | `symbols.Symbol.Kind` (63 named kinds) |
| The scalar kind of every projected value: element, string, integer, real, boolean, infinity | `queryexec.Value.Kind` |
| Run kind: plain, emphasis, strong, code, link, reference | `docir.TextRun.Kind` |
| A reference's target anchor and target document | `TextRun.Target`, `TargetDocument` |
| List style: bullet or numbered | `Content.Style` |
| Diagram rendering kind and flow direction | `Content.Rendering`, `Direction` |
| Source declaration behind every node, run, row and cell | `Origin` on each |

So the design is not "add information to the pipeline". It is "add a backend that consumes the
information already there, instead of a backend that consumes Markdown".

## The design in one paragraph

A new `docrender.HTML` renders a `docir.Document` straight to HTML, as a sibling of
`docrender.Markdown` and under the same rules — IR only, deterministic, byte-identical for
byte-identical input, every value escaped so no content can corrupt the structure. The markup
is ordinary semantic HTML: `<article>`, nested `<section>`, `<h1>`–`<h6>`, `<p>`, `<table>`
with `<caption>`, `<thead>`/`<tbody>`, `<th scope="col">`, `<ul>`/`<ol>`, `<figure>` with
`<figcaption>`, `<nav>` for the table of contents, `<em>`, `<strong>`, `<code>`, `<a>`. Model
information rides along as a `sysml-` class on each node and `data-` attributes naming the
model facts — the content kind, the declared name, the query, the element and its kind, the
projected column, the value kind — so a stylesheet can select structurally
(`article.sysml-document > section > h2`) or semantically
(`tr[data-element-kind="requirementUsage"]`) without the generator making layout decisions. The
CLI grows `-doc-form html`, `-render-documents` gains the same form for a linked set, and the
PDF converters that read HTML are pointed at this backend, which retires the Markdown
re-parsing layer and the caption marker with it.

## The markup

A worked shape, for a document with one section, a grouped table, a list and a diagram:

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Mass Report</title>
<style>@layer opensysml; @layer opensysml { .sysml-document { --sysml-measure: 38rem; … } }</style>
</head>
<body>
<article class="sysml-document" data-document="Reports::MassReport">
<h1 class="sysml-title">Mass Report</h1>
<nav class="sysml-toc" aria-label="Contents">
<h2>Contents</h2>
<ol>
  <li><a href="#Reports-MassReport-Airframe">Airframe</a>
    <ol><li><a href="#Reports-MassReport-Airframe-Budget">Mass budget</a></li></ol>
  </li>
</ol>
</nav>
<section class="sysml-section" id="Reports-MassReport-Airframe" data-content="section" data-name="Airframe">
<h2><span class="sysml-section-number">1</span> Airframe</h2>
<p class="sysml-paragraph" data-content="paragraph">
  The airframe carries <strong>every</strong> structural mass, per
  <a class="sysml-ref" href="#Reports-MassReport-Airframe-Budget">the mass budget</a>.
</p>
<table class="sysml-table" data-content="table" data-name="Budget" data-query="Queries::PartsByStage" data-group-by="stage">
<caption class="sysml-caption">Masses by stage</caption>
<thead>
<tr><th scope="col" data-column="stage">stage</th><th scope="col" data-column="mass">mass</th></tr>
</thead>
<tbody class="sysml-group" data-group="stage" data-group-key="Launch">
<tr class="sysml-group-heading"><th scope="rowgroup" colspan="2">stage: Launch</th></tr>
<tr class="sysml-row" data-element="Vehicles::Booster" data-element-kind="partUsage">
  <td class="sysml-cell" data-column="stage" data-value-kind="string">Launch</td>
  <td class="sysml-cell" data-column="mass" data-value-kind="real">
    <span class="sysml-value" data-value-kind="real">1200.0</span>
  </td>
</tr>
</tbody>
</table>
<ul class="sysml-list" data-content="list" data-query="Queries::OpenRequirements">
<li class="sysml-item" data-element="Reqs::MassLimit" data-element-kind="requirementUsage">
  <code>MassLimit</code> — total mass shall not exceed 1500 kg
</li>
</ul>
<dl class="sysml-definitions" data-content="definitions" data-name="requirementText" data-query="Queries::OpenRequirements">
<div class="sysml-entry" data-element="Reqs::MassLimit" data-element-kind="requirementUsage">
  <dt class="sysml-term">HLR-R001</dt>
  <dd class="sysml-description">The total mass shall not exceed 1500 kg.</dd>
</div>
</dl>
<figure class="sysml-diagram" data-content="diagram" data-name="Assembly" data-view="Views::AssemblyView" data-diagram-kind="interconnection" data-direction="LR">
<pre class="mermaid">flowchart LR
  …</pre>
<figcaption class="sysml-caption">Airframe assembly</figcaption>
</figure>
</section>
</article>
</body>
</html>
```

### The vocabulary

One class per node kind, in a `sysml-` namespace, and `data-` attributes for the model facts:

| Node | Element and class | Attributes |
|---|---|---|
| Document | `article.sysml-document` | `data-document` — the document's fully-qualified name |
| Title | `h1.sysml-title` | |
| Section | `section.sysml-section` wrapping `h2`–`h6` | `data-content="section"`, `data-name` |
| Paragraph | `p.sysml-paragraph` | `data-content`, `data-query` when query-backed |
| Table | `table.sysml-table`, caption as `<caption>` | `data-content`, `data-name`, `data-query`, `data-group-by` |
| Table group | `tbody.sysml-group` | `data-group`, `data-group-key` |
| Row | `tr.sysml-row` | `data-element`, `data-element-kind` |
| Cell | `td.sysml-cell` | `data-column`, `data-value-kind` |
| One value inside a multi-valued cell | `span.sysml-value` | `data-value-kind` |
| Separator between those values | `span.sysml-separator` | |
| Group key of a grouped table | `span.sysml-group-key` | |
| Column header | `th[scope=col]` | `data-column` |
| List | `ul.sysml-list` / `ol.sysml-list` | `data-content`, `data-query` |
| List item | `li.sysml-item` | `data-element`, `data-element-kind` |
| Definitions | `dl.sysml-definitions` | `data-content`, `data-name`, `data-query` |
| Definitions entry (one query row) | `div.sysml-entry` wrapping `dt.sysml-term` and `dd.sysml-description` | `data-element`, `data-element-kind` |
| Diagram | `figure.sysml-diagram`, caption as `<figcaption>` | `data-content`, `data-name`, `data-view`, `data-diagram-kind`, `data-direction` |
| Emphasis, strong, code run | `em`, `strong`, `code` | |
| Link run | `a.sysml-link` | `href` |
| Reference run | `a.sysml-ref` | `href`, `data-document` for a cross-document reference |
| Element-valued text | `span.sysml-element` | `data-element`, `data-element-kind` |
| Table of contents | `nav.sysml-toc` with nested `<ol>` | |

Decisions, each with its reason:

- **Semantic HTML first, classes second.** The structure is the tags: nested `<section>`
  elements rather than a flat run of headings, `<caption>` for a table's caption and
  `<figcaption>` for a diagram's, `<th scope="col">` for column headers, `<nav>` for the
  contents. That is what makes the output readable to a screen reader, to a static-site
  generator, and to a stylesheet nobody wrote for this project. Classes and `data-`
  attributes then add what HTML has no tag for — that this table is requirements and that
  row is a part — instead of substituting for structure HTML already has.
- **Nested sections, so depth is a selector.** Markdown can only express depth as a heading
  level, which is why today's intermediate saturates at `h6` and the contents list carries a
  `toc-2`/`toc-3` class to fake indentation. Real nesting lets `section section h3` and
  `nav.sysml-toc ol ol` do that work, and the heading level inside a section is still emitted
  (saturating at `h6`, matching Markdown) so a document remains navigable by heading.
- **`data-` attributes for model facts, not classes.** Element kinds are a 63-name
  vocabulary and columns are user-named; both would turn into unbounded class soup. As
  attributes they stay selectable (`[data-element-kind="requirementUsage"]`,
  `td[data-column="mass"]`), machine-readable for anything post-processing the HTML, and they
  keep the class namespace small enough to document in a table like the one above.
- **The kind vocabulary is the symbol table's, verbatim.** `data-element-kind` writes
  `symbols.SymbolKind.String()` — `partDef`, `requirementUsage`, `stateUsage` — rather than
  inventing presentation names. The precedent is `internal/semantic/highlight`, which emits the
  LSP's standardized token names for exactly this reason: one vocabulary, defined elsewhere,
  that a consumer can look up.
- **Anchors are the IR's stable identifiers, not positional numbers.** Sections get
  `id="<anchor>"` from `docir.AnchorFor`, so a link into a document survives inserting a
  section ahead of it; the current `sec-1-2` heading ids do not. A section the IR gave no
  anchor gets none, and a reference run keeps pointing at its target's anchor exactly as in
  Markdown.
- **Section numbers stay text.** CSS counters are the idiomatic way to number sections, and
  they cannot be read back for the contents list outside print engines that implement
  `target-counter`. Numbering is therefore emitted as `span.sysml-section-number` when asked
  for, which every consumer sees identically, and a stylesheet that would rather use counters
  can hide the span.
- **No script, no network reference.** The default page is self-contained: one inline
  stylesheet, no CDN, no JavaScript. A rendered document is an artifact that has to open
  from a file on a machine with no network, and a generated page that silently fetches a
  script is not that. Loading Mermaid is therefore opt-in: `-html-mermaid cdn` or
  `-html-mermaid <url>` adds one `<script src>` before `</body>` — nothing is bundled, the
  browser fetches the script under Mermaid's own terms — and a fragment refuses it, since the
  embedding page owns the shell.
- **Every separator is an element, not bare punctuation.** A multi-valued cell renders each
  value in `span.sysml-value` with the `, ` between them in `span.sysml-separator`, and a
  group key gets `span.sysml-group-key`. Markdown has to join those with punctuation; HTML
  does not, and a theme that wants stacked values or a bullet between them should not have
  to strip a comma the generator baked in. The punctuation is still real text, so the
  document reads correctly with styling switched off entirely.

### Diagrams

A diagram's rendering comes from the view engine as Mermaid source. The HTML backend writes
that source in `<pre class="mermaid">` inside the `<figure>` by default — it is exactly what
Mermaid's own client-side renderer looks for, so a site that already loads Mermaid renders it
with no further work, and a site that does not shows the source rather than nothing. A
standalone page asked to with `-html-mermaid` loads that renderer itself, from the pinned
jsDelivr release or a URL the caller names, and configures it with the text and edge limits
its largest chart fits under, since Mermaid's defaults refuse a large diagram of a large model
rather than draw it. When pre-rendered images are supplied, the `<pre>`
is replaced by `<img>` with the caption as its `alt` text; that is the path the PDF converters
use, since no print engine runs Mermaid. Table-kind views keep rendering as a table, as they do
in Markdown.

Supplying the images stays out of `docrender`: rendering them means running `mmdc` as a
subprocess, which is `docpdf`'s job and must not become a dependency of a pure renderer. So
`docrender` exposes the diagram sources in document order and accepts the resulting image
names in the same order — the ordering convention `docpdf` already uses internally today.

### Styling and overriding it

The default stylesheet exists so a rendered document looks like a document when it is opened,
and it is written on the assumption that it will be overridden. Three properties of the markup
make overriding it work, and they are structural decisions, not stylesheet decisions.

**The default stylesheet is in a cascade layer; user CSS is not.** Everything the backend
emits is wrapped in `@layer opensysml { … }`, and the layer is declared first. Unlayered CSS
always wins over layered CSS regardless of specificity, so a reader's
`.sysml-table { border: none }` beats the default's rule without matching its specificity and
without `!important` — and it keeps winning when the default stylesheet is rewritten in a later
release. This is the single most important decision in this section: without it, every default
rule is a specificity negotiation, and the usual outcome is a wall of `!important` in the
reader's stylesheet.

**Every value the default stylesheet uses is a custom property.** Fonts, the text measure,
spacing, rule and caption colors, table zebra striping are read from `--sysml-*` properties
declared on `.sysml-document`:

```css
/* re-theme without replacing anything */
.sysml-document {
  --sysml-font-body: "Public Sans", sans-serif;
  --sysml-measure: 42rem;
  --sysml-rule: #c8102e;
}
/* or restyle structurally — unlayered, so it wins */
tr[data-element-kind="requirementUsage"] { background: #fff8e1; }
.sysml-table .sysml-separator { display: none; }
.sysml-table .sysml-value { display: block; }
```

The token set is part of the documented contract, the same way the class vocabulary is: a
reader who only wants their organization's typography and rule color should never have to read
the default stylesheet, let alone fork it.

**No `style` attributes, no styling-only markup, no styling on `id`.** The backend never emits
an inline `style`, because an inline style cannot be overridden from a stylesheet at all. It
emits no wrapper `<div>` that exists only to give CSS a box — every element in the output is
there because the document model has that node — and it never asks a stylesheet to match an
`id`, since ids are model anchors and matching them would tie a theme to a particular
document. Class names are stable and unprefixed by depth: nesting expresses depth
(`section section .sysml-table`), so a theme is never forced to enumerate levels.

The flags follow from that:

- **`-html-theme <name>`** layers one of the bundled themes (`modern`, `print`, `report`;
  `default` names the default sheet alone) after the default sheet, in the same `<style>` and
  the same `opensysml` layer. A theme is written against the `--sysml-*` tokens and the class
  vocabulary and scopes every selector under `.sysml-document`, so it changes the look without
  changing the cascade contract: unlayered reader CSS still wins over default and theme alike.
  The themes live in `docrender/themes/*.css` and are embedded; the file names are the theme
  names, so adding a theme is adding a file.
- **`-html-css <file-or-url>`**, repeatable. A file's contents are inlined, so the artifact
  stays self-contained; a URL becomes a `<link>` for a site that serves its own. Each is
  emitted after the default, unlayered, in the order given.
- **`-html-no-default-css`** drops the built-in stylesheet entirely, for a reader who wants to
  start from nothing rather than from a layer. A theme needs the default under it, so the two
  are refused together.
- **`-html-default-css`** writes the built-in stylesheet to stdout, so "start from ours and
  edit" needs no source dive. It is a printing mode like the other informational flags, not a
  rendering option; `-html-theme` is the one rendering flag it takes, to write a theme's whole
  sheet instead.
- **`-html-fragment`** writes the `<article>` alone — no `<!DOCTYPE>`, `<head>` or stylesheet
  — for embedding in a site that brings its own CSS.
- **`-render-documents -doc-form html`** writes one shared `sysml-document.css` (default sheet
  plus theme, when one is named) beside the files and links it, rather than inlining the same bytes into every document, so a set has
  one stylesheet to override and it is a file the reader can replace on disk. `-html-css`
  additions are linked alongside it, in order.

The print stylesheet the PDF path needs — `@page` margins, page counters, page breaks — stays
with the PDF backend, where its `@page` rules belong, and is layered the same way so
`-html-css` works for PDF too.

## Surfaces

- **`-doc-form html`**, alongside `markdown` and `pdf`. Unlike PDF it needs no external tool
  and writes text, so it works on stdout and does not require `-o`.
- **`-render-documents <dir> -doc-form html`** writes a linked HTML set.
  `docrender.DocumentFileName` currently hardcodes `.md`; it takes the extension as a
  parameter, and cross-document reference destinations follow the form being rendered, so a
  set of HTML files links to `.html` and a set of Markdown files keeps linking to `.md`.
- **`-html-css`, `-html-no-default-css`, `-html-default-css` and `-html-fragment`**, refused
  for the other forms the way the `-pdf-*` flags already are, except `-html-css`, which the
  PDF form accepts too since its converters read the same HTML.
- **The shared deliverable options.** A title page, a contents list and section numbering are
  not PDF-specific — they are exactly what an HTML deliverable wants too — but they are
  spelled `-pdf-title-page`, `-pdf-toc` and `-pdf-number-sections` today. The proposal is to
  accept `-doc-title-page`, `-doc-toc` and `-doc-number-sections` for both forms and keep the
  `-pdf-*` spellings working as documented aliases, so no existing script breaks. **Open
  decision:** whether to keep the aliases indefinitely or deprecate them for 1.0.
- **REPL and service.** `%render-document` keeps printing Markdown; a terminal has no use for
  markup. The gRPC `RenderDocument` request grows a form field defaulting to Markdown, so the
  Python client can ask for HTML, and the LSP's document preview keeps its Markdown, which is
  what the protocol's preview surface renders.

## What this does to the PDF backend

Once `docrender` writes HTML from the IR, the intermediate in `docpdf` is redundant and its
losses are unnecessary. `docpdf.Render` takes the evaluated `docir.Document`; the HTML-input
converters (WeasyPrint, Prince) are handed the backend's page with the print stylesheet, and
the Markdown-input converter (pandoc) keeps receiving `docrender.Markdown`'s text, so all
three engines keep working. `internal/doc/docpdf/markdown.go`, `html.go` and `inline.go` — the
block parser, the page writer and the Markdown-inline-to-HTML translator — are gone, and with
them the caption marker convention in `docrender.Markdown`, the `[…]{.caption}` rewrite for
pandoc, and the `<br>` fold in table cells. `docpdf` keeps what it is actually for: locating
and running external tools, drawing diagrams with `mmdc`, typesetting formulas with KaTeX, and
the typed errors for a missing or failing one.

What a backend may draw or typeset out of process is listed by `docrender` from the IR —
`Diagrams` (graph-shaped diagram sources in the requested form, table-kind views excluded),
`Formulas` (distinct math, keyed as the HTML backend writes it) and `Captions` (table, diagram
and formula captions in document order) — and handed back through `HTMLOptions.DiagramImages`
and `HTMLOptions.Math`, the one seam where a diagram block becomes its rasterized image and a
formula its typeset HTML. Rasterizers for other diagram forms plug into that seam beside
`mermaid.go` without touching the renderer.

The print stylesheet is `internal/doc/docpdf/print.css`: `@page` geometry, the page counter, print
fonts and breaks, and the print treatment of the `sysml-*` classes, in `@layer opensysml-print`
declared after `@layer opensysml`. The cascade order for an HTML-input engine is the backend's
default sheet and theme, the print layer, KaTeX's stylesheet when the document has formulas,
then the reader's `-html-css` sheets unlayered — so the override contract of § *Styling and
overriding it* holds for PDF byte for byte, and `-html-theme` and `-html-no-default-css` mean
for PDF what they mean for HTML. Pandoc's own HTML carries pandoc's structure rather than the
backend's classes, so the pandoc engine keeps a stylesheet of its own (`pandoc.css`), attaches
`-html-css` sheets in its page's head after it, and rejects `-html-theme` and
`-html-no-default-css` with a typed error.

A sheet's relative `url()` and `@import` references resolve for PDF as they do for HTML:
against the output's own directory. The converters run in a temporary working directory, so
the PDF backend hands each the PDF's directory as the page's base — WeasyPrint's `--base-url`,
Prince's `--baseurl`, pandoc's `--resource-path` with `--base-url` for the engine it drives —
and references its own generated files (diagram images, the KaTeX stylesheet) by absolute
file URL so the base does not move them. A `docpdf.Render` caller names that directory in
`Options.BaseDir`; the CLI passes the `-o` path's.

The caption marker was the one deletion visible in existing output: an HTML comment in rendered
Markdown, so removing it changed Markdown goldens by that line only, without changing how any
Markdown renderer displays them. Measurement settled the pandoc question: pandoc has no syntax
that tells a caption from a paragraph that happens to be emphasized, so the pandoc converter's
generated Lua filter (the one that also swaps in the drawn diagrams and typeset formulas)
marks a caption when an emphasized paragraph matches the next of the document's caption texts,
in order, and the block after it is captionable — a table, a display formula, a diagram fence,
the rendering comment a table-kind diagram opens with, or a grouped table's key ahead of its
first subtable. An emphasized paragraph elsewhere stays body prose. Nothing of this reaches the
Markdown output.

## Test contract

- **Golden HTML** beside the existing Markdown goldens in `internal/doc/docrender/testdata`,
  covering the worked example and the linked set, with the same `-update` discipline.
- **Well-formedness**, not just golden equality: every golden is parsed with
  `golang.org/x/net/html` (already a dependency) and the tree asserted — sections nest,
  headings never skip a level, every `data-element-kind` is a name `symbols.SymbolKind`
  answers to, every `href="#…"` resolves to an `id` in the same document, and every
  cross-document `href` names a file the set contains.
- **Escaping**, as adversarial cases: element names, column names, captions and cell values
  containing `<`, `&`, `"`, `'`, a `</script>` sequence and a newline must appear as text and
  must not be able to close an attribute or introduce an element. This is the property that
  matters most, because unlike Markdown a broken escape here is an injection, not a typo.
  A supplied stylesheet is inlined, so `</style>` in it is the same class of hazard and gets
  the same treatment.
- **The override contract**, as assertions on the goldens rather than prose: no `style`
  attribute appears anywhere in the output, every class emitted is one the documented
  vocabulary lists, the default stylesheet is wrapped in `@layer opensysml` and declares the
  layer before using it, every declaration in it resolves through a `--sysml-*` property, and
  `-html-css` content lands after the layer, unlayered. A regression in any of these silently
  breaks reader stylesheets that the project never sees, which is why they are gates and not
  documentation.
- **Determinism**, rendering twice and comparing bytes, including for grouped tables and
  multi-document sets.
- **The PDF path unchanged**, per engine: the `docpdf` tests assert the prepared input each
  converter is handed (the backend's page, or the Markdown text with its filter) and the
  integration tests against real tools render the fixtures; the print stylesheet meets the
  override-contract assertions the HTML tests make of the default sheet (no `style`
  attributes, layered, `--sysml-*` tokens).
- **CLI surface**: the new form and flags, their conflicts, and stdout versus `-o`.
- **`docs/project/spec-compliance.md`** gains the rows for the form, the vocabulary and the
  linked HTML set, with honest status flags.

## Work plan

1. **The backend and its form.** `docrender.HTML` with the vocabulary above, the extension
   parameter on `DocumentFileName`, `-doc-form html`, `-render-documents` for HTML, the
   layered and tokenized default stylesheet with `-html-css`, `-html-no-default-css`,
   `-html-default-css` and `-html-fragment`, the shared deliverable options, goldens and the
   test contract, and the user documentation (`docs/reference/cli.md`, `docs/manual/outputs.md`,
   `docs/manual/interfaces.md`, the guide's document pages).
2. **The PDF migration** — landed. The HTML-input engines read the backend's page, the print
   stylesheet is the PDF backend's own `print.css`, the block parser, page writer, inline
   translator and the caption marker are gone, and `RenderDocument` carries a `form` field
   (`markdown` or `html`, capability `render_document_html`) with the Python client's
   `render_document(..., form=)` behind it.

Step 1 stood alone: it delivered the HTML form without touching PDF output. Step 2 was a
refactor whose user-visible changes are the caption comment leaving Markdown output and the
HTML stylesheet options reaching `-doc-form pdf`.

## Known limitations

- **No HTML for the REPL or the LSP preview.** Both stay Markdown by choice, stated above.
- **Client-side Mermaid only.** A page opened without a Mermaid script shows diagram source
  rather than a diagram; `-html-mermaid` has the page load the script, which still needs a
  browser with access to the script's URL. Pre-rendered images are available through the PDF
  path's machinery, but wiring `mmdc` into the HTML form — an `-html-diagrams svg` option — is
  deliberately out of scope and left as follow-on work.
- **PDF only through the CLI.** The service's `RenderDocument` offers `markdown` and `html`;
  PDF needs the CLI's external converter toolchain and is not a service form.
- **A PDF is one document.** `-render-documents` refuses `-doc-form pdf`, so a `Ref` into
  another document links to that document's page file name — as the HTML and Markdown forms
  write it — which no PDF beside it carries. In-document references are working links.
- **Pandoc's captions are matched, not marked.** Because the Markdown carries no caption
  marker, the pandoc engine identifies captions by text and position; an emphasized paragraph
  whose text equals the next caption and which sits directly ahead of that caption's block
  would be styled as the caption. The HTML-input engines carry captions as `<caption>` and
  `<figcaption>` and have no such ambiguity.
- **Three bundled themes, no house style.** `modern`, `print` and `report` are generic looks
  built on the layer and the token vocabulary; an organisation's house style is still a
  `-html-css` sheet of its own, and no theme is loaded from the network.
- **The class and token vocabulary becomes a compatibility surface.** Once readers write
  stylesheets against `sysml-` classes, `data-` attributes and `--sysml-*` properties,
  renaming one breaks them silently. It is documented in `docs/reference/` as a contract and
  changes belong in `CHANGELOG.md`, which is a cost this design accepts deliberately.
