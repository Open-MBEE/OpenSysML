using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Compares a normalized response against an expectation by the rules of "Comparison rules" in
/// conformance/README.md: only what the expectation names is compared, and every list it names
/// must have exactly the entries named; an unset field and one holding its default are the same;
/// Reals agree within a relative tolerance and integers by their digits. Scalars that are not
/// numbers compare by their spelling, as the reference runner compares them.
/// </summary>
internal static class Checks
{
    /// <summary>
    /// The relative difference two Reals may have and still be the same value: enough for another
    /// summation order, far below any difference a model would state. It applies to Reals alone;
    /// an integral field compared within it would let large counts and ids differ and pass.
    /// </summary>
    internal const double Tolerance = 1e-9;

    /// <summary>Every way an answer disagrees with an expectation.</summary>
    /// <param name="expect">What the answer must be.</param>
    /// <param name="actual">The normalized answer.</param>
    /// <returns>One message per mismatch, in a stable order; empty when the answer holds.</returns>
    internal static IReadOnlyList<string> Check(Expectation expect, IReadOnlyDictionary<string, object?> actual)
    {
        ArgumentNullException.ThrowIfNull(expect);
        ArgumentNullException.ThrowIfNull(actual);

        var failures = new List<string>();
        if (expect.Response is { } response)
        {
            failures.AddRange(Match("", response, actual));
        }

        foreach (var path in Sorted(expect.NonEmpty))
        {
            if (!Lookup(actual, path, out var value))
            {
                failures.Add($"{path}: not set, want a value");
            }
            else if (IsEmpty(value))
            {
                failures.Add($"{path}: empty, want a value");
            }
        }

        foreach (var path in Sorted(expect.Absent))
        {
            if (Lookup(actual, path, out var value) && !IsEmpty(value))
            {
                failures.Add($"{path}: set to {Render(value)}, want it unset");
            }
        }

        foreach (var path in Sorted(expect.Contains.Keys))
        {
            failures.AddRange(Contains(actual, path, expect.Contains[path]));
        }

        foreach (var path in Sorted(expect.ContainsAll.Keys))
        {
            failures.AddRange(ContainsAll(actual, path, expect.ContainsAll[path]));
        }

        foreach (var path in Sorted(expect.Counts.Keys))
        {
            var want = expect.Counts[path];
            if (!Count(actual, path, out var got))
            {
                failures.Add($"{path}: not a list or map, want {want} entries");
            }
            else if (got != want)
            {
                failures.Add($"{path}: {got} entries, want {want}");
            }
        }

        foreach (var path in Sorted(expect.MinCounts.Keys))
        {
            var want = expect.MinCounts[path];
            if (!Count(actual, path, out var got))
            {
                failures.Add($"{path}: not a list or map, want at least {want} entries");
            }
            else if (got < want)
            {
                failures.Add($"{path}: {got} entries, want at least {want}");
            }
        }

        return failures;
    }

    /// <summary>
    /// The value a dotted path reaches in a normalized tree: field names, map keys and list
    /// indices, as in <c>instances.0.feature_values.mass.value.real_value</c>.
    /// </summary>
    /// <param name="tree">The tree, or a part of it.</param>
    /// <param name="path">The dotted path.</param>
    /// <param name="value">The value there.</param>
    /// <returns>Whether the path reaches anything.</returns>
    internal static bool Lookup(object? tree, string path, out object? value)
    {
        value = tree;
        foreach (var segment in path.Split('.'))
        {
            if (!Step(value, segment, out value))
            {
                return false;
            }
        }

        return true;
    }

    /// <summary>A value as a scenario would write it, for messages.</summary>
    /// <param name="value">A normalized value or an expected <see cref="JsonElement"/>.</param>
    /// <returns>Its spelling.</returns>
    internal static string Render(object? value) => value switch
    {
        null => "nothing",
        string text => Spelling.Quote(text),
        JsonElement expected => RenderExpected(expected),
        IReadOnlyDictionary<string, object?> map =>
            "{" + string.Join(", ", map.Keys.Order(Spelling.CodePointOrder).Select(key => $"{key}: {Render(map[key])}")) + "}",
        IReadOnlyList<object?> list => "[" + string.Join(", ", list.Select(Render)) + "]",
        _ => Sprint(value),
    };

    private static List<string> Contains(IReadOnlyDictionary<string, object?> actual, string path, string want)
    {
        if (!Lookup(actual, path, out var value))
        {
            return [$"{path}: not set, want it to contain {Spelling.Quote(want)}"];
        }

        if (value is not string text)
        {
            return [$"{path}: {Render(value)} is not text, want it to contain {Spelling.Quote(want)}"];
        }

        return text.Contains(want, StringComparison.Ordinal)
            ? []
            : [$"{path}: {Spelling.Quote(text)} does not contain {Spelling.Quote(want)}"];
    }

