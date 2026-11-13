using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Reflection;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>Immutable lists of a type known only at run time, as many-valued properties hold them.</summary>
internal static class TypedList
{
    private static readonly ConcurrentDictionary<Type, Func<Array, object>> Wrappers = new();
    private static readonly ConcurrentDictionary<Type, object> Empties = new();

    /// <summary>A read-only <see cref="IReadOnlyList{T}"/> of <paramref name="itemType"/> holding <paramref name="items"/>, each of that type.</summary>
    internal static object Of(Type itemType, IReadOnlyList<object> items)
    {
        if (items.Count == 0)
        {
            return Empty(itemType);
        }

        var array = Array.CreateInstance(itemType, items.Count);
        for (var i = 0; i < items.Count; i++)
        {
            array.SetValue(items[i], i);
        }

        return Wrappers.GetOrAdd(itemType, Wrapper)(array);
    }

    /// <summary>The empty read-only <see cref="IReadOnlyList{T}"/> of <paramref name="itemType"/>.</summary>
    internal static object Empty(Type itemType) =>
        Empties.GetOrAdd(itemType, type => Wrappers.GetOrAdd(type, Wrapper)(Array.CreateInstance(type, 0)));

    private static Func<Array, object> Wrapper(Type itemType) =>
        typeof(TypedList).GetMethod(nameof(Wrap), BindingFlags.NonPublic | BindingFlags.Static)!
            .MakeGenericMethod(itemType)
            .CreateDelegate<Func<Array, object>>();

    private static ReadOnlyCollection<T> Wrap<T>(Array array) => Array.AsReadOnly((T[])array);
}
