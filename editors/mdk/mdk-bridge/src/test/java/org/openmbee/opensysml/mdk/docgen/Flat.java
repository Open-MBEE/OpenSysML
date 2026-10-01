package org.openmbee.opensysml.mdk.docgen;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** A bridge-result map as the plugin would hand it over. */
final class Flat {
  private Flat() {}

  static Map<String, Object> verify(String subject, String status) {
    Map<String, Object> flat = new LinkedHashMap<>();
    flat.put("operation", "VERIFY");
    flat.put("operationLabel", "Verify");
    flat.put("path", "V1_MDZIP");
    flat.put("subject", subject);
    flat.put("status", status);
    flat.put("elapsedMillis", 42L);
    flat.put("outcomes", List.of(
        row("label", "req1", "detail", "mass <= 10", "status", status, "elementId", "e1"),
        row("label", "req2", "detail", "power <= 5", "status", "PASSED")));
    flat.put("diagnostics", List.of(
        row("severity", "WARNING", "code", "choice-point", "message", "two guards hold", "location", "a.sysml:3:1")));
    flat.put("schedule", List.of("start", "check", "done"));
    return flat;
  }

  static Map<String, String> row(String... pairs) {
    Map<String, String> row = new LinkedHashMap<>();
    for (int i = 0; i < pairs.length; i += 2) row.put(pairs[i], pairs[i + 1]);
    return row;
  }
}
