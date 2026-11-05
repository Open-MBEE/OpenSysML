using System;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.VerifySatisfactionAsync"/> answers: one verdict per satisfaction assertion evaluated,
/// in declaration order, what the verification cases verifying each requirement asserted answered, and the
/// objects the verdicts are about. A model stating no assertion answers no verdict.
/// </summary>
public sealed record Satisfaction
{
    /// <summary>Creates a satisfaction, copying its collections.</summary>
    /// <param name="verdicts">One verdict per assertion, in declaration order.</param>
    /// <param name="verifications">What the verification cases verifying an asserted requirement answered.</param>
    /// <param name="instances">Every object a verdict is about, each once.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Satisfaction(IEnumerable<Verdict> verdicts, IEnumerable<VerificationVerdict> verifications, IEnumerable<Instance> instances, IEnumerable<Diagnostic> diagnostics)
    {
        Verdicts = new ValueList<Verdict>(verdicts);
        Verifications = new ValueList<VerificationVerdict>(verifications);
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>One verdict per assertion evaluated, in declaration order.</summary>
    public ValueList<Verdict> Verdicts { get; }

    /// <summary>
    /// What the body of every verification case verifying an asserted requirement answered, in the order the
    /// assertions were evaluated; each names its requirement, as the verdicts of that requirement's assertions do.
    /// </summary>
    public ValueList<VerificationVerdict> Verifications { get; }

    /// <summary>Every object a verdict is about, each once.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>Whether every verdict is decided and holds; true when there is none, so read it beside <see cref="Verdicts"/> where that matters.</summary>
    public bool Holds => Verdicts.All(verdict => verdict.Decided && verdict.Holds);

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);
}
