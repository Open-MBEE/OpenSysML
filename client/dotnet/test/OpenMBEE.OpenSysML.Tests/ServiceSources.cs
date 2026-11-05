using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using OpenMBEE.OpenSysML.Conformance;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>One conformance scenario, as the parity inventory reads it.</summary>
/// <param name="Id">The scenario's ID.</param>
/// <param name="Rpc">The bare name of the method it calls.</param>
/// <param name="RequiredCapabilities">The capabilities it requires of the service.</param>
internal sealed record Scenario(string Id, string Rpc, IReadOnlyList<string> RequiredCapabilities)
{
    /// <summary>Every scenario in conformance/scenarios, in file order, read by the conformance runner's own reader.</summary>
    internal static IReadOnlyList<Scenario> All { get; } =
        [.. ScenarioSuite.Load(Repository.PathOf("conformance", "scenarios"))
            .Select(scenario => new Scenario(scenario.Id, scenario.Method, scenario.RequiresCapabilities))];
}

/// <summary>The capability names the service declares, read from its Go source.</summary>
internal static partial class ServiceCapabilities
{
    /// <summary>Every capability name a <c>const CapabilityX = "name"</c> of the service declares.</summary>
    internal static IReadOnlySet<string> Names { get; } = Load();

    [GeneratedRegex("""^const Capability\w+ = "([a-z_]+)"$""", RegexOptions.Multiline)]
    private static partial Regex Declaration();

    private static HashSet<string> Load()
    {
        var names = new HashSet<string>(StringComparer.Ordinal);
        foreach (var path in Directory.GetFiles(Repository.PathOf("internal", "frontend", "grpc"), "*.go"))
        {
            if (path.EndsWith("_test.go", StringComparison.Ordinal))
            {
                continue;
            }

            foreach (Match declaration in Declaration().Matches(File.ReadAllText(path)))
            {
                names.Add(declaration.Groups[1].Value);
            }
        }

        return names;
    }
}
