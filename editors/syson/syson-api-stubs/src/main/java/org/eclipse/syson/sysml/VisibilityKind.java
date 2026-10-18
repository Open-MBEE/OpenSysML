package org.eclipse.syson.sysml;

public enum VisibilityKind {
    PRIVATE, PROTECTED, PUBLIC;

    public String getLiteral() {
        return name().toLowerCase();
    }
}
