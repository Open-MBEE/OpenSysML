using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Ranks candidate names by their Ratcliff/Obershelp similarity to a word, as Python's
/// <c>difflib.get_close_matches</c> does, so every client suggests the same names for a typo.
/// </summary>
internal static class NearNames
{
    private const double Cutoff = 0.6;
    private static readonly Comparer<string> NameComparer = Comparer<string>.Create(CompareNames);

    /// <summary>The candidates at least <c>0.6</c> similar to a word, best first, ties by name descending.</summary>
    /// <param name="word">The word that matched nothing.</param>
    /// <param name="candidates">The names it may have meant; a repeated one counts once.</param>
    /// <param name="limit">The most names to return.</param>
    internal static ValueList<string> Closest(string word, IEnumerable<string> candidates, int limit) =>
        new(candidates
            .Distinct(StringComparer.Ordinal)
            .Select(candidate => (Score: Ratio(candidate, word), Name: candidate))
            .Where(scored => scored.Score >= Cutoff)
            .OrderByDescending(scored => scored.Score)
            .ThenByDescending(scored => scored.Name, NameComparer)
            .Take(limit)
            .Select(scored => scored.Name));

    /// <summary>Twice the characters the two strings' matching blocks share, over their total length.</summary>
    internal static double Ratio(string a, string b)
    {
        var aCharacters = CodePoints(a);
        var bCharacters = CodePoints(b);
        var total = aCharacters.Length + bCharacters.Length;
        return total == 0 ? 1.0 : 2.0 * Matches(aCharacters, 0, aCharacters.Length, bCharacters, 0, bCharacters.Length, PopularCharacters(bCharacters)) / total;
    }

    private static int[] CodePoints(string text)
    {
        var characters = new List<int>(text.Length);
        for (var index = 0; index < text.Length;)
        {
            characters.Add(NextCodePoint(text, ref index));
        }

        return characters.ToArray();
    }

    private static int NextCodePoint(string text, ref int index)
    {
        var character = text[index++];
        return char.IsHighSurrogate(character) && index < text.Length && char.IsLowSurrogate(text[index])
            ? char.ConvertToUtf32(character, text[index++])
            : character;
    }

    private static int CompareNames(string a, string b)
    {
        var aIndex = 0;
        var bIndex = 0;
        while (aIndex < a.Length && bIndex < b.Length)
        {
            var order = NextCodePoint(a, ref aIndex).CompareTo(NextCodePoint(b, ref bIndex));
            if (order != 0)
            {
                return order;
            }
        }

        return a.Length.CompareTo(b.Length);
    }

    private static HashSet<int> PopularCharacters(ReadOnlySpan<int> characters)
    {
        if (characters.Length < 200)
        {
            return [];
        }

        var counts = new Dictionary<int, int>();
        foreach (var character in characters)
        {
            counts[character] = counts.GetValueOrDefault(character) + 1;
        }

        // difflib excludes these anchors globally, even when matching a smaller subrange.
        var threshold = characters.Length / 100 + 1;
        return counts.Where(entry => entry.Value > threshold).Select(entry => entry.Key).ToHashSet();
    }

    // The longest common block, leftmost in a then in b, and recursively the blocks either side of it.
    private static int Matches(ReadOnlySpan<int> a, int aLow, int aHigh, ReadOnlySpan<int> b, int bLow, int bHigh, HashSet<int> popular)
    {
        var bestI = aLow;
        var bestJ = bLow;
        var bestSize = 0;
        var previous = new int[bHigh - bLow + 1];
        for (var i = aLow; i < aHigh; i++)
        {
            var current = new int[bHigh - bLow + 1];
            for (var j = bLow; j < bHigh; j++)
            {
                if (a[i] != b[j] || popular.Contains(b[j]))
                {
                    continue;
                }

                var size = previous[j - bLow] + 1;
                current[j - bLow + 1] = size;
                if (size > bestSize)
                {
                    bestSize = size;
                    bestI = i - size + 1;
                    bestJ = j - size + 1;
                }
            }

            previous = current;
        }

        // Popular characters can extend an anchor, or an otherwise anchorless common prefix.
        while (bestI > aLow && bestJ > bLow && a[bestI - 1] == b[bestJ - 1])
        {
            bestI--;
            bestJ--;
            bestSize++;
        }

        while (bestI + bestSize < aHigh && bestJ + bestSize < bHigh && a[bestI + bestSize] == b[bestJ + bestSize])
        {
            bestSize++;
        }

        return bestSize == 0
            ? 0
            : bestSize
                + Matches(a, aLow, bestI, b, bLow, bestJ, popular)
                + Matches(a, bestI + bestSize, aHigh, b, bestJ + bestSize, bHigh, popular);
    }
}
