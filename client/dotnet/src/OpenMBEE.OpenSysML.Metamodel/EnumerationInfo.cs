using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.Linq;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// One enumeration of the SysML metamodel and the generated C# enum for it. The enum's members are its
/// literals in declaration order, PascalCased (<c>inout</c> is <c>FeatureDirectionKind.Inout</c>), and
/// each member's underlying value is its literal's position.
/// </summary>
public sealed class EnumerationInfo
{
    private readonly Enum[] values;
    private readonly FrozenDictionary<string, Enum> byLiteral;

    internal EnumerationInfo(string name, Type enumType, string[] literals)
    {
        if (!enumType.IsEnum || Enum.GetValues(enumType).Length != literals.Length)
        {
            throw new ArgumentException($"{name}: {enumType} is not an enum of {literals.Length} members", nameof(enumType));
        }

        Name = name;
        EnumType = enumType;
        Literals = Array.AsReadOnly<string>([.. literals]);
        values = [.. Literals.Select((_, index) => (Enum)Enum.ToObject(enumType, index))];
        byLiteral = Literals.Select((literal, index) => KeyValuePair.Create(literal, values[index])).ToFrozenDictionary(StringComparer.Ordinal);
    }

    /// <summary>The enumeration name, for example <c>FeatureDirectionKind</c>.</summary>
    public string Name { get; }

    /// <summary>The generated C# enum.</summary>
    public Type EnumType { get; }

    /// <summary>The literals a document writes, in declaration order, for example <c>in</c>, <c>inout</c>, <c>out</c>.</summary>
    public IReadOnlyList<string> Literals { get; }

    /// <summary>The enumeration of a generated enum.</summary>
    /// <typeparam name="TEnum">A generated metamodel enum, for example <c>FeatureDirectionKind</c>.</typeparam>
    /// <returns>Its enumeration.</returns>
    /// <exception cref="ArgumentException"><typeparamref name="TEnum"/> is not a generated metamodel enum.</exception>
    public static EnumerationInfo Of<TEnum>()
        where TEnum : struct, Enum =>
        Metaclasses.TryGetEnumeration(typeof(TEnum), out var enumeration)
            ? enumeration
            : throw new ArgumentException($"{typeof(TEnum)} is not a metamodel enumeration", nameof(TEnum));

    /// <summary>Reads a literal as the generated enum's member.</summary>
    /// <param name="literal">A literal as a document writes it, for example <c>inout</c>; matched exactly.</param>
    /// <param name="value">The member, when the literal is one of the enumeration's.</param>
    /// <returns>Whether it is.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="literal"/> is null.</exception>
    public bool TryParse(string literal, [NotNullWhen(true)] out Enum? value)
    {
        ArgumentNullException.ThrowIfNull(literal);
        return byLiteral.TryGetValue(literal, out value);
    }

    /// <summary>The literal a document writes for a member of the generated enum.</summary>
    /// <param name="value">A member of <see cref="EnumType"/>.</param>
    /// <returns>Its literal, for example <c>inout</c>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="value"/> is not a member of <see cref="EnumType"/>.</exception>
    public string GetLiteral(Enum value)
    {
        ArgumentNullException.ThrowIfNull(value);
        var index = Array.IndexOf(values, value);
        return index >= 0
            ? Literals[index]
            : throw new ArgumentException($"{value} is not a member of {EnumType.Name}", nameof(value));
    }

    /// <summary>The enumeration name.</summary>
    /// <returns><see cref="Name"/>.</returns>
    public override string ToString() => Name;
}
