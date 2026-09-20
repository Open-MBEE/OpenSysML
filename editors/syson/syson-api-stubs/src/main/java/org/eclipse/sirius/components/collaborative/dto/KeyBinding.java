package org.eclipse.sirius.components.collaborative.dto;

import java.util.Objects;

public record KeyBinding(boolean isCtrl, boolean isMeta, boolean isAlt, String key) {
    public KeyBinding {
        Objects.requireNonNull(key);
    }
}
