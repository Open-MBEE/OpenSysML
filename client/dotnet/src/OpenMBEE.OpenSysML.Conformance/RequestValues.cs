using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Numerics;
using Google.Protobuf.Collections;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Reads the values a scenario's request sends into the public <see cref="Value"/>s a typed call takes,
/// using only public constructors, so the call writes them back onto the wire itself. A value the public
/// types cannot hold as spelled is a request the API cannot express.
/// </summary>
internal static class RequestValues
{
    /// <summary>A request's map of values, by name; null when it holds none.</summary>
    /// <exception cref="UnexpressibleRequestException">A value cannot be held by the public types as spelled.</exception>
    internal static Dictionary<string, Value>? Map(MapField<string, Wire.Value> values, string field) =>
        values.Count == 0
            ? null
            : values.ToDictionary(entry => entry.Key, entry => Read(entry.Value, $"{field}[\"{entry.Key}\"]"), StringComparer.Ordinal);

    /// <summary>A request's list of values, in order.</summary>
    /// <exception cref="UnexpressibleRequestException">A value cannot be held by the public types as spelled.</exception>
    internal static List<Value> List(RepeatedField<Wire.Value> values, string field) => All(values, field);

    /// <summary>One value.</summary>
    /// <exception cref="UnexpressibleRequestException">The value cannot be held by the public types as spelled.</exception>
    internal static Value Read(Wire.Value value, string path)
    {
        try
        {
            return value.KindCase switch
            {
                Wire.Value.KindOneofCase.IntValue => new Value.IntegerValue(value.IntValue),
                Wire.Value.KindOneofCase.BigIntValue => new Value.BigIntegerValue(Integer(value.BigIntValue, path)),
                Wire.Value.KindOneofCase.RationalValue => new Value.RationalValue(Rational(value.RationalValue, path)),
                Wire.Value.KindOneofCase.RealValue => new Value.RealValue(value.RealValue),
                Wire.Value.KindOneofCase.Complex => new Value.ComplexValue(value.Complex.Real, value.Complex.Imaginary),
                Wire.Value.KindOneofCase.BoolValue => new Value.BooleanValue(value.BoolValue),
                Wire.Value.KindOneofCase.StringValue => new Value.TextValue(value.StringValue),
                Wire.Value.KindOneofCase.InstanceId => new Value.InstanceRefValue(value.InstanceId),
                Wire.Value.KindOneofCase.EnumLiteral => new Value.EnumLiteralValue(
                    value.EnumLiteral.LiteralId,
                    value.EnumLiteral.EnumerationId,
                    value.EnumLiteral.Name,
                    value.EnumLiteral.Value is { } scalar ? Read(scalar, path + ".enum_literal.value") : null),
                Wire.Value.KindOneofCase.Sequence => new Value.SequenceValue(All(value.Sequence.Elements, path + ".sequence")),
                Wire.Value.KindOneofCase.Set => new Value.SetValue(All(value.Set.Elements, path + ".set")),
                Wire.Value.KindOneofCase.Quantity => new Value.QuantityValue(Quantity(value.Quantity, path + ".quantity")),
                Wire.Value.KindOneofCase.Array => new Value.ArrayValue(value.Array.Dimensions, All(value.Array.Elements, path + ".array")),
                Wire.Value.KindOneofCase.Vector => new Value.VectorValue(
                    value.Vector.Components.Select((component, i) => Read(component, $"{path}.vector[{i}]") as Value.NumberValue
                        ?? throw new UnexpressibleRequestException($"the public API holds a vector's components as numbers, which {path}.vector[{i}] is not"))
                    .ToList()),
                Wire.Value.KindOneofCase.VectorQuantity => new Value.VectorQuantityValue(
                    value.VectorQuantity.Components.Select((component, i) => Quantity(component, $"{path}.vector_quantity[{i}]")).ToList()),
                Wire.Value.KindOneofCase.TensorQuantity => new Value.TensorQuantityValue(
                    value.TensorQuantity.Dimensions,
                    value.TensorQuantity.Components.Select((component, i) => Quantity(component, $"{path}.tensor_quantity[{i}]")).ToList()),
                Wire.Value.KindOneofCase.MeasurementRef => new Value.MeasurementRefValue(
                    value.MeasurementRef.Unit,
                    value.MeasurementRef.UnitTerm is { } reduction
                        ? Term(reduction)
                        : throw new UnexpressibleRequestException($"the public API holds a measurement reference with its reduction, which {path} has none of"),
                    value.MeasurementRef.UnitId.Length > 0 ? value.MeasurementRef.UnitId : null),
                Wire.Value.KindOneofCase.Function => new Value.FunctionValue(value.Function.CalcId, value.Function.SelfId == 0 ? null : value.Function.SelfId),
                Wire.Value.KindOneofCase.Metaobject => new Value.MetaobjectValue(value.Metaobject.ElementId, value.Metaobject.MetaclassId),
                Wire.Value.KindOneofCase.Null => new Value.NullValue(value.Null),
                Wire.Value.KindOneofCase.Unset when value.Unset => new Value.UnsetValue(),
                Wire.Value.KindOneofCase.Undetermined => new Value.UndeterminedValue(
                    value.Undetermined.Reason,
                    value.Undetermined.Count?.Lower ?? string.Empty,
                    value.Undetermined.Count?.Upper ?? string.Empty),
                Wire.Value.KindOneofCase.Infinity when value.Infinity => new Value.InfinityValue(),
                var kind => throw new UnexpressibleRequestException($"the public API holds no value as {path} spells it ({kind})"),
            };
        }
        catch (ArgumentException invalid)
        {
            var reason = invalid.ParamName is { } name ? invalid.Message.Replace($" (Parameter '{name}')", string.Empty, StringComparison.Ordinal) : invalid.Message;
            throw new UnexpressibleRequestException($"the public API cannot hold {path} as spelled: {reason}");
        }
    }

