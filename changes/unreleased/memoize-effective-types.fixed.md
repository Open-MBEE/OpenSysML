- The runtime computes a feature's effective types (the types it states conjoined with
  those it inherits through subsetting, redefinition or reference) once per model instead
  of on every object it makes: instantiation and satisfaction of models whose features
  subset typed features allocate as they did before objects were typed by every inherited type.
