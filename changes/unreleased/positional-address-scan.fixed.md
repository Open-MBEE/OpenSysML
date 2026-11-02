- Positional-address checks scan quoted identity segments without allocating a
  segment slice; quoted `@` names and escaped separators keep their meaning.
  Six interleaved runs reduce the 32-satellite API-JSON conversion from
  34,507,433.5 to 34,483,544 allocations against develop `bb5809b78`,
  a reduction of 23,889.5. Historical accounted develop measured
  34,373,317.5 allocations: the result still exceeds that total by about 0.32%.
  ISQ writer modes lose about 1,541 allocations; other writer allocation
  rows do not significantly improve. A compact-writer timing movement did
  not reproduce in six additional isolated alternating pairs.
  The remainder includes required semantic and output growth; neither a timing
  improvement nor recovery of the complete API-JSON writer increase is claimed.
