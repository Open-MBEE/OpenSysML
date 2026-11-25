- **A feature that reference-subsets another reaches the referenced feature's members from its own body.** In
  `ref part observer ::> source { succession first sent then got; }`, where `source` owns the two events, the checker
  reported `connector-type-featuring` on the succession and `feature-reference-featuring-types` on each end, and did the
  same for a `connect`, `bind`, `flow` or feature value written there. Reference subsetting is a subsetting (KerML
  8.3.3.3.9), so `observer` is featured wherever `source` is and the ends are featured within the connector's featuring
  type (8.3.4.5.3); the pinned pilot validator accepts these models. The same ends named without the reference
  (`ref part observer { succession first source::sent then source::got; }`) are still rejected, as the pilot rejects
  them, and a referencing feature still does not acquire the referenced feature's type.
  A succession written there that reverses an order the referenced feature declares is reported as
  `succession-order-cycle`, as it is under an ordinary subsetting.
