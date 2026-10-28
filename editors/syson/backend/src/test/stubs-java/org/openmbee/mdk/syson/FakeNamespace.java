package org.openmbee.mdk.syson;

import org.eclipse.emf.common.util.BasicEList;
import org.eclipse.emf.common.util.EList;
import org.eclipse.emf.ecore.EClass;
import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.Namespace;
import org.eclipse.syson.sysml.Relationship;
import org.eclipse.syson.sysml.SysmlPackage;

// SysON's unnamed root namespace: the resource root every uploaded document is filed under.
public class FakeNamespace extends FakeElement implements Namespace {
    private final EList<Relationship> relationships = new BasicEList<>();

    public FakeNamespace() {
        super(null);
    }

    public FakeNamespace member(Element element) {
        relationships.add(new FakeOwningMembership(this, element));
        return this;
    }

    @Override public EClass eClass() { return SysmlPackage.eINSTANCE.getNamespace(); }
    @Override public EList<Relationship> getOwnedRelationship() { return relationships; }
    @Override public EList<EObject> eContents() { return new BasicEList<EObject>(relationships); }
}
