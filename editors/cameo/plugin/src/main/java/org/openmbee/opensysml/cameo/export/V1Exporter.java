package org.openmbee.opensysml.cameo.export;

import com.nomagic.magicdraw.core.Application;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.core.project.ProjectDescriptor;
import com.nomagic.magicdraw.core.project.ProjectDescriptorsFactory;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.openmbee.opensysml.cameo.source.V1MdzipSource;

/** Exports a v1 project as a temporary .mdzip; a clean on-disk .mdzip is reused as-is. */
public final class V1Exporter {
  private V1Exporter() {}

  public static V1MdzipSource export(Project project) {
    String fileName = project.getFileName();
    if (fileName != null && fileName.toLowerCase().endsWith(".mdzip") && !project.isDirty()) {
      Path file = Path.of(fileName);
      if (Files.isRegularFile(file)) return new V1MdzipSource(file);
    }
    try {
      Path base = Path.of(System.getProperty("user.home"), ".opensysml", "cameo-exports");
      Files.createDirectories(base);
      Path directory = Files.createTempDirectory(base, "export");
      Path destination = directory.resolve("project.mdzip");
      exportModule(project, directory, destination);
      return new V1MdzipSource(destination, true);
    } catch (IOException exception) {
      throw new UncheckedIOException("cannot create the OpenSysML export directory", exception);
    }
  }

  private static void exportModule(Project project, Path directory, Path destination) {
    ProjectDescriptor descriptor =
        ProjectDescriptorsFactory.createLocalProjectDescriptor(project, destination.toFile());
    try {
      Application.getInstance().getProjectsManager()
          .exportModule(project, List.of(project.getPrimaryModel()), "OpenSysML run", descriptor);
    } catch (RuntimeException failure) {
      discard(destination, directory, failure);
      throw failure;
    }
  }

  private static void discard(Path destination, Path directory, RuntimeException failure) {
    try {
      Files.deleteIfExists(destination);
      Files.deleteIfExists(directory);
    } catch (IOException cleanupFailure) {
      failure.addSuppressed(cleanupFailure);
    }
  }
}
