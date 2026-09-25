package org.eclipse.syson.sysml.metamodel.services.textual;

import java.util.Objects;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

public class SysMLElementSerializer {
    public SysMLElementSerializer(SysMLSerializingOptions options, Consumer<Status> reportConsumer) {
        Objects.requireNonNull(options);
        Objects.requireNonNull(reportConsumer);
    }

    public String doSwitch(EObject eObject) {
        throw new UnsupportedOperationException("stub");
    }
}
