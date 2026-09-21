package org.openmbee.opensysml.cameo.selection;

import com.nomagic.magicdraw.core.Project;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import java.util.Optional;
import org.openmbee.opensysml.cameo.cameo.CameoElements;
import org.openmbee.opensysml.cameo.export.V1Exporter;
import org.openmbee.opensysml.cameo.model.ModelPath;
import org.openmbee.opensysml.cameo.model.PathSelector;

/** Decides whether a browser or diagram selection can run, and on which model path. */
public final class SelectionResolver {
  private final boolean textualService;

  public SelectionResolver() {
    this(PathSelector.textualServiceOnClasspath());
  }

  SelectionResolver(boolean textualService) {
    this.textualService = textualService;
  }

  public Optional<Selection> resolve(Project project, Object selected) {
    if (selected instanceof Element element) {
      return CameoElements.modelElement(element).map(subject -> new Selection(
          subject, ModelPath.V1_MDZIP,
          () -> V1Exporter.export(project),
          () -> CameoElements.index(project.getPrimaryModel())));
    }
    return textualService ? V2Selections.resolve(selected) : Optional.empty();
  }
}
