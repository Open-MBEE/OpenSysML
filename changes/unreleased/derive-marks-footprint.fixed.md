Deriving a feature's value marks what it stands on over the memoized footprint of its expression
without copying the footprint's reads to hold them via the object's feature. A derived read-write-read
allocates 3 104 B and 69 times instead of 3 328 B and 71 times, recovering a third of what #1619
added over 0.11.0's 2 688 B and 67 on the same toolchain; the marks #1619 records remain.
