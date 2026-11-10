package org.openmbee.mdk.magic.results;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalDouble;
import java.util.stream.Collectors;
import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.DocumentQueryResult;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.results.RunResult.Cell;
import org.openmbee.mdk.magic.results.RunResult.Outcome;
import org.openmbee.mdk.magic.results.RunResult.Row;
import org.openmbee.mdk.magic.results.RunResult.Table;
import org.openmbee.mdk.magic.results.RunResult.Status;

/** Maps Java client results onto {@link RunResult}; the subject always gets an outcome row. */
public final class ResultsMapper {
  private ResultsMapper() {}

  public static RunResult map(RunRequest request, Verification value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    outcomes.add(verdict(value.verdict()));
    value.verifications().forEach(row -> outcomes.add(verification(row)));
    return result(request, status(value.verdict()), outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, Satisfaction value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    value.verdicts().forEach(item -> outcomes.add(verdict(item)));
    value.verifications().forEach(row -> outcomes.add(verification(row)));
    Status status = aggregate(outcomes, value.holds());
    outcomes.add(0, new Outcome(request.subjectQualifiedName(), "satisfaction", status, request.subjectQualifiedName()));
    return result(request, status, outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, Validation value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    outcomes.add(verdict(value.summary()));
    value.verdicts().forEach(item -> outcomes.add(verdict(item)));
    value.verifications().forEach(row -> outcomes.add(verification(row)));
    return result(request, status(value.summary()), outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, ActionRun value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    outcomes.add(new Outcome(request.subjectQualifiedName(), "completed", Status.PASSED, request.subjectQualifiedName()));
    outputs(value.outputs(), outcomes);
    return result(request, Status.PASSED, outcomes, value.diagnostics(), time(value.finalTime()), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, StateRun value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    String last = value.statesVisited().isEmpty() ? "no state entered" : "in " + value.statesVisited().get(value.statesVisited().size() - 1);
    outcomes.add(new Outcome(request.subjectQualifiedName(), last, Status.PASSED, request.subjectQualifiedName()));
    outputs(value.finalContext(), outcomes);
    return result(request, Status.PASSED, outcomes, value.diagnostics(), time(value.finalTime()), value.statesVisited(), elapsed);
  }

  public static RunResult map(RunRequest request, Instantiation value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    outcomes.add(new Outcome(request.subjectQualifiedName(), value.reachable().size() + " reachable instance(s)", Status.PASSED, request.subjectQualifiedName()));
    outcomes.add(instance(value.root()));
    value.reachable().stream().filter(item -> item.id() != value.root().id()).forEach(item -> outcomes.add(instance(item)));
    return result(request, Status.PASSED, outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, Calculation value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    String detail = value.value().map(ResultsMapper::render).orElse("no result value");
    outcomes.add(new Outcome(request.subjectQualifiedName(), detail, Status.PASSED, request.subjectQualifiedName()));
    outputs(value.outputs(), outcomes);
    return result(request, Status.PASSED, outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  public static RunResult map(RunRequest request, Analysis value, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    value.verdicts().forEach(item -> outcomes.add(verdict(item)));
    value.verifications().forEach(row -> outcomes.add(verification(row)));
    Status status = aggregate(outcomes, value.holds());
    outcomes.add(0, new Outcome(request.subjectQualifiedName(), "analysis", status, request.subjectQualifiedName()));
    outputs(value.outputs(), outcomes);
    return result(request, status, outcomes, value.diagnostics(), Optional.empty(), List.of(), elapsed);
  }

  /** One table and outcome per query that answered, an error outcome per query that failed; no rows at all is INFO, any failure ERROR. */
  public static RunResult map(RunRequest request, List<QueryAnswer> answers, Duration elapsed) {
    List<Outcome> outcomes = new ArrayList<>();
    List<Table> tables = new ArrayList<>();
    for (QueryAnswer answer : answers) {
      if (answer.failure().isPresent()) {
        outcomes.add(QueryFailure.outcome(answer.query(), answer.failure().get()));
        continue;
      }
      Table table = table(answer.query(), answer.result().orElseThrow());
      tables.add(table);
      outcomes.add(new Outcome(answer.query(), describe(table), table.rows().isEmpty() ? Status.INFO : Status.PASSED, answer.query()));
    }
    Status status = Status.INFO;
    if (outcomes.stream().anyMatch(item -> item.status() == Status.ERROR)) status = Status.ERROR;
    else if (tables.stream().anyMatch(table -> !table.rows().isEmpty())) status = Status.PASSED;
    String subject = request.source().subjectName(request.subjectQualifiedName());
    if (answers.size() != 1 || !answers.get(0).query().equals(subject)) {
      outcomes.add(0, new Outcome(subject, answers.size() + (answers.size() == 1 ? " query" : " queries"), status, subject));
    }
    return new RunResult(
        request.operation(), request.source().path(), request.subjectQualifiedName(),
        status, outcomes, List.of(), Optional.empty(), List.of(), tables, elapsed);
  }

  static String describe(Table table) {
    if (table.rows().isEmpty()) return "no rows";
    return table.rows().size() + (table.rows().size() == 1 ? " row" : " rows") + " of " + String.join(", ", table.columns());
  }

  /** A query's answer as a table; a cell names an element when its one value is a reference, an object held under one, or a {@code qualifiedName}. */
  static Table table(String query, DocumentQueryResult result) {
    List<Row> rows = new ArrayList<>();
    for (DocumentRow row : result.rows()) {
      List<Cell> cells = new ArrayList<>();
      for (int column = 0; column < result.columns().size(); column++) {
        List<DocumentValue> values = column < row.cells().size() ? row.cells().get(column) : List.of();
        cells.add(new Cell(
            values.stream().map(ResultsMapper::render).collect(Collectors.joining(", ")),
            elementOf(result.columns().get(column), values)));
      }
      rows.add(new Row(row.element().id().isBlank() ? null : row.element().id(), cells));
    }
    return new Table(query, result.columns(), rows);
  }

  private static String elementOf(String column, List<DocumentValue> values) {
    if (values.size() != 1) return null;
    DocumentValue value = values.get(0);
    if (value instanceof DocumentValue.ElementRef ref) return ref.id();
    if (value instanceof DocumentValue.ObjectRef ref) return ref.element().map(DocumentValue.ElementRef::id).orElse(null);
    if (value instanceof DocumentValue.StringValue text && column.equals("qualifiedName")) return text.value();
    return null;
  }

  public static String render(DocumentValue value) {
    if (value instanceof DocumentValue.ElementRef item) return item.id();
    if (value instanceof DocumentValue.ObjectRef item) return item.path().isBlank() ? "#" + item.id() : item.path();
    if (value instanceof DocumentValue.StringValue item) return item.value();
    if (value instanceof DocumentValue.IntegerValue item) return Long.toString(item.value());
    if (value instanceof DocumentValue.BigIntegerValue item) return item.value().toString();
    if (value instanceof DocumentValue.RealValue item) return Double.toString(item.value());
    if (value instanceof DocumentValue.RationalValue item) return item.value().toString();
    if (value instanceof DocumentValue.BooleanValue item) return Boolean.toString(item.value());
    if (value instanceof DocumentValue.InfinityValue) return "*";
    if (value instanceof DocumentValue.QuantityValue item) {
      return item.quantity().magnitude() + item.quantity().unit().map(unit -> " " + unit).orElse("");
    }
    if (value instanceof DocumentValue.DocumentVerdict item) return item.status().isBlank() ? item.kind() : item.status();
    if (value instanceof DocumentValue.DocumentState item) return item.path().isBlank() ? item.name() : item.path();
    if (value instanceof DocumentValue.DocumentEvent item) return item.text().isBlank() ? item.kind() : item.text();
    return value.toString();
  }

  private static Status aggregate(List<Outcome> outcomes, boolean holds) {
    if (outcomes.stream().anyMatch(item -> item.status() == Status.ERROR)) return Status.ERROR;
    return holds ? Status.PASSED : Status.FAILED;
  }

  private static void outputs(Map<String, Value> outputs, List<Outcome> outcomes) {
    outputs.forEach((name, value) -> outcomes.add(new Outcome(name, render(value), Status.INFO, null)));
  }

  private static Outcome instance(Instance instance) {
    return new Outcome("#" + instance.id(), instance.typeSymbolId(), Status.INFO, instance.typeSymbolId());
  }

  private static Outcome verdict(Verdict verdict) {
    String detail = verdict.error().orElseGet(() -> verdict.condition().orElse(verdict.kind()));
    return new Outcome(verdict.element(), detail, status(verdict), verdict.elementId().orElse(verdict.element()));
  }

  private static Outcome verification(VerificationVerdict row) {
    Status status = switch (row.kind()) {
      case VerificationVerdict.PASS -> Status.PASSED;
      case VerificationVerdict.FAIL -> Status.FAILED;
      case VerificationVerdict.ERROR -> Status.ERROR;
      default -> Status.INCONCLUSIVE;
    };
    return new Outcome(row.caseId(), row.detail().orElse(row.kind()), status, row.requirementId().orElse(row.caseId()));
  }

  private static Status status(Verdict verdict) {
    if (verdict.error().isPresent()) return Status.ERROR;
    return verdict.holds() ? Status.PASSED : Status.FAILED;
  }

  private static Optional<String> time(OptionalDouble value) {
    return value.isPresent() ? Optional.of(Double.toString(value.getAsDouble())) : Optional.empty();
  }

  static String render(Value value) {
    if (value instanceof Value.IntegerValue item) return Long.toString(item.value());
    if (value instanceof Value.RealValue item) return Double.toString(item.value());
    if (value instanceof Value.BooleanValue item) return Boolean.toString(item.value());
    if (value instanceof Value.StringValue item) return item.value();
    return value.toString();
  }

  private static RunResult result(
      RunRequest request, Status status, List<Outcome> outcomes, List<Diagnostic> diagnostics,
      Optional<String> finalTime, List<String> schedule, Duration elapsed) {
    return new RunResult(
        request.operation(), request.source().path(), request.subjectQualifiedName(),
        status, outcomes, diagnostics, finalTime, schedule, elapsed);
  }
}
