package org.openmbee.opensysml.syson.validation;

import java.util.List;

import org.eclipse.emf.common.util.BasicDiagnostic;
import org.eclipse.emf.common.util.Diagnostic;
import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IValidationService;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunDiagnostic;
import org.openmbee.opensysml.syson.run.RunResult;
import org.springframework.stereotype.Service;

@Service
public class OpenSysMLValidationService implements IValidationService {
    private final RunResultStore store;

    public OpenSysMLValidationService(RunResultStore store) {
        this.store = store;
    }

    @Override
    public List<Object> validate(Object object, Object feature) {
        return List.of();
    }

    @Override
    public List<Object> validate(IEditingContext context) {
        return store.latest(context.getId()).map(this::diagnostics).orElseGet(List::of);
    }

    private List<Object> diagnostics(RunResult result) {
        return result.mappedDiagnostics().stream().map(mapped -> {
            RunDiagnostic diagnostic = mapped.diagnostic();
            int severity = switch (diagnostic.severity()) {
                case "error" -> Diagnostic.ERROR;
                case "warning" -> Diagnostic.WARNING;
                default -> Diagnostic.INFO;
            };
            Object[] data = mapped.element() == null ? new Object[0] : new Object[] { mapped.element() };
            return new BasicDiagnostic(severity, "opensysml", 0, diagnostic.message(), data);
        }).map(Object.class::cast).toList();
    }
}
