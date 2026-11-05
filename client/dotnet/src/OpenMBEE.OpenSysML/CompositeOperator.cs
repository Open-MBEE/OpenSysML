namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a <see cref="QueryConstraint.Composite"/> combines its constraints, the standard's
/// <c>CompositeConstraint.operator</c>. It has no unset member: a composite always names its operator.
/// </summary>
public enum CompositeOperator
{
    /// <summary><c>and</c>: every constraint holds.</summary>
    And = 1,

    /// <summary><c>or</c>: at least one constraint holds.</summary>
    Or = 2,
}
