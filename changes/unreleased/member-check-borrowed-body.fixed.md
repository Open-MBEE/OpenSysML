- Cardinality checks borrow an unchanged declaration body instead of copying it;
  satisfy subjects still precede that body. Six interleaved runs reduce the
  64-member requirement check from 1,427,643 to 1,426,489 B/op and remove one
  allocation. This recovers most of its increase over the accounted develop
  measurement (1,426,336 B/op), not all of it; no timing improvement is claimed.
