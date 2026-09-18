package org.openmbee.opensysml.conformance;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.gson.JsonObject;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** Reading the shared scenario files, and refusing ones the runner would misread. */
class ScenariosTest {

  @Test
  void theSharedSuiteIsRead() {
    List<Scenario> scenarios = Scenarios.load(Main.conformanceDirectory().resolve("scenarios"));
    assertFalse(scenarios.isEmpty());
    assertTrue(scenarios.stream().anyMatch(scenario -> scenario.method().equals("GetServerInfo")));
    scenarios.forEach(
        scenario -> {
          assertFalse(scenario.id().isEmpty());
          assertTrue(Api.declared(scenario.method()), scenario.id() + " names " + scenario.rpc());
        });
  }

  @Test
  void aQualifiedRpcNameIsTakenBare() {
    Scenario scenario =
        new Scenario(
            "x",
            "",
            "/sysml.SysMLService/Evaluate",
            List.of(),
            Optional.empty(),
            Optional.empty(),
            new JsonObject(),
            Expectations.response("{}"),
            "f");
    assertEquals("Evaluate", scenario.method());
  }

  @Test
  void aDuplicateIdIsRefused(@TempDir Path directory) throws IOException {
    write(directory.resolve("a.json"), scenarioFile("same"));
    write(directory.resolve("b.json"), scenarioFile("same"));
    IllegalArgumentException refused =
        assertThrows(IllegalArgumentException.class, () -> Scenarios.load(directory));
    assertTrue(refused.getMessage().contains("already declared"), refused.getMessage());
  }

  @Test
  void anUnknownMemberIsRefusedRatherThanIgnored(@TempDir Path directory) throws IOException {
    write(
        directory.resolve("a.json"),
        """
        {"scenarios": [{"id": "x", "rpc": "Evaluate", "expcet": {}}]}
        """);
    IllegalArgumentException refused =
        assertThrows(IllegalArgumentException.class, () -> Scenarios.load(directory));
    assertTrue(refused.getMessage().contains("unknown member expcet"), refused.getMessage());
  }

  @Test
  void aDirectoryWithNoScenarioFilesIsRefused(@TempDir Path directory) {
    assertThrows(IllegalArgumentException.class, () -> Scenarios.load(directory));
  }

  private static String scenarioFile(String id) {
    return "{\"scenarios\": [{\"id\": \"" + id + "\", \"rpc\": \"Evaluate\"}]}";
  }

  private static void write(Path file, String content) throws IOException {
    Files.writeString(file, content, StandardCharsets.UTF_8);
  }
}
