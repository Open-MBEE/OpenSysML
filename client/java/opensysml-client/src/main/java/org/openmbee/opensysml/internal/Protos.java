package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.AppliedEdit;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.CaseEvaluation;
import org.openmbee.opensysml.Condition;
import org.openmbee.opensysml.Conversion;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.DocumentQueryResult;
import org.openmbee.opensysml.DocumentRow;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Edit;
import org.openmbee.opensysml.EditFailure;
import org.openmbee.opensysml.EditResult;
import org.openmbee.opensysml.EditedDocument;
import org.openmbee.opensysml.EngineInfo;
import org.openmbee.opensysml.EnumLiteral;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Outcome;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Query;
import org.openmbee.opensysml.QueryElement;
import org.openmbee.opensysml.Referrer;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Sweep;
import org.openmbee.opensysml.SweepRange;
import org.openmbee.opensysml.SweepRow;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.Bound;
import org.openmbee.opensysml.proto.CalcOutput;
import org.openmbee.opensysml.proto.ApplyEditsResponse;
import org.openmbee.opensysml.proto.CompositeConstraint;
import org.openmbee.opensysml.proto.CompositeOperator;
import org.openmbee.opensysml.proto.Constraint;
import org.openmbee.opensysml.proto.ConvertResponse;
import org.openmbee.opensysml.proto.ExecuteActionResponse;
import org.openmbee.opensysml.proto.ExecuteStateResponse;
import org.openmbee.opensysml.proto.ExplorationStatus;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.MultiplicityInfo;
import org.openmbee.opensysml.proto.PrimitiveConstraint;
import org.openmbee.opensysml.proto.PrimitiveOperator;
import org.openmbee.opensysml.proto.QueryResultElement;
import org.openmbee.opensysml.proto.RunAnalysisResponse;
import org.openmbee.opensysml.proto.RunDocumentQueryResponse;
import org.openmbee.opensysml.proto.RunSweepResponse;
import org.openmbee.opensysml.proto.Span;
import org.openmbee.opensysml.proto.Specialization;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.TensorQuantity;
import org.openmbee.opensysml.proto.TypeInfo;
import org.openmbee.opensysml.proto.Undetermined;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import org.openmbee.opensysml.proto.ValidateInstanceResponse;
import org.openmbee.opensysml.proto.ValueSequence;
import org.openmbee.opensysml.proto.ValueSet;
import org.openmbee.opensysml.proto.VerifyConstraintResponse;
import org.openmbee.opensysml.proto.VerifyRequirementResponse;
import org.openmbee.opensysml.proto.VerifySatisfactionResponse;
import java.time.Duration;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalDouble;

/**
 * Reads the generated messages into the client's own immutable types, so no generated class and no
 * builder reaches a caller, and writes the client's values into the messages a request carries.
 */
public final class Protos {

  private Protos() {}

  /**
   * A value, absent when the message names no kind.
   *
   * @param value the generated value
   * @return the value, or empty when no arm is set
   */
  public static Optional<Value> value(org.openmbee.opensysml.proto.Value value) {
    return switch (value.getKindCase()) {
      case INT_VALUE -> Optional.of(new Value.IntegerValue(value.getIntValue()));
      case REAL_VALUE -> Optional.of(new Value.RealValue(value.getRealValue()));
      case COMPLEX ->
          Optional.of(
              new Value.ComplexValue(value.getComplex().getReal(), value.getComplex().getImaginary()));
      case BOOL_VALUE -> Optional.of(new Value.BooleanValue(value.getBoolValue()));
      case STRING_VALUE -> Optional.of(new Value.StringValue(value.getStringValue()));
      case INSTANCE_ID -> Optional.of(new Value.InstanceReference(value.getInstanceId()));
      case SEQUENCE -> Optional.of(sequence(value));
      case NULL -> Optional.of(new Value.NullValue());
      case QUANTITY -> Optional.of(new Value.QuantityValue(quantity(value.getQuantity())));
      case ENUM_LITERAL -> Optional.of(new Value.EnumerationValue(literal(value.getEnumLiteral())));
      case UNSET -> Optional.of(new Value.UnsetValue());
      case UNDETERMINED -> Optional.of(undetermined(value.getUndetermined()));
      case INFINITY -> Optional.of(infinity(value));
      case ARRAY -> Optional.of(array(value.getArray()));
      case VECTOR -> Optional.of(vector(value.getVector()));
      case VECTOR_QUANTITY -> Optional.of(vectorQuantity(value.getVectorQuantity()));
      case MEASUREMENT_REF -> Optional.of(measurementRef(value.getMeasurementRef()));
      case FUNCTION -> Optional.of(function(value.getFunction()));
      case SET -> Optional.of(set(value.getSet()));
      case TENSOR_QUANTITY -> Optional.of(tensorQuantity(value.getTensorQuantity()));
      case METAOBJECT -> Optional.of(metaobject(value.getMetaobject()));
      case KIND_NOT_SET -> Optional.empty();
    };
  }

  /** Only an asserted arm carries the unbounded value. */
  private static Value infinity(org.openmbee.opensysml.proto.Value value) {
    if (!value.getInfinity()) {
      throw new TransportException(
          "the service answered a malformed value: the infinity arm states no value unless it is"
              + " true",
          null);
    }
    return new Value.InfinityValue();
  }

  private static Value undetermined(org.openmbee.opensysml.proto.Undetermined undetermined) {
    org.openmbee.opensysml.proto.MultiplicityInfo count = undetermined.getCount();
    return new Value.UndeterminedValue(undetermined.getReason(), count.getLower(), count.getUpper());
  }

  private static Value array(org.openmbee.opensysml.proto.Array array) {
    List<Value> elements = new ArrayList<>(array.getElementsCount());
    for (org.openmbee.opensysml.proto.Value element : array.getElementsList()) {
      elements.add(readable(element));
    }
    try {
      return new Value.ArrayValue(array.getDimensionsList(), elements);
    } catch (IllegalArgumentException | ArithmeticException malformed) {
      throw new TransportException(
          "the service answered a malformed array: " + malformed.getMessage(), malformed);
    }
  }

  private static Value vector(org.openmbee.opensysml.proto.Vector vector) {
    List<Value> components = new ArrayList<>(vector.getComponentsCount());
    for (org.openmbee.opensysml.proto.Value component : vector.getComponentsList()) {
      components.add(
          switch (component.getKindCase()) {
            case INT_VALUE -> new Value.IntegerValue(component.getIntValue());
            case REAL_VALUE -> new Value.RealValue(component.getRealValue());
            default ->
                throw new TransportException(
                    "the service answered a malformed vector: component is "
                        + component.getKindCase().name().toLowerCase(Locale.ROOT)
                        + ", not a number",
                    null);
          });
    }
    return new Value.VectorValue(components);
  }

