package org.openmbee.opensysml.mdk.selection;

import java.util.Objects;
import java.util.function.Supplier;
import org.openmbee.opensysml.mdk.identity.IdentityResolver;
import org.openmbee.opensysml.mdk.model.ModelElement;
import org.openmbee.opensysml.mdk.model.ModelPath;
import org.openmbee.opensysml.mdk.source.ModelSource;

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
