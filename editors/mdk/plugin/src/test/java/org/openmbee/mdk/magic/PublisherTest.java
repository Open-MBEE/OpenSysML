package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.time.Duration;
import java.util.Map;
import java.util.Optional;
import java.util.prefs.Preferences;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.repository.ApiConfig;
import org.openmbee.mdk.magic.repository.ApiException;
import org.openmbee.mdk.magic.repository.PublishSettings;
import org.openmbee.mdk.magic.repository.Publisher;
import org.openmbee.mdk.magic.repository.SysmlApiClient;

/** A publication against an in-process API server: what it commits, what it leaves, what it refuses. */
class PublisherTest {
  private static final String MODEL = """
      [
        {"@type": "Package", "@id": "P", "declaredName": "P", "ownedMember": [{"@id": "A"}],
         "aliasIds": [], "owner": null, "sysx:statedDefaults": true, "sysx:completedProperties": ["owner"]},
        {"@type": "PartDefinition", "@id": "A", "declaredName": "A", "isAbstract": false, "owner": {"@id": "P"}}
      ]
      """;

  private FakeSysmlApi api;
  private Publisher publisher;

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

  private static Publisher.Target target(String project, String branch) {
    return new Publisher.Target(project, "", branch, Optional.empty(), false, "from the test");
  }

  @Test
  void publishesEveryElementToAnEmptyBranch() {
    Publisher.Result result = publisher.publish(MODEL, target("p1", ""));

    assertEquals("main", result.branch());
    assertEquals(2, result.created());
    assertEquals(0, result.updated());
    assertEquals(api.head("p1", "main"), result.commit().orElseThrow());
    assertEquals(result.head(), result.commit().orElseThrow());
    assertTrue(result.describe().contains("2 created, 0 updated, 0 deleted"), result.describe());

    JsonObject commit = api.posted.get(0);
    assertEquals("Commit", commit.get("@type").getAsString());
    assertEquals("from the test", commit.get("description").getAsString());
    JsonObject first = commit.getAsJsonArray("change").get(0).getAsJsonObject();
    assertEquals("DataVersion", first.get("@type").getAsString());
    assertEquals("P", first.getAsJsonObject("identity").get("@id").getAsString());
    JsonObject payload = first.getAsJsonObject("payload");
    assertEquals("Package", payload.get("@type").getAsString());
    assertTrue(payload.has("owner") && payload.get("owner").isJsonNull(), "a null is stated, not dropped");
    assertFalse(payload.keySet().stream().anyMatch(key -> key.startsWith("sysx:")), payload.toString());
    assertTrue(api.requests.contains("POST /projects/p1/commits?branchId=main"), api.requests.toString());
  }

  @Test
  void publishingAgainCommitsNothing() {
    publisher.publish(MODEL, target("p1", "main"));
    String head = api.head("p1", "main");

    Publisher.Result again = publisher.publish(MODEL, target("p1", "main"));

    assertTrue(again.commit().isEmpty());
    assertEquals(head, again.head());
    assertEquals(head, api.head("p1", "main"));
    assertEquals(1, api.posted.size());
    assertTrue(again.describe().contains("Nothing to publish"), again.describe());
  }

  @Test
  void commitsOnlyWhatChangedAndLeavesTheRestInPlace() {
    publisher.publish(MODEL, target("p1", "main"));
    String changed = MODEL.replace("\"isAbstract\": false", "\"isAbstract\": true")
        .replace("{\"@type\": \"Package\"", "{\"@type\": \"Package\", \"declaredShortName\": null,");
    String onlyA = "[" + changed.substring(changed.indexOf("{\"@type\": \"PartDefinition\""));

    Publisher.Result result = publisher.publish(onlyA, target("p1", "main"));

    assertEquals(0, result.created());
    assertEquals(1, result.updated());
    assertEquals(0, result.deleted());
    assertEquals(1, result.leftInPlace());
    assertTrue(result.describe().contains("left in place"), result.describe());
    assertEquals(1, api.posted.get(1).getAsJsonArray("change").size());
    assertTrue(api.elements("p1", "main").containsKey("P"));
    assertTrue(api.elements("p1", "main").get("A").get("isAbstract").getAsBoolean());
  }

  @Test
  void deletesWhatTheModelDroppedOnlyWhenAsked() {
    publisher.publish(MODEL, target("p1", "main"));
    String onlyP = MODEL.substring(0, MODEL.indexOf("{\"@type\": \"PartDefinition\"")).stripTrailing();
    onlyP = onlyP.substring(0, onlyP.length() - 1) + "]";

    Publisher.Result result = publisher.publish(
        onlyP, new Publisher.Target("p1", "", "main", Optional.empty(), true, ""));

    assertEquals(1, result.deleted());
    assertEquals(0, result.leftInPlace());
    assertEquals(Map.of("P", api.elements("p1", "main").get("P")), api.elements("p1", "main"));
    JsonObject deletion = api.posted.get(1).getAsJsonArray("change").get(0).getAsJsonObject();
    assertTrue(deletion.get("payload").isJsonNull());
    assertFalse(api.posted.get(1).has("description"));
  }

