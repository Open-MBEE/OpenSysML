- **A migrated DocGen `Image` of a Cameo table embeds the table, not a listing of its view.** An
  `Image` step (or a viewpoint-less view) drawing an instance table, generic table, matrix or
  relation map diagram wrote a `Diagram` of the view rendered `asElementTable`, which a document
  drew as an Element / Kind / Type / Declared-in dump of the view's members. The section now
  holds a `Table` over the same `… Rows` query the table's standalone document uses — written
  once, beside the view — captioned by the step's title, numbered among the tables, and the
  report row says which query it is over. A table whose definition has no query form is refused
  where the figure would be, with the reason, instead of drawn as the listing.
- **A DocGen view conforming to no viewpoint shows what MDK's default behavior shows.** Such a
  view wrote its documentation alone. It now follows `DocumentGenerator.parseView`: a view that
  is itself a diagram shows its own figure; any other shows, after its documentation, each
  diagram it exposes in order — a plain diagram as a figure, a table diagram as its table —
  and nothing for an exposed element that is not a diagram, the report row naming what it drew
  and what it left out. As in MDK, only a missing «Conform» means that: a view whose «Conform»
  names no element is refused with the reason, and the «View» stereotype's `viewpoint` tag
  chooses no method.
- **Collaborator paragraphs stand where their anchors put them.** Every collaborator paragraph
  followed the section's generated content. A paragraph with no `siblingId`/`parentId` now
  precedes it, as Cameo prints it; one anchored to a generated figure
  (`Containment_DiagramMainImage__<id>`) follows the figure, table or refusal the section drew
  for that diagram, with its followers after it — after the section's only figure when the anchor
  names no diagram of the model — and one whose anchor cannot be placed (another anchor kind, a
  tag naming nothing, an ambiguous section) follows the content with the reason in its row.
