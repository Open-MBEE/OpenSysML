using System;
using System.Collections.Generic;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The immutable collections values and options are built from.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ValueCollectionTests
{
    [Fact]
    public void AListIsEqualToAnotherHoldingEqualElementsInTheSameOrder()
    {
        var list = new ValueList<Value>([new Value.IntegerValue(1), new Value.TextValue("a")]);

        Assert.Equal(list, new ValueList<Value>([new Value.IntegerValue(1), new Value.TextValue("a")]));
        Assert.Equal(list.GetHashCode(), new ValueList<Value>([new Value.IntegerValue(1), new Value.TextValue("a")]).GetHashCode());
        Assert.True(list == new ValueList<Value>(list));
        Assert.NotEqual(list, new ValueList<Value>([new Value.TextValue("a"), new Value.IntegerValue(1)]));
        Assert.True(list != new ValueList<Value>([new Value.IntegerValue(1)]));
        Assert.False(list.Equals((object)list.ToList()));
        Assert.Equal(list, NonGeneric(list).Cast<Value>());
        Assert.Equal(new ValueList<double>([double.NaN, -0.0]), new ValueList<double>([double.NaN, 0.0]));
    }

    [Fact]
    public void TheDefaultListIsTheEmptyList()
    {
        ValueList<long> unset = default;

        Assert.Equal(new ValueList<long>([]), unset);
        Assert.Equal(new ValueList<long>([]).GetHashCode(), unset.GetHashCode());
        Assert.True(unset.IsEmpty);
        Assert.Empty(unset);
        Assert.Equal(0, unset.AsSpan().Length);
        Assert.Equal("[]", unset.ToString());
        Assert.Throws<ArgumentOutOfRangeException>(() => unset[0]);
    }

    [Fact]
    public void AListCopiesWhatItIsBuiltFromAndRefusesNull()
    {
        long[] source = [1, 2, 3];
        var list = new ValueList<long>(source);

        source[0] = 9;

        Assert.Equal([1L, 2L, 3L], list);
        Assert.Equal(2L, list[1]);
        Assert.Equal(3, list.Count);
        Assert.Throws<ArgumentOutOfRangeException>(() => list[3]);
        Assert.Throws<ArgumentOutOfRangeException>(() => list[-1]);
        Assert.Throws<ArgumentNullException>(() => new ValueList<long>(null!));
        Assert.Contains("Element 1 is null", Assert.Throws<ArgumentException>(() => new ValueList<string>(["a", null!])).Message, StringComparison.Ordinal);
        Assert.Equal("[1, 2, 3]", list.ToString());
    }

    [Fact]
    public void ADictionaryIsTheSameWhateverOrderItWasBuiltIn()
    {
        var entries = new[] { Entry("b", 2), Entry("a", 1), Entry("B", 3), Entry("ä", 4) };
        var forward = new ValueDictionary<Value>(entries);
        var backward = new ValueDictionary<Value>(entries.Reverse());

        Assert.Equal(forward, backward);
        Assert.True(forward == backward);
        Assert.True(forward.Equals((object)backward));
        Assert.False(forward.Equals((object)entries.ToDictionary()));
        Assert.Equal(forward, NonGeneric(backward).Cast<KeyValuePair<string, Value>>());
        Assert.Equal(forward.GetHashCode(), backward.GetHashCode());
        Assert.Equal(["B", "a", "b", "ä"], forward.Keys);
        Assert.Equal(["B", "a", "b", "ä"], backward.Select(entry => entry.Key));
        Assert.Equal([3L, 1L, 2L, 4L], forward.Values.Select(value => ((Value.IntegerValue)value).Value));
        Assert.Equal("{B: 3, a: 1, b: 2, ä: 4}", backward.ToString());
        Assert.NotEqual(forward, new ValueDictionary<Value>(entries[..3]));
        Assert.True(forward != new ValueDictionary<Value>([Entry("b", 2), Entry("a", 1), Entry("B", 3), Entry("ä", 5)]));
        Assert.NotEqual(forward, new ValueDictionary<Value>([Entry("b", 2), Entry("a", 1), Entry("B", 3), Entry("ae", 4)]));
    }

    [Fact]
    public void ADictionaryLooksNamesUpOrdinally()
    {
        var dictionary = new ValueDictionary<Value>([Entry("mass", 1), Entry("Mass", 2)]);

        Assert.Equal(2, dictionary.Count);
        Assert.True(dictionary.ContainsKey("Mass"));
        Assert.False(dictionary.ContainsKey("MASS"));
        Assert.True(dictionary.TryGetValue("mass", out var mass));
        Assert.Equal(new Value.IntegerValue(1), mass);
        Assert.False(dictionary.TryGetValue("speed", out var none));
        Assert.Null(none);
        Assert.Equal(new Value.IntegerValue(2), dictionary["Mass"]);
        Assert.Throws<KeyNotFoundException>(() => dictionary["speed"]);
        Assert.Throws<ArgumentNullException>(() => dictionary.ContainsKey(null!));
    }

    [Fact]
    public void TheDefaultDictionaryIsTheEmptyDictionary()
    {
        ValueDictionary<Value> unset = default;

        Assert.Equal(new ValueDictionary<Value>([]), unset);
        Assert.True(unset.IsEmpty);
        Assert.Empty(unset);
        Assert.Empty(unset.Keys);
        Assert.False(unset.ContainsKey("a"));
        Assert.Equal("{}", unset.ToString());
    }

    [Fact]
    public void ADictionaryCopiesWhatItIsBuiltFromAndRefusesRepeatsAndNull()
    {
        var source = new Dictionary<string, Value> { ["a"] = new Value.IntegerValue(1) };
        var dictionary = new ValueDictionary<Value>(source);

        source["a"] = new Value.IntegerValue(9);
        source["b"] = new Value.IntegerValue(2);

        Assert.Equal(new ValueDictionary<Value>([Entry("a", 1)]), dictionary);
        Assert.Contains("The name a repeats", Assert.Throws<ArgumentException>(() => new ValueDictionary<Value>([Entry("a", 1), Entry("b", 2), Entry("a", 1)])).Message, StringComparison.Ordinal);
        Assert.Throws<ArgumentException>(() => new ValueDictionary<Value>([KeyValuePair.Create<string, Value>(null!, new Value.IntegerValue(1))]));
        Assert.Throws<ArgumentException>(() => new ValueDictionary<Value>([KeyValuePair.Create<string, Value>("a", null!)]));
        Assert.Throws<ArgumentNullException>(() => new ValueDictionary<Value>(null!));
    }

    // Enumerates through the non-generic interface alone, which Cast would bypass for a generic sequence.
    private static List<object> NonGeneric(System.Collections.IEnumerable items)
    {
        var read = new List<object>();
        foreach (var item in items)
        {
            read.Add(item);
        }

        return read;
    }

    private static KeyValuePair<string, Value> Entry(string name, long value) => KeyValuePair.Create<string, Value>(name, new Value.IntegerValue(value));
}
