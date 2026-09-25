package org.openmbee.opensysml;

/**
 * What kind of refusal an {@link EditException} reports, so a caller acts on the kind rather than
 * on the message text. Every refusal is one of these: an edit is never silently dropped.
 */
public enum EditFailure {
  /** No failure, or one the service did not classify. */
  UNSPECIFIED,
  /** The request named no edit. */
  NO_OPERATIONS,
  /** No element of that name is in the model. */
  UNKNOWN_TARGET,
  /** The name denotes several declarations. */
  AMBIGUOUS_TARGET,
  /** The element can carry no value. */
  NOT_VALUED,
  /** The new value does not parse as an expression. */
  INVALID_VALUE,
  /** The new name does not lex as an identifier. */
  INVALID_NAME,
  /** The element declares no name to rewrite. */
  NOT_NAMED,
  /** References to the element would break. */
  RENAME_REFERENCED,
  /** Two edits cover the same bytes. */
  OVERLAPPING_EDITS,
  /** The edited source has errors the original had not. */
  RESULT_INVALID,
  /** An add-member owner does not exist. */
  OWNER_UNKNOWN,
  /** The owner cannot contain members. */
  OWNER_NOT_NAMESPACE,
  /** The kind is invalid for the document's language. */
  ILLEGAL_KIND,
  /** The owner already declares the name. */
  MEMBER_NAME_TAKEN,
  /** A delete would leave references dangling. */
  DELETE_REFERENCED,
  /** A move's owner is the target or inside it. */
  OWNER_INSIDE_TARGET,
  /** A move would leave a reference no spelling restores. */
  MOVE_REFERENCED,
  /** The target is referred to from a document the edit cannot rewrite. */
  REFERENCED_ELSEWHERE,
  /** A refusal kind this release of the client does not know. */
  UNRECOGNIZED
}
