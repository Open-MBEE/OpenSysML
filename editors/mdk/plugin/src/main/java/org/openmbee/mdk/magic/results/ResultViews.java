package org.openmbee.mdk.magic.results;

import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Reads what the engine's result-view text declares: one package holding one view. */
public final class ResultViews {
  private static final Pattern PACKAGE = Pattern.compile("^package (\\w+) \\{", Pattern.MULTILINE);
  private static final Pattern VIEW = Pattern.compile("^\\s*view (\\w+) : QueryResult \\{", Pattern.MULTILINE);

  private ResultViews() {}

  /** The qualified name of the view {@code text} declares; empty for text that declares none. */
  public static Optional<String> named(String text) {
    Matcher owner = PACKAGE.matcher(text);
    Matcher view = VIEW.matcher(text);
    if (!owner.find() || !view.find()) return Optional.empty();
    return Optional.of(owner.group(1) + "::" + view.group(1));
  }

  /** The file a view is written to beside {@code stem}'s model: named by the query, {@code ::} flattened. */
  public static String fileName(String stem, String query) {
    return stem + "." + query.replace("::", ".").replaceAll("[^A-Za-z0-9._-]", "_") + ".result.sysml";
  }
}
