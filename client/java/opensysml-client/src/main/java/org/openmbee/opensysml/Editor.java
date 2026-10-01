package org.openmbee.opensysml;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.function.UnaryOperator;

/**
 * Collects edits of one model and applies them as one batch: the authoring builder over {@link
 * Edit}. Obtained from {@link Model#edit()}.
 *
 * <p>Every method names elements by the ids the model reports and returns this editor, so
 * operations chain. {@link #add(Edit)} takes any edit, with every option its record exposes.
 * An editor applies once; build another from the edited model to edit further.
 *
 * <pre>{@code
 * EditResult result =
 *     model.edit()
 *         .setValue("Demo::sc::unitMass", "1050.0[SI::kg]")
 *         .addPart("Demo::Vehicle", "engine", m -> m.withType("Engine"))
 *         .apply();
 * }</pre>
 */
public final class Editor {

  private final Model model;
  private final List<Edit> edits = new ArrayList<>();
  private boolean applied;

  Editor(Model model) {
    this.model = Objects.requireNonNull(model, "model");
  }

  /**
   * The model the edits apply to.
   *
   * @return the model
   */
  public Model model() {
    return model;
  }

  /**
   * The edits collected so far, in order.
   *
   * @return a copy of the edits
   */
  public List<Edit> edits() {
    return List.copyOf(edits);
  }

  /**
   * Whether {@link #apply()} has been called.
   *
   * @return {@code true} once applied
   */
  public boolean applied() {
    return applied;
  }

  /**
   * Adds any edit.
   *
   * @param edit the edit
   * @return this editor
   * @throws IllegalStateException if the editor was already applied
   */
  public Editor add(Edit edit) {
    Objects.requireNonNull(edit, "edit");
    if (applied) {
      throw new IllegalStateException(
          "this editor has already been applied: build another editor from the edited model to"
              + " edit further");
    }
    edits.add(edit);
    return this;
  }

  /**
   * Applies the edits, as {@link Model#applyEdits(List)} does.
   *
   * @return the edited notation and what each operation changed
   * @throws IllegalStateException if the editor was already applied
   * @throws EditException if the service refused the batch, an empty one included
   */
  public EditResult apply() {
    return apply(EditOptions.defaults());
  }

  /**
   * Applies the edits with options, as {@link Model#applyEdits(List, EditOptions)} does.
   *
   * @param options the document edited and whether new documents' content is accepted
   * @return the edited notation and what each operation changed
   * @throws IllegalStateException if the editor was already applied
   * @throws EditException if the service refused the batch, an empty one included
   */
  public EditResult apply(EditOptions options) {
    if (applied) {
      throw new IllegalStateException(
          "this editor has already been applied: it describes an edit of the model it was made"
              + " from, so build another editor from the edited model");
    }
    EditResult result = model.applyEdits(List.copyOf(edits), options);
    applied = true;
    return result;
  }

  /**
   * Sets the value of an existing feature.
   *
   * @param target the feature
   * @param value the new value in notation
   * @return this editor
   */
  public Editor setValue(String target, String value) {
    return add(new Edit.SetValue(target, value));
  }

  /**
   * Renames a declaration and every reference to it.
   *
   * @param target the declaration
   * @param newName the new name
   * @return this editor
   */
  public Editor rename(String target, String newName) {
    return add(new Edit.Rename(target, newName));
  }

  /**
   * Removes a declaration.
   *
   * @param target the declaration
   * @return this editor
   */
  public Editor delete(String target) {
    return delete(target, false);
  }

  /**
   * Removes a declaration, and with {@code cascade} the declarations referring to it.
   *
   * @param target the declaration
   * @param cascade whether referring declarations go too
   * @return this editor
   */
  public Editor delete(String target, boolean cascade) {
    return add(new Edit.Delete(target, cascade));
  }

  /**
   * Moves a declaration to another namespace, respelling the references the move breaks.
   *
   * @param target the declaration
   * @param owner the receiving namespace; empty for the document root
   * @return this editor
   */
  public Editor move(String target, String owner) {
    return add(new Edit.Move(target, owner));
  }

  /**
   * Adds one declaration of any kind.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param kind the declaration keyword ({@code "part"}, {@code "attribute def"}, …)
   * @param name the declared name
   * @return this editor
   */
  public Editor addMember(String owner, String kind, String name) {
    return add(Edit.AddMember.of(owner, kind, name));
  }

