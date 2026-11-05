using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a node or edge is drawn, as a <c>DiagramLayout::Style</c> colours it. Each field is optional: an empty text, a
/// zero size or a false flag leaves it to the drawing style.
/// </summary>
public sealed record ViewStyle
{
    /// <summary>Creates a style.</summary>
    /// <param name="fill">The fill colour, as <c>#RRGGBB</c>.</param>
    /// <param name="line">The line colour, as <c>#RRGGBB</c>.</param>
    /// <param name="text">The text colour, as <c>#RRGGBB</c>.</param>
    /// <param name="font">The font face.</param>
    /// <param name="fontSize">The font size, in points.</param>
    /// <param name="bold">Whether text is bold.</param>
    /// <param name="italic">Whether text is italic.</param>
    /// <exception cref="ArgumentNullException">A text argument is null.</exception>
    public ViewStyle(string fill, string line, string text, string font, double fontSize, bool bold, bool italic)
    {
        ArgumentNullException.ThrowIfNull(fill);
        ArgumentNullException.ThrowIfNull(line);
        ArgumentNullException.ThrowIfNull(text);
        ArgumentNullException.ThrowIfNull(font);
        Fill = fill;
        Line = line;
        Text = text;
        Font = font;
        FontSize = fontSize;
        Bold = bold;
        Italic = italic;
    }

    /// <summary>The fill colour, as <c>#RRGGBB</c>; empty for the drawing style's.</summary>
    public string Fill { get; }

    /// <summary>The line colour, as <c>#RRGGBB</c>; empty for the drawing style's.</summary>
    public string Line { get; }

    /// <summary>The text colour, as <c>#RRGGBB</c>; empty for the drawing style's.</summary>
    public string Text { get; }

    /// <summary>The font face; empty for the drawing style's.</summary>
    public string Font { get; }

    /// <summary>The font size, in points; 0 for the drawing style's.</summary>
    public double FontSize { get; }

    /// <summary>Whether text is bold.</summary>
    public bool Bold { get; }

    /// <summary>Whether text is italic.</summary>
    public bool Italic { get; }
}
