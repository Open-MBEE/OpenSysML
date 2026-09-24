package org.openmbee.opensysml;

/**
 * What kind of failure an undecided {@link Verdict} or a failed calculation, analysis or validation
 * reports, so a caller acts on the kind rather than on the message text.
 */
public enum FailureReason {
  /** No failure, or one the service did not classify. */
  UNSPECIFIED,
  /** A condition or calculation that could not be evaluated. */
  EVALUATION,
  /** A symbol that declares something else than was asked about. */
  WRONG_KIND,
  /** Several objects carry the element: name one as the subject. */
  AMBIGUOUS_SUBJECT,
  /** A reason this release of the client does not know. */
  UNKNOWN
}
