package org.openmbee.opensysml.repository;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonParseException;
import com.google.gson.JsonParser;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.function.BooleanSupplier;

/**
 * Publishes a model, given as the API's JSON element array, to one project branch of a SysML v2
 * API server: one commit holding every element the branch lacks or holds differently. The
 * publication is one-way; what the branch holds is never read back into the model.
 */
public final class Publisher {
  private static final String EXTENSION_PREFIX = "sysx:";
  private static final String PROPERTY_TYPE = "@type";

  private final SysmlApiClient client;

  public Publisher(SysmlApiClient client) {
    this.client = Objects.requireNonNull(client);
  }

  /**
   * Where and how a model is published.
   *
   * @param project the project's id; empty creates a project named {@code projectName}
   * @param projectName the name of the project to create when {@code project} is empty
   * @param branch the branch's id; empty is the project's default branch
   * @param lastSeen the commit the last publication of this model left a branch at, asked once
   *     the project and branch are resolved, so an empty branch is held to its default's
   * @param deleteMissing whether elements the branch holds beyond the model are deleted
   * @param message the commit's description
   */
  public record Target(
      String project, String projectName, String branch, LastSeen lastSeen, boolean deleteMissing, String message) {
    public Target {
      Objects.requireNonNull(project);
      Objects.requireNonNull(projectName);
      Objects.requireNonNull(branch);
      Objects.requireNonNull(lastSeen);
      Objects.requireNonNull(message);
      if (project.isBlank() && projectName.isBlank()) {
        throw new IllegalArgumentException("a publication names a project id, or the name of the project to create");
      }
    }
  }

  /** Where a model's publications left the branches they wrote. */
  @FunctionalInterface
  public interface LastSeen {
    /** A model published nowhere yet, or one whose branch may be written wherever it is. */
    LastSeen NEVER = (project, branch) -> Optional.empty();

    Optional<String> commit(String project, String branch);
  }

  /**
   * What a publication did; the commit is empty when the branch already agreed with the model, or
   * when the publication was cancelled after it had created the project, which is then reported
   * so that the next publication goes to it rather than creating another.
   */
  public record Result(
      String project,
      String projectName,
      boolean createdProject,
      String branch,
      Optional<String> commit,
      String head,
      int created,
      int updated,
      int deleted,
      int leftInPlace,
      boolean cancelled) {

    public String describe() {
      StringBuilder text = new StringBuilder();
      if (createdProject) text.append("Created project ").append(projectName).append(" (").append(project).append(").\n");
      if (cancelled) {
        text.append("Cancelled before anything was published to branch ").append(branch).append('.');
      } else if (commit.isEmpty()) {
        text.append("Nothing to publish: branch ").append(branch).append(" of project ").append(project)
            .append(" already agrees with the model.");
      } else {
        text.append("Published to branch ").append(branch).append(" of project ").append(project)
            .append(" as commit ").append(commit.get()).append(": ")
            .append(created).append(" created, ").append(updated).append(" updated, ").append(deleted).append(" deleted.");
      }
      if (leftInPlace > 0) {
        text.append('\n').append(leftInPlace)
            .append(" element(s) the branch holds beyond the model were left in place.");
      }
      return text.toString();
    }
  }

  /** The branch moved since it was last published to or read, so nothing was written. */
  public static final class BranchMovedException extends ApiException {
    private static final long serialVersionUID = 1L;

    BranchMovedException(String branch, String project, String seen, String head) {
      super("branch " + branch + " of " + project + " moved from commit " + display(seen) + " to " + display(head)
          + " since it was last seen; nothing was written. Review the branch, then publish again");
    }

    private static String display(String commit) {
      return commit.isEmpty() ? "no commit" : commit;
    }
  }

  /** Publishes {@code apiJson}, the element array a conversion to {@code api-json} writes. */
  public Result publish(String apiJson, Target target) {
    return publish(apiJson, target, () -> false).orElseThrow();
  }

