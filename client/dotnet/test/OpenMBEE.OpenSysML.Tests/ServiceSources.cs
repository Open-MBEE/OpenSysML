using System;
using System.Collections.Generic;
using System.IO;
using System.Text.RegularExpressions;

namespace OpenMBEE.OpenSysML.Tests;

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
