- **The PSSM referee classifies a guard whose behavior acts on the model as having no
  translation.** A SysML v2 guard is a Boolean expression; a UML guard whose activity calls
  `trace(...)` before returning its value has no spelling, and the translation used to carry the
  value alone and silently drop the call. The classifier now names the construct (*guard side
  effect*), the emitter refuses it, and *Choice 005* moves from `fail` to `not-expressible`
  (17 `fail`, 40 `not-expressible`), adjudicated in `docs/project/pssm-referee.md`.
