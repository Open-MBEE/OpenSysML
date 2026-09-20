package org.openmbee.opensysml.syson.export;

import java.util.ArrayList;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.emf.ecore.resource.ResourceSet;
import org.eclipse.syson.sysml.impl.MembershipCacheAdapter;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLElementSerializer;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLSerializingOptions;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.FileNameDeresolver;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.springframework.stereotype.Service;

@Service
public class SysONElementSerializer implements ElementSerializer {
    @Override
    public String serialize(EObject root, Consumer<Status> report) {
        Resource resource = root.eResource();
        ResourceSet set = resource == null ? null : resource.getResourceSet();
        MembershipCacheAdapter adapter = new MembershipCacheAdapter();
        if (set != null) set.eAdapters().add(adapter);
        else if (resource != null) resource.eAdapters().add(adapter);
        try {
            SysMLSerializingOptions options = new SysMLSerializingOptions.Builder().lineSeparator("\n")
                    .nameDeresolver(new FileNameDeresolver()).indentation("\t").needEscapeCharacter(true).build();
            String text = new SysMLElementSerializer(options, report).doSwitch(root);
            return text == null ? "" : text;
        } finally {
            if (set != null) set.eAdapters().remove(adapter);
            else if (resource != null) resource.eAdapters().remove(adapter);
        }
    }
}
