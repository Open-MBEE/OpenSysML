package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.zip.ZipEntry;
import java.util.zip.ZipOutputStream;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.bridge.DocGenBridge;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.export.ModelExport;
import org.openmbee.mdk.magic.export.ResultViewFiles;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.mdk.magic.source.V1MdzipSource;
import org.openmbee.mdk.magic.source.V2TextualSource;
import org.openmbee.opensysml.ConnectionOptions;

/** Drives "Run query" answers written as QVTS::QueryResult views, and document rendering, through the real service. */
class ResultViewEndToEndTest {
  private static Engine engine;

  @BeforeAll
  static void openEngine() {
    Path binary = ServiceBinary.required();
    engine = new Engine(() -> ConnectionOptions.builder().binaryPath(binary).build());
  }

  @AfterAll
  static void closeEngine() {
    if (engine != null) engine.close();
  }

  private static V2TextualSource lander() throws Exception {
    Path examples = ServiceBinary.repository().resolve("examples/qvts");
    StringBuilder text = new StringBuilder();
    for (String file : List.of("lander.sysml", "duplicates.sysml", "queries.sysml")) {
      text.append(Files.readString(examples.resolve(file))).append('\n');
    }
    return new V2TextualSource(text.toString());
  }

  private static RunResult run(ModelSource source, String subject) {
    return engine.run(new RunRequest(Operation.RUN_QUERY, source, subject, List.of()), () -> false);
  }

  /** A value, an empty selection, a path error and a model error each come back as one view stating its outcome. */
  @Test
  void everyOutcomeOfAQueryIsWrittenAsAView() throws Exception {
    V2TextualSource source = lander();

    RunResult cabin = run(source, "QVTSDemo::CabinMass");
    assertEquals(Status.PASSED, cabin.status(), cabin.toString());
    RunResult.View view = cabin.views().get(0);
    assertEquals("QVTSDemo::CabinMass", view.query());
    assertEquals("CabinMassResult::cabinMass", view.name());
    assertTrue(view.text().contains("view cabinMass : QueryResult {"), view.text());
    assertTrue(view.text().contains("expose Lander::lander::cabin;"), view.text());
    assertTrue(view.text().contains("attribute :>> query = \"QVTSDemo::CabinMass\";"), view.text());
    assertTrue(view.text().contains("attribute :>> outcome = Outcome::established;"), view.text());
    assertEquals(view.text(), run(source, "QVTSDemo::CabinMass").views().get(0).text(), "the same run writes the same bytes");

    RunResult empty = run(source, "QVTSDemo::EmptySelection");
    assertEquals(Status.INFO, empty.status(), empty.toString());
    assertTrue(empty.views().get(0).text().contains("Outcome::established"), empty.views().toString());
    assertFalse(empty.views().get(0).text().contains("expose "), "an empty selection exposes nothing");

    RunResult missing = run(source, "QVTSDemo::MissingMassProject");
    assertEquals(Status.FAILED, missing.status(), missing.toString());
    String pathError = missing.views().get(0).text();
    assertTrue(pathError.contains("attribute :>> outcome = Outcome::pathError;"), pathError);
    assertTrue(pathError.contains("expose Lander::lander::cabinEngine;"), "the view exposes where the path stopped: " + pathError);

    RunResult ambiguous = run(source, "QVTSDemo::AmbiguousPart");
    assertEquals(Status.FAILED, ambiguous.status(), ambiguous.toString());
    assertTrue(ambiguous.views().get(0).text().contains("Outcome::modelError"), ambiguous.views().toString());
  }

  /** The files a run's views land in: next to a v2 project, beside the export of a v1 one, named by the query. */
  @Test
  void viewsAreWrittenBesideTheModelNamedByQuery() throws Exception {
    RunResult cabin = run(lander(), "QVTSDemo::CabinMass");
    Path directory = Files.createTempDirectory("magic-mdk-views");
    Path project = directory.resolve("lander.mdzip");

    Path anchor = ResultViewFiles.anchor(ModelPath.V2_TEXTUAL, project.toString()).orElseThrow();
    List<Path> written = ResultViewFiles.write(anchor, cabin.views());
    assertEquals(List.of(directory.resolve("lander.QVTSDemo.CabinMass.result.sysml")), written);
    assertEquals(cabin.views().get(0).text(), Files.readString(written.get(0)));

    assertEquals(project.toAbsolutePath(), ResultViewFiles.anchor(ModelPath.V1_MDZIP, project.toString()).orElseThrow(),
        "a v1 project not yet exported keeps its views next to the project");
    Path exported = directory.resolve("out").resolve("fleet.sysml");
    Files.createDirectories(exported.getParent());
    ResultViewFiles.exported(project.toString(), exported);
    assertEquals(exported, ResultViewFiles.anchor(ModelPath.V1_MDZIP, project.toString()).orElseThrow());
    assertEquals(project.toAbsolutePath(), ResultViewFiles.anchor(ModelPath.V2_TEXTUAL, project.toString()).orElseThrow());
    assertEquals(List.of(exported.resolveSibling("fleet.QVTSDemo.CabinMass.result.sysml")),
        ResultViewFiles.write(exported, cabin.views()));
    assertTrue(ResultViewFiles.anchor(ModelPath.V2_TEXTUAL, "").isEmpty(), "an unsaved project has nowhere to write");
  }

