package org.eclipse.sirius.components.collaborative.api;

import java.util.List;

import org.eclipse.sirius.components.core.api.IPayload;

import reactor.core.publisher.Flux;

public interface IEditingContextEventProcessor {
    String getEditingContextId();

    List<?> getRepresentationEventProcessors();

    Flux<IPayload> getOutputEvents();
}
