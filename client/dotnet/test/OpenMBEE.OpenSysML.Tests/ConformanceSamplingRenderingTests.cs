using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the runs and self-check calls returned as wire messages, reading only public
/// types. Every field the runs, self-check and recording responses can reach is set in a sample, and each runs and
/// self-check sample, read by the client and rendered back, comes out as it went in: a failed row, a distribution, a
/// finding or a runs' failure the client dropped, or one the rendering forgot, fails here. Recordings are rendered
/// through the adapter itself, in <see cref="ConformanceAdapterTests"/>.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceSamplingRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheRunsSelfCheckAndRecordingResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(Wire.RunRunsResponse.Descriptor, Wire.SelfCheckResponse.Descriptor, Wire.RecordRunResponse.Descriptor);
        IMessage[] samples =
        [
            CompleteSamplingWire.ActionRuns,
            CompleteSamplingWire.CaseRuns,
            CompleteSamplingWire.Unconcluded,
            CompleteSamplingWire.Unobserved,
            CompleteSamplingWire.SelfCheck,
            CompleteSamplingWire.Recording,
            CompleteSamplingWire.SweepRecording,
            CompleteSamplingWire.SampleRecording,
            CompleteSamplingWire.RefusedRecording,
            CompleteSamplingWire.FailedRunRecording,
            CompleteVerificationWire.Analysis,
            CompleteVerificationWire.FailedAnalysis,
            CompleteVerificationWire.Exploration,
            CompleteVerificationWire.Sweep,
            CompleteExecutionWire.ActionExploration,
            CompleteExecutionWire.StateExploration,
            .. CompleteWire.Values,
            CompleteWire.Diagnostic,
            CompleteWire.Instance,
            new Wire.SelfCheckResponse { Error = "e" },
            new Wire.RecordRunResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
            new Wire.RunSweepResponse { FailureReason = Wire.FailureReason.WrongKind, Error = "e" },
        ];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void RunsAreRenderedWithEveryRowDistributionAndConclusion()
    {
        foreach (var sample in new[] { CompleteSamplingWire.ActionRuns, CompleteSamplingWire.CaseRuns, CompleteSamplingWire.Unconcluded, new Wire.RunRunsResponse() })
        {
            Assert.Equal(sample, Rendering.Runs(RunsWire.Runs(sample, Model.RunRunsMethod)));
        }

        var failed = CompleteSamplingWire.Unobserved;
        var partial = Rendering.Runs(RunsWire.Runs(failed, Model.RunRunsMethod));
        partial.Error = failed.Error;
        partial.FailureReason = failed.FailureReason;
        Assert.Equal(failed, partial);
        Assert.False(Rendering.Runs(RunsWire.Runs(new Wire.RunRunsResponse(), Model.RunRunsMethod)).HasSeed);
        Assert.True(Rendering.Runs(RunsWire.Runs(new Wire.RunRunsResponse { Seed = 0 }, Model.RunRunsMethod)).HasSeed);
    }

    [Fact]
    public void ASelfCheckIsRenderedWithEveryFindingAndCount()
    {
        foreach (var sample in new[] { CompleteSamplingWire.SelfCheck, new Wire.SelfCheckResponse() })
        {
            Assert.Equal(sample, Rendering.SelfCheck(RunsWire.SelfCheck(sample)));
        }
    }
}
