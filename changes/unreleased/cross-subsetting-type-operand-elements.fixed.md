- The RDF conversion now writes the `CrossSubsetting` of a `crosses` (`=>`) clause, with the chain
  `Feature` and `FeatureChaining`s of a chain target, and one `Unioning`, `Intersecting` or
  `Differencing` per operand of `unions`, `intersects` and `differences`, each with its own
  `elementId` (`<owner>_cs`, `<owner>_un<i>`, `<owner>_is<i>`, `<owner>_df<i>`), so a query's
  `elementId` is stated for them; each read blank before.
