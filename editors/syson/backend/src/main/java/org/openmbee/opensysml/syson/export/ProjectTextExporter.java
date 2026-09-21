package org.openmbee.opensysml.syson.export;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.metamodel.util.ElementUtil;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.syson.identity.ElementIndex;

public class ProjectTextExporter implements ProjectExporter {
    private static final String SYSML_EXTENSION = ".sysml";

    private final ElementSerializer serializer;
    private final IIdentityService identityService;

    public ProjectTextExporter(ElementSerializer serializer, IIdentityService identityService) {
        this.serializer = serializer;
        this.identityService = identityService;
    }

    @Override
    public ExportedProject export(IEMFEditingContext context) {
        List<SourceDocument> documents = new ArrayList<>();
        List<ExportedProject.ExportMessage> messages = new ArrayList<>();
        List<ExportedProject.DocumentRange> ranges = new ArrayList<>();
        Map<String, ElementIndex.IndexedElement> entries = new LinkedHashMap<>();
        int documentIndex = 0;
        for (Resource resource : context.getDomain().getResourceSet().getResources()) {
            String uri = resource.getURI() == null ? "" : resource.getURI().toString();
            if (uri.startsWith(ElementUtil.KERML_LIBRARY_SCHEME) || uri.startsWith(ElementUtil.SYSML_LIBRARY_SCHEME)) continue;
            String base = resource.getURI() == null ? "" : resource.getURI().lastSegment();
            if (base == null || base.isBlank()) base = "document-" + documentIndex;
            if (!base.endsWith(SYSML_EXTENSION)) base += SYSML_EXTENSION;
            String name = base;
            int suffix = 1;
            while (containsDocument(documents, name)) {
                name = base.replace(SYSML_EXTENSION, "-" + suffix++ + SYSML_EXTENSION);
            }
            StringBuilder text = new StringBuilder();
            for (EObject root : resource.getContents()) {
                if (!(root instanceof Element element)) continue;
                if (!text.isEmpty() && text.charAt(text.length() - 1) != '\n') text.append('\n');
                int start = text.isEmpty() ? 1 : lineCount(text);
                List<Status> statuses = new ArrayList<>();
                String serialized = serializer.serialize(root, statuses::add);
                if (serialized == null) serialized = "";
                text.append(serialized);
                int end = Math.max(start,
                        lineCount(text) - (text.length() > 0 && text.charAt(text.length() - 1) == '\n' ? 1 : 0));
                ranges.add(new ExportedProject.DocumentRange(name, start, end, element));
                statuses.forEach(status -> messages.add(new ExportedProject.ExportMessage(ExportedProject.level(status),
                        status.message())));
                index(element, entries);
            }
            documents.add(SourceDocument.inline(name, text.toString()));
            documentIndex++;
        }
        return new ExportedProject(documents, new ElementIndex(entries), messages, ranges);
    }

    private void index(Element root, Map<String, ElementIndex.IndexedElement> entries) {
        visit(root, element -> {
            String qualifiedName = element.getQualifiedName();
            if (qualifiedName != null && !qualifiedName.isBlank()) {
                entries.put(qualifiedName, new ElementIndex.IndexedElement(qualifiedName, element.getElementId(),
                        identityService.getId(element), element));
            }
        });
    }

    private void visit(Element element, Consumer<Element> consumer) {
        consumer.accept(element);
        element.eContents().forEach(child -> {
            if (child instanceof Element childElement) visit(childElement, consumer);
        });
    }

    private int lineCount(CharSequence value) {
        int lines = 1;
        for (int index = 0; index < value.length(); index++) if (value.charAt(index) == '\n') lines++;
        return lines;
    }

    private boolean containsDocument(List<SourceDocument> documents, String name) {
        return documents.stream().anyMatch(document -> document.name().filter(name::equals).isPresent());
    }
}
