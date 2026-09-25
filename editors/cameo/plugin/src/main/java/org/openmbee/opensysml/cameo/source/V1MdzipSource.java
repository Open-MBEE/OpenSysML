package org.openmbee.opensysml.cameo.source;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Conversion;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.cameo.model.ModelPath;

/** A SysML v1 project archive, migrated to SysML v2 text by the service's XMI converter. */
public final class V1MdzipSource implements ModelSource {
  private final Path mdzip;
  private final boolean temporary;
  private List<Diagnostic> diagnostics = List.of();

  public V1MdzipSource(Path mdzip) {
    this(mdzip, false);
  }

  public V1MdzipSource(Path mdzip, boolean temporary) {
    this.mdzip = Objects.requireNonNull(mdzip);
    this.temporary = temporary;
  }

  public Path mdzip() {
    return mdzip;
  }

  @Override
  public ModelPath path() {
    return ModelPath.V1_MDZIP;
  }

  @Override
  public List<SourceDocument> sources(Connection connection) {
    Conversion conversion = connection.convertFile(mdzip, "sysml");
    List<Diagnostic> collected = new ArrayList<>();
    if (conversion.experimental() && !conversion.experimentalNotice().isBlank()) {
      collected.add(new Diagnostic(Diagnostic.Severity.WARNING, conversion.experimentalNotice(), "experimental-conversion", Optional.empty()));
    }
    collected.addAll(conversion.diagnostics());
    diagnostics = List.copyOf(collected);
    return List.of(SourceDocument.inline("model.sysml", conversion.content()));
  }

  @Override
  public List<Diagnostic> exportDiagnostics() {
    return diagnostics;
  }

  /** Deletes a temporary export and its directory; a reused project file is left alone. */
  @Override
  public void close() {
    if (!temporary) return;
    try {
      Files.deleteIfExists(mdzip);
      Files.deleteIfExists(mdzip.getParent());
    } catch (IOException exception) {
      throw new UncheckedIOException(exception);
    }
  }
}
