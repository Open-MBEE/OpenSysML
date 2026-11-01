- Expression, expression-part and body-symbol memo tables are initialized on
  their first write rather than for every semantic model. Six paired rounds
  against develop `ea9429f25` reduce empty-model load from 244,785 to 244,273
  B/op and 2,048 to 2,039 allocations, and the 64-member requirement check
  from 1,427,691.5 to 1,427,546 B/op and 15,381 to 15,378 allocations.
  This removes unused memo overhead, not the cost of required semantic objects;
  the pair predates the separate member-body-copy fix. Memoized identity and
  document-edit invalidation are preserved. No timing or RSS recovery is claimed.
