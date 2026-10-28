package org.openmbee.mdk.syson;

import org.eclipse.syson.sysml.LibraryPackage;

public class FakeLibraryPackage extends FakeElement implements LibraryPackage {
    public FakeLibraryPackage(String name) {
        super(name);
        library(true);
    }

    @Override public boolean isIsStandard() { return false; }
}
