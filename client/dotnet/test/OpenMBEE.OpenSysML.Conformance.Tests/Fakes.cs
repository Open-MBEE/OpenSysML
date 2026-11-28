using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>An adapter whose parses and answers the test scripts, recording what it was asked.</summary>
internal sealed class FakeScenarioClient : IScenarioClient
{
    private readonly object gate = new();

    public string Protocol { get; init; } = "fake";

    public string Service { get; init; } = "fake-service";

    public IReadOnlyCollection<string> Capabilities { get; init; } = [];

    /// <summary>Answers a parse; by default with a hash naming the model's first fixture.</summary>
    public Func<ScenarioModel, IReadOnlyList<FixtureSource>, CancellationToken, Task<ModelParse>> OnParse { get; init; } =
        (model, _, _) => Task.FromResult(new ModelParse($"hash-of-{model.FixtureNames[0]}", "", []));

    /// <summary>Answers a call; by default with the RPC's empty response.</summary>
    public Func<MethodDescriptor, IMessage, CancellationToken, Task<ScenarioAnswer>> OnCall { get; init; } =
        (method, _, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Answered(method.OutputType.Parser.ParseFrom(ByteString.Empty)));

    /// <summary>Every parse asked for, in order, with the documents it was given.</summary>
    public List<(ScenarioModel Model, IReadOnlyList<FixtureSource> Documents)> Parses { get; } = [];

    /// <summary>Every call made, in order.</summary>
    public List<(string Method, IMessage Request)> Calls { get; } = [];

    public Task<ModelParse> ParseAsync(ScenarioModel model, IReadOnlyList<FixtureSource> documents, CancellationToken cancellationToken)
    {
        lock (gate)
        {
            Parses.Add((model, documents));
        }

        return OnParse(model, documents, cancellationToken);
    }

    public Task<ScenarioAnswer> CallAsync(MethodDescriptor method, IMessage request, CancellationToken cancellationToken)
    {
        lock (gate)
        {
            Calls.Add((method.Name, request));
        }

        return OnCall(method, request, cancellationToken);
    }
}

/// <summary>A clock whose every reading is a fixed step after the one before, so each scenario lasts exactly one step.</summary>
internal sealed class StepClock(TimeSpan step) : TimeProvider
{
    private long now;

    public override long TimestampFrequency => TimeSpan.TicksPerSecond;

    public override long GetTimestamp() => Interlocked.Add(ref now, step.Ticks) - step.Ticks;
}
