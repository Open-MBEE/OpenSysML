package org.openmbee.opensysml.cameo.v2;

import com.dassault_systemes.modeler.kerml.model.kerml.Element;
import com.dassault_systemes.modeler.kerml.model.kerml.Namespace;
import com.dassault_systemes.modeler.magic.sysml.textual.SysMLTextualNotationService;
import org.openmbee.opensysml.cameo.source.V2TextualSource;

/** Exports the whole v2 model owning the selection as SysML textual notation. */
public final class V2Exporter {
  private V2Exporter() {}

  public static V2TextualSource export(Element element) {
    Element root = V2Elements.root(element);
    if (!(root instanceof Namespace namespace)) {
      throw new IllegalStateException("root of " + element.getQualifiedName() + " is not a namespace");
    }
    return new V2TextualSource(SysMLTextualNotationService.exportTextual(namespace));
  }
}
