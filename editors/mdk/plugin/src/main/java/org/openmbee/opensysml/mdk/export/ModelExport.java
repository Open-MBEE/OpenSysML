package org.openmbee.opensysml.mdk.export;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;

/**
 * Writes an exported model to disk: the first document at the destination, further documents
 * and the files the text refers to beside it under their relative paths, and the migration
 * report as {@code <stem>.migration-report.txt}.
 */
public final class ModelExport {
  private ModelExport() {}

  /** What one export wrote and what the service said about the text. */
  public record Result(Path model, List<Path> files, Optional<Path> report, List<Diagnostic> diagnostics) {
    public Result {
      Objects.requireNonNull(model);
      files = List.copyOf(files);
      Objects.requireNonNull(report);
      diagnostics = List.copyOf(diagnostics);
    }

    public long errorCount() {
      return diagnostics.stream().filter(item -> item.severity() == Diagnostic.Severity.ERROR).count();
    }

    public Result withDiagnostic(Diagnostic diagnostic) {
      List<Diagnostic> merged = new ArrayList<>(diagnostics);
      merged.add(diagnostic);
      return new Result(model, files, report, merged);
    }

    /** The account the completion dialog shows. */
    public String describe() {
      StringBuilder text = new StringBuilder("Wrote ").append(model);
      if (!files.isEmpty()) text.append("\nand ").append(files.size()).append(" file(s) beside it");
      report.ifPresent(path -> text.append("\nMigration report: ").append(path));
      long errors = errorCount();
      long warnings = diagnostics.stream().filter(item -> item.severity() == Diagnostic.Severity.WARNING).count();
      text.append("\n\n").append(errors).append(" error(s), ").append(warnings).append(" warning(s)");
      if (errors > 0) {
        text.append(" — the exported text does not parse clean");
        report.ifPresent(path -> text.append("; the report lists what the migration approximated"));
      }
      diagnostics.stream()
          .filter(item -> item.severity() != Diagnostic.Severity.INFO)
          .limit(10)
          .forEach(item -> text.append('\n').append(item.severity()).append(": ").append(item.message())
              .append(item.span().map(span -> " (" + span.file() + ":" + span.startLine() + ")").orElse("")));
      return text.toString();
    }
  }

  /**
   * Writes {@code documents} (the first to {@code destination}, the rest beside it by name) with
   * {@code files} and the report; creates the destination's directory.
   *
   * @throws IllegalArgumentException when a document name or file path leaves the destination's directory
   */
  public static Result write(
      Path destination, List<SourceDocument> documents, Map<String, byte[]> files,
      Optional<String> report, List<Diagnostic> diagnostics) {
    if (documents.isEmpty()) throw new IllegalArgumentException("nothing to export: the source produced no document");
    Path directory = destination.toAbsolutePath().normalize().getParent();
    if (directory == null) throw new IllegalArgumentException(destination + " has no parent directory");
    try {
      Files.createDirectories(directory);
      Files.writeString(destination, content(documents.get(0)), StandardCharsets.UTF_8);
      List<Path> written = new ArrayList<>();
      for (SourceDocument document : documents.subList(1, documents.size())) {
        Path target = inside(directory, document.name().orElseThrow(
            () -> new IllegalArgumentException("a further exported document has no name")));
        Files.createDirectories(target.getParent());
        Files.writeString(target, content(document), StandardCharsets.UTF_8);
        written.add(target);
      }
      for (Map.Entry<String, byte[]> file : files.entrySet()) {
        Path target = inside(directory, file.getKey());
        Files.createDirectories(target.getParent());
        Files.write(target, file.getValue());
        written.add(target);
      }
      Optional<Path> reportPath = Optional.empty();
      if (report.isPresent()) {
        Path target = directory.resolve(stem(destination) + ".migration-report.txt");
        Files.writeString(target, report.get(), StandardCharsets.UTF_8);
        reportPath = Optional.of(target);
      }
      return new Result(destination, written, reportPath, diagnostics);
    } catch (IOException exception) {
      throw new UncheckedIOException("cannot write " + destination + ": " + exception.getMessage(), exception);
    }
  }

  /** {@code name} without a {@code .sysml}/{@code .kerml} extension, for the files written beside it. */
  public static String stem(Path destination) {
    String name = destination.getFileName().toString();
    for (String extension : List.of(".sysml", ".kerml")) {
      if (name.toLowerCase().endsWith(extension)) return name.substring(0, name.length() - extension.length());
    }
    return name;
  }

  private static String content(SourceDocument document) {
    return document.content().orElseThrow(() -> new IllegalArgumentException(
        "exported document " + document.name().orElse("") + " carries no text"));
  }

  private static Path inside(Path directory, String relative) {
    Path target = directory.resolve(relative).normalize();
    if (relative.isBlank() || target.equals(directory) || !target.startsWith(directory)) {
      throw new IllegalArgumentException("exported path leaves the destination directory: " + relative);
    }
    return target;
  }
}
