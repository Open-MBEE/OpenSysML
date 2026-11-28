namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a <see cref="QueryConstraint.Comparison"/> compares a property against its values, the standard's
/// <c>PrimitiveConstraint.operator</c>. It has no unset member: a comparison always names its operator.
/// </summary>
public enum ComparisonOperator
{
    /// <summary><c>=</c>: the property equals any of the values.</summary>
    Equal = 1,

    /// <summary><c>&gt;</c>: the property, which must be ordered, is greater than the one value.</summary>
    Greater = 2,

    /// <summary><c>&lt;</c>: the property, which must be ordered, is less than the one value.</summary>
    Less = 3,

    /// <summary><c>&lt;=</c>: the property, which must be ordered, is at most the one value.</summary>
    LessOrEqual = 4,

    /// <summary><c>&gt;=</c>: the property, which must be ordered, is at least the one value.</summary>
    GreaterOrEqual = 5,

    /// <summary><c>in</c>: the property's value is one of the values.</summary>
    In = 6,

    /// <summary>
    /// <c>instanceOf</c>: the element's metaclass, which only the property <c>@type</c> names, conforms to one of the
    /// metaclasses the values name, so a <c>PartUsage</c> is an instance of <c>Usage</c>, <c>Feature</c> and
    /// <c>Element</c>.
    /// </summary>
    InstanceOf = 7,
}
