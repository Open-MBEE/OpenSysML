package org.eclipse.sirius.components.collaborative.api;

import java.util.List;

public interface IEditingContextEventProcessorRegistry {
    List<IEditingContextEventProcessor> getEditingContextEventProcessors();
}
