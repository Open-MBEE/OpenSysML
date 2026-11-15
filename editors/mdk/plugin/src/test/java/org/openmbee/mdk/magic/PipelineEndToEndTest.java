package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.zip.ZipEntry;
import java.util.zip.ZipOutputStream;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.mdk.magic.annotations.AnnotationPlan;
import org.openmbee.mdk.magic.annotations.AnnotationPlanner;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.QueryParameter;
import org.openmbee.mdk.magic.engine.QueryPlan;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.export.ModelExport;
import org.openmbee.mdk.magic.identity.IdentityIndex;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.model.SimpleElement;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.results.RunResult.Status;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.mdk.magic.source.V1MdzipSource;
import org.openmbee.mdk.magic.source.V2TextualSource;

/** Drives export output → convert → parse → operation → annotation plans through the real service. */
class PipelineEndToEndTest {
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

  private static RunResult run(Operation operation, ModelSource source, String subject) {
    return engine.run(new RunRequest(operation, source, subject, List.of()), () -> false);
  }

  /** The v1 path: a .mdzip wrapping the vehicle XMI, exactly as {@code V1Exporter} hands it over. */
  @Test
  void v1MdzipVerifyRequirementAndConstraint() throws Exception {
    Path directory = Files.createTempDirectory("magic-mdk-test");
    Path mdzip = directory.resolve("vehicle.mdzip");
    try (OutputStream output = Files.newOutputStream(mdzip); ZipOutputStream zip = new ZipOutputStream(output)) {
      zip.putNextEntry(new ZipEntry("com.nomagic.magicdraw.uml_model.model"));
      zip.write(Files.readAllBytes(ServiceBinary.repository().resolve("tests/migrate/testdata/xmi/vehicle.xmi")));
      zip.closeEntry();
    }
    var vehicle = new SimpleElement("_v", "Vehicle Design::Vehicle", "Vehicle", "vehicle");
    var constraint = new SimpleElement("_c", "Vehicle Design::Vehicle::positive mass", "positive mass", "constraint");
    var requirement = new SimpleElement("_r", "Requirements::Mass Requirement", "Mass Requirement", "requirement");
    var index = IdentityIndex.of(List.of(vehicle, constraint, requirement));

    try (V1MdzipSource source = new V1MdzipSource(mdzip, true)) {
      RunResult constraintResult = run(Operation.VERIFY, source, "'Vehicle Design'::Vehicle::'positive mass'");
      assertEquals(Status.PASSED, constraintResult.status(), constraintResult.toString());
      assertEquals(ModelPath.V1_MDZIP, constraintResult.path());
      List<AnnotationPlan> plans = AnnotationPlanner.plan(constraintResult, index);
      assertEquals("constraint", plans.get(0).target().handle());
      assertEquals(AnnotationPlan.Severity.INFO, plans.get(0).severity());

      RunResult requirementResult = run(Operation.VERIFY, source, "Requirements::'Mass Requirement'");
      // The v1 text requirement states no required constraint, so it holds vacuously and
      // maps back onto the Cameo requirement element as an info annotation.
      assertEquals(Status.PASSED, requirementResult.status(), requirementResult.toString());
      List<AnnotationPlan> requirementPlans = AnnotationPlanner.plan(requirementResult, index);
      assertEquals("requirement", requirementPlans.get(0).target().handle());
      assertEquals(AnnotationPlan.Severity.INFO, requirementPlans.get(0).severity());
    }
    assertFalse(Files.exists(mdzip), "temporary export is deleted with the source");
    assertFalse(Files.exists(directory));
  }

