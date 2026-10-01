package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.List;

import org.eclipse.sirius.components.collaborative.api.IEditingContextEventProcessor;
import org.eclipse.sirius.components.collaborative.api.IEditingContextEventProcessorRegistry;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResult;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.springframework.beans.factory.ObjectProvider;

import reactor.core.publisher.Sinks;

class RunResultStoreTest {
    @Test
    void clearsOnEditingContextDisposalAndResubscribes() {
        Sinks.Many<org.eclipse.sirius.components.core.api.IPayload> sink = Sinks.many().multicast().directBestEffort();
        IEditingContextEventProcessor processor = mock(IEditingContextEventProcessor.class);
        when(processor.getEditingContextId()).thenReturn("ctx");
        when(processor.getOutputEvents()).thenReturn(sink.asFlux());
        IEditingContextEventProcessorRegistry registry = mock(IEditingContextEventProcessorRegistry.class);
        when(registry.getEditingContextEventProcessors()).thenReturn(List.of(processor));
        @SuppressWarnings("unchecked")
        ObjectProvider<IEditingContextEventProcessorRegistry> provider = mock(ObjectProvider.class);
        when(provider.getIfAvailable()).thenReturn(registry);

        RunResultStore store = new RunResultStore(provider);
        RunResult result = RunResult.builder().operation(RunOperation.INSTANTIATE).target("A").ok(true).build();

        store.put("ctx", result);
        assertThat(store.latest("ctx")).contains(result);
        assertThat(sink.currentSubscriberCount()).isEqualTo(1);

        sink.tryEmitComplete();
        assertThat(store.latest("ctx")).isEmpty();
        assertThat(sink.currentSubscriberCount()).isZero();

        Sinks.Many<org.eclipse.sirius.components.core.api.IPayload> secondSink =
                Sinks.many().multicast().directBestEffort();
        when(processor.getOutputEvents()).thenReturn(secondSink.asFlux());
        store.put("ctx", result);
        assertThat(store.latest("ctx")).contains(result);
        assertThat(secondSink.currentSubscriberCount()).isEqualTo(1);
    }
}
