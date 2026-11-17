using System.Collections.Generic;
using System.Text;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>Builds normalized trees and expectations as scenarios and the normalizer write them.</summary>
internal static class Trees
{
    /// <summary>A message or map node.</summary>
    internal static SortedDictionary<string, object?> Map(params (string Key, object? Value)[] entries)
    {
        var map = new SortedDictionary<string, object?>(Spelling.CodePointOrder);
        foreach (var (key, value) in entries)
        {
            map.Add(key, value);
        }

        return map;
    }

    /// <summary>A list node.</summary>
    internal static List<object?> List(params object?[] items) => [.. items];

    /// <summary>An expectation, read from the JSON of a scenario's <c>expect</c> by the suite's own reader.</summary>
    internal static Expectation Expect(string json) => Scenario($$"""{"id":"t","rpc":"Evaluate","expect":{{json}}}""").Expect;

    /// <summary>One scenario, read from its JSON by the suite's own reader.</summary>
    internal static Scenario Scenario(string json) =>
        ScenarioSuite.Parse(Encoding.UTF8.GetBytes($$"""{"scenarios":[{{json}}]}"""), "test.json")[0];
}
