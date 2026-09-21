package org.openmbee.opensysml.cameo.engine;

import java.util.Objects;
import java.util.List;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.cameo.model.ModelElement;
import org.openmbee.opensysml.cameo.source.ModelSource;

public record RunRequest(
    Operation operation, ModelSource source, String subjectQualifiedName, List<Value> calcArgs) {
  public RunRequest {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(source);
    Objects.requireNonNull(subjectQualifiedName);
    calcArgs = List.copyOf(calcArgs);
  }
}
