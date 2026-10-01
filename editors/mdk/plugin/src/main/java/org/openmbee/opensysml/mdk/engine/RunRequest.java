package org.openmbee.opensysml.mdk.engine;

import java.util.Objects;
import java.util.List;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.mdk.model.ModelElement;
import org.openmbee.opensysml.mdk.source.ModelSource;

public record RunRequest(
    Operation operation, ModelSource source, String subjectQualifiedName, List<Value> calcArgs) {
  public RunRequest {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(source);
    Objects.requireNonNull(subjectQualifiedName);
    calcArgs = List.copyOf(calcArgs);
  }
}
