- **Fixed trajectory sampling of real FMPy results.** The reference runner now reads named
  fields from NumPy structured records when interpolating requested instants, so a sample
  between recorded steps no longer fails, and uses each variable's declared FMI type to
  interpolate continuous values while leaving discrete values at recorded steps.
