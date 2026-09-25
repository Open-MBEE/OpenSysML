package org.eclipse.sirius.components.core.api;

import java.util.List;
import java.util.Objects;
import java.util.UUID;

import org.eclipse.sirius.components.representations.Message;
import org.eclipse.sirius.components.representations.MessageLevel;

public record ErrorPayload(UUID id, String message, List<Message> messages) implements IPayload {
    public ErrorPayload {
        Objects.requireNonNull(id);
        Objects.requireNonNull(message);
        Objects.requireNonNull(messages);
    }

    public ErrorPayload(UUID id, String message) {
        this(id, message, List.of(new Message(message, MessageLevel.ERROR)));
    }

    public ErrorPayload(UUID id, List<Message> messages) {
        this(id, messages.stream().findFirst().map(Message::body).orElse(""), messages);
    }
}
