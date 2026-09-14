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