  /** The v2 path: textual notation as {@code SysMLTextualNotationService} exports it. */
  @Test
  void v2TextualVerifyInstantiateAndExecute() throws Exception {
    String text = Files.readString(Path.of("src/test/resources/fixtures/demo.sysml"));
    V2TextualSource source = new V2TextualSource(text);
    var index = IdentityIndex.of(List.of(
        new SimpleElement("p", "Demo::Vehicle::lightEnough", "lightEnough", "pass"),
        new SimpleElement("f", "Demo::Vehicle::tiny", "tiny", "fail"),
        new SimpleElement("v", "Demo::Vehicle", "Vehicle", "vehicle"),
        new SimpleElement("m", "Demo::Vehicle::massLimit", "massLimit", "req")));

    RunResult pass = run(Operation.VERIFY, source, "Demo::Vehicle::lightEnough");
    assertEquals(Status.PASSED, pass.status(), pass.toString());
    assertEquals(AnnotationPlan.Severity.INFO, AnnotationPlanner.plan(pass, index).get(0).severity());

    RunResult fail = run(Operation.VERIFY, source, "Demo::Vehicle::tiny");
    assertEquals(Status.FAILED, fail.status(), fail.toString());
    var failPlans = AnnotationPlanner.plan(fail, index);
    assertEquals(AnnotationPlan.Severity.ERROR, failPlans.get(0).severity());
    assertEquals("fail", failPlans.get(0).target().handle());

    RunResult requirement = run(Operation.VERIFY, source, "Demo::Vehicle::massLimit");
    assertEquals(Status.PASSED, requirement.status(), requirement.toString());

    RunResult instance = run(Operation.INSTANTIATE, source, "Demo::Vehicle");
    assertEquals(Status.PASSED, instance.status(), instance.toString());
    assertTrue(instance.outcomes().size() >= 2, instance.toString());

    RunResult states = run(Operation.EXECUTE_STATE, source, "Demo::Lifecycle");
    assertEquals(Status.PASSED, states.status(), states.toString());
    assertEquals(List.of("off"), states.schedule(), states.toString());

    RunResult calc = engine.run(new RunRequest(Operation.EVALUATE_CALC, source, "Demo::twice",
        List.of(new org.openmbee.opensysml.Value.IntegerValue(21))), () -> false);
    assertEquals(Status.PASSED, calc.status(), calc.toString());
    assertEquals("42", calc.outcomes().get(0).detail(), calc.toString());

    RunResult missing = run(Operation.VERIFY, source, "Demo::Nope");
    assertEquals(Status.ERROR, missing.status(), missing.toString());
  }

  private static V2TextualSource demo() throws IOException {
    return new V2TextualSource(Files.readString(Path.of("src/test/resources/fixtures/demo.sysml")));
  }

  /** Run query on the v2 path: the rows a query answers, mapped back to the elements they stand for. */
  @Test
  void v2TextualRunQueryMapsRowsBackToElements() throws Exception {
    V2TextualSource source = demo();
    var index = IdentityIndex.of(List.of(
        new SimpleElement("v", "Demo::Vehicle", "Vehicle", "vehicle"),
        new SimpleElement("w", "Demo::Wheel", "Wheel", "wheel"),
        new SimpleElement("q", "Demo::Parts", "Parts", "query")));

    RunResult parts = run(Operation.RUN_QUERY, source, "Demo::Parts");
    assertEquals(Status.PASSED, parts.status(), parts.toString());
    assertEquals(1, parts.tables().size(), parts.toString());
    RunResult.Table table = parts.tables().get(0);
    assertEquals(List.of("qualifiedName", "name"), table.columns());
    assertEquals(List.of("Demo::Vehicle", "Demo::Wheel"), table.rows().stream().map(RunResult.Row::elementId).toList());
    assertEquals(new RunResult.Cell("Demo::Vehicle", "Demo::Vehicle"), table.rows().get(0).cells().get(0));
    assertEquals(new RunResult.Cell("Vehicle", null), table.rows().get(0).cells().get(1));
    assertEquals(List.of(new RunResult.Outcome("Demo::Parts", "2 rows of qualifiedName, name", Status.PASSED, "Demo::Parts")),
        parts.outcomes());
    List<AnnotationPlan> plans = AnnotationPlanner.plan(parts, index);
    assertEquals(List.of("query", "vehicle", "wheel"), plans.stream().map(plan -> plan.target().handle()).toList());
  }