  /**
   * Adds one declaration of any kind, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param kind the declaration keyword
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addMember(
      String owner, String kind, String name, UnaryOperator<Edit.AddMember> options) {
    Objects.requireNonNull(options, "options");
    return add(options.apply(Edit.AddMember.of(owner, kind, name)));
  }

  /**
   * Adds a {@code package} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPackage(String owner, String name) {
    return addMember(owner, "package", name);
  }

  /**
   * Adds a {@code package} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPackage(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "package", name, options);
  }

  /**
   * Adds a {@code part def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPartDef(String owner, String name) {
    return addMember(owner, "part def", name);
  }

  /**
   * Adds a {@code part def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPartDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "part def", name, options);
  }

  /**
   * Adds a {@code part} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPart(String owner, String name) {
    return addMember(owner, "part", name);
  }

  /**
   * Adds a {@code part} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPart(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "part", name, options);
  }

  /**
   * Adds an {@code attribute def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addAttributeDef(String owner, String name) {
    return addMember(owner, "attribute def", name);
  }

  /**
   * Adds an {@code attribute def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addAttributeDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "attribute def", name, options);
  }

  /**
   * Adds an {@code attribute} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addAttribute(String owner, String name) {
    return addMember(owner, "attribute", name);
  }

  /**
   * Adds an {@code attribute} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addAttribute(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "attribute", name, options);
  }

  /**
   * Adds an {@code item def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addItemDef(String owner, String name) {
    return addMember(owner, "item def", name);
  }

  /**
   * Adds an {@code item def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addItemDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "item def", name, options);
  }

  /**
   * Adds an {@code item} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addItem(String owner, String name) {
    return addMember(owner, "item", name);
  }

  /**
   * Adds an {@code item} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addItem(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "item", name, options);
  }

  /**
   * Adds a {@code port def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPortDef(String owner, String name) {
    return addMember(owner, "port def", name);
  }

  /**
   * Adds a {@code port def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPortDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "port def", name, options);
  }

  /**
   * Adds a {@code port} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPort(String owner, String name) {
    return addMember(owner, "port", name);
  }

  /**
   * Adds a {@code port} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPort(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "port", name, options);
  }

  /**
   * Adds a {@code class} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addClass(String owner, String name) {
    return addMember(owner, "class", name);
  }

  /**
   * Adds a {@code class} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addClass(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "class", name, options);
  }

  /**
   * Adds a {@code struct} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addStruct(String owner, String name) {
    return addMember(owner, "struct", name);
  }

  /**
   * Adds a {@code struct} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addStruct(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "struct", name, options);
  }

  /**
   * Adds a {@code datatype} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addDatatype(String owner, String name) {
    return addMember(owner, "datatype", name);
  }

  /**
   * Adds a {@code datatype} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addDatatype(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "datatype", name, options);
  }

  /**
   * Adds a {@code classifier} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addClassifier(String owner, String name) {
    return addMember(owner, "classifier", name);
  }

  /**
   * Adds a {@code classifier} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addClassifier(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "classifier", name, options);
  }

  /**
   * Adds a {@code feature} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addFeature(String owner, String name) {
    return addMember(owner, "feature", name);
  }

  /**
   * Adds a {@code feature} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addFeature(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "feature", name, options);
  }

  /**
   * Adds an {@code assoc} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addAssoc(String owner, String name) {
    return addMember(owner, "assoc", name);
  }

  /**
   * Adds an {@code assoc} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addAssoc(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "assoc", name, options);
  }

  /**
   * Adds a {@code behavior} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addBehavior(String owner, String name) {
    return addMember(owner, "behavior", name);
  }

  /**
   * Adds a {@code behavior} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addBehavior(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "behavior", name, options);
  }

  /**
   * Adds a {@code function} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addFunction(String owner, String name) {
    return addMember(owner, "function", name);
  }

  /**
   * Adds a {@code function} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addFunction(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "function", name, options);
  }

  /**
   * Adds a {@code predicate} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPredicate(String owner, String name) {
    return addMember(owner, "predicate", name);
  }

  /**
   * Adds a {@code predicate} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPredicate(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "predicate", name, options);
  }

  /**
   * Adds an {@code interaction} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addInteraction(String owner, String name) {
    return addMember(owner, "interaction", name);
  }

  /**
   * Adds an {@code interaction} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addInteraction(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "interaction", name, options);
  }

  /**
   * Adds a {@code metaclass} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addMetaclass(String owner, String name) {
    return addMember(owner, "metaclass", name);
  }

  /**
   * Adds a {@code metaclass} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addMetaclass(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "metaclass", name, options);
  }

  /**
   * Adds a {@code state def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addStateDef(String owner, String name) {
    return addMember(owner, "state def", name);
  }

  /**
   * Adds a {@code state def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addStateDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "state def", name, options);
  }

  /**
   * Adds a {@code state} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addState(String owner, String name) {
    return addMember(owner, "state", name);
  }

  /**
   * Adds a {@code state} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addState(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "state", name, options);
  }

  /**
   * Adds a {@code requirement def} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addRequirementDef(String owner, String name) {
    return addMember(owner, "requirement def", name);
  }

  /**
   * Adds a {@code requirement def} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addRequirementDef(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "requirement def", name, options);
  }

  /**
   * Adds a {@code requirement} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addRequirement(String owner, String name) {
    return addMember(owner, "requirement", name);
  }

  /**
   * Adds a {@code requirement} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addRequirement(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "requirement", name, options);
  }

  /**
   * Adds a {@code perform action} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addPerformAction(String owner, String name) {
    return addMember(owner, "perform action", name);
  }

  /**
   * Adds a {@code perform action} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addPerformAction(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "perform action", name, options);
  }

  /**
   * Adds an {@code exhibit state} declaration.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @return this editor
   */
  public Editor addExhibitState(String owner, String name) {
    return addMember(owner, "exhibit state", name);
  }

