package org.openmbee.opensysml.cameo.model;

public record SimpleElement(String id, String qualifiedName, String humanName, Object handle)
    implements ModelElement {}
