- Interconnection diagrams no longer draw the boundary symbols of a positioned node (the pins of
  its `actor`, `subject` and `stakeholder` parameters and of its ports, and the ports it declares)
  on top of one another when their routes end at one point, or a route ends where an unconnected
  port's pin is drawn: the symbols of a side are spread along it, in the order their routes come
  from, each route ending on its own symbol. Symbols on one side keep clear of a corner a
  neighbouring side's symbol holds; a run too wide for the stretch left runs past the far corner,
  or is centred between two held corners. A lone symbol stays where its route places it.
- The SysML v1 migrator marks the connection names it makes up for an actor's anonymous
  association to a use case (`'Preview Image to Operator'`) as `MigrationMetadata::SynthesizedName`,
  so diagrams no longer label those lines; a named v1 association keeps its label.
