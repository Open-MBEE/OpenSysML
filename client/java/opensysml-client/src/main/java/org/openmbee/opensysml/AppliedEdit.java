package org.openmbee.opensysml;

import java.util.Objects;

/**
 * One byte range of the original source that an applied operation replaced, so a caller can report
 * or locate what changed.
 *
 * @param operationIndex index of the operation in the batch, so an applied edit maps back to what
 *     asked for it
 * @param target the element edited, as the request named it
 * @param offset the byte offset where the replacement starts, into the source the operation saw
 * @param length the bytes replaced; zero for text inserted where a feature had no value
 * @param oldText what was there; empty for an insertion
 * @param newText what replaced it
 * @param document the document the bytes are in, named as the parse named it and as {@link
 *     EditResult#documents()} lists it; set for a single-document model too
 */
public record AppliedEdit(
    int operationIndex,
    String target,
    int offset,
    int length,
    String oldText,
    String newText,
    String document) {

  /**
   * Creates an applied edit.
   *
   * @param operationIndex the operation's index
   * @param target the element edited, never {@code null}
   * @param offset where the replacement starts
   * @param length the bytes replaced
   * @param oldText what was there, never {@code null}
   * @param newText what was written, never {@code null}
   * @param document the document, never {@code null}
   */
  public AppliedEdit {
    Objects.requireNonNull(target, "target");
    Objects.requireNonNull(oldText, "oldText");
    Objects.requireNonNull(newText, "newText");
    Objects.requireNonNull(document, "document");
  }
}
