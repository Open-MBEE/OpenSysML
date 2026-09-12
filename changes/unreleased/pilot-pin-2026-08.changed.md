- **The pinned OMG pilot implementation is now release `2026-08` (`jupyter-sysml-kernel` 0.62.0)**,
  with the reference validators, the vendored standard library, the pinned corpora, grammars and
  Xpect suites, and every oracle baseline re-recorded at that pin. The grammars, the standard
  library and the validation-constraint set are unchanged from `2026-07`; what moved is the
  pilot's own behavior: it now reports a type's `disjoint` clauses (the six `kerml-examples`
  diagnostics it alone used to raise are gone), and its new Xpect assertion that a feature may
  not own two `crosses` clauses is met. The validator build passes the pin to Maven, so the
  wrapper no longer has to be re-pinned for a pilot release it does not yet default to, and it
  stamps `build/pilot-validator` with the pin it was built from so a later re-pin rebuilds a
  stale or incomplete validator instead of reporting it already built. The rebuild is staged
  beside the installed copy and swapped in only once complete, with the previous copy kept until
  the new one is in place, so a failed or interrupted run keeps the previous validator usable;
  the SysML, KerML and evaluator builds always go through that check before compiling against
  the jar.
- **The Xpect scope oracle narrows a scope to the inherited members only for a redefinition.**
  A `subsets` clause whose target is spelled differently from the declaring feature's own name
  was being treated as a redefinition, which restricted the names the scope check expected at
  that target to the inherited members alone.
