package org.eclipse.syson.sysml;

import org.eclipse.emf.common.util.EList;

public interface Relationship extends Element {
    EList<Element> getOwnedRelatedElement();

    boolean isIsImplied();
}
