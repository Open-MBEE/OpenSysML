package org.openmbee.mdk.magic.annotations;

import java.util.Objects;
import org.openmbee.mdk.magic.model.ModelElement;

/** One validation annotation to place on a Cameo element. */
public record AnnotationPlan(ModelElement target, Severity severity, String kind, String text) {
  public enum Severity {
    INFO,
    WARNING,
    ERROR
  }

  public AnnotationPlan {
    Objects.requireNonNull(target);
    Objects.requireNonNull(severity);
    Objects.requireNonNull(kind);
    Objects.requireNonNull(text);
  }
}
