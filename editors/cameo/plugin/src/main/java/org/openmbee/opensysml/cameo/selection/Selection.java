package org.openmbee.opensysml.cameo.selection;

import java.util.Objects;
import java.util.function.Supplier;
import org.openmbee.opensysml.cameo.identity.IdentityResolver;
import org.openmbee.opensysml.cameo.model.ModelElement;
import org.openmbee.opensysml.cameo.model.ModelPath;
import org.openmbee.opensysml.cameo.source.ModelSource;

/** A runnable selection: the subject, the path it runs on, and how to export and map it back. */
public record Selection(
    ModelElement subject, ModelPath path, Supplier<ModelSource> export, Supplier<IdentityResolver> index) {
  public Selection {
    Objects.requireNonNull(subject);
    Objects.requireNonNull(path);
    Objects.requireNonNull(export);
    Objects.requireNonNull(index);
  }
}
