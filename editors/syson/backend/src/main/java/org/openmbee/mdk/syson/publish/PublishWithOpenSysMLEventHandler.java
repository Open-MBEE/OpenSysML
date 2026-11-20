package org.openmbee.mdk.syson.publish;

import java.util.List;

import org.eclipse.sirius.components.collaborative.api.ChangeDescription;
import org.eclipse.sirius.components.collaborative.api.ChangeKind;
import org.eclipse.sirius.components.collaborative.api.IEditingContextEventHandler;
import org.eclipse.sirius.components.core.api.ErrorPayload;
import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IInput;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.Message;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.springframework.stereotype.Service;

import reactor.core.publisher.Sinks;

@Service
public class PublishWithOpenSysMLEventHandler implements IEditingContextEventHandler {
    private final PublishWithOpenSysMLService service;

    public PublishWithOpenSysMLEventHandler(PublishWithOpenSysMLService service) {
        this.service = service;
    }

    @Override
    public boolean canHandle(IEditingContext editingContext, IInput input) {
        return input instanceof PublishWithOpenSysMLInput;
    }

    @Override
    public void handle(Sinks.One<IPayload> payloadSink, Sinks.Many<ChangeDescription> changes,
            IEditingContext editingContext, IInput input) {
        if (!(input instanceof PublishWithOpenSysMLInput publishInput)) return;
        try {
            if (!(editingContext instanceof IEMFEditingContext emfContext)) {
                payloadSink.tryEmitValue(new ErrorPayload(publishInput.id(), "The editing context is not EMF-based."));
                return;
            }
            PublishResult result = service.publish(emfContext, publishInput);
            MessageLevel level = result.ok() ? MessageLevel.SUCCESS
                    : result.branchMoved() ? MessageLevel.WARNING : MessageLevel.ERROR;
            payloadSink.tryEmitValue(new PublishWithOpenSysMLSuccessPayload(publishInput.id(),
                    List.of(new Message(result.summary(), level)), result));
        } catch (RuntimeException exception) {
            payloadSink.tryEmitValue(new ErrorPayload(publishInput.id(), exception.getMessage() == null
                    ? exception.getClass().getSimpleName() : exception.getMessage()));
        } finally {
            // Publishing reads the model and changes nothing in the editing context.
            changes.tryEmitNext(new ChangeDescription(ChangeKind.NOTHING, publishInput.editingContextId(),
                    publishInput));
        }
    }
}
