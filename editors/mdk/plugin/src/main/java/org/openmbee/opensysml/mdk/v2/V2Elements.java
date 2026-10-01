package org.openmbee.opensysml.mdk.v2;

import com.dassault_systemes.modeler.kerml.model.kerml.Element;
import java.util.Optional;
import org.openmbee.opensysml.mdk.identity.ElementTree;
import org.openmbee.opensysml.mdk.identity.IdentityIndex;
import org.openmbee.opensysml.mdk.identity.QualifiedNames;
import org.openmbee.opensysml.mdk.model.ModelElement;
import org.openmbee.opensysml.mdk.model.SimpleElement;

/** Adapts SysML v2 (KerML) elements; the textual export keeps their qualified names verbatim. */
public final class V2Elements {
  private V2Elements() {}

  public static Optional<ModelElement> modelElement(Element element) {
    String qualifiedName = element.getQualifiedName();
    if (qualifiedName == null || qualifiedName.isBlank()) return Optional.empty();
    return Optional.of(new SimpleElement(
        element.getElementId(), QualifiedNames.normalize(qualifiedName), element.getName(), element));
  }

  public static Element root(Element element) {
    Element root = element;
    while (root.getOwner() != null) root = root.getOwner();
    return root;
  }

  public static IdentityIndex index(Element root) {
    return ElementTree.index(root, Element::getOwnedElement, V2Elements::modelElement);
  }
}
