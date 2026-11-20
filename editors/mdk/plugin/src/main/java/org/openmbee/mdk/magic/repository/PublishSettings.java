package org.openmbee.mdk.magic.repository;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.Objects;
import java.util.Optional;
import java.util.prefs.Preferences;

/**
 * What the publish dialog remembers for one Cameo project: the server, project and branch last
 * published to, and the commit each publication left a branch at. The token is never stored.
 */
public final class PublishSettings {
  private static final String URL = "url";
  private static final String PROJECT = "project";
  private static final String BRANCH = "branch";
  private static final String SEEN = "seen.";

  private final Preferences node;

  /** Settings under {@code root} for the model identified by {@code modelKey}, such as its file name. */
  public PublishSettings(Preferences root, String modelKey) {
    Objects.requireNonNull(root);
    this.node = root.node(digest(Objects.requireNonNull(modelKey)));
  }

  /** The settings of the model, in this user's preferences. */
  public static PublishSettings forModel(String modelKey) {
    return new PublishSettings(Preferences.userNodeForPackage(PublishSettings.class), modelKey);
  }

  public String url() {
    return node.get(URL, "");
  }

  public String project() {
    return node.get(PROJECT, "");
  }

  public String branch() {
    return node.get(BRANCH, "");
  }

  public void remember(String url, String project, String branch) {
    node.put(URL, url);
    node.put(PROJECT, project);
    node.put(BRANCH, branch);
  }

  /** The commit this model's last publication left the branch at, when it has been published there. */
  public Optional<String> lastSeen(String url, String project, String branch) {
    if (project.isBlank() || branch.isBlank()) return Optional.empty();
    return Optional.ofNullable(node.get(SEEN + digest(url + "\n" + project + "\n" + branch), null));
  }

  public void sawHead(String url, String project, String branch, String commit) {
    node.put(SEEN + digest(url + "\n" + project + "\n" + branch), commit);
  }

  /** Preference keys and node names are short; a digest names any model, server and branch. */
  private static String digest(String text) {
    try {
      byte[] hash = MessageDigest.getInstance("SHA-256").digest(text.getBytes(StandardCharsets.UTF_8));
      return HexFormat.of().formatHex(hash, 0, 16);
    } catch (NoSuchAlgorithmException impossible) {
      throw new IllegalStateException(impossible);
    }
  }
}
