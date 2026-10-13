package org.openmbee.opensysml.syson;

import org.eclipse.syson.sysml.impl.LibraryPackageImpl;

public class FakeLibraryPackage extends LibraryPackageImpl {
    public FakeLibraryPackage(String name) {
        setDeclaredName(name);
        setElementId("id-" + name);
    }
}