  /**
   * Adds an {@code exhibit state} declaration, with options.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param name the declared name
   * @param options what to add to the declaration, such as {@code m -> m.withType("T")}
   * @return this editor
   */
  public Editor addExhibitState(String owner, String name, UnaryOperator<Edit.AddMember> options) {
    return addMember(owner, "exhibit state", name, options);
  }

  /**
   * Adds a directed parameter usage, written without a kind keyword: {@code in x : T;}.
   *
   * @param owner the receiving declaration
   * @param direction {@code "in"}, {@code "out"} or {@code "inout"}
   * @param name the parameter name
   * @param type the parameter type
   * @return this editor
   */
  public Editor addParameter(String owner, String direction, String name, String type) {
    return add(Edit.AddMember.of(owner, "", name).withDirection(direction).withType(type));
  }

  /**
   * Adds an unnamed {@code return} parameter.
   *
   * @param owner the receiving calculation
   * @param type the result type
   * @return this editor
   */
  public Editor addReturn(String owner, String type) {
    return add(Edit.AddMember.of(owner, "return", "").withType(type));
  }

  /**
   * Adds an {@code action def} with input and output parameters.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param outputs the {@code out} parameters, in order
   * @return this editor
   */
  public Editor addActionDef(
      String owner, String name, List<Parameter> inputs, List<Parameter> outputs) {
    return withParameters("action def", owner, name, inputs, outputs);
  }

  /**
   * Adds an {@code action} with input and output parameters.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param outputs the {@code out} parameters, in order
   * @return this editor
   */
  public Editor addAction(
      String owner, String name, List<Parameter> inputs, List<Parameter> outputs) {
    return withParameters("action", owner, name, inputs, outputs);
  }

  /**
   * Adds a {@code calc def} with input parameters and a result type.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param returnType the type of its {@code return} parameter
   * @return this editor
   */
  public Editor addCalcDef(String owner, String name, List<Parameter> inputs, String returnType) {
    return calculation("calc def", owner, name, inputs, returnType, null);
  }

  /**
   * Adds a {@code calc def} whose result parameter is bound to an expression.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param returnType the type of its {@code return} parameter
   * @param returnExpression the expression bound to it
   * @return this editor
   */
  public Editor addCalcDef(
      String owner,
      String name,
      List<Parameter> inputs,
      String returnType,
      String returnExpression) {
    Objects.requireNonNull(returnExpression, "returnExpression");
    return calculation("calc def", owner, name, inputs, returnType, returnExpression);
  }

  /**
   * Adds a {@code calc} with input parameters and a result type.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param returnType the type of its {@code return} parameter
   * @return this editor
   */
  public Editor addCalc(String owner, String name, List<Parameter> inputs, String returnType) {
    return calculation("calc", owner, name, inputs, returnType, null);
  }

