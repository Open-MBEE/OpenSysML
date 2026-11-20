package org.openmbee.opensysml.repository;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonParseException;
import com.google.gson.JsonParser;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;

/**
 * A client of the standard SysML v2 API: projects, branches, a commit's elements and the commit
 * path. It speaks the same wire as {@code sysml -sync-apply}, both spellings of the paging
 * parameters included, and holds the bearer token to the configured server.
 */
public final class SysmlApiClient {
  static final int COLLECTION_PAGE_SIZE = 200;
  static final int ELEMENTS_PAGE_SIZE = 500;
  private static final int MAX_PAGES = 1000;
  private static final int MAX_REDIRECTS = 10;

  private final ApiConfig config;
  private final URI base;
  private final HttpClient http;

  public SysmlApiClient(ApiConfig config) {
    this.config = Objects.requireNonNull(config);
    this.base = config.checkedBase();
    // Redirects are followed here, so each hop is held to the token's rule.
    this.http = HttpClient.newBuilder()
        .connectTimeout(config.timeout())
        .followRedirects(HttpClient.Redirect.NEVER)
        .build();
  }

  public ApiConfig config() {
    return config;
  }

  /** One project as the API describes it; the default branch is empty when the server names none. */
  public record ProjectInfo(String id, String name, String defaultBranch) {}

  /** One branch; the head is empty for a branch no commit has reached. */
  public record BranchInfo(String id, String name, String head) {}

  /** A response body and the page it links as next, if any. */
  private record Answer(String body, Optional<String> next) {}

  public List<ProjectInfo> projects() {
    List<ProjectInfo> projects = new ArrayList<>();
    for (JsonObject item : collection(base + "/projects", "projects")) projects.add(project(item));
    return projects;
  }

  public ProjectInfo project(String id) {
    return project(object(get(projectPath(id)).body(), "project"));
  }

  /** Creates a project; an empty branch leaves the default branch to the server. */
  public ProjectInfo createProject(String name, String branch) {
    JsonObject request = new JsonObject();
    request.addProperty("@type", "Project");
    request.addProperty("name", name);
    if (!branch.isEmpty()) {
      JsonObject wanted = new JsonObject();
      wanted.addProperty("@id", branch);
      wanted.addProperty("name", branch);
      request.add("defaultBranch", wanted);
    }
    ProjectInfo created = project(object(send("POST", base + "/projects", request.toString()).body(), "created project"));
    if (created.id().isEmpty()) throw new ApiException("the server created project " + name + " but named no id for it");
    return created.name().isEmpty() ? new ProjectInfo(created.id(), name, created.defaultBranch()) : created;
  }

  public List<BranchInfo> branches(String project) {
    List<BranchInfo> branches = new ArrayList<>();
    for (JsonObject item : collection(projectPath(project) + "/branches", "branches")) branches.add(branch(item));
    return branches;
  }

  public BranchInfo branch(String project, String branch) {
    return branch(object(get(projectPath(project) + "/branches/" + segment(branch)).body(), "branch"));
  }

  /**
   * A commit's elements, paged through to the end. A server that links its pages is followed as
   * linked; one that ignores paging answers everything at once, and its repeats are dropped.
   */
  public List<JsonObject> elements(String project, String commit) {
    String first = projectPath(project) + "/commits/" + segment(commit) + "/elements?pageSize=" + ELEMENTS_PAGE_SIZE
        + "&page%5Bsize%5D=" + ELEMENTS_PAGE_SIZE;
    List<JsonObject> elements = new ArrayList<>();
    Set<String> seen = new HashSet<>();
    String target = first;
    for (int page = 1; page <= MAX_PAGES; page++) {
      Answer answer = get(target);
      JsonArray items = array(answer.body(), "elements page " + page);
      int fresh = 0;
      String last = "";
      for (JsonElement item : items) {
        if (!item.isJsonObject()) throw new ApiException("elements page " + page + " holds a value that is no element");
        JsonObject element = item.getAsJsonObject();
        last = text(element, "@id");
        if (!last.isEmpty() && !seen.add(last)) continue;
        elements.add(element);
        fresh++;
      }
      if (items.size() > ELEMENTS_PAGE_SIZE || fresh < items.size() || items.isEmpty()) return elements;
      if (answer.next().isPresent() && !answer.next().get().equals(target)) {
        target = answer.next().get();
      } else if (items.size() < ELEMENTS_PAGE_SIZE || last.isEmpty()) {
        return elements;
      } else {
        target = first + "&pageAfter=" + query(last) + "&page%5Bafter%5D=" + query(last);
      }
    }
    throw new ApiException("elements: the server paged on past " + MAX_PAGES + " pages");
  }