  /** The bridge's rendered view lists the elements its DocGen table lists, row for row. */
  @Test
  void aRenderedViewAgreesWithTheDocGenTable() throws Exception {
    V2TextualSource source = lander();
    RunResult masses = run(source, "QVTSDemo::MassTable");
    assertEquals(Status.PASSED, masses.status(), masses.toString());
    assertTrue(masses.views().get(0).header().isEmpty(), "a view is laid out only on request");

    RunResult rendered = engine.renderViews(source, masses, () -> false);
    RunResult.View view = rendered.views().get(0);
    assertEquals(List.of("query", "source", "outcome", "provenance"),
        view.header().stream().map(RunResult.Field::name).toList());
    assertEquals("QVTSDemo::MassTable", view.header().get(0).value());
    assertEquals("established", view.header().get(2).value());
    List<String> tableElements = masses.tables().get(0).rows().stream().map(RunResult.Row::elementId).distinct().toList();
    assertFalse(tableElements.isEmpty());
    assertEquals(tableElements, view.rows().stream().map(row -> row.get(0)).toList());

    Map<String, Object> flat = DocGenBridge.flatten(rendered);
    @SuppressWarnings("unchecked")
    Map<String, Object> flatView = ((List<Map<String, Object>>) flat.get("views")).get(0);
    assertEquals(view.name(), flatView.get("name"));
    assertEquals(view.text(), flatView.get("text"));
    assertEquals(view.rows(), flatView.get("rows"));
    assertEquals(Map.of("name", "outcome", "value", "established"), ((List<?>) flatView.get("header")).get(2));
  }

  /** "Render document…" on a v2 Document and on a QueryResult view writes Markdown and HTML named by what was rendered. */
  @Test
  void rendersADocumentAndAResultViewToFiles() throws Exception {
    V2TextualSource source = lander();
    Path directory = Files.createTempDirectory("magic-mdk-render");

    List<Path> report = engine.render(source, "QVTSDemo::MassReport", directory, () -> false).orElseThrow();
    assertEquals(List.of(directory.resolve("QVTSDemo.MassReport.md"), directory.resolve("QVTSDemo.MassReport.html")), report);
    assertTrue(Files.readString(report.get(0)).contains("# Lander Mass Report"), Files.readString(report.get(0)));
    assertTrue(Files.readString(report.get(1)).contains("<h1"), "the HTML form is a page");

    RunResult cabin = run(source, "QVTSDemo::CabinMass");
    V2TextualSource withView = new V2TextualSource(
        Files.readString(ServiceBinary.repository().resolve("examples/qvts/lander.sysml")) + "\n"
            + Files.readString(ServiceBinary.repository().resolve("examples/qvts/queries.sysml")) + "\n"
            + cabin.views().get(0).text());
    List<Path> view = engine.render(withView, "CabinMassResult::cabinMass", directory, () -> false).orElseThrow();
    assertEquals(directory.resolve("CabinMassResult.cabinMass.md"), view.get(0));
    String markdown = Files.readString(view.get(0));
    assertTrue(markdown.contains("Lander::lander::cabin"), markdown);
    assertTrue(markdown.contains("established"), markdown);
    assertTrue(Files.readString(view.get(1)).contains("Lander::lander::cabin"));

    IllegalArgumentException neither = assertThrows(IllegalArgumentException.class,
        () -> engine.render(source, "Lander::lander", directory, () -> false));
    assertEquals("Lander::lander is neither a DocumentQueries::Document nor a QVTS::QueryResult view", neither.getMessage());
    assertTrue(engine.render(source, "QVTSDemo::MassReport", directory, () -> true).isEmpty(), "a cancelled rendering writes nothing");
  }

  /** The v1 path: a DocGen «Document» renders through the Document the migration wrote for it, and its queries answer as views. */
  @Test
  void v1DocGenDocumentRendersAndItsQueriesAnswerAsViews() throws Exception {
    Path directory = Files.createTempDirectory("magic-mdk-test");
    Path mdzip = directory.resolve("documents.mdzip");
    try (OutputStream output = Files.newOutputStream(mdzip); ZipOutputStream zip = new ZipOutputStream(output)) {
      zip.putNextEntry(new ZipEntry("com.nomagic.magicdraw.uml_model.model"));
      zip.write(Files.readAllBytes(ServiceBinary.repository().resolve("tests/migrate/testdata/xmi/documents.xmi")));
      zip.closeEntry();
    }
    Path output = Files.createTempDirectory("magic-mdk-output");
    try (V1MdzipSource source = new V1MdzipSource(mdzip, true)) {
      RunResult rows = run(source, "Fleet Viewpoints::Parts Viewpoint::Parts Method::Fleet Parts");
      assertEquals(Status.PASSED, rows.status(), rows.toString());
      assertEquals(1, rows.views().size(), rows.toString());
      assertTrue(rows.views().get(0).text().contains("Outcome::established"), rows.views().toString());

      Path export = output.resolve("fleet.sysml");
      ModelExport.Result exported = engine.export(source, export, existing -> true, () -> false).orElseThrow();
      ResultViewFiles.exported(mdzip.toString(), exported.model());
      Path anchor = ResultViewFiles.anchor(ModelPath.V1_MDZIP, mdzip.toString()).orElseThrow();
      List<Path> written = ResultViewFiles.write(anchor, rows.views());
      assertEquals(exported.report().orElseThrow().getParent(), written.get(0).getParent(),
          "the view lands beside the migration report");
      assertTrue(written.get(0).getFileName().toString().startsWith("fleet."), written.toString());

      List<Path> rendered = engine.render(source, "Fleet Documents::Fleet Handbook", output.resolve("rendered"), () -> false)
          .orElseThrow();
      assertEquals(2, rendered.size(), rendered.toString());
      assertTrue(rendered.get(0).getFileName().toString().endsWith(".md"), rendered.toString());
      assertFalse(Files.readString(rendered.get(0)).isBlank());
      assertTrue(rendered.get(1).getFileName().toString().endsWith(".html"), rendered.toString());
    }
  }
}
