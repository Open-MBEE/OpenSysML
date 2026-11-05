using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// How the reference runner, tools/cmd/conformance, spells text, numbers and orders, so that this
/// runner's mismatch messages, scalar comparisons and labelling order read as the reference's do.
/// </summary>
internal static class Spelling
{
    /// <summary>Strings in the order of their Unicode code points, which is the order of their UTF-8 bytes.</summary>
    internal static IComparer<string> CodePointOrder { get; } = new CodePointComparer();

    /// <summary>A string quoted and escaped as Go's <c>strconv.Quote</c> writes it.</summary>
    /// <param name="text">The string.</param>
    /// <returns>The quoted string.</returns>
    internal static string Quote(string text)
    {
        var quoted = new StringBuilder(text.Length + 2).Append('"');
        for (var index = 0; index < text.Length; index++)
        {
            if (Rune.TryGetRuneAt(text, index, out var rune))
            {
                index += rune.Utf16SequenceLength - 1;
                Escape(quoted, rune);
            }
            else
            {
                // A lone surrogate is no character; it is written as the code unit it is.
                quoted.Append(CultureInfo.InvariantCulture, $"\\u{(int)text[index]:x4}");
            }
        }

        return quoted.Append('"').ToString();
    }

    /// <summary>
    /// A double as Go's <c>%v</c> writes it: the fewest digits that read back as the same double,
    /// in exponent form when the exponent is below -4 or at least 6.
    /// </summary>
    /// <param name="value">The double.</param>
    /// <returns>Its spelling.</returns>
    internal static string Real(double value)
    {
        if (double.IsNaN(value))
        {
            return "NaN";
        }

        if (double.IsInfinity(value))
        {
            return value > 0 ? "+Inf" : "-Inf";
        }

        var sign = double.IsNegative(value) ? "-" : "";
        var (digits, point) = ShortestDigits(Math.Abs(value));
        if (digits.Length == 0)
        {
            return sign + "0";
        }

        var exponent = point - 1;
        if (exponent < -4 || exponent >= 6)
        {
            var mantissa = digits.Length == 1 ? digits : $"{digits[0]}.{digits[1..]}";
            var magnitude = Math.Abs(exponent).ToString("00", CultureInfo.InvariantCulture);
            return $"{sign}{mantissa}e{(exponent < 0 ? '-' : '+')}{magnitude}";
        }

        if (point <= 0)
        {
            return $"{sign}0.{new string('0', -point)}{digits}";
        }

        return point >= digits.Length
            ? sign + digits + new string('0', point - digits.Length)
            : $"{sign}{digits[..point]}.{digits[point..]}";
    }

    /// <summary>
    /// The significant digits of a non-negative double's shortest round-trip spelling, without
    /// leading or trailing zeros, and where the decimal point falls: the value is
    /// 0.<i>digits</i> × 10^<i>point</i>.
    /// </summary>
    private static (string Digits, int Point) ShortestDigits(double value)
    {
        var text = value.ToString("R", CultureInfo.InvariantCulture);
        var exponentAt = text.IndexOf('E', StringComparison.Ordinal);
        var exponent = exponentAt < 0 ? 0 : int.Parse(text[(exponentAt + 1)..], NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture);
        var mantissa = exponentAt < 0 ? text : text[..exponentAt];

        var pointAt = mantissa.IndexOf('.', StringComparison.Ordinal);
        var whole = pointAt < 0 ? mantissa.Length : pointAt;
        var digits = mantissa.Replace(".", "", StringComparison.Ordinal);
        var leading = digits.Length - digits.TrimStart('0').Length;
        digits = digits.Trim('0');
        return (digits, whole - leading + exponent);
    }

    private static void Escape(StringBuilder quoted, Rune rune)
    {
        switch (rune.Value)
        {
            case '"':
                quoted.Append("\\\"");
                return;
            case '\\':
                quoted.Append("\\\\");
                return;
            case '\a':
                quoted.Append("\\a");
                return;
            case '\b':
                quoted.Append("\\b");
                return;
            case '\f':
                quoted.Append("\\f");
                return;
            case '\n':
                quoted.Append("\\n");
                return;
            case '\r':
                quoted.Append("\\r");
                return;
            case '\t':
                quoted.Append("\\t");
                return;
            case '\v':
                quoted.Append("\\v");
                return;
        }

        if (IsPrint(rune))
        {
            quoted.Append(rune.ToString());
        }
        else if (rune.Value < ' ' || rune.Value == 0x7F)
        {
            quoted.Append(CultureInfo.InvariantCulture, $"\\x{rune.Value:x2}");
        }
        else if (rune.IsBmp)
        {
            quoted.Append(CultureInfo.InvariantCulture, $"\\u{rune.Value:x4}");
        }
        else
        {
            quoted.Append(CultureInfo.InvariantCulture, $"\\U{rune.Value:x8}");
        }
    }

    /// <summary>Go's <c>unicode.IsPrint</c>: a letter, mark, number, punctuation, symbol or the ASCII space.</summary>
    private static bool IsPrint(Rune rune) =>
        rune.Value == ' ' || Rune.GetUnicodeCategory(rune) switch
        {
            UnicodeCategory.UppercaseLetter or UnicodeCategory.LowercaseLetter or UnicodeCategory.TitlecaseLetter
                or UnicodeCategory.ModifierLetter or UnicodeCategory.OtherLetter => true,
            UnicodeCategory.NonSpacingMark or UnicodeCategory.SpacingCombiningMark or UnicodeCategory.EnclosingMark => true,
            UnicodeCategory.DecimalDigitNumber or UnicodeCategory.LetterNumber or UnicodeCategory.OtherNumber => true,
            UnicodeCategory.ConnectorPunctuation or UnicodeCategory.DashPunctuation or UnicodeCategory.OpenPunctuation
                or UnicodeCategory.ClosePunctuation or UnicodeCategory.InitialQuotePunctuation
                or UnicodeCategory.FinalQuotePunctuation or UnicodeCategory.OtherPunctuation => true,
            UnicodeCategory.MathSymbol or UnicodeCategory.CurrencySymbol or UnicodeCategory.ModifierSymbol
                or UnicodeCategory.OtherSymbol => true,
            _ => false,
        };

    /// <summary>
    /// Ordinal order, except that a surrogate (a code point above U+FFFF) sorts after U+E000–U+FFFF,
    /// which UTF-16 code units alone would put it before.
    /// </summary>
    private sealed class CodePointComparer : IComparer<string>
    {
        public int Compare(string? x, string? y)
        {
            if (ReferenceEquals(x, y))
            {
                return 0;
            }

            if (x is null || y is null)
            {
                return x is null ? -1 : 1;
            }

            var length = Math.Min(x.Length, y.Length);
            for (var index = 0; index < length; index++)
            {
                var (a, b) = (x[index], y[index]);
                if (a == b)
                {
                    continue;
                }

                if (char.IsSurrogate(a) != char.IsSurrogate(b))
                {
                    return char.IsSurrogate(a) ? 1 : -1;
                }

                return a.CompareTo(b);
            }

            return x.Length.CompareTo(y.Length);
        }
    }
}
