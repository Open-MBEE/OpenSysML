- **A migrated Cameo table shows the rows, columns and nesting the tool shows.** The reader now
  keeps an instance or generic table's `excludedElements`, `displayMode` (`List`, `Compact tree`,
  `Complete tree`), `showScopeAsRoot`, `expandedRows`, `columnWidth`, stereotype-tag columns
  (`QPROP:stereotypeTags:<<Profile::Stereotype>>.tag`) and the row filter saved with it, and the
  migration lowers each: the rows of an instance table are the individual definitions its
  instance specifications became, not the slots typed by them, excluded rows are subtracted with
  `Except`, a tree mode nests the rows
  with `Tree` so a nested instance is listed at its depth under the row containing it, its name
  as written, `showScopeAsRoot` adds the scope as the root, `QPROP:Element:Id` and `Text` on
  requirements read the short name and documentation the migration wrote the «Requirement» tags
  to, `QPROP:Element:classifier` reads `general`, which now lists a usage's types as well as a
  definition's, a user stereotype's tag reads
  the metadata def feature it became (every value of a multi-valued tag), widths become
  `Table.columnWidths`, and the saved filter becomes `WhereText` over the projected columns its
  `ChoiceProperty` value selects — every column when the selection is empty, and no filter when the
  query reads none of the selected columns. The rows a table
  lists explicitly lead the scope's other elements in the order the tool listed them, so an
  unsorted table and each level of a tree keep that order. Each column is headed as the tool
  headed it — `Id`, `Text`, `classifier`, a tag's name over both of two columns reading the same
  tag — through the new `Table.columnLabels`, while the column names the query, `groupBy` and
  HTML `data-column` know stay unique. An element-valued
  cell — a `general`, a tag value, a feature whose declared value names an element of the model
  such as an enumeration literal, which was printed as the qualified name it was written with —
  is that element, printed by its effective name, the one the `name` property reads, as Cameo
  prints a classifier or a tag value; HTML carries the qualified name in `data-element`, and
  `WhereFeature` compares such a cell as the name it prints, or as its qualified name against a
  qualified value. A user profile
  marked «auxiliaryResource» is migrated like any other user profile, since its stereotypes'
  applications carry the user's data. The report row states each setting applied and each still
  refused.
- **Wide tables fit their pages in PDF.** A table of many columns collapsed its name column to a
  few characters and spread a handful of rows over dozens of pages. Cells now wrap at word
  boundaries and break a token only when nothing else fits, the header repeats on every page,
  rows do not split, a wide table's first column keeps a readable width, stated `columnWidths`
  size the columns proportionally though never narrower than a heading's longest word, and a
  table whose headings need more than the line its column count has — a portrait page in the
  ordinary type under seven columns, a landscape page in the wide type under eleven — is set on
  a landscape page in the dense type when they fit that line, whatever a theme sets ordinary
  cells at, and a table whose headings overrun even that line — or of more than twelve columns —
  is written on it as continuation tables that repeat the first column, at no less than its
  readable width, ahead of the columns that head unbroken beside it, under the caption marked
  "(continued)", in HTML and in the Markdown the pandoc engine converts alike; an unsized wide
  table's columns share the width evenly, none narrower than its heading, and headings that
  need more than the line between them share it in proportion rather than overrunning the page; a
  table without a caption continues without one. `DocumentQueries` gains `Tree`, `WhereText`, `Table.columnWidths`
  and `Table.columnLabels`; rows carry a nesting depth that Markdown marks with `↳` and HTML with
  `data-depth`.
