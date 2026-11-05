using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>How values are written for a reader: culture-invariant, and a Real never mistaken for an Integer.</summary>
internal static class ValueText
{
    /// <summary>A Real, shortest round-trip, with <c>.0</c> on a whole number so it reads as no Integer.</summary>
    internal static string Real(double value)
    {
        var text = value.ToString("R", CultureInfo.InvariantCulture);
        return double.IsFinite(value) && text.AsSpan().IndexOfAny('.', 'E') < 0 ? text + ".0" : text;
    }

    /// <summary>A number with no decoration: a scale or an exponent.</summary>
    internal static string Plain(double value) => value.ToString("R", CultureInfo.InvariantCulture);

    /// <summary>A whole number.</summary>
    internal static string Whole(long value) => value.ToString(CultureInfo.InvariantCulture);

    /// <summary>Text as a SysML string literal: double-quoted, with a quote or a backslash escaped.</summary>
    internal static string Quoted(string text)
    {
        var quoted = new StringBuilder(text.Length + 2).Append('"');
        foreach (var c in text)
        {
            if (c is '"' or '\\')
            {
                quoted.Append('\\');
            }

            quoted.Append(c);
        }

        return quoted.Append('"').ToString();
    }

    /// <summary>Items joined by a comma and a space, each culture-invariant.</summary>
    internal static string Join<T>(IEnumerable<T> items) =>
        string.Join(", ", Each(items));

    /// <summary>One item, culture-invariant where it can be.</summary>
    internal static string Item<T>(T item) => item switch
    {
        double real => Real(real),
        IFormattable formattable => formattable.ToString(null, CultureInfo.InvariantCulture),
        _ => item?.ToString() ?? string.Empty,
    };

    private static IEnumerable<string> Each<T>(IEnumerable<T> items)
    {
        foreach (var item in items)
        {
            yield return Item(item);
        }
    }
}
