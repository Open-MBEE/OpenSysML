using System.Collections.Generic;
using System.Linq;
using Google.Protobuf.Collections;

namespace OpenMBEE.OpenSysML.Internal.Mapping;

/// <summary>
/// Reads the execution and check answers into the client's immutable records. Every field is read
/// whatever the service advertises; a value is read by <see cref="ValueWire"/> and a trace record by
/// <see cref="DocumentWire"/>, each naming where in the answer it is when it is malformed.
/// </summary>
internal static class ExecutionWire
{
    /// <summary>Reads one run of an action.</summary>
    internal static ActionRun Run(Wire.ExecuteActionResponse response, string method) =>
        new(
            ValueWire.FromWire(response.Outputs, method, "outputs"),
            ModelWire.Diagnostics(response.Diagnostics),
            response.FinalTime,
            ValueWire.FromWire(response.PerformerAttributes, method, "performer_attributes"),
            response.OrderDependent,
            Trace(response.Trace, method),
            response.TraceDropped);

    /// <summary>Reads one run of a state machine.</summary>
    internal static StateRun Run(Wire.ExecuteStateResponse response, string method) =>
        new(
            response.StatesVisited,
            ValueWire.FromWire(response.FinalContext, method, "final_context"),
            ModelWire.Diagnostics(response.Diagnostics),
            response.FinalTime,
            Trace(response.Trace, method),
            response.TraceDropped,
            response.OrderDependent);

    /// <summary>Reads a run's trace, in the order the run made it.</summary>
    internal static List<DocumentEvent> Trace(RepeatedField<Wire.DocumentEvent> trace, string method) =>
        [.. trace.Select((record, i) => DocumentWire.Event(record, method, $"trace[{i}]"))];

    /// <summary>Reads an exploration: its outcomes, how it ended and the standing of the engine that answered.</summary>
    internal static Exploration Exploration(RepeatedField<Wire.Outcome> outcomes, Wire.ExplorationStatus status, Standing standing, string method) =>
        new(
            outcomes.Select((outcome, i) => Outcome(outcome, method, $"outcomes[{i}]")),
            status.Complete,
            status.Runs,
            status.BudgetsHit,
            status.RunsBudget,
            status.DepthBudget,
            status.ProbabilitiesLowerBound,
            status.FailedLinearizations,
            status.Unenumerated,
            Coverage(status.Coverage),
            standing);

    /// <summary>Reads what a check found; the failure an answer reports is the caller's to raise.</summary>
    internal static CheckResult Check(Wire.CheckBehaviorResponse response) =>
        new(
            response.Verdict,
            response.Lines,
            response.Search is { } search
                ? new CheckSearch(
                    search.States,
                    search.Moves,
                    search.MaxDepth,
                    search.BoundsHit,
                    search.Horizon,
                    search.HorizonSet,
                    search.MassBounded,
                    search.Notes,
                    search.Scope,
                    search.Status,
                    Coverage(search.Coverage))
                : null,
            response.Violations.Select(violation => new CheckViolation(
                violation.Kind,
                violation.Property,
                violation.Error,
                violation.Depth,
                violation.Probability,
                Witness(violation.Witness),
                violation.Text)),
            response.Divergences.Select(divergence => new CheckDivergence(
                divergence.Feature,
                divergence.Values.Select(value => new CheckDivergentValue(value.Value, Witness(value.Witness))))),
            response.Finals.Select(final => new CheckFinal(final.Outcome, final.Values, Witness(final.Witness))),
            response.Claim,
            response.FreeInputs,
            response.Assumptions,
            Witness(response.Witness),
            Witness(response.Contrast),
            StandingOf(response.Engine, response.Strength, response.Bounds),
            response.Reason);

    /// <summary>Reads the engine an answer names, the strength of its evidence and its bounds.</summary>
    internal static Standing StandingOf(string engine, string strength, RepeatedField<Wire.Bound> bounds) =>
        engine.Length == 0 && strength.Length == 0 && bounds.Count == 0
            ? Standing.None
            : new(engine, strength, bounds.Select(bound => new Bound(bound.Name, bound.Limit, bound.Reached)));

    private static Outcome Outcome(Wire.Outcome outcome, string method, string path) =>
        new(
            ValueWire.FromWire(outcome.Outputs, method, path + ".outputs"),
            outcome.FinalState,
            outcome.StatesVisited,
            outcome.Error,
            outcome.Linearizations,
            outcome.Witness,
            ModelWire.Diagnostics(outcome.Diagnostics),
            outcome.Probability,
            outcome.ProbabilityRange is { } range ? new ProbabilityRange(range.Min, range.Max) : null);

    private static Coverage? Coverage(Wire.Coverage? coverage) =>
        coverage is null ? null : new Coverage(coverage.States.Select(Entry), coverage.Transitions.Select(Entry));

    private static CoverageEntry Entry(Wire.CoverageEntry entry) => new(entry.Path, entry.Declaration, entry.Runs);

    private static CheckWitness? Witness(Wire.CheckWitness? witness) =>
        witness is null ? null : new CheckWitness(witness.Schedule, witness.Draws, witness.Choices, witness.Inputs, witness.FileName, witness.FileContent);
}
