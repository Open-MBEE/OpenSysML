package org.openmbee.opensysml.syson.export;

import java.util.List;
import java.util.Optional;

import org.eclipse.emf.ecore.EObject;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.syson.identity.ElementIndex;

public record ExportedProject(List<SourceDocument> documents, ElementIndex index, List<ExportMessage> messages,
        List<DocumentRange> ranges) {
    public record ExportMessage(MessageLevel level, String message) {}
    public record DocumentRange(String documentName, int startLine, int endLine, Element element) {}

    public Optional<ElementIndex.IndexedElement> elementAt(String documentName, int line) {
        return ranges.stream()
                .filter(range -> range.documentName().equals(documentName) && line >= range.startLine()
                        && line <= range.endLine())
                .findFirst()
                .flatMap(range -> index.byElement(range.element()));
    }

    public static MessageLevel level(Status status) {
        return switch (status.severity()) {
            case ERROR -> MessageLevel.ERROR;
            case WARNING -> MessageLevel.WARNING;
            case DEBUG, INFO -> MessageLevel.INFO;
        };
    }
}
