package org.openmbee.opensysml.syson.export;

import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

@FunctionalInterface
public interface ElementSerializer {
    String serialize(EObject root, Consumer<Status> report);
}
