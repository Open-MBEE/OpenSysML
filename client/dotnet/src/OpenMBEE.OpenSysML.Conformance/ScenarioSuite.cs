using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Reads the scenario files of a suite: every <c>*.json</c> of a directory, in file then
/// declaration order, so a run reports its scenarios in a stable order. Reading is strict: a
/// member the format does not define, a member stated twice, a value of the wrong kind, a scenario
/// without an id or an RPC, a model naming no fixture or both forms, and an id declared twice are
/// all refused, since a scenario silently read otherwise than it was written asserts nothing.
/// </summary>
internal static class ScenarioSuite
{
    private static readonly FrozenSet<string> FileMembers = FrozenSet.Create(StringComparer.Ordinal, "scenarios");

    private static readonly FrozenSet<string> ScenarioMembers = FrozenSet.Create(
        StringComparer.Ordinal,
        "id",
        "description",
        "rpc",
        "requires_capabilities",
        "expect_without_capability",
        "model",
        "request",
        "expect");

    private static readonly FrozenSet<string> ModelMembers = FrozenSet.Create(
        StringComparer.Ordinal, "fixture", "fixtures", "language", "strict_conformance");

    private static readonly FrozenSet<string> ExpectMembers = FrozenSet.Create(
        StringComparer.Ordinal,
        "status",
        "status_message_contains",
        "response",
        "contains",
        "contains_all",
        "non_empty",
        "absent",
        "counts",
        "min_counts");

    /// <summary>How the suite's JSON is read: no comments, no trailing commas.</summary>
    internal static readonly JsonDocumentOptions Strict = new()
    {
        AllowTrailingCommas = false,
        CommentHandling = JsonCommentHandling.Disallow,
        MaxDepth = 512,
    };

    private static readonly EnumerationOptions JsonFiles = new()
    {
        MatchType = MatchType.Simple,
        MatchCasing = MatchCasing.CaseSensitive,
        RecurseSubdirectories = false,
        IgnoreInaccessible = false,
    };

    /// <summary>Reads every scenario file of a directory.</summary>
    /// <param name="directory">The directory holding the scenario files, conformance/scenarios for the suite.</param>
    /// <returns>Every scenario, in file name then declaration order.</returns>
    /// <exception cref="InvalidDataException">The directory holds no scenario file, or one is malformed.</exception>
    /// <exception cref="IOException">A file cannot be read.</exception>
    internal static IReadOnlyList<Scenario> Load(string directory)
    {
        ArgumentNullException.ThrowIfNull(directory);
        var files = Directory.Exists(directory)
            ? Directory.GetFiles(directory, "*.json", JsonFiles).Order(Spelling.CodePointOrder).ToList()
            : [];
        if (files.Count == 0)
        {
            throw new InvalidDataException($"no scenario files in {directory}");
        }

        var scenarios = new List<Scenario>();
        var declared = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var file in files)
        {
            foreach (var scenario in Parse(File.ReadAllBytes(file), file))
            {
                if (!declared.TryAdd(scenario.Id, file))
                {
                    throw new InvalidDataException(
                        $"{file}: scenario id {Spelling.Quote(scenario.Id)} is already declared in {declared[scenario.Id]}");
                }

                scenarios.Add(scenario);
            }
        }

