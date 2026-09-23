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
import org.openmbee.opensysml.syson.identity.ElementIndex.IndexedElement;

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
        Map<String, IndexedElement> entries = new LinkedHashMap<>();
        Map<Element, IndexedElement> enclosing = new LinkedHashMap<>();
        Map<String, IndexedElement> byElementId = new LinkedHashMap<>();
        int documentIndex = 0;
        for (Resource resource : context.getDomain().getResourceSet().getResources()) {
            String uri = resource.getURI() == null ? "" : resource.getURI().toString();
            if (uri.startsWith(ElementUtil.KERML_LIBRARY_SCHEME) || uri.startsWith(ElementUtil.SYSML_LIBRARY_SCHEME)) continue;
            String name = documentName(resource, documents, documentIndex);
            exportResource(resource, name, documents, messages, ranges, entries, enclosing, byElementId);
            documentIndex++;
        }
        return new ExportedProject(documents, new ElementIndex(entries, enclosing, byElementId), messages, ranges);
    }

    // Names one resource: its URI's last segment as a .sysml name, suffixed when taken.
    private String documentName(Resource resource, List<SourceDocument> documents, int documentIndex) {
        String base = resource.getURI() == null ? "" : resource.getURI().lastSegment();
        if (base == null || base.isBlank()) base = "document-" + documentIndex;
        if (!base.endsWith(SYSML_EXTENSION)) base += SYSML_EXTENSION;
        String name = base;
        int suffix = 1;
        while (containsDocument(documents, name)) {
            name = base.replace(SYSML_EXTENSION, "-" + suffix++ + SYSML_EXTENSION);
        }
        return name;
    }

    // Serializes a resource's roots into one document, recording its ranges, messages and index entries.
    private void exportResource(Resource resource, String name, List<SourceDocument> documents,
            List<ExportedProject.ExportMessage> messages, List<ExportedProject.DocumentRange> ranges,
            Map<String, IndexedElement> entries, Map<Element, IndexedElement> enclosing,
            Map<String, IndexedElement> byElementId) {
        StringBuilder text = new StringBuilder();
        for (EObject root : resource.getContents()) {
            if (!(root instanceof Element element)) continue;
            if (!text.isEmpty() && text.charAt(text.length() - 1) != '\n') text.append('\n');
            int start = text.isEmpty() ? 1 : lineCount(text);
            List<Status> statuses = new ArrayList<>();
            ElementSerializer.Serialization serialized = serializer.serialize(root, statuses::add);
            String serializedText = serialized == null || serialized.text() == null ? "" : serialized.text();
            text.append(serializedText);
            int end = Math.max(start,
                    lineCount(text) - (text.length() > 0 && text.charAt(text.length() - 1) == '\n' ? 1 : 0));
            ranges.add(new ExportedProject.DocumentRange(name, start, end, element));
            if (serialized != null && !serialized.fragments().isEmpty()) {
                locate(element, start, strippedLines(serializedText), serialized.fragments(), name, ranges);
            }
            statuses.forEach(status -> messages.add(new ExportedProject.ExportMessage(ExportedProject.level(status),
                    status.message())));
            index(element, null, entries, enclosing, byElementId);
        }
        documents.add(SourceDocument.inline(name, text.toString()));
    }

    // Finds each visited child's fragment verbatim inside its parent's text (indentation stripped)
    // and records the line range it occupies; unmatched children pass the window to their own children.
    private void locate(Element parent, int parentStartLine, List<String> parentLines,
            Map<EObject, String> fragments, String name, List<ExportedProject.DocumentRange> ranges) {
        int cursor = 0;
        for (EObject child : parent.eContents()) {
            if (!(child instanceof Element childElement)) continue;
            String fragment = fragments.get(childElement);
            List<String> childLines = fragment == null || fragment.isBlank() ? null : strippedLines(fragment);
            if (childLines == null) {
                locate(childElement, parentStartLine, parentLines, fragments, name, ranges);
                continue;
            }
            int at = indexOfSubsequence(parentLines, childLines, cursor);
            if (at < 0) at = indexOfSubsequence(parentLines, childLines, 0);
            if (at < 0) {
                locate(childElement, parentStartLine, parentLines, fragments, name, ranges);
                continue;
            }
            int childStart = parentStartLine + at;
            ranges.add(new ExportedProject.DocumentRange(name, childStart, childStart + childLines.size() - 1,
                    childElement));
            cursor = at + childLines.size();
            locate(childElement, childStart, childLines, fragments, name, ranges);
        }
    }

    private List<String> strippedLines(String text) {
        return text.lines().map(String::strip).toList();
    }

    private int indexOfSubsequence(List<String> lines, List<String> fragment, int from) {
        for (int i = Math.max(0, from); i + fragment.size() <= lines.size(); i++) {
            boolean matches = true;
            for (int j = 0; j < fragment.size() && matches; j++) {
                matches = lines.get(i + j).equals(fragment.get(j));
            }
            if (matches) return i;
        }
        return -1;
    }

    private void index(Element element, IndexedElement nearest,
            Map<String, IndexedElement> entries, Map<Element, IndexedElement> enclosing,
            Map<String, IndexedElement> byElementId) {
        String qualifiedName = element.getQualifiedName();
        IndexedElement current = nearest;
        if (qualifiedName != null && !qualifiedName.isBlank()) {
            current = new IndexedElement(qualifiedName, element.getElementId(), identityService.getId(element),
                    element);
            entries.put(qualifiedName, current);
        } else if (nearest != null) {
            enclosing.put(element, nearest);
        }
        if (element.getElementId() != null && current != null) byElementId.put(element.getElementId(), current);
        for (EObject child : element.eContents()) {
            if (child instanceof Element childElement) index(childElement, current, entries, enclosing, byElementId);
        }
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
