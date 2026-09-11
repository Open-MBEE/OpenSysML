package org.openmbee.opensysml.conformance;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Optional;
import java.util.regex.Pattern;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** The command line, driven the way the CI conformance step drives it, short of the exit. */
class MainTest {

  @Test
  void theDefaultsAreTheRepositorySuiteOverConnect() {
    Main.Options options = Main.parse(new String[0]);
    assertEquals(Main.conformanceDirectory(), options.directory());
    assertTrue(options.binary().isEmpty());
    assertTrue(options.service().isEmpty());
    assertTrue(options.report().isEmpty());
    assertTrue(options.filter().isEmpty());
    assertEquals(List.of("connect"), options.protocols());
    assertEquals(Mutations.NONE, options.mutation());
    assertFalse(options.allowSkips());
    assertFalse(options.verbose());
  }

  @Test
  void everyFlagIsRead() {
    Main.Options options =
        Main.parse(
            new String[] {
              "-dir", "suite",
              "-binary", "bin/sysml-grpc",
              "-service", "localhost:50051",
              "-report", "out/report.json",
              "-run", "^parse/",
              "-protocols", " Connect , connect-json",
              "-mutate", "perturb-reals",
              "-allow-skips",
              "-v"
            });
    assertEquals(Path.of("suite"), options.directory());
    assertEquals(Optional.of(Path.of("bin/sysml-grpc")), options.binary());
    assertEquals(Optional.of("localhost:50051"), options.service());
    assertEquals(Optional.of(Path.of("out/report.json")), options.report());
    assertEquals("^parse/", options.filter().map(Pattern::pattern).orElseThrow());
    assertEquals(List.of("connect", "connect-json"), options.protocols());
    assertEquals(Mutations.PERTURB_REALS, options.mutation());
    assertTrue(options.allowSkips());
    assertTrue(options.verbose());
  }

  @Test
  void aUsageErrorNamesTheFlag() {
    assertMessage("unknown flag -nope", "-nope");
    assertMessage("-report wants a value", "-report");
    assertMessage("this client speaks the Connect protocol only", "-protocols", "connect,grpc");
    assertMessage("unknown protocol http", "-protocols", "http");
    assertMessage("unknown protocol ", "-protocols", "");
    assertThrows(IllegalArgumentException.class, () -> Main.parse(new String[] {"-mutate", "nothing"}));
  }

  @Test
  void aSuiteWithoutFixturesIsRefused(@TempDir Path directory) throws Exception {
    Path scenarios = Files.createDirectories(directory.resolve("scenarios"));
    try (Stream<Path> files = Files.list(Main.conformanceDirectory().resolve("scenarios"))) {
      for (Path file : files.toList()) {
        Files.copy(file, scenarios.resolve(file.getFileName().toString()));
      }
    }
    Main.Options options = Main.parse(new String[] {"-dir", directory.toString()});
    PrintStream out = new PrintStream(new ByteArrayOutputStream());
    IllegalArgumentException refused =
        assertThrows(IllegalArgumentException.class, () -> Main.run(options, out));
    assertTrue(refused.getMessage().startsWith("no fixtures directory at "), refused.getMessage());
  }

  @Test
  void aServiceAddressWithoutAPortIsRefused() {
    Main.Options options = Main.parse(new String[] {"-service", "localhost", "-allow-skips"});
    PrintStream out = new PrintStream(new ByteArrayOutputStream());
    IllegalArgumentException refused =
        assertThrows(IllegalArgumentException.class, () -> Main.run(options, out));
    assertEquals("-service wants host:port, got localhost", refused.getMessage());
  }

  @Test
  void theSuitePassesOverBothEncodingsAndWritesTheReport(@TempDir Path directory) throws Exception {
    Path report = directory.resolve("nested").resolve("report.json");
    ByteArrayOutputStream captured = new ByteArrayOutputStream();
    int status =
        Main.run(
            Main.parse(
                new String[] {
                  "-binary", ServiceBinary.required().toString(),
                  "-protocols", "connect,connect-json",
                  "-report", report.toString(),
                  "-allow-skips"
                }),
            new PrintStream(captured, true, StandardCharsets.UTF_8));
    String out = captured.toString(StandardCharsets.UTF_8);
    assertEquals(0, status, out);
    assertTrue(out.contains("total "), out);
    assertTrue(out.contains(" 0 failed, "), out);
    assertTrue(out.contains(" 0 in error"), out);

    JsonObject written = JsonParser.parseString(Files.readString(report, StandardCharsets.UTF_8)).getAsJsonObject();
    assertEquals(0, written.get("failed").getAsInt());
    assertEquals(0, written.get("errored").getAsInt());
    assertTrue(written.get("passed").getAsInt() > 20, written.toString());
    assertEquals(2, written.getAsJsonArray("protocols").size());
    assertFalse(written.get("service").getAsString().isEmpty());
  }

  @Test
  void skippedScenariosFailTheRunUnlessAllowed() {
    ByteArrayOutputStream captured = new ByteArrayOutputStream();
    int status =
        Main.run(
            Main.parse(new String[] {"-binary", ServiceBinary.required().toString()}),
            new PrintStream(captured, true, StandardCharsets.UTF_8));
    String out = captured.toString(StandardCharsets.UTF_8);
    assertEquals(1, status, out);
    assertTrue(out.contains("pass -allow-skips to accept that"), out);
  }

  @Test
  void aFilterRunsOnlyTheMatchingScenarios() {
    ByteArrayOutputStream captured = new ByteArrayOutputStream();
    int status =
        Main.run(
            Main.parse(
                new String[] {
                  "-binary", ServiceBinary.required().toString(),
                  "-run", "^server_info/",
                  "-allow-skips"
                }),
            new PrintStream(captured, true, StandardCharsets.UTF_8));
    String out = captured.toString(StandardCharsets.UTF_8);
    assertEquals(0, status, out);
    assertTrue(out.contains("total 1 scenarios: 1 passed"), out);
  }

  @Test
  void aCorruptedAnswerFailsTheRun() {
    ByteArrayOutputStream captured = new ByteArrayOutputStream();
    int status =
        Main.run(
            Main.parse(
                new String[] {
                  "-binary", ServiceBinary.required().toString(),
                  "-mutate", "rewrite-strings",
                  "-allow-skips",
                  "-v"
                }),
            new PrintStream(captured, true, StandardCharsets.UTF_8));
    String out = captured.toString(StandardCharsets.UTF_8);
    assertEquals(1, status, out);
    assertFalse(out.contains(" 0 failed, "), out);
  }

  private static void assertMessage(String prefix, String... args) {
    IllegalArgumentException refused =
        assertThrows(IllegalArgumentException.class, () -> Main.parse(args));
    assertTrue(refused.getMessage().startsWith(prefix), refused.getMessage());
  }
}
