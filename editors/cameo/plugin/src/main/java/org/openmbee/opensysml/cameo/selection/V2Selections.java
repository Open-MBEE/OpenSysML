package org.openmbee.opensysml.cameo.selection;

import com.dassault_systemes.modeler.kerml.model.kerml.Element;
import java.util.Optional;
import org.openmbee.opensysml.cameo.model.ModelPath;
import org.openmbee.opensysml.cameo.model.PathSelector;
import org.openmbee.opensysml.cameo.v2.V2Elements;
import org.openmbee.opensysml.cameo.v2.V2Exporter;

/** Touches KerML classes only here, so hosts without the v2 API never have to load them. */
final class V2Selections {
  private V2Selections() {}

  static Optional<Selection> resolve(Object selected) {
    if (!(selected instanceof Element element)) return Optional.empty();
    return V2Elements.modelElement(element).map(subject -> new Selection(
        subject, PathSelector.select(true, true),
        () -> V2Exporter.export(element),
        () -> V2Elements.index(V2Elements.root(element))));
  }
}
