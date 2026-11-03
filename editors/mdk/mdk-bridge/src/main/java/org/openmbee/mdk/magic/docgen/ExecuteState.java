package org.openmbee.mdk.magic.docgen;

/** Executes each target state definition; the stereotype to apply is named {@code org.openmbee.mdk.magic.docgen.ExecuteState}. */
public final class ExecuteState extends OpenSysMLQuery {
  public ExecuteState() {
    super("EXECUTE_STATE", "Execute state");
  }
}
