using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What the body of a verification case answered when it ran: the <c>VerdictKind</c> its return bound,
/// which is a separate answer from whether a requirement is satisfied. A service without
/// <see cref="Capabilities.VerificationVerdicts"/> reports none.
/// </summary>
public sealed record VerificationVerdict
{
    /// <summary>Creates a verification verdict.</summary>
    /// <param name="caseId">The qualified name of the verification case that ran.</param>
    /// <param name="kind"><c>pass</c>, <c>fail</c>, <c>inconclusive</c> or <c>error</c>.</param>
    /// <param name="detail">Why the body decided nothing, or the error that stopped it; empty for a pass or a fail.</param>
    /// <param name="subcase">Whether this is the verdict of a subcase the case performed.</param>
    /// <param name="requirementId">The requirement this verdict was reported for; empty when none.</param>
    /// <param name="instanceId">The object the case ran on as its owner; 0 when it ran on none.</param>
    /// <param name="instanceTypeId">The qualified name of that object's type; empty when none.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public VerificationVerdict(string caseId, string kind, string detail, bool subcase, string requirementId, long instanceId, string instanceTypeId)
    {
        ArgumentNullException.ThrowIfNull(caseId);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(detail);
        ArgumentNullException.ThrowIfNull(requirementId);
        ArgumentNullException.ThrowIfNull(instanceTypeId);
        CaseId = caseId;
        Kind = kind;
        Detail = detail;
        Subcase = subcase;
        RequirementId = requirementId;
        InstanceId = instanceId;
        InstanceTypeId = instanceTypeId;
    }

    /// <summary>The qualified name of the verification case definition or usage that ran.</summary>
    public string CaseId { get; }

    /// <summary>The <c>VerdictKind</c> the body produced: <c>pass</c>, <c>fail</c>, <c>inconclusive</c> or <c>error</c>.</summary>
    public string Kind { get; }

    /// <summary>Why the body decided nothing, or the text of the error that stopped its run; empty for a pass or a fail.</summary>
    public string Detail { get; }

    /// <summary>Whether this is the verdict of a subcase the case performed, which the library rolls up into no other.</summary>
    public bool Subcase { get; }

    /// <summary>
    /// The qualified name of the requirement this verdict was reported for, the one the case's objective
    /// verifies; empty when the case ran for itself or the requirement has no qualified name.
    /// </summary>
    public string RequirementId { get; }

    /// <summary>The object the case ran on as its owner, among the answer's instances; 0 when it ran on none.</summary>
    public long InstanceId { get; }

    /// <summary>The qualified name of the type of <see cref="InstanceId"/>; empty when it ran on none.</summary>
    public string InstanceTypeId { get; }

    /// <summary>Whether the body answered <c>pass</c>.</summary>
    public bool Passed => Kind == "pass";
}