  /** Run query on the v2 path: the parameters a plan reports, inherited and redefined ones included. */
  @Test
  void v2TextualRunQueryPlansParameters() throws Exception {
    V2TextualSource source = demo();
    QueryPlan plan = engine.plan(new RunRequest(Operation.RUN_QUERY, source, "Demo::NoPorts", List.of()), () -> false).orElseThrow();
    assertEquals(List.of("Demo::NoPorts"), plan.queries());
    assertEquals(List.of(
        new QueryParameter("Demo::NoPorts", "kind", "ScalarValues::String", QueryParameter.Kind.STRING, false, true),
        new QueryParameter("Demo::NoPorts", "root", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, false, false)),
        plan.parameters(), "the redefinition takes its type from the parameter it redefines; the inherited one follows");
    assertEquals("root", plan.soleElementParameter().orElseThrow().name());
    QueryPlan typed = engine.plan(new RunRequest(Operation.RUN_QUERY, source, "Demo::UnderPackages", List.of()), () -> false).orElseThrow();
    assertEquals(List.of(new QueryParameter("Demo::UnderPackages", "roots", "KerML::Root::Element", QueryParameter.Kind.ELEMENT, true, false)),
        typed.parameters(), "a redefinition that types the parameter anew keeps the multiplicity it inherits");
  }

  /** Run query on the v2 path: bindings of many, none and one, and a parameter left unbound. */
  @Test
  void v2TextualRunQueryBindsParameters() throws Exception {
    V2TextualSource source = demo();
    RunResult many = engine.run(new RunRequest(Operation.RUN_QUERY, source, "Demo::UnderPackages", List.of(), Map.of(
        "roots", List.of(new DocumentValue.ElementRef("Demo::Vehicle", ""), new DocumentValue.ElementRef("Demo::Lifecycle", "")))), () -> false);
    assertEquals(Status.PASSED, many.status(), many.toString());
    assertTrue(many.tables().get(0).rows().stream().map(RunResult.Row::elementId).toList().containsAll(
        List.of("Demo::Vehicle::wheel", "Demo::Lifecycle::off")), many.toString());

    RunResult none = engine.run(new RunRequest(Operation.RUN_QUERY, source, "Demo::NoPorts", List.of(),
        Map.of("root", List.of(new DocumentValue.ElementRef("Demo", "")))), () -> false);
    assertEquals(Status.INFO, none.status(), none.toString());
    assertEquals("no rows", none.outcomes().get(0).detail());
    assertEquals(List.of("qualifiedName", "name"), none.tables().get(0).columns());

    RunResult bound = engine.run(new RunRequest(Operation.RUN_QUERY, source, "Demo::PartsUnder", List.of(), Map.of(
        "root", List.of(new DocumentValue.ElementRef("Demo::Vehicle", "")),
        "kind", List.of(new DocumentValue.StringValue("PartUsage")))), () -> false);
    assertEquals(Status.PASSED, bound.status(), bound.toString());
    assertEquals(List.of("Demo::Vehicle::wheel"), bound.tables().get(0).rows().stream().map(RunResult.Row::elementId).toList());

    RunResult unbound = run(Operation.RUN_QUERY, source, "Demo::PartsUnder");
    assertEquals(Status.ERROR, unbound.status(), unbound.toString());
    assertTrue(unbound.outcomes().get(0).detail().contains("requires binding root"), unbound.toString());
  }

  /** Run query on the v2 path: a name the model lacks, a subject that is no query, and a binding no parameter takes. */
  @Test
  void v2TextualRunQueryReportsWhatItCannotRun() throws Exception {
    V2TextualSource source = demo();
    RunResult unresolved = run(Operation.RUN_QUERY, source, "Demo::Unresolved");
    assertEquals(Status.FAILED, unresolved.status(), "a name the model lacks is reported, not refused: " + unresolved);
    assertTrue(unresolved.outcomes().get(0).detail().contains("Demo::Nope"), unresolved.toString());
    assertEquals("Demo", unresolved.outcomes().get(1).elementId(), unresolved.toString());
    assertTrue(unresolved.tables().isEmpty());

    RunResult notAQuery = run(Operation.RUN_QUERY, source, "Demo::Vehicle");
    assertEquals(Status.ERROR, notAQuery.status(), notAQuery.toString());
    assertEquals("Demo::Vehicle is not a calc def specializing DocumentQueries::Query", notAQuery.outcomes().get(0).detail());

    RunResult unknown = engine.run(new RunRequest(Operation.RUN_QUERY, source, "Demo::Parts", List.of(),
        Map.of("depth", List.of(new DocumentValue.IntegerValue(2)))), () -> false);
    assertEquals(Status.ERROR, unknown.status(), unknown.toString());
    assertEquals("no parameter named depth on Demo::Parts", unknown.outcomes().get(0).detail());
  }