  @Test
  void createsTheProjectWhenNoIdIsGiven() {
    Publisher.Result result = publisher.publish(
        MODEL, new Publisher.Target("", "Rover", "", Optional.empty(), false, ""));

    assertTrue(result.createdProject());
    assertEquals("Rover", result.projectName());
    assertEquals("main", result.branch());
    assertEquals(2, api.elements(result.project(), "main").size());
    assertTrue(result.describe().startsWith("Created project Rover"), result.describe());
    assertThrows(IllegalArgumentException.class,
        () -> new Publisher.Target(" ", "", "", Optional.empty(), false, ""));
  }

  @Test
  void refusesABranchThatMovedSinceItWasLastSeen() {
    Publisher.Result first = publisher.publish(MODEL, target("p1", "main"));
    api.commit("p1", "main", new JsonArray());

    Publisher.BranchMovedException moved = assertThrows(Publisher.BranchMovedException.class, () -> publisher.publish(
        MODEL.replace("false", "true"),
        new Publisher.Target("p1", "", "main", first.commit(), false, "")));

    assertTrue(moved.getMessage().contains("nothing was written"), moved.getMessage());
    assertEquals(1, api.posted.size());
  }

  @Test
  void refusesABranchThatMovesBetweenTheReadAndTheWrite() {
    publisher.publish(MODEL, target("p1", "main"));
    // Another client commits once the publisher has read the branch's elements.
    api.afterElements = () -> api.commit("p1", "main", new JsonArray());

    Publisher.BranchMovedException moved = assertThrows(Publisher.BranchMovedException.class,
        () -> publisher.publish(MODEL.replace("false", "true"), target("p1", "main")));

    assertTrue(moved.getMessage().contains("nothing was written"), moved.getMessage());
    assertEquals(1, api.posted.size());
  }

  @Test
  void refusesAModelItCannotPublishWhole() {
    ApiException unheld = assertThrows(ApiException.class, () -> publisher.publish(
        "[{\"@type\": \"sysx:Thing\", \"@id\": \"T\"}]", target("p1", "main")));
    assertTrue(unheld.getMessage().contains("T (sysx:Thing)"), unheld.getMessage());

    assertThrows(ApiException.class, () -> publisher.publish("[]", target("p1", "main")));
    assertThrows(ApiException.class, () -> publisher.publish("{}", target("p1", "main")));
    assertThrows(ApiException.class, () -> publisher.publish("not json", target("p1", "main")));
    assertThrows(ApiException.class, () -> publisher.publish("[{\"@id\": \"X\"}]", target("p1", "main")));
    assertThrows(ApiException.class, () -> publisher.publish(
        "[{\"@type\":\"Package\",\"@id\":\"X\"},{\"@type\":\"Package\",\"@id\":\"X\"}]", target("p1", "main")));
    assertEquals(404, assertThrows(ApiException.class, () -> publisher.publish(MODEL, target("nope", ""))).status());
    assertEquals(404, assertThrows(ApiException.class, () -> publisher.publish(MODEL, target("p1", "nope"))).status());
    assertEquals(0, api.posted.size());
  }

  @Test
  void readsBackWhatAServerStoresWithoutSeeingAChange() {
    // A server may answer more than was posted and spell "no value" as an absent property.
    publisher.publish(MODEL, target("p1", "main"));
    JsonObject stored = JsonParser.parseString(
        "{\"@type\":\"Package\",\"@id\":\"P\",\"declaredName\":\"P\",\"ownedMember\":[{\"@id\":\"A\"}],\"elementId\":\"P\"}")
        .getAsJsonObject();
    JsonObject identity = new JsonObject();
    identity.addProperty("@id", "P");
    JsonObject version = new JsonObject();
    version.add("identity", identity);
    version.add("payload", stored);
    JsonArray changes = new JsonArray();
    changes.add(version);
    api.commit("p1", "main", changes);

    assertTrue(publisher.publish(MODEL, target("p1", "main")).commit().isEmpty());
  }

  @Test
  void remembersWhereAModelWasPublishedButNeverAToken() throws Exception {
    Preferences root = Preferences.userRoot().node("org/openmbee/mdk/magic/test-" + System.nanoTime());
    try {
      PublishSettings settings = new PublishSettings(root, "/models/Mission.mdzip");
      assertEquals("", settings.url());
      assertTrue(settings.lastSeen("http://localhost:8083", "p1", "main").isEmpty());

      settings.remember("http://localhost:8083", "p1", "main");
      settings.sawHead("http://localhost:8083", "p1", "main", "commit-1");

      PublishSettings reopened = new PublishSettings(root, "/models/Mission.mdzip");
      assertEquals("http://localhost:8083", reopened.url());
      assertEquals("p1", reopened.project());
      assertEquals("main", reopened.branch());
      assertEquals(Optional.of("commit-1"), reopened.lastSeen("http://localhost:8083", "p1", "main"));
      assertTrue(reopened.lastSeen("http://localhost:8083", "p1", "other").isEmpty());
      assertTrue(reopened.lastSeen("http://localhost:8083", "", "").isEmpty());
      assertEquals("", new PublishSettings(root, "/models/Other.mdzip").project());
    } finally {
      root.removeNode();
    }
  }
}
