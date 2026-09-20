package org.eclipse.sirius.components.collaborative.api;

import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IInput;
import org.eclipse.sirius.components.core.api.IPayload;

import reactor.core.publisher.Sinks;

public interface IEditingContextEventHandler {
    boolean canHandle(IEditingContext editingContext, IInput input);

    void handle(Sinks.One<IPayload> payloadSink, Sinks.Many<ChangeDescription> changeDescriptionSink,
            IEditingContext editingContext, IInput input);
}
