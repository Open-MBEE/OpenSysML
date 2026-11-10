package org.openmbee.mdk.magic.results;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Reads what the engine's result-view text declares: one package holding one view. */
public final class ResultViews {
  private static final Pattern PACKAGE = Pattern.compile("^package (\\w+) \\{", Pattern.MULTILINE);
  private static final Pattern VIEW = Pattern.compile("^\\s*view (\\w+) : QueryResult \\{", Pattern.MULTILINE);
  private static final Pattern PLAIN = Pattern.compile("\\w+(::\\w+)*");

  private ResultViews() {}

  /** The qualified name of the view {@code text} declares; empty for text that declares none. */
  public static Optional<String> named(String text) {
    Matcher owner = PACKAGE.matcher(text);
    Matcher view = VIEW.matcher(text);
    if (!owner.find() || !view.find()) return Optional.empty();
    return Optional.of(owner.group(1) + "::" + view.group(1));
  }

  /**
   * The file a view is written to beside {@code stem}'s model: named by the query, {@code ::}
   * flattened. A query whose text holds anything else a file name cannot keep also carries a digest
   * of the text, so two queries never share a file.
   */
  public static String fileName(String stem, String query) {
    String flat = query.replace("::", ".");
    String name = PLAIN.matcher(query).matches() ? flat : flat.replaceAll("[^A-Za-z0-9._-]", "_") + "~" + digest(query);
    return stem + "." + name + ".result.sysml";
  }

  private static String digest(String text) {
    try {
      byte[] hash = MessageDigest.getInstance("SHA-256").digest(text.getBytes(StandardCharsets.UTF_8));
      return HexFormat.of().formatHex(hash, 0, 4);
    } catch (NoSuchAlgorithmException exception) {
      throw new IllegalStateException("SHA-256 is required of every Java platform", exception);
    }
  }
}
