- **A library graph stripped of its source text converts back to notation whole.** Reading
  a `.ttl` of a bundled library file without `sysx:sourceText` failed on `Actions.sysml` at
  `aState.aTransition.accepter.acceptedMessage`: `accepter` is a feature every transition
  inherits from `Actions::TransitionAction`, and the reader checked the spelling against the
  bundled library rather than the notation it was rebuilding, so no spelling reached the
  graph's element. A graph whose roots are library packages under their normative ids is now
  read in that library file's place — its own declarations stand in for the bundled ones and
  a `.kerml` library is read in KerML's grammar when the roots record none — so the chain
  resolves and fifteen more KerML library files (`Clocks`, `Performances`,
  `ControlFunctions`, …) read back without source text. The target still has to be the
  graph's exact element.
