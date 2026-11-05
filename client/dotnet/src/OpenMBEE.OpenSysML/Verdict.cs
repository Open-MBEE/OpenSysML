using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One verification's answer: whether a condition held and, when it did not, which condition the model
/// answered false about. False with an empty <see cref="Error"/> is the model's own answer, a decided
/// verdict; false with an error is no answer at all, an undecided one. Neither is an exception.
/// </summary>
public sealed record Verdict
{
    /// <summary>Creates a verdict, copying its witness.</summary>
    /// <param name="kind">What was verified: <c>constraint</c>, <c>requirement</c>, <c>satisfy</c>, <c>objective</c>, <c>assertion</c> or <c>object</c>.</param>
    /// <param name="elementId">The qualified name of the element verified; empty for an anonymous one.</param>
    /// <param name="element">The element as a reader names it.</param>
    /// <param name="holds">Whether the condition holds.</param>
    /// <param name="condition">The condition that evaluated to false, as written; empty when none is named.</param>
    /// <param name="instanceId">The object the verdict is about; 0 when it is about declared values alone.</param>
    /// <param name="instanceTypeId">The qualified name of that object's type; empty when none.</param>
    /// <param name="error">Why nothing was decided; empty for a decided verdict.</param>
    /// <param name="failureReason">What kind of failure <paramref name="error"/> reports.</param>
    /// <param name="requirementId">The requirement a <c>satisfy</c> verdict asserts satisfied; empty otherwise.</param>
    /// <param name="standing">The engine that answered, the strength of its evidence and its bounds.</param>
    /// <param name="instancePath">The path from a validated object to the object this verdict is about; empty otherwise.</param>
    /// <param name="question">The question answered: <c>evaluate</c>, <c>holds</c> or <c>satisfiable</c>; empty when the service named none.</param>
    /// <param name="status">The answer to it: <c>holds</c>, <c>violated</c>, <c>undecided</c>, <c>satisfiable</c> or <c>unsatisfiable</c>; empty when the service named none.</param>
    /// <param name="witness">The assignment witnessing a violated or satisfiable answer, in the query's order.</param>
    /// <param name="notes">The assumptions and omissions the answer rests on, in report order.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">An assignment or a note is null.</exception>
    public Verdict(
        string kind,
        string elementId,
        string element,
        bool holds,
        string condition,
        long instanceId,
        string instanceTypeId,
        string error,
        FailureReason failureReason,
        string requirementId,
        Standing standing,
        string instancePath,
        string question,
        string status,
        IEnumerable<WitnessAssignment> witness,
        IEnumerable<VerdictNote> notes)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(elementId);
        ArgumentNullException.ThrowIfNull(element);
        ArgumentNullException.ThrowIfNull(condition);
        ArgumentNullException.ThrowIfNull(instanceTypeId);
        ArgumentNullException.ThrowIfNull(error);
        ArgumentNullException.ThrowIfNull(requirementId);
        ArgumentNullException.ThrowIfNull(standing);
        ArgumentNullException.ThrowIfNull(instancePath);
        ArgumentNullException.ThrowIfNull(question);
        ArgumentNullException.ThrowIfNull(status);
        Kind = kind;
        ElementId = elementId;
        Element = element;
        Holds = holds;
        Condition = condition;
        InstanceId = instanceId;
        InstanceTypeId = instanceTypeId;
        Error = error;
        FailureReason = failureReason;
        RequirementId = requirementId;
        Standing = standing;
        InstancePath = instancePath;
        Question = question;
        Status = status;
        Witness = new ValueList<WitnessAssignment>(witness);
        Notes = new ValueList<VerdictNote>(notes);
    }

    /// <summary>
    /// What was verified: <c>constraint</c>, <c>requirement</c> or <c>satisfy</c>; for a check an analysis case
    /// made, <c>objective</c> or <c>assertion</c>; for the summary of a validation, <c>object</c>.
    /// </summary>
    public string Kind { get; }

    /// <summary>The qualified name of the element verified; empty for an anonymous satisfy assertion or case assertion.</summary>
    public string ElementId { get; }

    /// <summary>The element as a reader names it: its qualified name, or an anonymous assertion as written (<c>satisfy Range by cruise</c>).</summary>
    public string Element { get; }

    /// <summary>Whether the condition holds; false for an undecided verdict too, which is no answer.</summary>
    public bool Holds { get; }

    /// <summary>The condition that evaluated to false, as written, when the runtime names one; empty otherwise.</summary>
    public string Condition { get; }

    /// <summary>The object the verdict is about, among the answer's instances; 0 when it is about declared values alone.</summary>
    public long InstanceId { get; }

    /// <summary>The qualified name of the type of <see cref="InstanceId"/>; empty when the verdict is about no object.</summary>
    public string InstanceTypeId { get; }

    /// <summary>
    /// Why nothing was decided: unbound features, incommensurable units, an exhausted step budget, an element of
    /// another kind; empty for a decided verdict.
    /// </summary>
    public string Error { get; }

    /// <summary>What kind of failure <see cref="Error"/> reports; <see cref="FailureReason.Unspecified"/> for a decided verdict.</summary>
    public FailureReason FailureReason { get; }

    /// <summary>
    /// The qualified name of the requirement a <c>satisfy</c> verdict asserts satisfied, which the verification
    /// verdicts reported for it name; empty for every other kind.
    /// </summary>
    public string RequirementId { get; }

    /// <summary>The engine that answered, the strength of its evidence and the bounds it ran under.</summary>
    public Standing Standing { get; }

    /// <summary>
    /// For a verdict a validation reports, the path from the validated object to the object this verdict is about
    /// (<c>engine</c>, <c>wheels[2]</c>); empty for the validated object itself and for every other call.
    /// </summary>
    public string InstancePath { get; }

    /// <summary>The question the verdict answers: <c>evaluate</c>, <c>holds</c> or <c>satisfiable</c>; empty when the service named none.</summary>
    public string Question { get; }

    /// <summary>
    /// The answer the question got: <c>holds</c>, <c>violated</c> or <c>undecided</c> for an evaluation or a holds
    /// question, <c>satisfiable</c> or <c>unsatisfiable</c> for a satisfiable one; empty when the service named none.
    /// </summary>
    public string Status { get; }

    /// <summary>The assignment witnessing a violated holds answer or a satisfiable one, in the query's order; empty otherwise.</summary>
    public ValueList<WitnessAssignment> Witness { get; }

    /// <summary>
    /// What a solver's answer rests on, in report order: the features it read as one value or object while only the
    /// default <c>0..*</c> governs them, and the object-holding features whose own assertions it does not state.
    /// Never set for an evaluation, and empty from a service without <see cref="Capabilities.VerdictNotes"/>.
    /// </summary>
    public ValueList<VerdictNote> Notes { get; }

    /// <summary>Whether the model answered: <see cref="Error"/> is empty, so <see cref="Holds"/> is the model's own answer.</summary>
    public bool Decided => Error.Length == 0;

    /// <summary>Whether the model answered false.</summary>
    public bool Violated => Decided && !Holds;
}
