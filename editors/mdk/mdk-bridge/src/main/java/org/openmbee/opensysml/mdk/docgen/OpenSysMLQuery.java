package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.magicdraw.core.Application;
import com.nomagic.magicdraw.uml.BaseElement;
import com.nomagic.uml2.ext.jmi.helpers.StereotypesHelper;
import com.nomagic.uml2.ext.magicdraw.actions.mdbasicactions.CallBehaviorAction;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import java.util.ArrayList;
import java.util.List;
import java.util.logging.Logger;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DocumentElement;
import org.openmbee.mdk.model.Query;

/**
 * A DocGen «JavaExtension» query that runs one OpenSysML operation on each target element, UML or
 * SysML v2 — any {@code BaseElement}; the plugin decides which path can run it. MDK
 * instantiates the subclass named by the applied stereotype (its fully qualified class name),
 * sets the targets, then calls {@link #visit}; each target yields a summary, an outcome table and
 * any diagnostics, and a failure yields an error paragraph rather than aborting the document.
 */
public abstract class OpenSysMLQuery extends Query {
  static final String ARGUMENTS_TAG = "arguments";

  private static final Logger LOG = Logger.getLogger(OpenSysMLQuery.class.getName());

  private final String operation;
  private final String operationLabel;
  private final BridgeRunner runner;

  protected OpenSysMLQuery(String operation, String operationLabel) {
    this(operation, operationLabel, new PluginLocator());
  }

  OpenSysMLQuery(String operation, String operationLabel, BridgeRunner runner) {
    this.operation = operation;
    this.operationLabel = operationLabel;
    this.runner = runner;
  }

  public String operation() {
    return operation;
  }

  @Override
  public List<DocumentElement> visit(boolean forViewEditor, String outputDir) {
    List<DocumentElement> elements = new ArrayList<>();
    if (getIgnore()) return elements;
    if (targets == null || targets.isEmpty()) {
      elements.add(new DBParagraph("OpenSysML " + operationLabel + ": no target elements."));
      return elements;
    }
    String arguments = arguments();
    for (Object target : targets) {
      if (!(target instanceof BaseElement element)) {
        elements.add(new DBParagraph(
            "OpenSysML " + operationLabel + ": skipped " + target + ", not a model element."));
        continue;
      }
      try {
        elements.addAll(DocBookRenderer.render(
            BridgeResult.from(runner.run(element, operation, arguments))));
      } catch (RuntimeException failure) {
        log("[ERROR] OpenSysML " + operationLabel + " of " + element.getHumanName() + ": " + failure.getMessage());
        elements.add(DocBookRenderer.failure(operationLabel, element.getHumanName(), failure));
      }
    }
    return elements;
  }

  /** The {@code arguments} tag of this query's stereotype, read from the action or the behavior it calls. */
  String arguments() {
    if (dgElement == null) return "";
    String value = tag(dgElement);
    if (value == null && dgElement instanceof CallBehaviorAction call && call.getBehavior() != null) {
      value = tag(call.getBehavior());
    }
    return value == null ? "" : value;
  }

  private String tag(Element element) {
    Object value = StereotypesHelper.getStereotypePropertyFirst(element, getClass().getName(), ARGUMENTS_TAG);
    return value == null ? null : String.valueOf(value);
  }

  private static void log(String message) {
    try {
      Application.getInstance().getGUILog().log(message);
    } catch (RuntimeException headless) {
      LOG.warning(message);
    }
  }
}
