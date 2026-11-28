using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// The scenarios this client is reviewed to skip, each for a stated reason: every scenario of an RPC
/// its public API does not cover yet, and the scenarios whose request the API cannot express. A run
/// that skips anything else, skips one of these for another reason, or runs one of them fails, so
/// neither a shrinking API nor a stale list passes quietly.
/// </summary>
/// <param name="UncoveredRpcs">The RPCs whose every scenario is skipped as not covered, by bare name.</param>
/// <param name="Unexpressible">The scenarios skipped as unexpressible, by id, with the reason the adapter gives.</param>
internal sealed record ExpectedSkips(FrozenSet<string> UncoveredRpcs, FrozenDictionary<string, string> Unexpressible)
{
    private static readonly FrozenSet<string> FileMembers = FrozenSet.Create(StringComparer.Ordinal, "uncovered_rpcs", "unexpressible");

    /// <summary>Reads the list.</summary>
    /// <param name="path">The file, client/dotnet/test/testdata/expected-skips.json in the checkout.</param>
    /// <returns>The list.</returns>
    /// <exception cref="InvalidDataException">The file is malformed.</exception>
    /// <exception cref="IOException">It cannot be read.</exception>
    internal static ExpectedSkips Load(string path) => Parse(File.ReadAllBytes(path), path);

    /// <summary>Reads a list from its JSON. Unknown or repeated members, and a name listed twice, are refused.</summary>
    /// <param name="json">The JSON, as UTF-8.</param>
    /// <param name="file">The file's name, for messages.</param>
    /// <returns>The list.</returns>
    /// <exception cref="InvalidDataException">The JSON is malformed.</exception>
    internal static ExpectedSkips Parse(ReadOnlyMemory<byte> json, string file)
    {
        ArgumentNullException.ThrowIfNull(file);
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(json, ScenarioSuite.Strict);
        }
        catch (JsonException e)
        {
            throw new InvalidDataException($"{file}: {e.Message}", e);
        }

        using (document)
        {
            var members = ScenarioSuite.Members(document.RootElement, "the list", FileMembers, file);
            var uncovered = new List<string>();
            if (members.TryGetValue("uncovered_rpcs", out var rpcs))
            {
                if (rpcs.ValueKind != JsonValueKind.Array)
                {
                    throw Malformed(file, $"uncovered_rpcs is {ScenarioSuite.Kind(rpcs)}, want a list");
                }

                foreach (var rpc in rpcs.EnumerateArray())
                {
                    var name = Text(rpc, "an entry of uncovered_rpcs", file);
                    if (uncovered.Contains(name, StringComparer.Ordinal))
                    {
                        throw Malformed(file, $"uncovered_rpcs lists {Spelling.Quote(name)} twice");
                    }

                    uncovered.Add(name);
                }
            }

            var unexpressible = new Dictionary<string, string>(StringComparer.Ordinal);
            if (members.TryGetValue("unexpressible", out var ids))
            {
                foreach (var (id, reason) in ScenarioSuite.Members(ids, "unexpressible", null, file))
                {
                    var text = Text(reason, $"the reason of {Spelling.Quote(id)}", file);
                    if (text.Length == 0)
                    {
                        throw Malformed(file, $"unexpressible gives {Spelling.Quote(id)} no reason");
                    }

                    unexpressible.Add(id, text);
                }
            }

            return new ExpectedSkips(uncovered.ToFrozenSet(StringComparer.Ordinal), unexpressible.ToFrozenDictionary(StringComparer.Ordinal));
        }
    }

    /// <summary>
    /// What is wrong with the list against a suite and the adapters: an RPC the service does not
    /// declare, an RPC listed uncovered that an adapter covers or an RPC no adapter covers that is not
    /// listed, a scenario id the suite does not declare, and a scenario of an uncovered RPC listed again.
    /// </summary>
    /// <param name="scenarios">The suite.</param>
    /// <param name="service">The service the scenarios address.</param>
    /// <param name="covered">The RPCs an adapter covers, by bare name.</param>
    /// <returns>Every problem, in a stable order; empty when there is none.</returns>
    internal IReadOnlyList<string> Problems(IEnumerable<Scenario> scenarios, ServiceDescriptor service, IEnumerable<string> covered)
    {
        ArgumentNullException.ThrowIfNull(scenarios);
        ArgumentNullException.ThrowIfNull(service);
        ArgumentNullException.ThrowIfNull(covered);
        var problems = new List<string>();
        var adapted = covered.ToHashSet(StringComparer.Ordinal);
        foreach (var rpc in UncoveredRpcs.Order(Spelling.CodePointOrder))
        {
            if (service.FindMethodByName(rpc) is null)
            {
                problems.Add($"the expected skips list {rpc} as uncovered, which is no RPC of {service.FullName}");
            }
            else if (adapted.Contains(rpc))
            {
                problems.Add($"the expected skips list {rpc} as uncovered, but an adapter covers it");
            }
        }

        foreach (var method in service.Methods.Where(method => !adapted.Contains(method.Name) && !UncoveredRpcs.Contains(method.Name)))
        {
            problems.Add($"no adapter covers {method.Name}, and the expected skips do not list it as uncovered");
        }

        var byId = scenarios.ToDictionary(scenario => scenario.Id, StringComparer.Ordinal);
        foreach (var id in Unexpressible.Keys.Order(Spelling.CodePointOrder))
        {
            if (!byId.TryGetValue(id, out var scenario))
            {
                problems.Add($"the expected skips name scenario {id}, which the suite does not declare");
            }
            else if (UncoveredRpcs.Contains(scenario.Method))
            {
                problems.Add($"the expected skips name scenario {id} as unexpressible, but its RPC {scenario.Method} is listed as uncovered");
            }
        }

        return problems;
    }

    /// <summary>Why a scenario is expected to be skipped, or null when it is expected to run.</summary>
    /// <param name="id">The scenario's id.</param>
    /// <param name="rpc">Its bare RPC.</param>
    /// <returns>The reason the adapter gives, or null.</returns>
    internal string? ReasonFor(string id, string rpc) =>
        UncoveredRpcs.Contains(rpc) ? ApiAdapters.Uncovered(rpc)
        : Unexpressible.TryGetValue(id, out var reason) ? reason
        : null;

    private static string Text(JsonElement element, string what, string file) =>
        element.ValueKind == JsonValueKind.String
            ? element.GetString()!
            : throw Malformed(file, $"{what} is {ScenarioSuite.Kind(element)}, want text");

    private static InvalidDataException Malformed(string file, string problem) => new($"{file}: {problem}");
}
