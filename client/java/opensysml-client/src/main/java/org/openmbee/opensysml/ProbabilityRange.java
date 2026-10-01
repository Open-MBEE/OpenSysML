package org.openmbee.opensysml;

/** Minimum and maximum model-draw probability over schedulers. */
public record ProbabilityRange(double min, double max) {

  /** Whether the range is a single probability, within floating-point tolerance. */
  public boolean exact() {
    return max - min <= 1e-12;
  }
}
