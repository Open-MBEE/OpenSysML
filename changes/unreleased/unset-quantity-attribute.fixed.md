- **An unset quantity attribute no longer fails `-instantiate`.** A part whose quantity
  attribute nothing values (`attribute mass :> ISQ::mass;`) made the materialization check
  evaluate the derived `dimensions` of the `MassValue` it does not hold and report
  `feature value MassValue.dimensions: no value for feature mRef` with exit status `2`, once
  per unset quantity of any ISQ kind. The check now descends only into objects a feature value
  holds, not into the object standing for an unset one, so such a model is reported clean and
  exits `0` while `%features` lists the attribute `<unset>` as before. A bound quantity still
  derives its `dimensions`, and a default that genuinely fails beside or beneath an unset
  quantity is still reported.
