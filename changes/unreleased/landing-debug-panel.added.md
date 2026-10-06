- **The landing page's state machine can be debugged in the browser.** A Debug panel under the
  diagram sends `Commit`, `Pull`, `Check` and `Push` to `ModelJourney`, steps through the
  `ExecuteState` trace forwards and backwards or plays it at a chosen speed, and lights the
  current state and transition on the diagram. It lists every trace record, marks events no
  transition accepted, and shows the events still waiting. `ModelJourney` is now a hub around
  Flexo with two free choices, the start and each `Pull` out of Flexo, and a seed field
  picks those branches reproducibly.
