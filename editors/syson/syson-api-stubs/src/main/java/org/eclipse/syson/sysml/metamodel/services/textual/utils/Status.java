package org.eclipse.syson.sysml.metamodel.services.textual.utils;

import java.text.MessageFormat;

public record Status(Severity severity, String message) {
    public static Status warning(String message, Object... arguments) {
        return new Status(Severity.WARNING, MessageFormat.format(message, arguments));
    }

    public static Status error(String message, Object... arguments) {
        return new Status(Severity.ERROR, MessageFormat.format(message, arguments));
    }

    public static Status info(String message, Object... arguments) {
        return new Status(Severity.INFO, MessageFormat.format(message, arguments));
    }

    public static Status debug(String message, Object... arguments) {
        return new Status(Severity.DEBUG, MessageFormat.format(message, arguments));
    }
}
