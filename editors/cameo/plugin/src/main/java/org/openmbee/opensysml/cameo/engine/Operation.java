package org.openmbee.opensysml.cameo.engine;

public enum Operation {
  INSTANTIATE("Instantiate"),
  EXECUTE_ACTION("Execute action"),
  EXECUTE_STATE("Execute state"),
  VERIFY("Verify"),
  EVALUATE_CALC("Evaluate calc"),
  RUN_ANALYSIS("Run analysis");

  private final String label;

  Operation(String label) {
    this.label = label;
  }

  public String label() {
    return label;
  }
}