  /** Run query over examples/qvts: a path or model error a query reports is FAILED outcomes at the elements the path stopped at. */
  @Test
  void v2TextualRunQueryMapsReportedFailuresToTheirElements() throws Exception {
    Path examples = ServiceBinary.repository().resolve("examples/qvts");
    StringBuilder text = new StringBuilder();
    for (String file : List.of("lander.sysml", "duplicates.sysml", "queries.sysml")) {
      text.append(Files.readString(examples.resolve(file))).append('\n');
    }
    V2TextualSource source = new V2TextualSource(text.toString());
    var index = IdentityIndex.of(List.of(
        new SimpleElement("l", "Lander::lander", "lander", "lander"),
        new SimpleElement("c", "Lander::lander::cabinEngine", "cabinEngine", "cabinEngine"),
        new SimpleElement("e", "Lander::lander::engineBay", "engineBay", "engineBay"),
        new SimpleElement("d", "Duplicates", "Duplicates", "duplicates"),
        new SimpleElement("q", "QVTSDemo::MissingMassProject", "MissingMassProject", "query")));

    RunResult missing = run(Operation.RUN_QUERY, source, "QVTSDemo::MissingMassProject");
    assertEquals(Status.FAILED, missing.status(), missing.toString());
    assertTrue(missing.tables().isEmpty(), missing.toString());
    String query = "QVTSDemo::MissingMassProject";
    assertEquals(new RunResult.Outcome(query, "path error at Feature(mass): expected a feature named mass on the element;"
        + " at Lander::lander::cabinEngine", Status.FAILED, "Lander::lander::cabinEngine"), missing.outcomes().get(1));
    assertEquals(new RunResult.Outcome(query, "path error at Feature(mass): expected a feature named mass on the element;"
        + " at Lander::lander::engineBay", Status.FAILED, "Lander::lander::engineBay"), missing.outcomes().get(2));
    assertEquals(3, missing.outcomes().size(), missing.toString());
    RunResult.Outcome summary = missing.outcomes().get(0);
    assertEquals(query, summary.elementId());
    assertEquals(Status.FAILED, summary.status());
    assertTrue(summary.detail().contains("reads feature mass of Lander::lander::cabinEngine, which has no such feature"
        + " (also absent on Lander::lander::engineBay)"), summary.detail());
    List<AnnotationPlan> plans = AnnotationPlanner.plan(missing, index);
    assertEquals(List.of("query", "cabinEngine", "engineBay"), plans.stream().map(plan -> plan.target().handle()).toList());
    assertTrue(plans.stream().allMatch(plan -> plan.severity() == AnnotationPlan.Severity.ERROR), plans.toString());

    RunResult misspelled = run(Operation.RUN_QUERY, source, "QVTSDemo::MisspelledName");
    assertEquals(Status.FAILED, misspelled.status(), misspelled.toString());
    assertEquals(new RunResult.Outcome("QVTSDemo::MisspelledName",
        "path error at Named(Lander::lander::cabni): expected exactly one element with that qualified name;"
            + " at Lander::lander; members: [cabin (PartUsage), cabinEngine (ConnectionUsage), engine (PartUsage),"
            + " engineBay (ConnectionUsage), scienceBay (PartUsage)]; nearest: cabin",
        Status.FAILED, "Lander::lander"), misspelled.outcomes().get(1));
    assertEquals(List.of("lander"), AnnotationPlanner.plan(misspelled, index).stream().map(plan -> plan.target().handle()).toList(),
        "the query is not in the index, so only the element the path stopped at is annotated");

    RunResult ambiguous = run(Operation.RUN_QUERY, source, "QVTSDemo::AmbiguousPart");
    assertEquals(Status.FAILED, ambiguous.status(), ambiguous.toString());
    assertEquals(new RunResult.Outcome("QVTSDemo::AmbiguousPart",
        "model error at Named(Duplicates::repeated): Duplicates violates ambiguousName: Duplicates::repeated (declared 2 times)",
        Status.FAILED, "Duplicates"), ambiguous.outcomes().get(1));
    assertEquals(List.of("duplicates"), AnnotationPlanner.plan(ambiguous, index).stream().map(plan -> plan.target().handle()).toList());

    RunResult answered = run(Operation.RUN_QUERY, source, "QVTSDemo::CabinMass");
    assertEquals(Status.PASSED, answered.status(), "the queries beside the failing ones still answer: " + answered);
  }

