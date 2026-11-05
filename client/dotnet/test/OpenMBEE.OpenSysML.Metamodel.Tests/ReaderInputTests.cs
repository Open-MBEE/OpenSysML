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

/// <summary>What the reader accepts as a document, how it types elements, and the documents it refuses.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReaderInputTests : IDisposable
{
    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-metamodel-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    /// <summary>A single element object, an array, a file, a stream, a parsed document and a sequence of elements all read.</summary>
    [Fact]
    public void ReadsEveryKindOfSource()
    {
        var engine = Element("e", "PartUsage", ("declaredName", "engine"));
        var text = engine.ToJsonString();
        var path = Path.Combine(directory, "elements.json");
        File.WriteAllText(path, Items(engine).ToJsonString());
        using var stream = new MemoryStream(Encoding.UTF8.GetBytes(text));
        using var document = JsonDocument.Parse(text);
        using var array = JsonDocument.Parse(Items(engine, Element("d", "PartDefinition")).ToJsonString());

        var graphs = new[]
        {
            ReadDocument(engine),
            Read(engine),
            MetamodelGraph.ReadJson(path),
            MetamodelGraph.ReadJson(path, new ReadOptions()),
            MetamodelGraph.ReadJson(stream),
            MetamodelGraph.ReadJson(document),
            MetamodelGraph.ReadJson(array.RootElement.EnumerateArray()),
            MetamodelGraph.ReadJson(array.RootElement.EnumerateArray(), new ReadOptions()),
        };

        Assert.All(graphs, graph => Assert.Equal("engine", ((IPartUsage)graph["e"]).DeclaredName));
        Assert.Equal(["e", "d"], graphs[^1].Elements.Select(element => element.Id));
        Assert.Equal("e", Assert.Single(graphs[1].All<IPartUsage>()).Id);
    }

    /// <summary>The reader neither disposes a caller's stream or document nor needs either once it returns.</summary>
    [Fact]
    public void BorrowedSourcesStayTheCallersAndTheGraphOutlivesThem()
    {
        var document = JsonDocument.Parse(Items(
            Element("d", "PartDefinition", ("declaredName", "Engine"), ("ownedFeature", Items(Ref("u")))),
            Element("u", "PartUsage", ("declaredName", "engine"), ("partDefinition", Items(Ref("d"))), ("direction", "in"), ("aliasIds", Items("a", "b")))).ToJsonString());
        var graph = MetamodelGraph.ReadJson(document);
        Assert.Equal(JsonValueKind.Array, document.RootElement.ValueKind);
        document.Dispose();

        var usage = (IPartUsage)graph["u"];
        var definition = (IPartDefinition)graph["d"];
        Assert.Equal(("engine", FeatureDirectionKind.In), (usage.DeclaredName, usage.Direction));
        Assert.Equal(["a", "b"], usage.AliasIds);
        Assert.Same(definition, Assert.Single(usage.PartDefinition));
        Assert.Same(usage, Assert.Single(definition.OwnedFeature));
        Assert.All(
            graph.Elements.SelectMany(element => element.MetaclassInfo.Properties.Select(property => (element, property))),
            pair => Assert.IsNotType<JsonElement>(Value(pair.element, pair.property)));

        using var stream = new MemoryStream(Encoding.UTF8.GetBytes(Element().ToJsonString()));
        MetamodelGraph.ReadJson(stream);
        Assert.True(stream.CanRead);
        stream.Position = 0;
        Assert.Equal("e", MetamodelGraph.ReadJson(stream).Elements.Single().Id);

        JsonElement[] elements;
        using (var items = JsonDocument.Parse(Items(Element("x", "Namespace")).ToJsonString()))
        {
            elements = [.. items.RootElement.EnumerateArray()];
            graph = MetamodelGraph.ReadJson(elements);
        }

        Assert.Throws<ObjectDisposedException>(() => elements[0].GetRawText());
        Assert.IsAssignableFrom<INamespace>(graph["x"]);
    }

    /// <summary>DataVersion and Commit envelopes are unwrapped, a null payload holding no element.</summary>
    [Fact]
    public void UnwrapsDataVersionAndCommitEnvelopes()
    {
        static JsonObject DataVersion(JsonNode? payload) => new() { ["identity"] = new JsonObject { ["@id"] = "identity" }, ["payload"] = payload?.DeepClone() };
        var first = Element("first", "PartUsage", ("declaredName", "one"));
        var second = Element("second", "PartUsage", ("declaredName", "two"));

        var versions = ReadDocument(Items(DataVersion(first), DataVersion(null)));
        Assert.Equal("first", Assert.Single(versions.Elements).Id);

        var commit = ReadDocument(new JsonObject { ["@type"] = "Commit", ["change"] = Items(DataVersion(second), DataVersion(first)) });
        Assert.Equal(["second", "first"], commit.Elements.Select(element => element.Id));
        Assert.Equal("two", ((IElement)commit["second"]).DeclaredName);

        Assert.Equal("one", ((IElement)ReadDocument(DataVersion(first))["first"]).DeclaredName);
        Assert.Empty(ReadDocument(DataVersion(null)).Elements);

        using var mixed = JsonDocument.Parse(Items(DataVersion(first), second).ToJsonString());
        Assert.Equal(["first", "second"], MetamodelGraph.ReadJson(mixed.RootElement.EnumerateArray()).Elements.Select(element => element.Id));
    }

    /// <summary>A <c>sysml:</c> prefix and the SysML namespace IRI, unversioned or versioned, name the metaclass; the written type is kept.</summary>
    [Theory]
    [InlineData("PartUsage")]
    [InlineData("sysml:PartUsage")]
    [InlineData("https://www.omg.org/spec/SysML#PartUsage")]
    [InlineData("https://www.omg.org/spec/SysML/20250201/PartUsage")]
    public void NormalizesKnownTypesAndKeepsTheWrittenType(string written)
    {
        var element = Read(Element("e", written))["e"];

        Assert.IsType<Elements.PartUsage>(element);
        Assert.Same(Metaclasses.PartUsage, element.MetaclassInfo);
        Assert.Equal(written, element.JsonType);
    }

    /// <summary>Only an eight-digit release versions the SysML namespace; anything else is a type the metamodel lacks.</summary>
    [Theory]
    [InlineData("https://www.omg.org/spec/SysML/2025020/PartUsage")]
    [InlineData("https://www.omg.org/spec/SysML/2025020x/PartUsage")]
    [InlineData("https://www.omg.org/spec/SysML/20250201PartUsage")]
    [InlineData("https://www.omg.org/spec/SysML/")]
    [InlineData("Sysml:PartUsage")]
    [InlineData("partUsage")]
    public void OtherSpellingsAreNotRecognized(string written)
    {
        var element = Read(Element("e", written))["e"];

        Assert.Same(Metaclasses.Element, element.MetaclassInfo);
        Assert.IsNotAssignableFrom<IPartUsage>(element);
    }

    /// <summary>An <c>@type</c> the metamodel lacks is read as <c>Element</c>, or as the first metaclass along its supertypes.</summary>
    [Fact]
    public void UnknownTypesFallBackToElementOrFollowSupertypes()
    {
        var fallback = Read(Element("e", "vendor:SpecialPart", ("declaredName", "special"), ("partDefinition", "ignored")))["e"];
        Assert.IsAssignableFrom<IElement>(fallback);
        Assert.IsNotAssignableFrom<INamespace>(fallback);
        Assert.Same(Metaclasses.Element, fallback.MetaclassInfo);
        Assert.Equal("vendor:SpecialPart", fallback.JsonType);
        Assert.Equal("special", ((IElement)fallback).DeclaredName);
        Assert.Throws<UnknownJsonKeyException>(() => fallback.GetValue("partDefinition"));

        var supertypes = new Dictionary<string, string> { ["vendor:SpecialPart"] = "Intermediate", ["Intermediate"] = "sysml:PartUsage" };
        var specialized = Read(new ReadOptions { Supertypes = supertypes }, Element("e", "vendor:SpecialPart"))["e"];
        Assert.IsType<Elements.PartUsage>(specialized);
        Assert.Equal("vendor:SpecialPart", specialized.JsonType);

        var prefixed = new Dictionary<string, string> { ["https://www.omg.org/spec/SysML#Gadget"] = "https://www.omg.org/spec/SysML/20250201/ItemUsage" };
        Assert.IsType<Elements.ItemUsage>(Read(new ReadOptions { Supertypes = prefixed }, Element("e", "sysml:Gadget"))["e"]);

        var dangling = new Dictionary<string, string> { ["vendor:SpecialPart"] = "vendor:Unmapped" };
        Assert.Same(Metaclasses.Element, Read(new ReadOptions { Supertypes = dangling }, Element("e", "vendor:SpecialPart"))["e"].MetaclassInfo);

        var ignored = new Dictionary<string, string> { ["PartUsage"] = "Namespace" };
        Assert.IsType<Elements.PartUsage>(Read(new ReadOptions { Supertypes = ignored }, Element())["e"]);
    }

    /// <summary>A type a document writes as an abstract metaclass, or maps to one, is read through that metaclass's interface.</summary>
    [Fact]
    public void AbstractTypesAreReadThroughTheirInterfaces()
    {
        var graph = Read(
            new ReadOptions { Supertypes = new Dictionary<string, string> { ["vendor:Link"] = "Relationship" } },
            Element("element", "Element", ("declaredName", "plain")),
            Element("link", "vendor:Link", ("source", Items(Ref("element"))), ("isImplied", true)));

        var element = (IElement)graph["element"];
        var link = Assert.IsAssignableFrom<IRelationship>(graph["link"]);
        Assert.Equal("plain", element.DeclaredName);
        Assert.True(element.MetaclassInfo.IsAbstract);
        Assert.False(element.GetType().IsVisible);
        Assert.Same(Metaclasses.Relationship, link.MetaclassInfo);
        Assert.Same(element, Assert.Single(link.Source));
        Assert.True(link.IsImplied);
    }

    /// <summary>Supertypes that cycle fail the read; a mapping that is ambiguous once types are normalized is refused.</summary>
    [Fact]
    public void CyclicOrAmbiguousSupertypesAreRefused()
    {
        var cyclic = new ReadOptions { Supertypes = new Dictionary<string, string> { ["VendorTypeA"] = "VendorTypeB", ["VendorTypeB"] = "sysml:VendorTypeA" } };
        var failure = Assert.Throws<MalformedDocumentException>(() => Read(cyclic, Element("e", "VendorTypeA")));
        Assert.Contains("the supertypes of @type \"VendorTypeA\" form a cycle", failure.Message, StringComparison.Ordinal);
        Assert.Single(Read(cyclic, Element()).Elements);

        var ambiguous = new ReadOptions { Supertypes = new Dictionary<string, string> { ["Gadget"] = "PartUsage", ["sysml:Gadget"] = "ItemUsage" } };
        Assert.Throws<ArgumentException>("options", () => Read(ambiguous, Element()));
        var agreeing = new ReadOptions { Supertypes = new Dictionary<string, string> { ["Gadget"] = "PartUsage", ["sysml:Gadget"] = "sysml:PartUsage" } };
        Assert.IsType<Elements.PartUsage>(Read(agreeing, Element("e", "Gadget"))["e"]);
    }

    /// <summary>Documents that are not element graphs fail the read with <see cref="MalformedDocumentException"/>.</summary>
    [Theory]
    [InlineData("null", "not null")]
    [InlineData("42", "not 42")]
    [InlineData("\"text\"", "not \"text\"")]
    [InlineData("{", "the stream is not a JSON document")]
    [InlineData("[1]", "item 1 of the document is 1, not an element object")]
    [InlineData("[[]]", "item 1 of the document is an array")]
    [InlineData("[{\"@type\": \"Element\"}]", "element 1 of the document has no string @id")]
    [InlineData("[{\"@id\": \"missing-type\"}]", "element \"missing-type\" has no string @type")]
    [InlineData("[{\"@id\": 3, \"@type\": \"Element\"}]", "element 1 of the document has no string @id")]
    [InlineData("[{\"@id\": \"e\", \"@type\": 3}]", "element \"e\" has no string @type")]
    [InlineData("[{\"@id\": \"e\", \"@type\": \"Element\"}, {\"@id\": \"e\", \"@type\": \"Namespace\"}]", "two elements have @id \"e\"")]
    [InlineData("[{\"identity\": {}, \"payload\": 3}]", "the payload of DataVersion 1 is 3, not an element object or null")]
    [InlineData("{\"change\": [3]}", "item 1 of the document is 3")]
    [InlineData("{\"change\": {}}", "element 1 of the document has no string @id")]
    [InlineData("[{\"@id\": \"e\", \"@type\": \"Element\", \"declaredName\": \"a\", \"declaredName\": \"b\"}]", "key \"declaredName\" twice")]
    [InlineData("[{\"@id\": \"e\", \"@type\": \"Element\", \"value\": NaN}]", "the stream is not a JSON document")]
    [InlineData("[{\"@id\": \"e\", \"@type\": \"Element\", \"value\": Infinity}]", "the stream is not a JSON document")]
    public void MalformedDocumentsFailTheRead(string json, string reason)
    {
        var failure = Assert.Throws<MalformedDocumentException>(() => ReadText(json));

        Assert.Contains(reason, failure.Message, StringComparison.Ordinal);
    }

    /// <summary>A key twice in one object is malformed however the caller parsed the document, and invalid UTF-8 is not JSON.</summary>
    [Fact]
    public void DuplicateKeysAndInvalidEncodingFailTheRead()
    {
        foreach (var json in new[]
        {
            "[{\"@id\": \"e\", \"@type\": \"Element\", \"@id\": \"f\"}]",
            "[{\"@id\": \"e\", \"@type\": \"PartUsage\", \"partDefinition\": [{\"@id\": \"e\", \"@id\": \"f\"}]}]",
            "[{\"identity\": {}, \"payload\": {\"@id\": \"e\", \"@type\": \"Element\"}, \"payload\": null}]",
            "{\"change\": [], \"change\": []}",
        })
        {
            using var lenient = JsonDocument.Parse(json);
            var failure = Assert.Throws<MalformedDocumentException>(() => MetamodelGraph.ReadJson(lenient));
            Assert.Contains("twice", failure.Message, StringComparison.Ordinal);
            Assert.Throws<MalformedDocumentException>(() => ReadText(json));
        }

        using var invalid = new MemoryStream([(byte)'[', 0xff, (byte)']']);
        var notJson = Assert.Throws<MalformedDocumentException>(() => MetamodelGraph.ReadJson(invalid));
        Assert.IsType<JsonException>(notJson.InnerException, exactMatch: false);

        var path = Path.Combine(directory, "broken.json");
        File.WriteAllText(path, "{");
        Assert.StartsWith(path + " is not a JSON document", Assert.Throws<MalformedDocumentException>(() => MetamodelGraph.ReadJson(path)).Message, StringComparison.Ordinal);
        Assert.Throws<FileNotFoundException>(() => MetamodelGraph.ReadJson(Path.Combine(directory, "absent.json")));
    }

    /// <summary>Null arguments are refused.</summary>
    [Fact]
    public void NullArgumentsAreRefused()
    {
        using var document = JsonDocument.Parse("[]");
        Assert.Throws<ArgumentNullException>("path", () => MetamodelGraph.ReadJson((string)null!));
        Assert.Throws<ArgumentNullException>("stream", () => MetamodelGraph.ReadJson((Stream)null!));
        Assert.Throws<ArgumentNullException>("document", () => MetamodelGraph.ReadJson((JsonDocument)null!));
        Assert.Throws<ArgumentNullException>("elements", () => MetamodelGraph.ReadJson((IEnumerable<JsonElement>)null!));
        Assert.Throws<ArgumentNullException>("options", () => MetamodelGraph.ReadJson(document, null!));
        Assert.Throws<ArgumentNullException>("options", () => MetamodelGraph.ReadJson(new MemoryStream(), null!));
        Assert.Throws<ArgumentNullException>("options", () => MetamodelGraph.ReadJson("x.json", null!));
        Assert.Throws<ArgumentNullException>("options", () => MetamodelGraph.ReadJson(document.RootElement.EnumerateArray(), null!));
    }

    private static object? Value(IMetamodelElement element, MetaProperty property)
    {
        try
        {
            return element.GetValue(property.JsonKey);
        }
        catch (NotSuppliedException)
        {
            return null;
        }
    }
}
