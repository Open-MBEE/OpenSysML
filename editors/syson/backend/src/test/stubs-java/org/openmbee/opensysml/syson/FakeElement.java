package org.openmbee.opensysml.syson;

import org.eclipse.emf.ecore.impl.MinimalEObjectImpl;
import org.eclipse.emf.ecore.EAnnotation;
import org.eclipse.emf.common.util.EList;
import org.eclipse.emf.common.util.BasicEList;
import org.eclipse.emf.common.util.AbstractTreeIterator;
import org.eclipse.emf.common.util.TreeIterator;
import org.eclipse.emf.ecore.EObject;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import org.eclipse.syson.sysml.Element;

public class FakeElement extends MinimalEObjectImpl.Container implements Element {
    private String declaredName;
    private String qualifiedName;
    private String elementId;
    private boolean library;
    private Element owner;
    private final List<FakeElement> children = new ArrayList<>();

    public FakeElement(String qualifiedName) {
        this.qualifiedName = qualifiedName;
        this.declaredName = qualifiedName == null ? null : qualifiedName.substring(qualifiedName.lastIndexOf("::") + 2);
        this.elementId = qualifiedName == null ? null : "id-" + qualifiedName;
    }

    public FakeElement library(boolean value) { library = value; return this; }
    public FakeElement owner(Element value) { owner = value; return this; }
    public FakeElement declaredName(String value) { declaredName = value; return this; }
    public FakeElement elementId(String value) { elementId = value; return this; }
    public FakeElement addChild(FakeElement child) {
        child.owner(this);
        children.add(child);
        return this;
    }
    @Override public String getDeclaredName() { return declaredName; }
    @Override public String getName() { return declaredName; }
    @Override public String getQualifiedName() { return qualifiedName; }
    @Override public String getElementId() { return elementId; }
    @Override public boolean isIsLibraryElement() { return library; }
    @Override public Element getOwner() { return owner; }
    @Override public EAnnotation getEAnnotation(String source) { return null; }
    @Override public EList<EAnnotation> getEAnnotations() { return new BasicEList<>(); }
    @Override public EList<EObject> eContents() {
        return new BasicEList<EObject>(children);
    }
    @Override public TreeIterator<EObject> eAllContents() {
        return new AbstractTreeIterator<EObject>(this, false) {
            @Override protected Iterator<? extends EObject> getChildren(Object object) {
                return object instanceof FakeElement fake ? fake.children.iterator() : List.<EObject>of().iterator();
            }
        };
    }
}
