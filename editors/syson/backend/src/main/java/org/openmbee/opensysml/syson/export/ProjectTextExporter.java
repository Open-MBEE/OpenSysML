package org.openmbee.opensysml.syson.export;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.function.Consumer;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.LibraryPackage;
import org.eclipse.syson.sysml.Namespace;
import org.eclipse.syson.sysml.OwningMembership;
import org.eclipse.syson.sysml.Relationship;
import org.eclipse.syson.sysml.SysmlPackage;
import org.eclipse.syson.sysml.metamodel.util.ElementUtil;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.identity.ElementIndex.IndexedElement;

public class ProjectTextExporter implements ProjectExporter {
    private static final String SYSML_EXTENSION = ".sysml";

    private final ElementSerializer serializer;
    private final IIdentityService identityService;
    private final BundledLibraries bundledLibraries;

    public ProjectTextExporter(ElementSerializer serializer, IIdentityService identityService) {
        this(serializer, identityService, BundledLibraries.NONE);
    }

    public ProjectTextExporter(ElementSerializer serializer, IIdentityService identityService,
            BundledLibraries bundledLibraries) {
        this.serializer = serializer;
        this.identityService = identityService;
        this.bundledLibraries = bundledLibraries;
    }

    // What one export collects: its documents, messages and ranges, and the element index's tables.
    private record Collected(List<SourceDocument> documents, List<ExportedProject.ExportMessage> messages,
            List<ExportedProject.DocumentRange> ranges, Map<String, IndexedElement> entries,
            Map<Element, IndexedElement> enclosing, Map<String, IndexedElement> byElementId) {
        Collected() {
            this(new ArrayList<>(), new ArrayList<>(), new ArrayList<>(), new LinkedHashMap<>(),
                    new LinkedHashMap<>(), new LinkedHashMap<>());
        }

        ExportedProject project() {
            return new ExportedProject(documents, new ElementIndex(entries, enclosing, byElementId), messages, ranges);
        }
    }

    @Override
    public ExportedProject export(IEMFEditingContext context) {
        Collected collected = new Collected();
        int documentIndex = 0;
        for (Resource resource : context.getDomain().getResourceSet().getResources()) {
            String uri = resource.getURI() == null ? "" : resource.getURI().toString();
            if (uri.startsWith(ElementUtil.KERML_LIBRARY_SCHEME) || uri.startsWith(ElementUtil.SYSML_LIBRARY_SCHEME)) continue;
            String name = documentName(resource, collected.documents(), documentIndex);
            exportResource(resource, name, collected);
            documentIndex++;
        }
        return collected.project();
    }

    // A project's copy of a library the service bundles would shadow the service's own, so an
    // unextended copy is left out; a copy declaring anything, at any depth, that the service's
    // library lacks is exported with a warning.
    private boolean skipsBundledCopy(LibraryPackage library, List<ExportedProject.ExportMessage> messages) {
        Set<String> bundled = bundledLibraries.declarations(library.getName()).orElse(null);
        if (bundled == null) return false;
        List<String> added = new ArrayList<>();
        String prefix = library.getQualifiedName() + "::";
        for (var iterator = library.eAllContents(); iterator.hasNext();) {
            String name = declaredName(iterator.next(), prefix);
            if (name != null && !bundled.contains(name)) added.add(name);
        }
        if (added.isEmpty()) {
            messages.add(new ExportedProject.ExportMessage(MessageLevel.INFO, "library package " + library.getName()
                    + " is provided by the OpenSysML service; the project's copy was not exported"));
            return true;
        }
        messages.add(new ExportedProject.ExportMessage(MessageLevel.WARNING, "library package " + library.getName()
                + " shadows the one the OpenSysML service provides because the project's copy adds "
                + String.join(", ", added) + "; documents and queries typed by the service's library will not be recognized"));
        return false;
    }

