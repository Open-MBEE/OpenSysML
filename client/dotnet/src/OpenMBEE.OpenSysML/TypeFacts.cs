using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The static type of a usage, or the classification of a definition, as far as the service derives
/// it without running the model. A fact the service does not know is empty.
/// </summary>
public sealed record TypeFacts
{
    /// <summary>Creates type facts.</summary>
    /// <param name="declared">The type name as written; empty when none is declared.</param>
    /// <param name="resolvedId">The qualified name of the resolved type; empty when unresolved or undeclared.</param>
    /// <param name="resolvedKind">The symbol kind of the resolved type (<c>partDef</c>, <c>attributeDef</c>, …).</param>
    /// <param name="primitive">The library scalar the type reduces to (<c>Boolean</c>, <c>Integer</c>, <c>Real</c>, …); empty when none.</param>
    /// <param name="primitiveSource">Where <paramref name="primitive"/> came from: <c>declared</c>, <c>value</c>, or empty.</param>
    /// <param name="isQuantity">Whether values of the type carry a measurement unit, so a feature value holds a quantity.</param>
    /// <param name="unit">The unit as written by the default value (<c>m/s</c>); empty when it names none.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public TypeFacts(string declared, string resolvedId, string resolvedKind, string primitive, string primitiveSource, bool isQuantity, string unit)
    {
        ArgumentNullException.ThrowIfNull(declared);
        ArgumentNullException.ThrowIfNull(resolvedId);
        ArgumentNullException.ThrowIfNull(resolvedKind);
        ArgumentNullException.ThrowIfNull(primitive);
        ArgumentNullException.ThrowIfNull(primitiveSource);
        ArgumentNullException.ThrowIfNull(unit);
        Declared = declared;
        ResolvedId = resolvedId;
        ResolvedKind = resolvedKind;
        Primitive = primitive;
        PrimitiveSource = primitiveSource;
        IsQuantity = isQuantity;
        Unit = unit;
    }

    /// <summary>The type name as written; empty when none is declared.</summary>
    public string Declared { get; }

    /// <summary>The qualified name of the resolved type; empty when unresolved or undeclared.</summary>
    public string ResolvedId { get; }

    /// <summary>The symbol kind of the resolved type (<c>partDef</c>, <c>attributeDef</c>, …); empty when unresolved.</summary>
    public string ResolvedKind { get; }

    /// <summary>The library scalar the type reduces to (<c>Boolean</c>, <c>Integer</c>, <c>Real</c>, …); empty when it is not a scalar value type.</summary>
    public string Primitive { get; }

    /// <summary>Where <see cref="Primitive"/> came from: <c>declared</c> from the declared or inherited type, <c>value</c> from the default value, or empty.</summary>
    public string PrimitiveSource { get; }

    /// <summary>Whether values of the type carry a measurement unit, so a feature value holds a quantity.</summary>
    public bool IsQuantity { get; }

    /// <summary>The unit as written by the default value (<c>m/s</c>); empty when it names none.</summary>
    public string Unit { get; }
}
