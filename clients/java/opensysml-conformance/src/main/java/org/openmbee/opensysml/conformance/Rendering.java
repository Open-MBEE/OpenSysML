package org.openmbee.opensysml.conformance;

import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.EnumLiteral;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.MultiplicityInfo;
import org.openmbee.opensysml.proto.Span;
import org.openmbee.opensysml.proto.Specialization;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.TypeInfo;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import org.openmbee.opensysml.proto.ValueSequence;
import java.util.List;
import java.util.Map;

/**
 * Writes the client's public types back into the generated messages the scenarios are stated
 * against. Comparing that rendering, rather than the transport's answer, is what makes a scenario a
 * test of the client's own reading of a response and not only of the service.
 */
final class Rendering {

  private Rendering() {}

  /**
   * A value.
   *
   * @param value the immutable value
   * @return the generated value
   */
  static org.openmbee.opensysml.proto.Value value(Value value) {
    org.openmbee.opensysml.proto.Value.Builder builder = org.openmbee.opensysml.proto.Value.newBuilder();
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
      sequence.elements().forEach(element -> elements.addElements(value(element)));
      builder.setSequence(elements);
    } else if (value instanceof Value.NullValue) {
      builder.setNull("");
    } else if (value instanceof Value.UnsetValue) {
      builder.setUnset(true);
    } else if (value instanceof Value.InfinityValue) {
      builder.setInfinity(true);
    } else if (value instanceof Value.QuantityValue quantity) {
      builder.setQuantity(quantity(quantity.quantity()));
    } else if (value instanceof Value.EnumerationValue literal) {
      builder.setEnumLiteral(literal(literal.literal()));
    } else if (value instanceof Value.ArrayValue array) {
      org.openmbee.opensysml.proto.Array.Builder elements =
          org.openmbee.opensysml.proto.Array.newBuilder().addAllDimensions(array.dimensions());
      array.elements().forEach(element -> elements.addElements(value(element)));
      builder.setArray(elements);
    } else if (value instanceof Value.VectorValue vector) {
      org.openmbee.opensysml.proto.Vector.Builder components =
          org.openmbee.opensysml.proto.Vector.newBuilder();
      vector.components().forEach(component -> components.addComponents(value(component)));
      builder.setVector(components);
    } else if (value instanceof Value.VectorQuantityValue vector) {
      org.openmbee.opensysml.proto.VectorQuantity.Builder components =
          org.openmbee.opensysml.proto.VectorQuantity.newBuilder();
      vector.components().forEach(component -> components.addComponents(quantity(component)));
      builder.setVectorQuantity(components);
    } else if (value instanceof Value.MeasurementRefValue ref) {
      org.openmbee.opensysml.proto.MeasurementRef.Builder reference =
          org.openmbee.opensysml.proto.MeasurementRef.newBuilder()
              .setUnit(ref.unit())
              .setUnitTerm(unitTerm(ref.reduction()));
      ref.unitId().ifPresent(reference::setUnitId);
      builder.setMeasurementRef(reference);
    } else if (value instanceof Value.FunctionValue function) {
      builder.setFunction(
          org.openmbee.opensysml.proto.Function.newBuilder()
              .setCalcId(function.calcId())
              .setSelfId(function.selfId().orElse(0L)));
    } else {
      throw new IllegalStateException("no rendering for " + value.getClass());
    }
    return builder.build();
  }

  private static org.openmbee.opensysml.proto.Quantity quantity(Quantity quantity) {
    org.openmbee.opensysml.proto.Quantity.Builder builder = org.openmbee.opensysml.proto.Quantity.newBuilder();
    if (quantity.magnitude() instanceof Long integral) {
      builder.setIntMagnitude(integral);
    } else {
      builder.setRealMagnitude(quantity.magnitude().doubleValue());
    }
    quantity.unit().ifPresent(builder::setUnit);
    quantity.reduction().ifPresent(reduction -> builder.setUnitTerm(unitTerm(reduction)));
    return builder.build();
  }

  private static UnitTerm unitTerm(Quantity.UnitTerm reduction) {
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

  private static org.openmbee.opensysml.proto.EnumLiteral literal(EnumLiteral literal) {
    return org.openmbee.opensysml.proto.EnumLiteral.newBuilder()
        .setLiteralId(literal.literalId())
        .setEnumerationId(literal.enumerationId())
        .setName(literal.name())
        .build();
  }

  /**
   * Diagnostics.
   *
   * @param diagnostics the immutable diagnostics
   * @return the generated diagnostics, in order
   */
  static List<org.openmbee.opensysml.proto.Diagnostic> diagnostics(List<Diagnostic> diagnostics) {
    return diagnostics.stream().map(Rendering::diagnostic).toList();
  }

  private static org.openmbee.opensysml.proto.Diagnostic diagnostic(Diagnostic diagnostic) {
    org.openmbee.opensysml.proto.Diagnostic.Builder builder =
        org.openmbee.opensysml.proto.Diagnostic.newBuilder()
            .setSeverity(diagnostic.severity().wireName())
            .setMessage(diagnostic.message())
            .setCode(diagnostic.code());
    diagnostic
        .span()
        .ifPresent(
            span ->
                builder.setSpan(
                    Span.newBuilder()
                        .setFile(span.file())
                        .setStartLine(span.startLine())
                        .setStartCol(span.startColumn())
                        .setEndLine(span.endLine())
                        .setEndCol(span.endColumn())));
    return builder.build();
  }

  /**
   * A symbol.
   *
   * @param symbol the immutable symbol
   * @return the generated symbol
   */
  static SymbolInfo symbol(Symbol symbol) {
    SymbolInfo.Builder builder =
        SymbolInfo.newBuilder()
            .setId(symbol.id())
            .setName(symbol.name())
            .setKind(symbol.kind())
            .putAllMetadata(symbol.metadata())
            .addAllChildIds(symbol.childIds())
            .setWithheldLibraryAttributes(symbol.withheldLibraryAttributes());
    for (Symbol.Attribute attribute : symbol.attributes()) {
      AttributeInfo.Builder rendered =
          AttributeInfo.newBuilder().setName(attribute.name()).setType(attribute.type());
      attribute.value().ifPresent(value -> rendered.setValue(value(value)));
      attribute.unit().ifPresent(rendered::setUnit);
      builder.addAttributes(rendered);
    }
    for (Symbol.Specialization specialization : symbol.specializations()) {
      Specialization.Builder rendered =
          Specialization.newBuilder()
              .setKind(specialization.kind())
              .setDeclared(specialization.declared());
      specialization.targetId().ifPresent(rendered::setTargetId);
      specialization.targetKind().ifPresent(rendered::setTargetKind);
      builder.addSpecializations(rendered);
    }
    symbol
        .typeFacts()
        .ifPresent(
            facts -> {
              TypeInfo.Builder rendered = TypeInfo.newBuilder().setQuantity(facts.quantity());
              facts.declared().ifPresent(rendered::setDeclared);
              facts.resolvedId().ifPresent(rendered::setResolvedId);
              facts.resolvedKind().ifPresent(rendered::setResolvedKind);
              facts.primitive().ifPresent(rendered::setPrimitive);
              facts.primitiveSource().ifPresent(rendered::setPrimitiveSource);
              facts.unit().ifPresent(rendered::setUnit);
              builder.setTypeInfo(rendered);
            });
    symbol
        .multiplicity()
        .ifPresent(
            multiplicity -> {
              MultiplicityInfo.Builder rendered = MultiplicityInfo.newBuilder();
              multiplicity.lower().ifPresent(rendered::setLower);
              multiplicity.upper().ifPresent(rendered::setUpper);
              builder.setMultiplicity(rendered);
            });
    return builder.build();
  }

  /**
   * An instance.
   *
   * @param instance the immutable instance
   * @return the generated instance
   */
  static org.openmbee.opensysml.proto.Instance instance(Instance instance) {
    org.openmbee.opensysml.proto.Instance.Builder builder =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(instance.id())
            .setTypeSymbolId(instance.typeSymbolId());
    for (Map.Entry<String, Instance.FeatureValue> entry : instance.featureValues().entrySet()) {
      Instance.FeatureValue featureValue = entry.getValue();
      FeatureValue.Builder rendered =
          FeatureValue.newBuilder()
              .setFeatureName(featureValue.featureName())
              .setMaterialized(featureValue.materialized());
      featureValue.value().ifPresent(value -> rendered.setValue(value(value)));
      featureValue.values().forEach(value -> rendered.addValues(value(value)));
      featureValue.error().ifPresent(rendered::setError);
      builder.putFeatureValues(entry.getKey(), rendered.build());
    }
    return builder.build();
  }
}
