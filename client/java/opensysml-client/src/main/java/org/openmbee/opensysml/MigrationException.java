package org.openmbee.opensysml;

import java.util.List;

/**
 * The service could not migrate a SysML v1 model at all: it was not a v1 model it could read, or
 * the target format cannot carry the result. An element the migration has no v2 form for is not
 * an error — it is left unmapped and reported in the {@link MigrationReport}.
 */
public class MigrationException extends ModelException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a migration exception.
   *
   * @param message the failure, as the service worded it
   */
  public MigrationException(String message) {
    super(message, List.of());
  }
}
