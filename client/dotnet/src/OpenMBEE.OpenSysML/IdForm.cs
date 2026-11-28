namespace OpenMBEE.OpenSysML;

/// <summary>How derived element ids are spelled when notation is written as a graph, as <c>sysml -id</c> spells them.</summary>
public enum IdForm
{
    /// <summary>Each derived from its qualified name, the default.</summary>
    Qualified,

    /// <summary>Name-based UUIDs under each root package, the library convention.</summary>
    Uuid,
}
