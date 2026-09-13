- **A declaration can be moved into another namespace of its document.** `internal/core/edit`
  gains `OpMove{Target, NewOwner}`: the declaration — its body, the comment block above it, a
  line comment after it and its own lines, the span a delete removes — is taken out of its owner
  and written at the end of the new owner's body where an added member would go, re-indented to
  its neighbors, with a body added to an owner declared without one and the empty owner meaning
  the document itself. Every reference the move would break is respelled to the shortest
  qualified name that still reaches the declaration, references inside the moved declaration
  included; an import the move leaves redundant or dangling is dropped or respelled. The move is
  one operation, so a refusal leaves neither half applied, and a batch it is part of is applied
  all-or-nothing as before. Refused, with the stable names `owner-inside-target`,
  `illegal-kind`, `member-name-taken`, `move-referenced` and `referenced-elsewhere`: moving a
  declaration into itself or into what it declares, into a body that does not admit its kind,
  beside a declaration of the same name, a reference no qualified name can respell, and a target
  another document of the workspace refers to. The service's `ApplyEdits` takes it as `MoveEdit`
  (`target`, `owner`), under the `authoring` capability like `add_member` and `delete`; the
  Python client as `Editor.move(target, owner)` and the Go client as `edit.Move`. The language
  server's `opensysml/applyModelEdit` takes it as the `move` operation (`target`, `owner`), and
  `opensysml/render` now gives each declared node the `notation` it was written with and lists,
  under that notation in the palette's `owners`, the nodes that admit it, so a client can offer
  the right destinations.
- **The VS Code diagram panel moves declarations.** A node's right-click menu gains **Move to…**,
  which lists the drawn declarations whose body may hold the node's kind — not the node itself,
  its present owner or anything it declares — and the document's top level, and moves the
  declaration into the one picked. The edit lands in the editor's buffer like typed text, so
  <kbd>Ctrl</kbd>+<kbd>Z</kbd> undoes it and the diagram redraws from the file.
