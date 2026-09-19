package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * An edit batch the service answered but refused: the request was well formed, and this is why the
 * model was not edited.
 *
 * <p>A {@link ModelException}, since the refusal is reported inside an answer rather than as a
 * call's status; the {@link #failure()} kind is what a caller acts on.
 */
public final class EditException extends ModelException {

  private static final long serialVersionUID = 1L;

  private final EditFailure failure;
  private final String failureName;
  private final List<String> referringElements;
  private final List<Referrer> referrers;

  /**
   * Creates an edit refusal.
   *
   * @param message the refusal, as the service worded it
   * @param failure what kind of refusal it is
   * @param failureName the refusal's wire name, kept for a kind this release does not know
   * @param diagnostics diagnostics the answer carried
   * @param referringElements where references to the refused target are made, as the service
   *     spelled them
   * @param referrers the referring declarations, each with its document
   */
  public EditException(
      String message,
      EditFailure failure,
      String failureName,
      List<Diagnostic> diagnostics,
      List<String> referringElements,
      List<Referrer> referrers) {
    super(message, diagnostics);
    this.failure = Objects.requireNonNull(failure, "failure");
    this.failureName = Objects.requireNonNull(failureName, "failureName");
    this.referringElements = List.copyOf(Objects.requireNonNull(referringElements, "referringElements"));
    this.referrers = List.copyOf(Objects.requireNonNull(referrers, "referrers"));
  }

  /**
   * What kind of refusal this is.
   *
   * @return the kind; {@link EditFailure#UNRECOGNIZED} for one this release does not know
   */
  public EditFailure failure() {
    return failure;
  }

  /**
   * The refusal's wire name, such as {@code "EDIT_FAILURE_UNKNOWN_TARGET"}: the enum's name for a
   * known kind, and the unrecognized name the service sent for one it does not have.
   *
   * @return the failure's name on the wire
   */
  public String failureName() {
    return failureName;
  }

  /**
   * Where the references to a refused rename's, delete's or move's target are made: the qualified
   * name of each referring namespace, suffixed with its document in parentheses when that is not
   * the document edited.
   *
   * @return the referring elements, empty for a refusal that names none
   */
  public List<String> referringElements() {
    return referringElements;
  }

  /**
   * The declarations referring to the refused target, each with the document declaring it, in
   * document then name order.
   *
   * @return the referrers, empty for a refusal that names none
   */
  public List<Referrer> referrers() {
    return referrers;
  }
}
