using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One generalization edge an element declares.</summary>
public sealed record Specialization
{
    /// <summary>Creates a specialization.</summary>
    /// <param name="kind">The relationship: <c>specializes</c>, <c>subsets</c>, <c>redefines</c> or <c>typing</c>.</param>
    /// <param name="declared">The target as written in the source (<c>Engine</c>, <c>Demo::Engine</c>).</param>
    /// <param name="targetId">The qualified name of the resolved target; empty when the name does not resolve.</param>
    /// <param name="targetKind">The symbol kind of the resolved target (<c>partDef</c>, …); empty when unresolved.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public Specialization(string kind, string declared, string targetId, string targetKind)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(declared);
        ArgumentNullException.ThrowIfNull(targetId);
        ArgumentNullException.ThrowIfNull(targetKind);
        Kind = kind;
        Declared = declared;
        TargetId = targetId;
        TargetKind = targetKind;
    }

    /// <summary>The relationship: <c>specializes</c>, <c>subsets</c>, <c>redefines</c> or <c>typing</c>.</summary>
    public string Kind { get; }

    /// <summary>The target as written in the source (<c>Engine</c>, <c>Demo::Engine</c>).</summary>
    public string Declared { get; }

    /// <summary>The qualified name of the resolved target; empty when the name does not resolve.</summary>
    public string TargetId { get; }

    /// <summary>The symbol kind of the resolved target (<c>partDef</c>, …); empty when unresolved.</summary>
    public string TargetKind { get; }
}
