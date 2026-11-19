package org.openmbee.mdk.magic.actions;

import com.nomagic.magicdraw.actions.MDAction;
import com.nomagic.magicdraw.core.Application;
import com.nomagic.magicdraw.core.Project;
import com.nomagic.task.ProgressStatus;
import com.nomagic.ui.ProgressStatusRunner;
import java.awt.GridBagConstraints;
import java.awt.GridBagLayout;
import java.awt.Insets;
import java.awt.event.ActionEvent;
import java.lang.reflect.InvocationTargetException;
import java.util.Arrays;
import java.util.Optional;
import java.util.function.Supplier;
import javax.swing.JCheckBox;
import javax.swing.JComponent;
import javax.swing.JLabel;
import javax.swing.JOptionPane;
import javax.swing.JPanel;
import javax.swing.JPasswordField;
import javax.swing.JTextField;
import javax.swing.SwingUtilities;
import org.openmbee.mdk.magic.engine.Engine;
import org.openmbee.mdk.magic.repository.ApiConfig;
import org.openmbee.mdk.magic.repository.PublishSettings;
import org.openmbee.mdk.magic.repository.Publisher;
import org.openmbee.mdk.magic.repository.SysmlApiClient;
import org.openmbee.mdk.magic.selection.Selection;
import org.openmbee.mdk.magic.source.ModelSource;
import org.openmbee.mdk.magic.ui.ResultsWindow;

/**
 * Publishes the project's model to a project branch of a SysML v2 API server, such as Flexo MMS:
 * a v1 project is migrated first, a v2 project is published as the tool exports it. One-way: the
 * branch is written, never read back into the model.
 */
final class PublishAction extends MDAction {
  static final String TITLE = "Magic MDK: Publish to SysML v2 API";

  private final Supplier<Engine> engine;
  private final Project project;
  private final Selection selection;

  PublishAction(String categoryId, Supplier<Engine> engine, Project project, Selection selection) {
    super(categoryId + ".publish", "Publish to SysML v2 API...", null, null);
    this.engine = engine;
    this.project = project;
    this.selection = selection;
  }

  /** What the dialog collected: the server and where on it the model goes. */
  private record Request(ApiConfig config, String project, String projectName, String branch, boolean deleteMissing, String message) {}

  /** Runs on the EDT: asks where to publish, exports the model, then hands the publication to the progress runner. */
  @Override
  public void actionPerformed(ActionEvent event) {
    PublishSettings settings = PublishSettings.forModel(modelKey());
    Optional<Request> asked = ask(settings);
    if (asked.isEmpty()) return;
    Request request = asked.get();
    SysmlApiClient client;
    ModelSource source;
    try {
      client = new SysmlApiClient(request.config());
      source = selection.export().get();
    } catch (RuntimeException exception) {
      JOptionPane.showMessageDialog(
          Application.getInstance().getMainFrame(), message(exception), TITLE, JOptionPane.ERROR_MESSAGE);
      return;
    }
    ProgressStatusRunner.runWithProgressStatus(
        status -> publish(source, client, request, settings, status), TITLE, true, 0);
  }

  private String modelKey() {
    String file = project.getFileName();
    return file == null || file.isBlank() ? String.valueOf(project.getName()) : file;
  }

  /** The publish dialog; empty when the user backs out. */
  private Optional<Request> ask(PublishSettings settings) {
    ApiConfig environment = ApiConfig.fromEnvironment(System.getenv());
    JTextField url = new JTextField(settings.url().isEmpty() ? environment.baseUrl() : settings.url(), 36);
    JPasswordField token = new JPasswordField(36);
    JTextField projectId = new JTextField(settings.project(), 36);
    JTextField projectName = new JTextField(String.valueOf(project.getName()), 36);
    JTextField branch = new JTextField(settings.branch(), 36);
    JTextField message = new JTextField("Publish " + project.getName() + " from Cameo", 36);
    JCheckBox deleteMissing = new JCheckBox("Delete elements the branch holds that the model does not");

    JPanel panel = new JPanel(new GridBagLayout());
    int row = 0;
    row = add(panel, row, "Server URL:", url);
    row = add(panel, row, "Bearer token:", token);
    row = add(panel, row, "", new JLabel(environment.hasToken()
        ? "Empty uses the token in " + ApiConfig.ENV_TOKEN + "."
        : "Empty sends no token. It is not stored."));
    row = add(panel, row, "Project id:", projectId);
    row = add(panel, row, "", new JLabel("Empty creates a project with the name below."));
    row = add(panel, row, "New project name:", projectName);
    row = add(panel, row, "Branch id:", branch);
    row = add(panel, row, "", new JLabel("Empty is the project's default branch."));
    row = add(panel, row, "Commit message:", message);
    add(panel, row, "", deleteMissing);

    while (true) {
      int answer = JOptionPane.showConfirmDialog(
          Application.getInstance().getMainFrame(), panel, TITLE, JOptionPane.OK_CANCEL_OPTION, JOptionPane.PLAIN_MESSAGE);
      if (answer != JOptionPane.OK_OPTION) return Optional.empty();
      char[] typed = token.getPassword();
      String bearer = typed.length == 0 ? environment.token() : new String(typed).strip();
      Arrays.fill(typed, '\0');
      String problem = null;
      ApiConfig config = environment.withBaseUrl(url.getText()).withToken(bearer);
      try {
        config.checkedBase();
      } catch (RuntimeException refused) {
        problem = message(refused);
      }
      if (problem == null && projectId.getText().isBlank() && projectName.getText().isBlank()) {
        problem = "Name the project to publish to, or the project to create.";
      }
      if (problem == null) {
        return Optional.of(new Request(
            config, projectId.getText().strip(), projectName.getText().strip(), branch.getText().strip(),
            deleteMissing.isSelected(), message.getText().strip()));
      }
      JOptionPane.showMessageDialog(Application.getInstance().getMainFrame(), problem, TITLE, JOptionPane.ERROR_MESSAGE);
    }
  }

