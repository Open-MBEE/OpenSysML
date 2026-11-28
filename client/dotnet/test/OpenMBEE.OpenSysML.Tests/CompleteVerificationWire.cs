using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Answers that set every field the verification, validation, calculation, analysis, sweep and engine responses
/// can reach, and each failure in band.
/// </summary>
internal static class CompleteVerificationWire
{
    /// <summary>The bounds an engine ran under, one reached.</summary>
    internal static IReadOnlyList<Wire.Bound> Bounds { get; } =
        [new Wire.Bound { Name = "solver", Limit = 5000, Reached = true }, new Wire.Bound { Name = "depth", Limit = 64 }];

    /// <summary>A violated verdict with every fact set, its witness one assignment with a value and one without, and a note of each kind.</summary>
    internal static Wire.Verdict Verdict => new()
    {
        Kind = "constraint",
        ElementId = "Demo::Vehicle::massLight",
        Element = "Demo::Vehicle::massLight",
        Holds = false,
        Condition = "mass < 1000 [kg]",
        InstanceId = 9,
        InstanceTypeId = "Demo::Vehicle",
        RequirementId = "Demo::massLimit",
        Engine = "solve",
        Strength = "witnessed",
        Bounds = { Bounds },
        InstancePath = "engine",
        Question = "holds",
        Status = "violated",
        Witness =
        {
            new Wire.WitnessAssignment { Feature = "Demo::Vehicle::mass", Value = new Wire.Value { RealValue = 1500.5 }, Unit = "kg", Exact = "3001/2" },
            new Wire.WitnessAssignment { Feature = "Demo::Vehicle::gear", Exact = "Gear::low" },
        },
        Notes =
        {
            new Wire.VerdictNote { Kind = "uncovered", Subject = "engine", Text = "Not in the query: what engine asserts about its own objects, as it may hold more than one object; instantiate and validate an object to check it" },
            new Wire.VerdictNote { Kind = "assumed_one", Subject = "Demo::Vehicle::gear", Text = "Assuming Demo::Vehicle::gear holds one value: it declares no multiplicity, so the default [0..*] governs it" },
        },
    };

    /// <summary>An undecided verdict: false with the error that left it so.</summary>
    internal static Wire.Verdict Undecided => new()
    {
        Kind = "constraint",
        ElementId = "Demo::sq",
        Element = "Demo::sq",
        Error = "exponentiation is outside the decided fragment",
        FailureReason = Wire.FailureReason.Undecided,
        Question = "holds",
        Status = "undecided",
    };

    /// <summary>What a verification case's body answered, every fact set.</summary>
    internal static Wire.VerificationVerdict Verification => new()
    {
        CaseId = "Demo::checkBound",
        Kind = "inconclusive",
        Detail = "the bound was not reached",
        Subcase = true,
        RequirementId = "Demo::bounded",
        InstanceId = 9,
        InstanceTypeId = "Demo::Vehicle",
    };

    /// <summary>A constraint's verdict about an object.</summary>
    internal static Wire.VerifyConstraintResponse Constraint => new()
    {
        Verdict = Verdict,
        Instances = { CompleteWire.Instance },
        Diagnostics = { CompleteWire.Diagnostic },
    };

    /// <summary>A requirement's verdict with the case verifying it.</summary>
    internal static Wire.VerifyRequirementResponse Requirement => new()
    {
        Verdict = Verdict,
        Instances = { CompleteWire.Instance },
        Diagnostics = { CompleteWire.Diagnostic },
        VerificationVerdicts = { Verification },
    };

    /// <summary>Two satisfaction verdicts, one decided and one not.</summary>
    internal static Wire.VerifySatisfactionResponse Satisfaction => new()
    {
        Verdicts = { Verdict, Undecided },
        Instances = { CompleteWire.Instance },
        Diagnostics = { CompleteWire.Diagnostic },
        VerificationVerdicts = { Verification },
    };

    /// <summary>A validation with its summary, a verdict and the objects reached.</summary>
    internal static Wire.ValidateInstanceResponse Validation => new()
    {
        Verdicts = { Verdict },
        Summary = new Wire.Verdict { Kind = "object", ElementId = "Demo::sedan", Element = "Demo::sedan", InstanceId = 9, InstanceTypeId = "Demo::Vehicle", Question = "evaluate", Status = "violated" },
        Instances = { CompleteWire.Instance, new Wire.Instance { Id = 10, TypeSymbolId = "Demo::Engine" } },
        Diagnostics = { CompleteWire.Diagnostic },
        VerificationVerdicts = { Verification },
        Bounded = true,
    };

    /// <summary>A calc usage's outputs, with the engine that computed them.</summary>
    internal static Wire.EvaluateCalcResponse Calculation => new()
    {
        Result = new Wire.Value { RealValue = 5 },
        Outputs = { new Wire.CalcOutput { Name = "area", Value = new Wire.Value { RealValue = 12 } }, new Wire.CalcOutput { Name = "perimeter", Value = new Wire.Value { IntValue = 14 } } },
        Diagnostics = { CompleteWire.Diagnostic },
        Engine = "run",
        Strength = "observed",
        Bounds = { Bounds },
    };

    /// <summary>A preview of a tool call, every fact set.</summary>
    internal static Wire.ToolDryRun ToolDryRun => new()
    {
        Action = "Demo::simulate",
        Tool = "solver",
        Version = "1.2",
        Manifest = "/tools/solver.json",
        Executable = "/usr/bin/solver",
        Protocol = "argv+none/csv",
        Args = { "--mass", "1500" },
        Env = { "HOME=<from this process>", "MODE=fast" },
        Cwd = "/work",
        Stdin = "template",
        StdinData = "mass=1500",
        InputFile = new Wire.ToolDryRunFile { Name = "in.csv", Format = "csv", Data = "mass\n1500\n" },
        OutputDir = true,
        Inputs = { new Wire.ToolDryRunInput { Variable = "m", Parameter = "mass", Value = "1500", Unit = "kg" } },
        Outputs = { new Wire.ToolDryRunOutput { Variable = "t", Parameter = "time" } },
        ReplySource = "file:out.csv",
        Lines = { "tool solver 1.2", "reply from file:out.csv" },
    };

