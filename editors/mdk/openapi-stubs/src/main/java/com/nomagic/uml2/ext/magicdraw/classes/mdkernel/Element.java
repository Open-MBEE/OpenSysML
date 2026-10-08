// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.uml2.ext.magicdraw.classes.mdkernel;

import com.nomagic.magicdraw.uml.BaseElement;
import java.util.Collection;

public interface Element extends BaseElement {
  Element getOwner();
  Collection<Element> getOwnedElement();
}
