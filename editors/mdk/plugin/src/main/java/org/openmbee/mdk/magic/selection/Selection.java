package org.openmbee.mdk.magic.selection;

import java.util.Objects;
import java.util.function.Supplier;
import org.openmbee.mdk.magic.identity.IdentityResolver;
import org.openmbee.mdk.magic.model.ModelElement;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.source.ModelSource;

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
