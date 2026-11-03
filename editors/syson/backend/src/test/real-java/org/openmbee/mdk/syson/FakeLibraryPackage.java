package org.openmbee.mdk.syson;

import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;

import org.eclipse.emf.common.util.AbstractTreeIterator;
import org.eclipse.emf.common.util.BasicEList;
import org.eclipse.emf.common.util.EList;
import org.eclipse.emf.common.util.TreeIterator;
import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.impl.LibraryPackageImpl;

public class FakeLibraryPackage extends LibraryPackageImpl {
    private final List<FakeElement> children = new ArrayList<>();

    public FakeLibraryPackage(String name) {
        setDeclaredName(name);
        setElementId("id-" + name);
    }

    public FakeLibraryPackage addChild(FakeElement child) {
        child.owner(this);
        children.add(child);
        return this;
    }

    @Override
    public EList<EObject> eContents() {
        return new BasicEList<>(children);
    }

    @Override
    public TreeIterator<EObject> eAllContents() {
        return new AbstractTreeIterator<EObject>(this, false) {
            @Override
            protected Iterator<? extends EObject> getChildren(Object object) {
                return object instanceof FakeElement fake ? fake.eContents().iterator()
                        : object instanceof FakeLibraryPackage fake ? fake.children.iterator()
                        : List.<EObject>of().iterator();
            }
        };
    }
}
