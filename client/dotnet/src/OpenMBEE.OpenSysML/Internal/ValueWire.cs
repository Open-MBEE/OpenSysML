using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Numerics;
using Google.Protobuf;
using Google.Protobuf.Collections;
using Kind = OpenMBEE.OpenSysML.Wire.Value.KindOneofCase;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Converts between <see cref="Value"/> and the service's <c>Value</c> message.
/// </summary>
/// <remarks>
/// <para>Reading checks what the schema makes canonical and never repairs it: a malformed answer is a
/// <see cref="TransportException"/> with status <see cref="StatusCode.Internal"/>, and a value of no
/// kind this client knows is an <see cref="UnsupportedValueException"/>, never a null. Every arm is
/// read whatever the service advertises.</para>
/// <para>Writing is two steps, so a request can order its checks: <see cref="ToWire(Value, string)"/>
/// refuses what no service accepts as an argument, as <see cref="InvalidRequestException"/>, and
/// <see cref="Fit"/> then holds the message to what one service advertises, applying the service's
/// own predicates in the service's own order.</para>
/// </remarks>
internal static class ValueWire
{
    /// <summary>The deepest nesting of values read or written; the protobuf runtime reads no deeper message anyway.</summary>
    internal const int MaxDepth = 100;

    /// <summary>The capabilities a sent value can need, in the order the service checks them.</summary>
    internal static IReadOnlyList<string> SendGateOrder { get; } =
    [
        Capabilities.ComplexValues,
        Capabilities.StructuredValues,
        Capabilities.MeasurementRefs,
        Capabilities.FunctionValues,
        Capabilities.InfinityValue,
        Capabilities.SetValues,
        Capabilities.TensorValues,
        Capabilities.MetaobjectValues,
        Capabilities.BigIntValues,
        Capabilities.RationalValues,
    ];

    /// <summary>Reads a value the service answered.</summary>
    /// <param name="value">The message.</param>
    /// <param name="method">The method answered, for the error.</param>
    /// <param name="path">Where the value is in the response, as <c>result</c>.</param>
    /// <returns>The value.</returns>
    /// <exception cref="TransportException">The value is malformed.</exception>
    /// <exception cref="UnsupportedValueException">The value, or one nested in it, is of no kind this client knows.</exception>
    internal static Value FromWire(Wire.Value value, string method, string path)
    {
        ArgumentNullException.ThrowIfNull(value);
        return Read(value, method, path, 0);
    }

    /// <summary>Reads the values of a repeated field, in order.</summary>
    internal static ValueList<Value> FromWire(IReadOnlyList<Wire.Value> values, string method, string path) =>
        new(values.Select((value, i) => Read(value, method, $"{path}[{i}]", 0)));

    /// <summary>Reads the values of a map field.</summary>
    internal static ValueDictionary<Value> FromWire(IReadOnlyDictionary<string, Wire.Value> values, string method, string path) =>
        new(values.Select(entry => KeyValuePair.Create(entry.Key, Read(entry.Value, method, $"{path}[{ValueText.Quoted(entry.Key)}]", 0))));

    /// <summary>Reads a quantity another message carries by the rules a value's quantity is read by.</summary>
    /// <exception cref="TransportException">The quantity is malformed.</exception>
    internal static Quantity FromWire(Wire.Quantity quantity, string method, string path)
    {
        ArgumentNullException.ThrowIfNull(quantity);
        try
        {
            return QuantityOf(quantity, method, path);
        }
        catch (ArgumentException invalid)
        {
            throw Malformed(method, path, Reason(invalid), invalid);
        }
    }

    /// <summary>Reads an Integer beyond int64 another message carries, as canonical decimal digits.</summary>
    /// <exception cref="TransportException">The digits are not canonical, or the integer is within int64.</exception>
    internal static BigInteger BigIntegerFromWire(string digits, string method, string path) => BigIntegerOf(digits, method, path);