    /// <summary>
    /// Checks every wanted string is at a path: a substring of the text there, or a member of the
    /// values there. A <c>*</c> segment takes one field from every entry of a list or map, as in
    /// <c>elements.*.id</c>.
    /// </summary>
    private static List<string> ContainsAll(IReadOnlyDictionary<string, object?> actual, string path, IReadOnlyList<string> wants)
    {
        if (Lookup(actual, path, out var at) && at is string text)
        {
            return [.. wants
                .Where(want => !text.Contains(want, StringComparison.Ordinal))
                .Select(want => $"{path}: does not contain {Spelling.Quote(want)}")];
        }

        if (!Values(actual, path, out var found))
        {
            return [$"{path}: neither text nor a list, want it to contain {Sprint(wants)}"];
        }

        return [.. wants
            .Where(want => !found.Any(item => item is string member && member == want))
            .Select(want => $"{path}: {Render(found)} does not contain {Spelling.Quote(want)}")];
    }

    /// <summary>The values at a path, expanding a <c>*</c> segment, and a list the path ends at into its members.</summary>
    private static bool Values(object? tree, string path, out IReadOnlyList<object?> found)
    {
        if (!Walk(tree, path.Split('.'), out var reached))
        {
            found = [];
            return false;
        }

        found = reached is [IReadOnlyList<object?> list] ? list : reached;
        return true;
    }

    private static bool Walk(object? current, ReadOnlySpan<string> segments, out List<object?> found)
    {
        found = [];
        if (segments.IsEmpty)
        {
            found.Add(current);
            return true;
        }

        if (segments[0] != "*")
        {
            return Step(current, segments[0], out var next) && Walk(next, segments[1..], out found);
        }

        IEnumerable<object?>? entries = current switch
        {
            IReadOnlyList<object?> list => list,
            IReadOnlyDictionary<string, object?> map => map.Keys.Order(Spelling.CodePointOrder).Select(key => map[key]),
            _ => null,
        };
        if (entries is null)
        {
            return false;
        }

        foreach (var entry in entries)
        {
            if (!Walk(entry, segments[1..], out var reached))
            {
                return false;
            }

            found.AddRange(reached);
        }

        return true;
    }

    /// <summary>
    /// Compares an expected tree against the answer's, reporting the paths that differ. Only what
    /// the expectation names is compared, so a field the schema adds later fails no scenario.
    /// </summary>
    private static List<string> Match(string path, JsonElement want, object? got)
    {
        switch (want.ValueKind)
        {
            case JsonValueKind.Object:
                if (got is not IReadOnlyDictionary<string, object?> actual)
                {
                    return [$"{At(path)}: {Render(got)}, want an object"];
                }

                var failures = new List<string>();
                foreach (var member in want.EnumerateObject().OrderBy(member => member.Name, Spelling.CodePointOrder))
                {
                    var child = Join(path, member.Name);
                    if (!actual.TryGetValue(member.Name, out var value))
                    {
                        // An unset field and a field left at its default are the same thing on the wire.
                        if (!IsDefault(member.Value))
                        {
                            failures.Add($"{child}: not set, want {Render(member.Value)}");
                        }

                        continue;
                    }

                    failures.AddRange(Match(child, member.Value, value));
                }

                return failures;

            case JsonValueKind.Array:
                if (got is not IReadOnlyList<object?> items)
                {
                    return [$"{At(path)}: {Render(got)}, want a list"];
                }

                var expected = want.GetArrayLength();
                if (items.Count != expected)
                {
                    return [$"{At(path)}: {items.Count} entries, want {expected} ({Render(got)})"];
                }

                return [.. want.EnumerateArray().SelectMany((item, index) =>
                    Match(Join(path, index.ToString(CultureInfo.InvariantCulture)), item, items[index]))];

            case JsonValueKind.Number:
                return MatchNumber(path, want.GetRawText(), got);

            default:
                return SprintExpected(want) == Sprint(got) ? [] : [$"{At(path)}: {Render(got)}, want {Render(want)}"];
        }
    }

    /// <summary>Compares an expected number, as the literal the scenario holds, against the value in the answer.</summary>
    private static List<string> MatchNumber(string path, string want, object? got)
    {
        switch (got)
        {
            case long integral:
                return MatchIntegral(path, want, integral.ToString(CultureInfo.InvariantCulture));
            case ulong unsigned:
                return MatchIntegral(path, want, unsigned.ToString(CultureInfo.InvariantCulture));
            case double real:
                return TryParseReal(want, out var number) && Near(real, number)
                    ? []
                    : [$"{At(path)}: {Spelling.Real(real)}, want {want}"];
            default:
                return [$"{At(path)}: {Render(got)}, want the number {want}"];
        }
    }

    /// <summary>Compares an integral field by its digits, so no difference is lost to floating point.</summary>
    private static List<string> MatchIntegral(string path, string want, string got) =>
        JsonInteger.Normalize(want) is { } digits && digits == got ? [] : [$"{At(path)}: {got}, want {want}"];

