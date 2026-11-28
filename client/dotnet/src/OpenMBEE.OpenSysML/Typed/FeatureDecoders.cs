using System.Numerics;

namespace OpenMBEE.OpenSysML.Typed;

/// <summary>
/// The decoders generated properties read library scalars and other values with, passed to
/// <see cref="TypedFeatures"/> as <see cref="FeatureDecoder{T}"/>. Each accepts the value arms that hold its type
/// exactly, or widen to it without loss of meaning, and reports any other value with <see cref="TypeMismatchException"/>.
/// </summary>
public static class FeatureDecoders
{
    /// <summary>A <c>Boolean</c>, as a <see cref="bool"/>.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static bool AsBoolean(string featureName, Value value, Instantiation? instantiation) =>
        value is Value.BooleanValue boolean ? boolean.Value : throw new TypeMismatchException(featureName, "bool", value);

    /// <summary>A <c>String</c>, as a <see cref="string"/>.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static string AsString(string featureName, Value value, Instantiation? instantiation) =>
        value is Value.TextValue text ? text.Value : throw new TypeMismatchException(featureName, "string", value);

    /// <summary>An <c>Integer</c> or <c>Natural</c>, exactly, as a <see cref="BigInteger"/> whatever its size.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static BigInteger AsInteger(string featureName, Value value, Instantiation? instantiation) => value switch
    {
        Value.IntegerValue integer => integer.Value,
        Value.BigIntegerValue big => big.Value,
        _ => throw new TypeMismatchException(featureName, "BigInteger", value),
    };

    /// <summary>A <c>Rational</c>, exactly: an integer is itself over one, and a finite real the rational it is exactly.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static Rational AsRational(string featureName, Value value, Instantiation? instantiation) => value switch
    {
        Value.RationalValue rational => rational.Value,
        Value.IntegerValue integer => new Rational(integer.Value, BigInteger.One),
        Value.BigIntegerValue big => new Rational(big.Value, BigInteger.One),
        Value.RealValue real when double.IsFinite(real.Value) => Rational.FromExactDouble(real.Value),
        _ => throw new TypeMismatchException(featureName, "Rational", value),
    };

    /// <summary>A <c>Real</c>, as a <see cref="double"/>: an integer or exact rational widens to the nearest double.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static double AsReal(string featureName, Value value, Instantiation? instantiation) => value switch
    {
        Value.RealValue real => real.Value,
        Value.IntegerValue integer => integer.Value,
        Value.BigIntegerValue big => (double)big.Value,
        Value.RationalValue rational => rational.Value.ToDouble(),
        _ => throw new TypeMismatchException(featureName, "double", value),
    };

    /// <summary>A <c>Complex</c>: a real number widens to one with no imaginary part.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static Complex AsComplex(string featureName, Value value, Instantiation? instantiation) => value switch
    {
        Value.ComplexValue complex => new Complex(complex.Real, complex.Imaginary),
        Value.RealValue real => new Complex(real.Value, 0),
        Value.IntegerValue integer => new Complex(integer.Value, 0),
        Value.BigIntegerValue big => new Complex((double)big.Value, 0),
        Value.RationalValue rational => new Complex(rational.Value.ToDouble(), 0),
        _ => throw new TypeMismatchException(featureName, "Complex", value),
    };

    /// <summary>A value written with a unit: its magnitude and the unit it is expressed in.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static Quantity AsQuantity(string featureName, Value value, Instantiation? instantiation) =>
        value is Value.QuantityValue quantity ? quantity.Quantity : throw new TypeMismatchException(featureName, "Quantity", value);

    /// <summary>A value of an enumeration: one of its literals, which is a value rather than an instance.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    /// <exception cref="TypeMismatchException">The value is of another type.</exception>
    public static Value.EnumLiteralValue AsEnumLiteral(string featureName, Value value, Instantiation? instantiation) =>
        value as Value.EnumLiteralValue ?? throw new TypeMismatchException(featureName, "an enumeration literal", value);

    /// <summary>Any value, as it arrived, for a feature whose type has no sound C# counterpart.</summary>
    /// <param name="featureName">The feature's name, which a failure names.</param>
    /// <param name="value">One value the feature holds.</param>
    /// <param name="instantiation">Unused: the value is not an object.</param>
    /// <returns>The decoded value.</returns>
    public static Value AsValue(string featureName, Value value, Instantiation? instantiation) => value;
}
