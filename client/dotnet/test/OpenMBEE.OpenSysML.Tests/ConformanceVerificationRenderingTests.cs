using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the verification, validation, calculation, analysis, sweep and engine calls
/// returned as wire messages, reading only public types. Every field those responses can reach is set in a sample,
/// and each sample, read by the client and rendered back, comes out as it went in: a witness, a partial output or a
/// failed row the client dropped, or one the rendering forgot, fails here.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceVerificationRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheVerificationResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(
            Wire.VerifyConstraintResponse.Descriptor,
            Wire.VerifyRequirementResponse.Descriptor,
            Wire.VerifySatisfactionResponse.Descriptor,
            Wire.ValidateInstanceResponse.Descriptor,
            Wire.EvaluateCalcResponse.Descriptor,
            Wire.RunAnalysisResponse.Descriptor,
            Wire.RunSweepResponse.Descriptor,
            Wire.ListEnginesResponse.Descriptor);
        IMessage[] samples =
        [
            CompleteVerificationWire.Constraint,
            CompleteVerificationWire.Requirement,
            CompleteVerificationWire.Satisfaction,
            CompleteVerificationWire.Validation,
            CompleteVerificationWire.Calculation,
            CompleteVerificationWire.Analysis,
            CompleteVerificationWire.FailedAnalysis,
            CompleteVerificationWire.Exploration,
            CompleteVerificationWire.Sweep,
            CompleteVerificationWire.Engines,
            CompleteExecutionWire.ActionExploration,
            CompleteExecutionWire.StateExploration,
            .. CompleteWire.Values,
            CompleteWire.Diagnostic,
            CompleteWire.Instance,
            new Wire.VerifyConstraintResponse { Error = "e" },
            new Wire.VerifyRequirementResponse { Error = "e" },
            new Wire.VerifySatisfactionResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
            new Wire.ValidateInstanceResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
            new Wire.EvaluateCalcResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
            new Wire.RunSweepResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
        ];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void AVerdictIsRenderedWithItsStandingAndWitness()
    {
        foreach (var sample in new[] { CompleteVerificationWire.Verdict, CompleteVerificationWire.Undecided, new Wire.Verdict() })
        {
            Assert.Equal(sample, Rendering.Verdict(VerificationWire.Verdict(sample, Model.VerifyConstraintMethod, "verdict")));
        }

        var verification = CompleteVerificationWire.Verification;
        Assert.Equal(verification, Assert.Single(Rendering.Verifications(VerificationWire.Verifications([verification]))));
    }

    [Fact]
    public void ACalculationAnAnalysisAndASweepAreRenderedWithEveryFact()
    {
        foreach (var sample in new[] { CompleteVerificationWire.Calculation, new Wire.EvaluateCalcResponse() })
        {
            var read = new Calculation(
                sample.Result is { } result ? ValueWire.FromWire(result, Model.EvaluateCalcMethod, "result") : null,
                VerificationWire.Outputs(sample.Outputs, Model.EvaluateCalcMethod, "outputs"),
                ModelWire.Diagnostics(sample.Diagnostics),
                ExecutionWire.StandingOf(sample.Engine, sample.Strength, sample.Bounds));
            Assert.Equal(sample, Rendering.Calculation(read));
        }

        foreach (var sample in new[] { CompleteVerificationWire.Analysis, new Wire.RunAnalysisResponse() })
        {
            Assert.Equal(sample, Rendering.Analysis(VerificationWire.Analysis(sample, Model.RunAnalysisMethod)));
        }

        var failed = CompleteVerificationWire.FailedAnalysis;
        var partial = Rendering.Analysis(VerificationWire.Analysis(failed, Model.RunAnalysisMethod));
        partial.Error = failed.Error;
        partial.FailureReason = failed.FailureReason;
        Assert.Equal(failed, partial);

        foreach (var sample in new[] { CompleteVerificationWire.Sweep, new Wire.RunSweepResponse() })
        {
            Assert.Equal(sample, Rendering.Sweep(VerificationWire.Sweep(sample, Model.RunSweepMethod)));
        }
    }

    [Fact]
    public void AnAnalysisExplorationIsRenderedWithItsStanding()
    {
        var sample = CompleteVerificationWire.Exploration;
        var read = ExecutionWire.Exploration(sample.Outcomes, sample.Exploration, ExecutionWire.StandingOf(sample.Engine, sample.Strength, sample.Bounds), Model.RunAnalysisMethod);

        Assert.Equal(sample.Outcomes, Rendering.Outcomes(read));
        Assert.Equal(sample.Exploration, Rendering.ExplorationStatus(read));
        Assert.Equal((sample.Engine, sample.Strength), (read.Standing.Engine, read.Standing.Strength));
        Assert.Equal(sample.Bounds, Rendering.Bounds(read.Standing));
    }

    [Fact]
    public void EveryEngineIsRenderedWithEveryFact()
    {
        var sample = CompleteVerificationWire.Engines;

        Assert.Equal(sample.Engines, sample.Engines.Select(VerificationWire.Engine).Select(Rendering.Engine));
    }
}