    /// <summary>One run of a case: every field a run reports, a tool dry run's preview included.</summary>
    internal static Wire.RunAnalysisResponse Analysis => new()
    {
        Outputs = { new Wire.CalcOutput { Name = "total", Value = new Wire.Value { RealValue = 12 } }, new Wire.CalcOutput { Name = "best", Value = new Wire.Value { InstanceId = 9 } } },
        Verdicts = { new Wire.Verdict { Kind = "objective", ElementId = "An::CostAnalysis::affordable", Element = "affordable", Holds = true, InstanceId = 9, InstanceTypeId = "Demo::Vehicle" }, Verdict },
        Instances = { CompleteWire.Instance },
        Diagnostics = { CompleteWire.Diagnostic },
        VerificationVerdicts = { Verification },
        Evaluations =
        {
            new Wire.CaseEvaluation { FunctionId = "An::score", Arguments = { new Wire.Value { InstanceId = 9 } }, Result = new Wire.Value { RealValue = 0.5 }, Selected = true },
            new Wire.CaseEvaluation { FunctionId = "An::score", Arguments = { new Wire.Value { InstanceId = 10 } }, Error = "division by zero", Tied = true },
        },
        Engine = "run",
        Strength = "observed",
        Bounds = { Bounds },
        ToolDryRun = ToolDryRun,
    };

    /// <summary>A run that failed after computing an output and making an evaluation.</summary>
    internal static Wire.RunAnalysisResponse FailedAnalysis => new()
    {
        Outputs = { new Wire.CalcOutput { Name = "total", Value = new Wire.Value { RealValue = 7 } } },
        Verdicts = { Undecided },
        Instances = { CompleteWire.Instance },
        Diagnostics = { CompleteWire.Diagnostic },
        Evaluations = { new Wire.CaseEvaluation { FunctionId = "An::score", Arguments = { new Wire.Value { InstanceId = 9 } }, Error = "no value" } },
        Error = "analysis run failed: step budget exhausted",
        FailureReason = Wire.FailureReason.Evaluation,
        Engine = "run",
        Strength = "not covered",
        Bounds = { Bounds },
    };

    /// <summary>An exploration of a case, with the standing of the engine that answered.</summary>
    internal static Wire.RunAnalysisResponse Exploration
    {
        get
        {
            var exploring = CompleteExecutionWire.ActionExploration;
            return new Wire.RunAnalysisResponse
            {
                Outcomes = { exploring.Outcomes },
                Exploration = exploring.Exploration,
                Engine = "explore",
                Strength = "bounded",
                Bounds = { Bounds },
            };
        }
    }

    /// <summary>A sampled sweep of three rows: a passing row, a failed one keeping what it computed, and a bare one.</summary>
    internal static Wire.RunSweepResponse Sweep => new()
    {
        Rows =
        {
            new Wire.SweepRow
            {
                Inputs = { new Wire.CalcOutput { Name = "limit", Value = new Wire.Value { RealValue = 20 } } },
                Outputs = { new Wire.CalcOutput { Name = "total", Value = new Wire.Value { RealValue = 37 } } },
                Verdicts = { new Wire.Verdict { Kind = "objective", Element = "affordable", Holds = true, InstanceId = 9, InstanceTypeId = "Sw::barge" } },
                ElapsedMicros = 220,
                Evaluations = { new Wire.CaseEvaluation { FunctionId = "Sw::score", Arguments = { new Wire.Value { InstanceId = 9 } }, Result = new Wire.Value { IntValue = 1 }, Selected = true } },
            },
            new Wire.SweepRow
            {
                Inputs = { new Wire.CalcOutput { Name = "limit", Value = new Wire.Value { IntValue = 0 } } },
                Outputs = { new Wire.CalcOutput { Name = "total", Value = new Wire.Value { RealValue = 3 } } },
                Verdicts = { Undecided },
                ElapsedMicros = 17,
                Error = "division by zero",
                FailureReason = Wire.FailureReason.Evaluation,
            },
            new Wire.SweepRow(),
        },
        Parameters = { "limit" },
        Sampled = true,
        Seed = ulong.MaxValue,
        Diagnostics = { CompleteWire.Diagnostic },
        Instances = { CompleteWire.Instance },
        Engine = "sweep",
        Strength = "observed",
        Bounds = { Bounds },
    };

    /// <summary>A built-in engine and a manifest engine that is not served, every fact of each set.</summary>
    internal static Wire.ListEnginesResponse Engines => new()
    {
        Engines =
        {
            new Wire.EngineInfo
            {
                Name = "solve",
                Authority = "proved",
                Answers = { "holds", "satisfiable" },
                Bounds = { "solver" },
                Process = "z3",
                ProcessFound = "/usr/bin/z3",
                Ready = true,
                Kind = "built-in",
                Protocol = "-",
                Served = true,
            },
            new Wire.EngineInfo
            {
                Name = "ext",
                Authority = "observed",
                Answers = { "compute" },
                Process = "ext-engine",
                Unavailable = "not served: start the service with -serve-external-engines ext",
                Kind = "engine",
                Protocol = "stdio/jsonrpc",
                Source = "/engines/ext.json",
                Command = "/opt/ext/bin/ext-engine",
                Version = "0.3",
            },
        },
    };
}
