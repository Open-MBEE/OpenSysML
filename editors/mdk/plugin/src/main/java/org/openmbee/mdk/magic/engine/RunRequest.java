package org.openmbee.mdk.magic.engine;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Value;

/** One operation on one subject; {@code calcArgs} feed "Evaluate calc", {@code queryBindings} (by binding key, element names as the host spells them) feed "Run query". */
public record RunRequest(
    Operation operation,
    ModelSource source,
    String subjectQualifiedName,
    List<Value> calcArgs,
    Map<String, List<DocumentValue>> queryBindings) {
  public RunRequest {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(source);
    Objects.requireNonNull(subjectQualifiedName);
    calcArgs = List.copyOf(calcArgs);
    Objects.requireNonNull(queryBindings);
    Map<String, List<DocumentValue>> copied = new LinkedHashMap<>();
    queryBindings.forEach((name, values) -> copied.put(name, List.copyOf(values)));
    queryBindings = java.util.Collections.unmodifiableMap(copied);
  }

  public RunRequest(Operation operation, ModelSource source, String subjectQualifiedName, List<Value> calcArgs) {
    this(operation, source, subjectQualifiedName, calcArgs, Map.of());
  }

  public RunRequest withQueryBindings(Map<String, List<DocumentValue>> bindings) {
    return new RunRequest(operation, source, subjectQualifiedName, calcArgs, bindings);
  }
}
