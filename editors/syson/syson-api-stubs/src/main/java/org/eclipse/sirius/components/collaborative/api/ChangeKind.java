package org.eclipse.sirius.components.collaborative.api;

public final class ChangeKind {
    public static final String NOTHING = "NOTHING";
    public static final String REPRESENTATION_CREATION = "REPRESENTATION_CREATION";
    public static final String REPRESENTATION_DELETION = "REPRESENTATION_DELETION";
    public static final String REPRESENTATION_RENAMING = "REPRESENTATION_RENAMING";
    public static final String SEMANTIC_CHANGE = "SEMANTIC_CHANGE";
    public static final String RELOAD_REPRESENTATION = "RELOAD_REPRESENTATION";
    public static final String REPRESENTATION_METADATA_UPDATE = "REPRESENTATION_METADATA_UPDATE";

    private ChangeKind() {
        throw new UnsupportedOperationException("stub");
    }
}
