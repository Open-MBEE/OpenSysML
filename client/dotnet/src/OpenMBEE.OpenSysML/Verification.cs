using System;
using System.Collections.Generic;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.VerifyConstraintAsync"/> and <see cref="Model.VerifyRequirementAsync"/> answer: the
/// verdict, what the verification cases verifying a requirement answered beside it, and the objects it is about.
/// </summary>
public sealed record Verification
{
    /// <summary>Creates a verification, copying its collections.</summary>
    /// <param name="verdict">The verdict.</param>
    /// <param name="verifications">What the verification cases verifying a requirement answered.</param>
    /// <param name="instances">The objects reachable from the verdict's subject, the subject among them.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Verification(Verdict verdict, IEnumerable<VerificationVerdict> verifications, IEnumerable<Instance> instances, IEnumerable<Diagnostic> diagnostics)
    {
        ArgumentNullException.ThrowIfNull(verdict);
        Verdict = verdict;
        Verifications = new ValueList<VerificationVerdict>(verifications);
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>The verdict.</summary>
    public Verdict Verdict { get; }

    /// <summary>What the body of every verification case verifying the requirement answered; empty for a constraint.</summary>
    public ValueList<VerificationVerdict> Verifications { get; }

    /// <summary>The objects reachable from the verdict's subject, the subject among them; empty when the verdict is about none.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>Whether the verdict is decided and holds.</summary>
    public bool Holds => Verdict.Decided && Verdict.Holds;

    /// <summary>The object the verdict is about; null when it is about declared values alone.</summary>
    public Instance? Subject => InstanceSearch.Find(Instances, Verdict.InstanceId);

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);
}
