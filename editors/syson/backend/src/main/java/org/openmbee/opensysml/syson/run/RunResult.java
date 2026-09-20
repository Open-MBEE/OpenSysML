package org.openmbee.opensysml.syson.run;

import java.util.List;

import org.eclipse.syson.sysml.Element;

public final class RunResult {
    public record MappedDiagnostic(RunDiagnostic diagnostic, Element element) {
    }

    private final String modelHash;
    private final RunOperation operation;
    private final String target;
    private final boolean ok;
    private final String verdict;
    private final String schedule;
    private final Double finalTime;
    private final List<RunNamedValue> outputs;
    private final List<String> trace;
    private final List<RunOutcome> outcomes;
    private final String resultText;
    private final List<RunVerdict> verdicts;
    private final List<RunInstance> instances;
    private final List<MappedDiagnostic> mappedDiagnostics;

    public RunResult(String modelHash, RunOperation operation, String target, boolean ok, String verdict, String schedule,
            Double finalTime, List<RunNamedValue> outputs, List<String> trace, String resultText,
            List<RunOutcome> outcomes, List<RunVerdict> verdicts, List<RunInstance> instances,
            List<MappedDiagnostic> mappedDiagnostics) {
        this.modelHash = modelHash;
        this.operation = operation;
        this.target = target;
        this.ok = ok;
        this.verdict = verdict;
        this.schedule = schedule;
        this.finalTime = finalTime;
        this.outputs = List.copyOf(outputs);
        this.trace = List.copyOf(trace);
        this.outcomes = List.copyOf(outcomes);
        this.resultText = resultText;
        this.verdicts = List.copyOf(verdicts);
        this.instances = List.copyOf(instances);
        this.mappedDiagnostics = List.copyOf(mappedDiagnostics);
    }

    public RunResult(String modelHash, RunOperation operation, String target, boolean ok, String verdict, String schedule,
            Double finalTime, List<RunNamedValue> outputs, List<String> trace, String resultText,
            List<RunVerdict> verdicts, List<RunInstance> instances, List<MappedDiagnostic> mappedDiagnostics) {
        this(modelHash, operation, target, ok, verdict, schedule, finalTime, outputs, trace, resultText, List.of(),
                verdicts, instances, mappedDiagnostics);
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
    public List<RunOutcome> outcomes() { return outcomes; }
    public String resultText() { return resultText; }
    public List<RunDiagnostic> diagnostics() {
        return mappedDiagnostics.stream().map(MappedDiagnostic::diagnostic).toList();
    }
    public List<RunVerdict> verdicts() { return verdicts; }
    public List<RunInstance> instances() { return instances; }
    public List<MappedDiagnostic> mappedDiagnostics() { return mappedDiagnostics; }

    public static RunResult failure(String hash, RunOperation operation, String target, String message) {
        RunDiagnostic diagnostic = new RunDiagnostic("error", message, "", null, null, null, null, null);
        return new RunResult(hash, operation, target, false, null, null, null, List.of(), List.of(), null,
                List.of(), List.of(), List.of(), List.of(new MappedDiagnostic(diagnostic, null)));
    }
}
