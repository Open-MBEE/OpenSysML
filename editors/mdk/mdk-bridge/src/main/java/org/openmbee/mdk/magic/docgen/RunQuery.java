package org.openmbee.mdk.magic.docgen;

/**
 * Runs the document queries each target stands for, bound by the {@code arguments} tag, and lays
 * their rows out as tables; the stereotype to apply is named {@code org.openmbee.mdk.magic.docgen.RunQuery}.
 */
public final class RunQuery extends OpenSysMLQuery {
  public RunQuery() {
    super("RUN_QUERY", "Run query");
  }
}
