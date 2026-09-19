---
name: testing-doc-pdf
description: How to end-to-end test the sysml PDF document backend (internal/doc/docpdf + -doc-form pdf) on Linux — provisioning the pinned WeasyPrint/mermaid/KaTeX/Graphviz/PlantUML toolchain, rendering the worked example and the docrender goldens, and proving inline runs, anchors, links, LaTeX formulas and every diagram form render rather than appearing literal.
---

# Testing the PDF document backend

## Toolchain
- Provision once: `./scripts/download-doc-pdf-toolchain.sh` installs a pinned WeasyPrint venv, pandoc, mermaid-cli, KaTeX, Graphviz (unpacked from the upstream Ubuntu .debs; its `neato` plugin needs the host's `libgts-0.7-5`) and the PlantUML jar under `build/doc-pdf/`. PlantUML needs a `java` on `PATH` (or `OPENSYSML_JAVA`).
- Export before any PDF render (env does NOT persist between exec tool calls even in the same shell session — re-export in the same command line):
  - `OPENSYSML_WEASYPRINT=$PWD/build/doc-pdf/weasyprint/bin/weasyprint`
  - `OPENSYSML_MMDC=$PWD/build/doc-pdf/mermaid/node_modules/.bin/mmdc`
  - `OPENSYSML_MMDC_PUPPETEER=$PWD/build/doc-pdf/mermaid/puppeteer.json`
  - `OPENSYSML_PANDOC=$PWD/build/doc-pdf/pandoc-3.10.2/bin/pandoc` (for `-pdf-engine pandoc`)
  - `OPENSYSML_KATEX=$PWD/build/doc-pdf/katex/node_modules/.bin/katex` (only a document with formulas needs it)
  - `OPENSYSML_DOT=$PWD/build/doc-pdf/graphviz/bin/dot` (for `-diagram-form dot`; absent, DOT blocks stay as source under a notice)
  - `OPENSYSML_PLANTUML_JAR=$PWD/build/doc-pdf/plantuml/plantuml-1.2026.8.jar` (for `-diagram-form plantuml`; absent, PlantUML blocks stay as source under a notice)
- `go test -run Installed ./internal/doc/docpdf` runs the real-toolchain integration tests (they skip per missing tool); the rest of the package's tests use fake tools and need nothing installed. `OPENSYSML_REQUIRE_PDF_TOOLCHAIN=1` turns every such skip but Prince's into a failure — set it once the script has run, so a misconfigured variable cannot pass as a skip. This is how the CI `pdf-toolchain` job runs them.
- Diagram forms: render the worked example with `-diagram-form dot` and `-diagram-form plantuml` too; the PDF must contain no "did not draw" notice (pypdf `extract_text`) and the page's XObjects grow by one per diagram. With `OPENSYSML_DOT=/nonexistent` the render still succeeds and the notice names `OPENSYSML_DOT`. A `// layout: neato -n` block (the DOT writer emits one for a positioned diagram) must run neato: `dot -Kneato` failing with "no layout engine support" means the GTS library is missing.

## Rendering
- Worked example: `bin/sysml docs/manual/examples/observatory.sysml -render-document Observatory::MassReport -doc-form pdf -pdf-title-page -pdf-toc -pdf-number-sections -o /tmp/observatory.pdf`. It exercises emphasis, code span, external Link, Ref (`#breakdown`), a standalone `<a id="breakdown"></a>` anchor line, grouped table (`**zone: ...**` headings), numbered list and two mermaid diagrams.
- Richest input: `internal/doc/docrender/testdata/telescope_report.golden.md` covers every inline construct including escaped `\*`/`\|`/`` \` `` prose and reference links. Drive it via a throwaway `tmp_render_main.go` at repo root calling `docpdf.Render(md, "weasyprint", docpdf.Options{...})` with `go run` (delete afterwards).
- Markdown regression: `-o foo.md` must diff-equal `docs/manual/examples/observatory.md`. Note `-doc-form` takes `markdown` or `pdf` (not `md`).
- Captions: docrender writes `<!-- caption -->` on its own line immediately before every caption's `*text*` line; docpdf only treats marked lines as captions (`p.caption`, small/gray), so a bare `*text*` paragraph must stay body-sized. To test the distinction, author a Document with a Paragraph whose single Span has `style = "emphasis"` alongside a captioned Table (model needs `private import DocumentQueries::*;` and `private import KerML::Root::Element;`; queries take `in root : Element` bound per-Table). Font-level proof: pypdf `extract_text(visitor_text=...)` size param — caption renders ~12.67pt vs 14.67pt body. Verify under BOTH engines (`-pdf-engine weasyprint` and `pandoc`): the pandoc path rewrites markers to `[*text*]{.caption}` spans, so also assert extracted text contains no `{.caption}` or `[*`.
- Formulas: `internal/doc/docrender/testdata/math_report.sysml` → `Optics::OpticsReport` has an inline `$m \propto D^{2.5}_{\text{eff}}$` beside an escaped prose `\$`, a captioned `Formula` that is a `Ref` target, a multi-line display block, a `\$` inside math, and query-driven math list items. Render under both engines and check with pypdf: the page's `/Font` `/BaseFont`s must include `KaTeX_Main`, `KaTeX_Math-Italic` (and `KaTeX_Size3` for the tall parenthesis); extracted text must contain `∝`, `π`, `θ`, `λ` and none of `$$`, `\propto`, `\frac`, `_{`, `^{`, `katex`, `<span`. Text from absolutely-positioned KaTeX spans extracts out of reading order (superscripts on their own lines) — that is pypdf, not a bug. For a visual check, `pip install pypdfium2` into the WeasyPrint venv and `PdfDocument(path)[0].render(scale=1.3).to_pil().save(png)`.
- KaTeX is run with `--format html` (no MathML), deliberately: the MathML branch carries the LaTeX source as `<annotation>` text, which paged-media engines would set literally. A malformed formula (`\frac{a}`) fails the run with `KaTeX parse error: ...` from the CLI's stderr, not a PDF with a gap.
- `docpdf.Render`'s second argument is the engine NAME (`"weasyprint"`, `"pandoc"`, `"prince"`), not a binary path; binaries come from the OPENSYSML_* env vars.
- A `<!-- caption -->` marker not followed by a fully-emphasized line is a typed `dangling-caption` error ("caption marker without a caption line after it").

## Verifying the PDF is real, not literal
- Install pypdf into the WeasyPrint venv (`build/doc-pdf/weasyprint/bin/pip install pypdf`) — the box's system python is a venv where `pip --user` fails.
- Text assertions: extracted text must contain no `<a id`, `**`, `](`, or backticks; escaped source (`\*not\*`) legitimately leaves literal `*` — check against the source before calling it a failure.
- Font-level proof of styling (visual zoom can be ambiguous): use `page.extract_text(visitor_text=...)` and check `/BaseFont` — emphasis → `DejaVu-Serif-Oblique`, strong/group headings → `DejaVu-Serif-Bold`, code → a mono face.
- Link proof: iterate `page['/Annots']` — external links are `/URI` actions; Ref links and TOC entries must be internal `/Dest` (e.g. `breakdown`, `sec-1`), not `/URI` to `#...`.
- Visual/recorded: open the PDF in Chrome (`google-chrome --start-maximized file:///tmp/x.pdf`); typing a file:// URL into an already-focused omnibox may get treated as a search — click the omnibox, ctrl+a, then type. Clicking a Ref link should scroll to the target section.
