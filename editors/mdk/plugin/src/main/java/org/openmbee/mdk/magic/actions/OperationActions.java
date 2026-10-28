package org.openmbee.mdk.magic.actions;

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
import java.io.File;
import java.lang.reflect.InvocationTargetException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Locale;
import java.util.List;
import java.util.Optional;
import java.util.function.Supplier;
import javax.swing.JFileChooser;
import javax.swing.JOptionPane;
import javax.swing.SwingUtilities;
import javax.swing.filechooser.FileNameExtensionFilter;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Value;
import org.openmbee.mdk.magic.annotations.AnnotationPlanner;
import org.openmbee.mdk.magic.annotations.Annotations;
import org.openmbee.mdk.magic.docgen.DocGenExtensions;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.engine.Operation;
import org.openmbee.mdk.magic.engine.RunRequest;
import org.openmbee.mdk.magic.export.ModelExport;
import org.openmbee.mdk.magic.identity.IdentityResolver;
import org.openmbee.mdk.magic.results.RunResult;
import org.openmbee.mdk.magic.selection.Selection;
import org.openmbee.mdk.magic.selection.SelectionResolver;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.mdk.magic.source.V1MdzipSource;
import org.openmbee.mdk.magic.ui.ResultsWindow;

/**
 * The "Magic MDK" context-menu group: one action per operation and the SysML v2 export, offered
 * for a single selected element, plus the DocGen set-up action on the model root.
 */
