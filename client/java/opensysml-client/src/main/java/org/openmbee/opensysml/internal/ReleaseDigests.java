package org.openmbee.opensysml.internal;

import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.Optional;

/**
 * The SHA-256 digest this client pins for a release asset, keyed by repository, release tag and
 * asset name.
 *
 * <p>Read from the copy of {@code clients/release-digests.json} shipped in this jar: a table
 * resolved at run time from outside the published artifact would not be a pin.
 */
public final class ReleaseDigests {

  /** Where the table sits on the classpath; {@code scripts/sync-release-digests.py} writes it. */
  public static final String RESOURCE = "/release-digests.json";

  private static final ReleaseDigests SHIPPED = load();

  private final Map<?, ?> table;

  private ReleaseDigests(Map<?, ?> table) {
    this.table = table;
  }

  /**
   * The table this jar carries.
   *
   * @return the shipped pins, empty when the resource is missing or unreadable
   */
  public static ReleaseDigests shipped() {
    return SHIPPED;
  }

  /**
   * A table built in memory, for a test standing in for the shipped one.
   *
   * @param table repository to release tag to asset name to hex digest
   * @return the pins
   */
  public static ReleaseDigests of(Map<?, ?> table) {
    return new ReleaseDigests(table);
  }

  /**
   * The digest pinned for one asset of one release.
   *
   * @param githubRepo repository (owner/repo)
   * @param version release tag, resolved (never {@code latest})
   * @param asset asset name (e.g. {@code sysml-grpc-linux-amd64})
   * @return the hex digest, or empty when nothing is pinned for it
   */
  public Optional<String> pin(String githubRepo, String version, String asset) {
    Object releases = table.get(githubRepo);
    Object assets = releases instanceof Map<?, ?> map ? map.get(version) : null;
    Object digest = assets instanceof Map<?, ?> map ? map.get(asset) : null;
    return digest instanceof String hex ? Optional.of(hex) : Optional.empty();
  }

  /**
   * Whether the table holds any pin at all.
   *
   * @return {@code true} when it is empty, which a jar missing the resource is
   */
  public boolean isEmpty() {
    return table.isEmpty();
  }

  private static ReleaseDigests load() {
    try (InputStream in = ReleaseDigests.class.getResourceAsStream(RESOURCE)) {
      if (in == null) {
        return new ReleaseDigests(Map.of());
      }
      Object parsed = Json.parse(new String(in.readAllBytes(), StandardCharsets.UTF_8));
      return new ReleaseDigests(parsed instanceof Map<?, ?> map ? map : Map.of());
    } catch (IOException | IllegalArgumentException e) {
      // A jar without a readable table pins nothing, which refuses downloads rather than
      // failing to load the client.
      return new ReleaseDigests(Map.of());
    }
  }
}
