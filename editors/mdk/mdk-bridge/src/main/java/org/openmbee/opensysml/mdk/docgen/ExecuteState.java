package org.openmbee.opensysml.mdk.docgen;

/** Executes each target state definition; the stereotype to apply is named {@code org.openmbee.opensysml.mdk.docgen.ExecuteState}. */
public final class ExecuteState extends OpenSysMLQuery {
  public ExecuteState() {
    super("EXECUTE_STATE", "Execute state");
  }
}
