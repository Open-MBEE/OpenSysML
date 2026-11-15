package org.openmbee.mdk.magic.docgen;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * One run as the Magic MDK plugin hands it across the classloader boundary: a map of JDK
 * values, read here into a typed view. The keys are the contract with
 * {@code org.openmbee.mdk.magic.bridge.DocGenBridge} in the plugin; change both together.
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
    List<String> schedule,
    List<Table> tables) {

  public record Outcome(String label, String detail, String status, Optional<String> elementId) {}

  public record Diagnostic(String severity, String code, String message, Optional<String> location) {}

  /** The rows one query answered; a cell's {@code elementId} names the element its value stands for. */
  public record Table(String query, List<String> columns, List<Row> rows) {
    public Table {
      Objects.requireNonNull(query);
      columns = List.copyOf(columns);
      rows = List.copyOf(rows);
    }
  }

  public record Row(Optional<String> elementId, List<Cell> cells) {
    public Row {
      Objects.requireNonNull(elementId);
      cells = List.copyOf(cells);
    }
  }

  public record Cell(String text, Optional<String> elementId) {
    public Cell {
      Objects.requireNonNull(text);
      Objects.requireNonNull(elementId);
    }
  }

  public BridgeResult(
      String operation,
      String operationLabel,
      String subject,
      String status,
      long elapsedMillis,
      Optional<String> finalTime,
      List<Outcome> outcomes,
      List<Diagnostic> diagnostics,
      List<String> schedule) {
    this(operation, operationLabel, subject, status, elapsedMillis, finalTime, outcomes, diagnostics, schedule, List.of());
  }

  public BridgeResult {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(operationLabel);
    Objects.requireNonNull(subject);
    Objects.requireNonNull(status);
    Objects.requireNonNull(finalTime);
    outcomes = List.copyOf(outcomes);
    diagnostics = List.copyOf(diagnostics);
    schedule = List.copyOf(schedule);
    tables = List.copyOf(tables);
  }

  private static final String ELEMENT_ID = "elementId";

  public static BridgeResult from(Map<String, Object> flat) {
    List<Outcome> outcomes = rows(flat, "outcomes").stream()
        .map(row -> new Outcome(
            required(row, "label"), required(row, "detail"), required(row, "status"),
            Optional.ofNullable(row.get(ELEMENT_ID))))
        .toList();
    List<Diagnostic> diagnostics = rows(flat, "diagnostics").stream()
        .map(row -> new Diagnostic(
            required(row, "severity"), required(row, "code"), required(row, "message"),
            Optional.ofNullable(row.get("location"))))
        .toList();
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
        outcomes, diagnostics, schedule, tables(flat.get("tables")));
  }

  private static List<Table> tables(Object value) {
    List<Table> tables = new ArrayList<>();
    if (!(value instanceof List<?> list)) return tables;
    for (Object item : list) {
      if (!(item instanceof Map<?, ?> table)) continue;
      List<Row> rows = new ArrayList<>();
      if (table.get("rows") instanceof List<?> flatRows) {
        for (Object flatRow : flatRows) {
          if (!(flatRow instanceof Map<?, ?> row)) continue;
          List<Cell> cells = new ArrayList<>();
          if (row.get("cells") instanceof List<?> flatCells) {
            for (Object flatCell : flatCells) {
              if (flatCell instanceof Map<?, ?> cell) {
                cells.add(new Cell(String.valueOf(cell.get("text")), optionalText(cell.get(ELEMENT_ID))));
              }
            }
          }
          rows.add(new Row(optionalText(row.get(ELEMENT_ID)), cells));
        }
      }
      Object query = table.get("query");
      if (query == null) throw new IllegalArgumentException("bridge result table lacks query");
      tables.add(new Table(String.valueOf(query), strings(table.get("columns")), rows));
    }
    return tables;
  }

  private static List<String> strings(Object value) {
    List<String> strings = new ArrayList<>();
    if (value instanceof List<?> list) {
      for (Object item : list) strings.add(String.valueOf(item));
    }
    return strings;
  }

  private static Optional<String> optionalText(Object value) {
    return value == null ? Optional.empty() : Optional.of(String.valueOf(value));
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
