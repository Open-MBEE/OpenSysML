- **A `filter` member's reflected visibility and its condition's typing id agree with the conversion.**
  An `ElementFilterMembership` now reads the visibility written ahead of `filter` (`private filter @M;`
  is `private`, a bare `filter` is `public` whatever its package's own visibility; KerML 8.2.3.4.1,
  8.3.2.4.3) instead of its owning namespace's, and the `FeatureTyping` of a type argument's
  parameter (`@M`, `x istype T`, `x as T`) reads the `elementId` the conversion writes
  (`…_in<n>_ft0`) instead of none.