    /// <summary>Reads an exact Rational another message carries, as canonical terms no double holds.</summary>
    /// <exception cref="TransportException">The terms are not canonical, not in lowest terms, or a double.</exception>
    internal static Rational RationalFromWire(Wire.Rational terms, string method, string path) => RationalOf(terms, method, path);

    /// <summary>The failure of reading a malformed answer.</summary>
    internal static TransportException MalformedAnswer(string method, string path, string reason) => Malformed(method, path, reason);

    /// <summary>Writes a value for a request, refusing one no service reads as an argument.</summary>
    /// <param name="value">The value.</param>
    /// <param name="path">Where the value is in the request, as <c>named_arguments["x"]</c>, for the error.</param>
    /// <returns>A message of the caller's own.</returns>
    /// <exception cref="InvalidRequestException">The value, or one nested in it, cannot be sent.</exception>
    internal static Wire.Value ToWire(Value value, string path)
    {
        ArgumentNullException.ThrowIfNull(value);
        return Write(value, path, 0);
    }

    /// <summary>Writes a quantity another message carries by the rules a value's quantity is written by.</summary>
    /// <exception cref="InvalidRequestException">The quantity cannot be sent.</exception>
    internal static Wire.Quantity ToWire(Quantity quantity, string path)
    {
        ArgumentNullException.ThrowIfNull(quantity);
        return WriteQuantity(quantity, path);
    }

    /// <summary>Writes the values of a repeated field, in order.</summary>
    internal static List<Wire.Value> ToWire(ValueList<Value> values, string path) =>
        values.Select((value, i) => Write(value, $"{path}[{i}]", 0)).ToList();

    /// <summary>Writes the values of a map field, in ordinal order of their names.</summary>
    internal static List<KeyValuePair<string, Wire.Value>> ToWire(ValueDictionary<Value> values, string path) =>
        values.Select(entry => KeyValuePair.Create(entry.Key, Write(entry.Value, $"{path}[{ValueText.Quoted(entry.Key)}]", 0))).ToList();

    /// <summary>
    /// Holds a written value to what a service advertises: first, to a service without
    /// <c>rational_values</c>, each Rational a <see cref="double"/> holds exactly becomes that
    /// double, which is the same number; then each capability the value still needs is required,
    /// in the order the service checks them.
    /// </summary>
    /// <param name="sent">A message <see cref="ToWire(Value, string)"/> wrote, rewritten in place.</param>
    /// <param name="capabilities">What the service advertises.</param>
    /// <exception cref="CapabilityException">The service does not advertise a capability the value needs.</exception>
    internal static void Fit(Wire.Value sent, Capabilities capabilities)
    {
        ArgumentNullException.ThrowIfNull(sent);
        ArgumentNullException.ThrowIfNull(capabilities);
        if (!capabilities.Has(Capabilities.RationalValues))
        {
            RationalsAsReals(sent);
        }

        foreach (var capability in RequiredCapabilities(sent))
        {
            capabilities.Require(capability);
        }
    }

    /// <summary>
    /// The capabilities a service requires of a value it is sent, in the order it checks them, by its
    /// own predicates: a kind counts wherever a sequence, set or array element or a vector component
    /// holds it, and an Integer beyond <see cref="long"/> or a Rational also as a quantity's magnitude
    /// and inside an enumeration literal's scalar.
    /// </summary>
    internal static IReadOnlyList<string> RequiredCapabilities(Wire.Value value)
    {
        var needed = new HashSet<string>(StringComparer.Ordinal);
        foreach (var each in Closure(value, throughLiterals: false))
        {
            if (KindCapability(each.KindCase) is { } capability)
            {
                needed.Add(capability);
            }
        }

        foreach (var each in Closure(value, throughLiterals: true))
        {
            if (Holds(each, Kind.BigIntValue, Wire.Quantity.MagnitudeOneofCase.BigIntMagnitude))
            {
                needed.Add(Capabilities.BigIntValues);
            }

            if (Holds(each, Kind.RationalValue, Wire.Quantity.MagnitudeOneofCase.RationalMagnitude))
            {
                needed.Add(Capabilities.RationalValues);
            }
        }

        return SendGateOrder.Where(needed.Contains).ToList();
    }

