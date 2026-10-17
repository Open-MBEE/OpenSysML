- The `Style` and `Note` annotations of an inherited `start` or `done` node (`about Run::start`)
  now colour and annotate that node in the rendering, as its `Layout` positions it; the SysML v1
  migrator writes them for an initial node, flow final node, initial pseudostate or final state
  symbol the export does not position, instead of dropping the colour and freeing the note.