  /**
   * Adds a {@code calc} whose result parameter is bound to an expression.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param inputs the {@code in} parameters, in order
   * @param returnType the type of its {@code return} parameter
   * @param returnExpression the expression bound to it
   * @return this editor
   */
  public Editor addCalc(
      String owner,
      String name,
      List<Parameter> inputs,
      String returnType,
      String returnExpression) {
    Objects.requireNonNull(returnExpression, "returnExpression");
    return calculation("calc", owner, name, inputs, returnType, returnExpression);
  }

  /**
   * Adds a {@code perform <action>;} usage naming an existing action.
   *
   * @param owner the receiving declaration
   * @param action the performed action
   * @return this editor
   */
  public Editor addPerform(String owner, String action) {
    return addMember(owner, "perform", action);
  }

  /**
   * Adds an {@code exhibit <state>;} usage naming an existing state.
   *
   * @param owner the receiving declaration
   * @param state the exhibited state
   * @return this editor
   */
  public Editor addExhibit(String owner, String state) {
    return addMember(owner, "exhibit", state);
  }

  /**
   * Adds an {@code entry}, {@code do} or {@code exit} action to a state body.
   *
   * @param owner the state
   * @param kind {@code "entry"}, {@code "do"} or {@code "exit"}
   * @param name the action name
   * @return this editor
   * @throws IllegalArgumentException for any other kind
   */
  public Editor addStateAction(String owner, String kind, String name) {
    Objects.requireNonNull(kind, "kind");
    if (!List.of("entry", "do", "exit").contains(kind)) {
      throw new IllegalArgumentException("kind must be 'entry', 'do' or 'exit', not '" + kind + "'");
    }
    return addMember(owner, kind + " action", name);
  }

  /**
   * Adds a {@code constraint def} whose body is an expression.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param expression the constraint expression, written in {@code { ... }}
   * @return this editor
   */
  public Editor addConstraintDef(String owner, String name, String expression) {
    return add(Edit.AddMember.of(owner, "constraint def", name).withBodyExpression(expression));
  }

  /**
   * Adds a {@code constraint} whose body is an expression.
   *
   * @param owner the receiving namespace
   * @param name the declared name
   * @param expression the constraint expression, written in {@code { ... }}
   * @return this editor
   */
  public Editor addConstraint(String owner, String name, String expression) {
    return add(Edit.AddMember.of(owner, "constraint", name).withBodyExpression(expression));
  }

  /**
   * Adds an asserted constraint usage, {@code assert constraint name { expression }}.
   *
   * @param owner the receiving declaration
   * @param name the constraint name; empty for an anonymous one
   * @param expression the asserted expression
   * @param negated whether it is written {@code assert not constraint}
   * @return this editor
   */
  public Editor addAssertConstraint(
      String owner, String name, String expression, boolean negated) {
    return add(
        Edit.AddMember.of(owner, negated ? "assert not constraint" : "assert constraint", name)
            .withBodyExpression(expression));
  }

  /**
   * Adds an anonymous {@code assert <ref>;} usage.
   *
   * @param owner the receiving declaration
   * @param ref the asserted constraint
   * @param negated whether it is written {@code assert not}
   * @return this editor
   */
  public Editor addAssert(String owner, String ref, boolean negated) {
    return addMember(owner, negated ? "assert not" : "assert", ref);
  }

  /**
   * Adds an unnamed {@code objective} member.
   *
   * @param owner the case receiving it
   * @return this editor
   */
  public Editor addObjective(String owner) {
    return addMember(owner, "objective", "");
  }

  /**
   * Adds a named {@code objective} member.
   *
   * @param owner the case receiving it
   * @param name the objective name
   * @return this editor
   */
  public Editor addObjective(String owner, String name) {
    return addMember(owner, "objective", name);
  }

  /**
   * Adds {@code verify <requirement>;} to a verification case or its objective.
   *
   * @param owner the case or objective
   * @param requirement the verified requirement
   * @return this editor
   */
  public Editor addVerify(String owner, String requirement) {
    return add(Edit.AddVerify.of(owner, requirement));
  }

  /**
   * Adds a {@code satisfy <requirement>;} usage.
   *
   * @param owner the receiving declaration
   * @param requirement the satisfied requirement
   * @return this editor
   */
  public Editor addSatisfy(String owner, String requirement) {
    return add(Edit.AddSatisfy.of(owner, requirement));
  }

  /**
   * Adds a {@code satisfy <requirement> by <by>;} usage.
   *
   * @param owner the receiving declaration
   * @param requirement the satisfied requirement
   * @param by the satisfying feature
   * @return this editor
   */
  public Editor addSatisfy(String owner, String requirement, String by) {
    return add(Edit.AddSatisfy.of(owner, requirement).withSatisfyingFeature(by));
  }

