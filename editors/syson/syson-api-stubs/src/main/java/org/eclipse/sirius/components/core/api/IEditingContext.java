package org.eclipse.sirius.components.core.api;

public interface IEditingContext {
    String EDITING_CONTEXT = "editingContext";

    String getId();

    default void dispose() {
    }
}