  private static Value vectorQuantity(org.openmbee.opensysml.proto.VectorQuantity vector) {
    if (vector.getComponentsCount() == 0) {
      throw new TransportException(
          "the service answered a malformed vector quantity: it has no components", null);
    }
    List<Quantity> components = new ArrayList<>(vector.getComponentsCount());
    for (org.openmbee.opensysml.proto.Quantity component : vector.getComponentsList()) {
      components.add(quantity(component));
    }
    return new Value.VectorQuantityValue(components);
  }

  private static Value set(org.openmbee.opensysml.proto.ValueSet set) {
    List<Value> elements = new ArrayList<>(set.getElementsCount());
    for (org.openmbee.opensysml.proto.Value element : set.getElementsList()) {
      elements.add(readable(element));
    }
    try {
      return new Value.SetValue(elements);
    } catch (IllegalArgumentException malformed) {
      throw new TransportException(
          "the service answered a malformed set: " + malformed.getMessage(), malformed);
    }
  }

  private static Value tensorQuantity(org.openmbee.opensysml.proto.TensorQuantity tensor) {
    List<Quantity> components = new ArrayList<>(tensor.getComponentsCount());
    for (org.openmbee.opensysml.proto.Quantity component : tensor.getComponentsList()) {
      components.add(quantity(component));
    }
    try {
      return new Value.TensorQuantityValue(tensor.getDimensionsList(), components);
    } catch (IllegalArgumentException | ArithmeticException malformed) {
      throw new TransportException(
          "the service answered a malformed tensor quantity: " + malformed.getMessage(),
          malformed);
    }
  }

  private static Value measurementRef(org.openmbee.opensysml.proto.MeasurementRef ref) {
    if (ref.getUnit().isEmpty() && ref.getUnitId().isEmpty() && !ref.hasUnitTerm()) {
      throw new TransportException(
          "the service answered a malformed measurement reference: it names no unit", null);
    }
    if (!ref.hasUnitTerm()) {
      throw new TransportException(
          "the service answered a malformed measurement reference "
              + (ref.getUnit().isEmpty() ? ref.getUnitId() : ref.getUnit())
              + ": it has no reduction to base units",
          null);
    }
    return new Value.MeasurementRefValue(
        ref.getUnit(), unitTerm(ref.getUnitTerm()), present(ref.getUnitId()));
  }

  private static Value metaobject(org.openmbee.opensysml.proto.Metaobject metaobject) {
    if (metaobject.getElementId().isEmpty()) {
      throw new TransportException(
          "the service answered a malformed metaobject: it names no element", null);
    }
    return new Value.MetaobjectValue(metaobject.getElementId(), metaobject.getMetaclassId());
  }

  private static Value function(org.openmbee.opensysml.proto.Function function) {
    if (function.getCalcId().isEmpty()) {
      throw new TransportException(
          "the service answered a malformed function: it names no calc", null);
    }
    return new Value.FunctionValue(
        function.getCalcId(),
        function.getSelfId() == 0 ? Optional.empty() : Optional.of(function.getSelfId()));
  }

  private static Value sequence(org.openmbee.opensysml.proto.Value value) {
    List<Value> elements = new ArrayList<>();
    for (org.openmbee.opensysml.proto.Value element : value.getSequence().getElementsList()) {
      elements.add(readable(element));
    }
    return new Value.Sequence(elements);
  }

  /**
   * A value the client must read rather than drop: answering nothing for a value that was sent
   * would read as a shorter sequence, or as a feature holding no value at all.
   */
  private static Value readable(org.openmbee.opensysml.proto.Value value) {
    return value(value)
        .orElseThrow(
            () ->
                new TransportException(
                    "the service answered a value of a kind this client does not know", null));
  }

  /**
   * A quantity.
   *
   * @param quantity the generated quantity
   * @return the immutable quantity
   * @throws TransportException when the quantity carries no magnitude, which no number stands in for
   */
  public static Quantity quantity(org.openmbee.opensysml.proto.Quantity quantity) {
    Number magnitude =
        switch (quantity.getMagnitudeCase()) {
          case INT_MAGNITUDE -> Long.valueOf(quantity.getIntMagnitude());
          case REAL_MAGNITUDE -> Double.valueOf(quantity.getRealMagnitude());
          case MAGNITUDE_NOT_SET ->
              throw new TransportException(
                  "the service answered a malformed quantity in ["
                      + quantity.getUnit()
                      + "]: it has no magnitude",
                  null);
        };
    Optional<Quantity.UnitTerm> reduction =
        quantity.hasUnitTerm() ? Optional.of(unitTerm(quantity.getUnitTerm())) : Optional.empty();
    return new Quantity(magnitude, present(quantity.getUnit()), reduction);
  }

  private static Quantity.UnitTerm unitTerm(UnitTerm term) {
    List<Quantity.UnitFactor> factors = new ArrayList<>();
    for (UnitFactor factor : term.getFactorsList()) {
      factors.add(new Quantity.UnitFactor(factor.getUnitId(), factor.getExponent()));
    }
    return new Quantity.UnitTerm(term.getScaleNum(), term.getScaleDen(), factors);
  }

  /**
   * An enumeration literal.
   *
   * @param literal the generated literal
   * @return the immutable literal
   */
  public static EnumLiteral literal(org.openmbee.opensysml.proto.EnumLiteral literal) {
    return new EnumLiteral(
        literal.getLiteralId(),
        literal.getEnumerationId(),
        literal.getName(),
        literal.hasValue() ? Optional.of(readable(literal.getValue())) : Optional.empty());
  }

  /**
   * Diagnostics.
   *
   * @param diagnostics the generated diagnostics
   * @return immutable diagnostics, in order
   */
  public static List<Diagnostic> diagnostics(
      List<org.openmbee.opensysml.proto.Diagnostic> diagnostics) {
    List<Diagnostic> read = new ArrayList<>(diagnostics.size());
    for (org.openmbee.opensysml.proto.Diagnostic diagnostic : diagnostics) {
      read.add(
          new Diagnostic(
              Diagnostic.Severity.fromWireName(diagnostic.getSeverity()),
              diagnostic.getMessage(),
              diagnostic.getCode(),
              diagnostic.hasSpan() ? Optional.of(span(diagnostic.getSpan())) : Optional.empty()));
    }
    return List.copyOf(read);
  }

  private static Diagnostic.Span span(Span span) {
    return new Diagnostic.Span(
        span.getFile(),
        span.getStartLine(),
        span.getStartCol(),
        span.getEndLine(),
        span.getEndCol());
  }

  /**
   * A symbol.
   *
   * @param symbol the generated symbol
   * @return the immutable symbol
   */
  public static Symbol symbol(SymbolInfo symbol) {
    List<Symbol.Attribute> attributes = new ArrayList<>(symbol.getAttributesCount());
    for (AttributeInfo attribute : symbol.getAttributesList()) {
      attributes.add(
          new Symbol.Attribute(
              attribute.getName(),
              attribute.getType(),
              attribute.hasValue()
                  ? Optional.of(readable(attribute.getValue()))
                  : Optional.<Value>empty(),
              present(attribute.getUnit())));
    }
    List<Symbol.Specialization> specializations = new ArrayList<>(symbol.getSpecializationsCount());
    for (Specialization specialization : symbol.getSpecializationsList()) {
      specializations.add(
          new Symbol.Specialization(
              specialization.getKind(),
              specialization.getDeclared(),
              present(specialization.getTargetId()),
              present(specialization.getTargetKind())));
    }
    return new Symbol(
        symbol.getId(),
        symbol.getName(),
        symbol.getKind(),
        symbol.getMetadataMap(),
        symbol.getChildIdsList(),
        attributes,
        symbol.hasTypeInfo() ? Optional.of(typeFacts(symbol.getTypeInfo())) : Optional.empty(),
        symbol.hasMultiplicity()
            ? Optional.of(multiplicity(symbol.getMultiplicity()))
            : Optional.empty(),
        specializations,
        symbol.getWithheldLibraryAttributes());
  }

