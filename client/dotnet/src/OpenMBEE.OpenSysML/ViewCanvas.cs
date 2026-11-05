using System;

namespace OpenMBEE.OpenSysML;

/// <summary>The drawing surface a view states, as a <c>DiagramLayout::Canvas</c> sizes it.</summary>
public sealed record ViewCanvas
{
    /// <summary>Creates a canvas.</summary>
    /// <param name="unit">The unit positions and sizes are in (<c>px</c>); empty when the view states none.</param>
    /// <param name="width">The surface's width, when <paramref name="hasSize"/>.</param>
    /// <param name="height">The surface's height, when <paramref name="hasSize"/>.</param>
    /// <param name="hasSize">Whether the view states the surface's size.</param>
    /// <exception cref="ArgumentNullException"><paramref name="unit"/> is null.</exception>
    public ViewCanvas(string unit, double width, double height, bool hasSize)
    {
        ArgumentNullException.ThrowIfNull(unit);
        Unit = unit;
        Width = width;
        Height = height;
        HasSize = hasSize;
    }

    /// <summary>The unit positions and sizes are in (<c>px</c>); empty when the view states none.</summary>
    public string Unit { get; }

    /// <summary>The surface's width, when <see cref="HasSize"/>.</summary>
    public double Width { get; }

    /// <summary>The surface's height, when <see cref="HasSize"/>.</summary>
    public double Height { get; }

    /// <summary>Whether the view states the surface's size.</summary>
    public bool HasSize { get; }
}
