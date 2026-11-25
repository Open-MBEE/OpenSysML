- **Fixed FMI trajectories fabricating values for discrete outputs.** The reference runner now
  reads each variable's declared variability and causality from the FMU description, blending
  only continuous Real/Float values between recorded steps and holding discrete values at a
  recorded step.
- **Fixed FMI trajectories losing an event at stop time.** When the solver records both sides
  of an event at one instant, a trajectory sample at that instant now reads the last row,
  matching the scalar output at `stopTime`.
