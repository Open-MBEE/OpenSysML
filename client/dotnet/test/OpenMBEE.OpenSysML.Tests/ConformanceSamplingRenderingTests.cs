using System;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the runs, self-check and recording calls returned as wire messages, reading
/// only public types. Every field those responses can reach is set in a sample, and each sample, read by the client and
/// rendered back, comes out as it went in: a failed row, a distribution, a finding, a recorded document or a runs'
/// failure the client dropped, or one the rendering forgot, fails here.
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

    [Fact]
    public async Task ARecordingIsRenderedWithItsRunsRecordsDocumentsAndModel()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        foreach (var sample in new[] { CompleteSamplingWire.Recording, CompleteSamplingWire.SweepRecording, CompleteSamplingWire.SampleRecording })
        {
            var recording = new Recording(
                sample.Run is { } run ? VerificationWire.Analysis(run, Model.RecordRunMethod, "run") : null,
                sample.Sweep is { } sweep ? VerificationWire.Sweep(sweep, Model.RecordRunMethod, "sweep") : null,
                sample.Sample is { } runs ? RunsWire.Runs(runs, Model.RecordRunMethod, "sample") : null,
                sample.Package,
                sample.Definition,
                sample.Records,
                sample.Source,
                sample.Content,
                RunsWire.Documents(sample.Documents),
                sample.Skipped,
                sample.Reasons,
                ModelWire.Diagnostics(sample.Diagnostics),
                connection.ModelByHash(sample.ModelHash));
            Assert.Equal(sample, Rendering.Recording(recording));
        }
    }

    [Fact]
    public void ARecordingNotMadeIsRenderedWithWhatItKeptAndWhereItsRunsReportedTheFailure()
    {
        var swept = new Wire.RecordRunResponse
        {
            Sweep = new Wire.RunSweepResponse { Error = "e", FailureReason = Wire.FailureReason.WrongKind, Engine = "sweep" },
            Error = "e",
            FailureReason = Wire.FailureReason.WrongKind,
        };
        var sampled = new Wire.RecordRunResponse
        {
            Sample = new Wire.RunRunsResponse { Error = "e", FailureReason = Wire.FailureReason.Evaluation, Engine = "sweep" },
            Error = "e",
            FailureReason = Wire.FailureReason.Evaluation,
        };
        var unconcluded = new Wire.RecordRunResponse { Sample = CompleteSamplingWire.Unconcluded, Skipped = 1, Error = "nothing recorded: no run completed" };
        foreach (var sample in new[] { CompleteSamplingWire.RefusedRecording, CompleteSamplingWire.FailedRunRecording, swept, sampled, unconcluded, new Wire.RecordRunResponse { Error = "e" } })
        {
            var ranFailure = sample.Run?.Error ?? sample.Sweep?.Error ?? sample.Sample?.Error ?? string.Empty;
            var failure = new RecordException(
                sample.Error,
                Model.RecordRunMethod,
                VerificationWire.Reason(sample.FailureReason),
                ModelWire.Diagnostics(sample.Diagnostics),
                sample.Run is { } run ? VerificationWire.Analysis(run, Model.RecordRunMethod, "run") : null,
                sample.Sweep is { } sweep ? VerificationWire.Sweep(sweep, Model.RecordRunMethod, "sweep") : null,
                sample.Sample is { } runs ? RunsWire.Runs(runs, Model.RecordRunMethod, "sample") : null,
                ranFailure.Length > 0,
                sample.Package,
                sample.Definition,
                sample.Records,
                sample.Source,
                sample.Skipped,
                sample.Reasons);
            Assert.Equal(sample, Rendering.Recording(failure));
        }
    }
}
