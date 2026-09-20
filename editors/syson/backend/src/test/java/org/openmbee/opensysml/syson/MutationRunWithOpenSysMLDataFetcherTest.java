package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.function.Supplier;
import java.util.concurrent.CompletableFuture;

import org.eclipse.sirius.components.core.api.ErrorPayload;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.run.MutationRunWithOpenSysMLDataFetcher;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;

import graphql.schema.DataFetchingEnvironment;
import reactor.core.publisher.Mono;
import tools.jackson.databind.ObjectMapper;

class MutationRunWithOpenSysMLDataFetcherTest {
    @Test
    void returnsErrorPayloadForDuplicateInputs() throws Exception {
        UUID id = UUID.randomUUID();
        DataFetchingEnvironment environment = mock(DataFetchingEnvironment.class);
        ObjectMapper objectMapper = mock(ObjectMapper.class);
        IExceptionWrapper exceptionWrapper = mock(IExceptionWrapper.class);
        IEditingContextDispatcher dispatcher = mock(IEditingContextDispatcher.class);
        RunWithOpenSysMLInput converted = new RunWithOpenSysMLInput(id, "ctx", "obj", RunOperation.INSTANTIATE,
                Map.of(), List.of(), List.of(), null, null);
        when(environment.getArgument("input")).thenReturn(Map.of("id", id, "inputs",
                List.of(Map.of("name", "x", "expression", "1"), Map.of("name", "x", "expression", "2"))));
        when(objectMapper.convertValue(any(Map.class), eq(RunWithOpenSysMLInput.class))).thenReturn(converted);
        when(exceptionWrapper.wrapMono(any(), any())).thenAnswer(invocation -> {
            @SuppressWarnings("unchecked")
            Supplier<Mono<IPayload>> supplier = invocation.getArgument(0);
            return supplier.get();
        });

        CompletableFuture<IPayload> future = new MutationRunWithOpenSysMLDataFetcher(objectMapper, exceptionWrapper,
                dispatcher).get(environment);

        IPayload payload = future.get();
        assertThat(payload).isInstanceOf(ErrorPayload.class);
        assertThat(((ErrorPayload) payload).message()).isEqualTo("duplicate input name: x");
        verify(dispatcher, never()).dispatchMutation(any(), any());
    }
}
