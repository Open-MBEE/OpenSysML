package org.eclipse.syson.sysml;

import org.eclipse.emf.ecore.EModelElement;

public interface Element extends EModelElement {
    String getDeclaredName();

    String getName();

    String getQualifiedName();

    String getElementId();

    boolean isIsLibraryElement();

    Element getOwner();
}
