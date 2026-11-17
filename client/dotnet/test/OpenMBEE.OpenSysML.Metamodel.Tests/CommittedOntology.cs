using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using OpenMBEE.OpenSysML.Tests;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>
/// The metaclass hierarchy as the repository's committed Turtle states it
/// (internal/translate/rdf/ontology/sysml-metaclasses.ttl), read independently of the generated code,
/// and the provenance constants of the table both were generated from.
/// </summary>
internal static partial class CommittedOntology
{
    /// <summary>Every metaclass, by name.</summary>
    internal static IReadOnlyDictionary<string, CommittedMetaclass> Metaclasses { get; } = ReadHierarchy();

    /// <summary>A Go string constant of internal/translate/rdf/ontology/table.go.</summary>
    internal static string TableConstant(string name)
    {
        var table = File.ReadAllText(Repository.PathOf("internal", "translate", "rdf", "ontology", "table.go"));
        var match = Regex.Match(table, $"^const {Regex.Escape(name)} = \"([^\"]*)\"$", RegexOptions.Multiline);
        return match.Success ? match.Groups[1].Value : throw new InvalidOperationException($"table.go declares no {name}");
    }

    /// <summary>The metaclass and every superclass of it, by the Turtle's subClassOf statements.</summary>
    internal static IReadOnlySet<string> AncestorsOrSelf(string name)
    {
        var closure = new HashSet<string>(StringComparer.Ordinal);
        var pending = new Stack<string>([name]);
        while (pending.TryPop(out var next))
        {
            if (closure.Add(next))
            {
                foreach (var parent in Metaclasses[next].Parents)
                {
                    pending.Push(parent);
                }
            }
        }

        return closure;
    }

    private static Dictionary<string, CommittedMetaclass> ReadHierarchy()
    {
        var text = File.ReadAllText(Repository.PathOf("internal", "translate", "rdf", "ontology", "sysml-metaclasses.ttl"));
        var result = new Dictionary<string, CommittedMetaclass>(StringComparer.Ordinal);
        foreach (Match block in Block().Matches(text))
        {
            var name = block.Groups["name"].Value;
            var body = block.Groups["body"].Value;
            var parents = SubClassOf().Match(body) is { Success: true } statement
                ? statement.Groups["parents"].Value.Split(',', StringSplitOptions.TrimEntries).Select(parent => parent["sysml:".Length..]).ToArray()
                : [];
            result.Add(name, new CommittedMetaclass(name, parents, body.Contains("sysx:abstract \"true\"^^xsd:boolean", StringComparison.Ordinal)));
        }

        return result.Count > 0 ? result : throw new InvalidOperationException("the committed hierarchy declares no metaclass");
    }

    [GeneratedRegex(@"^sysml:(?<name>\w+)\n(?<body>(?:    .*\n)+)", RegexOptions.Multiline)]
    private static partial Regex Block();

    [GeneratedRegex(@"rdfs:subClassOf (?<parents>sysml:\w+(?:, sysml:\w+)*)")]
    private static partial Regex SubClassOf();
}

/// <summary>One metaclass of the committed hierarchy.</summary>
internal sealed record CommittedMetaclass(string Name, IReadOnlyList<string> Parents, bool IsAbstract);
