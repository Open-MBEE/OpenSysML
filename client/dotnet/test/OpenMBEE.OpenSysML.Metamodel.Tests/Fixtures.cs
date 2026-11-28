using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>Elements built the way a reader builds them: created by metaclass, then supplied.</summary>
internal static class Fixtures
{
    /// <summary>Every concrete metaclass.</summary>
    internal static IReadOnlyList<MetaclassInfo> Concrete { get; } = [.. Metaclasses.All.Where(metaclass => !metaclass.IsAbstract)];

    /// <summary>One element of each concrete metaclass, with nothing supplied, its <c>@id</c> the metaclass name.</summary>
    internal static IReadOnlyList<MetamodelElement> OneOfEach() =>
        [.. Concrete.Select(metaclass => metaclass.CreateElement(metaclass.Name, metaclass.Name))];

    /// <summary>A concrete metaclass conforming to <paramref name="range"/>.</summary>
    internal static MetaclassInfo ConcreteConformingTo(MetaclassInfo range) =>
        Concrete.First(metaclass => metaclass.ConformsTo(range));

    /// <summary>A value of one of <paramref name="property"/>'s values, distinct for each <paramref name="seed"/>.</summary>
    internal static object ValueOf(MetaProperty property, int seed) => property.RangeKind switch
    {
        PropertyRangeKind.Metaclass => ConcreteConformingTo(property.RangeMetaclass!).CreateElement($"target-{seed}", "target"),
        PropertyRangeKind.Enumeration => Enum.ToObject(property.ValueType, seed % property.RangeEnumeration!.Literals.Count),
        _ when property.ValueType == typeof(bool) => seed % 2 == 0,
        _ when property.ValueType == typeof(string) => $"text-{seed}",
        _ when property.ValueType == typeof(int) => seed,
        _ when property.ValueType == typeof(double) => seed + 0.5,
        _ => throw new InvalidOperationException($"{property} has an unexpected value type {property.ValueType}"),
    };

    /// <summary>A value to supply for <paramref name="property"/>: one value, or a one-value list of them.</summary>
    internal static object SuppliedValue(MetaProperty property, int seed)
    {
        var value = ValueOf(property, seed);
        if (!property.IsMany)
        {
            return value;
        }

        var list = Array.CreateInstance(property.ValueType, 1);
        list.SetValue(value, 0);
        return list;
    }
}
