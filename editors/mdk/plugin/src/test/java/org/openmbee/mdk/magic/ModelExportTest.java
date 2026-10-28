package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.DisabledOnOs;
import org.junit.jupiter.api.condition.OS;
import org.junit.jupiter.api.io.TempDir;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.mdk.magic.export.ModelExport;

/** The on-disk shape of an export: text, files beside it, the report, and the paths it refuses. */
class ModelExportTest {
  private static final List<SourceDocument> MODEL = List.of(SourceDocument.inline("model.sysml", "package P;\n"));

  @Test
  void writesTextFilesAndReportBesideTheDestination(@TempDir Path directory) throws Exception {
    Path destination = directory.resolve("out").resolve("Mission.sysml");
    Map<String, byte[]> files = Map.of("images/logo.png", new byte[] {1, 2, 3});

    ModelExport.Result result = ModelExport.write(
        destination, MODEL, files, Optional.of("migrated 3 element(s)\n"), List.of(), existing -> true).orElseThrow();

    assertEquals("package P;\n", Files.readString(destination));
    assertEquals(List.of(directory.resolve("out/images/logo.png")), result.files());
    assertEquals(3, Files.readAllBytes(result.files().get(0)).length);
    assertEquals(directory.resolve("out/Mission.migration-report.txt"), result.report().orElseThrow());
    assertEquals("migrated 3 element(s)\n", Files.readString(result.report().orElseThrow()));
    assertEquals(0, result.errorCount());
    assertTrue(result.describe().contains("Mission.migration-report.txt"), result.describe());
    assertTrue(result.describe().contains("0 error(s), 0 warning(s)"), result.describe());
  }

  @Test
  void textOnlyExportWritesNoReport(@TempDir Path directory) throws Exception {
    Path destination = directory.resolve("v2.sysml");
    ModelExport.Result result = ModelExport.write(destination, MODEL, Map.of(), Optional.empty(), List.of(), existing -> true).orElseThrow();
    assertEquals(Optional.empty(), result.report());
    assertEquals(List.of(), result.files());
    assertEquals(List.of("v2.sysml"), Files.list(directory).map(path -> path.getFileName().toString()).toList());
  }

  @Test
  void furtherDocumentsLandBesideTheFirstByName(@TempDir Path directory) throws Exception {
    List<SourceDocument> documents = List.of(
        SourceDocument.inline("model.sysml", "package A;\n"), SourceDocument.inline("lib/B.sysml", "package B;\n"));
    ModelExport.Result result = ModelExport.write(directory.resolve("A.sysml"), documents, Map.of(), Optional.empty(), List.of(), existing -> true).orElseThrow();
    assertEquals(List.of(directory.resolve("lib/B.sysml")), result.files());
    assertEquals("package B;\n", Files.readString(directory.resolve("lib/B.sysml")));
  }

  @Test
  void refusesFilesThatLeaveTheDestinationDirectory(@TempDir Path directory) {
    Path destination = directory.resolve("out").resolve("model.sysml");
    for (String escape : List.of("../outside.png", "/etc/passwd", "")) {
      IllegalArgumentException refused = assertThrows(IllegalArgumentException.class,
          () -> ModelExport.write(destination, MODEL, Map.of(escape, new byte[0]), Optional.empty(), List.of(), existing -> true), escape);
      assertTrue(refused.getMessage().contains("leaves the destination directory"), refused.getMessage());
    }
    assertFalse(Files.exists(directory.resolve("outside.png")));
  }

  @Test
  void describeCountsAndListsDiagnostics(@TempDir Path directory) {
    List<Diagnostic> diagnostics = List.of(
        new Diagnostic(Diagnostic.Severity.INFO, "migrated 1 element(s)", "migration-report", Optional.empty()),
        new Diagnostic(Diagnostic.Severity.WARNING, "approximated X", "approximate", Optional.empty()),
        new Diagnostic(Diagnostic.Severity.ERROR, "unresolved Y", "nameres", Optional.empty()));
    ModelExport.Result result = ModelExport.write(directory.resolve("m.sysml"), MODEL, Map.of(), Optional.empty(), diagnostics, existing -> true).orElseThrow();
    assertEquals(1, result.errorCount());
    String text = result.describe();
    assertTrue(text.contains("1 error(s), 1 warning(s)"), text);
    assertTrue(text.contains("ERROR: unresolved Y"), text);
    assertTrue(text.contains("WARNING: approximated X"), text);
    assertFalse(text.contains("migrated 1 element(s)"), text);
    assertEquals(2, result.withDiagnostic(diagnostics.get(2)).errorCount());
  }

  @Test
  void stemDropsTheNotationExtensionOnly() {
    assertEquals("Mission", ModelExport.stem(Path.of("Mission.sysml")));
    assertEquals("Lib", ModelExport.stem(Path.of("x/Lib.KerML")));
    assertEquals("notes.txt", ModelExport.stem(Path.of("notes.txt")));
  }

