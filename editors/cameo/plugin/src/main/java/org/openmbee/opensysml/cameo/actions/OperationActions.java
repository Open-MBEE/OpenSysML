package org.openmbee.opensysml.cameo.actions;

import com.nomagic.actions.ActionsManager;
import com.nomagic.magicdraw.actions.BrowserContextAMConfigurator;
import com.nomagic.magicdraw.actions.DiagramContextAMConfigurator;
import com.nomagic.magicdraw.actions.MDAction;
import com.nomagic.magicdraw.actions.MDActionsCategory;
import com.nomagic.magicdraw.core.Application;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.ui.browser.Node;
import com.nomagic.magicdraw.ui.browser.Tree;
import com.nomagic.magicdraw.uml.symbols.DiagramPresentationElement;
import com.nomagic.magicdraw.uml.symbols.PresentationElement;
import com.nomagic.task.ProgressStatus;
import com.nomagic.ui.ProgressStatusRunner;
import com.nomagic.utils.PriorityProvider;
import java.awt.event.ActionEvent;
import java.util.List;
import java.util.Optional;
import java.util.function.Supplier;
import javax.swing.JOptionPane;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.cameo.annotations.AnnotationPlanner;
import org.openmbee.opensysml.cameo.annotations.Annotations;
import org.openmbee.opensysml.cameo.engine.Engine;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.engine.RunRequest;
import org.openmbee.opensysml.cameo.identity.IdentityResolver;
import org.openmbee.opensysml.cameo.results.RunResult;
import org.openmbee.opensysml.cameo.selection.Selection;
import org.openmbee.opensysml.cameo.selection.SelectionResolver;
import org.openmbee.opensysml.cameo.source.ModelSource;
import org.openmbee.opensysml.cameo.ui.ResultsWindow;

/** The "OpenSysML" context-menu group: one action per operation, offered for a single selected element. */
public final class OperationActions implements BrowserContextAMConfigurator, DiagramContextAMConfigurator {
  private static final String CATEGORY_ID = "org.openmbee.opensysml";

  private final Supplier<Engine> engine;
  private final SelectionResolver resolver;

  public OperationActions(Supplier<Engine> engine) {
    this(engine, new SelectionResolver());
  }

  OperationActions(Supplier<Engine> engine, SelectionResolver resolver) {
    this.engine = engine;
    this.resolver = resolver;
  }

  @Override
  public void configure(ActionsManager manager, Tree browser) {
    Node[] nodes = browser.getSelectedNodes();
    if (nodes.length == 1) offer(manager, nodes[0].getUserObject());
  }

  @Override
  public void configure(
      ActionsManager manager, DiagramPresentationElement diagram, PresentationElement[] selected, PresentationElement requestor) {
    if (selected.length == 1) offer(manager, selected[0].getElement());
  }

  @Override
  public int getPriority() {
    return PriorityProvider.MEDIUM_PRIORITY;
  }

  private void offer(ActionsManager manager, Object selected) {
    Project project = Application.getInstance().getProject();
    if (project == null) return;
    Optional<Selection> selection = resolver.resolve(project, selected);
    if (selection.isEmpty()) return;
    MDActionsCategory category = new MDActionsCategory(CATEGORY_ID, "OpenSysML");
    category.setNested(true);
    for (Operation operation : Operation.values()) {
      category.addAction(new OperationAction(operation, project, selection.get()));
    }
    manager.addCategory(category);
  }

  private final class OperationAction extends MDAction {
    private final Operation operation;
    private final Project project;
    private final Selection selection;

    OperationAction(Operation operation, Project project, Selection selection) {
      super(CATEGORY_ID + "." + operation.name().toLowerCase(), operation.label(), null, null);
      this.operation = operation;
      this.project = project;
      this.selection = selection;
    }

    /** Runs on the EDT: asks for calc arguments, exports the model, then hands the run to the progress runner. */
    @Override
    public void actionPerformed(ActionEvent event) {
      List<Value> args = List.of();
      if (operation == Operation.EVALUATE_CALC) {
        String text = JOptionPane.showInputDialog(
            Application.getInstance().getMainFrame(),
            "Arguments for " + selection.subject().qualifiedName() + " (comma separated)",
            "OpenSysML: Evaluate calc", JOptionPane.QUESTION_MESSAGE);
        if (text == null) return;
        args = CalcArguments.parse(text);
      }
      // Model access stays on the EDT; only the service calls go to the progress runner's thread.
      ModelSource source;
      IdentityResolver index;
      try {
        source = selection.export().get();
      } catch (RuntimeException exception) {
        JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), exception.getMessage(),
            "OpenSysML: export failed", JOptionPane.ERROR_MESSAGE);
        return;
      }
      try {
        index = selection.index().get();
      } catch (RuntimeException exception) {
        source.close();
        throw exception;
      }
      RunRequest request = new RunRequest(operation, source, selection.subject().qualifiedName(), args);
      ProgressStatusRunner.runWithProgressStatus(
          status -> run(request, index, status), "OpenSysML: " + operation.label(), true, 0);
    }

    /** Runs off the EDT under the progress dialog; only the final display hops back onto it. */
    private void run(RunRequest request, IdentityResolver index, ProgressStatus status) {
      status.setDescription(operation.label() + " " + request.subjectQualifiedName());
      RunResult result;
      try (ModelSource source = request.source()) {
        result = engine.get().run(request, status::isCancel);
      }
      if (result.status() == RunResult.Status.CANCELLED) return;
      ResultsWindow.onEdt(() -> {
        ResultsWindow.forProject(project).show(result, index);
        Annotations.apply(project, AnnotationPlanner.plan(result, index));
      });
    }
  }
}
