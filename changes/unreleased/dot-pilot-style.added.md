- **The DOT render form draws in the SysML v2 Pilot visualizer's Standard B&W style.** A
  `-render-form dot`, `%render <name> dot`, `opensysml/render` or document DOT artifact is now
  drawn as the OMG SysML v2 Pilot Implementation's PlantUML visualizer draws it, after the
  `sysmlbw` PlantUML skin by Hisashi Miyashita (Mgnite Inc.) shipped with the Pilot: Helvetica
  text, white fills, thin `#181818` lines (`penwidth=0.5` on nodes, `1` on edges, edge text at
  13 pt), a definition square and a usage `rounded`, the name in bold over the `«keyword»` line in
  italics, clusters unfilled with black borders (`1.5` for a package, `0.5` for an element or a
  region), a connection at `penwidth=3`, and an unnamed initial or final pseudo-state as the
  filled black UML dot. The `graph`, `node` and `edge` defaults open every digraph and a node
  lists only what it deviates in. Nothing else moved: node IDs, label text and escaping, `pos`
  splines, cluster anchors, `lhead`/`ltail`, the header comments and the order of nodes and edges
  are as before, so a diagram laid out from the old text lays out the same. Producing DOT still
  needs no Graphviz installation. The translation, what the skin says that Graphviz cannot draw,
  and the provenance are recorded in `docs/project/view-rendering-forms.md`.
