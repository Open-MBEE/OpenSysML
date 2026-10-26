package org.openmbee.opensysml.mdk.source;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.mdk.model.ModelPath;

/** Model text for one run; {@link #close()} releases anything exported for it. */
public interface ModelSource extends AutoCloseable {
  ModelPath path();
  List<SourceDocument> sources(Connection connection);
  List<Diagnostic> exportDiagnostics();

  /** Files the model text refers to by relative path (the images its views embed); known after {@link #sources}. */
  default Map<String, byte[]> files() {
    return Map.of();
  }

  /** The full migration report when the source is a migration that asked for one; known after {@link #sources}. */
  default Optional<String> reportText() {
    return Optional.empty();
  }

  /** The service's name for an element the host names {@code qualifiedName}; identity unless the export renames roots. */
  default String subjectName(String qualifiedName) {
    return qualifiedName;
  }

  /** The host's name for an element the service names {@code subjectName}; the inverse of {@link #subjectName}. */
  default String elementName(String subjectName) {
    return subjectName;
  }

  @Override
  default void close() {}
}