  /**
   * Adds a {@code require} or {@code assume} constraint to a requirement body.
   *
   * @param owner the requirement
   * @param kind {@code "require"} or {@code "assume"}
   * @param expression the constraint expression
   * @return this editor
   */
  public Editor addRequirementConstraint(String owner, String kind, String expression) {
    return add(Edit.AddRequirementConstraint.of(owner, kind, expression));
  }

  /**
   * Adds a {@code require constraint { expression }}.
   *
   * @param owner the requirement
   * @param expression the constraint expression
   * @return this editor
   */
  public Editor addRequireConstraint(String owner, String expression) {
    return addRequirementConstraint(owner, "require", expression);
  }

  /**
   * Adds an {@code assume constraint { expression }}.
   *
   * @param owner the requirement
   * @param expression the constraint expression
   * @return this editor
   */
  public Editor addAssumeConstraint(String owner, String expression) {
    return addRequirementConstraint(owner, "assume", expression);
  }

  /**
   * Adds a {@code transition first <source> then <target>;} to a state body; {@link
   * Edit.AddTransition} adds a name, trigger, guard and effect.
   *
   * @param owner the state
   * @param source the source state
   * @param target the target state
   * @return this editor
   */
  public Editor addTransition(String owner, String source, String target) {
    return add(Edit.AddTransition.of(owner, source, target));
  }

  /**
   * Adds an entry transition, {@code entry; then <target>;}, to a state body.
   *
   * @param owner the state
   * @param target the initial state
   * @return this editor
   */
  public Editor addEntryTransition(String owner, String target) {
    return add(Edit.AddTransition.entry(owner, target));
  }

  /**
   * Adds a {@code metadata <type>;} usage.
   *
   * @param owner the receiving declaration
   * @param metadataType the metadata definition
   * @return this editor
   */
  public Editor addMetadata(String owner, String metadataType) {
    return add(Edit.AddMetadata.of(owner, metadataType));
  }

  /**
   * Adds a {@code metadata <type> { feature = value; ... }} usage.
   *
   * @param owner the receiving declaration
   * @param metadataType the metadata definition
   * @param values each feature's value in notation, in iteration order
   * @return this editor
   */
  public Editor addMetadata(String owner, String metadataType, Map<String, String> values) {
    Objects.requireNonNull(values, "values");
    List<Edit.MetadataValue> bindings = new ArrayList<>(values.size());
    values.forEach((feature, value) -> bindings.add(new Edit.MetadataValue(feature, value)));
    return add(Edit.AddMetadata.of(owner, metadataType).withValues(bindings));
  }

  /**
   * Adds a metadata prefix ({@code #Type}) to an existing declaration.
   *
   * @param target the declaration
   * @param metadataType the metadata definition
   * @return this editor
   */
  public Editor addMetadataPrefix(String target, String metadataType) {
    return add(Edit.AddMetadataPrefix.of(target, metadataType));
  }

  /**
   * Adds {@code doc /* body *}{@code /} as the first body member of a declaration.
   *
   * @param target the declaration
   * @param body plain documentation text
   * @return this editor
   */
  public Editor addDocumentation(String target, String body) {
    return add(Edit.AddDocumentation.of(target, body));
  }

  /**
   * Adds a {@code comment} where a new member of {@code owner} goes.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param body plain comment text
   * @return this editor
   */
  public Editor addComment(String owner, String body) {
    return add(Edit.AddComment.of(owner, body));
  }

  /**
   * Adds a {@code comment about a, b} where a new member of {@code owner} goes.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param body plain comment text
   * @param about the annotated elements, resolved from {@code owner}
   * @return this editor
   */
  public Editor addComment(String owner, String body, List<String> about) {
    return add(Edit.AddComment.of(owner, body).withAbout(about));
  }

  /**
   * Writes the line note {@code // text} above a declaration.
   *
   * @param target the declaration
   * @param text one line of note text
   * @return this editor
   */
  public Editor addNote(String target, String text) {
    return add(new Edit.AddNote(target, text));
  }

  /**
   * Adds an import declaration; {@link Edit.AddImport} adds visibility, recursion and filters.
   *
   * @param owner the receiving namespace; empty for the document root
   * @param target the imported name, {@code A::*} for a namespace import
   * @return this editor
   */
  public Editor addImport(String owner, String target) {
    return add(Edit.AddImport.of(owner, target));
  }

