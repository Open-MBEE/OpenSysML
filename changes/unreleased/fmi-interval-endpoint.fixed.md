- **Fixed a decimal sampling interval dropping its final sample.** A 0–0.3 s run sampled every
  0.1 s now asks for four instants including 0.3 s; the sample bound and explicit-instant path
  are unchanged.
