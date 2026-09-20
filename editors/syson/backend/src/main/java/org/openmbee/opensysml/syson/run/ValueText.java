package org.openmbee.opensysml.syson.run;

import java.util.List;
import java.util.stream.Collectors;

import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Value;

public final class ValueText {
    private ValueText() {}

    public static String render(Value value) {
        if (value instanceof Value.IntegerValue integer) return Long.toString(integer.value());
        if (value instanceof Value.RealValue real) return Double.toString(real.value());
        if (value instanceof Value.ComplexValue complex) return complex.format();
        if (value instanceof Value.BooleanValue bool) return Boolean.toString(bool.value());
        if (value instanceof Value.StringValue string) return "\"" + string.value() + "\"";
        if (value instanceof Value.NullValue) return "null";
        if (value instanceof Value.UnsetValue) return "unset";
        if (value instanceof Value.UndeterminedValue undetermined) return undetermined.reason();
        if (value instanceof Value.InfinityValue) return "infinity";
        if (value instanceof Value.InstanceReference reference) return "#" + reference.instanceId();
        if (value instanceof Value.Sequence sequence) return list(sequence.elements());
        if (value instanceof Value.SetValue set) return list(set.elements());
        if (value instanceof Value.ArrayValue array) return list(array.elements());
        if (value instanceof Value.VectorValue vector) return list(vector.components());
        if (value instanceof Value.QuantityValue quantity) return quantity(quantity.quantity());
        if (value instanceof Value.VectorQuantityValue vector)
            return vector.components().stream().map(ValueText::quantity).collect(Collectors.joining(", ", "[", "]"));
        if (value instanceof Value.TensorQuantityValue tensor)
            return tensor.components().stream().map(ValueText::quantity).collect(Collectors.joining(", ", "[", "]"));
        if (value instanceof Value.MeasurementRefValue measurement) return measurement.unit();
        if (value instanceof Value.FunctionValue function) return function.calcId();
        if (value instanceof Value.MetaobjectValue metaobject) return metaobject.elementId();
        if (value instanceof Value.EnumerationValue enumeration) return enumeration.literal().name();
        return value.toString();
    }

    private static String list(List<Value> values) {
        return values.stream().map(ValueText::render).collect(Collectors.joining(", ", "[", "]"));
    }

    private static String quantity(Quantity value) {
        return value.magnitude() + value.unit().map(unit -> " " + unit).orElse("");
    }
}
