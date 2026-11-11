package org.openmbee.mdk.magic.bridge;

import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.uml.BaseElement;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.Supplier;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Value;
import org.openmbee.mdk.magic.actions.CalcArguments;
import org.openmbee.mdk.magic.actions.QueryArguments;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.QueryPlan;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.selection.Selection;
import org.openmbee.mdk.magic.selection.SelectionResolver;
import org.openmbee.mdk.magic.source.ModelSource;

/**
 * The entry point MDK's DocGen extension calls, reflectively, from its own classloader: only
 * Cameo and JDK types cross the boundary, so the OpenSysML client stays private to this plugin.
 * Keys and values of the returned map are the contract documented on {@code BridgeResult} in the
 * mdk-bridge module; change both together.
 */
public final class DocGenBridge {
  public static final String METHOD = "docGen";

  private final Supplier<Engine> engine;
  private final SelectionResolver resolver;

  public DocGenBridge(Supplier<Engine> engine) {
    this(engine, new SelectionResolver());
  }

  DocGenBridge(Supplier<Engine> engine, SelectionResolver resolver) {
    this.engine = engine;
    this.resolver = resolver;
  }

  /**
   * Runs {@code operation} (an {@link Operation} name) on {@code element} — a UML element on the v1
   * path or a SysML v2 element on the textual path — and flattens the result.
   *
   * @throws IllegalArgumentException when the operation is unknown, the element is not a model
   *     element OpenSysML can run, or the calc arguments or query bindings do not parse
   */
  public Map<String, Object> run(Project project, BaseElement element, String operation, String calcArguments) {
    Operation parsed = operation(operation);
    List<Value> args = parsed == Operation.EVALUATE_CALC ? CalcArguments.parse(calcArguments) : List.of();
    Selection selection = resolver.resolve(project, element).orElseThrow(() -> new IllegalArgumentException(
        "Magic MDK cannot run " + element.getHumanName() + ": not a model element on a supported path"));
    ModelSource source = selection.export().get();
    RunResult result;
    try {
      RunRequest request = new RunRequest(parsed, source, selection.subject().qualifiedName(), args);
      if (parsed == Operation.RUN_QUERY && calcArguments != null && !calcArguments.isBlank()) {
        QueryPlan plan = engine.get().plan(request, () -> false).orElseThrow();
        request = request.withQueryBindings(QueryArguments.parse(calcArguments, plan));
      }
      result = engine.get().run(request, () -> false);
    } catch (RuntimeException failure) {
      try {
        source.close();
      } catch (RuntimeException closeFailure) {
        failure.addSuppressed(closeFailure);
      }
      throw failure;
    }
    try {
      source.close();
    } catch (RuntimeException closeFailure) {
      result = result.withLeadingDiagnostics(List.of(new Diagnostic(
          Diagnostic.Severity.WARNING,
          "temporary export not removed: " + closeFailure.getMessage(),
          "export-cleanup",
          Optional.empty())));
    }
    return flatten(result);
  }

  public static Operation operation(String name) {
    for (Operation candidate : Operation.values()) {
      if (candidate.name().equals(name)) return candidate;
    }
    throw new IllegalArgumentException("unknown OpenSysML operation: " + name);
  }

  /** The JDK-only shape of a {@link RunResult}; every value is a String, Long, List or Map. */
  public static Map<String, Object> flatten(RunResult result) {
    Map<String, Object> flat = new LinkedHashMap<>();
    flat.put("operation", result.operation().name());
    flat.put("operationLabel", result.operation().label());
    flat.put("path", result.path().name());
    flat.put("subject", result.subject());
    flat.put("status", result.status().name());
    flat.put("elapsedMillis", result.elapsed().toMillis());
    result.finalTime().ifPresent(time -> flat.put("finalTime", time));
    List<Map<String, String>> outcomes = new ArrayList<>();
    for (RunResult.Outcome outcome : result.outcomes()) {
      Map<String, String> row = new LinkedHashMap<>();
      row.put("label", outcome.label());
      row.put("detail", outcome.detail());
      row.put("status", outcome.status().name());
      if (outcome.elementId() != null) row.put("elementId", outcome.elementId());
      outcomes.add(row);
    }
    flat.put("outcomes", outcomes);
    List<Map<String, String>> diagnostics = new ArrayList<>();
    for (Diagnostic diagnostic : result.diagnostics()) {
      Map<String, String> row = new LinkedHashMap<>();
      row.put("severity", diagnostic.severity().name());
      row.put("code", diagnostic.code());
      row.put("message", diagnostic.message());
      diagnostic.span().ifPresent(span ->
          row.put("location", span.file() + ":" + span.startLine() + ":" + span.startColumn()));
      diagnostics.add(row);
    }
    flat.put("diagnostics", diagnostics);
    flat.put("schedule", List.copyOf(result.schedule()));
    List<Map<String, Object>> tables = new ArrayList<>();
    for (RunResult.Table table : result.tables()) {
      Map<String, Object> flatTable = new LinkedHashMap<>();
      flatTable.put("query", table.query());
      flatTable.put("columns", List.copyOf(table.columns()));
      List<Map<String, Object>> rows = new ArrayList<>();
      for (RunResult.Row row : table.rows()) {
        Map<String, Object> flatRow = new LinkedHashMap<>();
        if (row.elementId() != null) flatRow.put("elementId", row.elementId());
        List<Map<String, String>> cells = new ArrayList<>();
        for (RunResult.Cell cell : row.cells()) {
          Map<String, String> flatCell = new LinkedHashMap<>();
          flatCell.put("text", cell.text());
          if (cell.elementId() != null) flatCell.put("elementId", cell.elementId());
          cells.add(flatCell);
        }
        flatRow.put("cells", cells);
        rows.add(flatRow);
      }
      flatTable.put("rows", rows);
      tables.add(flatTable);
    }
    flat.put("tables", tables);
    return flat;
  }
}