  /**
   * Adds a connection-like usage between two features.
   *
   * @param owner the receiving declaration
   * @param kind {@code "connection"}, {@code "interface"}, {@code "allocation"}, {@code "flow"},
   *     {@code "succession"}, …
   * @param from the source end
   * @param to the target end
   * @return this editor
   */
  public Editor addConnection(String owner, String kind, String from, String to) {
    return add(Edit.AddConnection.of(owner, kind, from, to));
  }

  /**
   * Adds an {@code allocate <from> to <to>;} usage.
   *
   * @param owner the receiving declaration
   * @param from the allocated feature
   * @param to the feature allocated to
   * @return this editor
   */
  public Editor addAllocation(String owner, String from, String to) {
    return addConnection(owner, "allocation", from, to);
  }

  /**
   * Adds a {@code flow from <from> to <to>;} usage.
   *
   * @param owner the receiving declaration
   * @param from the source
   * @param to the target
   * @return this editor
   */
  public Editor addFlow(String owner, String from, String to) {
    return addConnection(owner, "flow", from, to);
  }

  /**
   * Adds a {@code first <from> then <to>;} succession.
   *
   * @param owner the receiving declaration
   * @param from the earlier occurrence
   * @param to the later occurrence
   * @return this editor
   */
  public Editor addSuccession(String owner, String from, String to) {
    return addConnection(owner, "succession", from, to);
  }

  /**
   * Adds {@code first <ref>;} to an action body.
   *
   * @param owner the action body
   * @param ref the node the body starts at
   * @return this editor
   */
  public Editor addFirst(String owner, String ref) {
    return add(Edit.AddSequence.first(owner, ref));
  }

  /**
   * Adds {@code then <ref>;} to an action body.
   *
   * @param owner the action body
   * @param ref the node sequenced to
   * @return this editor
   */
  public Editor addThen(String owner, String ref) {
    return add(Edit.AddSequence.then(owner, ref));
  }

  /**
   * Adds {@code then <kind> <name>;}, declaring the member sequenced to.
   *
   * @param owner the action body
   * @param kind {@code "action"}, {@code "perform action"}, {@code "state"}, {@code "merge"},
   *     {@code "decide"}, {@code "join"} or {@code "fork"}
   * @param name the declared name
   * @return this editor
   */
  public Editor addThenMember(String owner, String kind, String name) {
    return add(Edit.AddSequence.thenMember(owner, kind, name));
  }

  /**
   * Adds {@code then accept <payload>;}.
   *
   * @param owner the action body
   * @param payload the payload parameter or trigger
   * @return this editor
   */
  public Editor addAccept(String owner, String payload) {
    return add(Edit.AddSequence.accept(owner, payload));
  }

  /**
   * Adds {@code then send <payload>;}.
   *
   * @param owner the action body
   * @param payload the payload expression
   * @return this editor
   */
  public Editor addSend(String owner, String payload) {
    return add(Edit.AddSequence.send(owner, payload));
  }

  /**
   * Adds {@code then send <payload> to <to>;}.
   *
   * @param owner the action body
   * @param payload the payload expression
   * @param to the receiver expression
   * @return this editor
   */
  public Editor addSend(String owner, String payload, String to) {
    return add(Edit.AddSequence.send(owner, payload).withTarget(to));
  }

  /**
   * Adds {@code then assign <target> := <value>;}.
   *
   * @param owner the action body
   * @param target the assigned feature
   * @param value the assigned expression
   * @return this editor
   */
  public Editor addAssign(String owner, String target, String value) {
    return add(Edit.AddSequence.assign(owner, target, value));
  }

  /**
   * Adds {@code then if <condition> { <body> }}.
   *
   * @param owner the action body
   * @param condition the condition
   * @param body the then branch
   * @return this editor
   */
  public Editor addIf(String owner, String condition, Body body) {
    return add(Edit.AddSequence.ifNode(owner, condition, body.items()));
  }

  /**
   * Adds {@code then if <condition> { <body> } else { <elseBody> }}; an empty else branch is
   * omitted.
   *
   * @param owner the action body
   * @param condition the condition
   * @param body the then branch
   * @param elseBody the else branch
   * @return this editor
   */
  public Editor addIf(String owner, String condition, Body body, Body elseBody) {
    return add(
        Edit.AddSequence.ifNode(owner, condition, body.items()).withElseBody(elseBody.items()));
  }

