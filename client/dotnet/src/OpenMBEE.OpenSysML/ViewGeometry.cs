namespace OpenMBEE.OpenSysML;

/// <summary>Where a node is drawn on a rendering's canvas, as a <c>DiagramLayout::Layout</c> places it.</summary>
public sealed record ViewGeometry
{
    internal ViewGeometry(double x, double y, double width, double height, bool hasSize, bool collapsed, bool unplaced)
    {
        X = x;
        Y = y;
        Width = width;
        Height = height;
        HasSize = hasSize;
        Collapsed = collapsed;
        Unplaced = unplaced;
    }

    /// <summary>The left of the node's box; no position when <see cref="Unplaced"/>.</summary>
    public double X { get; }

    /// <summary>The top of the node's box; no position when <see cref="Unplaced"/>.</summary>
    public double Y { get; }

    /// <summary>The box's width, when <see cref="HasSize"/>.</summary>
    public double Width { get; }

    /// <summary>The box's height, when <see cref="HasSize"/>.</summary>
    public double Height { get; }

    /// <summary>Whether the layout states the box's size.</summary>
    public bool HasSize { get; }

    /// <summary>Whether the layout folds the node's members away.</summary>
    public bool Collapsed { get; }

    /// <summary>Whether the layout places nothing, only folding or hiding, so the rendering lays the node out itself.</summary>
    public bool Unplaced { get; }
}
