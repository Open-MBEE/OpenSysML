- **Arithmetic, comparison and aggregation over a quantity on a measurement scale agree with
  `ConvertQuantity`.** A magnitude on an `IntervalScale` such as `SI::'°C_abs'` or `Time::UTC`
  is a point on an affine scale, and the operators treated the scale as a ratio unit: `300.0 [K]
  == ConvertQuantity(300.0 [K], SI::'°C_abs')`, `300.0 [K] < 30.0 [SI::'°C_abs']`, `warm + 10.0
  [SI::'°C']` and `5.0 [Time::UTC] + 3.0 [s]` were `incommensurable units`, while `2 * warm`,
  `warm * 2.0 [s]` (a `['°C_abs'*s]` unit) and `5.0 [Time::UTC] + 3.0 [Time::UTC]` computed
  meaningless points. A point now moves by a difference in a commensurable ratio unit
  (`36.85 ['°C_abs']`, `8.0 [UTC]`), two points subtract to a difference in the scale's declared
  `unit` (`warm - 10.0 [SI::'°C_abs']` is `16.85 ['°C']`, `5.0 [Time::UTC] - 3.0 [Time::UTC]` is
  `2.0 [s]`), `==`, `!=`, `<`, `<=`, `>`, `>=`, `min` and `max` carry the right operand onto the
  left operand's reference through the scale's anchor before comparing, and set membership and
  collection equality equate `293.15 [K]` with `20.0 [SI::'°C_abs']`.
- **Operations a point on a scale does not define are typed errors.** `point + point`,
  `k * point`, `point * q`, `point / q`, `q / point`, `magnitude - point`, `point ** n`,
  `sqrt(point)`, `-point` and `sum`/`product` over points are `ErrScalePoint` naming the scale
  and the operation, and a point on an `OrdinalScale`, `CyclicRatioScale` or
  `LogarithmicScale` refuses interval arithmetic instead of behaving as a ratio unit. A
  measurement scale is no longer accepted as a factor of a unit term, on the wire included, and
  the static dimension check warns about a refused operation a literal makes certain while
  accepting `point + difference`.
