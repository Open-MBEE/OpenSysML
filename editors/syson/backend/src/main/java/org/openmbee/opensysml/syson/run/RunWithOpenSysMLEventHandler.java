package org.openmbee.opensysml.syson.run;

import java.util.ArrayList;
import java.util.List;

import org.eclipse.sirius.components.collaborative.api.ChangeDescription;
import org.eclipse.sirius.components.collaborative.api.ChangeKind;
import org.eclipse.sirius.components.collaborative.api.IEditingContextEventHandler;
import org.eclipse.sirius.components.core.api.ErrorPayload;
import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IInput;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.core.api.IObjectSearchService;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.Message;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.eclipse.syson.sysml.Element;
import org.openmbee.opensysml.syson.run.DiagnosticMapper;
import reactor.core.publisher.Sinks;
import org.springframework.stereotype.Service;

@Service
public class RunWithOpenSysMLEventHandler implements IEditingContextEventHandler {
    private final IObjectSearchService objectSearchService;
    private final RunWithOpenSysMLService service;

    public RunWithOpenSysMLEventHandler(IObjectSearchService objectSearchService, RunWithOpenSysMLService service) {
        this.objectSearchService = objectSearchService;
        this.service = service;
    }

    @Override
    public boolean canHandle(IEditingContext editingContext, IInput input) {
        return input instanceof RunWithOpenSysMLInput;
    }

    @Override
    public void handle(Sinks.One<IPayload> payloadSink, Sinks.Many<ChangeDescription> changes,
            IEditingContext editingContext, IInput input) {
        if (!(input instanceof RunWithOpenSysMLInput runInput)) return;
        try {
            Object object = objectSearchService.getObject(editingContext, runInput.objectId()).orElse(null);
            if (!(object instanceof Element element)) {
                payloadSink.tryEmitValue(new ErrorPayload(runInput.id(), "The selected object is not a SysML element."));
                return;
            }
            if (!(editingContext instanceof IEMFEditingContext emfContext)) {
                payloadSink.tryEmitValue(new ErrorPayload(runInput.id(), "The editing context is not EMF-based."));
                return;
            }
            RunResult result = service.run(emfContext, element, runInput);
            List<Message> messages = new ArrayList<>();
            messages.add(new Message(result.ok() ? "OpenSysML run completed." : "OpenSysML run failed.",
                    result.ok() ? MessageLevel.SUCCESS : MessageLevel.ERROR));
            result.diagnostics().forEach(diagnostic -> messages.add(new Message(diagnostic.message(),
                    DiagnosticMapper.level(diagnostic))));
            payloadSink.tryEmitValue(new RunWithOpenSysMLSuccessPayload(runInput.id(), messages, result));
            changes.tryEmitNext(new ChangeDescription(ChangeKind.NOTHING, runInput.editingContextId(), runInput));
        } catch (RuntimeException exception) {
            payloadSink.tryEmitValue(new ErrorPayload(runInput.id(), exception.getMessage() == null
                    ? exception.getClass().getSimpleName() : exception.getMessage()));
        }
    }
}
