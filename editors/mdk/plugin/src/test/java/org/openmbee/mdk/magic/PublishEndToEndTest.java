package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.gson.JsonObject;
import java.nio.file.Path;
import java.time.Duration;
import java.util.Map;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.repository.ApiConfig;
import org.openmbee.mdk.magic.repository.Publisher;
import org.openmbee.mdk.magic.repository.SysmlApiClient;
import org.openmbee.mdk.magic.source.V2TextualSource;

/** Drives export text → the real service's api-json conversion → a commit on an API server. */
class PublishEndToEndTest {
  private static final String MODEL = """
      package Mission {
        part def Rover {
          attribute mass : ScalarValues::Real = 899.0;
        }
        part rover : Rover;
      }
      """;

  private static Engine engine;
  private FakeSysmlApi api;
  private Publisher publisher;

  @BeforeAll
  static void openEngine() {
    Path binary = ServiceBinary.required();
    engine = new Engine(() -> ConnectionOptions.builder().binaryPath(binary).build());
  }

  @AfterAll
  static void closeEngine() {
    if (engine != null) engine.close();
  }

  @BeforeEach
  void start() throws Exception {
    api = new FakeSysmlApi();
    api.project("p1", "Mission", "main");
    publisher = new Publisher(new SysmlApiClient(new ApiConfig(api.url(), "", Duration.ofSeconds(10), false)));
  }

  @AfterEach
  void stop() {
    api.close();
  }

  private static Publisher.Target target(boolean deleteMissing) {
    return new Publisher.Target("p1", "", "main", Publisher.LastSeen.NEVER, deleteMissing, "");
  }

  @Test
  void publishesAModelThenOnlyItsChanges() {
    Publisher.Result first = engine.publish(new V2TextualSource(MODEL), publisher, target(false), () -> false).orElseThrow();

    Map<String, JsonObject> held = api.elements("p1", "main");
    assertEquals(held.size(), first.created());
    assertTrue(held.values().stream().anyMatch(element -> element.get("@type").getAsString().equals("PartDefinition")
        && element.get("declaredName").getAsString().equals("Rover")), held.keySet().toString());
    assertTrue(held.values().stream().noneMatch(
        element -> element.keySet().stream().anyMatch(key -> key.startsWith("sysx:"))));

    Publisher.Result again = engine.publish(new V2TextualSource(MODEL), publisher, target(false), () -> false).orElseThrow();
    assertTrue(again.commit().isEmpty(), again.describe());

    String renamed = MODEL.replace("part rover : Rover;", "part rover : Rover;\n  part spare : Rover;");
    Publisher.Result grown = engine.publish(new V2TextualSource(renamed), publisher, target(false), () -> false).orElseThrow();
    assertTrue(grown.created() > 0, grown.describe());
    assertTrue(grown.created() < first.created(), grown.describe());
    assertEquals(0, grown.deleted());

    Publisher.Result shrunk = engine.publish(new V2TextualSource(MODEL), publisher, target(true), () -> false).orElseThrow();
    assertEquals(grown.created(), shrunk.deleted(), shrunk.describe());
    assertEquals(held.keySet(), api.elements("p1", "main").keySet());
  }

  @Test
  void refusesAModelWithErrors() {
    IllegalStateException refused = assertThrows(IllegalStateException.class,
        () -> engine.publish(new V2TextualSource("package Broken { part def"), publisher, target(false), () -> false));

    assertTrue(refused.getMessage().contains("was not published"), refused.getMessage());
    assertEquals(0, api.posted.size());
    assertFalse(api.requests.stream().anyMatch(request -> request.startsWith("POST")));
  }

  @Test
  void aCancelledPublicationSendsNothing() {
    assertTrue(engine.publish(new V2TextualSource(MODEL), publisher, target(false), () -> true).isEmpty());
    assertEquals(0, api.requests.size());
  }
}
