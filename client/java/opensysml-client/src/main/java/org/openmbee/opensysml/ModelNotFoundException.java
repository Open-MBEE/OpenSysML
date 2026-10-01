package org.openmbee.opensysml;

/**
 * The service no longer holds the model a call named. Its model cache is bounded, so a model
 * loaded long ago and many models back may have been evicted; load it again.
 */
public class ModelNotFoundException extends ServiceException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates the exception.
   *
   * @param message what the service reported
   */
  public ModelNotFoundException(String message) {
    super(StatusCode.NOT_FOUND, message);
  }
}
