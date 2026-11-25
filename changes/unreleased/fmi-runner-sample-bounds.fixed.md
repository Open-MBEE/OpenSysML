- **Fixed trajectory samples that reported unobserved time instants.** The reference runner
  now refuses an instant outside the simulated run instead of relabelling its last recorded
  value, and spaces evenly spaced samples over recorded time rather than row index, so extra
  solver-event rows no longer move a requested midpoint.
