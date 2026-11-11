package org.openmbee.mdk.magic.docgen;

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

  /** A run whose one query answered with a view; {@code rendered} adds the header and rows RenderView lays out. */
  static Map<String, Object> view(boolean rendered) {
    Map<String, Object> flat = verify("A::Q", "PASSED");
    Map<String, Object> view = new LinkedHashMap<>();
    view.put("query", "A::Q");
    view.put("name", "QResult::q");
    view.put("text", "package QResult {\n\tview q : QueryResult {\n\t\texpose A::a;\n\t}\n}\n");
    view.put("header", rendered
        ? List.of(row("name", "query", "value", "A::Q"), row("name", "source", "value", "a.sysml"),
            row("name", "outcome", "value", "established"), row("name", "provenance", "value", "sha256:1"))
        : List.of());
    view.put("columns", rendered ? List.of("element", "kind", "type", "owner") : List.of());
    view.put("rows", rendered ? List.of(List.of("A::a", "part", "", "A")) : List.of());
    flat.put("views", List.of(view));
    return flat;
  }

  static Map<String, String> row(String... pairs) {
    Map<String, String> row = new LinkedHashMap<>();
    for (int i = 0; i < pairs.length; i += 2) row.put(pairs[i], pairs[i + 1]);
    return row;
  }
}
