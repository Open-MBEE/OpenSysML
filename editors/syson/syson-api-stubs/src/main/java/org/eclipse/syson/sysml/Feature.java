package org.eclipse.syson.sysml;

import org.eclipse.emf.common.util.EList;

public interface Feature extends Type {
    EList<Redefinition> getOwnedRedefinition();
}
