package org.openmbee.opensysml.mdk.docgen;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * One run as the OpenSysML MDK plugin hands it across the classloader boundary: a map of JDK
 * values, read here into a typed view. The keys are the contract with
 * {@code org.openmbee.opensysml.mdk.bridge.DocGenBridge} in the plugin; change both together.
 */
public record BridgeResult(
    String operation,
    String operationLabel,
    String subject,
    String status,
    long elapsedMillis,
    Optional<String> finalTime,
    List<Outcome> outcomes,
    List<Diagnostic> diagnostics,
    List<String> schedule) {

  public record Outcome(String label, String detail, String status, Optional<String> elementId) {}

  public record Diagnostic(String severity, String code, String message, Optional<String> location) {}

  public BridgeResult {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(operationLabel);
    Objects.requireNonNull(subject);
    Objects.requireNonNull(status);
    Objects.requireNonNull(finalTime);
    outcomes = List.copyOf(outcomes);
    diagnostics = List.copyOf(diagnostics);
    schedule = List.copyOf(schedule);
  }

  public static BridgeResult from(Map<String, Object> flat) {
    List<Outcome> outcomes = new ArrayList<>();
    for (Map<String, String> row : rows(flat, "outcomes")) {
      outcomes.add(new Outcome(
          required(row, "label"), required(row, "detail"), required(row, "status"),
          Optional.ofNullable(row.get("elementId"))));
    }
    List<Diagnostic> diagnostics = new ArrayList<>();
    for (Map<String, String> row : rows(flat, "diagnostics")) {
      diagnostics.add(new Diagnostic(
          required(row, "severity"), required(row, "code"), required(row, "message"),
          Optional.ofNullable(row.get("location"))));
    }
    List<String> schedule = new ArrayList<>();
    Object steps = flat.get("schedule");
    if (steps instanceof List<?> list) {
      for (Object step : list) schedule.add(String.valueOf(step));
    }
    Object elapsed = flat.get("elapsedMillis");
    return new BridgeResult(
        text(flat, "operation"), text(flat, "operationLabel"), text(flat, "subject"), text(flat, "status"),
        elapsed instanceof Number number ? number.longValue() : 0L,
        Optional.ofNullable(flat.get("finalTime")).map(String::valueOf),
        outcomes, diagnostics, schedule);
  }

  private static List<Map<String, String>> rows(Map<String, Object> flat, String key) {
    List<Map<String, String>> rows = new ArrayList<>();
    Object value = flat.get(key);
    if (!(value instanceof List<?> list)) return rows;
    for (Object item : list) {
      if (item instanceof Map<?, ?> map) {
        Map<String, String> row = new java.util.LinkedHashMap<>();
        map.forEach((k, v) -> row.put(String.valueOf(k), v == null ? null : String.valueOf(v)));
        rows.add(row);
      }
    }
    return rows;
  }

  private static String text(Map<String, Object> flat, String key) {
    Object value = flat.get(key);
    if (value == null) throw new IllegalArgumentException("bridge result lacks " + key);
    return String.valueOf(value);
  }

  private static String required(Map<String, String> row, String key) {
    String value = row.get(key);
    if (value == null) throw new IllegalArgumentException("bridge result row lacks " + key);
    return value;
  }
}
