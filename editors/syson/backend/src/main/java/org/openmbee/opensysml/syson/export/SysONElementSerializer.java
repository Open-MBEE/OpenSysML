package org.openmbee.opensysml.syson.export;

import java.util.IdentityHashMap;
import java.util.Map;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.emf.ecore.resource.ResourceSet;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.impl.MembershipCacheAdapter;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLElementSerializer;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLSerializingOptions;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.FileNameDeresolver;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.springframework.stereotype.Service;

@Service
public class SysONElementSerializer implements ElementSerializer {
    @Override
    public Serialization serialize(EObject root, Consumer<Status> report) {
        Resource resource = root.eResource();
        ResourceSet set = resource == null ? null : resource.getResourceSet();
        MembershipCacheAdapter adapter = new MembershipCacheAdapter();
        if (set != null) set.eAdapters().add(adapter);
        else if (resource != null) resource.eAdapters().add(adapter);
        try {
            SysMLSerializingOptions options = new SysMLSerializingOptions.Builder().lineSeparator("\n")
                    .nameDeresolver(new FileNameDeresolver()).indentation("\t").needEscapeCharacter(true).build();
            RecordingSerializer serializer = new RecordingSerializer(options, report);
            String text = serializer.doSwitch(root);
            return new Serialization(text == null ? "" : text, serializer.fragments());
        } finally {
            if (set != null) set.eAdapters().remove(adapter);
            else if (resource != null) resource.eAdapters().remove(adapter);
        }
    }

    // Records each visited Element's serialized text so callers can map it to line ranges.
    private static final class RecordingSerializer extends SysMLElementSerializer {
        private final Map<EObject, String> fragments = new IdentityHashMap<>();

        RecordingSerializer(SysMLSerializingOptions options, Consumer<Status> report) {
            super(options, report);
        }

        @Override
        public String doSwitch(EObject eObject) {
            String result = super.doSwitch(eObject);
            if (result != null && eObject instanceof Element) fragments.put(eObject, result);
            return result;
        }

        Map<EObject, String> fragments() {
            return Map.copyOf(fragments);
        }
    }
}
