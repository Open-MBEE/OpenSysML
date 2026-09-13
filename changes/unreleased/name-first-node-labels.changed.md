- **Diagram node labels lead with the element's name.** Every Mermaid and DOT node written by
  the view engine — `-render`, `-render-all`, `%render`, `opensysml/render` and a document's
  diagram blocks — now heads its label the way the graphical notation heads a compartment: the
  name on the first line, with ` : Type` after it for a typed usage (`pump : Pump`), the kind on
  the next line in guillemets (`«part»`, `«part def»`), and any note (`initial`, `already shown`,
  `own flow`) after that; an anonymous element leads with its kind alone. Mermaid breaks the
  lines with `<br>` in the flowchart, `stateDiagram-v2` and `sequenceDiagram` grammars alike, and
  DOT writes an HTML-like label (`label=<<b>pump : Pump</b><br/><font point-size="10">«part»</font>>`)
  with the name in bold and the keyword line smaller, for clusters as for nodes, escaping `&`,
  `<`, `>` and `"` in a name. The text form keeps the keyword first, as the notation declares it,
  but writes the type after a colon — `part pump : Pump` instead of `part pump (Pump)`. Edge
  labels, sequence messages, geometry comments and notices are unchanged. The `opensysml/render`
  result carries a node's declared type in its own `type` field; `detail` holds the notes alone.
  A declared type is spelled as written: a conjugated port type keeps its `~`, a global name its
  `$::`, a name that is not a basic one its quotes, and a usage typed by several types lists them
  all (`base : Mount, Cart`). A Mermaid flowchart reserves one line of height for a `subgraph`
  title, so an interconnection or action rendering whose cluster title spans more opens on a YAML
  frontmatter block (`config: flowchart: subGraphTitleMargin: bottom: <n>`, 24px per extra line)
  that keeps the title clear of the first child; a flowchart without such a cluster, a tree, a
  state and a sequence diagram carry none.
