using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Answers that set every field the execution and check responses can reach: one run of an action and
/// of a state machine, an exploration of each, a check that found everything, each failure in band, and
/// document values of every arm as a trace's instants.
/// </summary>
internal static class CompleteExecutionWire
{
    /// <summary>An element value naming an element.</summary>
    internal static Wire.DocumentValue Element { get; } = new() { ElementId = "Garage::car", ElementType = "PartUsage" };

    /// <summary>An object with the usage it is held under.</summary>
    internal static Wire.DocumentObject Object { get; } = new() { InstanceId = 2, Path = "Garage::car", Element = Element };

    /// <summary>A verdict row with every fact set.</summary>
    internal static Wire.DocumentVerdict Verdict { get; } = new()
    {
        Assertion = new Wire.DocumentValue { ElementId = "Garage::massKnown", ElementType = "ConstraintUsage" },
        Kind = "constraint",
        Text = "assert constraint massKnown",
        Path = "Garage::car",
        Verdict = "violated",
        Condition = "mass > 0",
        Reason = "mass is 0",
        Verification = { "fail", "pass" },
    };

    /// <summary>A state row with every fact set.</summary>
    internal static Wire.DocumentState State { get; } = new()
    {
        Object = Object,
        Machine = "lp",
        Name = "dim",
        StatePath = "on.dim",
        State = new Wire.DocumentValue { ElementId = "Lamp::on::dim", ElementType = "StateUsage" },
        Region = "light",
        Enclosing = { "on" },
    };

