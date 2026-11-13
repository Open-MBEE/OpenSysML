using System;
using System.Globalization;
using System.Text.RegularExpressions;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>Reads whole protobuf-JSON numbers without rounding through a floating-point value.</summary>
internal static partial class JsonInteger
{
    /// <summary>Canonical digits when the spelling is a whole int64 or uint64 value; null otherwise.</summary>
    /// <param name="text">A number or quoted integer's spelling, including decimal or exponent notation.</param>
    /// <returns>The exact digits, with negative zero written as zero.</returns>
    internal static string? Normalize(string text)
    {
        ArgumentNullException.ThrowIfNull(text);
        var match = Number().Match(text);
        if (!match.Success)
        {
            return null;
        }

        var fraction = match.Groups["fraction"].Value;
        var coefficient = (match.Groups["whole"].Value + fraction).TrimStart('0');
        if (coefficient.Length == 0)
        {
            return "0";
        }

        var exponent = 0;
        var statedExponent = match.Groups["exponent"];
        if (statedExponent.Success
            && !int.TryParse(statedExponent.ValueSpan, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out exponent))
        {
            return null;
        }

        var significant = coefficient.TrimEnd('0');
        var zeroes = (long)exponent - fraction.Length + coefficient.Length - significant.Length;
        // No 64-bit integer needs more than twenty digits; bound allocation before expanding an exponent.
        if (zeroes < 0 || significant.Length + zeroes > 20)
        {
            return null;
        }

        var digits = significant + new string('0', (int)zeroes);
        if (match.Groups["sign"].Value == "-")
        {
            digits = "-" + digits;
        }

        return long.TryParse(digits, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out _)
            || ulong.TryParse(digits, NumberStyles.None, CultureInfo.InvariantCulture, out _)
            ? digits
            : null;
    }

    [GeneratedRegex(@"\A(?<sign>-?)(?:(?<whole>0|[1-9][0-9]*)(?:\.(?<fraction>[0-9]*))?|\.(?<fraction>[0-9]+))(?:[eE](?<exponent>[+-]?[0-9]+))?\z", RegexOptions.CultureInvariant)]
    private static partial Regex Number();
}
