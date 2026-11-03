package org.openmbee.mdk.magic.model;

public record SimpleElement(String id, String qualifiedName, String humanName, Object handle)
    implements ModelElement {}
