namespace OpenMBEE.OpenSysML;

/// <summary>
/// The optional notation elements a view rendering can draw besides those its kind of rendering shows, as
/// <see cref="RenderViewOptions.Show"/> names them and <see cref="RenderedView.Shown"/> reports them. A name the
/// service does not know is refused by it with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public static class ViewNotation
{
    /// <summary>Every optional notation element.</summary>
    public const string All = "all";

    /// <summary>The compartments under a node's name: the lines its label carries after its title.</summary>
    public const string Compartments = "compartments";

    /// <summary>The names connections, interfaces, flows, bindings and allocations are labelled with.</summary>
    public const string EdgeLabels = "edge-labels";

    /// <summary>The «keyword» lines heading a node.</summary>
    public const string Keywords = "keywords";

    /// <summary>The <c>[n]</c> multiplicities after a usage's name.</summary>
    public const string Multiplicities = "multiplicities";

    /// <summary>
    /// The symbols on a node's boundary standing for its parameters: a case's actor, subject and stakeholder
    /// parameters and an action's pins. Left out, a connection that ended at one ends at the node declaring it.
    /// </summary>
    public const string ParameterSymbols = "parameter-symbols";

    /// <summary>The port symbols on a part's boundary. Left out, a connection that ended at a port ends at the part owning it.</summary>
    public const string PortSymbols = "port-symbols";

    /// <summary>The qualifying prefixes of a node's name.</summary>
    public const string Qualifiers = "qualifiers";

    /// <summary>The <c>: Type</c> texts after a usage's name and a port's.</summary>
    public const string Typings = "typings";
}
