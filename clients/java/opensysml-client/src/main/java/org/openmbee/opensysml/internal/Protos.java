package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.EnumLiteral;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.InstantiateResponse;
import org.openmbee.opensysml.proto.MultiplicityInfo;
import org.openmbee.opensysml.proto.Span;
import org.openmbee.opensysml.proto.Specialization;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.TypeInfo;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;

/**
 * Reads the generated messages into the client's own immutable types, so no generated class and no
 * builder reaches a caller.
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
      case INFINITY -> Optional.of(infinity(value));
      case ARRAY -> Optional.of(array(value.getArray()));
      case VECTOR -> Optional.of(vector(value.getVector()));
      case VECTOR_QUANTITY -> Optional.of(vectorQuantity(value.getVectorQuantity()));
      case MEASUREMENT_REF -> Optional.of(measurementRef(value.getMeasurementRef()));
      case FUNCTION -> Optional.of(function(value.getFunction()));
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
        literal.getLiteralId(), literal.getEnumerationId(), literal.getName());
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
}
