package org.openmbee.opensysml.mdk.export;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.LinkOption;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Predicate;
import java.util.stream.Stream;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;

/**
 * Writes an exported model to disk: the first document at the destination, further documents
 * and the files the text refers to beside it under their relative paths, and the migration
 * report as {@code <stem>.migration-report.txt}; staged, so a failed export leaves the previous
 * files in place.
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
   * {@code files} and the report; creates the destination's directory. Every file is first
   * written to a staging directory beside the destination and moved into place only once all of
   * them are complete, so a failure part-way leaves what was there before. Outputs that would
   * replace existing files are put to {@code replaceExisting} first; when it declines, nothing
   * is written.
   *
   * @return the export, or empty when {@code replaceExisting} declined
   * @throws IllegalArgumentException when a document name or file path leaves the destination's
   *     directory (through {@code ..} or an absolute path), goes through a symbolic link, or two outputs share a path
   */
  public static Optional<Result> write(
      Path destination, List<SourceDocument> documents, Map<String, byte[]> files,
      Optional<String> report, List<Diagnostic> diagnostics, Predicate<List<Path>> replaceExisting) {
    if (documents.isEmpty()) throw new IllegalArgumentException("nothing to export: the source produced no document");
    Path chosen = destination.toAbsolutePath().normalize();
    Path directory = chosen.getParent();
    if (directory == null) throw new IllegalArgumentException(destination + " has no parent directory");
    try {
      Files.createDirectories(directory);
      Path model = inside(directory, chosen.getFileName().toString());
      Map<Path, byte[]> planned = new LinkedHashMap<>();
      planned.put(model, content(documents.get(0)).getBytes(StandardCharsets.UTF_8));
      List<Path> written = new ArrayList<>();
      for (SourceDocument document : documents.subList(1, documents.size())) {
        Path target = inside(directory, document.name().orElseThrow(
            () -> new IllegalArgumentException("a further exported document has no name")));
        plan(planned, target, content(document).getBytes(StandardCharsets.UTF_8));
        written.add(target);
      }
      for (Map.Entry<String, byte[]> file : files.entrySet()) {
        Path target = inside(directory, file.getKey());
        plan(planned, target, file.getValue());
        written.add(target);
      }
      Optional<Path> reportPath = Optional.empty();
      if (report.isPresent()) {
        Path target = inside(directory, stem(model) + ".migration-report.txt");
        plan(planned, target, report.get().getBytes(StandardCharsets.UTF_8));
        reportPath = Optional.of(target);
      }
      List<Path> existing = planned.keySet().stream().filter(Files::exists).toList();
      if (!existing.isEmpty() && !replaceExisting.test(existing)) return Optional.empty();
      commit(directory, planned);
      return Optional.of(new Result(model, written, reportPath, diagnostics));
    } catch (IOException exception) {
      throw new UncheckedIOException("cannot write " + destination + ": " + exception.getMessage(), exception);
    }
  }

  private static void plan(Map<Path, byte[]> planned, Path target, byte[] bytes) {
    if (planned.putIfAbsent(target, bytes) != null) {
      throw new IllegalArgumentException("two exported files share the path " + target);
    }
  }

  /** Writes every planned file under a staging directory and creates every target directory, then moves them into place. */
  private static void commit(Path directory, Map<Path, byte[]> planned) throws IOException {
    Path staging = Files.createTempDirectory(directory, ".opensysml-export-");
    try {
      Map<Path, Path> staged = new LinkedHashMap<>();
      for (Map.Entry<Path, byte[]> entry : planned.entrySet()) {
        Path copy = staging.resolve(directory.relativize(entry.getKey()));
        Files.createDirectories(copy.getParent());
        Files.write(copy, entry.getValue());
        staged.put(copy, entry.getKey());
      }
      for (Path target : staged.values()) Files.createDirectories(target.getParent());
      for (Map.Entry<Path, Path> entry : staged.entrySet()) move(entry.getKey(), entry.getValue());
    } finally {
      remove(staging);
    }
  }

  private static void move(Path from, Path to) throws IOException {
    try {
      Files.move(from, to, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
    } catch (AtomicMoveNotSupportedException unsupported) {
      Files.move(from, to, StandardCopyOption.REPLACE_EXISTING);
    }
  }

  private static void remove(Path staging) throws IOException {
    try (Stream<Path> entries = Files.walk(staging)) {
      for (Path entry : entries.sorted(Comparator.reverseOrder()).toList()) Files.deleteIfExists(entry);
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

  /**
   * {@code relative} under {@code directory}, refused when it leaves the directory lexically, when
   * any component below the directory is a symbolic link (a move would replace the link, not its
   * target), or when the nearest existing ancestor resolves outside the directory.
   */
  private static Path inside(Path directory, String relative) throws IOException {
    Path target = directory.resolve(relative).normalize();
    if (relative.isBlank() || target.equals(directory) || !target.startsWith(directory)) {
      throw new IllegalArgumentException("exported path leaves the destination directory: " + relative);
    }
    for (Path step = target; !step.equals(directory); step = step.getParent()) {
      if (Files.isSymbolicLink(step)) {
        throw new IllegalArgumentException(
            "exported path goes through a symbolic link, " + step + ": export to its target instead");
      }
    }
    Path existing = target;
    while (!Files.exists(existing, LinkOption.NOFOLLOW_LINKS)) existing = existing.getParent();
    if (!existing.toRealPath().startsWith(directory.toRealPath())) {
      throw new IllegalArgumentException("exported path leaves the destination directory through a link: " + relative);
    }
    return target;
  }
}