  private static Symbol.TypeFacts typeFacts(TypeInfo typeInfo) {
    return new Symbol.TypeFacts(
        present(typeInfo.getDeclared()),
        present(typeInfo.getResolvedId()),
        present(typeInfo.getResolvedKind()),
        present(typeInfo.getPrimitive()),
        present(typeInfo.getPrimitiveSource()),
        typeInfo.getQuantity(),
        present(typeInfo.getUnit()));
  }

  private static Symbol.Multiplicity multiplicity(MultiplicityInfo multiplicity) {
    return new Symbol.Multiplicity(
        present(multiplicity.getLower()), present(multiplicity.getUpper()));
  }

  /**
   * An instance.
   *
   * @param instance the generated instance
   * @return the immutable instance
   */
  public static Instance instance(org.openmbee.opensysml.proto.Instance instance) {
    Map<String, Instance.FeatureValue> featureValues = new LinkedHashMap<>();
    for (Map.Entry<String, FeatureValue> entry : instance.getFeatureValuesMap().entrySet()) {
      FeatureValue featureValue = entry.getValue();
      List<Value> values = new ArrayList<>(featureValue.getValuesCount());
      for (org.openmbee.opensysml.proto.Value each : featureValue.getValuesList()) {
        values.add(readable(each));
      }
      featureValues.put(
          entry.getKey(),
          new Instance.FeatureValue(
              featureValue.getFeatureName(),
              featureValue.hasValue()
                  ? Optional.of(readable(featureValue.getValue()))
                  : Optional.<Value>empty(),
              values,
              featureValue.getMaterialized(),
              present(featureValue.getError())));
    }
    return new Instance(instance.getId(), instance.getTypeSymbolId(), featureValues);
  }

  /**
   * What an instantiation built.
   *
   * @param response the generated answer
   * @return the immutable instantiation
   */
  public static Instantiation instantiation(InstantiateResponse response) {
    List<Instance> reachable = new ArrayList<>(response.getInstancesCount());
    for (org.openmbee.opensysml.proto.Instance instance : response.getInstancesList()) {
      reachable.add(instance(instance));
    }
    return new Instantiation(
        instance(response.getInstance()), reachable, diagnostics(response.getDiagnosticsList()));
  }

  /**
   * A string field, absent when it holds its default.
   *
   * @param field the field value
   * @return the string, or empty when it is empty
   */
  public static Optional<String> present(String field) {
    return field.isEmpty() ? Optional.empty() : Optional.of(field);
  }

  /**
   * A value as a request carries it.
   *
   * @param value the immutable value
   * @return the generated value
   */
  public static org.openmbee.opensysml.proto.Value proto(Value value) {
    org.openmbee.opensysml.proto.Value.Builder builder =
        org.openmbee.opensysml.proto.Value.newBuilder();
    if (value instanceof Value.IntegerValue integral) {
      builder.setIntValue(integral.value());
    } else if (value instanceof Value.RealValue real) {
      builder.setRealValue(real.value());
    } else if (value instanceof Value.ComplexValue complex) {
      builder.setComplex(
          org.openmbee.opensysml.proto.Complex.newBuilder()
              .setReal(complex.real())
              .setImaginary(complex.imaginary()));
    } else if (value instanceof Value.BooleanValue flag) {
      builder.setBoolValue(flag.value());
    } else if (value instanceof Value.StringValue text) {
      builder.setStringValue(text.value());
    } else if (value instanceof Value.InstanceReference reference) {
      builder.setInstanceId(reference.instanceId());
    } else if (value instanceof Value.Sequence sequence) {
      ValueSequence.Builder elements = ValueSequence.newBuilder();
      sequence.elements().forEach(element -> elements.addElements(proto(element)));
      builder.setSequence(elements);
    } else if (value instanceof Value.NullValue) {
      builder.setNull("");
    } else if (value instanceof Value.UnsetValue) {
      builder.setUnset(true);
    } else if (value instanceof Value.UndeterminedValue undetermined) {
      builder.setUndetermined(
          Undetermined.newBuilder()
              .setReason(undetermined.reason())
              .setCount(
                  MultiplicityInfo.newBuilder()
                      .setLower(undetermined.countLower())
                      .setUpper(undetermined.countUpper())));
    } else if (value instanceof Value.InfinityValue) {
      builder.setInfinity(true);
    } else if (value instanceof Value.QuantityValue quantity) {
      builder.setQuantity(proto(quantity.quantity()));
    } else if (value instanceof Value.EnumerationValue literal) {
      builder.setEnumLiteral(proto(literal.literal()));
    } else if (value instanceof Value.ArrayValue array) {
      org.openmbee.opensysml.proto.Array.Builder elements =
          org.openmbee.opensysml.proto.Array.newBuilder().addAllDimensions(array.dimensions());
      array.elements().forEach(element -> elements.addElements(proto(element)));
      builder.setArray(elements);
    } else if (value instanceof Value.VectorValue vector) {
      org.openmbee.opensysml.proto.Vector.Builder components =
          org.openmbee.opensysml.proto.Vector.newBuilder();
      vector.components().forEach(component -> components.addComponents(proto(component)));
      builder.setVector(components);
    } else if (value instanceof Value.VectorQuantityValue vector) {
      org.openmbee.opensysml.proto.VectorQuantity.Builder components =
          org.openmbee.opensysml.proto.VectorQuantity.newBuilder();
      vector.components().forEach(component -> components.addComponents(proto(component)));
      builder.setVectorQuantity(components);
    } else if (value instanceof Value.MeasurementRefValue ref) {
      org.openmbee.opensysml.proto.MeasurementRef.Builder reference =
          org.openmbee.opensysml.proto.MeasurementRef.newBuilder()
              .setUnit(ref.unit())
              .setUnitTerm(proto(ref.reduction()));
      ref.unitId().ifPresent(reference::setUnitId);
      builder.setMeasurementRef(reference);
    } else if (value instanceof Value.FunctionValue function) {
      builder.setFunction(
          org.openmbee.opensysml.proto.Function.newBuilder()
              .setCalcId(function.calcId())
              .setSelfId(function.selfId().orElse(0L)));
    } else if (value instanceof Value.SetValue set) {
      ValueSet.Builder elements = ValueSet.newBuilder();
      set.elements().forEach(element -> elements.addElements(proto(element)));
      builder.setSet(elements);
    } else if (value instanceof Value.TensorQuantityValue tensor) {
      TensorQuantity.Builder components =
          TensorQuantity.newBuilder().addAllDimensions(tensor.dimensions());
      tensor.components().forEach(component -> components.addComponents(proto(component)));
      builder.setTensorQuantity(components);
    } else if (value instanceof Value.MetaobjectValue metaobject) {
      builder.setMetaobject(
          org.openmbee.opensysml.proto.Metaobject.newBuilder()
              .setElementId(metaobject.elementId())
              .setMetaclassId(metaobject.metaclassId()));
    } else {
      throw new IllegalArgumentException("no wire form for " + value.getClass().getName());
    }
    return builder.build();
  }

