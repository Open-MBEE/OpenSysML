- **A migrated Cameo table shows the rows, columns and nesting the tool shows.** The reader now
  keeps an instance or generic table's `excludedElements`, `displayMode` (`List`, `Compact tree`,
  `Complete tree`), `showScopeAsRoot`, `expandedRows`, `columnWidth`, stereotype-tag columns
  (`QPROP:stereotypeTags:<<Profile::Stereotype>>.tag`) and the row filter saved with it, and the
  migration lowers each: excluded rows are subtracted with `Except`, a tree mode nests the rows
  with `Tree` so a nested instance prints its own short name at its depth rather than its dotted
  owner path, `showScopeAsRoot` adds the scope as the root, `QPROP:Element:Id` and `Text` on
  requirements read the short name and documentation the migration wrote the «Requirement» tags
  to, `QPROP:Element:classifier` reads the new `general` property, a user stereotype's tag reads
  the metadata def feature it became (every value of a multi-valued tag), widths become
  `Table.columnWidths`, and the saved filter becomes `WhereText` over the projected columns its
  `ChoiceProperty` value selects — every column when the selection is empty. A user profile
  marked «auxiliaryResource» is migrated like any other user profile, since its stereotypes'
  applications carry the user's data. The report row states each setting applied and each still
  refused.
- **Wide tables fit their pages in PDF.** A table of many columns collapsed its name column to a
  few characters and spread a handful of rows over dozens of pages. Cells now wrap at word
  boundaries and break a token only when nothing else fits, the header repeats on every page,
  rows do not split, a wide table's first column keeps a readable width, stated `columnWidths`
  size the columns proportionally, and a table of more than twelve columns is written as
  continuation tables that repeat the first column under the caption marked "(continued)", in
  HTML and in the Markdown the pandoc engine converts alike. `DocumentQueries` gains `Tree`,
  `WhereText` and `Table.columnWidths`; rows carry a nesting depth that Markdown marks with `↳`
  and HTML with `data-depth`.