    private static string? KindCapability(Kind kind) => kind switch
    {
        Kind.Complex => Capabilities.ComplexValues,
        Kind.Array or Kind.Vector or Kind.VectorQuantity => Capabilities.StructuredValues,
        Kind.MeasurementRef => Capabilities.MeasurementRefs,
        Kind.Function => Capabilities.FunctionValues,
        Kind.Infinity => Capabilities.InfinityValue,
        Kind.Set => Capabilities.SetValues,
        Kind.TensorQuantity => Capabilities.TensorValues,
        Kind.Metaobject => Capabilities.MetaobjectValues,
        _ => null,
    };

    // Whether a value is the arm itself, or a quantity whose magnitude is: its own, or a component's.
    private static bool Holds(Wire.Value value, Kind arm, Wire.Quantity.MagnitudeOneofCase magnitude) => value.KindCase switch
    {
        Kind.Quantity => value.Quantity.MagnitudeCase == magnitude,
        Kind.VectorQuantity => value.VectorQuantity.Components.Any(component => component.MagnitudeCase == magnitude),
        Kind.TensorQuantity => value.TensorQuantity.Components.Any(component => component.MagnitudeCase == magnitude),
        var kind => kind == arm,
    };

    // The value and every value nested in it, found without recursion so no depth exhausts the stack.
    private static List<Wire.Value> Closure(Wire.Value root, bool throughLiterals)
    {
        var found = new List<Wire.Value>();
        var pending = new Stack<Wire.Value>();
        pending.Push(root);
        while (pending.TryPop(out var value))
        {
            found.Add(value);
            foreach (var nested in Nested(value))
            {
                pending.Push(nested);
            }

            if (throughLiterals && value.KindCase == Kind.EnumLiteral && value.EnumLiteral.Value is { } scalar)
            {
                pending.Push(scalar);
            }
        }

        return found;
    }

    // The values a value holds directly, as the service lists them: a literal's scalar is not one.
    private static RepeatedField<Wire.Value> Nested(Wire.Value value) => value.KindCase switch
    {
        Kind.Sequence => value.Sequence.Elements,
        Kind.Set => value.Set.Elements,
        Kind.Array => value.Array.Elements,
        Kind.Vector => value.Vector.Components,
        _ => [],
    };

    private static void RationalsAsReals(Wire.Value root)
    {
        foreach (var value in Closure(root, throughLiterals: true))
        {
            switch (value.KindCase)
            {
                case Kind.RationalValue when ExactDouble(value.RationalValue) is { } real:
                    value.RealValue = real;
                    break;
                case Kind.Quantity:
                    RationalAsReal(value.Quantity);
                    break;
                case Kind.VectorQuantity:
                    value.VectorQuantity.Components.ToList().ForEach(RationalAsReal);
                    break;
                case Kind.TensorQuantity:
                    value.TensorQuantity.Components.ToList().ForEach(RationalAsReal);
                    break;
            }
        }
    }

    /// <summary>Rewrites a quantity's Rational magnitude a <see cref="double"/> holds exactly as that double, the same number.</summary>
    internal static void RationalAsReal(Wire.Quantity quantity)
    {
        if (quantity.MagnitudeCase == Wire.Quantity.MagnitudeOneofCase.RationalMagnitude && ExactDouble(quantity.RationalMagnitude) is { } real)
        {
            quantity.RealMagnitude = real;
        }
    }

    /// <summary>The double a Rational this client wrote is exactly, if any.</summary>
    internal static double? ExactDouble(Wire.Rational terms) =>
        new Rational(Integer(terms.Numerator), Integer(terms.Denominator)).TryGetExactDouble(out var real) ? real : null;

    private static BigInteger Integer(string digits) =>
        BigInteger.Parse(digits, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture);

