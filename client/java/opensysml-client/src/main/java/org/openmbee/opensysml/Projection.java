package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * The API JSON declarations and annotations a caller reads.
 *
 * <p>The three selectors are unioned. An empty selector contributes nothing; all three empty write
 * the whole model. Every element written keeps the shape an unprojected conversion gives it.
 *
 * @param qualifiedNames elements whose written qualifiedName is one of these
 * @param metaclasses elements whose written {@code @type} is one of these
 * @param annotationTypes annotations whose annotating element's metaclass or metadata-definition
 *     qualified name is one of these, with the element each annotates
 */
public record Projection(
    List<String> qualifiedNames, List<String> metaclasses, List<String> annotationTypes) {

  /** A projection with no selectors, which writes the whole model. */
  public static final Projection EMPTY =
      new Projection(List.of(), List.of(), List.of());

  /** Copies the selector lists so a caller cannot change the projection after it is built. */
  public Projection {
    qualifiedNames = List.copyOf(Objects.requireNonNull(qualifiedNames, "qualifiedNames"));
    metaclasses = List.copyOf(Objects.requireNonNull(metaclasses, "metaclasses"));
    annotationTypes = List.copyOf(Objects.requireNonNull(annotationTypes, "annotationTypes"));
  }

  /** Reports whether this projection places no limit on what is written. */
  public boolean isEmpty() {
    return qualifiedNames.isEmpty() && metaclasses.isEmpty() && annotationTypes.isEmpty();
  }
}
