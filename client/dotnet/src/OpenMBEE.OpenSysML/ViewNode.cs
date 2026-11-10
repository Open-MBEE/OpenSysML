using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>One node of a view rendering, with the node it is nested in.</summary>
public sealed record ViewNode
{
    /// <summary>Creates a node.</summary>
    /// <param name="id">The node's id within the rendering, which an edge and a child's parent name.</param>
    /// <param name="kind">What the notation calls the element (<c>part def</c>, <c>state</c>, <c>fork</c>).</param>
    /// <param name="name">The element's name; empty for an anonymous one.</param>
    /// <param name="nameSynthesized">Whether the model did not give the name.</param>
    /// <param name="type">The declared type of a typed usage, as the notation writes it; empty otherwise.</param>
    /// <param name="detail">What else the rendering says about the node; empty when nothing.</param>
    /// <param name="text">The text heading a node whose name is not shown; empty otherwise.</param>
    /// <param name="standIn">Whether a migration made the node up, standing for no element of its source.</param>
    /// <param name="parent">The id of the node this one is nested in; empty for a root.</param>
    /// <param name="ports">The ports drawn on its border, in order; copied.</param>
    /// <param name="origin">Where the element is declared; null when the service located it nowhere.</param>
    /// <param name="geometry">Where a layout draws it; null when no layout annotates it.</param>
    /// <param name="style">How a style draws it; null when no style colours it.</param>
    /// <param name="compartment">The compartment of its parent's box it is a row of (<c>attributes</c>, <c>parts</c>); empty for a node drawn as a box.</param>
    /// <param name="inherited">Whether the owner it is drawn or listed under has it through typing, subclassification or subsetting rather than declares it.</param>
    /// <param name="arrow">For an item box drawn on a connection line, the end the item goes toward: <c>right</c> for the line's second, <c>left</c> for its first; empty for every other node.</param>
    /// <param name="parallel">Whether the state's substates run in parallel.</param>
    /// <exception cref="ArgumentNullException">An argument other than the origin, geometry or style is null.</exception>
    /// <exception cref="ArgumentException">A port is null.</exception>
    internal ViewNode(
        string id,
        string kind,
        string name,
        bool nameSynthesized,
        string type,
        string detail,
        string text,
        bool standIn,
        string parent,
        IEnumerable<ViewPort> ports,
        SourceSpan? origin,
        ViewGeometry? geometry,
        ViewStyle? style,
        string compartment,
        bool inherited,
        string arrow,
        bool parallel)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(type);
        ArgumentNullException.ThrowIfNull(detail);
        ArgumentNullException.ThrowIfNull(text);
        ArgumentNullException.ThrowIfNull(parent);
        ArgumentNullException.ThrowIfNull(compartment);
        ArgumentNullException.ThrowIfNull(arrow);
        Id = id;
        Kind = kind;
        Name = name;
        NameSynthesized = nameSynthesized;
        Type = type;
        Detail = detail;
        Text = text;
        StandIn = standIn;
        Parent = parent;
        Ports = new ValueList<ViewPort>(ports);
        Origin = origin;
        Geometry = geometry;
        Style = style;
        Compartment = compartment;
        Inherited = inherited;
        Arrow = arrow;
        Parallel = parallel;
    }

    /// <summary>The node's id within the rendering, which an edge and a child's <see cref="Parent"/> name.</summary>
    public string Id { get; }

    /// <summary>What the notation calls the element (<c>part def</c>, <c>state</c>, <c>fork</c>).</summary>
    public string Kind { get; }

    /// <summary>The element's name: qualified for a node the view exposes, simple for one nested in it; empty for an anonymous element.</summary>
    public string Name { get; }

    /// <summary>Whether the model did not give <see cref="Name"/>: a name to key by, not one a picture shows.</summary>
    public bool NameSynthesized { get; }

    /// <summary>The declared type of a typed usage, as the notation writes it after the colon; empty for a definition or an untyped usage.</summary>
    public string Type { get; }

    /// <summary>What else the rendering says about the node, such as a state's <c>initial</c>; empty when there is nothing to add.</summary>
    public string Detail { get; }

    /// <summary>The text heading a node whose name is not shown, in place of its kind: a literal, an accept's event, a send's message.</summary>
    public string Text { get; }

    /// <summary>Whether a migration made the node up, standing for no element of its source, which a positioned drawing may elide.</summary>
    public bool StandIn { get; }

    /// <summary>The <see cref="Id"/> of the node this one is nested in; empty for a root.</summary>
    public string Parent { get; }

    /// <summary>The ports drawn on the node's border, which an edge may end at.</summary>
    public ValueList<ViewPort> Ports { get; }

    /// <summary>Where the element is declared; null when the service located it nowhere.</summary>
    public SourceSpan? Origin { get; }

    /// <summary>Where a layout draws the node; null when no layout annotates it.</summary>
    public ViewGeometry? Geometry { get; }

    /// <summary>How a style draws the node; null when no style colours it.</summary>
    public ViewStyle? Style { get; }

    /// <summary>
    /// The compartment of the <see cref="Parent"/> node's box this node is drawn as a row of (<c>attributes</c>,
    /// <c>parts</c>, <c>ports</c>), as <see cref="RenderViewOptions.Compartments"/> folds them; empty for a node drawn as
    /// a box.
    /// </summary>
    public string Compartment { get; }

    /// <summary>
    /// Whether the owner the node is drawn or listed under has the usage through typing, subclassification or subsetting
    /// rather than declares it, which the notation marks with <c>^</c> before the name. <see cref="Name"/> stays the bare name.
    /// </summary>
    public bool Inherited { get; }

    /// <summary>
    /// For an item box drawn on a connection line (a flow, succession flow or message whose ends are the connection's),
    /// the way the item goes: <c>right</c> toward the line's second end, <c>left</c> toward its first; empty for every other node.
    /// </summary>
    public string Arrow { get; }

    /// <summary>Whether the state's substates run in parallel, which the notation writes as <c>«parallel»</c> under the name.</summary>
    public bool Parallel { get; }
}
