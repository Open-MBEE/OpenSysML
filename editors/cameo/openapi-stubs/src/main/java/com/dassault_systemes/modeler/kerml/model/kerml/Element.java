// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.dassault_systemes.modeler.kerml.model.kerml;

import com.dassault_systemes.modeler.foundation.model.ModelElement;
import java.util.List;

public interface Element extends ModelElement {
  String getElementId();
  String getName();
  String getQualifiedName();
  Element getOwner();
  List<Element> getOwnedElement();
}