  /** Posts a commit request to a branch and returns the id of the commit the server made. */
  public String postChanges(String project, String branch, JsonObject commit) {
    String target = projectPath(project) + "/commits";
    if (!branch.isEmpty()) target += "?branchId=" + query(branch);
    return text(object(send("POST", target, commit.toString()).body(), "commit"), "@id");
  }

  private List<JsonObject> collection(String target, String what) {
    List<JsonObject> out = new ArrayList<>();
    Set<String> seen = new HashSet<>();
    String next = paged(target, "");
    for (int page = 1; page <= MAX_PAGES; page++) {
      Answer answer = get(next);
      JsonArray items = array(answer.body(), what + " page " + page);
      String last = "";
      boolean repeated = false;
      for (JsonElement item : items) {
        if (!item.isJsonObject()) throw new ApiException(what + " page " + page + " holds a value that is no object");
        last = text(item.getAsJsonObject(), "@id");
        if (last.isEmpty() || !seen.add(last)) {
          repeated = true;
          continue;
        }
        out.add(item.getAsJsonObject());
      }
      if (repeated || last.isEmpty()) return out;
      if (answer.next().isPresent() && !answer.next().get().equals(next)) {
        next = answer.next().get();
      } else if (items.size() < COLLECTION_PAGE_SIZE) {
        return out;
      } else {
        next = paged(target, last);
      }
    }
    throw new ApiException(what + ": the server paged on past " + MAX_PAGES + " pages");
  }

  private static String paged(String target, String after) {
    String out = target + (target.contains("?") ? "&" : "?") + "page%5Bsize%5D=" + COLLECTION_PAGE_SIZE
        + "&pageSize=" + COLLECTION_PAGE_SIZE;
    if (!after.isEmpty()) out += "&page%5Bafter%5D=" + query(after) + "&pageAfter=" + query(after);
    return out;
  }

  private Answer get(String target) {
    return send("GET", target, null);
  }

  /** One request, its redirects followed while the token may travel to each target. */
  private Answer send(String method, String target, String body) {
    URI uri = URI.create(target);
    for (int hop = 0; hop <= MAX_REDIRECTS; hop++) {
      HttpRequest.Builder request = HttpRequest.newBuilder(uri).timeout(config.timeout());
      if (config.hasToken()) request.header("Authorization", "Bearer " + config.token());
      request.header("Accept", "application/json");
      if (body == null) {
        request.method(method, HttpRequest.BodyPublishers.noBody());
      } else {
        request.header("Content-Type", "application/json");
        request.method(method, HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8));
      }
      HttpResponse<String> response;
      try {
        response = http.send(request.build(), HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
      } catch (IOException exception) {
        throw new ApiException("could not reach " + config.baseUrl() + ": " + describe(exception), exception);
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new ApiException("the request to " + config.baseUrl() + " was interrupted", interrupted);
      }
      int status = response.statusCode();
      if (status == 307 || status == 308 || (body == null && (status == 301 || status == 302 || status == 303))) {
        Optional<String> location = response.headers().firstValue("Location");
        if (location.isEmpty()) throw new ApiException(method + " " + uri + ": redirected without a Location", status, null);
        URI to = uri.resolve(location.get());
        tokenStays(uri, to, "redirect");
        uri = to;
        continue;
      }
      if (status < 200 || status >= 300) {
        throw new ApiException(method + " " + uri + ": HTTP " + status + excerpt(response.body()), status, null);
      }
      return new Answer(response.body(), nextLink(response, uri));
    }
    throw new ApiException(method + " " + target + ": stopped after " + MAX_REDIRECTS + " redirects");
  }

