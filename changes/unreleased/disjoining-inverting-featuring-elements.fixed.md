- The RDF conversion now writes one `Disjoining`, `FeatureInverting` or `TypeFeaturing` per target
  of a declaration's `disjoint from`, `inverse of` and `featured by` clause, with the chain
  `Feature` and `FeatureChaining`s of a chain target, each with its own `elementId`
  (`<owner>_dj<i>`, `<owner>_iv<i>`, `<owner>_tf<i>`), so a query's `elementId` is stated for
  them; each read blank before.
