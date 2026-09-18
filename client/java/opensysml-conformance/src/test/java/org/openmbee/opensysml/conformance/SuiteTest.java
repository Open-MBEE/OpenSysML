package org.openmbee.opensysml.conformance;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.Encoding;
import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Optional;
import java.util.function.UnaryOperator;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/**
 * Runs the shared suite through the client, over both Connect encodings, and shows the comparison
 * is not vacuous: with an answer deliberately corrupted the same scenarios fail.
 */
class SuiteTest {

  private static Path conformance;
  private static List<Scenario> scenarios;

  @BeforeAll
  static void readTheSuite() {
    conformance = Main.conformanceDirectory();
    scenarios = Scenarios.load(conformance.resolve("scenarios"));
    assertTrue(Files.isDirectory(conformance.resolve("fixtures")));
  }

  @Test
  void everyCoveredScenarioPassesOverProtobufBodies() {
    Report.Summary summary = run(Encoding.PROTOBUF, Mutations.NONE);
    assertEquals(0, summary.failed, () -> failures(summary));
    assertEquals(0, summary.errored, () -> failures(summary));
    assertEquals(scenarios.size(), summary.passed + summary.skipped);
    assertTrue(summary.passed > 20, "only " + summary.passed + " scenarios ran");
  }

  @Test
  void theSameScenariosPassOverJsonBodies() {
    Report.Summary protobuf = run(Encoding.PROTOBUF, Mutations.NONE);
    Report.Summary json = run(Encoding.JSON, Mutations.NONE);
    assertEquals(protobuf.passed, json.passed);
    assertEquals(0, json.failed, () -> failures(json));
    assertEquals(0, json.errored, () -> failures(json));
  }

  @Test
  void theSkippedScenariosAreTheOnesV1DoesNotCover() {
    Report.Summary summary = run(Encoding.PROTOBUF, Mutations.NONE);
    List<String> skipped =
        summary.results.stream()
            .filter(result -> result.outcome.equals("skip"))
            .map(result -> result.rpc)
            .distinct()
            .toList();
    skipped.forEach(rpc -> assertTrue(Api.declared(rpc), rpc + " is not an RPC of the service"));
    assertTrue(
        skipped.stream().noneMatch(rpc -> Api.COVERED.contains(rpc) && rpc.equals("Evaluate")),
        "Evaluate is covered and must not be skipped wholesale: " + skipped);
    assertEquals(summary.skipped, summary.results.stream().filter(r -> r.outcome.equals("skip")).count());
  }

  @Test
  void aCorruptedAnswerIsCaught() {
    for (Mutations mutation :
        List.of(Mutations.PERTURB_REALS, Mutations.TRUNCATE_LISTS, Mutations.REWRITE_STRINGS)) {
      Report.Summary summary = run(Encoding.PROTOBUF, mutation);
      assertTrue(summary.failed > 0, mutation + " changed an answer and no scenario noticed");
      assertEquals(0, summary.errored, () -> failures(summary));
    }
  }

  @Test
  void theReportIsWrittenInTheShapeTheGoRunnerWrites(@TempDir Path directory) throws Exception {
    Report report = new Report();
    report.add(run(Encoding.PROTOBUF, Mutations.NONE));
    Path file = directory.resolve("report.json");
    Files.writeString(
        file, new com.google.gson.Gson().toJson(report) + "\n", StandardCharsets.UTF_8);
    String written = Files.readString(file, StandardCharsets.UTF_8);
    assertTrue(written.contains("\"duration_ms\""), written.substring(0, 200));
    assertTrue(written.contains("\"protocols\""));
    assertTrue(written.contains("\"skipped\""));
    assertFalse(written.contains("durationMs"));
  }

  private static Report.Summary run(Encoding encoding, UnaryOperator<com.google.protobuf.Message> mutation) {
    ConnectionOptions options =
        ConnectionOptions.builder().binaryPath(ServiceBinary.required()).encoding(encoding).build();
    ByteArrayOutputStream captured = new ByteArrayOutputStream();
    try (Connection connection = Connection.open(options);
        PrintStream out = new PrintStream(captured, true, StandardCharsets.UTF_8)) {
      Runner runner =
          new Runner(
              encoding == Encoding.JSON ? "connect-json" : "connect",
              connection,
              conformance.resolve("fixtures"),
              out,
              false,
              mutation);
      return runner.runAll(scenarios, Optional.empty());
    }
  }

  private static String failures(Report.Summary summary) {
    StringBuilder message = new StringBuilder();
    for (Report.Result result : summary.results) {
      if (result.outcome.equals("fail") || result.outcome.equals("error")) {
        message.append(result.outcome).append(' ').append(result.id).append(": ");
        message.append(result.reason == null ? result.failures : result.reason).append('\n');
      }
    }
    return message.toString();
  }
}
