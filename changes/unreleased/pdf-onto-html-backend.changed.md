- **PDF output is laid out from the HTML document backend's page.** `-doc-form pdf` hands
  WeasyPrint and Prince the same semantic HTML `-doc-form html` writes, under the backend's
  default stylesheet and a print stylesheet layered after it, instead of a page reconstructed from
  the rendered Markdown; pandoc keeps reading the Markdown. `-html-theme`, `-html-no-default-css`
  and `-html-css` now reach a WeasyPrint or Prince PDF exactly as they reach HTML — a user
  stylesheet is unlayered, so it overrides the default and print layers without `!important` — and
  are refused with a typed error for pandoc, whose page is its own. Tables, figures, formulas and
  cross-references carry their `sysml-*` classes and `data-*` attributes into the PDF's HTML.
- **Markdown output no longer carries a caption marker.** The `<!-- caption -->` comment the
  Markdown backend wrote ahead of a table's or diagram's emphasized caption is gone; the caption
  stays an emphasized paragraph. Pandoc recognizes captions by matching those paragraphs against
  the document's captions in order, so an emphasized paragraph elsewhere stays prose. A caption
  is written without its surrounding blanks, which CommonMark would otherwise read as literal
  asterisks or as indented code; a blank caption writes no paragraph.