  @Test
  void asksBeforeReplacingExistingFilesAndWritesNothingWhenDeclined(@TempDir Path directory) throws Exception {
    Path destination = directory.resolve("Mission.sysml");
    Path image = directory.resolve("images/logo.png");
    Files.createDirectories(image.getParent());
    Files.write(image, new byte[] {9});
    Files.writeString(destination, "old\n");
    List<List<Path>> asked = new ArrayList<>();

    Optional<ModelExport.Result> declined = ModelExport.write(
        destination, MODEL, Map.of("images/logo.png", new byte[] {1}), Optional.of("report\n"), List.of(),
        existing -> {
          asked.add(existing);
          return false;
        });

    assertEquals(Optional.empty(), declined);
    assertEquals(List.of(List.of(destination, image)), asked);
    assertEquals("old\n", Files.readString(destination));
    assertArrayEquals(new byte[] {9}, Files.readAllBytes(image));
    assertFalse(Files.exists(directory.resolve("Mission.migration-report.txt")));
    assertEquals(List.of("Mission.sysml", "images"), Files.list(directory).map(path -> path.getFileName().toString()).sorted().toList());

    ModelExport.write(destination, MODEL, Map.of("images/logo.png", new byte[] {1}), Optional.empty(), List.of(), existing -> true);
    assertEquals("package P;\n", Files.readString(destination));
    assertArrayEquals(new byte[] {1}, Files.readAllBytes(image));
  }

  @Test
  void failedExportLeavesThePreviousFilesInPlace(@TempDir Path directory) throws Exception {
    Path destination = directory.resolve("Mission.sysml");
    Files.writeString(destination, "old\n");
    Files.writeString(directory.resolve("images"), "a file where the export needs a directory\n");

    assertThrows(UncheckedIOException.class, () -> ModelExport.write(
        destination, MODEL, Map.of("images/logo.png", new byte[] {1}), Optional.empty(), List.of(), existing -> true));

    assertEquals("old\n", Files.readString(destination));
    assertEquals(List.of("Mission.sysml", "images"), Files.list(directory).map(path -> path.getFileName().toString()).sorted().toList());
  }

  @Test
  @DisabledOnOs(OS.WINDOWS)
  void refusesFilesThatLeaveTheDestinationDirectoryThroughALink(@TempDir Path directory) throws Exception {
    Path outside = Files.createDirectories(directory.resolve("outside"));
    Path out = Files.createDirectories(directory.resolve("out"));
    Files.createSymbolicLink(out.resolve("images"), outside);

    IllegalArgumentException refused = assertThrows(IllegalArgumentException.class, () -> ModelExport.write(
        out.resolve("model.sysml"), MODEL, Map.of("images/logo.png", new byte[] {1}), Optional.empty(), List.of(), existing -> true));

    assertTrue(refused.getMessage().contains("symbolic link"), refused.getMessage());
    assertEquals(List.of(), Files.list(outside).toList());
    assertFalse(Files.exists(out.resolve("model.sysml")));
  }

  @Test
  @DisabledOnOs(OS.WINDOWS)
  void refusesALinkedDestinationAndLeavesTheLinkAndItsTarget(@TempDir Path directory) throws Exception {
    Path revision = Files.createDirectories(directory.resolve("revisions")).resolve("v1.sysml");
    Files.writeString(revision, "old\n");
    Path current = Files.createSymbolicLink(directory.resolve("current.sysml"), revision);
    Path dangling = Files.createSymbolicLink(directory.resolve("gone.sysml"), directory.resolve("missing.sysml"));

    for (Path destination : List.of(current, dangling)) {
      IllegalArgumentException refused = assertThrows(IllegalArgumentException.class,
          () -> ModelExport.write(destination, MODEL, Map.of(), Optional.empty(), List.of(), existing -> true));
      assertTrue(refused.getMessage().contains("export to its target instead"), refused.getMessage());
      assertTrue(Files.isSymbolicLink(destination));
    }
    assertEquals("old\n", Files.readString(revision));
    assertFalse(Files.exists(directory.resolve("missing.sysml")));
  }

  @Test
  @DisabledOnOs(OS.WINDOWS)
  void refusesALinkBelowAnExistingDirectoryOnTheWay(@TempDir Path directory) throws Exception {
    Path out = Files.createDirectories(directory.resolve("out"));
    Files.createDirectories(out.resolve("images"));
    Files.createSymbolicLink(out.resolve("images/icons"), Files.createDirectories(out.resolve("elsewhere")));

    IllegalArgumentException refused = assertThrows(IllegalArgumentException.class, () -> ModelExport.write(
        out.resolve("model.sysml"), MODEL, Map.of("images/icons/a.png", new byte[] {1}), Optional.empty(), List.of(),
        existing -> true));

    assertTrue(refused.getMessage().contains("symbolic link"), refused.getMessage());
    assertEquals(List.of(), Files.list(out.resolve("elsewhere")).toList());
  }
}
