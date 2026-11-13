namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Answers that set every field the runs, self-check and recording responses can reach, and each failure in band.
/// </summary>
internal static class CompleteSamplingWire
{
    /// <summary>A numeric observable's distribution, every summary of it set.</summary>
    internal static Wire.RunDistribution Numeric => new()
    {
        Name = "t",
        Unit = "s",
        Count = 3,
        Numeric = true,
        Min = new Wire.Value { RealValue = 1.25 },
        Max = new Wire.Value { RealValue = 3.75 },
        Mean = new Wire.Value { RationalValue = new Wire.Rational { Numerator = "7", Denominator = "3" } },
        Deviation = 1.3227,
        Percentiles = { new Wire.RunPercentile { Percentile = 50, Value = new Wire.Value { RealValue = 1.75 } }, new Wire.RunPercentile { Percentile = 90, Value = new Wire.Value { RealValue = 3.75 } } },
        Histogram =
        {
            new Wire.RunHistogramBin { Low = new Wire.Value { RealValue = 1.25 }, High = new Wire.Value { RealValue = 2.5 }, Count = 2 },
            new Wire.RunHistogramBin { Low = new Wire.Value { RealValue = 2.5 }, High = new Wire.Value { RealValue = 3.75 }, Count = 1 },
        },
    };

    /// <summary>An observable that is not numeric: each distinct value counted.</summary>
    internal static Wire.RunDistribution Counted => new()
    {
        Name = "mode",
        Count = 3,
        Values =
        {
            new Wire.RunCountedValue { Value = new Wire.Value { StringValue = "Demo::idle" }, Count = 2 },
            new Wire.RunCountedValue { Value = new Wire.Value { StringValue = "Demo::busy" }, Count = 1 },
        },
    };

    /// <summary>Three seeded runs of an action: a completed one, a failed one keeping what it observed, and a bare one.</summary>
    internal static Wire.RunRunsResponse ActionRuns => new()
    {
        Rows =
        {
            new Wire.SweepRow
            {
                Inputs = { new Wire.CalcOutput { Name = "run", Value = new Wire.Value { IntValue = 1 } } },
                Outputs =
                {
                    new Wire.CalcOutput { Name = "t", Value = new Wire.Value { RealValue = 1.25 } },
                    new Wire.CalcOutput { Name = "mode", Value = new Wire.Value { InstanceId = 9 } },
                },
                ElapsedMicros = 812,
            },
            new Wire.SweepRow
            {
                Inputs = { new Wire.CalcOutput { Name = "run", Value = new Wire.Value { IntValue = 2 } } },
                Outputs = { new Wire.CalcOutput { Name = "t", Value = new Wire.Value { RealValue = 3.75 } } },
                ElapsedMicros = 17,
                Error = "action execution failed: deadlock",
                FailureReason = Wire.FailureReason.Evaluation,
            },
            new Wire.SweepRow(),
        },
        Observables = { "t", "mode" },
        Distributions = { Numeric, Counted },
        Lines = { "t: 3 run(s), min 1.25, mean 2.25, max 3.75", "  1.25..2.5 ####  2" },
        Diagnostics = { CompleteWire.Diagnostic },
        Instances = { CompleteWire.Instance },
        Engine = "sweep",
        Strength = "observed",
        Bounds = { CompleteVerificationWire.Bounds },
        Seed = 7,
    };

    /// <summary>Runs of a Monte Carlo case, concluded over the sample: the conclusion's outputs and verdicts, the row's settled.</summary>
    internal static Wire.RunRunsResponse CaseRuns => new()
    {
        Rows =
        {
            new Wire.SweepRow
            {
                Inputs = { new Wire.CalcOutput { Name = "run", Value = new Wire.Value { IntValue = 1 } } },
                Outputs = { new Wire.CalcOutput { Name = "observed", Value = new Wire.Value { RealValue = 1.5 } } },
                Verdicts = { CompleteVerificationWire.Verdict },
                ElapsedMicros = 377,
                Evaluations = { new Wire.CaseEvaluation { FunctionId = "MC::score", Arguments = { new Wire.Value { InstanceId = 9 } }, Result = new Wire.Value { IntValue = 1 }, Selected = true } },
            },
        },
        Observables = { "observed" },
        Distributions = { Numeric },
        Lines = { "✓ Mc over 1 run(s)" },
        Outputs = { new Wire.CalcOutput { Name = "runs", Value = new Wire.Value { IntValue = 1 } }, new Wire.CalcOutput { Name = "mean", Value = new Wire.Value { RealValue = 1.5 } } },
        Verdicts = { CompleteVerificationWire.Verdict, CompleteVerificationWire.Undecided },
        Instances = { CompleteWire.Instance },
        Engine = "sweep",
        Strength = "observed",
        Bounds = { CompleteVerificationWire.Bounds },
        Seed = ulong.MaxValue,
    };

    /// <summary>Runs whose case has no conclusion, drawn from no seed under fixed draws.</summary>
    internal static Wire.RunRunsResponse Unconcluded => new()
    {
        Rows = { new Wire.SweepRow { Error = "analysed subject is unbound", FailureReason = Wire.FailureReason.Evaluation } },
        Lines = { "? Mc: no run completed, so the case is not concluded" },
        Unconcluded = "no run completed, so the case is not concluded",
        Engine = "sweep",
    };