  /**
   * Adds {@code then while <condition> { <body> }}.
   *
   * @param owner the action body
   * @param condition the loop condition
   * @param body the loop body
   * @return this editor
   */
  public Editor addWhile(String owner, String condition, Body body) {
    return add(Edit.AddSequence.whileLoop(owner, condition, body.items()));
  }

  /**
   * Adds {@code then loop { <body> } until <until>;}.
   *
   * @param owner the action body
   * @param body the loop body
   * @param until the post-condition
   * @return this editor
   */
  public Editor addLoop(String owner, Body body, String until) {
    return add(Edit.AddSequence.loop(owner, body.items()).withUntil(until));
  }

  /**
   * Adds {@code then loop { <body> }}.
   *
   * @param owner the action body
   * @param body the loop body
   * @return this editor
   */
  public Editor addLoop(String owner, Body body) {
    return add(Edit.AddSequence.loop(owner, body.items()));
  }

  /**
   * Adds {@code then for <variable> in <collection> { <body> }}.
   *
   * @param owner the action body
   * @param variable the loop variable
   * @param collection the collection expression
   * @param body the loop body
   * @return this editor
   */
  public Editor addFor(String owner, String variable, String collection, Body body) {
    return add(Edit.AddSequence.forLoop(owner, variable, collection, body.items()));
  }

  /**
   * Adds {@code then terminate;}.
   *
   * @param owner the action body
   * @return this editor
   */
  public Editor addTerminate(String owner) {
    return add(Edit.AddSequence.terminate(owner));
  }

  /**
   * Adds {@code if <guard> then <ref>;}.
   *
   * @param owner the action body
   * @param guard the guard expression
   * @param ref the target
   * @return this editor
   */
  public Editor addGuardedThen(String owner, String guard, String ref) {
    return add(Edit.AddSequence.guardedThen(owner, guard, ref));
  }

  /**
   * Adds {@code else <ref>;}.
   *
   * @param owner the action body
   * @param ref the target
   * @return this editor
   */
  public Editor addElse(String owner, String ref) {
    return add(Edit.AddSequence.elseThen(owner, ref));
  }

  private Editor withParameters(
      String kind, String owner, String name, List<Parameter> inputs, List<Parameter> outputs) {
    Objects.requireNonNull(inputs, "inputs");
    Objects.requireNonNull(outputs, "outputs");
    addMember(owner, kind, name);
    String qualified = qualified(owner, name);
    inputs.forEach(p -> add(p.edit(qualified, "in")));
    outputs.forEach(p -> add(p.edit(qualified, "out")));
    return this;
  }

  private Editor calculation(
      String kind,
      String owner,
      String name,
      List<Parameter> inputs,
      String returnType,
      String returnExpression) {
    Objects.requireNonNull(inputs, "inputs");
    Objects.requireNonNull(returnType, "returnType");
    addMember(owner, kind, name);
    String qualified = qualified(owner, name);
    inputs.forEach(p -> add(p.edit(qualified, "in")));
    Edit.AddMember result = Edit.AddMember.of(qualified, "return", "").withType(returnType);
    return add(returnExpression == null ? result : result.withValue(returnExpression));
  }

  private static String qualified(String owner, String name) {
    return owner.isEmpty() ? name : owner + "::" + name;
  }

  /**
   * One parameter of an action or calculation an editor declares.
   *
   * @param name the parameter name
   * @param type its type; empty for none
   */
  public record Parameter(String name, String type) {

    /**
     * Validates the parameter.
     *
     * @param name the parameter name, never {@code null}
     * @param type its type, never {@code null}
     */
    public Parameter {
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(type, "type");
    }

    private Edit.AddMember edit(String owner, String direction) {
      Edit.AddMember member = Edit.AddMember.of(owner, "", name).withDirection(direction);
      return type.isEmpty() ? member : member.withType(type);
    }
  }

  /**
   * The statements of a nested {@code if}, loop or {@code for} body. The first ordinary
   * statement is written without {@code then} and later ones with it; {@link #add} takes an
   * {@link Edit.AddSequence} as built, for options the shorthands do not take.
   */
  public static final class Body {

    private final List<Edit.AddSequence> items = new ArrayList<>();

    /** Creates an empty body, written {@code { }}. */
    public Body() {
      // Statements are added by the methods below.
    }

    /**
     * The body items collected so far.
     *
     * @return a copy of the items
     */
    public List<Edit.AddSequence> items() {
      return List.copyOf(items);
    }

