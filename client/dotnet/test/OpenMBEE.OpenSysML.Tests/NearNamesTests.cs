using System.Linq;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The names a failed lookup suggests, held to Python's <c>difflib.get_close_matches</c>, whose answers
/// below were computed by Python itself, so every client suggests the same names for a typo.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class NearNamesTests
{
    public static TheoryData<string, string[], string[]> Difflib => new()
    {
        { "Vehicel", ["Vehicle", "Engine", "Demo::Vehicle", "Demo::Engine", "mass", "engine"], ["Vehicle", "Demo::Vehicle"] },
        { "engin", ["Engine", "engine", "Demo::Engine", "Demo::Vehicle::engine", "power"], ["engine", "Engine"] },
        { "Demo::Vehicl", ["Demo::Vehicle", "Vehicle", "Demo::Engine", "Demo"], ["Demo::Vehicle", "Vehicle"] },
        { "zzz", ["Vehicle", "Engine"], [] },
        { "mass", ["mass", "Mass", "masses", "pass", "mast", "bass"], ["mass", "masses", "pass"] },
        { "\U0001F600a", ["\U0001F601a", "\U0001F600b", "a"], ["a"] },
        { "aa", ["aa\uE000", "aa\U0001F600", "aa\uD7FF"], ["aa\U0001F600", "aa\uE000", "aa\uD7FF"] },
        { "b" + new string('a', 198), [new string('a', 198) + "b"], [new string('a', 198) + "b"] },
        { "b" + new string('a', 199), [new string('a', 199) + "b"], [] },
        { "b" + new string('a', 200), [new string('a', 200) + "b", "b" + new string('a', 200)], ["b" + new string('a', 200)] },
    };

    [Theory]
    [MemberData(nameof(Difflib))]
    public void SuggestsWhatPythonsDifflibSuggests(string word, string[] candidates, string[] expected)
    {
        Assert.Equal(expected, NearNames.Closest(word, candidates, 3));
    }

    [Fact]
    public void ScoresAsDifflibsRatio()
    {
        Assert.Equal(0.75, NearNames.Ratio("abcd", "bcda"));
        Assert.Equal(1.0, NearNames.Ratio(string.Empty, string.Empty));
        Assert.Equal(2.0 / 3.0, NearNames.Ratio("GAATTCAG", "GGATCGA"), 12);
        Assert.Equal(12.0 / 14.0, NearNames.Ratio("Vehicle", "Vehicel"), 12);
    }

    public static TheoryData<string, string, double> UnicodeAndPopularRatios => new()
    {
        { "\U0001F600a", "\U0001F601a", 0.5 },
        { "\U0001F600", "\U0001F600", 1.0 },
        { "\U0001F600", string.Empty, 0.0 },
        { "\uD800a", "\uD801a", 0.5 },
        { new string('a', 198) + "b", "b" + new string('a', 198), 198.0 / 199.0 },
        { new string('a', 199) + "b", "b" + new string('a', 199), 1.0 / 200.0 },
        { new string('a', 200) + "b", "b" + new string('a', 200), 1.0 / 201.0 },
        { new string('a', 200), new string('a', 200), 1.0 },
        { new string('a', 200) + "bc", new string('a', 200) + "bd", 201.0 / 202.0 },
        { "x" + new string('a', 200) + "z", "y" + new string('a', 200) + "z", 201.0 / 202.0 },
        { new string('a', 100) + new string('b', 100) + "x", new string('b', 100) + new string('a', 100) + "x", 1.0 / 201.0 },
        { string.Concat(Enumerable.Repeat("\U0001F600", 100)) + "a", "a" + string.Concat(Enumerable.Repeat("\U0001F600", 100)), 100.0 / 101.0 },
    };

    [Theory]
    [MemberData(nameof(UnicodeAndPopularRatios))]
    public void UnicodeAndPopularCharactersScoreAsDifflibsRatio(string a, string b, double expected)
    {
        Assert.Equal(expected, NearNames.Ratio(a, b), 12);
    }

    [Fact]
    public void ARepeatedCandidateIsSuggestedOnce()
    {
        // difflib itself would answer ab twice; a suggestion list names each name once, as the Java client's does.
        Assert.Equal(["ab", "abe", "abd"], NearNames.Closest("ab", ["ba", "ab", "abc", "abd", "abe", "ab"], 3));
        Assert.Empty(NearNames.Closest("x", Enumerable.Empty<string>(), 3));
    }
}