    /// <summary>Runs that were made but asked for a feature no completed run produced: the failure beside the table.</summary>
    internal static Wire.RunRunsResponse Unobserved => new()
    {
        Rows = { new Wire.SweepRow { Inputs = { new Wire.CalcOutput { Name = "run", Value = new Wire.Value { IntValue = 1 } } }, ElapsedMicros = 261 } },
        Error = "invalid run request: no completed run of MC::Settle produced a value named nope (the clock is observed as clock)",
        FailureReason = Wire.FailureReason.Evaluation,
        Diagnostics = { CompleteWire.Diagnostic },
        Instances = { CompleteWire.Instance },
        Engine = "sweep",
        Strength = "observed",
        Bounds = { CompleteVerificationWire.Bounds },
        Seed = 7,
    };

    /// <summary>A self-check that fails: every kind of finding, the counts and the applications.</summary>
    internal static Wire.SelfCheckResponse SelfCheck => new()
    {
        Status = "fails",
        Findings =
        {
            new Wire.SelfCheckFinding
            {
                Status = "fails",
                Subject = "SysMLValidation::validatePartUsage for engine",
                Location = "m.sysml:4:5",
                Constraint = "SysMLValidation::validatePartUsage",
                Element = "engine",
                Line = "✗ SysMLValidation::validatePartUsage for engine (m.sysml:4:5)",
            },
            new Wire.SelfCheckFinding
            {
                Status = "warning",
                Subject = "Rules",
                Message = "the rule package yields no constraint",
                Line = "! Rules: the rule package yields no constraint",
            },
            new Wire.SelfCheckFinding(),
        },
        Elements = 33,
        Checks = 280,
        Violations = 1,
        Unevaluated = 2,
        EvaluationErrors = 3,
        Applications =
        {
            new Wire.SelfCheckApplication { Constraint = "SysMLValidation::validateAttributeUsageFeatures", Count = 5 },
            new Wire.SelfCheckApplication { Constraint = "SysMLValidation::validatePartUsage" },
        },
        Summary = "Self-model check: 33 elements, 280 checks, 1 violations, 2 unevaluated",
        Diagnostics = { CompleteWire.Diagnostic },
    };

    /// <summary>One run of a case recorded into the model's one document, with every fact of the recording.</summary>
    internal static Wire.RecordRunResponse Recording => new()
    {
        Run = CompleteVerificationWire.Analysis,
        Package = "Sw",
        Definition = "Sw::CostAnalysisRun",
        Records = { "Sw::CostAnalysis_run1" },
        Source = "package Sw {\n    part CostAnalysis_run1 : CostAnalysisRun;\n}\n",
        Content = "package Sw {\n\tpart barge : Ship;\n    part CostAnalysis_run1 : CostAnalysisRun;\n}\n",
        Documents = { new Wire.EditedDocument { Name = "sweep.sysml", Content = "package Sw {\n\tpart barge : Ship;\n    part CostAnalysis_run1 : CostAnalysisRun;\n}\n" } },
        Skipped = 1,
        Reasons = { "run 2 not recorded: mass is unread" },
        Diagnostics = { CompleteWire.Diagnostic },
        ModelHash = "recorded",
    };

    /// <summary>A sweep recorded as a document of its own.</summary>
    internal static Wire.RecordRunResponse SweepRecording => new()
    {
        Sweep = CompleteVerificationWire.Sweep,
        Package = "Records",
        Definition = "Records::CostAnalysisRun",
        Records = { "Records::CostAnalysis_run1", "Records::CostAnalysis_run3" },
        Source = "package Records;\n",
        Documents = { new Wire.EditedDocument { Name = "Records.sysml", Content = "package Records;\n" } },
        Skipped = 1,
        ModelHash = "swept",
    };

    /// <summary>A Monte Carlo sample recorded with its conclusion.</summary>
    internal static Wire.RecordRunResponse SampleRecording => new()
    {
        Sample = CaseRuns,
        Package = "Records",
        Definition = "Records::McRun",
        Records = { "Records::Mc_run1", "Records::Mc_run2" },
        Source = "package Records;\n",
        Documents = { new Wire.EditedDocument { Name = "Records.sysml", Content = "package Records;\n" } },
        ModelHash = "sampled",
    };

    /// <summary>A recording refused because its records would not analyze cleanly: the run and the declarations kept, nothing recorded.</summary>
    internal static Wire.RecordRunResponse RefusedRecording => new()
    {
        Run = CompleteVerificationWire.Analysis,
        Package = "Records",
        Definition = "Records::CostAnalysisRun",
        Records = { "Records::CostAnalysis_run1" },
        Source = "package Records;\n",
        Skipped = 2,
        Reasons = { "run 2 not recorded: mass is unread" },
        Error = "the model was left unchanged: duplicate name CostAnalysisRun",
        Diagnostics = { CompleteWire.Diagnostic },
    };

    /// <summary>A recording whose run itself failed: the run's failure is the recording's.</summary>
    internal static Wire.RecordRunResponse FailedRunRecording
    {
        get
        {
            var run = CompleteVerificationWire.FailedAnalysis;
            return new Wire.RecordRunResponse { Run = run, Error = run.Error, FailureReason = run.FailureReason };
        }
    }
}
