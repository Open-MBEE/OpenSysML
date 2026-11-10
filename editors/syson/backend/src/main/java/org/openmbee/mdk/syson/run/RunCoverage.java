package org.openmbee.mdk.syson.run;

import java.util.List;

import org.openmbee.opensysml.Coverage;

// The states entered and transitions taken across every run of an exploration, not only each outcome's witness.
public record RunCoverage(List<Entry> states, List<Entry> transitions) {
    public record Entry(String path, String declaration, int runs) {
    }

    public RunCoverage {
        states = List.copyOf(states);
        transitions = List.copyOf(transitions);
    }

    public static RunCoverage of(Coverage coverage) {
        return new RunCoverage(entries(coverage.states()), entries(coverage.transitions()));
    }

    private static List<Entry> entries(List<Coverage.Entry> entries) {
        return entries.stream().map(entry -> new Entry(entry.path(), entry.declaration(), entry.runs())).toList();
    }
}
