- **The extent `all T` is taken over the whole model, not the namespaces enclosing the
  expression.** The roots of `all T` are every object the run holds and every namespace-level
  object usage of every document of the loaded model that may hold a `T` — an imported package's,
  an unrelated un-imported package's and the standard library's alike, so `size(all Car)` in a
  package importing `Gaps` counts `Gaps::car` as `Gaps` itself does, and `all Clock` answers
  `Time::universalClock` — materialized as the extent is taken, in document-name then declaration
  order, the order `.metadata` gives cross-file annotations. The extent is of the type (KerML 1.0
  §7.3.2.1, §7.4.9.2), not of what the expression's namespace sees, so what it used to answer from
  a sibling package's expression is now what it answers from anywhere. A usage nested in a
  definition nothing instantiates still contributes no object, the library's `[0..*]` collections
  (`Parts::parts`, `Items::items`) neither count nor refuse, a namespace-level collection or port
  that may hold a `T` still refuses the extent with `ErrExtentUnavailable` naming it, and a usage of
  another document that cannot be read ends the extent with that usage's error naming it. The
  model's namespace usages are enumerated once per model and judged once per run and type, and a
  binding to an extent is read again when a re-analysis adds or removes such a usage in any
  document.
