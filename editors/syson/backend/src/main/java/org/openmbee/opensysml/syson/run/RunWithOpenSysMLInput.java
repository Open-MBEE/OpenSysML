package org.openmbee.opensysml.syson.run;

import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.eclipse.sirius.components.core.api.IInput;

public record RunWithOpenSysMLInput(UUID id, String editingContextId, String objectId, RunOperation operation,
        Map<String, String> inputs, List<String> events, List<String> arguments, String schedule, String subject)
        implements IInput {
    public RunWithOpenSysMLInput {
        inputs = inputs == null ? Map.of() : Map.copyOf(inputs);
        events = events == null ? List.of() : List.copyOf(events);
        arguments = arguments == null ? List.of() : List.copyOf(arguments);
    }
}
