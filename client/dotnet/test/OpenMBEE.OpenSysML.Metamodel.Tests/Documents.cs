using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>JSON documents built for the reader's tests, and the reads of them.</summary>
internal static class Documents
{
    /// <summary>The sysml-toolkit <c>full-json</c> export the Python client's reader is tested against, read in place.</summary>
    internal static string ToolkitFixture { get; } = OpenMBEE.OpenSysML.Tests.Repository.PathOf("client", "python", "tests", "fixtures", "metamodel", "bigger.full.json");

    /// <summary>A part usage with <c>@id</c> <c>e</c> and nothing else.</summary>
    internal static JsonObject Element() => Element("e", "PartUsage");

    /// <summary>An element object with its <c>@id</c>, <c>@type</c> and properties.</summary>
    internal static JsonObject Element(string id, string type, params (string Key, JsonNode? Value)[] properties)
    {
        var element = new JsonObject { ["@id"] = id, ["@type"] = type };
        foreach (var (key, value) in properties)
        {
            element[key] = value?.DeepClone();
        }

        return element;
    }

    /// <summary>A reference by <c>@id</c>.</summary>
    internal static JsonObject Ref(string id) => new() { ["@id"] = id };

    /// <summary>An array of the nodes.</summary>
    internal static JsonArray Items(params JsonNode?[] items) => new([.. items.Select(item => item?.DeepClone())]);

    /// <summary>Reads the elements as an array document, parsed by the caller's <see cref="JsonDocument"/>.</summary>
    internal static MetamodelGraph Read(params JsonNode[] elements) => Read(new ReadOptions(), elements);

    /// <summary>Reads the elements as an array document, parsed by the caller's <see cref="JsonDocument"/>.</summary>
    internal static MetamodelGraph Read(ReadOptions options, params JsonNode[] elements) => ReadDocument(Items(elements), options);

    /// <summary>Reads a document parsed by the caller's <see cref="JsonDocument"/>, then disposes it.</summary>
    internal static MetamodelGraph ReadDocument(JsonNode document, ReadOptions? options = null)
    {
        using var parsed = JsonDocument.Parse(document.ToJsonString());
        return MetamodelGraph.ReadJson(parsed, options ?? new ReadOptions());
    }

    /// <summary>Reads JSON text as a UTF-8 stream, as the reader parses a file.</summary>
    internal static MetamodelGraph ReadText(string json, ReadOptions? options = null)
    {
        using var stream = new MemoryStream(Encoding.UTF8.GetBytes(json));
        return MetamodelGraph.ReadJson(stream, options ?? new ReadOptions());
    }
}
