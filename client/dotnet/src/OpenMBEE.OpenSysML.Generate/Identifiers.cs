using System;
using System.Collections.Frozen;
using System.Globalization;
using System.Text;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>How SysML names become C# identifiers and literals.</summary>
internal static class Identifiers
{
    // Reserved and intrinsic keywords, plus contextual keywords restricted in type declarations.
    private static readonly FrozenSet<string> Keywords = FrozenSet.ToFrozenSet(
        [
            "abstract", "as", "base", "bool", "break", "byte", "case", "catch", "char", "checked", "class", "const",
            "continue", "decimal", "default", "delegate", "do", "double", "else", "enum", "event", "explicit",
            "extern", "false", "finally", "fixed", "float", "for", "foreach", "goto", "if", "implicit", "in", "int",
            "interface", "internal", "is", "lock", "long", "namespace", "new", "null", "object", "operator", "out",
            "override", "params", "private", "protected", "public", "readonly", "ref", "return", "sbyte", "sealed",
            "short", "sizeof", "stackalloc", "static", "string", "struct", "switch", "this", "throw", "true", "try",
            "typeof", "uint", "ulong", "unchecked", "unsafe", "ushort", "using", "virtual", "void", "volatile", "while",
            "__arglist", "__makeref", "__reftype", "__refvalue", "record", "file", "required", "scoped",
        ],
        StringComparer.Ordinal);

    /// <summary>Whether a word is a C# keyword, which an identifier spells with a leading <c>@</c>.</summary>
    internal static bool IsKeyword(string word) => Keywords.Contains(word);

    /// <summary>A C# identifier for a type or namespace segment: formatting characters removed, each character no identifier may hold made <c>_</c>.</summary>
    internal static string TypeName(string name)
    {
        var builder = new StringBuilder(name.Length + 1);
        foreach (var rune in name.EnumerateRunes())
        {
            if (Rune.GetUnicodeCategory(rune) == UnicodeCategory.Format)
            {
                continue;
            }

            builder.Append(IsPart(rune) ? rune.ToString() : "_");
        }

        if (builder.Length == 0 || !IsStart(Rune.GetRuneAt(builder.ToString(), 0)))
        {
            builder.Insert(0, '_');
        }

        return builder.ToString();
    }

    /// <summary>
    /// A C# property name for a feature: the name split where it holds anything but letters and digits, each
    /// part's first letter upper-cased, and a leading digit kept off the start by <c>_</c>; empty when the
    /// name holds no letter or digit.
    /// </summary>
    internal static string MemberName(string name)
    {
        var builder = new StringBuilder(name.Length + 1);
        var startOfPart = true;
        foreach (var rune in name.EnumerateRunes())
        {
            if (!IsWordPart(rune))
            {
                startOfPart = true;
                continue;
            }

            builder.Append((startOfPart ? Rune.ToUpperInvariant(rune) : rune).ToString());
            startOfPart = false;
        }

        if (builder.Length > 0 && !IsStart(Rune.GetRuneAt(builder.ToString(), 0)))
        {
            builder.Insert(0, '_');
        }

        return builder.ToString();
    }

    /// <summary>An identifier as C# source spells it: a keyword with a leading <c>@</c>.</summary>
    internal static string Escaped(string identifier) => IsKeyword(identifier) ? "@" + identifier : identifier;

    /// <summary>Whether a string is an identifier as written, neither empty nor a keyword nor holding a character no identifier may.</summary>
    internal static bool IsPlainIdentifier(string text) =>
        text.Length > 0 && !IsKeyword(text) && TypeName(text) == text;

    /// <summary>A C# string literal holding the text exactly.</summary>
    internal static string Literal(string text)
    {
        var builder = new StringBuilder(text.Length + 2).Append('"');
        foreach (var c in text)
        {
            switch (c)
            {
                case '"':
                    builder.Append("\\\"");
                    break;
                case '\\':
                    builder.Append("\\\\");
                    break;
                default:
                    if (char.IsControl(c) || char.IsSurrogate(c) || c is '\u2028' or '\u2029')
                    {
                        builder.Append("\\u").Append(((int)c).ToString("x4", CultureInfo.InvariantCulture));
                    }
                    else
                    {
                        builder.Append(c);
                    }

                    break;
            }
        }

        return builder.Append('"').ToString();
    }

    /// <summary>Text made safe for an XML documentation comment on one line.</summary>
    internal static string DocText(string text)
    {
        var builder = new StringBuilder(text.Length);
        foreach (var c in text)
        {
            builder.Append(c switch
            {
                '&' => "&amp;",
                '<' => "&lt;",
                '>' => "&gt;",
                _ when char.IsControl(c) || c is '\u2028' or '\u2029' => " ",
                _ => c.ToString(),
            });
        }

        return builder.ToString();
    }

    // Letters, digits and the marks that combine with them; anything else separates two parts of a member name.
    private static bool IsWordPart(Rune rune) => rune.IsBmp && (rune.Value != '_' && IsStart(rune) || Rune.GetUnicodeCategory(rune) is
        UnicodeCategory.DecimalDigitNumber or UnicodeCategory.NonSpacingMark or UnicodeCategory.SpacingCombiningMark);

    // C# identifier characters are UTF-16 code units, not supplementary Unicode scalars.
    private static bool IsStart(Rune rune) => rune.IsBmp && (rune.Value == '_' || Rune.GetUnicodeCategory(rune) is
        UnicodeCategory.UppercaseLetter or UnicodeCategory.LowercaseLetter or UnicodeCategory.TitlecaseLetter
        or UnicodeCategory.ModifierLetter or UnicodeCategory.OtherLetter or UnicodeCategory.LetterNumber);

    private static bool IsPart(Rune rune) => rune.IsBmp && (IsStart(rune) || Rune.GetUnicodeCategory(rune) is
        UnicodeCategory.DecimalDigitNumber or UnicodeCategory.ConnectorPunctuation
        or UnicodeCategory.NonSpacingMark or UnicodeCategory.SpacingCombiningMark);
}
