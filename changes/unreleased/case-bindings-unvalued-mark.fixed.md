- **A case-local a run left declared without a value stays unvalued in the frame its conditions
  read.** `frame.markUnvalued` allocated its map on a copy of the frame, so a mark on a frame that
  held no unvalued name yet was lost, and an objective or assertion naming such a local could read
  through to a binding of the same name instead of being undecided for the missing value.
