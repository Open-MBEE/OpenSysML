package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.spi.ToolProvider;
import org.junit.jupiter.api.Test;

/**
 * The compatibility floor: every vendor class the compiled plugin links against must be listed in the
 * public OpenAPI Javadoc of the oldest supported release (editors/mdk/compat/), except the classes of
 * the optional SysML v2 textual path, which may use the newest release's API because PathSelector only
 * reaches them after probing for the service.
 */
class OpenApiFloorTest {
  private static final String FLOOR = "2024x";
  private static final String TARGET = "2026xRefresh1";
  private static final Set<String> V2_ONLY = Set.of(
      "org.openmbee.mdk.magic.v2.V2Exporter",
      "org.openmbee.mdk.magic.v2.V2Elements",
      "org.openmbee.mdk.magic.selection.V2Selections");
  private static final Pattern DEPENDENCY = Pattern.compile("^\\s+(\\S+)\\s+->\\s+(\\S+)");
  private static final Path BASE = Path.of(System.getProperty("basedir", "."));

  @Test
  void everyVendorClassOfTheV1PathExistsOnTheFloorRelease() throws IOException {
    Set<String> floor = index(FLOOR);
    Set<String> target = index(TARGET);
    assertTrue(floor.size() > 2000 && target.size() > floor.size(), "class indexes look truncated");
    Set<String> missing = new TreeSet<>();
    Set<String> v2Seen = new TreeSet<>();
    int vendorReferences = 0;
    for (String line : jdeps().split("\n")) {
      Matcher m = DEPENDENCY.matcher(line);
      if (!m.find()) continue;
      String from = outer(m.group(1));
      String to = m.group(2).replace('$', '.');
      if (!to.startsWith("com.nomagic.") && !to.startsWith("com.dassault_systemes.")) continue;
      vendorReferences++;
      Set<String> allowed = V2_ONLY.contains(from) ? target : floor;
      if (V2_ONLY.contains(from) && !floor.contains(to)) v2Seen.add(to);
      if (!allowed.contains(to)) missing.add(from + " -> " + to);
    }
    assertTrue(vendorReferences > 20, "jdeps reported no vendor references; is target/classes compiled?");
    assertEquals(Set.of(), missing, "vendor classes absent from the " + FLOOR + " OpenAPI (or " + TARGET + " for the v2 path)");
    assertFalse(v2Seen.isEmpty(), "the v2 exemption is stale: its classes no longer need anything newer than " + FLOOR);
  }

  private static String outer(String className) {
    int nested = className.indexOf('$');
    return nested < 0 ? className : className.substring(0, nested);
  }

  private static Set<String> index(String release) throws IOException {
    Path file = BASE.resolve("../compat/openapi-classes-" + release + ".txt").normalize();
    return new HashSet<>(Files.readAllLines(file));
  }

  private static String jdeps() {
    Path classes = BASE.resolve("target/classes");
    assertTrue(Files.isDirectory(classes), "no compiled classes under " + classes);
    ToolProvider jdeps = ToolProvider.findFirst("jdeps").orElseThrow();
    StringWriter out = new StringWriter();
    StringWriter err = new StringWriter();
    int code = jdeps.run(new PrintWriter(out), new PrintWriter(err), List.of("-verbose:class", "-filter:none", classes.toString()).toArray(String[]::new));
    assertEquals(0, code, "jdeps failed: " + err);
    return out.toString();
  }
}