  /**
   * Values by name, as a request's map carries them.
   *
   * @param values the immutable values by name
   * @return the generated values by name
   */
  public static Map<String, org.openmbee.opensysml.proto.Value> protos(Map<String, Value> values) {
    Map<String, org.openmbee.opensysml.proto.Value> out = new LinkedHashMap<>();
    values.forEach((name, value) -> out.put(name, proto(value)));
    return out;
  }

  /**
   * Values in order, as a request's list carries them.
   *
   * @param values the immutable values
   * @return the generated values, in order
   */
  public static List<org.openmbee.opensysml.proto.Value> protos(List<Value> values) {
    return values.stream().map(Protos::proto).toList();
  }

  private static org.openmbee.opensysml.proto.Quantity proto(Quantity quantity) {
    org.openmbee.opensysml.proto.Quantity.Builder builder =
        org.openmbee.opensysml.proto.Quantity.newBuilder();
    if (quantity.magnitude() instanceof Long integral) {
      builder.setIntMagnitude(integral);
    } else {
      builder.setRealMagnitude(quantity.magnitude().doubleValue());
    }
    quantity.unit().ifPresent(builder::setUnit);
    quantity.reduction().ifPresent(reduction -> builder.setUnitTerm(proto(reduction)));
    return builder.build();
  }

  private static UnitTerm proto(Quantity.UnitTerm reduction) {
    UnitTerm.Builder term =
        UnitTerm.newBuilder()
            .setScaleNum(reduction.scaleNumerator())
            .setScaleDen(reduction.scaleDenominator());
    for (Quantity.UnitFactor factor : reduction.factors()) {
      term.addFactors(
          UnitFactor.newBuilder().setUnitId(factor.unitId()).setExponent(factor.exponent()));
    }
    return term.build();
  }

  private static org.openmbee.opensysml.proto.EnumLiteral proto(EnumLiteral literal) {
    org.openmbee.opensysml.proto.EnumLiteral.Builder builder =
        org.openmbee.opensysml.proto.EnumLiteral.newBuilder()
            .setLiteralId(literal.literalId())
            .setEnumerationId(literal.enumerationId())
            .setName(literal.name());
    literal.value().ifPresent(value -> builder.setValue(proto(value)));
    return builder.build();
  }

  /**
   * A query as a request carries it.
   *
   * @param query the immutable query
   * @return the generated query
   */
  public static org.openmbee.opensysml.proto.Query proto(Query query) {
    org.openmbee.opensysml.proto.Query.Builder builder =
        org.openmbee.opensysml.proto.Query.newBuilder()
            .addAllScope(query.scope())
            .addAllSelect(query.select());
    query.where().ifPresent(where -> builder.setWhere(proto(where)));
    return builder.build();
  }

  private static Constraint proto(Condition condition) {
    Constraint.Builder builder = Constraint.newBuilder();
    if (condition instanceof Condition.Comparison comparison) {
      builder.setPrimitive(
          PrimitiveConstraint.newBuilder()
              .setInverse(comparison.inverse())
              .setProperty(comparison.property())
              .setOperator(proto(comparison.operator()))
              .addAllValue(comparison.values()));
    } else if (condition instanceof Condition.Combination combination) {
      CompositeConstraint.Builder composite =
          CompositeConstraint.newBuilder().setOperator(proto(combination.operator()));
      combination.conditions().forEach(each -> composite.addConstraint(proto(each)));
      builder.setComposite(composite);
    }
    return builder.build();
  }

  private static PrimitiveOperator proto(Condition.Comparison.Operator operator) {
    return switch (operator) {
      case EQUAL -> PrimitiveOperator.PRIMITIVE_OPERATOR_EQUAL;
      case GREATER -> PrimitiveOperator.PRIMITIVE_OPERATOR_GREATER;
      case LESS -> PrimitiveOperator.PRIMITIVE_OPERATOR_LESS;
    };
  }

  private static CompositeOperator proto(Condition.Combination.Operator operator) {
    return switch (operator) {
      case AND -> CompositeOperator.COMPOSITE_OPERATOR_AND;
      case OR -> CompositeOperator.COMPOSITE_OPERATOR_OR;
    };
  }

  /**
   * The elements a query selected.
   *
   * @param elements the generated elements
   * @return immutable elements, in order
   */
  public static List<QueryElement> queryElements(List<QueryResultElement> elements) {
    return elements.stream()
        .map(
            element ->
                new QueryElement(element.getId(), element.getType(), element.getPropertiesMap()))
        .toList();
  }

  /**
   * A failure reason.
   *
   * @param reason the generated reason
   * @return the matching reason, {@link FailureReason#UNKNOWN} for one this release does not know
   */
  public static FailureReason failureReason(org.openmbee.opensysml.proto.FailureReason reason) {
    return switch (reason) {
      case FAILURE_REASON_UNSPECIFIED -> FailureReason.UNSPECIFIED;
      case FAILURE_REASON_EVALUATION -> FailureReason.EVALUATION;
      case FAILURE_REASON_WRONG_KIND -> FailureReason.WRONG_KIND;
      case FAILURE_REASON_AMBIGUOUS_SUBJECT -> FailureReason.AMBIGUOUS_SUBJECT;
      case UNRECOGNIZED -> FailureReason.UNKNOWN;
    };
  }

  /**
   * The standing of an answer.
   *
   * @param engine the engine field
   * @param strength the strength field
   * @param bounds the bounds
   * @return the immutable standing
   */
  public static Standing standing(String engine, String strength, List<Bound> bounds) {
    List<Standing.Bound> out = new ArrayList<>(bounds.size());
    for (Bound bound : bounds) {
      out.add(new Standing.Bound(bound.getName(), bound.getLimit(), bound.getReached()));
    }
    return new Standing(engine, strength, out);
  }

  /**
   * A verdict.
   *
   * @param verdict the generated verdict
   * @return the immutable verdict
   */
  public static Verdict verdict(org.openmbee.opensysml.proto.Verdict verdict) {
    return new Verdict(
        verdict.getKind(),
        present(verdict.getElementId()),
        verdict.getElement(),
        verdict.getHolds(),
        present(verdict.getCondition()),
        verdict.getInstanceId() == 0 ? Optional.empty() : Optional.of(verdict.getInstanceId()),
        present(verdict.getInstanceTypeId()),
        present(verdict.getError()),
        failureReason(verdict.getFailureReason()),
        present(verdict.getRequirementId()),
        present(verdict.getInstancePath()),
        standing(verdict.getEngine(), verdict.getStrength(), verdict.getBoundsList()));
  }

