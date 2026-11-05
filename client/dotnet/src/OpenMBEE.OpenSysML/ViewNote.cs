using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A note box drawn on a view rendering's canvas, as a <c>DiagramLayout::Note</c> states it: anchored to a node, to an
/// edge by its end nodes, or free on the surface. One note stated about several elements is one note per element, all
/// of one origin, drawn as one box.
/// </summary>
public sealed record ViewNote
{
    /// <summary>Creates a note.</summary>
    /// <param name="text">The note's text.</param>
    /// <param name="anchor">The <see cref="ViewNode.Id"/> it is anchored to; empty for none.</param>
    /// <param name="edgeFrom">The node an edge it is anchored to leaves; empty for none.</param>
    /// <param name="edgeTo">The node an edge it is anchored to reaches; empty for none.</param>
    /// <param name="x">The left of its box.</param>
    /// <param name="y">The top of its box.</param>
    /// <param name="width">Its box's width, when <paramref name="hasSize"/>.</param>
    /// <param name="height">Its box's height, when <paramref name="hasSize"/>.</param>
    /// <param name="hasSize">Whether the note states its box's size.</param>
    /// <param name="origin">Where the note is stated; null when the service located it nowhere.</param>
    /// <exception cref="ArgumentNullException">A text argument is null.</exception>
    public ViewNote(string text, string anchor, string edgeFrom, string edgeTo, double x, double y, double width, double height, bool hasSize, SourceSpan? origin)
    {
        ArgumentNullException.ThrowIfNull(text);
        ArgumentNullException.ThrowIfNull(anchor);
        ArgumentNullException.ThrowIfNull(edgeFrom);
        ArgumentNullException.ThrowIfNull(edgeTo);
        Text = text;
        Anchor = anchor;
        EdgeFrom = edgeFrom;
        EdgeTo = edgeTo;
        X = x;
        Y = y;
        Width = width;
        Height = height;
        HasSize = hasSize;
        Origin = origin;
    }

    /// <summary>The note's text.</summary>
    public string Text { get; }

    /// <summary>The <see cref="ViewNode.Id"/> the note is anchored to; empty when it is anchored to an edge or free.</summary>
    public string Anchor { get; }

    /// <summary>The <see cref="ViewNode.Id"/> an edge the note is anchored to leaves; empty otherwise.</summary>
    public string EdgeFrom { get; }

    /// <summary>The <see cref="ViewNode.Id"/> an edge the note is anchored to reaches; empty otherwise.</summary>
    public string EdgeTo { get; }

    /// <summary>The left of the note's box.</summary>
    public double X { get; }

    /// <summary>The top of the note's box.</summary>
    public double Y { get; }

    /// <summary>The box's width, when <see cref="HasSize"/>.</summary>
    public double Width { get; }

    /// <summary>The box's height, when <see cref="HasSize"/>.</summary>
    public double Height { get; }

    /// <summary>Whether the note states its box's size.</summary>
    public bool HasSize { get; }

    /// <summary>Where the note is stated; null when the service located it nowhere.</summary>
    public SourceSpan? Origin { get; }
}
