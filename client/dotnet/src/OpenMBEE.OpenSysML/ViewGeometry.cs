namespace OpenMBEE.OpenSysML;

/// <summary>Where a node is drawn on a rendering's canvas, as a <c>DiagramLayout::Layout</c> places it.</summary>
/// <param name="X">The left of the node's box; no position when <paramref name="Unplaced"/>.</param>
/// <param name="Y">The top of the node's box; no position when <paramref name="Unplaced"/>.</param>
/// <param name="Width">The box's width, when <paramref name="HasSize"/>.</param>
/// <param name="Height">The box's height, when <paramref name="HasSize"/>.</param>
/// <param name="HasSize">Whether the layout states the box's size.</param>
/// <param name="Collapsed">Whether the layout folds the node's members away.</param>
/// <param name="Unplaced">Whether the layout places nothing, only folding or hiding, so the rendering lays the node out itself.</param>
public sealed record ViewGeometry(double X, double Y, double Width, double Height, bool HasSize, bool Collapsed, bool Unplaced);
