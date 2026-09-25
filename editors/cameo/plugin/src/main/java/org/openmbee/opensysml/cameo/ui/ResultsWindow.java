package org.openmbee.opensysml.cameo.ui;

import com.nomagic.magicdraw.core.Application;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.ui.ProjectWindow;
import com.nomagic.magicdraw.ui.ProjectWindowsManager;
import com.nomagic.magicdraw.ui.WindowComponentInfo;
import com.nomagic.magicdraw.ui.WindowsManager;
import com.nomagic.magicdraw.ui.browser.WindowComponentContent;
import com.nomagic.magicdraw.uml.BaseElement;
import java.awt.BorderLayout;
import java.awt.Component;
import java.awt.event.MouseAdapter;
import java.awt.event.MouseEvent;
import java.util.List;
import java.util.Map;
import java.util.WeakHashMap;
import javax.swing.JLabel;
import javax.swing.JList;
import javax.swing.JPanel;
import javax.swing.JScrollPane;
import javax.swing.JSplitPane;
import javax.swing.JTable;
import javax.swing.ListSelectionModel;
import javax.swing.SwingUtilities;
import javax.swing.table.DefaultTableModel;
import org.openmbee.opensysml.cameo.identity.IdentityResolver;
import org.openmbee.opensysml.cameo.model.ModelElement;
import org.openmbee.opensysml.cameo.results.RunResult;

/** Dockable "OpenSysML Results" window; double-clicking an outcome selects its element in the browser. */
public final class ResultsWindow implements WindowComponentContent {
  public static final String ID = "org.openmbee.opensysml.results";
  private static final Map<Project, ResultsWindow> WINDOWS = new WeakHashMap<>();

  private final Project project;
  private final JPanel panel = new JPanel(new BorderLayout());
  private final JLabel header = new JLabel(" ");
  private final ResultsTableModel outcomes = new ResultsTableModel();
  private final JTable outcomesTable = new JTable(outcomes);
  private final DefaultTableModel diagnostics = readOnly("Severity", "Message", "Code", "Location");
  private final JList<String> schedule = new JList<>();
  private boolean registered;
  private IdentityResolver resolver = symbolId -> List.of();

  private ResultsWindow(Project project) {
    this.project = project;
    outcomesTable.setSelectionMode(ListSelectionModel.SINGLE_SELECTION);
    outcomesTable.addMouseListener(new MouseAdapter() {
      @Override
      public void mouseClicked(MouseEvent event) {
        if (event.getClickCount() == 2) selectInBrowser(outcomesTable.rowAtPoint(event.getPoint()));
      }
    });
    panel.add(header, BorderLayout.NORTH);
    JSplitPane tables = new JSplitPane(
        JSplitPane.VERTICAL_SPLIT, new JScrollPane(outcomesTable), new JScrollPane(new JTable(diagnostics)));
    tables.setResizeWeight(0.6);
    JSplitPane all = new JSplitPane(JSplitPane.VERTICAL_SPLIT, tables, new JScrollPane(schedule));
    all.setResizeWeight(0.8);
    panel.add(all, BorderLayout.CENTER);
  }

  public static synchronized ResultsWindow forProject(Project project) {
    return WINDOWS.computeIfAbsent(project, ResultsWindow::new);
  }

  public static synchronized void forget(Project project) {
    WINDOWS.remove(project);
  }

  /** Must run on the EDT; registers the window with the project on first use. */
  public void show(RunResult result, IdentityResolver resolver) {
    this.resolver = resolver;
    header.setText(headerText(result));
    outcomes.set(result.outcomes());
    diagnostics.setRowCount(0);
    result.diagnostics().forEach(item -> diagnostics.addRow(new Object[] {
        item.severity(), item.message(), item.code(),
        item.span().map(span -> span.file() + ":" + span.startLine() + ":" + span.startColumn()).orElse("")}));
    schedule.setListData(result.schedule().toArray(String[]::new));
    ProjectWindowsManager manager = Application.getInstance().getMainFrame().getProjectWindowsManager();
    if (!registered) {
      manager.addWindow(project, new ProjectWindow(new WindowComponentInfo(
          ID, "OpenSysML Results", null, WindowsManager.SIDE_SOUTH, WindowsManager.STATE_DOCKED, false), this));
      registered = true;
    }
    manager.activateWindow(project, ID);
  }

  static String headerText(RunResult result) {
    return result.operation().label() + " of " + result.subject() + " (" + result.path() + "): " + result.status()
        + "   final time " + result.finalTime().orElse("-") + "   elapsed " + result.elapsed().toMillis() + " ms";
  }

  private void selectInBrowser(int row) {
    String elementId = row < 0 ? null : outcomes.outcome(row).elementId();
    if (elementId == null) return;
    for (ModelElement element : resolver.resolve(elementId)) {
      if (element.handle() instanceof BaseElement base) {
        project.getBrowser().getContainmentTree().openNode(base, true, true);
        return;
      }
    }
  }

  @Override
  public Component getWindowComponent() {
    return panel;
  }

  @Override
  public Component getDefaultFocusComponent() {
    return outcomesTable;
  }

  public static void onEdt(Runnable task) {
    if (SwingUtilities.isEventDispatchThread()) task.run();
    else SwingUtilities.invokeLater(task);
  }

  private static DefaultTableModel readOnly(String... columns) {
    return new DefaultTableModel(columns, 0) {
      @Override
      public boolean isCellEditable(int row, int column) {
        return false;
      }
    };
  }
}
