package org.openmbee.opensysml.cameo.results;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.engine.RunRequest;
import org.openmbee.opensysml.cameo.model.ModelPath;

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

  public RunResult {
    Objects.requireNonNull(operation);
    Objects.requireNonNull(path);
    Objects.requireNonNull(subject);
    Objects.requireNonNull(status);
    outcomes = List.copyOf(outcomes);
    diagnostics = List.copyOf(diagnostics);
    finalTime = Objects.requireNonNull(finalTime);
    schedule = List.copyOf(schedule);
    Objects.requireNonNull(elapsed);
  }

  public RunResult withLeadingDiagnostics(List<Diagnostic> leading) {
    if (leading.isEmpty()) return this;
    List<Diagnostic> merged = new ArrayList<>(leading);
    merged.addAll(diagnostics);
    return new RunResult(operation, path, subject, status, outcomes, merged, finalTime, schedule, elapsed);
  }

  public RunResult withElapsed(Duration total) {
    return new RunResult(
        operation, path, subject, status, outcomes, diagnostics, finalTime, schedule, total);
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
