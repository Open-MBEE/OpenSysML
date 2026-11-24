package org.openmbee.opensysml.repository;

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
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

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
    return new Publisher.Target(project, "", branch, Publisher.LastSeen.NEVER, false, "from the test");
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
        onlyP, new Publisher.Target("p1", "", "main", Publisher.LastSeen.NEVER, true, ""));

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
        MODEL, new Publisher.Target("", "Rover", "", Publisher.LastSeen.NEVER, false, ""));

    assertTrue(result.createdProject());
    assertEquals("Rover", result.projectName());
    assertEquals("main", result.branch());
    assertEquals(2, api.elements(result.project(), "main").size());
    assertTrue(result.describe().startsWith("Created project Rover"), result.describe());
    assertThrows(IllegalArgumentException.class,
        () -> new Publisher.Target(" ", "", "", Publisher.LastSeen.NEVER, false, ""));
  }

  @Test
  void refusesABranchThatMovedSinceItWasLastSeen() {
    Publisher.Result first = publisher.publish(MODEL, target("p1", "main"));
    api.commit("p1", "main", new JsonArray());

    Publisher.BranchMovedException moved = assertThrows(Publisher.BranchMovedException.class, () -> publisher.publish(
        MODEL.replace("false", "true"),
        new Publisher.Target("p1", "", "main", (project, branch) -> first.commit(), false, "")));

    assertTrue(moved.getMessage().contains("nothing was written"), moved.getMessage());
    assertEquals(1, api.posted.size());
  }

  @Test
  void holdsTheDefaultBranchToWhatWasLastSeenOfIt() {
    Publisher.Result first = publisher.publish(MODEL, target("p1", "main"));
    api.commit("p1", "main", new JsonArray());
    // The branch is left empty; it resolves to main, whose last-seen head still guards it.
    Publisher.LastSeen seen = (project, branch) ->
        project.equals("p1") && branch.equals("main") ? first.commit() : Optional.empty();

    assertThrows(Publisher.BranchMovedException.class, () -> publisher.publish(
        MODEL.replace("false", "true"), new Publisher.Target("p1", "", "", seen, false, "")));
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
  void aCancellationWhileTheBranchIsReadSendsNoCommit() {
    publisher.publish(MODEL, target("p1", "main"));
    boolean[] cancelled = new boolean[1];
    api.afterElements = () -> cancelled[0] = true;

    Optional<Publisher.Result> result = publisher.publish(
        "[" + MODEL.substring(MODEL.indexOf("{\"@type\": \"PartDefinition\"")),
        new Publisher.Target("p1", "", "main", Publisher.LastSeen.NEVER, true, ""), () -> cancelled[0]);

    assertTrue(result.isEmpty());
    assertTrue(cancelled[0], "the branch was read before the cancellation");
    assertEquals(1, api.posted.size());
    assertEquals(2, api.elements("p1", "main").size());
  }

  @Test
  void aCancellationAfterCreatingTheProjectReportsIt() {
    int[] asked = new int[1];

    // The first question is asked before the project is created, the second after.
    Publisher.Result result = publisher.publish(
        MODEL, new Publisher.Target("", "Rover", "", Publisher.LastSeen.NEVER, false, ""), () -> asked[0]++ > 0)
        .orElseThrow();

    assertTrue(result.cancelled());
    assertTrue(result.createdProject());
    assertTrue(result.commit().isEmpty());
    assertEquals("main", result.branch());
    assertEquals(0, api.posted.size());
    assertTrue(result.describe().contains("Created project Rover (" + result.project() + ")"), result.describe());
    assertTrue(result.describe().contains("Cancelled before anything was published"), result.describe());
  }

  @Test
  void refusesAModelItCannotPublishWhole() {
    Publisher.Target main = target("p1", "main");
    ApiException unheld = assertThrows(ApiException.class, () -> publisher.publish(
        "[{\"@type\": \"sysx:Thing\", \"@id\": \"T\"}]", main));
    assertTrue(unheld.getMessage().contains("T (sysx:Thing)"), unheld.getMessage());

    assertThrows(ApiException.class, () -> publisher.publish("[]", main));
    assertThrows(ApiException.class, () -> publisher.publish("{}", main));
    assertThrows(ApiException.class, () -> publisher.publish("not json", main));
    assertThrows(ApiException.class, () -> publisher.publish("[{\"@id\": \"X\"}]", main));
    assertThrows(ApiException.class, () -> publisher.publish(
        "[{\"@type\":\"Package\",\"@id\":\"X\"},{\"@type\":\"Package\",\"@id\":\"X\"}]", main));
    Publisher.Target noProject = target("nope", "");
    assertEquals(404, assertThrows(ApiException.class, () -> publisher.publish(MODEL, noProject)).status());
    Publisher.Target noBranch = target("p1", "nope");
    assertEquals(404, assertThrows(ApiException.class, () -> publisher.publish(MODEL, noBranch)).status());
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
}
