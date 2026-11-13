using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Xunit;
using static OpenMBEE.OpenSysML.Metamodel.Tests.Documents;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>The keys a document writes that no metamodel property has, kept on request and independent of the source.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReaderExtensionTests : IDisposable
{
    private static readonly ReadOptions Keep = new() { KeepExtensions = true };

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-extensions-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    /// <summary>By default nothing is kept, and asking for it says how to keep it rather than answering empty.</summary>
    [Fact]
    public void ExtensionsAreNotKeptByDefault()
    {
        Assert.False(new ReadOptions().KeepExtensions);
        var graph = Read(Element("e", "PartUsage", ("sysx:sourceText", "part e;")), Element("plain", "Namespace"));

        Assert.False(graph.KeepsExtensions);
        Assert.All(graph.Elements, element => Assert.Contains(
            "ReadOptions.KeepExtensions",
            Assert.Throws<InvalidOperationException>(() => element.Extensions).Message,
            StringComparison.Ordinal));
        Assert.Throws<InvalidOperationException>(() => Metaclasses.PartUsage.CreateElement("x", "PartUsage").Extensions);
    }

    /// <summary>Kept, every member the metaclass has no property for is there as written, and only those.</summary>
    [Fact]
    public void KeptExtensionsAreTheMembersNoPropertyHasAsWritten()
    {
        var mapping = new JsonObject { ["type"] = Items("Real"), ["relatedElement"] = Items(Ref("e"), "Real") };
        var graph = Read(
            Keep,
            Element(
                "e",
                "AttributeUsage",
                ("sysx:sourceText", "attribute power : Real = 300.0;"),
                ("declaredName", "power"),
                ("sysx:sourceLine", 3),
                ("sysx:hasBody", false),
                ("sysx:mappingValues", mapping),
                ("value", Items(Ref("e"), null)),
                ("bogus", null),
                ("@context", "https://example.org/context")),
            Element("plain", "Namespace", ("declaredName", "P")));

        Assert.True(graph.KeepsExtensions);
        var usage = (IAttributeUsage)graph["e"];
        var extensions = usage.Extensions;
        Assert.Equal(
            ["@context", "bogus", "sysx:hasBody", "sysx:mappingValues", "sysx:sourceLine", "sysx:sourceText", "value"],
            extensions.Keys.Order(StringComparer.Ordinal));
        Assert.Equal("attribute power : Real = 300.0;", extensions["sysx:sourceText"].GetString());
        Assert.Equal(3, extensions["sysx:sourceLine"].GetInt32());
        Assert.Equal(JsonValueKind.False, extensions["sysx:hasBody"].ValueKind);
        Assert.Equal(JsonValueKind.Null, extensions["bogus"].ValueKind);
        Assert.True(JsonNode.DeepEquals(mapping, JsonNode.Parse(extensions["sysx:mappingValues"].GetRawText())));
        Assert.Equal(2, extensions["value"].GetArrayLength());
        Assert.Equal("power", usage.DeclaredName);
        Assert.Throws<UnknownJsonKeyException>(() => usage.GetValue("sysx:sourceText"));
        Assert.Empty(graph["plain"].Extensions);
        Assert.Throws<NotSupportedException>(() => ((IDictionary<string, JsonElement>)extensions).Add("x", default));
    }

    /// <summary>
    /// An element of a type the metamodel lacks keeps the keys of its own type beyond those of the metaclass it
    /// is read as, unvalidated: they are not that metaclass's properties.
    /// </summary>
    [Fact]
    public void AnUnrecognizedTypeKeepsTheKeysItsFallbackLacks()
    {
        var special = Read(Keep, Element("e", "vendor:SpecialPart", ("declaredName", "special"), ("partDefinition", Items(Ref("missing"), null))))["e"];

        Assert.Same(Metaclasses.Element, special.MetaclassInfo);
        Assert.Equal(["partDefinition"], special.Extensions.Keys);
        Assert.Equal(JsonValueKind.Null, special.Extensions["partDefinition"][1].ValueKind);
        Assert.Equal("special", ((IElement)special).DeclaredName);
    }

    /// <summary>Kept extensions read after every kind of source is disposed or deleted, so none is held.</summary>
    [Fact]
    public void KeptExtensionsOutliveEverySource()
    {
        var text = Items(Element("e", "PartUsage", ("sysx:sourceText", "part e;"), ("sysx:memberIndex", 7))).ToJsonString();
        var path = Path.Combine(directory, "elements.json");
        File.WriteAllText(path, text);

        var fromPath = MetamodelGraph.ReadJson(path, Keep);
        File.Delete(path);
        MetamodelGraph fromStream;
        using (var stream = new MemoryStream(Encoding.UTF8.GetBytes(text)))
        {
            fromStream = MetamodelGraph.ReadJson(stream, Keep);
        }

        MetamodelGraph fromDocument;
        MetamodelGraph fromElements;
        using (var document = JsonDocument.Parse(text))
        {
            fromDocument = MetamodelGraph.ReadJson(document, Keep);
            fromElements = MetamodelGraph.ReadJson(document.RootElement.EnumerateArray().ToArray(), Keep);
        }

        Assert.All(new[] { fromPath, fromStream, fromDocument, fromElements }, graph =>
        {
            Assert.Equal("part e;", graph["e"].Extensions["sysx:sourceText"].GetString());
            Assert.Equal(7, graph["e"].Extensions["sysx:memberIndex"].GetInt32());
        });
    }

    /// <summary>The sysml-toolkit export writes only metamodel properties, so every element keeps nothing.</summary>
    [Fact]
    public void TheToolkitFixtureHasNoExtensions()
    {
        var graph = MetamodelGraph.ReadJson(ToolkitFixture, Keep);

        Assert.Equal(87, graph.Elements.Count);
        Assert.All(graph.Elements, element => Assert.Empty(element.Extensions));
    }
}
