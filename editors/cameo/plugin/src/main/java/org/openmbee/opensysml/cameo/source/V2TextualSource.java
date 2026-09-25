package org.openmbee.opensysml.cameo.source;

import java.util.List;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.cameo.model.ModelPath;

public final class V2TextualSource implements ModelSource {
  private final String text;

  public V2TextualSource(String text) {
    this.text = text;
  }

  @Override
  public ModelPath path() {
    return ModelPath.V2_TEXTUAL;
  }

  @Override
  public List<SourceDocument> sources(Connection connection) {
    return List.of(SourceDocument.inline("model.sysml", text));
  }

  @Override
  public List<Diagnostic> exportDiagnostics() {
    return List.of();
  }
}