  /** Run query on the v1 path: the migration report leads from a DocGen table structure to the query written for its rows. */
  @Test
  void v1MdzipRunQueryOfADocGenTable() throws Exception {
    Path directory = Files.createTempDirectory("magic-mdk-test");
    Path mdzip = directory.resolve("documents.mdzip");
    try (OutputStream output = Files.newOutputStream(mdzip); ZipOutputStream zip = new ZipOutputStream(output)) {
      zip.putNextEntry(new ZipEntry("com.nomagic.magicdraw.uml_model.model"));
      zip.write(Files.readAllBytes(ServiceBinary.repository().resolve("tests/migrate/testdata/xmi/documents.xmi")));
      zip.closeEntry();
    }
    try (V1MdzipSource source = new V1MdzipSource(mdzip, true)) {
      String table = "Fleet Viewpoints::Parts Viewpoint::Parts Method::Fleet Parts";
      QueryPlan plan = engine.plan(new RunRequest(Operation.RUN_QUERY, source, table, List.of()), () -> false).orElseThrow();
      assertEquals(List.of("Model::Fleet Documents::Fleet Handbook Fleet Parts Rows"), plan.queries());
      assertEquals(List.of("candidates"), plan.parameters().stream().map(QueryParameter::name).toList());
      assertTrue(plan.parameters().get(0).hasValue(), "the migrated query computes its own candidates");

      RunResult rows = run(Operation.RUN_QUERY, source, table);
      assertEquals(Status.PASSED, rows.status(), rows.toString());
      assertEquals(new RunResult.Outcome(table, "1 query", Status.PASSED, table), rows.outcomes().get(0));
      assertEquals("Fleet Documents::Fleet Handbook Fleet Parts Rows", rows.outcomes().get(1).label());
      RunResult.Table answered = rows.tables().get(0);
      assertEquals("Fleet Documents::Fleet Handbook Fleet Parts Rows", answered.query());
      assertTrue(answered.columns().contains("qualifiedName"), answered.toString());
      assertTrue(answered.rows().stream().allMatch(row -> row.elementId() != null && !row.elementId().startsWith("Model::")),
          "rows name the elements as Cameo does: " + answered);

      RunResult document = run(Operation.RUN_QUERY, source, "Fleet Documents::Fleet Handbook");
      assertEquals(Status.PASSED, document.status(), document.toString());
      assertTrue(document.tables().size() > 1, "a Document stands for every query its tables and lists run: " + document.outcomes());

      RunResult view = run(Operation.RUN_QUERY, source, "Fleet Documents::Notes");
      assertEquals(Status.ERROR, view.status(), view.toString());
      assertTrue(view.outcomes().get(0).detail().endsWith("and the migration maps it to no Document, section or table over one"),
          view.toString());
    }
  }

