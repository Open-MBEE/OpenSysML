using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.Linq;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// Every metaclass and enumeration of the SysML metamodel, and the metamodel release they were
/// generated from. A static property per metaclass, for example <c>Metaclasses.PartUsage</c>, names its
/// <see cref="MetaclassInfo"/>. Its metadata collections are immutable snapshots.
/// </summary>
public static partial class Metaclasses
{
    private static IReadOnlyList<MetaclassInfo>? all;
    private static IReadOnlyList<EnumerationInfo>? enumerations;
    private static FrozenDictionary<string, MetaclassInfo>? byName;
    private static FrozenDictionary<Type, MetaclassInfo>? byType;
    private static FrozenDictionary<string, EnumerationInfo>? enumerationsByName;
    private static FrozenDictionary<Type, EnumerationInfo>? enumerationsByType;

    /// <summary>Every metaclass, abstract ones included, in ordinal order of their names.</summary>
    public static IReadOnlyList<MetaclassInfo> All => all!;

    /// <summary>Every enumeration, in ordinal order of their names.</summary>
    public static IReadOnlyList<EnumerationInfo> Enumerations => enumerations!;

    /// <summary>Finds a metaclass by name, for example <c>PartUsage</c>.</summary>
    /// <param name="name">The metaclass name, matched exactly.</param>
    /// <param name="metaclass">The metaclass, when there is one.</param>
    /// <returns>Whether the metamodel has a metaclass of that name.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    public static bool TryGet(string name, [NotNullWhen(true)] out MetaclassInfo? metaclass)
    {
        ArgumentNullException.ThrowIfNull(name);
        return byName!.TryGetValue(name, out metaclass);
    }

    /// <summary>Finds the metaclass of a generated interface or element class.</summary>
    /// <param name="type">For example <c>typeof(IPartUsage)</c> or <c>typeof(Elements.PartUsage)</c>.</param>
    /// <param name="metaclass">The metaclass, when the type is one generated for a metaclass.</param>
    /// <returns>Whether it is.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="type"/> is null.</exception>
    public static bool TryGet(Type type, [NotNullWhen(true)] out MetaclassInfo? metaclass)
    {
        ArgumentNullException.ThrowIfNull(type);
        return byType!.TryGetValue(type, out metaclass);
    }

    /// <summary>The metaclass of a generated interface or element class.</summary>
    /// <typeparam name="T">For example <c>IPartUsage</c> or <c>Elements.PartUsage</c>.</typeparam>
    /// <returns>Its metaclass.</returns>
    /// <exception cref="ArgumentException"><typeparamref name="T"/> is not generated for a metaclass.</exception>
    public static MetaclassInfo Of<T>()
        where T : IMetamodelElement =>
        TryGet(typeof(T), out var metaclass)
            ? metaclass
            : throw new ArgumentException($"{typeof(T)} is not generated for a metaclass", nameof(T));

    /// <summary>Finds an enumeration by name, for example <c>VisibilityKind</c>.</summary>
    /// <param name="name">The enumeration name, matched exactly.</param>
    /// <param name="enumeration">The enumeration, when there is one.</param>
    /// <returns>Whether the metamodel has an enumeration of that name.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    public static bool TryGetEnumeration(string name, [NotNullWhen(true)] out EnumerationInfo? enumeration)
    {
        ArgumentNullException.ThrowIfNull(name);
        return enumerationsByName!.TryGetValue(name, out enumeration);
    }

    internal static bool TryGetEnumeration(Type enumType, [NotNullWhen(true)] out EnumerationInfo? enumeration) =>
        enumerationsByType!.TryGetValue(enumType, out enumeration);

    /// <summary>Indexes what the generated static constructor built, last.</summary>
    private static void Register(MetaclassInfo[] metaclasses, EnumerationInfo[] enumerationList)
    {
        all = Array.AsReadOnly<MetaclassInfo>([.. metaclasses.OrderBy(metaclass => metaclass.Name, StringComparer.Ordinal)]);
        enumerations = Array.AsReadOnly<EnumerationInfo>([.. enumerationList.OrderBy(enumeration => enumeration.Name, StringComparer.Ordinal)]);
        byName = metaclasses.ToFrozenDictionary(metaclass => metaclass.Name, StringComparer.Ordinal);
        byType = metaclasses
            .Select(metaclass => KeyValuePair.Create(metaclass.InterfaceType, metaclass))
            .Concat(metaclasses
                .Where(metaclass => metaclass.ElementType is not null)
                .Select(metaclass => KeyValuePair.Create(metaclass.ElementType!, metaclass)))
            .ToFrozenDictionary();
        enumerationsByName = enumerationList.ToFrozenDictionary(enumeration => enumeration.Name, StringComparer.Ordinal);
        enumerationsByType = enumerationList.ToFrozenDictionary(enumeration => enumeration.EnumType);
    }
}
