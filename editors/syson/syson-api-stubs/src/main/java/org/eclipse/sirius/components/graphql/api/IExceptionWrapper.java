package org.eclipse.sirius.components.graphql.api;

import java.util.List;
import java.util.Optional;
import java.util.function.Supplier;

import org.eclipse.sirius.components.core.api.IInput;
import org.eclipse.sirius.components.core.api.IPayload;

import graphql.relay.Connection;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

public interface IExceptionWrapper {
    Flux<IPayload> wrapFlux(Supplier<Flux<IPayload>> supplier, IInput input);

    IPayload wrap(Supplier<IPayload> supplier, IInput input);

    <T> List<T> wrapList(Supplier<List<T>> supplier);

    <T> Optional<T> wrapOptional(Supplier<Optional<T>> supplier);

    <T> Connection<T> wrapConnection(Supplier<Connection<T>> supplier);

    Mono<IPayload> wrapMono(Supplier<Mono<IPayload>> supplier, IInput input);
}
