using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One generalization edge an element declares, as written: the relationships <see cref="Model.ElementPropertiesAsync"/>
/// reflects under <c>ownedSpecialization</c>, without the implied ones the metamodel adds.
/// </summary>
public sealed record Specialization
{
    /// <summary>Creates a specialization.</summary>
    /// <param name="kind">The relationship: <c>specializes</c>, <c>subsets</c>, <c>redefines</c>, <c>typing</c> or <c>references</c> (reference subsetting, <c>::&gt;</c> and <c>include</c>).</param>
    /// <param name="declared">The target as written in the source (<c>Engine</c>, <c>Demo::Engine</c>).</param>
    /// <param name="targetId">The element id of the resolved target; empty when the name does not resolve.</param>
    /// <param name="targetKind">The symbol kind of the resolved target (<c>partDef</c>, …); empty when unresolved.</param>
    /// <param name="targetQualifiedName">The qualified name of the resolved target; empty when unresolved, or from a service that predates it.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    internal Specialization(string kind, string declared, string targetId, string targetKind, string targetQualifiedName = "")
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(declared);
        ArgumentNullException.ThrowIfNull(targetId);
        ArgumentNullException.ThrowIfNull(targetKind);
        ArgumentNullException.ThrowIfNull(targetQualifiedName);
        Kind = kind;
        Declared = declared;
        TargetId = targetId;
        TargetKind = targetKind;
        TargetQualifiedName = targetQualifiedName;
    }

    /// <summary>The relationship: <c>specializes</c>, <c>subsets</c>, <c>redefines</c>, <c>typing</c> or <c>references</c> (reference subsetting, <c>::&gt;</c> and <c>include</c>).</summary>
    public string Kind { get; }

    /// <summary>The target as written in the source (<c>Engine</c>, <c>Demo::Engine</c>).</summary>
    public string Declared { get; }

    /// <summary>
    /// The element id of the resolved target, as <see cref="Symbol.Id"/> spells it, for a lookup by <see cref="Lookup.ElementId"/>;
    /// empty when the name does not resolve. A <c>references</c> feature chain (<c>x.p</c>) is one edge per chaining feature.
    /// </summary>
    public string TargetId { get; }

    /// <summary>The symbol kind of the resolved target (<c>partDef</c>, …); empty when unresolved.</summary>
    public string TargetKind { get; }

    /// <summary>The qualified name of the resolved target, for display; empty when unresolved, or from a service that predates it.</summary>
    public string TargetQualifiedName { get; }
}
