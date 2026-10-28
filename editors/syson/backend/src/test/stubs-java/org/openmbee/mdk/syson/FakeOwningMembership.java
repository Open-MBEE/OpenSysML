package org.openmbee.mdk.syson;

import org.eclipse.emf.common.util.BasicEList;
import org.eclipse.emf.common.util.EList;
import org.eclipse.emf.ecore.EObject;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.OwningMembership;
import org.eclipse.syson.sysml.VisibilityKind;

public class FakeOwningMembership extends FakeElement implements OwningMembership {
    private final EList<Element> owned = new BasicEList<>();

    public FakeOwningMembership(Element owner, Element element) {
        super(null);
        owner(owner);
        if (element instanceof FakeElement fake) fake.owner(owner);
        owned.add(element);
    }

    @Override public EList<Element> getOwnedRelatedElement() { return owned; }
    @Override public boolean isIsImplied() { return false; }
    @Override public Element getMemberElement() { return owned.isEmpty() ? null : owned.get(0); }
    @Override public String getMemberName() { return null; }
    @Override public VisibilityKind getVisibility() { return VisibilityKind.PUBLIC; }
    @Override public EList<EObject> eContents() { return new BasicEList<EObject>(owned); }
}
