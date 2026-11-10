package org.openmbee.mdk.magic.results;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.function.UnaryOperator;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.model.ModelPath;

/** What one run produced, shaped for the results window and the annotation planner. */
public record RunResult(
    Operation operation,
    ModelPath path,
    String subject,
    Status status,
    List<Outcome> outcomes,
    List<Diagnostic> diagnostics,
    Optional<String> finalTime,
    List<String> schedule,
    List<Table> tables,
    List<View> views,
    Duration elapsed) {
  public enum Status {
    PASSED,
    FAILED,
    INCONCLUSIVE,
    ERROR,
    CANCELLED,
    INFO
  }

  /** One row of the outcome list; {@code elementId} is the service symbol id, or null. */
  public record Outcome(String label, String detail, Status status, String elementId) {
    public Outcome {
      Objects.requireNonNull(label);
      Objects.requireNonNull(detail);
      Objects.requireNonNull(status);
    }
  }

  /** The rows a query answered: one table per query, its columns the properties projected. */
  public record Table(String query, List<String> columns, List<Row> rows) {
    public Table {
      Objects.requireNonNull(query);
      columns = List.copyOf(columns);
      rows = List.copyOf(rows);
    }
  }

  /** One row; {@code elementId} is the service symbol id of the element it stands for, or null. */
  public record Row(String elementId, List<Cell> cells) {
    public Row {
      cells = List.copyOf(cells);
    }
  }

  /** One cell; {@code elementId} is the element the value names, or null for a plain value. */
  public record Cell(String text, String elementId) {
    public Cell {
      Objects.requireNonNull(text);
    }
  }

  /**
   * A query's answer as a {@code QVTS::QueryResult} view: {@code name} is the view's qualified
   * name and {@code text} the SysML declaring it. {@code header}, {@code columns} and {@code rows}
   * are the view as {@code RenderView} lays it out, empty until it was rendered.
   */
  public record View(
      String query, String name, String text, List<Field> header, List<String> columns, List<List<String>> rows) {
    public View {
      Objects.requireNonNull(query);
      Objects.requireNonNull(name);
      Objects.requireNonNull(text);
      header = List.copyOf(header);
      columns = List.copyOf(columns);
      rows = rows.stream().map(List::copyOf).toList();
    }

    public View(String query, String name, String text) {
      this(query, name, text, List.of(), List.of(), List.of());
    }
  }

  /** One attribute a result view binds: query, source, outcome or provenance. */
  public record Field(String name, String value) {
    public Field {
      Objects.requireNonNull(name);
      Objects.requireNonNull(value);
    }
  }

  public RunResult(
      Operation operation,
      ModelPath path,
      String subject,
      Status status,
      List<Outcome> outcomes,
      List<Diagnostic> diagnostics,
      Optional<String> finalTime,
      List<String> schedule,
      List<Table> tables,
      Duration elapsed) {
    this(operation, path, subject, status, outcomes, diagnostics, finalTime, schedule, tables, List.of(), elapsed);
  }

  public RunResult(
      Operation operation,
      ModelPath path,
      String subject,
      Status status,
      List<Outcome> outcomes,
      List<Diagnostic> diagnostics,
      Optional<String> finalTime,
      List<String> schedule,
      Duration elapsed) {
    this(operation, path, subject, status, outcomes, diagnostics, finalTime, schedule, List.of(), elapsed);
  }

  public RunResult {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(path);
    Objects.requireNonNull(subject);
    Objects.requireNonNull(status);
    outcomes = List.copyOf(outcomes);
    diagnostics = List.copyOf(diagnostics);
    finalTime = Objects.requireNonNull(finalTime);
    schedule = List.copyOf(schedule);
    tables = List.copyOf(tables);
    views = List.copyOf(views);
    Objects.requireNonNull(elapsed);
  }

  /** The number of rows over every table. */
  public int rowCount() {
    return tables.stream().mapToInt(table -> table.rows().size()).sum();
  }

  /** The result with every outcome label and element id, and every table's query and element ids, renamed by {@code rename}. */
  public RunResult withOutcomeNames(UnaryOperator<String> rename) {
    List<Outcome> renamed = outcomes.stream()
        .map(outcome -> new Outcome(rename.apply(outcome.label()), outcome.detail(), outcome.status(),
            outcome.elementId() == null ? null : rename.apply(outcome.elementId())))
        .toList();
    List<Table> renamedTables = tables.stream()
        .map(table -> new Table(rename.apply(table.query()), table.columns(),
            table.rows().stream().map(row -> rename(row, rename)).toList()))
        .toList();
    return new RunResult(
        operation, path, subject, status, renamed, diagnostics, finalTime, schedule, renamedTables, views, elapsed);
  }

  private static Row rename(Row row, UnaryOperator<String> rename) {
    List<Cell> cells = row.cells().stream()
        .map(cell -> cell.elementId() == null ? cell : new Cell(
            cell.text().equals(cell.elementId()) ? rename.apply(cell.text()) : cell.text(),
            rename.apply(cell.elementId())))
        .toList();
    return new Row(row.elementId() == null ? null : rename.apply(row.elementId()), cells);
  }

  public RunResult withLeadingDiagnostics(List<Diagnostic> leading) {
    if (leading.isEmpty()) return this;
    List<Diagnostic> merged = new ArrayList<>(leading);
    merged.addAll(diagnostics);
    return new RunResult(
        operation, path, subject, status, outcomes, merged, finalTime, schedule, tables, views, elapsed);
  }

  /** The result with {@code trailing} after its diagnostics. */
  public RunResult withDiagnostics(List<Diagnostic> trailing) {
    if (trailing.isEmpty()) return this;
    List<Diagnostic> merged = new ArrayList<>(diagnostics);
    merged.addAll(trailing);
    return new RunResult(
        operation, path, subject, status, outcomes, merged, finalTime, schedule, tables, views, elapsed);
  }

  /** The result with {@code rendered} in place of its views. */
  public RunResult withViews(List<View> rendered) {
    return new RunResult(
        operation, path, subject, status, outcomes, diagnostics, finalTime, schedule, tables, rendered, elapsed);
  }

  public RunResult withElapsed(Duration total) {
    return new RunResult(
        operation, path, subject, status, outcomes, diagnostics, finalTime, schedule, tables, views, total);
  }

  public static RunResult cancelled(RunRequest request) {
    return new RunResult(
        request.operation(), request.source().path(), request.subjectQualifiedName(),
        Status.CANCELLED, List.of(), List.of(), Optional.empty(), List.of(), Duration.ZERO);
  }

  public static RunResult error(RunRequest request, Throwable failure, Duration elapsed) {
    String message = failure.getMessage() == null ? failure.getClass().getName() : failure.getMessage();
    return new RunResult(
        request.operation(), request.source().path(), request.subjectQualifiedName(), Status.ERROR,
        List.of(new Outcome(request.subjectQualifiedName(), message, Status.ERROR, request.subjectQualifiedName())),
        List.of(), Optional.empty(), List.of(), elapsed);
  }
}
