package org.openmbee.mdk.syson;

import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.OwningMembership;
import org.eclipse.syson.sysml.SysmlFactory;
import org.eclipse.syson.sysml.impl.NamespaceImpl;

// SysON's unnamed root namespace: the resource root every uploaded document is filed under.
public class FakeNamespace extends NamespaceImpl {
    public FakeNamespace member(Element element) {
        OwningMembership membership = SysmlFactory.eINSTANCE.createOwningMembership();
        membership.getOwnedRelatedElement().add(element);
        getOwnedRelationship().add(membership);
        return this;
    }
}