    /**
     * Adds an item as built, its owner being ignored.
     *
     * @param item the item
     * @return this body
     */
    public Body add(Edit.AddSequence item) {
      items.add(Objects.requireNonNull(item, "item"));
      return this;
    }

    private Body statement(Edit.AddSequence item) {
      return add(items.isEmpty() ? item.withoutThen() : item);
    }

    /**
     * Adds {@code first <ref>;}.
     *
     * @param ref the node the body starts at
     * @return this body
     */
    public Body addFirst(String ref) {
      return add(Edit.AddSequence.first("", ref));
    }

    /**
     * Adds {@code then <ref>;}.
     *
     * @param ref the node sequenced to
     * @return this body
     */
    public Body addThen(String ref) {
      return add(Edit.AddSequence.then("", ref));
    }

    /**
     * Adds {@code then <kind> <name>;}.
     *
     * @param kind the declared kind, such as {@code "action"}
     * @param name the declared name
     * @return this body
     */
    public Body addThenMember(String kind, String name) {
      return add(Edit.AddSequence.thenMember("", kind, name));
    }

    /**
     * Adds an {@code action <name>;} node.
     *
     * @param name the action name
     * @return this body
     */
    public Body addAction(String name) {
      return add(Edit.AddSequence.node("", "action", name));
    }

    /**
     * Adds {@code accept <payload>;}.
     *
     * @param payload the payload parameter or trigger
     * @return this body
     */
    public Body addAccept(String payload) {
      return statement(Edit.AddSequence.accept("", payload));
    }

    /**
     * Adds {@code send <payload>;}.
     *
     * @param payload the payload expression
     * @return this body
     */
    public Body addSend(String payload) {
      return statement(Edit.AddSequence.send("", payload));
    }

    /**
     * Adds {@code send <payload> to <to>;}.
     *
     * @param payload the payload expression
     * @param to the receiver expression
     * @return this body
     */
    public Body addSend(String payload, String to) {
      return statement(Edit.AddSequence.send("", payload).withTarget(to));
    }

    /**
     * Adds {@code assign <target> := <value>;}.
     *
     * @param target the assigned feature
     * @param value the assigned expression
     * @return this body
     */
    public Body addAssign(String target, String value) {
      return statement(Edit.AddSequence.assign("", target, value));
    }

    /**
     * Adds {@code if <condition> { <body> }}.
     *
     * @param condition the condition
     * @param body the then branch
     * @return this body
     */
    public Body addIf(String condition, Body body) {
      return statement(Edit.AddSequence.ifNode("", condition, body.items()));
    }

    /**
     * Adds {@code if <condition> { <body> } else { <elseBody> }}.
     *
     * @param condition the condition
     * @param body the then branch
     * @param elseBody the else branch; an empty one is omitted
     * @return this body
     */
    public Body addIf(String condition, Body body, Body elseBody) {
      return statement(
          Edit.AddSequence.ifNode("", condition, body.items()).withElseBody(elseBody.items()));
    }

    /**
     * Adds {@code while <condition> { <body> }}.
     *
     * @param condition the loop condition
     * @param body the loop body
     * @return this body
     */
    public Body addWhile(String condition, Body body) {
      return statement(Edit.AddSequence.whileLoop("", condition, body.items()));
    }

    /**
     * Adds {@code loop { <body> }}.
     *
     * @param body the loop body
     * @return this body
     */
    public Body addLoop(Body body) {
      return statement(Edit.AddSequence.loop("", body.items()));
    }

    /**
     * Adds {@code for <variable> in <collection> { <body> }}.
     *
     * @param variable the loop variable
     * @param collection the collection expression
     * @param body the loop body
     * @return this body
     */
    public Body addFor(String variable, String collection, Body body) {
      return statement(Edit.AddSequence.forLoop("", variable, collection, body.items()));
    }

    /**
     * Adds {@code terminate;}.
     *
     * @return this body
     */
    public Body addTerminate() {
      return statement(Edit.AddSequence.terminate(""));
    }

    /**
     * Adds {@code if <guard> then <ref>;}.
     *
     * @param guard the guard expression
     * @param ref the target
     * @return this body
     */
    public Body addGuardedThen(String guard, String ref) {
      return add(Edit.AddSequence.guardedThen("", guard, ref));
    }

    /**
     * Adds {@code else <ref>;}.
     *
     * @param ref the target
     * @return this body
     */
    public Body addElse(String ref) {
      return add(Edit.AddSequence.elseThen("", ref));
    }
  }
}
