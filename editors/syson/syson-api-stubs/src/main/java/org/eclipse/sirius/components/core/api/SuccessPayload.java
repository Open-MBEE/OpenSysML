package org.eclipse.sirius.components.core.api;

import java.util.List;
import java.util.Objects;
import java.util.UUID;

import org.eclipse.sirius.components.representations.Message;

public record SuccessPayload(UUID id, List<Message> messages) implements IPayload {
    public SuccessPayload {
        Objects.requireNonNull(id);
        Objects.requireNonNull(messages);
    }

    public SuccessPayload(UUID id) {
        this(id, List.of());
    }
}
