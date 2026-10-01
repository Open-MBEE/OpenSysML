package org.openmbee.opensysml.mdk.source;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Migration;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.mdk.model.ModelPath;

/** A SysML v1 project archive, migrated to SysML v2 text by the service's Migrate call. */
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
    Migration migration = connection.migrateFile(mdzip, "sysml");
    List<Diagnostic> collected = new ArrayList<>();
    if (!migration.experimentalNotice().isBlank()) {
      collected.add(new Diagnostic(Diagnostic.Severity.WARNING, migration.experimentalNotice(), "experimental-migration", Optional.empty()));
    }
    if (!migration.report().summary().isBlank()) {
      collected.add(new Diagnostic(Diagnostic.Severity.INFO, migration.report().summary(), "migration-report", Optional.empty()));
    }
    diagnostics = List.copyOf(collected);
    return List.of(SourceDocument.inline("model.sysml", migration.content()));
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
