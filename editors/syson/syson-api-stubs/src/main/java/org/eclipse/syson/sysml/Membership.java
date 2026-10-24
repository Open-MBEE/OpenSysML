package org.eclipse.syson.sysml;

public interface Membership extends Relationship {
    VisibilityKind getVisibility();

    Element getMemberElement();

    String getMemberName();
}