  /**
   * Publishes as {@link #publish(String, Target)} does, unless {@code cancelled} answers true
   * before the commit is sent: the result is then empty and the branch untouched.
   */
  public Optional<Result> publish(String apiJson, Target target, BooleanSupplier cancelled) {
    Map<String, JsonObject> local = payloads(apiJson);
    if (local.isEmpty()) throw new ApiException("the model holds no element to publish");
    if (cancelled.getAsBoolean()) return Optional.empty();

    boolean createdProject = target.project().isBlank();
    SysmlApiClient.ProjectInfo project = createdProject ? createProject(target) : existingProject(target);
    String branchId = target.branch().isBlank() ? project.defaultBranch() : target.branch().strip();
    if (branchId.isEmpty()) {
      throw new ApiException("project " + project.id() + " names no default branch; name the branch to publish to");
    }
    String head = client.branch(project.id(), branchId).head();
    Optional<String> seen = createdProject ? Optional.empty() : target.lastSeen().commit(project.id(), branchId);
    if (seen.isPresent() && !seen.get().equals(head)) {
      throw new BranchMovedException(branchId, project.id(), seen.get(), head);
    }

    Changes changes = Changes.between(local, held(project.id(), head), target.deleteMissing());
    if (cancelled.getAsBoolean()) return stopped(project, createdProject, branchId, head);
    if (changes.versions.isEmpty()) {
      return Optional.of(new Result(
          project.id(), project.name(), createdProject, branchId, Optional.empty(), head, 0, 0, 0, changes.leftInPlace, false));
    }

    // The API takes no precondition, so the head is read again just before the write.
    String now = client.branch(project.id(), branchId).head();
    if (!now.equals(head)) throw new BranchMovedException(branchId, project.id(), head, now);
    if (cancelled.getAsBoolean()) return stopped(project, createdProject, branchId, head);
    JsonObject commit = new JsonObject();
    commit.addProperty(PROPERTY_TYPE, "Commit");
    commit.add("change", changes.versions);
    if (!target.message().isBlank()) commit.addProperty("description", target.message());
    String made = client.postChanges(project.id(), branchId, commit);
    if (made.isEmpty()) {
      throw new ApiException("the server accepted the commit to branch " + branchId + " of " + project.id()
          + " but named no commit; read the branch before publishing again");
    }
    return Optional.of(new Result(
        project.id(), project.name(), createdProject, branchId, Optional.of(made), made, changes.created, changes.updated, changes.deleted, changes.leftInPlace, false));
  }

  private SysmlApiClient.ProjectInfo createProject(Target target) {
    SysmlApiClient.ProjectInfo project = client.createProject(target.projectName().strip(), target.branch().strip());
    // The server fills in what the create left to it: the default branch.
    return project.defaultBranch().isEmpty() ? named(client.project(project.id()), project) : project;
  }

  private SysmlApiClient.ProjectInfo existingProject(Target target) {
    SysmlApiClient.ProjectInfo project = client.project(target.project().strip());
    if (!project.id().isEmpty()) return project;
    return new SysmlApiClient.ProjectInfo(target.project().strip(), project.name(), project.defaultBranch());
  }

  /** The elements the branch holds at {@code head} by id; none for a branch with no commit. */
  private Map<String, JsonObject> held(String project, String head) {
    Map<String, JsonObject> remote = new LinkedHashMap<>();
    if (head.isEmpty()) return remote;
    for (JsonObject element : client.elements(project, head)) {
      String id = SysmlApiClient.text(element, "@id");
      if (!id.isEmpty()) remote.put(id, element);
    }
    return remote;
  }

  /** The versions one commit carries to bring a branch to the model, and what each of them does. */
  private static final class Changes {
    final JsonArray versions = new JsonArray();
    int created;
    int updated;
    int deleted;
    int leftInPlace;

    static Changes between(Map<String, JsonObject> local, Map<String, JsonObject> remote, boolean deleteMissing) {
      Changes changes = new Changes();
      for (Map.Entry<String, JsonObject> entry : local.entrySet()) {
        JsonObject held = remote.get(entry.getKey());
        if (held != null && !differs(entry.getValue(), held)) continue;
        if (held == null) {
          changes.created++;
        } else {
          changes.updated++;
        }
        changes.versions.add(version(entry.getKey(), entry.getValue()));
      }
      for (String id : remote.keySet()) {
        if (local.containsKey(id)) continue;
        if (deleteMissing) {
          changes.versions.add(version(id, JsonNull.INSTANCE));
          changes.deleted++;
        } else {
          changes.leftInPlace++;
        }
      }
      return changes;
    }