    // The name a member declares under the library, spelled as a bundled library's declarations are;
    // null for anything that is not an element of the library.
    private static String declaredName(EObject member, String prefix) {
        String qualifiedName = member instanceof Element element ? element.getQualifiedName() : null;
        if (qualifiedName == null || !qualifiedName.startsWith(prefix)) return null;
        return qualifiedName.substring(prefix.length()).replace("'", "");
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
    // SysON files each uploaded document under an unnamed root Namespace that its serializer never
    // prints, so that namespace's relationships are the roots, each exported on its own.
    private void exportResource(Resource resource, String name, Collected collected) {
        StringBuilder text = new StringBuilder();
        boolean skippedLibrary = false;
        for (EObject root : resource.getContents()) {
            if (root instanceof Element element && !exportRoots(element, text, name, collected)) {
                skippedLibrary = true;
            }
        }
        if (skippedLibrary && text.isEmpty()) return;
        collected.documents().add(SourceDocument.inline(name, text.toString()));
    }

    // Exports one of a resource's roots, or each relationship of SysON's unnamed root namespace as a
    // root of its own; false when any of them was left out.
    private boolean exportRoots(Element element, StringBuilder text, String name, Collected collected) {
        if (!isRootNamespace(element)) return exportRoot(element, List.of(element), text, name, collected);
        boolean exported = true;
        for (Relationship relationship : ((Namespace) element).getOwnedRelationship()) {
            List<Element> declared = relationship instanceof OwningMembership membership
                    ? List.copyOf(membership.getOwnedRelatedElement())
                    : List.of(relationship);
            if (!exportRoot(relationship, declared, text, name, collected)) exported = false;
        }
        return exported;
    }

    private static boolean isRootNamespace(Element element) {
        return element.eClass() == SysmlPackage.eINSTANCE.getNamespace() && element.getOwner() == null
                && element.getName() == null;
    }

    // Appends one root's text and records the elements it declares; false when the root is an
    // unextended copy of a bundled library and was left out.
    private boolean exportRoot(EObject root, List<Element> declared, StringBuilder text, String name,
            Collected collected) {
        if (declared.size() == 1 && declared.get(0) instanceof LibraryPackage library
                && skipsBundledCopy(library, collected.messages())) return false;
        if (!text.isEmpty() && text.charAt(text.length() - 1) != '\n') text.append('\n');
        int start = text.isEmpty() ? 1 : lineCount(text);
        List<Status> statuses = new ArrayList<>();
        ElementSerializer.Serialization serialized = serializer.serialize(root, statuses::add);
        String serializedText = serialized == null || serialized.text() == null ? "" : serialized.text();
        text.append(serializedText);
        int end = Math.max(start,
                lineCount(text) - (text.length() > 0 && text.charAt(text.length() - 1) == '\n' ? 1 : 0));
        for (Element element : declared) {
            collected.ranges().add(new ExportedProject.DocumentRange(name, start, end, element));
            if (serialized != null && !serialized.fragments().isEmpty()) {
                locate(element, start, strippedLines(serializedText), serialized.fragments(), name,
                        collected.ranges());
            }
            index(element, null, collected);
        }
        statuses.forEach(status -> collected.messages().add(new ExportedProject.ExportMessage(
                ExportedProject.level(status), status.message())));
        return true;
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

    private void index(Element element, IndexedElement nearest, Collected collected) {
        String qualifiedName = element.getQualifiedName();
        IndexedElement current = nearest;
        if (qualifiedName != null && !qualifiedName.isBlank()) {
            current = new IndexedElement(qualifiedName, element.getElementId(), identityService.getId(element),
                    element);
            collected.entries().put(qualifiedName, current);
        } else if (nearest != null) {
            collected.enclosing().put(element, nearest);
        }
        if (element.getElementId() != null && current != null) {
            collected.byElementId().put(element.getElementId(), current);
        }
        for (EObject child : element.eContents()) {
            if (child instanceof Element childElement) index(childElement, current, collected);
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
