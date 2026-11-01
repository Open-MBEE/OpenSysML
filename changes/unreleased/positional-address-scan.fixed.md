- Positional-address checks scan quoted identity segments without allocating a
  segment slice; quoted `@` names and escaped separators keep their meaning.
  Six interleaved runs reduce the 32-satellite API-JSON conversion from
  34,507,434 to 34,483,606.5 allocations, recovering 23,827.5 of its increase
  over the accounted develop measurement (34,373,317.5 allocations).
  The remainder includes required semantic and output growth; neither a timing
  improvement nor recovery of the complete API-JSON writer increase is claimed.
