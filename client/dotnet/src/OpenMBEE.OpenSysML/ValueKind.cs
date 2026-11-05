using System.Diagnostics.CodeAnalysis;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Which kind of <see cref="Value"/> a value is: one member per arm of the service's <c>Value</c>.
/// It is a discriminator to switch on, not a guarantee that a switch is exhaustive: a client
/// release that reads a new arm adds a member.
/// </summary>
public enum ValueKind
{
    /// <summary>An Integer within <see cref="long"/>: <see cref="Value.IntegerValue"/>.</summary>
    [SuppressMessage("Naming", "CA1720:Identifier contains type name", Justification = "Integer is the KerML type this kind names.")]
    Integer,

    /// <summary>An Integer beyond <see cref="long"/>: <see cref="Value.BigIntegerValue"/>.</summary>
    BigInteger,

    /// <summary>A Real: <see cref="Value.RealValue"/>.</summary>
    Real,

    /// <summary>An exact Rational: <see cref="Value.RationalValue"/>.</summary>
    Rational,

    /// <summary>A complex number: <see cref="Value.ComplexValue"/>.</summary>
    Complex,

    /// <summary>A Boolean: <see cref="Value.BooleanValue"/>.</summary>
    Boolean,

    /// <summary>A String: <see cref="Value.TextValue"/>.</summary>
    Text,

    /// <summary>A reference to an instance of the response: <see cref="Value.InstanceRefValue"/>.</summary>
    InstanceRef,

    /// <summary>A literal of an enumeration: <see cref="Value.EnumLiteralValue"/>.</summary>
    EnumLiteral,

    /// <summary>An ordered sequence: <see cref="Value.SequenceValue"/>.</summary>
    Sequence,

    /// <summary>A unique, unordered collection: <see cref="Value.SetValue"/>.</summary>
    Set,

    /// <summary>A magnitude in a unit: <see cref="Value.QuantityValue"/>.</summary>
    Quantity,

    /// <summary>A multidimensional array: <see cref="Value.ArrayValue"/>.</summary>
    Array,

    /// <summary>A numerical vector: <see cref="Value.VectorValue"/>.</summary>
    Vector,

    /// <summary>A vector of quantities: <see cref="Value.VectorQuantityValue"/>.</summary>
    VectorQuantity,

    /// <summary>A tensor of quantities: <see cref="Value.TensorQuantityValue"/>.</summary>
    TensorQuantity,

    /// <summary>A unit by itself: <see cref="Value.MeasurementRefValue"/>.</summary>
    MeasurementRef,

    /// <summary>A calc held as a value: <see cref="Value.FunctionValue"/>.</summary>
    Function,

    /// <summary>An element reflected on as its metaclass: <see cref="Value.MetaobjectValue"/>.</summary>
    Metaobject,

    /// <summary>The null value, or one the service could not send: <see cref="Value.NullValue"/>.</summary>
    Null,

    /// <summary>A materialized feature holding no value: <see cref="Value.UnsetValue"/>.</summary>
    Unset,

    /// <summary>A result the model leaves open: <see cref="Value.UndeterminedValue"/>.</summary>
    Undetermined,

    /// <summary>The unbounded value <c>*</c>: <see cref="Value.InfinityValue"/>.</summary>
    Infinity,
}