        return scenarios;
    }

    /// <summary>Reads the scenarios of one scenario file.</summary>
    /// <param name="json">The file's content, as UTF-8.</param>
    /// <param name="file">The file's name, for messages and <see cref="Scenario.File"/>.</param>
    /// <returns>Its scenarios, in declaration order.</returns>
    /// <exception cref="InvalidDataException">The file is malformed.</exception>
    internal static IReadOnlyList<Scenario> Parse(ReadOnlyMemory<byte> json, string file)
    {
        ArgumentNullException.ThrowIfNull(file);
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(json, Strict);
        }
        catch (JsonException e)
        {
            throw new InvalidDataException($"{file}: {e.Message}", e);
        }

        using (document)
        {
            var members = Members(document.RootElement, "the file", FileMembers, file);
            var scenarios = new List<Scenario>();
            if (!Present(members, "scenarios", out var list))
            {
                return scenarios;
            }

            if (list.ValueKind != JsonValueKind.Array)
            {
                throw Malformed(file, $"scenarios is {Kind(list)}, want a list");
            }

            var index = 0;
            foreach (var element in list.EnumerateArray())
            {
                scenarios.Add(ReadScenario(element, $"scenario {index}", file));
                index++;
            }

            return scenarios;
        }
    }

    private static Scenario ReadScenario(JsonElement element, string where, string file)
    {
        var members = Members(element, where, ScenarioMembers, file);
        var id = Text(members, "id", where, file);
        var rpc = Text(members, "rpc", where, file);
        if (id.Length == 0 || rpc.Length == 0)
        {
            throw Malformed(file, "every scenario needs an id and an rpc");
        }

        ScenarioModel? model = null;
        if (Present(members, "model", out var modelElement))
        {
            model = ReadModel(modelElement, $"{id}: model", file);
        }

        Expectation? withoutCapability = null;
        if (Present(members, "expect_without_capability", out var fallback))
        {
            withoutCapability = ReadExpectation(fallback, $"{id}: expect_without_capability", file);
        }

        return new Scenario(
            id,
            Text(members, "description", id, file),
            rpc,
            Texts(members, "requires_capabilities", id, file),
            withoutCapability,
            model,
            Present(members, "request", out var request) ? request.Clone() : null,
            Present(members, "expect", out var expect) ? ReadExpectation(expect, $"{id}: expect", file) : Expectation.Success,
            file);
    }

    private static ScenarioModel ReadModel(JsonElement element, string where, string file)
    {
        var members = Members(element, where, ModelMembers, file);
        var model = new ScenarioModel(
            Text(members, "fixture", where, file),
            Texts(members, "fixtures", where, file),
            Text(members, "language", where, file),
            Flag(members, "strict_conformance", where, file));
        return (model.Fixture.Length == 0, model.Fixtures.Count == 0) switch
        {
            (true, true) => throw Malformed(file, $"{where}: a model needs a fixture or fixtures"),
            (false, false) => throw Malformed(file, $"{where}: a model names either a fixture or fixtures, not both"),
            _ => model,
        };
    }

    private static Expectation ReadExpectation(JsonElement element, string where, string file)
    {
        var members = Members(element, where, ExpectMembers, file);
        JsonElement? response = null;
        if (Present(members, "response", out var tree))
        {
            if (tree.ValueKind != JsonValueKind.Object)
            {
                throw Malformed(file, $"{where}: response is {Kind(tree)}, want an object");
            }

            RejectRepeatedMembers(tree, $"{where}: response", file);
            response = tree.Clone();
        }

        return new Expectation(
            Text(members, "status", where, file),
            Text(members, "status_message_contains", where, file),
            response,
            Map(members, "contains", where, file, (value, at) => Text(value, at, file)),
            Map<IReadOnlyList<string>>(members, "contains_all", where, file, (value, at) => Texts(value, at, file)),
            Texts(members, "non_empty", where, file),
            Texts(members, "absent", where, file),
            Map(members, "counts", where, file, (value, at) => Count(value, at, file)),
            Map(members, "min_counts", where, file, (value, at) => Count(value, at, file)));
    }

    /// <summary>
    /// The members of an object, refusing anything but an object, a member <paramref name="allowed"/>
    /// does not name when it names some, and a member stated twice.
    /// </summary>
    internal static Dictionary<string, JsonElement> Members(
        JsonElement element, string where, FrozenSet<string>? allowed, string file)
    {
        if (element.ValueKind != JsonValueKind.Object)
        {
            throw Malformed(file, $"{where} is {Kind(element)}, want an object");
        }

        var members = new Dictionary<string, JsonElement>(StringComparer.Ordinal);
        foreach (var member in element.EnumerateObject())
        {
            if (allowed is not null && !allowed.Contains(member.Name))
            {
                throw Malformed(file, $"{where}: unknown member {Spelling.Quote(member.Name)}");
            }

            if (!members.TryAdd(member.Name, member.Value))
            {
                throw Malformed(file, $"{where}: member {Spelling.Quote(member.Name)} is stated twice");
            }
        }

        return members;
    }

    /// <summary>Refuses an object anywhere in a tree that states a member twice, which would leave what it expects ambiguous.</summary>
    private static void RejectRepeatedMembers(JsonElement tree, string where, string file)
    {
        switch (tree.ValueKind)
        {
            case JsonValueKind.Object:
                foreach (var (name, member) in Members(tree, where, allowed: null, file))
                {
                    RejectRepeatedMembers(member, $"{where}.{name}", file);
                }

                break;
            case JsonValueKind.Array:
                var index = 0;
                foreach (var item in tree.EnumerateArray())
                {
                    RejectRepeatedMembers(item, $"{where}.{index}", file);
                    index++;
                }

                break;
        }
    }

    /// <summary>Whether a member is there; a null member is not, as it is not in the reference's reading.</summary>
    private static bool Present(Dictionary<string, JsonElement> members, string name, out JsonElement value) =>
        members.TryGetValue(name, out value) && value.ValueKind != JsonValueKind.Null;

    private static string Text(Dictionary<string, JsonElement> members, string name, string where, string file) =>
        Present(members, name, out var value) ? Text(value, $"{where}: {name}", file) : "";

    private static string Text(JsonElement value, string where, string file) =>
        value.ValueKind == JsonValueKind.String
            ? value.GetString()!
            : throw Malformed(file, $"{where} is {Kind(value)}, want text");

    private static List<string> Texts(Dictionary<string, JsonElement> members, string name, string where, string file) =>
        Present(members, name, out var value) ? Texts(value, $"{where}: {name}", file) : [];

    private static List<string> Texts(JsonElement value, string where, string file)
    {
        if (value.ValueKind != JsonValueKind.Array)
        {
            throw Malformed(file, $"{where} is {Kind(value)}, want a list of text");
        }

        return [.. value.EnumerateArray().Select((item, index) => Text(item, $"{where}.{index}", file))];
    }

    private static bool Flag(Dictionary<string, JsonElement> members, string name, string where, string file)
    {
        if (!Present(members, name, out var value))
        {
            return false;
        }

        return value.ValueKind switch
        {
            JsonValueKind.True => true,
            JsonValueKind.False => false,
            _ => throw Malformed(file, $"{where}: {name} is {Kind(value)}, want true or false"),
        };
    }

    private static int Count(JsonElement value, string where, string file) =>
        value.ValueKind == JsonValueKind.Number && value.TryGetInt32(out var count)
            ? count
            : throw Malformed(file, $"{where} is {value.GetRawText()}, want a whole number");

    private static Dictionary<string, T> Map<T>(
        Dictionary<string, JsonElement> members, string name, string where, string file, Func<JsonElement, string, T> read)
    {
        var map = new Dictionary<string, T>(StringComparer.Ordinal);
        if (!Present(members, name, out var value))
        {
            return map;
        }

        foreach (var (path, entry) in Members(value, $"{where}: {name}", allowed: null, file))
        {
            map[path] = read(entry, $"{where}: {name}.{path}");
        }

        return map;
    }

    private static InvalidDataException Malformed(string file, string problem) => new($"{file}: {problem}");

    /// <summary>A JSON value's kind, as a message names it: <c>an object</c>, <c>a list</c>, <c>text</c>, ….</summary>
    internal static string Kind(JsonElement element) => element.ValueKind switch
    {
        JsonValueKind.Object => "an object",
        JsonValueKind.Array => "a list",
        JsonValueKind.String => "text",
        JsonValueKind.Number => "a number",
        JsonValueKind.True or JsonValueKind.False => "a boolean",
        _ => "null",
    };
}