public final class OperationActions implements BrowserContextAMConfigurator, DiagramContextAMConfigurator {
  private static final String CATEGORY_ID = "org.openmbee.opensysml";
  private static final String SYSML_EXTENSION = ".sysml";

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
    boolean root = selected == project.getPrimaryModel();
    if (selection.isEmpty() && !root) return;
    MDActionsCategory category = new MDActionsCategory(CATEGORY_ID, "Magic MDK");
    category.setNested(true);
    for (Operation operation : Operation.values()) {
      if (selection.isPresent()) category.addAction(new OperationAction(operation, project, selection.get()));
    }
    if (selection.isPresent()) category.addAction(new ExportAction(project, selection.get()));
    if (root) category.addAction(new InstallDocGenExtensionsAction(project));
    manager.addCategory(category);
  }

  /** Creates the «JavaExtension» stereotypes MDK DocGen needs to call the OpenSysML extension. */
  private static final class InstallDocGenExtensionsAction extends MDAction {
    private final Project project;

    InstallDocGenExtensionsAction(Project project) {
      super(CATEGORY_ID + ".docgen-extensions", "Add MDK DocGen extension stereotypes", null, null);
      this.project = project;
    }

    @Override
    public void actionPerformed(ActionEvent event) {
      try {
        JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), DocGenExtensions.install(project).describe(),
            "Magic MDK: DocGen extensions", JOptionPane.INFORMATION_MESSAGE);
      } catch (IllegalStateException failure) {
        JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), failure.getMessage(),
            "Magic MDK: DocGen extensions", JOptionPane.ERROR_MESSAGE);
      }
    }
  }

  /**
   * Writes the project as SysML v2 text: a v1 project is migrated, its diagrams laid out as views
   * from their own symbol streams (or from an MTIP export, when one is chosen); a v2 project's text
   * is written as the tool exports it. The migration report lands beside the file.
   */
  private final class ExportAction extends MDAction {
    static final String TITLE = "Magic MDK: Export as SysML v2";
    private final Project project;
    private final Selection selection;

    ExportAction(Project project, Selection selection) {
      super(CATEGORY_ID + ".export", "Export as SysML v2...", null, null);
      this.project = project;
      this.selection = selection;
    }

    /** Runs on the EDT: asks for the destination, exports the model, then hands the write to the progress runner. */
    @Override
    public void actionPerformed(ActionEvent event) {
      Optional<Path> chosen = chooseDestination();
      if (chosen.isEmpty()) return;
      Path destination = chosen.get();
      ModelSource source;
      try {
        source = selection.export().get();
      } catch (RuntimeException exception) {
        JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), exception.getMessage(), TITLE, JOptionPane.ERROR_MESSAGE);
        return;
      }
      if (source instanceof V1MdzipSource v1) {
        Optional<Optional<Path>> layout = chooseLayout();
        if (layout.isEmpty()) {
          source.close();
          return;
        }
        if (layout.get().isPresent()) source = v1.withLayout(layout.get().get());
      }
      ModelSource exported = source;
      ProgressStatusRunner.runWithProgressStatus(
          status -> export(exported, destination, status), TITLE, true, 0);
    }

    /** The `.sysml` file to write (the extension is added to any other name), confirmed when it exists; empty when the user backs out. */
    private Optional<Path> chooseDestination() {
      JFileChooser chooser = new JFileChooser();
      chooser.setDialogTitle(TITLE);
      chooser.setFileFilter(new FileNameExtensionFilter("SysML v2 text (*.sysml)", "sysml"));
      chooser.setSelectedFile(new File(defaultFileName()));
      while (true) {
        if (chooser.showSaveDialog(Application.getInstance().getMainFrame()) != JFileChooser.APPROVE_OPTION) {
          return Optional.empty();
        }
        Path destination = chooser.getSelectedFile().toPath();
        String name = destination.getFileName().toString();
        if (!name.toLowerCase(Locale.ROOT).endsWith(SYSML_EXTENSION) && !name.toLowerCase(Locale.ROOT).endsWith(".kerml")) {
          destination = destination.resolveSibling(name + SYSML_EXTENSION);
        }
        if (!Files.exists(destination)) return Optional.of(destination);
        int overwrite = JOptionPane.showConfirmDialog(
            Application.getInstance().getMainFrame(), destination + " exists. Replace it?", TITLE,
            JOptionPane.YES_NO_CANCEL_OPTION);
        if (overwrite == JOptionPane.YES_OPTION) return Optional.of(destination);
        if (overwrite != JOptionPane.NO_OPTION) return Optional.empty();
      }
    }

    /**
     * Whether to lay the views out from an MTIP export: empty when the user cancels, an empty
     * inner value for the archive's own symbol streams, else the chosen HUDS XML file.
     */
    private Optional<Optional<Path>> chooseLayout() {
      int choice = JOptionPane.showConfirmDialog(
          Application.getInstance().getMainFrame(),
          "Lay the diagrams out from an MTIP export of this project?\n"
              + "Without one, the views take their geometry from the project's own diagram symbols.",
          TITLE, JOptionPane.YES_NO_CANCEL_OPTION, JOptionPane.QUESTION_MESSAGE);
      if (choice == JOptionPane.NO_OPTION) return Optional.of(Optional.empty());
      if (choice != JOptionPane.YES_OPTION) return Optional.empty();
      JFileChooser chooser = new JFileChooser();
      chooser.setDialogTitle(TITLE + ": MTIP export");
      chooser.setFileFilter(new FileNameExtensionFilter("MTIP export (*.xml)", "xml"));
      if (chooser.showOpenDialog(Application.getInstance().getMainFrame()) != JFileChooser.APPROVE_OPTION) {
        return Optional.empty();
      }
      return Optional.of(Optional.of(chooser.getSelectedFile().toPath()));
    }

    private String defaultFileName() {
      String name = project.getName();
      String stem = name == null || name.isBlank() ? "model" : name.replaceAll("[\\\\/:*?\"<>|]", "_");
      return stem + SYSML_EXTENSION;
    }

    /**
     * Asks, on the EDT, before the export replaces files beside the model other than the
     * destination the save dialog already confirmed; images and reports of another export live there.
     */
    private boolean confirmReplacing(Path destination, List<Path> existing) {
      List<Path> others = existing.stream()
          .filter(path -> !path.equals(destination.toAbsolutePath().normalize()))
          .toList();
      if (others.isEmpty()) return true;
      StringBuilder message = new StringBuilder("The export replaces these existing files:\n");
      others.stream().limit(20).forEach(path -> message.append("  ").append(path).append('\n'));
      if (others.size() > 20) message.append("  … and ").append(others.size() - 20).append(" more\n");
      message.append("Replace them?");
      boolean[] answer = new boolean[1];
      try {
        SwingUtilities.invokeAndWait(() -> answer[0] = JOptionPane.showConfirmDialog(
            Application.getInstance().getMainFrame(), message.toString(), TITLE,
            JOptionPane.YES_NO_OPTION, JOptionPane.WARNING_MESSAGE) == JOptionPane.YES_OPTION);
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        return false;
      } catch (InvocationTargetException failure) {
        throw new IllegalStateException(failure.getCause());
      }
      return answer[0];
    }

    /** Runs off the EDT under the progress dialog; only the dialogs hop back onto it. */
    private void export(ModelSource source, Path destination, ProgressStatus status) {
      status.setDescription("Exporting " + selection.subject().qualifiedName() + " to " + destination.getFileName());
      Optional<ModelExport.Result> result;
      try {
        result = engine.get().export(source, destination, existing -> confirmReplacing(destination, existing), status::isCancel);
      } catch (RuntimeException failure) {
        try {
          source.close();
        } catch (RuntimeException closeFailure) {
          failure.addSuppressed(closeFailure);
        }
        String message = failure.getMessage() == null ? failure.getClass().getName() : failure.getMessage();
        ResultsWindow.onEdt(() -> JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), "Export failed: " + message, TITLE, JOptionPane.ERROR_MESSAGE));
        return;
      }
      try {
        source.close();
      } catch (RuntimeException closeFailure) {
        result = result.map(written -> written.withDiagnostic(new Diagnostic(
            Diagnostic.Severity.WARNING,
            "temporary export not removed: " + closeFailure.getMessage(),
            "export-cleanup",
            Optional.empty())));
      }
      if (result.isEmpty()) return;
      ModelExport.Result written = result.get();
      ResultsWindow.onEdt(() -> JOptionPane.showMessageDialog(
          Application.getInstance().getMainFrame(), written.describe(), TITLE,
          written.errorCount() > 0 ? JOptionPane.WARNING_MESSAGE : JOptionPane.INFORMATION_MESSAGE));
    }
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
            "Magic MDK: Evaluate calc", JOptionPane.QUESTION_MESSAGE);
        if (text == null) return;
        try {
          args = CalcArguments.parse(text);
        } catch (IllegalArgumentException exception) {
          JOptionPane.showMessageDialog(
              Application.getInstance().getMainFrame(), exception.getMessage(),
              "Magic MDK: Evaluate calc", JOptionPane.ERROR_MESSAGE);
          return;
        }
      }
      // Model access stays on the EDT; only the service calls go to the progress runner's thread.
      ModelSource source = null;
      IdentityResolver index;
      try {
        source = selection.export().get();
        index = selection.index().get();
      } catch (RuntimeException exception) {
        if (source != null) {
          try {
            source.close();
          } catch (RuntimeException closeException) {
            exception.addSuppressed(closeException);
          }
        }
        JOptionPane.showMessageDialog(
            Application.getInstance().getMainFrame(), exception.getMessage(),
            "Magic MDK: export failed", JOptionPane.ERROR_MESSAGE);
        return;
      }
      RunRequest request = new RunRequest(operation, source, selection.subject().qualifiedName(), args);
      ProgressStatusRunner.runWithProgressStatus(
          status -> run(request, index, status), "Magic MDK: " + operation.label(), true, 0);
    }

    /** Runs off the EDT under the progress dialog; only the final display hops back onto it. */
    private void run(RunRequest request, IdentityResolver index, ProgressStatus status) {
      status.setDescription(operation.label() + " " + request.subjectQualifiedName());
      RunResult result = null;
      RuntimeException cleanupFailure = null;
      try {
        result = engine.get().run(request, status::isCancel);
      } catch (RuntimeException failure) {
        try {
          request.source().close();
        } catch (RuntimeException closeFailure) {
          failure.addSuppressed(closeFailure);
        }
        throw failure;
      }
      try {
        request.source().close();
      } catch (RuntimeException closeFailure) {
        cleanupFailure = closeFailure;
      }
      if (cleanupFailure != null && result != null) {
        result = result.withLeadingDiagnostics(List.of(new Diagnostic(
            Diagnostic.Severity.WARNING,
            "temporary export not removed: " + cleanupFailure.getMessage(),
            "export-cleanup",
            Optional.empty())));
      }
      if (result.status() == RunResult.Status.CANCELLED) return;
      RunResult completed = result;
      IdentityResolver completedIndex = index;
      ResultsWindow.onEdt(() -> {
        ResultsWindow.forProject(project).show(completed, completedIndex);
        Annotations.apply(project, AnnotationPlanner.plan(completed, completedIndex));
      });
    }
  }
}
