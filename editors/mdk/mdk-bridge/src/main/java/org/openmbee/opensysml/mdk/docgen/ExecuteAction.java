package org.openmbee.opensysml.mdk.docgen;

/** Executes each target action definition; the stereotype to apply is named {@code org.openmbee.opensysml.mdk.docgen.ExecuteAction}. */
public final class ExecuteAction extends OpenSysMLQuery {
  public ExecuteAction() {
    super("EXECUTE_ACTION", "Execute action");
  }
}
