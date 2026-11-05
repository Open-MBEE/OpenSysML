using System;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.RunAnalysisAsync"/> answers: what one run of an analysis case computed and what its
/// objective and each assertion in its body decided, the verification verdicts of a verification case, the
/// applications it made of its own calcs, and the objects they name. A tool dry run answers the
/// <see cref="ToolDryRun"/> preview instead, with nothing of the run kept.
/// </summary>
public sealed record Analysis
{
    /// <summary>Creates an analysis, copying its collections.</summary>
    /// <param name="outputs">The case's out and return parameters, in declaration order.</param>
    /// <param name="verdicts">What the objective and each assertion decided, the objective first.</param>
    /// <param name="verifications">What a verification case's body answered: its own verdict, then each subcase's.</param>
    /// <param name="evaluations">Each application the run made of one of the case's calcs as a function value.</param>
    /// <param name="instances">The objects reachable from the subject and named by an output or evaluation.</param>
    /// <param name="diagnostics">What the run noted about itself.</param>
    /// <param name="standing">The engine that answered, the strength of its evidence and its bounds.</param>
    /// <param name="toolDryRun">What an external tool would have been given; null unless a dry run reached one.</param>
    /// <exception cref="ArgumentNullException">A collection or <paramref name="standing"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Analysis(
        IEnumerable<CalcOutput> outputs,
        IEnumerable<Verdict> verdicts,
        IEnumerable<VerificationVerdict> verifications,
        IEnumerable<CaseEvaluation> evaluations,
        IEnumerable<Instance> instances,
        IEnumerable<Diagnostic> diagnostics,
        Standing standing,
        ToolDryRun? toolDryRun)
    {
        ArgumentNullException.ThrowIfNull(standing);
        Outputs = new ValueList<CalcOutput>(outputs);
        Verdicts = new ValueList<Verdict>(verdicts);
        Verifications = new ValueList<VerificationVerdict>(verifications);
        Evaluations = new ValueList<CaseEvaluation>(evaluations);
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Standing = standing;
        ToolDryRun = toolDryRun;
    }

    /// <summary>The case's out and return parameters, in declaration order; a value returned into an unnamed result is named <c>result</c>.</summary>
    public ValueList<CalcOutput> Outputs { get; }

    /// <summary>What the case's objective and each assertion in its body decided, the objective first: kind <c>objective</c> or <c>assertion</c>.</summary>
    public ValueList<Verdict> Verdicts { get; }

    /// <summary>What the body answered when the case is a verification case: its own verdict first, then each subcase's.</summary>
    public ValueList<VerificationVerdict> Verifications { get; }

    /// <summary>Each application the run made of one of the case's own calcs as a function value, in the order first made, the selected one marked.</summary>
    public ValueList<CaseEvaluation> Evaluations { get; }

    /// <summary>The objects reachable from the subject the case ran on, and those an output or an evaluation names.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the run noted about itself: its choice points and the guards it could not evaluate.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The engine that answered, the strength of its evidence and the bounds it ran under.</summary>
    public Standing Standing { get; }

    /// <summary>What the external tool the run reached would have been given; null unless <see cref="AnalysisOptions.ToolDryRun"/> asked and one was reached.</summary>
    public ToolDryRun? ToolDryRun { get; }

    /// <summary>Whether every verdict is decided and holds; true for a case stating none.</summary>
    public bool Holds => Verdicts.All(verdict => verdict.Decided && verdict.Holds);

    /// <summary>The objective's verdict; null for a case with no objective.</summary>
    public Verdict? Objective => Verdicts.FirstOrDefault(verdict => verdict.Kind == "objective");

    /// <summary>The evaluation of the alternative the case selected; null when it selected none.</summary>
    public CaseEvaluation? Selected => Evaluations.FirstOrDefault(evaluation => evaluation.Selected);

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);

    /// <summary>The object a reference refers to.</summary>
    /// <param name="reference">A reference among this analysis's values.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
    public Instance? Resolve(Value.InstanceRefValue reference)
    {
        ArgumentNullException.ThrowIfNull(reference);
        return FindInstance(reference.InstanceId);
    }
}
