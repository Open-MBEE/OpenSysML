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

    /// <summary>
    /// The form to also write the rendering in, into <see cref="RenderedView.Text"/>, as <c>-render-form</c> names it:
    /// <c>text</c>, <c>mermaid</c>, <c>markdown</c>, <c>dot</c>, <c>plantuml</c>, <c>d2</c>, <c>drawio</c>, <c>csv</c> or
    /// <c>tsv</c>; null writes none. The service refuses a form there is none of, or one the view's kind is not written in
    /// (<c>dot</c> of a table). It and every writer option below need <see cref="Capabilities.RenderViewForm"/>.
    /// </summary>
    public string? Form { get; init; }

    /// <summary>The palette a graph form fills nodes from, as <c>-render-palette</c> names them; null draws in black and white.</summary>
    public string? Palette { get; init; }

    /// <summary>The drawing style of the DOT or Mermaid form, as <c>-render-style</c> names them: <c>pilot</c> (the default) or <c>cameo</c>.</summary>
    public string? Style { get; init; }

    /// <summary>How the DOT or PlantUML form routes its edges, as <c>-render-lines</c> names them: <c>splines</c> (the default), <c>ortho</c> or <c>polyline</c>.</summary>
    public string? Lines { get; init; }

    /// <summary>The flow direction of a graph-shaped form: <c>TB</c>, <c>LR</c>, <c>RL</c> or <c>BT</c>; null leaves each kind's default, which is what the CLI writes.</summary>
    public string? Direction { get; init; }

    /// <summary>
    /// The source link template each node and edge of the form is written with, as <c>-render-link</c> takes it, with the
    /// placeholders <c>{file}</c>, <c>{line}</c>, <c>{col}</c>, <c>{qname}</c> and <c>{id}</c>; null writes no links.
    /// </summary>
    public string? Link { get; init; }

    /// <summary>
    /// Where a graph form of a view some Layout positions puts the nodes none does, as <c>-render-unplaced</c> names it:
    /// <c>omit</c> (the default) or <c>strip</c>.
    /// </summary>
    public string? Unplaced { get; init; }

    /// <summary>Whether a form or a writer option is asked for, which <see cref="Capabilities.RenderViewForm"/> gates as one.</summary>
    internal bool AsksForm =>
        !string.IsNullOrEmpty(Form) || !string.IsNullOrEmpty(Palette) || !string.IsNullOrEmpty(Style) || !string.IsNullOrEmpty(Lines)
        || !string.IsNullOrEmpty(Direction) || !string.IsNullOrEmpty(Link) || !string.IsNullOrEmpty(Unplaced);
}
