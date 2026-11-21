package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.UUID;

import org.eclipse.sirius.components.collaborative.api.ChangeDescription;
import org.eclipse.sirius.components.core.api.ErrorPayload;
import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.syson.publish.PublishResult;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLEventHandler;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLInput;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLService;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLSuccessPayload;
import org.openmbee.mdk.syson.run.RunOperation;
import org.openmbee.mdk.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.repository.ApiException;

import reactor.core.publisher.Sinks;

class PublishWithOpenSysMLEventHandlerTest {
    private final PublishWithOpenSysMLService service = mock(PublishWithOpenSysMLService.class);
    private final PublishWithOpenSysMLEventHandler handler = new PublishWithOpenSysMLEventHandler(service);
    private final PublishWithOpenSysMLInput input = new PublishWithOpenSysMLInput(UUID.randomUUID(), "ctx", "p1", "",
            "main", "", false, null, "");

    private IPayload handle(IEditingContext context) {
        Sinks.One<IPayload> payload = Sinks.one();
        Sinks.Many<ChangeDescription> changes = Sinks.many().unicast().onBackpressureBuffer();
        handler.handle(payload, changes, context, input);
        assertThat(changes.asFlux().blockFirst()).isNotNull();
        return payload.asMono().block();
    }

    private static PublishResult result(boolean ok, boolean moved) {
        return new PublishResult(ok, moved, "summary", "http://localhost:8083", "p1", "Mission", false, "main",
                ok ? "c1" : null, ok ? "c1" : "", 0, 0, 0, 0, List.of());
    }

    @Test
    void handlesOnlyPublishInputs() {
        IEditingContext context = mock(IEditingContext.class);
        assertThat(handler.canHandle(context, input)).isTrue();
        assertThat(handler.canHandle(context, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "obj",
                RunOperation.INSTANTIATE, null, null, null, null, null, null))).isFalse();
    }

    @Test
    void reportsAPublicationAsASuccessMessage() {
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(service.publish(context, input)).thenReturn(result(true, false));

        PublishWithOpenSysMLSuccessPayload payload = (PublishWithOpenSysMLSuccessPayload) handle(context);

        assertThat(payload.result().commit()).isEqualTo("c1");
        assertThat(payload.messages().get(0).level()).isEqualTo(MessageLevel.SUCCESS);
    }

    @Test
    void reportsAMovedBranchAsAWarningAndARefusalAsAnError() {
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(service.publish(context, input)).thenReturn(result(false, true));
        assertThat(((PublishWithOpenSysMLSuccessPayload) handle(context)).messages().get(0).level())
                .isEqualTo(MessageLevel.WARNING);

        when(service.publish(context, input)).thenReturn(result(false, false));
        assertThat(((PublishWithOpenSysMLSuccessPayload) handle(context)).messages().get(0).level())
                .isEqualTo(MessageLevel.ERROR);
    }

    @Test
    void reportsAServerFailureAsAnErrorPayload() {
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(service.publish(context, input)).thenThrow(new ApiException("the server answered 401"));

        ErrorPayload payload = (ErrorPayload) handle(context);

        assertThat(payload.messages().get(0).body()).isEqualTo("the server answered 401");
    }

    @Test
    void refusesAnEditingContextThatIsNotEmfBased() {
        ErrorPayload payload = (ErrorPayload) handle(mock(IEditingContext.class));

        assertThat(payload.messages().get(0).body()).isEqualTo("The editing context is not EMF-based.");
    }
}
