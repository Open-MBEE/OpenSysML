package org.eclipse.sirius.components.graphql.api;

import org.eclipse.sirius.components.core.api.IInput;
import org.eclipse.sirius.components.core.api.IPayload;

import reactor.core.publisher.Mono;

public interface IEditingContextDispatcher {
    Mono<IPayload> dispatchQuery(String editingContextId, IInput input);

    Mono<IPayload> dispatchMutation(String editingContextId, IInput input);
}