  /**
   * Verdicts.
   *
   * @param verdicts the generated verdicts
   * @return immutable verdicts, in order
   */
  public static List<Verdict> verdicts(List<org.openmbee.opensysml.proto.Verdict> verdicts) {
    return verdicts.stream().map(Protos::verdict).toList();
  }

  /**
   * The body verdicts of verification cases.
   *
   * @param verdicts the generated verdicts
   * @return immutable verdicts, in order
   */
  public static List<VerificationVerdict> verificationVerdicts(
      List<org.openmbee.opensysml.proto.VerificationVerdict> verdicts) {
    return verdicts.stream()
        .map(
            verdict ->
                new VerificationVerdict(
                    verdict.getCaseId(),
                    verdict.getKind(),
                    present(verdict.getDetail()),
                    verdict.getSubcase(),
                    present(verdict.getRequirementId())))
        .toList();
  }

  /**
   * Instances.
   *
   * @param instances the generated instances
   * @return immutable instances, in order
   */
  public static List<Instance> instances(List<org.openmbee.opensysml.proto.Instance> instances) {
    return instances.stream().map(Protos::instance).toList();
  }

  /**
   * A constraint's verification.
   *
   * @param response the generated answer
   * @return the immutable verification
   */
  public static Verification verification(VerifyConstraintResponse response) {
    return new Verification(
        verdict(response.getVerdict()),
        List.of(),
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * A requirement's verification.
   *
   * @param response the generated answer
   * @return the immutable verification
   */
  public static Verification verification(VerifyRequirementResponse response) {
    return new Verification(
        verdict(response.getVerdict()),
        verificationVerdicts(response.getVerificationVerdictsList()),
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * The satisfaction assertions' verdicts.
   *
   * @param response the generated answer
   * @return the immutable satisfaction
   */
  public static Satisfaction satisfaction(VerifySatisfactionResponse response) {
    return new Satisfaction(
        verdicts(response.getVerdictsList()),
        verificationVerdicts(response.getVerificationVerdictsList()),
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * An object's validation.
   *
   * @param response the generated answer
   * @return the immutable validation
   */
  public static Validation validation(ValidateInstanceResponse response) {
    return new Validation(
        verdict(response.getSummary()),
        verdicts(response.getVerdictsList()),
        verificationVerdicts(response.getVerificationVerdictsList()),
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()),
        response.getBounded());
  }

  /**
   * Values by name, read from a request's or an answer's map.
   *
   * @param values the generated values by name
   * @return immutable values by name
   */
  public static Map<String, Value> values(Map<String, org.openmbee.opensysml.proto.Value> values) {
    Map<String, Value> out = new LinkedHashMap<>();
    values.forEach((name, value) -> out.put(name, readable(value)));
    return out;
  }

  /**
   * Named outputs, in the order reported.
   *
   * @param outputs the generated outputs
   * @return immutable values by name
   */
  public static Map<String, Value> outputs(List<CalcOutput> outputs) {
    Map<String, Value> out = new LinkedHashMap<>();
    for (CalcOutput output : outputs) {
      out.put(output.getName(), readable(output.getValue()));
    }
    return out;
  }

  /**
   * What a calc computed.
   *
   * @param response the generated answer
   * @return the immutable calculation
   */
  public static Calculation calculation(
      org.openmbee.opensysml.proto.EvaluateCalcResponse response) {
    return new Calculation(
        response.hasResult() ? Optional.of(readable(response.getResult())) : Optional.empty(),
        outputs(response.getOutputsList()),
        diagnostics(response.getDiagnosticsList()),
        standing(response.getEngine(), response.getStrength(), response.getBoundsList()));
  }

  /**
   * What an analysis case's run produced.
   *
   * @param response the generated answer
   * @return the immutable analysis
   */
  public static Analysis analysis(RunAnalysisResponse response) {
    List<CaseEvaluation> evaluations = new ArrayList<>(response.getEvaluationsCount());
    for (org.openmbee.opensysml.proto.CaseEvaluation evaluation : response.getEvaluationsList()) {
      List<Value> arguments = new ArrayList<>(evaluation.getArgumentsCount());
      for (org.openmbee.opensysml.proto.Value argument : evaluation.getArgumentsList()) {
        arguments.add(readable(argument));
      }
      evaluations.add(
          new CaseEvaluation(
              evaluation.getFunctionId(),
              arguments,
              evaluation.hasResult() ? Optional.of(readable(evaluation.getResult())) : Optional.empty(),
              present(evaluation.getError()),
              evaluation.getSelected(),
              evaluation.getTied()));
    }
    return new Analysis(
        outputs(response.getOutputsList()),
        verdicts(response.getVerdictsList()),
        verificationVerdicts(response.getVerificationVerdictsList()),
        evaluations,
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()),
        standing(response.getEngine(), response.getStrength(), response.getBoundsList()));
  }

  /**
   * What an action's run produced.
   *
   * @param response the generated answer
   * @param finalTimeReported whether the service reports a run's final time
   * @return the immutable run
   */
  public static ActionRun actionRun(ExecuteActionResponse response, boolean finalTimeReported) {
    return new ActionRun(
        values(response.getOutputsMap()),
        finalTimeReported ? OptionalDouble.of(response.getFinalTime()) : OptionalDouble.empty(),
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * What a state machine's run produced.
   *
   * @param response the generated answer
   * @param finalTimeReported whether the service reports a run's final time
   * @return the immutable run
   */
  public static StateRun stateRun(ExecuteStateResponse response, boolean finalTimeReported) {
    return new StateRun(
        response.getStatesVisitedList(),
        values(response.getFinalContextMap()),
        finalTimeReported ? OptionalDouble.of(response.getFinalTime()) : OptionalDouble.empty(),
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * What an exploration reached.
   *
   * @param outcomes the generated outcomes
   * @param status how the search ended
   * @return the immutable exploration
   */
  public static Exploration exploration(
      List<org.openmbee.opensysml.proto.Outcome> outcomes, ExplorationStatus status) {
    List<Outcome> out = new ArrayList<>(outcomes.size());
    for (org.openmbee.opensysml.proto.Outcome outcome : outcomes) {
      out.add(
          new Outcome(
              values(outcome.getOutputsMap()),
              present(outcome.getFinalState()),
              outcome.getStatesVisitedList(),
              present(outcome.getError()),
              outcome.getLinearizations(),
              outcome.getWitnessList(),
              diagnostics(outcome.getDiagnosticsList())));
    }
    return new Exploration(
        out,
        status.getComplete(),
        status.getRuns(),
        status.getBudgetsHitList(),
        status.getRunsBudget(),
        status.getDepthBudget());
  }

  /**
   * The engines a service answers with.
   *
   * @param engines the generated descriptions
   * @return immutable descriptions, in order
   */
  public static List<EngineInfo> engines(List<org.openmbee.opensysml.proto.EngineInfo> engines) {
    return engines.stream()
        .map(
            engine ->
                new EngineInfo(
                    engine.getName(),
                    engine.getAuthority(),
                    engine.getAnswersList(),
                    engine.getBoundsList(),
                    engine.getProcess(),
                    engine.getProcessFound(),
                    engine.getReady(),
                    engine.getUnavailable(),
                    engine.getKind(),
                    engine.getProtocol(),
                    engine.getSource(),
                    engine.getCommand(),
                    engine.getVersion(),
                    engine.getServed()))
        .toList();
  }

  /**
   * A source document as a parse request carries it.
   *
   * @param document the immutable document
   * @return the generated document
   */
  public static org.openmbee.opensysml.proto.SourceDocument proto(SourceDocument document) {
    org.openmbee.opensysml.proto.SourceDocument.Builder builder =
        org.openmbee.opensysml.proto.SourceDocument.newBuilder();
    document.file().ifPresent(file -> builder.setFilePath(file.toString()));
    document.content().ifPresent(builder::setContent);
    document.name().ifPresent(builder::setName);
    document.language().ifPresent(language -> builder.setLanguage(language.wireName()));
    return builder.build();
  }

  /**
   * One edit as an apply-edits request carries it.
   *
   * @param edit the immutable edit
   * @return the generated operation
   */
  public static org.openmbee.opensysml.proto.EditOperation proto(Edit edit) {
    org.openmbee.opensysml.proto.EditOperation.Builder builder =
        org.openmbee.opensysml.proto.EditOperation.newBuilder();
    if (edit instanceof Edit.SetValue setValue) {
      builder.setSetValue(
          org.openmbee.opensysml.proto.SetValueEdit.newBuilder()
              .setTarget(setValue.target())
              .setValue(setValue.value()));
    } else if (edit instanceof Edit.Rename rename) {
      builder.setRename(
          org.openmbee.opensysml.proto.RenameEdit.newBuilder()
              .setTarget(rename.target())
              .setNewName(rename.newName()));
    } else if (edit instanceof Edit.AddMember addMember) {
      org.openmbee.opensysml.proto.AddMemberEdit.Builder add =
          org.openmbee.opensysml.proto.AddMemberEdit.newBuilder()
              .setOwner(addMember.owner())
              .setKind(addMember.kind())
              .setName(addMember.name());
      addMember.type().ifPresent(add::setType);
      addMember.multiplicity().ifPresent(add::setMultiplicity);
      addMember.value().ifPresent(add::setValue);
      add.addAllSpecializes(addMember.specializes());
      builder.setAddMember(add);
    } else if (edit instanceof Edit.Delete delete) {
      builder.setDelete(
          org.openmbee.opensysml.proto.DeleteEdit.newBuilder()
              .setTarget(delete.target())
              .setCascade(delete.cascade()));
    } else if (edit instanceof Edit.Move move) {
      builder.setMove(
          org.openmbee.opensysml.proto.MoveEdit.newBuilder()
              .setTarget(move.target())
              .setOwner(move.owner()));
    }
    return builder.build();
  }

  /**
   * Edits as an apply-edits request carries them.
   *
   * @param edits the immutable edits
   * @return the generated operations, in order
   */
  public static List<org.openmbee.opensysml.proto.EditOperation> edits(List<Edit> edits) {
    return edits.stream().map(Protos::proto).toList();
  }

  /**
   * A conversion's answer.
   *
   * @param response the generated answer
   * @return the immutable conversion
   */
  public static Conversion conversion(ConvertResponse response) {
    return new Conversion(
        response.getContent(),
        response.getFromFormat(),
        response.getToFormat(),
        diagnostics(response.getDiagnosticsList()),
        response.getExperimental(),
        response.getExperimentalNotice());
  }

  /**
   * An edit batch's answer.
   *
   * @param response the generated answer
   * @return the immutable result
   */
  public static EditResult editResult(ApplyEditsResponse response) {
    List<AppliedEdit> applied = new ArrayList<>(response.getAppliedCount());
    for (org.openmbee.opensysml.proto.AppliedEdit edit : response.getAppliedList()) {
      applied.add(
          new AppliedEdit(
              edit.getOperationIndex(),
              edit.getTarget(),
              edit.getOffset(),
              edit.getLength(),
              edit.getOldText(),
              edit.getNewText(),
              edit.getDocument()));
    }
    List<EditedDocument> documents = new ArrayList<>(response.getDocumentsCount());
    for (org.openmbee.opensysml.proto.EditedDocument document : response.getDocumentsList()) {
      documents.add(new EditedDocument(document.getName(), document.getContent()));
    }
    return new EditResult(
        response.getContent(),
        applied,
        documents,
        diagnostics(response.getDiagnosticsList()));
  }

  /**
   * An edit refusal's kind.
   *
   * @param failure the generated kind
   * @return the matching kind, {@link EditFailure#UNRECOGNIZED} for one this release does not know
   */
  public static EditFailure editFailure(org.openmbee.opensysml.proto.EditFailure failure) {
    return switch (failure) {
      case EDIT_FAILURE_UNSPECIFIED -> EditFailure.UNSPECIFIED;
      case EDIT_FAILURE_NO_OPERATIONS -> EditFailure.NO_OPERATIONS;
      case EDIT_FAILURE_UNKNOWN_TARGET -> EditFailure.UNKNOWN_TARGET;
      case EDIT_FAILURE_AMBIGUOUS_TARGET -> EditFailure.AMBIGUOUS_TARGET;
      case EDIT_FAILURE_NOT_VALUED -> EditFailure.NOT_VALUED;
      case EDIT_FAILURE_INVALID_VALUE -> EditFailure.INVALID_VALUE;
      case EDIT_FAILURE_INVALID_NAME -> EditFailure.INVALID_NAME;
      case EDIT_FAILURE_NOT_NAMED -> EditFailure.NOT_NAMED;
      case EDIT_FAILURE_RENAME_REFERENCED -> EditFailure.RENAME_REFERENCED;
      case EDIT_FAILURE_OVERLAPPING_EDITS -> EditFailure.OVERLAPPING_EDITS;
      case EDIT_FAILURE_RESULT_INVALID -> EditFailure.RESULT_INVALID;
      case EDIT_FAILURE_OWNER_UNKNOWN -> EditFailure.OWNER_UNKNOWN;
      case EDIT_FAILURE_OWNER_NOT_NAMESPACE -> EditFailure.OWNER_NOT_NAMESPACE;
      case EDIT_FAILURE_ILLEGAL_KIND -> EditFailure.ILLEGAL_KIND;
      case EDIT_FAILURE_MEMBER_NAME_TAKEN -> EditFailure.MEMBER_NAME_TAKEN;
      case EDIT_FAILURE_DELETE_REFERENCED -> EditFailure.DELETE_REFERENCED;
      case EDIT_FAILURE_OWNER_INSIDE_TARGET -> EditFailure.OWNER_INSIDE_TARGET;
      case EDIT_FAILURE_MOVE_REFERENCED -> EditFailure.MOVE_REFERENCED;
      case EDIT_FAILURE_REFERENCED_ELSEWHERE -> EditFailure.REFERENCED_ELSEWHERE;
      case UNRECOGNIZED -> EditFailure.UNRECOGNIZED;
    };
  }

  /**
   * An edit refusal's wire name: the enum's for a known kind, {@code EDIT_FAILURE_<n>} for a number
   * this build has no name for.
   *
   * @param failure the generated kind
   * @param value its number, as {@code getFailureValue()} reports it — {@link
   *     org.openmbee.opensysml.proto.EditFailure#UNRECOGNIZED} itself carries none
   * @return its name on the wire
   */
  public static String editFailureName(
      org.openmbee.opensysml.proto.EditFailure failure, int value) {
    return failure == org.openmbee.opensysml.proto.EditFailure.UNRECOGNIZED
        ? "EDIT_FAILURE_" + value
        : failure.name();
  }

  /**
   * The referrers of a refused edit.
   *
   * @param referrers the generated referrers
   * @return immutable referrers, in order
   */
  public static List<Referrer> referrers(
      List<org.openmbee.opensysml.proto.Referrer> referrers) {
    return referrers.stream()
        .map(referrer -> new Referrer(referrer.getName(), referrer.getDocument()))
        .toList();
  }

  /**
   * A sweep range as a request carries it.
   *
   * @param range the immutable range
   * @return the generated range
   */
  public static org.openmbee.opensysml.proto.SweepRange proto(SweepRange range) {
    org.openmbee.opensysml.proto.SweepRange.Builder builder =
        org.openmbee.opensysml.proto.SweepRange.newBuilder()
            .setParameter(range.parameter())
            .setStart(proto(range.start()))
            .setEnd(proto(range.end()));
    range.step().ifPresent(step -> builder.setStep(proto(step)));
    return builder.build();
  }

  /**
   * What a sweep ran.
   *
   * @param response the generated answer
   * @return the immutable table
   */
  public static Sweep sweep(RunSweepResponse response) {
    List<SweepRow> rows = new ArrayList<>(response.getRowsCount());
    for (org.openmbee.opensysml.proto.SweepRow row : response.getRowsList()) {
      List<CaseEvaluation> evaluations = new ArrayList<>(row.getEvaluationsCount());
      for (org.openmbee.opensysml.proto.CaseEvaluation evaluation : row.getEvaluationsList()) {
        List<Value> arguments = new ArrayList<>(evaluation.getArgumentsCount());
        for (org.openmbee.opensysml.proto.Value argument : evaluation.getArgumentsList()) {
          arguments.add(readable(argument));
        }
        evaluations.add(
            new CaseEvaluation(
                evaluation.getFunctionId(),
                arguments,
                evaluation.hasResult()
                    ? Optional.of(readable(evaluation.getResult()))
                    : Optional.empty(),
                present(evaluation.getError()),
                evaluation.getSelected(),
                evaluation.getTied()));
      }
      rows.add(
          new SweepRow(
              outputs(row.getInputsList()),
              outputs(row.getOutputsList()),
              verdicts(row.getVerdictsList()),
              evaluations,
              Duration.of(row.getElapsedMicros(), ChronoUnit.MICROS),
              row.getError(),
              failureReason(row.getFailureReason())));
    }
    return new Sweep(
        rows,
        response.getParametersList(),
        response.getSampled(),
        response.getSeed(),
        instances(response.getInstancesList()),
        diagnostics(response.getDiagnosticsList()),
        standing(response.getEngine(), response.getStrength(), response.getBoundsList()));
  }

  /**
   * A document-query value.
   *
   * @param value the generated value
   * @return the immutable value; an {@link DocumentValue.ElementRef} for one naming no kind, which
   *     is how an anonymous element's bare {@code element_type} reads
   */
  public static DocumentValue documentValue(org.openmbee.opensysml.proto.DocumentValue value) {
    DocumentValue read =
        switch (value.getKindCase()) {
          case ELEMENT_ID -> new DocumentValue.ElementRef(value.getElementId(), "");
          case STRING_VALUE -> new DocumentValue.StringValue(value.getStringValue());
          case INT_VALUE -> new DocumentValue.IntegerValue(value.getIntValue());
          case REAL_VALUE -> new DocumentValue.RealValue(value.getRealValue());
          case BOOL_VALUE -> new DocumentValue.BooleanValue(value.getBoolValue());
          case INFINITY -> new DocumentValue.InfinityValue();
          case QUANTITY -> new DocumentValue.QuantityValue(quantity(value.getQuantity()));
          case VERDICT -> documentVerdict(value.getVerdict());
          case OBJECT -> objectRef(value.getObject());
          case STATE -> documentState(value.getState());
          case EVENT -> documentEvent(value.getEvent());
          case KIND_NOT_SET ->
              new DocumentValue.ElementRef("", "");
        };
    if (read instanceof DocumentValue.ElementRef element && !value.getElementType().isEmpty()) {
      return new DocumentValue.ElementRef(element.id(), value.getElementType());
    }
    return read;
  }

  private static DocumentValue.ObjectRef objectRef(
      org.openmbee.opensysml.proto.DocumentObject object) {
    return new DocumentValue.ObjectRef(
        object.getInstanceId(),
        object.getPath(),
        object.hasElement()
            ? Optional.of(elementRef(object.getElement()))
            : Optional.empty());
  }

  /** An element value as the reference it names, including an anonymous one's bare type. */
  private static DocumentValue.ElementRef elementRef(
      org.openmbee.opensysml.proto.DocumentValue value) {
    if (value.getKindCase()
        == org.openmbee.opensysml.proto.DocumentValue.KindCase.ELEMENT_ID) {
      return new DocumentValue.ElementRef(value.getElementId(), value.getElementType());
    }
    return new DocumentValue.ElementRef("", value.getElementType());
  }

  private static DocumentValue.DocumentVerdict documentVerdict(
      org.openmbee.opensysml.proto.DocumentVerdict verdict) {
    return new DocumentValue.DocumentVerdict(
        verdict.hasAssertion()
            ? elementRef(verdict.getAssertion())
            : new DocumentValue.ElementRef("", ""),
        verdict.getKind(),
        verdict.getText(),
        verdict.getPath(),
        verdict.getVerdict(),
        verdict.getCondition(),
        verdict.getReason(),
        verdict.getVerificationList());
  }

  private static DocumentValue.DocumentState documentState(
      org.openmbee.opensysml.proto.DocumentState state) {
    return new DocumentValue.DocumentState(
        objectRef(state.getObject()),
        state.getMachine(),
        state.getName(),
        state.getStatePath(),
        state.hasState() ? Optional.of(elementRef(state.getState())) : Optional.empty(),
        state.getRegion(),
        state.getEnclosingList());
  }

  private static DocumentValue.DocumentEvent documentEvent(
      org.openmbee.opensysml.proto.DocumentEvent event) {
    return new DocumentValue.DocumentEvent(
        event.getKind(),
        documentValue(event.getTime()),
        event.getText(),
        event.hasObject() ? Optional.of(objectRef(event.getObject())) : Optional.empty(),
        event.getMachine(),
        event.getState(),
        event.getFrom(),
        event.getTo(),
        event.hasTarget() ? Optional.of(objectRef(event.getTarget())) : Optional.empty(),
        event.getEvent(),
        event.getPayloadList(),
        event.getAlternativesList(),
        event.getTaken());
  }

  /**
   * A document-query value as a request's binding carries it.
   *
   * @param value the immutable value
   * @return the generated value
   */
  public static org.openmbee.opensysml.proto.DocumentValue proto(DocumentValue value) {
    org.openmbee.opensysml.proto.DocumentValue.Builder builder =
        org.openmbee.opensysml.proto.DocumentValue.newBuilder();
    if (value instanceof DocumentValue.ElementRef element) {
      if (!element.id().isEmpty()) {
        builder.setElementId(element.id());
      }
      if (!element.type().isEmpty()) {
        builder.setElementType(element.type());
      }
    } else if (value instanceof DocumentValue.ObjectRef object) {
      org.openmbee.opensysml.proto.DocumentObject.Builder reference =
          org.openmbee.opensysml.proto.DocumentObject.newBuilder()
              .setInstanceId(object.id())
              .setPath(object.path());
      object.element().ifPresent(element -> reference.setElement(proto(element)));
      builder.setObject(reference);
    } else if (value instanceof DocumentValue.StringValue text) {
      builder.setStringValue(text.value());
    } else if (value instanceof DocumentValue.IntegerValue integer) {
      builder.setIntValue(integer.value());
    } else if (value instanceof DocumentValue.RealValue real) {
      builder.setRealValue(real.value());
    } else if (value instanceof DocumentValue.BooleanValue flag) {
      builder.setBoolValue(flag.value());
    } else if (value instanceof DocumentValue.InfinityValue) {
      builder.setInfinity(true);
    } else if (value instanceof DocumentValue.QuantityValue quantity) {
      builder.setQuantity(proto(quantity.quantity()));
    } else if (value instanceof DocumentValue.DocumentVerdict verdict) {
      builder.setVerdict(
          org.openmbee.opensysml.proto.DocumentVerdict.newBuilder()
              .setAssertion(proto(verdict.assertion()))
              .setKind(verdict.kind())
              .setText(verdict.text())
              .setPath(verdict.path())
              .setVerdict(verdict.status())
              .setCondition(verdict.condition())
              .setReason(verdict.reason())
              .addAllVerification(verdict.verification()));
    } else if (value instanceof DocumentValue.DocumentState state) {
      org.openmbee.opensysml.proto.DocumentState.Builder row =
          org.openmbee.opensysml.proto.DocumentState.newBuilder()
              .setObject(protoObject(state.object()))
              .setMachine(state.machine())
              .setName(state.name())
              .setStatePath(state.path())
              .setRegion(state.region())
              .addAllEnclosing(state.enclosing());
      state.state().ifPresent(element -> row.setState(proto(element)));
      builder.setState(row);
    } else if (value instanceof DocumentValue.DocumentEvent event) {
      org.openmbee.opensysml.proto.DocumentEvent.Builder record =
          org.openmbee.opensysml.proto.DocumentEvent.newBuilder()
              .setKind(event.kind())
              .setTime(proto(event.time()))
              .setText(event.text())
              .setMachine(event.machine())
              .setState(event.state())
              .setFrom(event.from())
              .setTo(event.to())
              .setEvent(event.event())
              .addAllPayload(event.payload())
              .addAllAlternatives(event.alternatives())
              .setTaken(event.taken());
      event.object().ifPresent(object -> record.setObject(protoObject(object)));
      event.target().ifPresent(target -> record.setTarget(protoObject(target)));
      builder.setEvent(record);
    }
    return builder.build();
  }

  private static org.openmbee.opensysml.proto.DocumentObject protoObject(
      DocumentValue.ObjectRef object) {
    org.openmbee.opensysml.proto.DocumentObject.Builder builder =
        org.openmbee.opensysml.proto.DocumentObject.newBuilder()
            .setInstanceId(object.id())
            .setPath(object.path());
    object.element().ifPresent(element -> builder.setElement(proto(element)));
    return builder.build();
  }

  /**
   * A document query's answer: its columns, and each row's element read the way the query's kind
   * reports it.
   *
   * @param response the generated answer
   * @return the immutable result
   */
  public static DocumentQueryResult documentQueryResult(RunDocumentQueryResponse response) {
    List<String> columns = new ArrayList<>(response.getColumnsCount());
    for (org.openmbee.opensysml.proto.DocumentQueryColumn column : response.getColumnsList()) {
      columns.add(column.getName());
    }
    List<DocumentRow> rows = new ArrayList<>(response.getRowsCount());
    for (org.openmbee.opensysml.proto.DocumentQueryRow row : response.getRowsList()) {
      List<List<DocumentValue>> cells = new ArrayList<>(row.getCellsCount());
      for (org.openmbee.opensysml.proto.DocumentQueryCell cell : row.getCellsList()) {
        List<DocumentValue> values = new ArrayList<>(cell.getValuesCount());
        for (org.openmbee.opensysml.proto.DocumentValue value : cell.getValuesList()) {
          values.add(documentValue(value));
        }
        cells.add(values);
      }
      rows.add(documentRow(row.getElement(), cells));
    }
    return new DocumentQueryResult(columns, rows);
  }

  private static DocumentRow documentRow(
      org.openmbee.opensysml.proto.DocumentValue element, List<List<DocumentValue>> cells) {
    DocumentValue value = documentValue(element);
    if (value instanceof DocumentValue.DocumentVerdict verdict) {
      return new DocumentRow(
          verdict.assertion(), cells, Optional.of(verdict), Optional.empty(),
          Optional.empty(), Optional.empty());
    }
    if (value instanceof DocumentValue.ObjectRef object) {
      return new DocumentRow(
          object.element().orElse(new DocumentValue.ElementRef("", "")), cells, Optional.empty(),
          Optional.of(object), Optional.empty(), Optional.empty());
    }
    if (value instanceof DocumentValue.DocumentState state) {
      return new DocumentRow(
          state.object().element().orElse(new DocumentValue.ElementRef("", "")), cells,
          Optional.empty(), Optional.of(state.object()), Optional.of(state), Optional.empty());
    }
    if (value instanceof DocumentValue.DocumentEvent event) {
      return new DocumentRow(
          event.object()
              .flatMap(DocumentValue.ObjectRef::element)
              .orElse(new DocumentValue.ElementRef("", "")),
          cells, Optional.empty(), event.object(), Optional.empty(), Optional.of(event));
    }
    if (value instanceof DocumentValue.ElementRef ref) {
      return new DocumentRow(
          ref, cells, Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty());
    }
    return new DocumentRow(
        new DocumentValue.ElementRef("", element.getElementType()),
        cells, Optional.empty(), Optional.empty(), Optional.empty(), Optional.empty());
  }
}
