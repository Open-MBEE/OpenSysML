- The runtime judges a one-value sequence written to a unique feature as repeating nothing
  before building the set it judges repeats by, so a calculation's result held in its
  one-element shape is read into a unique feature without allocating: a loop stepping a
  calculation usage allocates as it did before results were held in their effective multiplicity.
