package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.repository.ApiConfig;
import org.openmbee.mdk.magic.repository.ApiException;
import org.openmbee.mdk.magic.repository.SysmlApiClient;

/** The SysML v2 API client over real HTTP: the wire it speaks, its paging, and where the token may go. */
class SysmlApiClientTest {
  private FakeSysmlApi api;

  @BeforeEach
  void start() throws Exception {
    api = new FakeSysmlApi();
  }

  @AfterEach
  void stop() {
    api.close();
  }

  private SysmlApiClient client(String token) {
    return new SysmlApiClient(new ApiConfig(api.url(), token, Duration.ofSeconds(10), false));
  }

  private static JsonArray creates(int count) {
    JsonArray changes = new JsonArray();
    for (int i = 0; i < count; i++) {
      JsonObject payload = new JsonObject();
      payload.addProperty("@id", "e" + i);
      payload.addProperty("@type", "Package");
      JsonObject identity = new JsonObject();
      identity.addProperty("@id", "e" + i);
      JsonObject version = new JsonObject();
      version.add("identity", identity);
      version.add("payload", payload);
      changes.add(version);
    }
    return changes;
  }

  @Test
  void readsProjectsAndBranches() {
    api.project("p1", "Mission", "main");
    SysmlApiClient client = client("");

    assertEquals(List.of(new SysmlApiClient.ProjectInfo("p1", "Mission", "main")), client.projects());
    assertEquals(new SysmlApiClient.ProjectInfo("p1", "Mission", "main"), client.project("p1"));
    assertEquals(List.of(new SysmlApiClient.BranchInfo("main", "main", "")), client.branches("p1"));
    assertEquals("", client.branch("p1", "main").head());
    assertTrue(api.authorizations.stream().allMatch(String::isEmpty), "no token, no Authorization header");
  }

  @Test
  void createsAProjectWithTheBranchAsked() {
    SysmlApiClient.ProjectInfo created = client("").createProject("Rover", "trunk");

    assertEquals("Rover", created.name());
    assertEquals("trunk", created.defaultBranch());
    assertEquals("", api.head(created.id(), "trunk"));
  }

  @Test
  void sendsTheBearerTokenAndReportsARefusalWithItsStatus() {
    api.project("p1", "Mission", "main");
    api.requiredToken = "s3cret";

    assertEquals("Mission", client("s3cret").project("p1").name());
    assertEquals("Bearer s3cret", api.authorizations.get(0));

    ApiException refused = assertThrows(ApiException.class, () -> client("wrong").project("p1"));
    assertEquals(401, refused.status());
    assertFalse(refused.getMessage().contains("wrong"), refused.getMessage());
  }

  @Test
  void reportsAMissingProjectAs404() {
    ApiException missing = assertThrows(ApiException.class, () -> client("").project("nope"));
    assertEquals(404, missing.status());
  }

  @Test
  void escapesIdsInPathsAndQueries() {
    api.project("a b/c", "Odd", "feature/x y");
    SysmlApiClient client = client("");

    assertEquals("Odd", client.project("a b/c").name());
    assertEquals("feature/x y", client.branch("a b/c", "feature/x y").id());
    JsonObject commit = new JsonObject();
    commit.add("change", creates(1));
    String made = client.postChanges("a b/c", "feature/x y", commit);
    assertEquals(made, api.head("a b/c", "feature/x y"));
  }

  @Test
  void pagesElementsByCursor() {
    api.project("p1", "Mission", "main");
    String commit = api.commit("p1", "main", creates(1203));
    api.paging = FakeSysmlApi.Paging.CURSOR;

    assertEquals(1203, client("").elements("p1", commit).size());
    assertEquals(3, api.requests.stream().filter(request -> request.contains("/elements")).count());
  }

  @Test
  void followsLinkedPagesWhateverTheirSize() {
    api.project("p1", "Mission", "main");
    String commit = api.commit("p1", "main", creates(25));
    api.paging = FakeSysmlApi.Paging.LINKED;
    api.pageLimit = 10;

    List<JsonObject> elements = client("").elements("p1", commit);
    assertEquals(25, elements.size());
    assertEquals("e24", elements.get(24).get("@id").getAsString());
  }

  @Test
  void followsALinkThatIsAQueryAlone() {
    api.project("p1", "Mission", "main");
    String commit = api.commit("p1", "main", creates(25));
    api.paging = FakeSysmlApi.Paging.LINKED;
    api.pageLimit = 10;
    api.queryOnlyLinks = true;

    assertEquals(25, client("").elements("p1", commit).size());
    assertTrue(api.requests.contains("GET /projects/p1/commits/" + commit + "/elements?cursor=e9"), api.requests.toString());
  }

