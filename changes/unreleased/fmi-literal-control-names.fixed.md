- **Fixed literal FMU variables named like FMI control selectors.** A variable whose
  archive name is `fmi:stepTime` or `fmi:series:<name>` is now set and read as a model
  variable on every surface, including session routing; genuine step and trajectory
  selectors keep their own paths and a real step-plus-series call is still refused.
