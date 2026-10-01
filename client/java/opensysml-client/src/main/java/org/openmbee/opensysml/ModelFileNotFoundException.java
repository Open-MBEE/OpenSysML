package org.openmbee.opensysml;

/**
 * The service could not read the source file a call named, for its own process.
 */
public class ModelFileNotFoundException extends ServiceException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates the exception.
   *
   * @param message what the service reported
   */
  public ModelFileNotFoundException(String message) {
    super(StatusCode.NOT_FOUND, message);
  }
}
