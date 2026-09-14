- **A view renders as PlantUML with the new `plantuml` form.** `sysml -render <view> -render-form
  plantuml` (and `.puml` files under `-render-all -render-form plantuml`), `%render <view>
  plantuml [palette]` at the prompt, and `"form": "plantuml"` on the editor's `opensysml/render`
  request write a tree, interconnection, state, action or sequence rendering as an `@startuml` …
  `@enduml` file in the Standard B&W style of the OMG SysML v2 Pilot Implementation's visualizer,
  the style carried inline as a `<style>` block so the file stands alone: a tree as a class diagram,
  an interconnection as nested `rectangle` blocks with the Pilot's heavy undirected connectors and
  dashed flows, a state or action rendering as the `state` grammar with composite states, `[*]`
  starts and PlantUML's pseudostate stereotypes, and a sequence — the one kind DOT does not write —
  as participants and messages. Labels lead with the name in bold over an italic `«keyword»` line,
  the keyword doubling as a stereotype the style selects definitions and usages by; the header,
  notices and DiagramLayout geometry are `'` comments, with a notice that PlantUML pins no position
  (the `dot` form does). The named palettes fill the nodes with the same colours the DOT form
  uses, a sequence's participants included. Producing PlantUML needs no Java and no PlantUML jar;
  nothing runs one. Mermaid stays the default machine-readable form, and a `table` view has no
  PlantUML form.
- **A document renders its diagrams as PlantUML on request.** `sysml -render-document …
  -diagram-form plantuml` (and `-render-documents`), `%render-document <name> plantuml` at the
  prompt, and `"diagramForm": "plantuml"` on `opensysml/renderDocument` write every graph-shaped
  `Diagram` block as a fenced ` ```plantuml ` block in Markdown and as `<pre class="plantuml">` in
  HTML; the PDF backend keeps the source under a notice that it does not draw PlantUML, looking
  for no PlantUML tool. A table-kind block is a table whichever form. A `Diagram` block's `palette`
  is now accepted on a sequence diagram too, since the PlantUML form fills its participants; a
  palette on a table remains a typed error.
