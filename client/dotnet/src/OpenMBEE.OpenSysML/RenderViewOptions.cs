using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>How <see cref="Model.RenderViewAsync"/> renders a view. Collections are copied when they are set.</summary>
public sealed class RenderViewOptions
{
    private readonly ValueList<string> show;

    /// <summary>The service's defaults: <see cref="RenderViewPorts.Minimal"/> ports, nothing shown besides the rendering's own, the view's own depth and <see cref="RenderViewCompartments.Positioned"/> compartments.</summary>
    public static RenderViewOptions Default { get; } = new();

    /// <summary>Which ports the rendering draws on its nodes' borders.</summary>
    public RenderViewPorts Ports { get; init; }

    /// <summary>
    /// The optional notation elements to draw besides those the view's rendering shows, named as
    /// <see cref="ViewNotation"/> names them; empty draws only the rendering's own.
    /// </summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Show
    {
        get => show;
        init => show = new ValueList<string>(value);
    }

    /// <summary>
    /// How many levels of nested elements to draw below each exposed element, overriding the depth the view states;
    /// null keeps the view's own. A depth is positive.
    /// </summary>
    public int? Depth { get; init; }

    /// <summary>
    /// How a tree rendering folds a node's owned members into compartment rows; <see cref="RenderViewCompartments.Positioned"/>
    /// by default. A service older than the field draws every member as a node of its own whatever this asks.
    /// </summary>
    public RenderViewCompartments Compartments { get; init; }
}
