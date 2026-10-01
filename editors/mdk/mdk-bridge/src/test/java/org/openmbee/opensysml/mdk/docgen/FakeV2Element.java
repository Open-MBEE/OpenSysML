package org.openmbee.opensysml.mdk.docgen;

import com.dassault_systemes.modeler.kerml.model.kerml.Element;
import java.util.List;

/** A SysML v2 (KerML API) element: a {@code BaseElement} that is not a UML {@code Element}. */
record FakeV2Element(String name) implements Element {
  @Override
  public String getID() { return "v2-" + name; }

  @Override
  public String getHumanName() { return "Part Def " + name; }

  @Override
  public String getElementId() { return getID(); }

  @Override
  public String getName() { return name; }

  @Override
  public String getQualifiedName() { return "Demo::" + name; }

  @Override
  public Element getOwner() { return null; }

  @Override
  public List<Element> getOwnedElement() { return List.of(); }
}
