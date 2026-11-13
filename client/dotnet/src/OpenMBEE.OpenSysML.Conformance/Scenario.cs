using System.Collections.Generic;
using System.Linq;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// One conformance case: a call to make and what it must answer, as a scenario file in
/// conformance/scenarios states it. See conformance/README.md.
/// </summary>
/// <param name="Id">Unique across the suite; a report and a filter address the scenario by it.</param>
/// <param name="Description">Why the scenario is part of the contract.</param>
/// <param name="Rpc">The method, bare (<c>Evaluate</c>) or qualified (<c>sysml.SysMLService/Evaluate</c>).</param>
/// <param name="RequiresCapabilities">What GetServerInfo must report for <paramref name="Expect"/> to apply.</param>
/// <param name="ExpectWithoutCapability">What a service reporting none of them must answer instead, if stated.</param>
/// <param name="Model">The model parsed before the call, whose hash fills <c>${model_hash}</c>, if any.</param>
/// <param name="Request">The request as protobuf-JSON with its placeholders unresolved, or null when absent.</param>
/// <param name="Expect">What the answer must be.</param>
/// <param name="File">The scenario file it was read from, for messages.</param>
internal sealed record Scenario(
    string Id,
    string Description,
    string Rpc,
    IReadOnlyList<string> RequiresCapabilities,
    Expectation? ExpectWithoutCapability,
    ScenarioModel? Model,
    JsonElement? Request,
    Expectation Expect,
    string File)
{
    /// <summary>The method as a bare name: whatever follows the last <c>/</c> of <see cref="Rpc"/>.</summary>
    internal string Method => Rpc[(Rpc.LastIndexOf('/') + 1)..];
}

/// <summary>
/// The source a scenario needs parsed before its call, named by fixture so no scenario carries a
/// machine's path: one fixture parsed alone, or several parsed as one model of several documents,
/// each named by its fixture.
/// </summary>
/// <param name="Fixture">The one fixture, or empty when <paramref name="Fixtures"/> names several.</param>
/// <param name="Fixtures">The fixtures parsed together, or empty when <paramref name="Fixture"/> names one.</param>
/// <param name="Language">The language the source is read as, or empty for the service's default.</param>
/// <param name="StrictConformance">Whether the parse asks for strict conformance.</param>
internal sealed record ScenarioModel(string Fixture, IReadOnlyList<string> Fixtures, string Language, bool StrictConformance)
{
    /// <summary>Whether the model is several documents parsed together rather than one fixture.</summary>
    internal bool IsMultiDocument => Fixtures.Count > 0;

    /// <summary>Every fixture the model reads, in order.</summary>
    internal IReadOnlyList<string> FixtureNames => IsMultiDocument ? Fixtures : [Fixture];

    /// <summary>
    /// What identifies the parse, so one run parses each distinct model once. One fixture and the
    /// same fixture as a list of one are different parses, of one file and of several documents.
    /// </summary>
    internal string Key =>
        $"{(IsMultiDocument ? "sources" : "file")}[{string.Join(" ", FixtureNames.Select(Spelling.Quote))}]|{Language}|{(StrictConformance ? "true" : "false")}";
}

/// <summary>
/// What a call must answer. Every member is optional and all of them must hold; an empty
/// <paramref name="Status"/> means the call must succeed.
/// </summary>
/// <param name="Status">The canonical status name the call must end with, or empty for OK.</param>
/// <param name="StatusMessageContains">A substring the status message must contain, or empty.</param>
/// <param name="Response">The tree compared field by field against the answer, or null.</param>
/// <param name="Contains">Path to a substring the text there must contain.</param>
/// <param name="ContainsAll">Path to strings all of which must be there.</param>
/// <param name="NonEmpty">Paths that must hold a value other than their default.</param>
/// <param name="Absent">Paths that must be unset or hold their default.</param>
/// <param name="Counts">Path to the exact number of entries of the list or map there.</param>
/// <param name="MinCounts">Path to a lower bound on that number.</param>
internal sealed record Expectation(
    string Status,
    string StatusMessageContains,
    JsonElement? Response,
    IReadOnlyDictionary<string, string> Contains,
    IReadOnlyDictionary<string, IReadOnlyList<string>> ContainsAll,
    IReadOnlyList<string> NonEmpty,
    IReadOnlyList<string> Absent,
    IReadOnlyDictionary<string, int> Counts,
    IReadOnlyDictionary<string, int> MinCounts)
{
    /// <summary>The expectation that asks only for success.</summary>
    internal static Expectation Success { get; } = new(
        "",
        "",
        null,
        new Dictionary<string, string>(),
        new Dictionary<string, IReadOnlyList<string>>(),
        [],
        [],
        new Dictionary<string, int>(),
        new Dictionary<string, int>());

    /// <summary>The status this expectation names, which is OK when it names none.</summary>
    internal string WantStatus => Status.Length == 0 ? "OK" : Status;
}
