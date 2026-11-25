- **Fixed FMI trajectories jumping between close solver events.** When a requested instant falls
  within the rounding tolerance of a recorded event, the runner now groups only rows recorded at
  that exact timestamp, so a later distinct step can no longer replace the event's value.
  Duplicate rows at one timestamp still resolve to the post-event value.
