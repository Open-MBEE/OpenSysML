package org.openmbee.opensysml.cameo.annotations;

import com.nomagic.magicdraw.annotation.Annotation;
import com.nomagic.magicdraw.annotation.AnnotationManager;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.uml.BaseElement;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.WeakHashMap;

/** Posts plans as validation annotations, replacing those from the project's previous run. */
public final class Annotations {
  private static final Map<Project, List<Annotation>> POSTED = new WeakHashMap<>();

  private Annotations() {}

  public static synchronized void apply(Project project, List<AnnotationPlan> plans) {
    AnnotationManager manager = AnnotationManager.getInstance(project);
    List<Annotation> previous = POSTED.getOrDefault(project, List.of());
    List<Annotation> added = new ArrayList<>();
    for (AnnotationPlan plan : plans) {
      if (!(plan.target().handle() instanceof BaseElement element)) continue;
      added.add(new Annotation(
          Annotation.getSeverityLevel(project, plan.severity().name().toLowerCase()),
          plan.kind(), plan.text(), element));
    }
    manager.update(previous, added);
    POSTED.put(project, added);
  }

  public static synchronized void clear(Project project) {
    List<Annotation> previous = POSTED.remove(project);
    if (previous != null && !previous.isEmpty()) {
      AnnotationManager.getInstance(project).update(previous, List.of());
    }
  }
}
