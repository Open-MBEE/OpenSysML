package org.eclipse.sirius.components.core.api;

import java.util.Optional;

public interface IObjectSearchService {
    Optional<Object> getObject(IEditingContext editingContext, String objectId);
}
