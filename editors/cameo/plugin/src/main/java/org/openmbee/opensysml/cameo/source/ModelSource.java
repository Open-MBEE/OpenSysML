package org.openmbee.opensysml.cameo.source;

import java.util.List;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.cameo.model.ModelPath;

/** Model text for one run; {@link #close()} releases anything exported for it. */
public interface ModelSource extends AutoCloseable {
  ModelPath path();
  List<SourceDocument> sources(Connection connection);
  List<Diagnostic> exportDiagnostics();

  @Override
  default void close() {}
}