    private static JsonObject version(String id, JsonElement payload) {
      JsonObject identity = new JsonObject();
      identity.addProperty("@id", id);
      JsonObject version = new JsonObject();
      version.addProperty(PROPERTY_TYPE, "DataVersion");
      version.add("identity", identity);
      version.add("payload", payload);
      return version;
    }
  }

  /** A cancellation before the commit: nothing to report, unless a project was created on the way. */
  private static Optional<Result> stopped(
      SysmlApiClient.ProjectInfo project, boolean createdProject, String branch, String head) {
    if (!createdProject) return Optional.empty();
    return Optional.of(new Result(project.id(), project.name(), true, branch, Optional.empty(), head, 0, 0, 0, 0, true));
  }

  private static SysmlApiClient.ProjectInfo named(SysmlApiClient.ProjectInfo read, SysmlApiClient.ProjectInfo created) {
    return new SysmlApiClient.ProjectInfo(
        created.id(), read.name().isEmpty() ? created.name() : read.name(), read.defaultBranch());
  }

  /**
   * The elements by id, each as the payload the server stores: the conversion's {@code sysx:}
   * extension properties have no place in the API and are left out. An element of a class the
   * API has no metaclass for is refused, since no reference to it could be held either.
   */
  static Map<String, JsonObject> payloads(String apiJson) {
    JsonElement parsed;
    try {
      parsed = JsonParser.parseString(apiJson);
    } catch (JsonParseException malformed) {
      throw new ApiException("the converted model is not JSON: " + malformed.getMessage(), malformed);
    }
    if (!parsed.isJsonArray()) throw new ApiException("the converted model is not an array of elements");
    Map<String, JsonObject> out = new LinkedHashMap<>();
    List<String> unheld = new ArrayList<>();
    for (JsonElement item : parsed.getAsJsonArray()) {
      if (!item.isJsonObject()) throw new ApiException("the converted model holds a value that is no element");
      JsonObject element = item.getAsJsonObject();
      String id = SysmlApiClient.text(element, "@id");
      String type = SysmlApiClient.text(element, PROPERTY_TYPE);
      if (id.isEmpty() || type.isEmpty()) throw new ApiException("the converted model holds an element without an @id or an @type");
      if (type.contains(":")) {
        unheld.add(id + " (" + type + ")");
        continue;
      }
      JsonObject payload = new JsonObject();
      for (Map.Entry<String, JsonElement> property : element.entrySet()) {
        if (!property.getKey().startsWith(EXTENSION_PREFIX)) payload.add(property.getKey(), property.getValue());
      }
      if (out.put(id, payload) != null) throw new ApiException("the converted model holds element " + id + " twice");
    }
    if (!unheld.isEmpty()) {
      throw new ApiException("the SysML v2 API has no metaclass for " + unheld.size() + " element(s) of the model, so they cannot be published: "
          + String.join(", ", unheld.subList(0, Math.min(unheld.size(), 10))) + (unheld.size() > 10 ? ", …" : ""));
    }
    return out;
  }

  /** Whether the branch holds any property of the payload differently; what else the server adds is its own. */
  static boolean differs(JsonObject payload, JsonObject held) {
    for (Map.Entry<String, JsonElement> property : payload.entrySet()) {
      JsonElement theirs = held.get(property.getKey());
      if (!same(property.getValue(), theirs == null ? JsonNull.INSTANCE : theirs)) return true;
    }
    return false;
  }

  /** An absent or null value and an empty list are one: servers spell "no value" either way. */
  private static boolean same(JsonElement ours, JsonElement theirs) {
    if (empty(ours) && empty(theirs)) return true;
    return ours.equals(theirs);
  }

  private static boolean empty(JsonElement value) {
    return value.isJsonNull() || (value.isJsonArray() && value.getAsJsonArray().isEmpty());
  }
}
