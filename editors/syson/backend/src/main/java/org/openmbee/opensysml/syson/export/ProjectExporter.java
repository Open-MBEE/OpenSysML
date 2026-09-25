package org.openmbee.opensysml.syson.export;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;

@FunctionalInterface
public interface ProjectExporter {
    ExportedProject export(IEMFEditingContext context);
}
