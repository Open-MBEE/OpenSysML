- **`terminate` runs in an action.** A `then terminate;` node, a named terminate action usage
  (`action stop terminate;`, whose marker the parser used to drop) reached by a succession, and a
  `terminate;` statement of a nested action node's body (which lowering used to leave out) end the
  performance they are written in with the outputs assigned so far: later nodes do not run, every
  other token of that performance is dropped — a forked branch still running or parked at an
  `accept` included — in an order the trace records, and a nested node's parent continues along the
  node's succession. `terminate <name>;` ends the ongoing performance of the named action node of
  the flow it is in or of a flow around it, the node itself included. A performance that already
  ended, a name that is no action node of an enclosing flow, and an occurrence target
  (`terminate this;`) are each a typed error rather than a silent no-op; a `terminate` in a
  calculation or in a state's body is refused as before.
