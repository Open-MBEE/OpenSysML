- **The fUML test models are read and every activity is classified before any is translated.**
  `internal/fuml` reads the pinned Eclipse UML2 XMI — activities, nodes, pins, control and object
  flows with guards and weights, parameters, classes, operations, signals, associations,
  structured nodes, exception handlers and cross-references into the foundational library —
  through the XMI element walker shared with the PSSM referee, which now accepts every OMG XMI
  namespace version. `Classify` files each of the 43 test-model activities and 12
  exception-model activities as expressible, `differs-by-design` (an action the reference
  implementation fired once per object token, which SysML v2 performs once with every delivery)
  or `not-expressible`, with a reason naming the construct and where it occurs; the per-activity
  checklist and counts (24, 4, 15; 12) are pinned by test, and CI downloads the suite and runs
  the reader and classifier gates on their own. See `docs/project/fuml-referee.md`.
