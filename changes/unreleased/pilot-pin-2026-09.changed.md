- **The pinned OMG pilot implementation is now release `2026-09` (`jupyter-sysml-kernel` 0.63.0)**,
  with the reference validators, the vendored standard library, the pinned corpora, grammars and
  Xpect suites, and every oracle baseline re-recorded at that pin. The standard library moved:
  `Flows.sysml` publishes the repair the errata overlay carried as `Flows-46`, `Flows-99` and
  `Flows-103` (`Message` and `messages` own their `source`/`target` ends as usages, `messages` is
  an action), so those three entries are retired, and the library project stamps are
  `1.1.0-dev.20260901` / `2.1.0-dev.20260901`. The validator gains two constraints,
  `validateConnectionDefinitionEndsAreUsages` and `validateFlowDefinitionEndsAreUsages`, which
  `W10BEndKindPass` already reports (`connection-def-end-not-usage`, `flow-def-end-not-usage`), so
  both negative cases move from ours-only to both-reject; the pilot's metadata examples now draw
  ten pilot-only `Bound features should have conforming types` warnings on enumeration-valued
  metadata bindings, adjudicated as the reference's.
- **An alias member without a name parses.** `alias for Base::exactlyOne;` — `AliasMember`'s
  `memberName` is optional in the KerML grammar — was rejected with `"for" is a reserved keyword`;
  the pilot's new `MultiplicityAlias_valid.kerml.xt` suite declares it clean, and so do we.
