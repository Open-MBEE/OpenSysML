- **The VS Code diagram panel edits the model.** An **Add…** menu on the panel and a right-click
  menu on every node the file declares add a member (`part`, `port`, `state`, `action`, a `def`, …),
  add a connection, flow, succession or transition between two nodes, rename a declaration, or
  delete one — with a confirmation, and a second one before a delete cascades to the declarations
  that refer to it. The kinds offered follow the diagram: an interconnection diagram offers parts,
  ports and connections, a state diagram states and transitions, an action or sequence diagram
  actions, control nodes and successions, a tree every kind the language has — `subject`, `actor`
  and `stakeholder` only on a requirement or case, `objective` only on a case. Every action is a
  source-preserving edit of the `.sysml` or `.kerml` file, applied to the editor's buffer like typed
  text: <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it, comments and layout outside the edited lines are
  untouched, and the diagram redraws from what the file now says. An edit that would leave the file
  with an error it did not have is refused and the message names the diagnostic. Layout is not
  persisted and nodes are not dragged.
- **`opensysml/applyModelEdit` LSP request.** Turns a list of model operations (`setValue`,
  `rename`, `addMember`, `addConnection`, `delete`) on a document at a stated version into a
  versioned `WorkspaceEdit` the client applies itself, so the change lands in the editor's own undo
  history and the server learns of it through `textDocument/didChange`. A version that no longer
  matches is answered `stale`; an edit the re-analysis refuses is answered with the operation at
  fault, a stable failure name, the diagnostics the edited text would have had and the declarations
  still referring to a target. The server advertises it as `experimental.openSysmlApplyModelEdit`.
  `opensysml/render` now gives each node its qualified name (`fqn`) for the request to target, and a
  `palette` naming the member and connection kinds a diagram of that kind offers and, for a member
  only some bodies declare, the nodes it may go into.
- **Source-preserving connection edits.** `internal/core/edit` gains `OpAddConnection`, which writes
  a `connection`, `interface`, `allocation`, `binding`, `flow`, `succession` or `transition` (KerML:
  `connector`, `binding`, `flow`, `succession`) into an owner's body with its ends spelled as they
  resolve from that scope, and refuses a kind the language does not have, a type on a kind that takes
  none, an end that does not resolve, or a name already taken.
