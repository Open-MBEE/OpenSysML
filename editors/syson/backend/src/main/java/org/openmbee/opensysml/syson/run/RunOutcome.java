package org.openmbee.opensysml.syson.run;

import java.util.List;

public record RunOutcome(List<RunNamedValue> outputs, String finalState, List<String> trace, String error,
        int linearizations, List<String> witness) {
    public RunOutcome {
        outputs = List.copyOf(outputs);
        trace = List.copyOf(trace);
        witness = List.copyOf(witness);
    }
}
