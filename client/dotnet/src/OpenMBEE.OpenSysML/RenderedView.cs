using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A declared view, or a targeted pseudo-view, rendered as machine-readable diagram data: every field the view renderer
/// produced. Nodes are flattened, parents before children, each naming its <see cref="ViewNode.Parent"/>;
/// <see cref="Roots"/> and <see cref="ChildrenOf"/> walk them as the tree they are.
/// </summary>
public sealed record RenderedView
{
    /// <summary>Creates a rendered view, copying its collections.</summary>
    /// <param name="view">The view rendered, by qualified name.</param>
    /// <param name="kind">The rendering produced (<c>interconnection</c>, <c>tree</c>, <c>table</c>).</param>
    /// <param name="stated">How the kind was decided.</param>
    /// <param name="nodes">Every node, parents before children.</param>
    /// <param name="edges">The edges.</param>
    /// <param name="columns">A tabular rendering's columns.</param>
    /// <param name="rows">A tabular rendering's rows.</param>
    /// <param name="canvas">The drawing surface the view states; null for none.</param>
    /// <param name="notes">The note boxes drawn.</param>
    /// <param name="notices">What the rendering could not represent.</param>
    /// <param name="shown">The optional notation elements drawn.</param>
    /// <param name="frame">The view frame drawn round a graph-shaped rendering; null for a frameless one.</param>
    /// <exception cref="ArgumentNullException">An argument other than the canvas or the frame is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public RenderedView(
        string view,
        string kind,
        string stated,
        IEnumerable<ViewNode> nodes,
        IEnumerable<ViewEdge> edges,
        IEnumerable<string> columns,
        IEnumerable<ViewRow> rows,
        ViewCanvas? canvas,
        IEnumerable<ViewNote> notes,
        IEnumerable<string> notices,
        IEnumerable<string> shown,
        ViewFrame? frame)
    {
        ArgumentNullException.ThrowIfNull(view);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(stated);
        View = view;
        Kind = kind;
        Stated = stated;
        Nodes = new ValueList<ViewNode>(nodes);
        Edges = new ValueList<ViewEdge>(edges);
        Columns = new ValueList<string>(columns);
        Rows = new ValueList<ViewRow>(rows);
        Canvas = canvas;
        Notes = new ValueList<ViewNote>(notes);
        Notices = new ValueList<string>(notices);
        Shown = new ValueList<string>(shown);
        Frame = frame;
    }

    /// <summary>The view rendered, by qualified name; empty for a rendering of exposed elements alone.</summary>
    public string View { get; }

    /// <summary>The rendering produced (<c>interconnection</c>, <c>tree</c>, <c>state</c>, <c>action</c>, <c>table</c>, ...), as the service spells it.</summary>
    public string Kind { get; }

    /// <summary>How <see cref="Kind"/> was decided: the rendering the view states, or how it was inferred.</summary>
    public string Stated { get; }

    /// <summary>Every node of the rendering, parents before children.</summary>
    public ValueList<ViewNode> Nodes { get; }

    /// <summary>The edges joining nodes, by <see cref="ViewNode.Id"/>.</summary>
    public ValueList<ViewEdge> Edges { get; }

    /// <summary>A tabular rendering's columns; empty for every other kind.</summary>
    public ValueList<string> Columns { get; }

    /// <summary>A tabular rendering's rows; empty for every other kind.</summary>
    public ValueList<ViewRow> Rows { get; }

    /// <summary>The drawing surface the view states; null when it states none.</summary>
    public ViewCanvas? Canvas { get; }

    /// <summary>The note boxes drawn on the canvas.</summary>
    public ValueList<ViewNote> Notes { get; }

    /// <summary>What the rendering could not represent and, when it has no node, edge or row, why it has none.</summary>
    public ValueList<string> Notices { get; }

    /// <summary>The optional notation elements the rendering draws, in the service's registry order, as <see cref="ViewNotation"/> names them.</summary>
    public ValueList<string> Shown { get; }

    /// <summary>The view frame drawn round a graph-shaped rendering; null for a frameless one, such as a table.</summary>
    public ViewFrame? Frame { get; }

    /// <summary>The nodes nested in no other, in order.</summary>
    public IEnumerable<ViewNode> Roots => Nodes.Where(node => node.Parent.Length == 0);

    /// <summary>The node of an id, or null when the rendering has none.</summary>
    /// <param name="id">A <see cref="ViewNode.Id"/>.</param>
    /// <returns>The node, or null.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="id"/> is null.</exception>
    public ViewNode? Node(string id)
    {
        ArgumentNullException.ThrowIfNull(id);
        return Nodes.FirstOrDefault(node => node.Id == id);
    }

    /// <summary>
    /// The nodes nested directly in a node, in order: the rows of its compartments, which name their
    /// <see cref="ViewNode.Compartment"/>, as well as the boxes drawn inside it.
    /// </summary>
    /// <param name="node">The node.</param>
    /// <returns>Its children.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="node"/> is null.</exception>
    public IEnumerable<ViewNode> ChildrenOf(ViewNode node)
    {
        ArgumentNullException.ThrowIfNull(node);
        return Nodes.Where(child => child.Parent.Length > 0 && child.Parent == node.Id);
    }
}
