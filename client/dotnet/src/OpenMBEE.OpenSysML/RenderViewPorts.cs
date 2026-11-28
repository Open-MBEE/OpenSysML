namespace OpenMBEE.OpenSysML;

/// <summary>Which ports a view rendering draws on its nodes' borders, as <c>sysml -ports</c> chooses them.</summary>
public enum RenderViewPorts
{
    /// <summary>The ports the rendering connects or the view exposes, the default.</summary>
    Minimal,

    /// <summary>Every port each drawn node has.</summary>
    Full,
}
