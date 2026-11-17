using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// One way of answering scenarios, which the runner judges: a protocol through which a client calls
/// the service. An adapter answers through the client's public typed API and renders what that
/// API returned as the RPC's response message, so the comparison sees exactly what a caller was
/// given; it does not send the decoded request raw. The runner owns everything else: capability
/// gating, fixtures and placeholders, the once-per-run parse of each model, timeouts, status and
/// response comparison, and the results. Nothing here starts or stops a service.
/// The runner bounds its wait independently; adapters must still honor cancellation to release
/// their own in-flight work, whose late failures the runner observes.
/// </summary>
internal interface IScenarioClient
{
    /// <summary>The protocol's name in the report, such as <c>grpc</c>, <c>connect</c> or <c>connect-json</c>.</summary>
    string Protocol { get; }

    /// <summary>What answered, for the report: the service binary or address.</summary>
    string Service { get; }

    /// <summary>The capabilities the service reported, which gate the scenarios that need one.</summary>
    IReadOnlyCollection<string> Capabilities { get; }

    /// <summary>
    /// Parses a scenario's model and returns what the parse reported. A parse the service refused
    /// may be thrown; the runner reports either as an error in the scenario, not a disagreement.
    /// </summary>
    /// <param name="model">The model the scenario names: one fixture, or several parsed as one model.</param>
    /// <param name="documents">The fixtures' sources, in the model's order, each named by its fixture.</param>
    /// <param name="cancellationToken">Ends the parse at the runner's deadline or when the run is cancelled.</param>
    /// <returns>The parse's outcome.</returns>
    Task<ModelParse> ParseAsync(ScenarioModel model, IReadOnlyList<FixtureSource> documents, CancellationToken cancellationToken);

    /// <summary>Makes one scenario's call.</summary>
    /// <param name="method">The RPC the scenario names.</param>
    /// <param name="request">The scenario's request, decoded into the RPC's request message with its placeholders resolved.</param>
    /// <param name="cancellationToken">Ends the call at the runner's deadline or when the run is cancelled.</param>
    /// <returns>The response, the refusal, or why the client cannot express the request.</returns>
    Task<ScenarioAnswer> CallAsync(MethodDescriptor method, IMessage request, CancellationToken cancellationToken);
}

/// <summary>A fixture's source, named as a parse of several documents names it.</summary>
/// <param name="Name">The fixture's name.</param>
/// <param name="Content">Its text.</param>
internal sealed record FixtureSource(string Name, string Content);

/// <summary>What a parse of a scenario's model reported.</summary>
/// <param name="Hash">The hash the service gave the model; empty when it gave none.</param>
/// <param name="Error">The failure the answer reported in band, or empty.</param>
/// <param name="Diagnostics">The parse's diagnostics, in the order reported.</param>
internal sealed record ModelParse(string Hash, string Error, IReadOnlyList<ModelParseDiagnostic> Diagnostics);

/// <summary>One diagnostic of a parse.</summary>
/// <param name="Severity">Its severity as the schema spells it, such as <c>error</c>.</param>
/// <param name="Message">What it says.</param>
internal sealed record ModelParseDiagnostic(string Severity, string Message);

/// <summary>What a client answered to a scenario's call.</summary>
internal abstract record ScenarioAnswer
{
    private ScenarioAnswer()
    {
    }

    /// <summary>The call succeeded.</summary>
    /// <param name="Response">The RPC's response message, rendered from what the client's API returned.</param>
    internal sealed record Answered(IMessage Response) : ScenarioAnswer;

    /// <summary>The call ended with a status other than OK.</summary>
    /// <param name="Status">The status.</param>
    /// <param name="Message">The status message.</param>
    internal sealed record Refused(StatusCode Status, string Message) : ScenarioAnswer;

    /// <summary>The client's API cannot express the request, which the runner reports as a skip for this reason.</summary>
    /// <param name="Reason">What the API cannot express.</param>
    internal sealed record Unexpressible(string Reason) : ScenarioAnswer;
}
