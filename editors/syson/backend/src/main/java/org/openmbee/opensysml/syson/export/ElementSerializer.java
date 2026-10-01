package org.openmbee.opensysml.syson.export;

import java.util.Map;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

@FunctionalInterface
public interface ElementSerializer {
    Serialization serialize(EObject root, Consumer<Status> report);

    record Serialization(String text, Map<EObject, String> fragments) {
        public static Serialization of(String text) {
            return new Serialization(text, Map.of());
        }
    }
}
