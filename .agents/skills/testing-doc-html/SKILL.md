---
name: testing-doc-html
description: How to end-to-end test the sysml HTML document backend (internal/doc/docrender/html.go + `-doc-form html`) in a real browser on Linux — the exact CLI invocations, which fixtures exercise which feature, how to prove the CSS-override/cascade-layer contract visually, and the traps that make an HTML render look fine when it is not.
---

# Testing the HTML document backend

## Build and fixtures

```bash
make build                      # bin/sysml
mkdir -p /tmp/ht && cp internal/doc/docrender/testdata/telescope_report.sysml \
    internal/doc/docrender/testdata/linked_reports.sysml /tmp/ht/
```

- `telescope_report.sysml` → document `Observatory::MassReport`. The single best fixture:
  sections at three depths, a captioned table, a `groupBy` table (one `<tbody>` + group
  heading row per group), ordered/unordered/`code`-styled lists, inline spans, external
  `Link`s, intra-document `Ref`s, two Mermaid `Diagram` figures, and deliberately tricky
  names (`baffle|shroud *tricky*`, `<b>&plain</b>`) that double as escaping cases.
- `linked_reports.sysml` → `Observatory::SystemReport` + `Observatory::Mass Appendix`, the
  cross-document link pair. File names are percent-ish escaped:
  `Observatory-Mass.20Appendix.html`. See `internal/doc/docrender/html_crossdoc_test.go`
  for the exact expected `href`/anchor shapes.
- `math_report.sysml` → `Optics::OpticsReport`, the formula fixture: an inline `style = "math"`
  span next to a prose `$`, a captioned `Formula` block that a `Ref` targets, a display formula
  containing `\$`, and math list items driven by a query.

## Invocations that matter

```bash
bin/sysml m.sysml -render-document Observatory::MassReport -doc-form html -o report.html
bin/sysml m.sysml -render-document Observatory::MassReport -doc-form html \
    -doc-title-page -doc-toc -doc-number-sections -o report_full.html
bin/sysml m.sysml -render-document Observatory::MassReport -doc-form html \
    -html-css theme.css -o report_theme.html      # file is inlined; a URL is <link>ed
bin/sysml m.sysml -render-document Observatory::MassReport -doc-form html -html-fragment -o frag.html
bin/sysml math_report.sysml -render-document Optics::OpticsReport -doc-form html -html-math cdn -o math.html
bin/sysml m.sysml -render-document Observatory::MassReport -doc-form html -html-no-default-css -o nocss.html
bin/sysml linked_reports.sysml -render-documents site -doc-form html   # pages + sysml-document.css
bin/sysml -html-default-css -o default.css                             # no model needed
```

Flags are declared in `cmd/sysml/main.go` (~lines 325-336); the combination rules live in
`cmd/sysml/render_document.go` (`documentForm`, `documentSetForm`) — e.g. `-html-fragment`
with `-html-no-default-css` is rejected, and `-html-fragment` is refused for
`-render-documents`. `-doc-title-page/-doc-toc/-doc-number-sections` alias the older
`-pdf-*` flags onto the same variables.

Open everything as `file:///tmp/ht/...` in Chrome; no server is needed and the pages make no
network requests. A useful sanity check: DevTools → Network shows exactly **1** request
(the document) even for the default-styled page, because the sheet is inlined.

## Proving the CSS-override contract (the part a weak test misses)

`internal/doc/docrender/document.css` wraps everything in `@layer opensysml`, so *any*
unlayered reader rule must win regardless of specificity. Test all three mechanisms with a
sheet that uses **no `!important`**:

```css
.sysml-document { --sysml-accent: red; }          /* token override */
.sysml-table { border: 3px solid green; }         /* lower specificity than the default rule */
[data-element-kind="partUsage"] { background: #ffe; }  /* data-attribute hook */
```

Pass = red title/links, a 3px green outer border on every table, pale-yellow rows for
part usages. Only the middle rule distinguishes a real cascade layer from a normal sheet —
without `@layer` the more specific `.sysml-document .sysml-table` default would win.

For a `-render-documents` set, append a rule to `site/sysml-document.css` and hard-reload
(`ctrl+shift+r`) **both** pages: if only one changes, the sheet is being inlined per page
rather than shared.

## Traps / expected-but-surprising behaviour

- Mermaid diagrams are emitted as `<pre class="mermaid">` **source**, intentionally. Nothing
  is drawn unless you add a Mermaid script yourself. Not a bug.
- Formulas likewise stay LaTeX source between `\(…\)`/`\[…\]` in `.sysml-math` elements until
  `-html-math cdn|<url>` adds MathJax (`MathScriptURL` in `internal/doc/docrender/html.go` pins
  the release). Typesetting is confined to `.sysml-math` (`processHtmlClass`), so the prose `$`
  next to the inline formula must stay a literal dollar sign once MathJax has run — that is the
  check that distinguishes scoped typesetting from a page-wide scan. Loading from the CDN needs
  network access; without it the page degrades to source, which is expected.
- A query that matches nothing yields a header-only `<table>` (no `<tbody>`), and an empty
  `List` renders nothing at all. Expect the "Missing Subsystems" section to look bare.
- Linked-set pages can be short enough to fit the viewport, so a cross-document
  `...#anchor` navigation may not visibly scroll. Verify the anchor's section is on screen
  and the URL fragment matches rather than expecting movement.
- `-html-default-css -o file` should be byte-identical to
  `internal/doc/docrender/document.css` and to the `sysml-document.css` a set writes;
  `cmp` all three.
- Escaping: build an adversarial model whose titles/cells/captions/link targets contain
  `</script><script>alert(1)</script>`, `"`, `&`, `--></style>`. Everything must appear as
  literal text, DevTools Console must be empty, and no alert may fire. Attribute values are
  escaped as `&#34;`/`&#39;`, so grep for `<script` (should be 0) rather than for `alert`.

## Devin Secrets Needed

None — the backend is fully offline.
