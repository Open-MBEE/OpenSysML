using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the execution and check calls returned as wire messages, reading
/// only public types. Every field the execution and check responses can reach is set in a sample, and
/// each sample, read by the client and rendered back, comes out as it went in: a trace record, a witness
/// or an outcome fact the client dropped, or one the rendering forgot, fails here.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceExecutionRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheExecutionAndCheckResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(Wire.ExecuteActionResponse.Descriptor, Wire.ExecuteStateResponse.Descriptor, Wire.CheckBehaviorResponse.Descriptor);
        IMessage[] samples =
        [
            CompleteExecutionWire.ActionRun,
            CompleteExecutionWire.ActionExploration,
            CompleteExecutionWire.StateRun,
            CompleteExecutionWire.StateExploration,
            CompleteExecutionWire.Check,
            CompleteExecutionWire.FailedAction,
            CompleteExecutionWire.FailedState,
            CompleteExecutionWire.FailedCheck,
            .. CompleteWire.Values,
            CompleteWire.Diagnostic,
        ];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void AnActionRunIsRenderedWithEveryFact()
    {
        var bare = new Wire.ExecuteActionResponse();

        Assert.Equal(CompleteExecutionWire.ActionRun, Rendering.ActionRun(ExecutionWire.Run(CompleteExecutionWire.ActionRun, Model.ExecuteActionMethod)));
        Assert.Equal(bare, Rendering.ActionRun(ExecutionWire.Run(bare, Model.ExecuteActionMethod)));
    }

    [Fact]
    public void AStateRunIsRenderedWithItsWholeTrace()
    {
        var bare = new Wire.ExecuteStateResponse();

        Assert.Equal(CompleteExecutionWire.StateRun, Rendering.StateRun(ExecutionWire.Run(CompleteExecutionWire.StateRun, Model.ExecuteStateMethod)));
        Assert.Equal(bare, Rendering.StateRun(ExecutionWire.Run(bare, Model.ExecuteStateMethod)));
    }

    [Fact]
    public void AnExplorationIsRenderedWithEveryOutcomeAndHowItEnded()
    {
        foreach (var sample in new[] { CompleteExecutionWire.ActionExploration, new Wire.ExecuteActionResponse { Exploration = new Wire.ExplorationStatus() } })
        {
            var read = ExecutionWire.Exploration(sample.Outcomes, sample.Exploration, Standing.None, Model.ExecuteActionMethod);

            Assert.Equal(sample.Outcomes, Rendering.Outcomes(read));
            Assert.Equal(sample.Exploration, Rendering.ExplorationStatus(read));
        }
    }

    [Fact]
    public void ACheckResultIsRenderedWithEveryWitness()
    {
        var bare = new Wire.CheckBehaviorResponse();

        Assert.Equal(CompleteExecutionWire.Check, Rendering.CheckResult(ExecutionWire.Check(CompleteExecutionWire.Check)));
        Assert.Equal(bare, Rendering.CheckResult(ExecutionWire.Check(bare)));
    }

    [Fact]
    public void EveryKindOfDocumentValueIsRenderedAsItWasRead()
    {
        var kinds = new System.Collections.Generic.HashSet<Type>();
        foreach (var sample in CompleteExecutionWire.DocumentValues.Concat(CompleteExecutionWire.SparseDocumentValues))
        {
            var read = DocumentWire.Value(sample, Model.ExecuteStateMethod, "trace[0].time");
            kinds.Add(read.GetType());

            Assert.Equal(sample, Rendering.DocumentValue(read));
        }

        Assert.Equal(
            typeof(DocumentValue).GetNestedTypes().Where(type => !type.IsAbstract).Select(type => type.Name).Order(StringComparer.Ordinal),
            kinds.Select(type => type.Name).Order(StringComparer.Ordinal));
    }
}
