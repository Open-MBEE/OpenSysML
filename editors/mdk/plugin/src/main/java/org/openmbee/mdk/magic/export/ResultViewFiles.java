package org.openmbee.mdk.magic.export;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.results.ResultViews;
import org.openmbee.mdk.magic.results.RunResult;

/**
 * Writes the {@code QVTS::QueryResult} views of a run as files named by query: beside the SysML v2
 * text a v1 project was last exported to (where its migration report is), else next to the
 * project file, which is where a v2 project's always go.
 */
public final class ResultViewFiles {
  /** The model file each project (by its file name) was last exported to in this session. */
  private static final Map<String, Path> EXPORTS = new ConcurrentHashMap<>();

  private ResultViewFiles() {}

  /** Remembers that the project stored at {@code projectFile} was exported to {@code model}. */
  public static void exported(String projectFile, Path model) {
    if (projectFile != null && !projectFile.isBlank()) EXPORTS.put(projectFile, model.toAbsolutePath().normalize());
  }

  /** The model file the views of a run on {@code path} are written beside; empty for a project never saved nor exported. */
  public static Optional<Path> anchor(ModelPath path, String projectFile) {
    if (projectFile == null || projectFile.isBlank()) return Optional.empty();
    Path exported = path == ModelPath.V1_MDZIP ? EXPORTS.get(projectFile) : null;
    return Optional.of(exported != null ? exported : Path.of(projectFile).toAbsolutePath().normalize());
  }

  /**
   * Writes each view beside {@code anchor} as {@code <anchor stem>.<query>.result.sysml}, replacing
   * the file a previous run of the same query wrote.
   *
   * @return the files written, in the views' order
   * @throws UncheckedIOException when a file cannot be written
   */
  public static List<Path> write(Path anchor, List<RunResult.View> views) {
    Path directory = anchor.toAbsolutePath().normalize().getParent();
    if (directory == null) throw new IllegalArgumentException(anchor + " has no parent directory");
    String name = anchor.getFileName().toString();
    int extension = name.lastIndexOf('.');
    String stem = extension > 0 ? name.substring(0, extension) : name;
    List<Path> written = new ArrayList<>();
    try {
      for (RunResult.View view : views) {
        Path target = directory.resolve(ResultViews.fileName(stem, view.query()));
        Files.writeString(target, view.text(), StandardCharsets.UTF_8);
        written.add(target);
      }
    } catch (IOException exception) {
      throw new UncheckedIOException("cannot write a query result view: " + exception.getMessage(), exception);
    }
    return List.copyOf(written);
  }
}
