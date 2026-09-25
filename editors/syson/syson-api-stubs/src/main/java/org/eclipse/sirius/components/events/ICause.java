package org.eclipse.sirius.components.events;

import java.util.UUID;

public interface ICause {
    UUID id();

    default ICause causedBy() {
        return null;
    }
}
