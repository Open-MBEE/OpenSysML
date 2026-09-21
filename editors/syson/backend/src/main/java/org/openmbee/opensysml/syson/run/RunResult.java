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

    private RunResult(Builder builder) {
        this.modelHash = builder.modelHash;
        this.operation = builder.operation;
        this.target = builder.target;
        this.ok = builder.ok;
        this.verdict = builder.verdict;
        this.schedule = builder.schedule;
        this.finalTime = builder.finalTime;
        this.outputs = List.copyOf(builder.outputs);
        this.trace = List.copyOf(builder.trace);
        this.outcomes = List.copyOf(builder.outcomes);
        this.resultText = builder.resultText;
        this.verdicts = List.copyOf(builder.verdicts);
        this.instances = List.copyOf(builder.instances);
        this.mappedDiagnostics = List.copyOf(builder.mappedDiagnostics);
    }

    public static Builder builder() {
        return new Builder();
    }

    public static final class Builder {
        private String modelHash;
        private RunOperation operation;
        private String target;
        private boolean ok;
        private String verdict;
        private String schedule;
        private Double finalTime;
        private List<RunNamedValue> outputs = List.of();
        private List<String> trace = List.of();
        private List<RunOutcome> outcomes = List.of();
        private String resultText;
        private List<RunVerdict> verdicts = List.of();
        private List<RunInstance> instances = List.of();
        private List<MappedDiagnostic> mappedDiagnostics = List.of();

        public Builder modelHash(String modelHash) { this.modelHash = modelHash; return this; }
        public Builder operation(RunOperation operation) { this.operation = operation; return this; }
        public Builder target(String target) { this.target = target; return this; }
        public Builder ok(boolean ok) { this.ok = ok; return this; }
        public Builder verdict(String verdict) { this.verdict = verdict; return this; }
        public Builder schedule(String schedule) { this.schedule = schedule; return this; }
        public Builder finalTime(Double finalTime) { this.finalTime = finalTime; return this; }
        public Builder outputs(List<RunNamedValue> outputs) { this.outputs = outputs; return this; }
        public Builder trace(List<String> trace) { this.trace = trace; return this; }
        public Builder outcomes(List<RunOutcome> outcomes) { this.outcomes = outcomes; return this; }
        public Builder resultText(String resultText) { this.resultText = resultText; return this; }
        public Builder verdicts(List<RunVerdict> verdicts) { this.verdicts = verdicts; return this; }
        public Builder instances(List<RunInstance> instances) { this.instances = instances; return this; }
        public Builder mappedDiagnostics(List<MappedDiagnostic> mappedDiagnostics) {
            this.mappedDiagnostics = mappedDiagnostics;
            return this;
        }
        public RunResult build() { return new RunResult(this); }
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
        return RunResult.builder().modelHash(hash).operation(operation).target(target)
                .mappedDiagnostics(List.of(new MappedDiagnostic(diagnostic, null))).build();
    }
}