    private static Value Read(Wire.Value value, string method, string path, int depth)
    {
        if (depth > MaxDepth)
        {
            throw Malformed(method, path, $"it nests deeper than {MaxDepth} values");
        }

        try
        {
            return value.KindCase switch
            {
                Kind.IntValue => new Value.IntegerValue(value.IntValue),
                Kind.BigIntValue => new Value.BigIntegerValue(BigIntegerOf(value.BigIntValue, method, path)),
                Kind.RationalValue => new Value.RationalValue(RationalOf(value.RationalValue, method, path)),
                Kind.RealValue => new Value.RealValue(value.RealValue),
                Kind.Complex => new Value.ComplexValue(value.Complex.Real, value.Complex.Imaginary),
                Kind.BoolValue => new Value.BooleanValue(value.BoolValue),
                Kind.StringValue => new Value.TextValue(value.StringValue),
                Kind.InstanceId => new Value.InstanceRefValue(value.InstanceId),
                Kind.EnumLiteral => new Value.EnumLiteralValue(
                    value.EnumLiteral.LiteralId,
                    value.EnumLiteral.EnumerationId,
                    value.EnumLiteral.Name,
                    value.EnumLiteral.Value is { } scalar ? Read(scalar, method, path + ".enum_literal.value", depth + 1) : null),
                Kind.Sequence => new Value.SequenceValue(ReadAll(value.Sequence.Elements, method, path + ".sequence", depth)),
                Kind.Set => new Value.SetValue(ReadAll(value.Set.Elements, method, path + ".set", depth)),
                Kind.Quantity => new Value.QuantityValue(QuantityOf(value.Quantity, method, path + ".quantity")),
                Kind.Array => new Value.ArrayValue(value.Array.Dimensions, ReadAll(value.Array.Elements, method, path + ".array", depth)),
                Kind.Vector => new Value.VectorValue(value.Vector.Components.Select((component, i) => NumberOf(component, method, $"{path}.vector[{i}]"))),
                Kind.VectorQuantity => new Value.VectorQuantityValue(
                    value.VectorQuantity.Components.Select((component, i) => QuantityOf(component, method, $"{path}.vector_quantity[{i}]")).ToList()),
                Kind.TensorQuantity => new Value.TensorQuantityValue(
                    value.TensorQuantity.Dimensions,
                    value.TensorQuantity.Components.Select((component, i) => QuantityOf(component, method, $"{path}.tensor_quantity[{i}]")).ToList()),
                Kind.MeasurementRef => new Value.MeasurementRefValue(
                    value.MeasurementRef.Unit,
                    value.MeasurementRef.UnitTerm is { } reduction
                        ? TermOf(reduction)
                        : throw Malformed(method, path, $"the measurement reference [{value.MeasurementRef.Unit}] has no reduction to base units"),
                    value.MeasurementRef.UnitId.Length > 0 ? value.MeasurementRef.UnitId : null),
                Kind.Function => new Value.FunctionValue(value.Function.CalcId, value.Function.SelfId == 0 ? null : value.Function.SelfId),
                Kind.Metaobject => new Value.MetaobjectValue(value.Metaobject.ElementId, value.Metaobject.MetaclassId),
                Kind.Null => new Value.NullValue(value.Null),
                Kind.Unset => value.Unset ? new Value.UnsetValue() : throw Malformed(method, path, "the unset arm states no value unless it is true"),
                Kind.Undetermined => new Value.UndeterminedValue(
                    value.Undetermined.Reason,
                    value.Undetermined.Count?.Lower ?? string.Empty,
                    value.Undetermined.Count?.Upper ?? string.Empty),
                Kind.Infinity => value.Infinity ? new Value.InfinityValue() : throw Malformed(method, path, "the infinity arm states no value unless it is true"),
                _ => throw Unsupported(value, method, path),
            };
        }
        catch (ArgumentException invalid)
        {
            throw Malformed(method, path, Reason(invalid), invalid);
        }
    }