  /**
   * The export of a v1 project: the archive's own symbol stream gives every diagram its view
   * geometry, the migration report lands beside the text, and the temporary export is removed.
   */
  @Test
  void v1MdzipExportWritesDiagramLayoutFromTheProjectStreams() throws Exception {
    Path directory = Files.createTempDirectory("magic-mdk-test");
    Path exportDirectory = Files.createDirectory(directory.resolve("export"));
    Path mdzip = exportDirectory.resolve("requirements.mdzip");
    Path fixtures = ServiceBinary.repository().resolve("tests/migrate/testdata/xmi");
    try (OutputStream output = Files.newOutputStream(mdzip); ZipOutputStream zip = new ZipOutputStream(output)) {
      zip.putNextEntry(new ZipEntry("com.nomagic.magicdraw.uml_model.model"));
      zip.write(Files.readAllBytes(fixtures.resolve("requirement_diagram.xmi")));
      zip.closeEntry();
      zip.putNextEntry(new ZipEntry("BINARY-requirement_diagram"));
      zip.write(Files.readAllBytes(fixtures.resolve("requirement_diagram.stream.xml")));
      zip.closeEntry();
    }
    Path destination = directory.resolve("exported").resolve("Requirements.sysml");

    Optional<ModelExport.Result> cancelled = engine.export(new V1MdzipSource(mdzip), destination, existing -> true, () -> true);
    assertTrue(cancelled.isEmpty());
    assertFalse(Files.exists(destination));

    ModelExport.Result result;
    try (V1MdzipSource source = new V1MdzipSource(mdzip, true)) {
      result = engine.export(source, destination, existing -> true, () -> false).orElseThrow();
    }
    String text = Files.readString(destination);
    assertTrue(text.contains("@DiagramLayout::Canvas"), text);
    assertTrue(text.contains("metadata DiagramLayout::Layout about"), text);
    assertEquals(0, result.errorCount(), result.describe());
    assertTrue(result.diagnostics().stream().anyMatch(item -> "migration-report".equals(item.code())), result.describe());
    Path report = result.report().orElseThrow();
    assertEquals(destination.resolveSibling("Requirements.migration-report.txt"), report);
    assertTrue(Files.readString(report).contains("migrated"), report.toString());
    assertFalse(Files.exists(mdzip), "temporary export is deleted with the source");
    assertFalse(Files.exists(exportDirectory));
  }

  /**
   * The export of a v2 project writes the tool's text as it is, with no report and no files, and
   * carries the service's parse diagnostics on it (the fixture's {@code accept Start} resolves
   * to nothing).
   */
  @Test
  void v2TextualExportWritesTheTextVerbatim() throws Exception {
    String text = Files.readString(Path.of("src/test/resources/fixtures/demo.sysml"));
    Path destination = Files.createTempDirectory("magic-mdk-test").resolve("demo.sysml");
    ModelExport.Result result = engine.export(new V2TextualSource(text), destination, existing -> true, () -> false).orElseThrow();
    assertEquals(text, Files.readString(destination));
    assertEquals(Optional.empty(), result.report());
    assertEquals(List.of(), result.files());
    assertEquals(1, result.errorCount(), result.describe());
    assertTrue(result.diagnostics().stream().anyMatch(item -> item.message().contains("Start")), result.describe());
    assertEquals(List.of("demo.sysml"), result.diagnostics().stream()
        .filter(item -> item.severity() == Diagnostic.Severity.ERROR)
        .map(item -> item.span().orElseThrow().file()).distinct().toList(), "diagnostics name the saved file");
  }

  @Test
  void cancellationBeforeAndDuringRun() throws Exception {
    V2TextualSource source = new V2TextualSource(Files.readString(Path.of("src/test/resources/fixtures/demo.sysml")));
    RunRequest request = new RunRequest(Operation.VERIFY, source, "Demo::Vehicle::lightEnough", List.of());
    assertEquals(Status.CANCELLED, engine.run(request, () -> true).status());
    AtomicBoolean cancel = new AtomicBoolean();
    RunResult result = engine.run(request, () -> cancel.getAndSet(true));
    assertEquals(Status.CANCELLED, result.status(), result.toString());
    assertEquals(Status.PASSED, engine.run(request, () -> false).status(), "connection reopens after a cancel");
  }
}
