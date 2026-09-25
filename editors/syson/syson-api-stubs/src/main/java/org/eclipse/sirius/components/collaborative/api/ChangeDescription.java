package org.eclipse.sirius.components.collaborative.api;

import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

import org.eclipse.sirius.components.events.ICause;

public class ChangeDescription {
    private final String kind;
    private final String sourceId;
    private final ICause cause;
    private final Map<String, Object> parameters;

    public ChangeDescription(String kind, String sourceId, ICause cause) {
        this(kind, sourceId, cause, new HashMap<>());
    }

    public ChangeDescription(String kind, String sourceId, ICause cause, Map<String, Object> parameters) {
        this.kind = Objects.requireNonNull(kind);
        this.sourceId = Objects.requireNonNull(sourceId);
        this.cause = Objects.requireNonNull(cause);
        this.parameters = Objects.requireNonNull(parameters);
    }

    public String getKind() {
        return this.kind;
    }

    public String getSourceId() {
        return this.sourceId;
    }

    public ICause getCause() {
        return this.cause;
    }

    public Map<String, Object> getParameters() {
        return this.parameters;
    }
}
