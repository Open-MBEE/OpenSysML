package org.openmbee.mdk.magic.results;

import org.openmbee.mdk.magic.results.RunResult.Outcome;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.opensysml.ServiceException;

/**
 * The one place a query's failure becomes an outcome. Today the service answers a {@code
 * ServiceException} whose message holds the path or model error; a typed query outcome (kind,
 * step, frontier elements, findings) is mapped here once the Java client carries it.
 */
public final class QueryFailure {
  private QueryFailure() {}

  public static Outcome outcome(String query, RuntimeException failure) {
    String message = failure instanceof ServiceException service ? service.serviceMessage() : failure.getMessage();
    return new Outcome(query, message == null ? failure.getClass().getName() : message, Status.ERROR, query);
  }
}
