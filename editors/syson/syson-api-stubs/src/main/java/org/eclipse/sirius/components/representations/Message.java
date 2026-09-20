package org.eclipse.sirius.components.representations;

import java.util.Objects;

public record Message(String body, MessageLevel level) {
    public Message {
        Objects.requireNonNull(body);
        Objects.requireNonNull(level);
    }
}
