package org.openmbee.mdk.magic.results;

import org.openmbee.mdk.magic.results.RunResult.Outcome;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.opensysml.ServiceException;

/** The one place a query's failure becomes an outcome: today the service message of a {@code ServiceException}, later the client's typed query outcome. */
public final class QueryFailure {
  private QueryFailure() {}

  public static Outcome outcome(String query, RuntimeException failure) {
    String message = failure instanceof ServiceException service ? service.serviceMessage() : failure.getMessage();
    return new Outcome(query, message == null ? failure.getClass().getName() : message, Status.ERROR, query);
  }
}
