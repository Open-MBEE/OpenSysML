package org.openmbee.opensysml.repository;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * An in-process SysML v2 API server holding projects, branches and commits in memory, driven
 * over real HTTP; the editor plugins' tests use it from this module's test jar.
 */
public final class FakeSysmlApi implements AutoCloseable {
  /** How the element listing pages: by Link header, by the Flexo cursor, or not at all. */
  public enum Paging { LINKED, CURSOR, IGNORED }

  private final HttpServer server;
  private final Map<String, JsonObject> projects = new LinkedHashMap<>();
  private final Map<String, Map<String, String>> heads = new LinkedHashMap<>();
  private final Map<String, Map<String, JsonObject>> commits = new LinkedHashMap<>();
  public final List<String> requests = new ArrayList<>();
  public final List<String> authorizations = new ArrayList<>();
  public final List<JsonObject> posted = new ArrayList<>();
  public Paging paging = Paging.CURSOR;
  public int pageLimit = Integer.MAX_VALUE;
  /** Whether a next-page link is the query alone, relative to the listing. */
  public boolean queryOnlyLinks;
  public String requiredToken = "";
  /** Run once, after the next element listing is answered: a concurrent writer. */
  public Runnable afterElements = () -> {};
  private int minted;

  public FakeSysmlApi() throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext("/", this::handle);
    server.start();
  }

  public String url() {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }

  /** Seeds a project with a default branch at no commit. */
  public synchronized void project(String id, String name, String branch) {
    JsonObject project = new JsonObject();
    project.addProperty("@id", id);
    project.addProperty("@type", "Project");
    project.addProperty("name", name);
    JsonObject reference = new JsonObject();
    reference.addProperty("@id", branch);
    project.add("defaultBranch", reference);
    projects.put(id, project);
    heads.computeIfAbsent(id, key -> new LinkedHashMap<>()).put(branch, "");
  }

  public synchronized String head(String project, String branch) {
    return heads.get(project).get(branch);
  }

  public synchronized Map<String, JsonObject> elements(String project, String branch) {
    return new LinkedHashMap<>(commits.getOrDefault(head(project, branch), Map.of()));
  }

  /** Commits changes as another client would, moving the branch. */
  public synchronized String commit(String project, String branch, JsonArray changes) {
    Map<String, JsonObject> next = new LinkedHashMap<>(commits.getOrDefault(heads.get(project).get(branch), Map.of()));
    for (JsonElement change : changes) {
      JsonObject version = change.getAsJsonObject();
      String id = version.getAsJsonObject("identity").get("@id").getAsString();
      if (version.get("payload").isJsonNull()) {
        next.remove(id);
      } else {
        next.put(id, version.getAsJsonObject("payload"));
      }
    }
    String commit = "commit-" + ++minted;
    commits.put(commit, next);
    heads.get(project).put(branch, commit);
    return commit;
  }

  private void handle(HttpExchange exchange) throws IOException {
    try (exchange) {
      String method = exchange.getRequestMethod();
      String path = exchange.getRequestURI().getRawPath();
      String query = exchange.getRequestURI().getRawQuery();
      String body = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
      String authorization = exchange.getRequestHeaders().getFirst("Authorization");
      synchronized (this) {
        requests.add(method + " " + path + (query == null ? "" : "?" + query));
        authorizations.add(authorization == null ? "" : authorization);
      }
      if (!requiredToken.isEmpty() && !("Bearer " + requiredToken).equals(authorization)) {
        answer(exchange, 401, "{\"error\":\"unauthorized\"}");
        return;
      }
      route(exchange, method, path.split("/"), parameters(query), body);
    }
  }

  private void route(HttpExchange exchange, String method, String[] parts, Map<String, String> query, String body)
      throws IOException {
    // parts: "", "projects", id, "branches"|"commits", id, "elements"
    if (parts.length < 2 || !parts[1].equals("projects")) {
      answer(exchange, 404, "{}");
      return;
    }
    if (parts.length == 2 && method.equals("POST")) {
      JsonObject request = JsonParser.parseString(body).getAsJsonObject();
      String branch = request.has("defaultBranch")
          ? request.getAsJsonObject("defaultBranch").get("@id").getAsString() : "main";
      String id;
      synchronized (this) {
        id = "project-" + ++minted;
      }
      project(id, request.get("name").getAsString(), branch);
      answer(exchange, 200, projects.get(id).toString());
      return;
    }
    if (parts.length == 2) {
      JsonArray all = new JsonArray();
      synchronized (this) {
        projects.values().forEach(all::add);
      }
      answer(exchange, 200, all.toString());
      return;
    }
    String project = decode(parts[2]);
    JsonObject known;
    synchronized (this) {
      known = projects.get(project);
    }
    if (known == null) {
      answer(exchange, 404, "{\"error\":\"no project " + project + "\"}");
      return;
    }
    if (parts.length == 3) {
      answer(exchange, 200, known.toString());
    } else if (parts[3].equals("branches") && parts.length == 4) {
      JsonArray all = new JsonArray();
      synchronized (this) {
        heads.get(project).forEach((id, head) -> all.add(branch(id, head)));
      }
      answer(exchange, 200, all.toString());
    } else if (parts[3].equals("branches") && parts.length == 5) {
      String id = decode(parts[4]);
      String head;
      synchronized (this) {
        head = heads.get(project).get(id);
      }
      if (head == null) {
        answer(exchange, 404, "{\"error\":\"no branch " + id + "\"}");
      } else {
        answer(exchange, 200, branch(id, head).toString());
      }
    } else if (parts[3].equals("commits") && parts.length == 4 && method.equals("POST")) {
      JsonObject request = JsonParser.parseString(body).getAsJsonObject();
      String branch = query.getOrDefault("branchId", known.getAsJsonObject("defaultBranch").get("@id").getAsString());
      String commit;
      synchronized (this) {
        posted.add(request);
        commit = commit(project, branch, request.getAsJsonArray("change"));
      }
      JsonObject made = new JsonObject();
      made.addProperty("@id", commit);
      made.addProperty("@type", "Commit");
      answer(exchange, 200, made.toString());
    } else if (parts[3].equals("commits") && parts.length == 6 && parts[5].equals("elements")) {
      elements(exchange, decode(parts[4]), query);
    } else {
      answer(exchange, 404, "{}");
    }
  }

  private void elements(HttpExchange exchange, String commit, Map<String, String> query) throws IOException {
    List<JsonObject> all;
    synchronized (this) {
      if (!commits.containsKey(commit)) {
        answer(exchange, 404, "{\"error\":\"no commit " + commit + "\"}");
        return;
      }
      all = new ArrayList<>(commits.get(commit).values());
    }
    int size = paging == Paging.IGNORED
        ? all.size() : Math.min(pageLimit, Integer.parseInt(query.getOrDefault("pageSize", "100")));
    int start = 0;
    String after = query.get(paging == Paging.LINKED ? "cursor" : "pageAfter");
    if (paging != Paging.IGNORED && after != null) {
      for (int i = 0; i < all.size(); i++) {
        if (all.get(i).get("@id").getAsString().equals(after)) start = i + 1;
      }
    }
    int end = Math.min(all.size(), start + size);
    JsonArray page = new JsonArray();
    all.subList(start, end).forEach(page::add);
    if (paging == Paging.LINKED && end < all.size()) {
      String last = all.get(end - 1).get("@id").getAsString();
      exchange.getResponseHeaders().add(
          "Link", "<" + (queryOnlyLinks ? "" : exchange.getRequestURI().getRawPath()) + "?cursor=" + last
              + ">; rel=\"next\"");
    }
    Runnable concurrent;
    synchronized (this) {
      concurrent = afterElements;
      afterElements = () -> {};
    }
    concurrent.run();
    answer(exchange, 200, page.toString());
  }

  private static JsonObject branch(String id, String head) {
    JsonObject branch = new JsonObject();
    branch.addProperty("@id", id);
    branch.addProperty("@type", "Branch");
    branch.addProperty("name", id);
    if (!head.isEmpty()) {
      JsonObject commit = new JsonObject();
      commit.addProperty("@id", head);
      branch.add("head", commit);
    }
    return branch;
  }

  private static Map<String, String> parameters(String query) {
    Map<String, String> out = new LinkedHashMap<>();
    if (query == null) return out;
    for (String pair : query.split("&")) {
      int equals = pair.indexOf('=');
      if (equals > 0) out.put(decode(pair.substring(0, equals)), decode(pair.substring(equals + 1)));
    }
    return out;
  }

  private static String decode(String text) {
    return URLDecoder.decode(text, StandardCharsets.UTF_8);
  }

  private static void answer(HttpExchange exchange, int status, String body) throws IOException {
    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
    exchange.getResponseHeaders().add("Content-Type", "application/json");
    exchange.sendResponseHeaders(status, bytes.length);
    exchange.getResponseBody().write(bytes);
  }

  @Override
  public void close() {
    server.stop(0);
  }
}
