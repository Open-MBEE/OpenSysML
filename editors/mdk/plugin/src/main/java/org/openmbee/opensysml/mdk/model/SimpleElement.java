package org.openmbee.opensysml.mdk.model;

public record SimpleElement(String id, String qualifiedName, String humanName, Object handle)
    implements ModelElement {}
