- **DOT diagrams can be filled from a named colourblind-safe palette.** `-render-palette <name>`
  on `-render` and `-render-all`, `%render <name> dot <name>` (completed with <kbd>Tab</kbd>), a
  `palette` field on `opensysml/render`, and a `palette` attribute on a document's `Diagram` block
  fill the DOT form's nodes by keyword family — a `part def` and its `part` usages share a hue, a
  `port` takes the next, and so on through item, port, attribute, action, state, requirement,
  constraint, connection, interface, use case, case, allocation, analysis, verification, enum,
  occurrence and flow. The palettes are `okabe-ito` (Okabe & Ito), `tol-bright`, `tol-muted` and
  `tol-light` (Paul Tol), `brewer-set2` and `brewer-dark2` (ColorBrewer), and the sequential
  `viridis` and `cividis` (matplotlib), which are sampled evenly across the families a diagram
  draws. A definition is filled with the family colour and a usage with a lighter tint of it, both
  bordered in the colour; every fill is lightened until black text on it reads at the WCAG 2 AA
  ratio of 4.5:1, and pseudo-states, control nodes and cluster borders stay black and white. The
  palette applies to the DOT form alone: a Mermaid artifact notes it as `%% not represented:`,
  the text and Markdown forms ignore it, and an HTML document figure carries it as `data-palette`.
  A name that is no palette is refused on every surface with the names there are (`invalid-palette`
  in a document plan, `unsupported-palette` on a table or sequence diagram). The default, with no
  palette named, is the black-and-white style.
