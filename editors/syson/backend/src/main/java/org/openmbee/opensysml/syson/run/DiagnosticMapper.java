package org.openmbee.opensysml.syson.run;

import java.util.Optional;

import org.eclipse.sirius.components.representations.MessageLevel;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.identity.ElementIndex;

public final class DiagnosticMapper {
    public record Mapped(RunDiagnostic diagnostic, org.eclipse.syson.sysml.Element element) {}

    private DiagnosticMapper() {}

    public static Mapped map(Diagnostic diagnostic, ExportedProject project) {
        String document = null;
        Integer line = null;
        ElementIndex.IndexedElement element = null;
        if (diagnostic.span().isPresent()) {
            Diagnostic.Span span = diagnostic.span().orElseThrow();
            document = span.file();
            line = span.startLine();
            element = project.elementAt(document, line).orElse(null);
        }
        Optional<ElementIndex.IndexedElement> named = project.index().firstNamedIn(diagnostic.message());
        if (named.isPresent()) element = named.orElseThrow();
        String severity = switch (diagnostic.severity()) {
            case ERROR -> "error";
            case WARNING -> "warning";
            case INFO, UNKNOWN -> "info";
        };
        RunDiagnostic mapped = new RunDiagnostic(severity, diagnostic.message(), diagnostic.code(), document, line,
                element == null ? null : element.qualifiedName(), element == null ? null : element.elementId(),
                element == null ? null : element.siriusId());
        return new Mapped(mapped, element == null ? null : element.element());
    }

    public static MessageLevel level(RunDiagnostic diagnostic) {
        return switch (diagnostic.severity()) {
            case "error" -> MessageLevel.ERROR;
            case "warning" -> MessageLevel.WARNING;
            default -> MessageLevel.INFO;
        };
    }
}
