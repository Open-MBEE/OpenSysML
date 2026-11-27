- **The Go client's `Session` calls can be cancelled** (`client/opensysml`): `PerformContext`,
  `AdvanceContext`, `SendContext`, `AcceptsContext`, `EvaluateContext`, `InstantiateContext`,
  `FeatureContext` and `SetFeatureContext` take a `context.Context`. When it ends, a run under way
  stops at its next step and a call waiting behind another gives up, answering `CodeCanceled` or
  `CodeDeadlineExceeded`; the session stays usable. The existing methods are unchanged.
