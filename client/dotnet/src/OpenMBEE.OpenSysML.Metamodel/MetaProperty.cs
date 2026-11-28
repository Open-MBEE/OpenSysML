using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// One property of the SysML metamodel, as its declaring metaclass states it: the JSON key a document
/// writes it under, the C# property generated for it, and its range and multiplicity.
/// </summary>
public sealed class MetaProperty
{
    private IReadOnlyList<MetaProperty>? redefines;
    private IReadOnlyList<MetaProperty>? subsets;
    private MetaProperty? opposite;
    private bool linked;

    internal MetaProperty(
        MetaclassInfo declaringMetaclass,
        string jsonKey,
        string name,
        PropertyRangeKind rangeKind,
        MetaclassInfo? rangeMetaclass,
        EnumerationInfo? rangeEnumeration,
        Type? primitiveType,
        bool isMany,
        bool isOrdered,
        bool isUnique,
        bool isDerived,
        int lowerBound,
        string? defaultLiteral)
    {
        if ((rangeKind == PropertyRangeKind.Metaclass) != (rangeMetaclass is not null)
            || (rangeKind == PropertyRangeKind.Enumeration) != (rangeEnumeration is not null)
            || (rangeKind == PropertyRangeKind.Primitive) != (primitiveType is not null)
            || (primitiveType is not null && primitiveType != typeof(bool) && primitiveType != typeof(string)
                && primitiveType != typeof(int) && primitiveType != typeof(double)))
        {
            throw new ArgumentException($"{declaringMetaclass.Name}::{jsonKey}: the range does not match its kind");
        }

        DeclaringMetaclass = declaringMetaclass;
        JsonKey = jsonKey;
        Name = name;
        RangeKind = rangeKind;
        RangeMetaclass = rangeMetaclass;
        RangeEnumeration = rangeEnumeration;
        IsMany = isMany;
        IsOrdered = isOrdered;
        IsUnique = isUnique;
        IsDerived = isDerived;
        LowerBound = lowerBound;
        DefaultLiteral = defaultLiteral;
        ValueType = rangeKind switch
        {
            PropertyRangeKind.Metaclass => rangeMetaclass!.InterfaceType,
            PropertyRangeKind.Enumeration => rangeEnumeration!.EnumType,
            PropertyRangeKind.Primitive => primitiveType!,
            _ => throw new ArgumentOutOfRangeException(nameof(rangeKind), rangeKind, null),
        };
    }

    /// <summary>The metaclass that declares the property.</summary>
    public MetaclassInfo DeclaringMetaclass { get; }

    /// <summary>The key a document writes the property under, for example <c>ownedFeature</c>.</summary>
    public string JsonKey { get; }

    /// <summary>The name of the generated C# property, for example <c>OwnedFeature</c>.</summary>
    public string Name { get; }

    /// <summary>The property as the metamodel names it, for example <c>Type::ownedFeature</c>.</summary>
    public string QualifiedName => $"{DeclaringMetaclass.Name}::{JsonKey}";

    /// <summary>The property's IRI in the SysML namespace, for example <c>https://www.omg.org/spec/SysML#Type_ownedFeature</c>.</summary>
    public string Iri => $"https://www.omg.org/spec/SysML#{DeclaringMetaclass.Name}_{JsonKey}";

    /// <summary>What the property's values are.</summary>
    public PropertyRangeKind RangeKind { get; }

    /// <summary>The metaclass the property refers to, when <see cref="RangeKind"/> is <see cref="PropertyRangeKind.Metaclass"/>.</summary>
    public MetaclassInfo? RangeMetaclass { get; }

    /// <summary>The enumeration of the property's literals, when <see cref="RangeKind"/> is <see cref="PropertyRangeKind.Enumeration"/>.</summary>
    public EnumerationInfo? RangeEnumeration { get; }

    /// <summary>
    /// The C# type of one value: the range metaclass's interface, the enumeration's enum, or
    /// <see cref="bool"/>, <see cref="string"/>, <see cref="int"/> or <see cref="double"/>.
    /// </summary>
    public Type ValueType { get; }

    /// <summary>Whether the property holds any number of values, read as a list, rather than at most one.</summary>
    public bool IsMany { get; }

    /// <summary>Whether a many-valued property's values are ordered; false for a single-valued one.</summary>
    public bool IsOrdered { get; }

    /// <summary>Whether a many-valued property's values are distinct.</summary>
    public bool IsUnique { get; }

    /// <summary>Whether the metamodel derives the property rather than an element owning it; an export of owned properties omits it.</summary>
    public bool IsDerived { get; }

    /// <summary>The lower bound of the property's multiplicity.</summary>
    public int LowerBound { get; }

    /// <summary>The metamodel's default literal, for example <c>false</c> or <c>public</c>; null when it states none. Nothing applies it to a document that omits the property.</summary>
    public string? DefaultLiteral { get; }

    /// <summary>The properties this one redefines.</summary>
    public IReadOnlyList<MetaProperty> Redefines => linked ? redefines! : throw Unlinked();

    /// <summary>The properties this one subsets.</summary>
    public IReadOnlyList<MetaProperty> Subsets => linked ? subsets! : throw Unlinked();

    /// <summary>The navigable property at the other end of the same association, when there is one.</summary>
    public MetaProperty? Opposite => linked ? opposite : throw Unlinked();

    /// <summary>The qualified name.</summary>
    /// <returns><see cref="QualifiedName"/>.</returns>
    public override string ToString() => QualifiedName;

    /// <summary>Records the properties this one names, once every property exists.</summary>
    internal void Link(MetaProperty[] redefined, MetaProperty[] subsetted, MetaProperty? oppositeEnd)
    {
        if (linked)
        {
            throw new InvalidOperationException($"{QualifiedName} is linked already");
        }

        redefines = Array.AsReadOnly<MetaProperty>([.. redefined]);
        subsets = Array.AsReadOnly<MetaProperty>([.. subsetted]);
        opposite = oppositeEnd;
        linked = true;
    }

    private InvalidOperationException Unlinked() => new($"{QualifiedName} is not linked");
}
