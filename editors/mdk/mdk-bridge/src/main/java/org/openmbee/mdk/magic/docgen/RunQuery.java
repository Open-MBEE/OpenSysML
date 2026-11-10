package org.openmbee.mdk.magic.docgen;

/** Runs each target's document queries, bound by the {@code arguments} tag; the stereotype to apply is named {@code org.openmbee.mdk.magic.docgen.RunQuery}. */
public final class RunQuery extends OpenSysMLQuery {
  public RunQuery() {
    super("RUN_QUERY", "Run query");
  }
}
