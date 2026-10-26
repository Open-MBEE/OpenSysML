package org.openmbee.opensysml.mdk.source;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Migration;
import org.openmbee.opensysml.MigrationEntry;
import org.openmbee.opensysml.MigrationOptions;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.mdk.model.ModelPath;

/**
 * A SysML v1 project archive, migrated to SysML v2 text by the service's Migrate call. The
 * migration lays every diagram out as a view from the archive's own symbol streams; an MTIP
 * export of the same project ({@link #withLayout}) augments that geometry.
 */
public final class V1MdzipSource implements ModelSource {
  private final Path mdzip;
  private final boolean temporary;
  private final Optional<Path> layout;
  private List<Diagnostic> diagnostics = List.of();
  private List<String> rootPackages = List.of();
  private Map<String, byte[]> files = Map.of();
  private Optional<String> reportText = Optional.empty();

  public V1MdzipSource(Path mdzip) {
    this(mdzip, false);
  }

  public V1MdzipSource(Path mdzip, boolean temporary) {
    this(mdzip, temporary, Optional.empty());
  }

  private V1MdzipSource(Path mdzip, boolean temporary, Optional<Path> layout) {
    this.mdzip = Objects.requireNonNull(mdzip);
    this.temporary = temporary;
    this.layout = Objects.requireNonNull(layout);
  }

  /** The same archive, migrated with the MTIP (HUDS XML) export {@code layout} as the layout augment. */
  public V1MdzipSource withLayout(Path layout) {
    return new V1MdzipSource(mdzip, temporary, Optional.of(layout));
  }

  public Path mdzip() {
    return mdzip;
  }

  public Optional<Path> layout() {
    return layout;
  }

  @Override
  public ModelPath path() {
    return ModelPath.V1_MDZIP;
  }

  @Override
  public List<SourceDocument> sources(Connection connection) {
    MigrationOptions options = MigrationOptions.defaults().withReport(true);
    if (layout.isPresent()) options = options.withLayoutFile(layout.get());
    Migration migration = connection.migrateFile(mdzip, "sysml", options);
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
    files = migration.files();
    reportText = migration.report().text().isBlank() ? Optional.empty() : Optional.of(migration.report().text());
    return List.of(SourceDocument.inline("model.sysml", migration.content()));
  }

  @Override
  public Map<String, byte[]> files() {
    return files;
  }

  @Override
  public Optional<String> reportText() {
    return reportText;
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