    private static bool TryParseReal(string text, out double number) =>
        double.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out number)
        && double.IsFinite(number)
        && (number != 0 || JsonInteger.Normalize(text) == "0");

    /// <summary>Whether two Reals are the same within <see cref="Tolerance"/> of the larger.</summary>
    private static bool Near(double got, double want)
    {
        if (!double.IsFinite(got) || !double.IsFinite(want))
        {
            return false;
        }

        if (got == want)
        {
            return true;
        }

        var scale = Math.Max(Math.Abs(got), Math.Abs(want));
        return Math.Abs(got - want) <= Tolerance * scale;
    }

    /// <summary>Whether an expected value is what an unset field holds.</summary>
    private static bool IsDefault(JsonElement want) => want.ValueKind switch
    {
        JsonValueKind.Null or JsonValueKind.False => true,
        JsonValueKind.Number => JsonInteger.Normalize(want.GetRawText()) == "0",
        JsonValueKind.String => want.GetString()!.Length == 0,
        JsonValueKind.Array => want.GetArrayLength() == 0,
        JsonValueKind.Object => !want.EnumerateObject().Any(),
        _ => false,
    };

    /// <summary>Whether a normalized value is what an unset field holds.</summary>
    private static bool IsEmpty(object? value) => value switch
    {
        null => true,
        bool flag => !flag,
        double real => real == 0,
        long integral => integral == 0,
        ulong unsigned => unsigned == 0,
        string text => text.Length == 0,
        IReadOnlyList<object?> list => list.Count == 0,
        IReadOnlyDictionary<string, object?> map => map.Count == 0,
        _ => false,
    };

    /// <summary>The number of entries of the list or map at a path; nothing there is an unset, so empty, one.</summary>
    private static bool Count(IReadOnlyDictionary<string, object?> tree, string path, out int count)
    {
        count = 0;
        if (!Lookup(tree, path, out var value))
        {
            return true;
        }

        switch (value)
        {
            case IReadOnlyList<object?> list:
                count = list.Count;
                return true;
            case IReadOnlyDictionary<string, object?> map:
                count = map.Count;
                return true;
            default:
                return false;
        }
    }

    private static bool Step(object? current, string segment, out object? next)
    {
        next = null;
        switch (current)
        {
            case IReadOnlyDictionary<string, object?> map:
                return map.TryGetValue(segment, out next);
            case IReadOnlyList<object?> list:
                if (!int.TryParse(segment, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out var index)
                    || index < 0
                    || index >= list.Count)
                {
                    return false;
                }

                next = list[index];
                return true;
            default:
                return false;
        }
    }

    /// <summary>An expected scalar as the reference prints it: text as itself, a boolean, or <c>&lt;nil&gt;</c>.</summary>
    private static string SprintExpected(JsonElement want) => want.ValueKind switch
    {
        JsonValueKind.String => want.GetString()!,
        JsonValueKind.True => "true",
        JsonValueKind.False => "false",
        _ => "<nil>",
    };

    /// <summary>A normalized value as the reference prints it with <c>fmt.Sprint</c>.</summary>
    private static string Sprint(object? value) => value switch
    {
        null => "<nil>",
        string text => text,
        bool flag => flag ? "true" : "false",
        double real => Spelling.Real(real),
        long integral => integral.ToString(CultureInfo.InvariantCulture),
        ulong unsigned => unsigned.ToString(CultureInfo.InvariantCulture),
        IReadOnlyDictionary<string, object?> map =>
            "map[" + string.Join(" ", map.Keys.Order(Spelling.CodePointOrder).Select(key => $"{key}:{Sprint(map[key])}")) + "]",
        IReadOnlyList<object?> list => "[" + string.Join(" ", list.Select(Sprint)) + "]",
        _ => Convert.ToString(value, CultureInfo.InvariantCulture) ?? "",
    };

    private static string RenderExpected(JsonElement expected) => expected.ValueKind switch
    {
        JsonValueKind.String => Spelling.Quote(expected.GetString()!),
        JsonValueKind.Number => expected.GetRawText(),
        JsonValueKind.True => "true",
        JsonValueKind.False => "false",
        JsonValueKind.Null => "nothing",
        JsonValueKind.Object => "{" + string.Join(", ", expected.EnumerateObject()
            .OrderBy(member => member.Name, Spelling.CodePointOrder)
            .Select(member => $"{member.Name}: {RenderExpected(member.Value)}")) + "}",
        _ => "[" + string.Join(", ", expected.EnumerateArray().Select(RenderExpected)) + "]",
    };

    private static IEnumerable<string> Sorted(IEnumerable<string> paths) => paths.Order(Spelling.CodePointOrder);

    private static string At(string path) => path.Length == 0 ? "response" : path;

    private static string Join(string path, string segment) => path.Length == 0 ? segment : $"{path}.{segment}";
}
