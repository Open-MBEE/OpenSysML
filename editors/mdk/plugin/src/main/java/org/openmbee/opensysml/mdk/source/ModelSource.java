package org.openmbee.opensysml.mdk.source;

import java.util.List;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.mdk.model.ModelPath;

/** Model text for one run; {@link #close()} releases anything exported for it. */
public interface ModelSource extends AutoCloseable {
  ModelPath path();
  List<SourceDocument> sources(Connection connection);
  List<Diagnostic> exportDiagnostics();

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
