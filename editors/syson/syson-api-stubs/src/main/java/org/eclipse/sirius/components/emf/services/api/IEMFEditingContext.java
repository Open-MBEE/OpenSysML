package org.eclipse.sirius.components.emf.services.api;

import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.emf.edit.domain.AdapterFactoryEditingDomain;
import org.eclipse.sirius.components.core.api.IEditingContext;

public interface IEMFEditingContext extends IEditingContext {
    String RESOURCE_SCHEME = "sirius";

    AdapterFactoryEditingDomain getDomain();

    @Override
    default void dispose() {
        this.getDomain().getResourceSet().getResources().forEach(Resource::unload);
    }
}
