- **An unresolved name that is the unquoted start of a declared name says so, on every
  surface.** Names such as `'SA-506'` or `'HLR-R001'` hold characters no basic name can, so typed
  bare they read as an identifier followed by something else — `T::SA-506` is `T::SA` and `-506`,
  the subtraction `T::SA - 506` where an expression is expected. The failure used to stop at
  `unresolved reference: T::SA`, or at `"-506" cannot follow SA` for an object reference; it now
  offers the quoted declaration and states the rule: `unresolved reference: T::SA — did you mean
  T::'SA-506'? Names containing '-' must be quoted.` The offer comes from the declarations in
  scope (and, at the prompt, of the kinds the command acts on) whose unquoted spelling starts with
  the identifier read, never from the rest of the text; the characters named are the ones those
  declarations hold. The hint reaches the analysis diagnostics and their quick fixes, expression
  evaluation (`-e`, `%eval`, `%calc` arguments), every meta-command and command-line flag that
  resolves a name (`%instantiate`, `-instantiate`, `%state`, `-requirement`, …) and the object
  references `%features` and its kin read. Parsing is unchanged: `SA-506` is still a subtraction
  wherever an expression is valid.
- **`%instantiate` echoes a spelling `%features` can read.** Its `Use %features … to inspect` line
  used to repeat the name as typed, which for a bare `T::SA-506` names nothing; it now writes the
  declaration's own notation (`T::'SA-506'`) whenever the typed spelling would not read as an
  object reference.
