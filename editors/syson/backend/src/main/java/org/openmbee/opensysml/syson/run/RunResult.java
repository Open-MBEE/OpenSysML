package org.openmbee.opensysml.syson.run;

import java.util.List;
import java.util.Map;

import org.eclipse.syson.sysml.Element;

public final class RunResult {
    private final String modelHash;
    private final RunOperation operation;
    private final String target;
    private final boolean ok;
    private final String verdict;
    private final String schedule;
    private final Double finalTime;
    private final List<RunNamedValue> outputs;
    private final List<String> trace;
    private final String resultText;
    private final List<RunDiagnostic> diagnostics;
    private final List<RunVerdict> verdicts;
    private final List<RunInstance> instances;
    private final Map<RunDiagnostic, Element> diagnosticElements;

    public RunResult(String modelHash, RunOperation operation, String target, boolean ok, String verdict, String schedule,
            Double finalTime, List<RunNamedValue> outputs, List<String> trace, String resultText,
            List<RunDiagnostic> diagnostics, List<RunVerdict> verdicts, List<RunInstance> instances,
            Map<RunDiagnostic, Element> diagnosticElements) {
        this.modelHash = modelHash;
        this.operation = operation;
        this.target = target;
        this.ok = ok;
        this.verdict = verdict;
        this.schedule = schedule;
        this.finalTime = finalTime;
        this.outputs = List.copyOf(outputs);
        this.trace = List.copyOf(trace);
        this.resultText = resultText;
        this.diagnostics = List.copyOf(diagnostics);
        this.verdicts = List.copyOf(verdicts);
        this.instances = List.copyOf(instances);
        this.diagnosticElements = Map.copyOf(diagnosticElements);
    }

    public String modelHash() { return modelHash; }
    public RunOperation operation() { return operation; }
    public String target() { return target; }
    public boolean ok() { return ok; }
    public String verdict() { return verdict; }
    public String schedule() { return schedule; }
    public Double finalTime() { return finalTime; }
    public List<RunNamedValue> outputs() { return outputs; }
    public List<String> trace() { return trace; }
    public String resultText() { return resultText; }
    public List<RunDiagnostic> diagnostics() { return diagnostics; }
    public List<RunVerdict> verdicts() { return verdicts; }
    public List<RunInstance> instances() { return instances; }
    public Element elementFor(RunDiagnostic diagnostic) { return diagnosticElements.get(diagnostic); }

    public static RunResult failure(String hash, RunOperation operation, String target, String message) {
        RunDiagnostic diagnostic = new RunDiagnostic("error", message, "", null, null, null, null, null);
        return new RunResult(hash, operation, target, false, null, null, null, List.of(), List.of(), null,
                List.of(diagnostic), List.of(), List.of(), Map.of());
    }
}