    private static List<Value> ReadAll(IEnumerable<Wire.Value> values, string method, string path, int depth) =>
        values.Select((value, i) => Read(value, method, $"{path}[{i}]", depth + 1)).ToList();

    private static Value.NumberValue NumberOf(Wire.Value value, string method, string path) => value.KindCase switch
    {
        Kind.IntValue => new Value.IntegerValue(value.IntValue),
        Kind.BigIntValue => new Value.BigIntegerValue(BigIntegerOf(value.BigIntValue, method, path)),
        Kind.RationalValue => new Value.RationalValue(RationalOf(value.RationalValue, method, path)),
        Kind.RealValue => new Value.RealValue(value.RealValue),
        var kind => throw Malformed(method, path, $"a vector component is {kind}, not a number"),
    };

    private static Quantity QuantityOf(Wire.Quantity quantity, string method, string path)
    {
        Value.NumberValue magnitude = quantity.MagnitudeCase switch
        {
            Wire.Quantity.MagnitudeOneofCase.IntMagnitude => new Value.IntegerValue(quantity.IntMagnitude),
            Wire.Quantity.MagnitudeOneofCase.BigIntMagnitude => new Value.BigIntegerValue(BigIntegerOf(quantity.BigIntMagnitude, method, path)),
            Wire.Quantity.MagnitudeOneofCase.RationalMagnitude => new Value.RationalValue(RationalOf(quantity.RationalMagnitude, method, path)),
            Wire.Quantity.MagnitudeOneofCase.RealMagnitude => new Value.RealValue(quantity.RealMagnitude),
            _ => throw Malformed(method, path, $"the quantity in [{quantity.Unit}] has no magnitude"),
        };
        return new Quantity(magnitude, quantity.Unit, quantity.UnitTerm is { } reduction ? TermOf(reduction) : null);
    }

    private static UnitTerm TermOf(Wire.UnitTerm term) =>
        new(term.ScaleNum, term.ScaleDen, term.Factors.Select(factor => new UnitFactor(factor.UnitId, factor.Exponent)));

    // An Integer beyond int64 as the schema spells one: canonical decimal digits, never one within int64.
    private static BigInteger BigIntegerOf(string digits, string method, string path)
    {
        if (!Canonical(digits, signed: true) || digits == "0")
        {
            throw Malformed(method, path, $"the integer {ValueText.Quoted(digits)} is not canonical decimal digits");
        }

        var integer = Integer(digits);
        if (integer >= long.MinValue && integer <= long.MaxValue)
        {
            throw Malformed(method, path, $"the integer {digits} is within int64, which int_value carries");
        }

        return integer;
    }

    // An exact Rational as the schema spells one: canonical terms in lowest terms over a positive
    // denominator, and no number a double holds, which real_value carries.
    private static Rational RationalOf(Wire.Rational terms, string method, string path)
    {
        var spelled = $"{ValueText.Quoted(terms.Numerator)}/{ValueText.Quoted(terms.Denominator)}";
        if (!Canonical(terms.Numerator, signed: true) || !Canonical(terms.Denominator, signed: false) || terms.Denominator == "0")
        {
            throw Malformed(method, path, $"the rational {spelled} is not canonical decimal terms over a positive denominator");
        }

        var (numerator, denominator) = (Integer(terms.Numerator), Integer(terms.Denominator));
        var rational = new Rational(numerator, denominator);
        if (rational.Numerator != numerator || rational.Denominator != denominator)
        {
            throw Malformed(method, path, $"the rational {terms.Numerator}/{terms.Denominator} is not in lowest terms");
        }

        if (rational.TryGetExactDouble(out _))
        {
            throw Malformed(method, path, $"the rational {rational} is a double, which real_value carries");
        }

        return rational;
    }

    // Decimal digits with no leading zero and no '+', a '-' only where signed; zero is "0", never "-0".
    private static bool Canonical(string digits, bool signed)
    {
        if (digits == "0")
        {
            return true;
        }

        var magnitude = signed && digits.StartsWith('-') ? digits.AsSpan(1) : digits.AsSpan();
        return magnitude.Length > 0 && magnitude[0] != '0' && !magnitude.ContainsAnyExceptInRange('0', '9');
    }

