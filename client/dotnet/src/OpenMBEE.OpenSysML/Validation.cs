using System;
using System.Collections.Generic;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.ValidateInstanceAsync"/> answers: one verdict per assertion about the validated object
/// and the objects it holds, in the order they were reached, and the <see cref="Summary"/> about the object as a whole.
/// </summary>
public sealed record Validation
{
    /// <summary>Creates a validation, copying its collections.</summary>
    /// <param name="summary">The verdict about the object as a whole, of kind <c>object</c>.</param>
    /// <param name="verdicts">One verdict per assertion, in the order the objects were reached.</param>
    /// <param name="verifications">What the verification cases verifying a requirement a verdict is about answered.</param>
    /// <param name="instances">Every object reached, the validated one first.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="bounded">Whether nesting deeper than the validation descends was left unvalidated.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Validation(
        Verdict summary,
        IEnumerable<Verdict> verdicts,
        IEnumerable<VerificationVerdict> verifications,
        IEnumerable<Instance> instances,
        IEnumerable<Diagnostic> diagnostics,
        bool bounded)
    {
        ArgumentNullException.ThrowIfNull(summary);
        Summary = summary;
        Verdicts = new ValueList<Verdict>(verdicts);
        Verifications = new ValueList<VerificationVerdict>(verifications);
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Bounded = bounded;
    }

    /// <summary>
    /// The verdict about the object as a whole, of kind <c>object</c>: it holds when every assertion holds and every
    /// held object was reached; undecided, with its error, when an assertion is undecided or nesting was left unreached.
    /// </summary>
    public Verdict Summary { get; }

    /// <summary>One verdict per assertion, each about the object its <see cref="Verdict.InstancePath"/> reaches, in the order the objects were reached.</summary>
    public ValueList<Verdict> Verdicts { get; }

    /// <summary>What the body of every verification case verifying a requirement a verdict is about answered, once per requirement.</summary>
    public ValueList<VerificationVerdict> Verifications { get; }

    /// <summary>Every object reached, the validated one first.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>Whether nesting deeper than the validation descends, or past its budget, was left unvalidated; the summary then decides nothing.</summary>
    public bool Bounded { get; }

    /// <summary>Whether the summary is decided and holds.</summary>
    public bool Holds => Summary.Decided && Summary.Holds;

    /// <summary>The validated object; null when the service reported no object.</summary>
    public Instance? Root => Instances.IsEmpty ? null : Instances[0];

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);
}
