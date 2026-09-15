- **The language server runs the behavior a diagram draws.** The new `opensysml/debug/*`
  requests (`start`, `step`, `continue`, `send`, `advance`, `breakpoints`, `stop`), advertised as
  the `openSysmlDebug` capability, execute the state machine or action a `state` or `action` view
  renders — with the executors the REPL's `%state` and `%action` debuggers use, optionally as
  performed by an instantiated part — and answer every request with a snapshot in the IDs of that
  view's `opensysml/render` result: the active states (composite states and regions included, one
  chain per orthogonal region), each action token with the node it sits at, the edge it arrived by
  and the join edges or signal it waits for, the transitions and successions taken since the last
  snapshot, the events queued and the messages pending, the runtime's clock, its notes, a completed
  action's results, and whether the run is `running`, `waiting`, `suspended`, `completed`,
  `failed` or `ended`. Breakpoints are set by render node ID and pause a run as a token reaches the
  node or the state becomes active; a signal the behavior accepts nowhere is refused rather than
  queued to be lost. A session follows the document: an edit that leaves as they were the
  declarations the run reads — the target's, its performer's, and every declaration those name
  and the named name in turn (specialized and typing definitions, invoked actions, accepted
  signals, feature types, values a guard or a `send` names) — keeps it running and reports the
  snapshot in the fresh IDs through the new `opensysml/debugChanged` notification, while one that
  rewrites or removes any of them, makes the run read one it did not, or rewrites the declared
  view, ends it and says why. To place runtime state on a rendering, the runtime
  now records the transitions a state machine fires (`StateExecutor.FiredTransitions`) and the
  successions each token travels (`ActionExecutor.Traversals`, `Token.Within` for the nested flows
  it runs in), a `view.StateLocator`/`view.ActionLocator` map lowered vertices, action nodes and
  edges to render IDs by their position within the declaration, and document, they were written in
  (a node or edge inherited from another document is drawn from that document and keeps its place
  as either document is edited), and `model.Workspace.NewRuntime` builds a
  runtime model over a workspace's documents whose `Dependencies` lists the declarations a run
  reads; a lowered `StateGraph` or `ActionGraph` lists the
  declarations it took content from besides its own (`Inherited`), which the root node of a
  `state` or `action` rendering carries as `view.Node.Inherited`. See
  [the LSP reference](docs/reference/lsp.md).