    /// <summary>A trace record with every fact set.</summary>
    internal static Wire.DocumentEvent Event { get; } = new()
    {
        Kind = "send",
        Time = new Wire.DocumentValue { Quantity = new Wire.Quantity { RealMagnitude = 2.5, Unit = "s", UnitTerm = new Wire.UnitTerm { ScaleNum = 1, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "SI::second", Exponent = 1 } } } } },
        Object = Object,
        Machine = "lp",
        State = "on",
        From = "off",
        To = "on",
        Target = new Wire.DocumentObject { InstanceId = 3, Path = "#3" },
        Event = "Switch",
        Payload = { "level = 2" },
        Alternatives = { "a", "b" },
        Taken = "b",
        Text = "send Switch to #3",
        Token = long.MaxValue,
        Step = 3,
        Feature = "this.level",
        Value = new Wire.Value { IntValue = 2 },
    };

    /// <summary>An action's assign record, every fact an action run adds set: the token, the step, the feature written and its value.</summary>
    internal static Wire.DocumentEvent Assign { get; } = new()
    {
        Kind = "assign",
        Time = new Wire.DocumentValue { RealValue = 1.5 },
        Object = Object,
        Machine = "fill",
        Text = "assign count := 3",
        Token = 4,
        Step = 7,
        Feature = "count",
        Value = new Wire.Value { IntValue = 3 },
    };

    /// <summary>A document value of every arm, and an anonymous element, each part set.</summary>
    internal static IReadOnlyList<Wire.DocumentValue> DocumentValues { get; } =
    [
        Element,
        new() { ElementType = "AssertConstraintUsage" },
        new() { Object = Object },
        new() { StringValue = "text" },
        new() { IntValue = -3 },
        new() { BigIntValue = "9223372036854775808" },
        new() { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } },
        new() { RealValue = 2.5 },
        new() { BoolValue = true },
        new() { Infinity = true },
        new() { Quantity = new Wire.Quantity { BigIntMagnitude = "-9223372036854775809", Unit = "kg" } },
        new() { Verdict = Verdict },
        new() { State = State },
        new() { Event = Event },
        new() { Event = Assign },
    ];

    /// <summary>Document values whose optional parts are left out, which must stay left out.</summary>
    internal static IReadOnlyList<Wire.DocumentValue> SparseDocumentValues { get; } =
    [
        new(),
        new() { ElementId = string.Empty },
        new() { Object = new Wire.DocumentObject { InstanceId = 4 } },
        new() { Verdict = new Wire.DocumentVerdict { Assertion = new Wire.DocumentValue(), Verdict = "holds" } },
        new() { State = new Wire.DocumentState { State = new Wire.DocumentValue(), Name = "idle" } },
        new() { Event = new Wire.DocumentEvent { Kind = "choice" } },
        new() { RealValue = -0.0 },
    ];

    /// <summary>A diagnostic of a choice point, as a run notes one.</summary>
    internal static Wire.Diagnostic ChoicePoint { get; } = new()
    {
        Severity = "info",
        Message = "choice point: split",
        Code = "choice-point",
        OrderDependent = { "winner" },
    };

    /// <summary>One run of an action, every field set.</summary>
    internal static Wire.ExecuteActionResponse ActionRun { get; } = new()
    {
        Outputs = { ["result"] = new Wire.Value { IntValue = 15 }, ["mass"] = new Wire.Value { Quantity = new Wire.Quantity { RealMagnitude = 2, Unit = "kg" } } },
        Diagnostics = { ChoicePoint },
        FinalTime = 8,
        PerformerAttributes = { ["this.level"] = new Wire.Value { IntValue = 2 } },
        OrderDependent = { ["result"] = 1 },
        Trace = { Assign, new Wire.DocumentEvent { Kind = "complete", Step = 8, Text = "complete" } },
        TraceDropped = 5,
    };

    /// <summary>A trace: the complete record, then one record per kind of instant.</summary>
    internal static IReadOnlyList<Wire.DocumentEvent> Trace { get; } =
        [Event, .. DocumentValues.Concat(SparseDocumentValues).Select((time, i) => new Wire.DocumentEvent { Kind = "transition", Time = time, Text = $"record {i}" })];

    /// <summary>One run of a state machine, every field set, its trace holding every kind of instant.</summary>
    internal static Wire.ExecuteStateResponse StateRun { get; } = new()
    {
        StatesVisited = { "init", "Running", "done" },
        FinalContext = { ["count"] = new Wire.Value { IntValue = 3 }, ["this.level"] = new Wire.Value { RealValue = 1.5 } },
        Diagnostics = { ChoicePoint },
        FinalTime = 4.5,
        Trace = { Trace },
        TraceDropped = 2,
        OrderDependent = { ["count"] = 2 },
    };

    /// <summary>An outcome with every field set.</summary>
    internal static Wire.Outcome Outcome { get; } = new()
    {
        Outputs = { ["winner"] = new Wire.Value { IntValue = 2 } },
        FinalState = "done",
        StatesVisited = { "init", "done" },
        Error = "action execution failed: deadlock",
        Linearizations = 3,
        Witness = { "split: [left right] took right" },
        Diagnostics = { ChoicePoint },
        Probability = 0.25,
        ProbabilityRange = new Wire.ProbabilityRange { Min = 0.25, Max = 0.75 },
    };

    /// <summary>How an exploration ended, every field set.</summary>
    internal static Wire.ExplorationStatus Status { get; } = new()
    {
        Complete = false,
        Runs = 8,
        BudgetsHit = { "runs", "depth" },
        RunsBudget = 8,
        DepthBudget = 4,
        ProbabilitiesLowerBound = true,
        FailedLinearizations = 1,
        Unenumerated = { "Test::race::split" },
    };

    /// <summary>An exploration of an action: a complete outcome and a bare one.</summary>
    internal static Wire.ExecuteActionResponse ActionExploration { get; } = new()
    {
        Outcomes = { Outcome, new Wire.Outcome { Outputs = { ["winner"] = new Wire.Value { IntValue = 1 } }, Linearizations = 1 } },
        Exploration = Status,
    };

    /// <summary>An exploration of a state machine.</summary>
    internal static Wire.ExecuteStateResponse StateExploration { get; } = new()
    {
        Outcomes = { Outcome },
        Exploration = new Wire.ExplorationStatus { Complete = true, Runs = 2, RunsBudget = 1024, DepthBudget = 64 },
    };

    /// <summary>A witness with every field set.</summary>
    internal static Wire.CheckWitness Witness { get; } = new()
    {
        Schedule = "replay",
        Draws = { "0.5" },
        Choices = { "split: [a b] took b" },
        Inputs = { "x = 3" },
        FileName = "fill-1.witness",
        FileContent = "# witness\nsplit: b\n",
    };

    /// <summary>What a check found, every field set: a search, violations, divergences, finals and a symbolic claim.</summary>
    internal static Wire.CheckBehaviorResponse Check { get; } = new()
    {
        Verdict = "fails",
        Lines = { "Action Plant::Tank::fill: fails", "  property Plant::Tank::low violated at depth 3" },
        Search = new Wire.CheckSearch
        {
            States = 11,
            Moves = 10,
            MaxDepth = 6,
            BoundsHit = { "depth" },
            Horizon = 5,
            HorizonSet = true,
            MassBounded = true,
            Notes = { "clock stopped at the horizon" },
            Scope = { "horizon" },
            Status = "bounded",
        },
        Violations =
        {
            new Wire.CheckViolation { Kind = "property", Property = "Plant::Tank::low", Depth = 3, Probability = 0.5, Witness = Witness, Text = "property Plant::Tank::low violated" },
            new Wire.CheckViolation { Kind = "failure", Error = "assignment failed", Text = "failure" },
        },
        Divergences =
        {
            new Wire.CheckDivergence
            {
                Feature = "this.level",
                Values = { new Wire.CheckDivergentValue { Value = "1", Witness = Witness }, new Wire.CheckDivergentValue { Value = "2" } },
            },
        },
        Finals = { new Wire.CheckFinal { Outcome = "done", Values = { ["this.level"] = "2", ["this.capacity"] = "5" }, Witness = Witness }, new Wire.CheckFinal { Outcome = "done" } },
        Claim = "violated",
        FreeInputs = { "x" },
        Assumptions = { "Plant::Tank::capped" },
        Witness = Witness,
        Contrast = new Wire.CheckWitness { Schedule = "declared" },
        Engine = "check",
        Strength = "witnessed",
        Bounds = { new Wire.Bound { Name = "depth", Limit = 64, Reached = true }, new Wire.Bound { Name = "states", Limit = 100000 } },
        Reason = "a bound was hit",
    };

    /// <summary>A traced run of an action that failed, with the clock it stopped at and its trace up to the failure.</summary>
    internal static Wire.ExecuteActionResponse FailedAction { get; } = new()
    {
        Error = "action execution failed: still waiting at 5 s without completing",
        Diagnostics = { ChoicePoint },
        FinalTime = 2,
        Trace = { Assign },
        TraceDropped = 1,
    };

    /// <summary>A traced run of a state machine that failed, with its trace up to the failure.</summary>
    internal static Wire.ExecuteStateResponse FailedState { get; } = new()
    {
        Error = "state machine execution failed: no transition",
        Diagnostics = { ChoicePoint },
        FinalTime = 1,
        Trace = { Event },
        TraceDropped = 3,
    };

    /// <summary>A check refused in band, which the service answers undecided.</summary>
    internal static Wire.CheckBehaviorResponse FailedCheck { get; } = new() { Verdict = "undecided", Error = "behavior not found: Plant::Tank::drain" };
}