  /** With a token to carry, a successor request stays on the configured server and off plaintext. */
  private void tokenStays(URI from, URI to, String what) {
    if (!config.hasToken()) return;
    try {
      config.checkUrl(to.toString());
    } catch (ApiException refused) {
      throw new ApiException(what + " from " + from + " refused: " + refused.getMessage());
    }
    if ("https".equalsIgnoreCase(from.getScheme()) && !"https".equalsIgnoreCase(to.getScheme())) {
      throw new ApiException(what + " from " + from + " refused: the token stays on https");
    }
    if (!to.getHost().equalsIgnoreCase(base.getHost()) || port(to) != port(base)) {
      throw new ApiException(what + " from " + from + " refused: the token is for " + base.getAuthority() + " only");
    }
  }

  private static int port(URI uri) {
    if (uri.getPort() != -1) return uri.getPort();
    return "https".equalsIgnoreCase(uri.getScheme()) ? 443 : 80;
  }

  /** The rel="next" target of a Link header, resolved against the request and held to the token's rule. */
  private Optional<String> nextLink(HttpResponse<String> response, URI requested) {
    for (String field : response.headers().allValues("Link")) {
      for (String link : field.split(",")) {
        int semicolon = link.indexOf(';');
        if (semicolon < 0) continue;
        String parameters = link.substring(semicolon + 1);
        if (!parameters.contains("rel=\"next\"") && !parameters.contains("rel=next")) continue;
        String target = link.substring(0, semicolon).strip();
        if (target.startsWith("<") && target.endsWith(">")) target = target.substring(1, target.length() - 1);
        URI to;
        try {
          to = resolve(requested, target);
        } catch (IllegalArgumentException malformed) {
          return Optional.empty();
        }
        tokenStays(requested, to, "next page linked");
        return Optional.of(to.toString());
      }
    }
    return Optional.empty();
  }

  /** RFC 3986 resolution; a query-only reference keeps the whole path, which {@link URI#resolve} does not. */
  private static URI resolve(URI requested, String reference) {
    if (!reference.startsWith("?")) return requested.resolve(reference);
    String whole = requested.toString();
    int end = whole.indexOf('#');
    if (end >= 0) whole = whole.substring(0, end);
    end = whole.indexOf('?');
    if (end >= 0) whole = whole.substring(0, end);
    return URI.create(whole + reference);
  }

  private String projectPath(String project) {
    return base + "/projects/" + segment(project);
  }

  private static ProjectInfo project(JsonObject json) {
    JsonElement branch = json.get("defaultBranch");
    String defaultBranch = branch != null && branch.isJsonObject() ? text(branch.getAsJsonObject(), "@id") : "";
    return new ProjectInfo(text(json, "@id"), text(json, "name"), defaultBranch);
  }

  private static BranchInfo branch(JsonObject json) {
    JsonElement head = json.get("head");
    String commit = head != null && head.isJsonObject() ? text(head.getAsJsonObject(), "@id") : "";
    return new BranchInfo(text(json, "@id"), text(json, "name"), commit);
  }

  static String text(JsonObject json, String name) {
    JsonElement value = json.get(name);
    if (value == null || value instanceof JsonNull || !value.isJsonPrimitive()) return "";
    return value.getAsString();
  }

  private static JsonObject object(String body, String what) {
    JsonElement parsed = parse(body, what);
    if (!parsed.isJsonObject()) throw new ApiException("decode " + what + ": the server answered no JSON object");
    return parsed.getAsJsonObject();
  }

  private static JsonArray array(String body, String what) {
    JsonElement parsed = parse(body, what);
    if (!parsed.isJsonArray()) throw new ApiException("decode " + what + ": the server answered no JSON array");
    return parsed.getAsJsonArray();
  }

  private static JsonElement parse(String body, String what) {
    try {
      return JsonParser.parseString(body);
    } catch (JsonParseException malformed) {
      throw new ApiException("decode " + what + ": " + malformed.getMessage(), malformed);
    }
  }

  private static String segment(String value) {
    return query(value).replace("+", "%20");
  }

  private static String query(String value) {
    return URLEncoder.encode(value, StandardCharsets.UTF_8);
  }

  private static String describe(IOException exception) {
    String message = exception.getMessage();
    return message == null || message.isBlank() ? exception.getClass().getSimpleName().toLowerCase(Locale.ROOT) : message;
  }

  private static String excerpt(String body) {
    String text = body == null ? "" : body.strip();
    if (text.isEmpty()) return "";
    return ": " + (text.length() > 500 ? text.substring(0, 500) + "…" : text);
  }
}
