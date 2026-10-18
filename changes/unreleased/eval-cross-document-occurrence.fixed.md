- `%eval` of a usage declared in another document than the prompt's scope reads the object
  already materialized for it (the one a behavior was performed on) instead of materializing a
  second one holding the declaration's defaults.