  @Test
  void takesAServerIgnoringPagingAtItsWord() {
    api.project("p1", "Mission", "main");
    String commit = api.commit("p1", "main", creates(1203));
    api.paging = FakeSysmlApi.Paging.IGNORED;

    assertEquals(1203, client("").elements("p1", commit).size());
    assertEquals(1, api.requests.stream().filter(request -> request.contains("/elements")).count());
  }

  @Test
  void refusesToCarryTheTokenToAnotherServer() throws Exception {
    List<String> leaked = new ArrayList<>();
    HttpServer elsewhere = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    elsewhere.createContext("/", exchange -> {
      leaked.add(String.valueOf(exchange.getRequestHeaders().getFirst("Authorization")));
      exchange.sendResponseHeaders(200, -1);
      exchange.close();
    });
    HttpServer redirecting = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    redirecting.createContext("/", exchange -> {
      exchange.getResponseHeaders().add(
          "Location", "http://127.0.0.1:" + elsewhere.getAddress().getPort() + "/projects/p1");
      exchange.sendResponseHeaders(307, -1);
      exchange.close();
    });
    elsewhere.start();
    redirecting.start();
    try {
      String url = "http://127.0.0.1:" + redirecting.getAddress().getPort();
      SysmlApiClient client = new SysmlApiClient(new ApiConfig(url, "s3cret", Duration.ofSeconds(10), false));

      ApiException refused = assertThrows(ApiException.class, () -> client.project("p1"));
      assertTrue(refused.getMessage().contains("the token is for"), refused.getMessage());
      assertFalse(refused.getMessage().contains("s3cret"), refused.getMessage());
      assertEquals(List.of(), leaked);
    } finally {
      redirecting.stop(0);
      elsewhere.stop(0);
    }
  }

  @Test
  void followsARedirectOnTheSameServer() throws Exception {
    api.project("p1", "Mission", "main");
    HttpServer redirecting = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    redirecting.createContext("/", exchange -> {
      exchange.getResponseHeaders().add("Location", api.url() + exchange.getRequestURI().getRawPath());
      exchange.sendResponseHeaders(307, -1);
      exchange.close();
    });
    redirecting.start();
    try {
      // Without a token there is nothing a redirect could carry off.
      String url = "http://127.0.0.1:" + redirecting.getAddress().getPort();
      SysmlApiClient client = new SysmlApiClient(new ApiConfig(url, "", Duration.ofSeconds(10), false));
      assertEquals("Mission", client.project("p1").name());
    } finally {
      redirecting.stop(0);
    }
  }

  @Test
  void reportsAnUnreachableServer() throws Exception {
    String url = api.url();
    api.close();
    SysmlApiClient client = new SysmlApiClient(new ApiConfig(url, "", Duration.ofSeconds(5), false));

    ApiException unreachable = assertThrows(ApiException.class, client::projects);
    assertEquals(0, unreachable.status());
    assertTrue(unreachable.getMessage().startsWith("could not reach " + url), unreachable.getMessage());
  }

  @Test
  void refusesPlaintextToAnotherMachineUnlessOptedIn() {
    ApiConfig remote = new ApiConfig("http://mms.example.org/api/", "t", Duration.ofSeconds(5), false);
    ApiException refused = assertThrows(ApiException.class, () -> new SysmlApiClient(remote));
    assertTrue(refused.getMessage().contains("in the clear"), refused.getMessage());

    assertEquals("http://mms.example.org/api", new ApiConfig("http://mms.example.org/api/", "t", Duration.ZERO, true)
        .checkedBase().toString());
    assertEquals("https", new ApiConfig("https://mms.example.org", "t", Duration.ZERO, false).checkedBase().getScheme());
    for (String local : List.of("http://localhost:8083", "http://127.0.0.1:8083", "http://[::1]:8083", "http://api.localhost")) {
      new ApiConfig(local, "t", Duration.ZERO, false).checkedBase();
    }
    for (String bad : List.of("ftp://mms.example.org", "mms.example.org", "http://", "http://bad host")) {
      assertThrows(ApiException.class, () -> new ApiConfig(bad, "", Duration.ZERO, false).checkedBase(), bad);
    }
  }

  @Test
  void readsTheEnvironmentTheCommandLineReads() {
    ApiConfig defaults = ApiConfig.fromEnvironment(Map.of());
    assertEquals(ApiConfig.DEFAULT_BASE_URL, defaults.baseUrl());
    assertFalse(defaults.hasToken());
    assertFalse(defaults.allowPlainHttp());

    ApiConfig set = ApiConfig.fromEnvironment(Map.of(
        "FLEXO_SYSMLV2_URL", "http://mms.example.org/", "FLEXO_INTEROP_TOKEN", "s3cret", "FLEXO_ALLOW_PLAIN_HTTP", "1"));
    assertEquals("http://mms.example.org", set.baseUrl());
    assertTrue(set.allowPlainHttp());
    assertEquals(ApiConfig.DEFAULT_TIMEOUT, set.timeout());
    assertFalse(set.toString().contains("s3cret"), set.toString());
  }
}
