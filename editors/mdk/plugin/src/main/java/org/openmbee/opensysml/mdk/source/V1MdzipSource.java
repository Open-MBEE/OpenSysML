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
import org.openmbee.opensysml.MigrationEntry;
import org.openmbee.opensysml.MigrationOptions;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.mdk.model.ModelPath;

/** A SysML v1 project archive, migrated to SysML v2 text by the service's Migrate call. */
public final class V1MdzipSource implements ModelSource {
  private final Path mdzip;
  private final boolean temporary;
  private List<Diagnostic> diagnostics = List.of();
  private List<String> rootPackages = List.of();

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
    Migration migration = connection.migrateFile(mdzip, "sysml", MigrationOptions.defaults().withReport(true));
    List<Diagnostic> collected = new ArrayList<>();
    if (!migration.experimentalNotice().isBlank()) {
      collected.add(new Diagnostic(Diagnostic.Severity.WARNING, migration.experimentalNotice(), "experimental-migration", Optional.empty()));
    }
    if (!migration.report().summary().isBlank()) {
      collected.add(new Diagnostic(Diagnostic.Severity.INFO, migration.report().summary(), "migration-report", Optional.empty()));
    }
    diagnostics = List.copyOf(collected);
    rootPackages = migration.report().entries().stream()
        .filter(entry -> "Model".equals(entry.kind()) && !entry.target().isBlank())
        .map(MigrationEntry::target)
        .toList();
    return List.of(SourceDocument.inline("model.sysml", migration.content()));
  }

  /** The packages the migration wrote the project's root Models as, in report order. */
  public List<String> rootPackages() {
    return rootPackages;
  }

  /**
   * Cameo names an element below its root Model; the migration writes that Model as a package, so
   * the service's name starts with it. A project of several root Models leaves the name as given.
   */
  @Override
  public String subjectName(String qualifiedName) {
    if (rootPackages.size() != 1) return qualifiedName;
    String root = rootPackages.get(0);
    if (qualifiedName.equals(root) || qualifiedName.startsWith(root + "::")) return qualifiedName;
    return root + "::" + qualifiedName;
  }

  @Override
  public String elementName(String subjectName) {
    for (String root : rootPackages) {
      if (subjectName.startsWith(root + "::")) return subjectName.substring(root.length() + 2);
    }
    return subjectName;
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
