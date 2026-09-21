package org.openmbee.opensysml.cameo.cameo;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.NamedElement;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Optional;
import java.util.function.Function;
import org.openmbee.opensysml.cameo.identity.ElementTree;
import org.openmbee.opensysml.cameo.identity.IdentityIndex;
import org.openmbee.opensysml.cameo.identity.QualifiedNames;
import org.openmbee.opensysml.cameo.model.ModelElement;
import org.openmbee.opensysml.cameo.model.SimpleElement;

/** Adapts SysML v1 (UML) elements; names are walked below the root model like the migrator does. */
public final class CameoElements {
  private CameoElements() {}

  public static Optional<ModelElement> modelElement(Element element) {
    if (!(element instanceof NamedElement named) || named.getName() == null || named.getName().isBlank()) {
      return Optional.empty();
    }
    return Optional.of(new SimpleElement(named.getID(), qualifiedName(named), named.getHumanName(), named));
  }

  public static String qualifiedName(NamedElement element) {
    return nameWalk(element, value -> ((Element) value).getOwner(), value -> ((NamedElement) value).getName());
  }

  public static IdentityIndex index(Element root) {
    return ElementTree.index(root, Element::getOwnedElement, CameoElements::modelElement);
  }

  public static String nameWalk(Object element, Function<Object, Object> owner, Function<Object, String> name) {
    List<String> names = new ArrayList<>();
    Object current = element;
    while (current != null) {
      Object next = owner.apply(current);
      if (next == null) break;
      String currentName = name.apply(current);
      if (currentName != null && !currentName.isBlank()) names.add(currentName);
      current = next;
    }
    Collections.reverse(names);
    return QualifiedNames.normalize(String.join("::", names));
  }
}