  private static int add(JPanel panel, int row, String label, JComponent field) {
    GridBagConstraints left = new GridBagConstraints();
    left.gridx = 0;
    left.gridy = row;
    left.anchor = GridBagConstraints.LINE_END;
    left.insets = new Insets(2, 4, 2, 4);
    panel.add(new JLabel(label), left);
    GridBagConstraints right = new GridBagConstraints();
    right.gridx = 1;
    right.gridy = row;
    right.anchor = GridBagConstraints.LINE_START;
    right.fill = GridBagConstraints.HORIZONTAL;
    right.weightx = 1;
    right.insets = new Insets(2, 4, 2, 4);
    panel.add(field, right);
    return row + 1;
  }

  /** Runs off the EDT under the progress dialog; only the dialogs hop back onto it. */
  private void publish(
      ModelSource source, SysmlApiClient client, Request request, PublishSettings settings, ProgressStatus status) {
    String url = request.config().baseUrl();
    status.setDescription("Publishing " + project.getName() + " to " + url);
    Publisher publisher = new Publisher(client);
    Optional<Publisher.Result> result;
    try {
      Publisher.LastSeen seen = (id, branch) -> settings.lastSeen(url, id, branch);
      try {
        result = engine.get().publish(source, publisher, target(request, seen), status::isCancel);
      } catch (Publisher.BranchMovedException moved) {
        if (!confirm(moved.getMessage() + ".\n\nPublish onto the branch as it is now?")) throw moved;
        result = engine.get().publish(source, publisher, target(request, Publisher.LastSeen.NEVER), status::isCancel);
      }
    } catch (RuntimeException failure) {
      closeQuietly(source, failure);
      String text = message(failure);
      ResultsWindow.onEdt(() -> JOptionPane.showMessageDialog(
          Application.getInstance().getMainFrame(), "Publish failed: " + text, TITLE, JOptionPane.ERROR_MESSAGE));
      return;
    }
    String cleanup = "";
    try {
      source.close();
    } catch (RuntimeException closeFailure) {
      cleanup = "\nThe temporary export was not removed: " + message(closeFailure);
    }
    if (result.isEmpty()) return;
    Publisher.Result published = result.get();
    settings.remember(url, published.project(), published.branch());
    settings.sawHead(url, published.project(), published.branch(), published.head());
    String text = published.describe() + cleanup;
    ResultsWindow.onEdt(() -> JOptionPane.showMessageDialog(
        Application.getInstance().getMainFrame(), text, TITLE, JOptionPane.INFORMATION_MESSAGE));
  }

  private static Publisher.Target target(Request request, Publisher.LastSeen seen) {
    return new Publisher.Target(
        request.project(), request.projectName(), request.branch(), seen, request.deleteMissing(), request.message());
  }

  private static boolean confirm(String question) {
    boolean[] answer = new boolean[1];
    try {
      SwingUtilities.invokeAndWait(() -> answer[0] = JOptionPane.showConfirmDialog(
          Application.getInstance().getMainFrame(), question, TITLE,
          JOptionPane.YES_NO_OPTION, JOptionPane.WARNING_MESSAGE) == JOptionPane.YES_OPTION);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      return false;
    } catch (InvocationTargetException failure) {
      throw new IllegalStateException(failure.getCause());
    }
    return answer[0];
  }

  private static void closeQuietly(ModelSource source, RuntimeException failure) {
    try {
      source.close();
    } catch (RuntimeException closeFailure) {
      failure.addSuppressed(closeFailure);
    }
  }

  private static String message(RuntimeException failure) {
    return failure.getMessage() == null ? failure.getClass().getName() : failure.getMessage();
  }
}
