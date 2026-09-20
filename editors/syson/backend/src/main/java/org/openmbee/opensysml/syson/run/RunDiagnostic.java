package org.openmbee.opensysml.syson.run;

public record RunDiagnostic(String severity, String message, String code, String documentName, Integer line,
        String qualifiedName, String elementId, String siriusId) {
}
