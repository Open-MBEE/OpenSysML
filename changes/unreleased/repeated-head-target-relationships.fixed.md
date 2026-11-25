- The RDF conversion now writes one relationship element per name of a head clause that names a
  target again (`: A, A`, `specializes A, A`, `subsets g, g`, `redefines g, g`, `references g, g`,
  `crosses g, g`, `unions A, A`, `intersects A, A`, `differences A, A`, `disjoint from A, A`,
  `featured by A, A`), each with its own `elementId`, as `about x, x` already did. It wrote one
  element for both names before, so a query read the same `elementId` for two relationships, and
  the repeated name was lost when the notation was read back from the graph alone.
  A repeated name also comes back in the heads written in shorthand (`return :> v, v`,
  `perform a :>> b, b`), and under the keyword it was written with (`subsets g, g specializes g`).
