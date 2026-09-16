- **The VS Code diagram moves a declaration when a node is dropped on another with
  <kbd>Shift</kbd> held.** While <kbd>Shift</kbd> is down, the node under the dragged one is
  outlined when its body admits the dragged declaration — the same admission the node menu's
  **Move to…** applies — and the status line says what releasing does; a node that cannot hold
  it is not outlined and releasing there puts the node back with the reason. Releasing writes
  the declaration's new position and its move as one `applyModelEdit` request, so one
  <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes both, and the diagram redraws it under its new owner where
  it was dropped; a move the server refuses — a name already taken, a declaration another file
  refers to — is shown in the status line and the node goes back. A drag without
  <kbd>Shift</kbd>, or released over empty canvas, writes a `Layout` as before.
