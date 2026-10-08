package org.openmbee.opensysml.mdk.docgen;

/** Evaluates each target calculation with the {@code arguments} tag; the stereotype to apply is named {@code org.openmbee.opensysml.mdk.docgen.EvaluateCalc}. */
public final class EvaluateCalc extends OpenSysMLQuery {
  public EvaluateCalc() {
    super("EVALUATE_CALC", "Evaluate calc");
  }
}
