package org.eclipse.syson.sysml.metamodel.services.textual.utils;

import org.eclipse.syson.sysml.Element;

public interface INameDeresolver {
    String getDeresolvedName(Element element, Element context);
}
