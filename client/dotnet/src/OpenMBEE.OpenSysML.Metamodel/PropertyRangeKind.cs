namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>What a <see cref="MetaProperty"/>'s values are.</summary>
public enum PropertyRangeKind
{
    /// <summary>References to elements of a metaclass, <see cref="MetaProperty.RangeMetaclass"/>.</summary>
    Metaclass,

    /// <summary>Literals of an enumeration, <see cref="MetaProperty.RangeEnumeration"/>.</summary>
    Enumeration,

    /// <summary>
    /// Values of a primitive datatype, read as the <see cref="MetaProperty.ValueType"/> it maps to:
    /// <see cref="bool"/> for <c>xsd:boolean</c>, <see cref="string"/> for <c>xsd:string</c>,
    /// <see cref="int"/> for <c>xsd:int</c> and a finite <see cref="double"/> for <c>owl:real</c>.
    /// </summary>
    Primitive,
}