    private static List<Value> All(RepeatedField<Wire.Value> values, string path) =>
        [.. values.Select((value, i) => Read(value, $"{path}[{i}]"))];

    private static Quantity Quantity(Wire.Quantity quantity, string path)
    {
        Value.NumberValue magnitude = quantity.MagnitudeCase switch
        {
            Wire.Quantity.MagnitudeOneofCase.IntMagnitude => new Value.IntegerValue(quantity.IntMagnitude),
            Wire.Quantity.MagnitudeOneofCase.BigIntMagnitude => new Value.BigIntegerValue(Integer(quantity.BigIntMagnitude, path)),
            Wire.Quantity.MagnitudeOneofCase.RationalMagnitude => new Value.RationalValue(Rational(quantity.RationalMagnitude, path)),
            Wire.Quantity.MagnitudeOneofCase.RealMagnitude => new Value.RealValue(quantity.RealMagnitude),
            _ => throw new UnexpressibleRequestException($"the public API holds a quantity with its magnitude, which {path} has none of"),
        };
        return new Quantity(magnitude, quantity.Unit, quantity.UnitTerm is { } reduction ? Term(reduction) : null);
    }

    private static UnitTerm Term(Wire.UnitTerm term) =>
        new(term.ScaleNum, term.ScaleDen, term.Factors.Select(factor => new UnitFactor(factor.UnitId, factor.Exponent)));

    // The public types hold an integer and a rational by value; one spelled otherwise than they write it cannot be sent as spelled.
    private static BigInteger Integer(string digits, string path)
    {
        if (!BigInteger.TryParse(digits, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out var integer)
            || integer.ToString(CultureInfo.InvariantCulture) != digits)
        {
            throw new UnexpressibleRequestException($"the public API writes an integer in canonical digits, which {path}'s \"{digits}\" is not");
        }

        return integer;
    }

    private static Rational Rational(Wire.Rational terms, string path)
    {
        var rational = new Rational(Integer(terms.Numerator, path), Integer(terms.Denominator, path));
        if (rational.Numerator.ToString(CultureInfo.InvariantCulture) != terms.Numerator
            || rational.Denominator.ToString(CultureInfo.InvariantCulture) != terms.Denominator)
        {
            throw new UnexpressibleRequestException($"the public API writes a rational in lowest terms over a positive denominator, which {path}'s {terms.Numerator}/{terms.Denominator} is not");
        }

        return rational;
    }
}
