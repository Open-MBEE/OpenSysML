package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.zip.ZipEntry;
import java.util.zip.ZipOutputStream;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.cameo.annotations.AnnotationPlan;
import org.openmbee.opensysml.cameo.annotations.AnnotationPlanner;
import org.openmbee.opensysml.cameo.engine.Engine;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.engine.RunRequest;
import org.openmbee.opensysml.cameo.identity.IdentityIndex;
import org.openmbee.opensysml.cameo.model.ModelPath;
import org.openmbee.opensysml.cameo.model.SimpleElement;
import org.openmbee.opensysml.cameo.results.RunResult;
import org.openmbee.opensysml.cameo.results.RunResult.Status;
import org.openmbee.opensysml.cameo.source.ModelSource;
import org.openmbee.opensysml.cameo.source.V1MdzipSource;
import org.openmbee.opensysml.cameo.source.V2TextualSource;

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
    Path directory = Files.createTempDirectory("opensysml-cameo-test");
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
      // The v1 text requirement carries no evaluable condition, so the engine reports an error verdict
      // that still maps back onto the Cameo requirement element as a warning annotation.
      assertEquals(Status.ERROR, requirementResult.status(), requirementResult.toString());
      List<AnnotationPlan> requirementPlans = AnnotationPlanner.plan(requirementResult, index);
      assertEquals("requirement", requirementPlans.get(0).target().handle());
      assertEquals(AnnotationPlan.Severity.WARNING, requirementPlans.get(0).severity());
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
