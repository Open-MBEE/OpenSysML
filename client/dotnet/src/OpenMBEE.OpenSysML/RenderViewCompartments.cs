namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a tree rendering folds a node's owned members into compartment rows of its box, as <c>sysml -compartments</c>
/// chooses them. A row is a <see cref="ViewNode"/> whose <see cref="ViewNode.Compartment"/> names the compartment.
/// </summary>
public enum RenderViewCompartments
{
    /// <summary>Only the members a Layout leaves without a position, of a tree some Layout positions; the default.</summary>
    Positioned,

    /// <summary>Every owned usage that no Layout, nested definition or package keeps as a node of its own.</summary>
    Most,

    /// <summary>The port usages, with the ports nested under them.</summary>
    Ports,
}
