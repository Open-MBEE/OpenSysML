package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.function.Supplier;

import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.syson.publish.MutationPublishWithOpenSysMLDataFetcher;
import org.openmbee.mdk.syson.publish.PublishResult;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLInput;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLSuccessPayload;

import graphql.schema.DataFetchingEnvironment;
import reactor.core.publisher.Mono;
import tools.jackson.databind.ObjectMapper;

class MutationPublishWithOpenSysMLDataFetcherTest {
    @Test
    void dispatchesThePublicationToItsEditingContext() throws Exception {
        UUID id = UUID.randomUUID();
        DataFetchingEnvironment environment = mock(DataFetchingEnvironment.class);
        ObjectMapper objectMapper = mock(ObjectMapper.class);
        IExceptionWrapper exceptionWrapper = mock(IExceptionWrapper.class);
        IEditingContextDispatcher dispatcher = mock(IEditingContextDispatcher.class);
        PublishWithOpenSysMLInput input = new PublishWithOpenSysMLInput(id, "ctx", " p1 ", null, null, null, true,
                "c0", null);
        IPayload answer = new PublishWithOpenSysMLSuccessPayload(id, List.of(), new PublishResult(true, false, "", "",
                "p1", "", false, "main", null, "c0", 0, 0, 0, 0, List.of()));
        when(environment.getArgument("input")).thenReturn(Map.of("id", id));
        when(objectMapper.convertValue(any(Map.class), eq(PublishWithOpenSysMLInput.class))).thenReturn(input);
        when(dispatcher.dispatchMutation("ctx", input)).thenReturn(Mono.just(answer));
        when(exceptionWrapper.wrapMono(any(), any())).thenAnswer(invocation -> {
            @SuppressWarnings("unchecked")
            Supplier<Mono<IPayload>> supplier = invocation.getArgument(0);
            return supplier.get();
        });

        IPayload payload = new MutationPublishWithOpenSysMLDataFetcher(objectMapper, exceptionWrapper, dispatcher)
                .get(environment).get();

        assertThat(payload).isSameAs(answer);
        assertThat(input.project()).isEqualTo("p1");
        assertThat(input.projectName()).isEmpty();
        assertThat(input.token()).isEmpty();
    }
}
