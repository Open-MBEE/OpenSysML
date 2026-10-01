// Compile-only stub of the Cameo OpenAPI surface the plugin uses; never shipped.
package com.nomagic.magicdraw.openapi.uml;

import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;

public class ReadOnlyElementException extends Exception {
  public ReadOnlyElementException(Element element) { super("read-only element"); }
}