    private static Wire.Value Write(Value value, string path, int depth)
    {
        if (depth > MaxDepth)
        {
            throw Refused(path, $"nests deeper than {MaxDepth} values");
        }

        return value switch
        {
            Value.IntegerValue integer => new Wire.Value { IntValue = integer.Value },
            Value.BigIntegerValue integer => new Wire.Value { BigIntValue = Digits(integer.Value) },
            Value.RationalValue rational => new Wire.Value { RationalValue = Terms(rational.Value) },
            Value.RealValue real => new Wire.Value { RealValue = real.Value },
            Value.ComplexValue complex => new Wire.Value { Complex = new Wire.Complex { Real = complex.Real, Imaginary = complex.Imaginary } },
            Value.BooleanValue boolean => new Wire.Value { BoolValue = boolean.Value },
            Value.TextValue text => new Wire.Value { StringValue = text.Value },
            Value.InstanceRefValue reference => new Wire.Value { InstanceId = reference.InstanceId },
            Value.EnumLiteralValue literal => new Wire.Value
            {
                EnumLiteral = new Wire.EnumLiteral
                {
                    LiteralId = literal.LiteralId,
                    EnumerationId = literal.EnumerationId,
                    Name = literal.Name,
                    Value = literal.Scalar is { } scalar ? Write(scalar, path + ".enum_literal.value", depth + 1) : null,
                },
            },
            Value.SequenceValue sequence => new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { WriteAll(sequence.Elements, path + ".sequence", depth) } } },
            Value.SetValue set => new Wire.Value { Set = new Wire.ValueSet { Elements = { WriteAll(set.Elements, path + ".set", depth) } } },
            Value.QuantityValue quantity => new Wire.Value { Quantity = WriteQuantity(quantity.Quantity, path + ".quantity") },
            Value.ArrayValue array => new Wire.Value
            {
                Array = new Wire.Array { Dimensions = { array.Dimensions }, Elements = { WriteAll(array.Elements, path + ".array", depth) } },
            },
            Value.VectorValue vector => new Wire.Value
            {
                Vector = new Wire.Vector { Components = { vector.Components.Select((component, i) => Write(component, $"{path}.vector[{i}]", depth + 1)) } },
            },
            Value.VectorQuantityValue vector => new Wire.Value
            {
                VectorQuantity = new Wire.VectorQuantity
                {
                    Components = { vector.Components.Select((component, i) => WriteQuantity(component, $"{path}.vector_quantity[{i}]")) },
                },
            },
            Value.TensorQuantityValue tensor => new Wire.Value
            {
                TensorQuantity = new Wire.TensorQuantity
                {
                    Dimensions = { tensor.Dimensions },
                    Components = { tensor.Components.Select((component, i) => WriteQuantity(component, $"{path}.tensor_quantity[{i}]")) },
                },
            },
            Value.MeasurementRefValue reference => new Wire.Value
            {
                MeasurementRef = new Wire.MeasurementRef
                {
                    Unit = reference.Unit,
                    UnitTerm = WriteTerm(reference.Reduction, path),
                    UnitId = reference.UnitId ?? string.Empty,
                },
            },
            Value.FunctionValue { SelfId: { } self } function =>
                throw Refused(path, $"is the function {function.CalcId} closing over instance({ValueText.Whole(self)}), which lives only within the response that held it"),
            Value.FunctionValue function => new Wire.Value { Function = new Wire.Function { CalcId = function.CalcId } },
            Value.MetaobjectValue metaobject => new Wire.Value
            {
                Metaobject = new Wire.Metaobject { ElementId = metaobject.ElementId, MetaclassId = metaobject.MetaclassId },
            },
            Value.NullValue { IsUnsupported: true } nothing =>
                throw Refused(path, $"stands for a value the service could not send ({nothing.Unsupported}), which it would read back as null"),
            Value.NullValue => new Wire.Value { Null = string.Empty },
            Value.UnsetValue =>
                throw Refused(path, "is unset, which is what a feature with no value reads as, never an argument; leave the argument out instead"),
            Value.UndeterminedValue =>
                throw Refused(path, "is undetermined, which is a result the model leaves open, never an argument"),
            Value.InfinityValue => new Wire.Value { Infinity = true },
            _ => throw new InvalidOperationException($"No wire form is written for a {value.Kind} value."),
        };
    }

    private static IEnumerable<Wire.Value> WriteAll(ValueList<Value> values, string path, int depth) =>
        values.Select((value, i) => Write(value, $"{path}[{i}]", depth + 1));

    private static Wire.Quantity WriteQuantity(Quantity quantity, string path)
    {
        if (quantity.Reduction is null && quantity.Unit.Length > 0)
        {
            throw Refused(path, $"is in [{quantity.Unit}] with no reduction to base units, so the service cannot tell what it measures; give its reduction, or use a quantity the service answered");
        }

        var wire = new Wire.Quantity
        {
            Unit = quantity.Unit,
            UnitTerm = quantity.Reduction is { } reduction ? WriteTerm(reduction, path) : null,
        };
        switch (quantity.Magnitude)
        {
            case Value.IntegerValue integer:
                wire.IntMagnitude = integer.Value;
                break;
            case Value.BigIntegerValue integer:
                wire.BigIntMagnitude = Digits(integer.Value);
                break;
            case Value.RationalValue rational:
                wire.RationalMagnitude = Terms(rational.Value);
                break;
            case Value.RealValue real:
                wire.RealMagnitude = real.Value;
                break;
            default:
                throw new InvalidOperationException($"No wire form is written for a {quantity.Magnitude.Kind} magnitude.");
        }

        return wire;
    }

    private static Wire.UnitTerm WriteTerm(UnitTerm term, string path)
    {
        var (numerator, denominator) = (term.ScaleNumerator, term.ScaleDenominator);
        if (!double.IsFinite(numerator) || !double.IsFinite(denominator) || numerator == 0 || denominator == 0)
        {
            throw Refused(path, $"has a unit scale of {ValueText.Plain(numerator)}/{ValueText.Plain(denominator)}, which reduces to no base unit");
        }

        if (term.Factors.Any(factor => factor.UnitId.Length == 0))
        {
            throw Refused(path, "has a unit factor naming no base unit");
        }

        return new Wire.UnitTerm
        {
            ScaleNum = numerator,
            ScaleDen = denominator,
            Factors = { term.Factors.Select(factor => new Wire.UnitFactor { UnitId = factor.UnitId, Exponent = factor.Exponent }) },
        };
    }

    private static string Digits(BigInteger integer) => integer.ToString(CultureInfo.InvariantCulture);

    private static Wire.Rational Terms(Rational rational) =>
        new() { Numerator = Digits(rational.Numerator), Denominator = Digits(rational.Denominator) };

    private static TransportException Malformed(string method, string path, string reason, Exception? cause = null) =>
        new(StatusCode.Internal, $"The service answered a malformed value at {path}: {reason}.", method, cause);

    private static UnsupportedValueException Unsupported(Wire.Value value, string method, string path)
    {
        var bytes = value.ToByteArray();
        var what = bytes.Length == 0
            ? "names no kind of value"
            : "is of a kind this client does not know; a newer client may read it";
        return new UnsupportedValueException($"The service answered a value at {path} that {what}.", method, path, bytes);
    }

    private static InvalidRequestException Refused(string path, string reason) => new($"The value at {path} {reason}.");

    // A domain refusal's message without the parameter name .NET appends to it.
    private static string Reason(ArgumentException invalid) =>
        (invalid.ParamName is { } name ? invalid.Message.Replace($" (Parameter '{name}')", string.Empty, StringComparison.Ordinal) : invalid.Message)
            .TrimEnd('.');
}
