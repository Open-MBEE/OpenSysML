package org.openmbee.opensysml.mdk.docgen;

import com.nomagic.magicdraw.core.Project;
import com.nomagic.magicdraw.openapi.uml.ModelElementsManager;
import com.nomagic.magicdraw.openapi.uml.ReadOnlyElementException;
import com.nomagic.magicdraw.openapi.uml.SessionManager;
import com.nomagic.magicdraw.uml.Finder;
import com.nomagic.uml2.ext.jmi.helpers.StereotypesHelper;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Element;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Generalization;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.NamedElement;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Package;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Property;
import com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Type;
import com.nomagic.uml2.ext.magicdraw.mdprofiles.Profile;
import com.nomagic.uml2.ext.magicdraw.mdprofiles.Stereotype;
import com.nomagic.uml2.impl.ElementsFactory;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.openmbee.opensysml.mdk.engine.Operation;

/**
 * Adds to a project the stereotypes MDK's DocGen needs before it will call the OpenSysML
 * extension: one per operation, each specialising MDK's abstract «JavaExtension» and named after
 * the extension class it loads. They live in a profile of their own, applied to the primary model
 * so they are applicable; running it again is a no-op for what already exists.
 */
public final class DocGenExtensions {
  public static final String MDK_PROFILE = "SysML Extensions";
  public static final String BASE_STEREOTYPE = "JavaExtension";
  public static final String PROFILE_NAME = "OpenSysML MDK DocGen";
  public static final String ARGUMENTS_TAG = "arguments";
  static final String EXTENSION_PACKAGE = "org.openmbee.opensysml.mdk.docgen";
  static final String STRING_TYPE = "UML Standard Profile::UML2 Metamodel::PrimitiveTypes::String";
  private static final List<String> METACLASSES = List.of("Activity", "CallBehaviorAction");

  /** Extension class simple names by operation; the mdk-bridge module defines a class per entry. */
  static final Map<Operation, String> EXTENSION_CLASSES = Map.of(
      Operation.INSTANTIATE, "Instantiate",
      Operation.EXECUTE_ACTION, "ExecuteAction",
      Operation.EXECUTE_STATE, "ExecuteState",
      Operation.VERIFY, "Verify",
      Operation.EVALUATE_CALC, "EvaluateCalc",
      Operation.RUN_ANALYSIS, "RunAnalysis");

  /** What one install did; {@code notes} carries anything the user should know about. */
  public record Report(List<String> created, List<String> existing, List<String> notes) {
    public String describe() {
      StringBuilder text = new StringBuilder();
      if (!created.isEmpty()) text.append("Created: ").append(String.join(", ", created)).append('\n');
      if (!existing.isEmpty()) text.append("Already present: ").append(String.join(", ", existing)).append('\n');
      for (String note : notes) text.append(note).append('\n');
      return text.toString().strip();
    }
  }

  private DocGenExtensions() {}

  public static String stereotypeName(Operation operation) {
    return EXTENSION_PACKAGE + "." + EXTENSION_CLASSES.get(operation);
  }

  public static boolean mdkProfileUsed(Project project) {
    return StereotypesHelper.getProfile(project, MDK_PROFILE) != null;
  }

  /**
   * Creates the missing stereotypes inside one undoable session.
   *
   * @throws IllegalStateException when MDK's profile is not used by the project or the model is
   *     read-only
   */
  public static Report install(Project project) {
    Profile profile = StereotypesHelper.getProfile(project, MDK_PROFILE);
    if (profile == null) {
      throw new IllegalStateException("The project does not use MDK's \"" + MDK_PROFILE
          + "\" profile. Install MDK and add the profile (File > Use Project) first.");
    }
    Stereotype base = StereotypesHelper.getStereotype(project, BASE_STEREOTYPE, profile);
    if (base == null) {
      throw new IllegalStateException(
          "MDK's \"" + MDK_PROFILE + "\" profile has no «" + BASE_STEREOTYPE + "» stereotype.");
    }
    SessionManager sessions = SessionManager.getInstance();
    sessions.createSession(project, "OpenSysML MDK: add DocGen extension stereotypes");
    try {
      Report report = create(project, base);
      sessions.closeSession(project);
      return report;
    } catch (RuntimeException | ReadOnlyElementException failure) {
      sessions.cancelSession(project);
      throw new IllegalStateException("Could not add the stereotypes: " + failure.getMessage(), failure);
    }
  }

  private static Report create(Project project, Stereotype base) throws ReadOnlyElementException {
    List<String> created = new ArrayList<>();
    List<String> existing = new ArrayList<>();
    List<String> notes = new ArrayList<>();
    ElementsFactory factory = project.getElementsFactory();
    ModelElementsManager elements = ModelElementsManager.getInstance();
    Package root = project.getPrimaryModel();
    Profile container = child(root, PROFILE_NAME).filter(Profile.class::isInstance).map(Profile.class::cast)
        .orElse(null);
    if (container == null) {
      container = factory.createProfileInstance();
      container.setName(PROFILE_NAME);
      elements.addElement(container, root);
      created.add("profile " + PROFILE_NAME);
    }
    List<com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Class> metaclasses = new ArrayList<>();
    for (String name : METACLASSES) {
      com.nomagic.uml2.ext.magicdraw.classes.mdkernel.Class metaclass =
          StereotypesHelper.getMetaClassByName(project, name);
      if (metaclass == null) throw new IllegalStateException("UML metaclass " + name + " not found");
      metaclasses.add(metaclass);
    }
    Type string = Finder.byQualifiedName().find(project, STRING_TYPE);
    if (string == null) {
      notes.add("The UML String type was not found; the \"" + ARGUMENTS_TAG + "\" tag is untyped.");
    }
    for (Operation operation : Operation.values()) {
      String name = stereotypeName(operation);
      if (child(container, name).isPresent()) {
        existing.add("«" + name + "»");
        continue;
      }
      Stereotype stereotype = StereotypesHelper.createStereotype(container, name, metaclasses);
      Generalization generalization = factory.createGeneralizationInstance();
      generalization.setGeneral(base);
      generalization.setSpecific(stereotype);
      elements.addElement(generalization, stereotype);
      if (operation == Operation.EVALUATE_CALC) {
        Property arguments = factory.createPropertyInstance();
        arguments.setName(ARGUMENTS_TAG);
        if (string != null) arguments.setType(string);
        elements.addElement(arguments, stereotype);
      }
      created.add("«" + name + "»");
    }
    if (!StereotypesHelper.getAppliedProfiles(root).contains(container)) {
      StereotypesHelper.applyProfile(root, container);
      created.add("application of " + PROFILE_NAME + " to " + root.getName());
    }
    return new Report(created, existing, notes);
  }

  private static Optional<Element> child(Element owner, String name) {
    for (Element candidate : owner.getOwnedElement()) {
      if (candidate instanceof NamedElement named && name.equals(named.getName())) return Optional.of(candidate);
    }
    return Optional.empty();
  }
}
