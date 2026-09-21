package org.eclipse.sirius.components.core.api;

import java.util.List;

public interface IValidationService {
    List<Object> validate(IEditingContext editingContext);

    List<Object> validate(Object object, Object feature);
}
