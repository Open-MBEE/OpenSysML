package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.NamedElement;
import java.util.Collection;
import java.util.List;

record FakeElement(String name) implements NamedElement {
  @Override
  public String getID() { return "id-" + name; }

  @Override
  public String getHumanName() { return "Part " + name; }

  @Override
  public Element getOwner() { return null; }

  @Override
  public Collection<Element> getOwnedElement() { return List.of(); }

  @Override
  public String getName() { return name; }

  @Override
  public String getQualifiedName() { return "Model::" + name; }
}
