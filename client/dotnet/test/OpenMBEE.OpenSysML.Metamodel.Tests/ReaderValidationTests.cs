using System;
using System.IO;
using System.Reflection;
using System.Text;
using System.Text.Json;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>Structural validation across every source, independent of property conversion and extension retention.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReaderValidationTests : IDisposable
{
    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-reader-validation-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [InlineData("""[{"@id":"e","@type":"Element","vendor:data":{"a":1,"a":2}}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","declaredName":{"a":1,"a":2}}]""")]
    [InlineData("""[{"@id":"e","@type":"PartUsage","partDefinition":[{"@id":"missing"},{"@id":"e","@id":"f"}]}]""")]
    [InlineData("""[{"@id":"e","@type":"PartUsage","partDefinition":[null,{"@id":"e","@id":"f"}]}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","ownedRelationship":[{"@id":"missing","extra":{"a":1,"a":2}}]}]""")]
    [InlineData("""[{"identity":{"a":1,"a":2},"payload":null}]""")]
    [InlineData("""{"change":[],"metadata":{"nested":[{"a":1,"a":2}]}}""")]
    [InlineData("""[{"@id":"e","@type":"Element","vendor:data":{"a":1,"\u0061":2}}]""")]
    public void NestedDuplicateKeysFailEverySource(string json)
    {
        foreach (var options in Options())
        {
            CheckSources(Encoding.UTF8.GetBytes(json), options, read =>
            {
                var failure = Assert.Throws<MalformedDocumentException>(read);
                Assert.Contains("twice", failure.Message, StringComparison.Ordinal);
            });
        }
    }

    [Theory]
    [InlineData("""[{"@id":"!","@type":"Element"}]""")]
    [InlineData("""[{"@id":"e","@type":"!"}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","declaredName":"!"}]""")]
    [InlineData("""[{"@id":"e","@type":"PartUsage","direction":"!"}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","aliasIds":["!"]}]""")]
    [InlineData("""[{"@id":"e","@type":"PartUsage","partDefinition":[{"@id":"!"}]}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","vendor:data":{"value":"!"}}]""")]
    [InlineData("""[{"identity":{"value":"!"},"payload":null}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","!":1}]""")]
    public void InvalidUtf8InsideStringsFailsEverySource(string json)
    {
        var marker = json.IndexOf('!', StringComparison.Ordinal);
        foreach (var invalid in new byte[][] { [0xff], [0xc0, 0xaf], [0xed, 0xa0, 0x80], [0xe2, 0x82] })
        {
            byte[] bytes = [.. Encoding.UTF8.GetBytes(json[..marker]), .. invalid, .. Encoding.UTF8.GetBytes(json[(marker + 1)..])];
            foreach (var options in Options())
            {
                CheckSources(bytes, options, read =>
                    Assert.Throws<MalformedDocumentException>(read));
            }
        }
    }

    [Theory]
    [InlineData("""[{"@id":"\uD800","@type":"Element"}]""")]
    [InlineData("""[{"@id":"\uDC00","@type":"Element"}]""")]
    [InlineData("""[{"@id":"e","@type":"\uD800"}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","\uD800":1}]""")]
    [InlineData("""[{"@id":"e","@type":"Element","vendor:data":{"\uDC00":1}}]""")]
    [InlineData("""[{"identity":{"\uD800":1},"payload":null}]""")]
    public void UndecodableIdentityAndKeysAreDocumentFailures(string json)
    {
        foreach (var options in Options())
        {
            CheckSources(Encoding.UTF8.GetBytes(json), options, read =>
            {
                var failure = Assert.Throws<MalformedDocumentException>(read);
                Assert.Contains("Unicode", failure.Message, StringComparison.Ordinal);
            });
        }
    }

    [Theory]
    [InlineData("declaredName", "\"declaredName\":\"\\uD800\"")]
    [InlineData("declaredName", "\"declaredName\":\"\\uDC00\"")]
    [InlineData("aliasIds", "\"aliasIds\":[\"valid\",\"\\uD800\"]")]
    [InlineData("direction", "\"direction\":\"\\uD800\"")]
    [InlineData("owningRelationship", "\"owningRelationship\":{\"@id\":\"\\uD800\"}")]
    [InlineData("owningRelationship", "\"owningRelationship\":{\"@ref\":\"\\uDC00\"}")]
    [InlineData("ownedRelationship", "\"ownedRelationship\":[{\"@id\":\"\\uD800\"}]")]
    [InlineData("ownedRelationship", "\"ownedRelationship\":[{\"@ref\":\"\\uD800\"}]")]
    [InlineData("partDefinition", "\"partDefinition\":[{\"@id\":\"\\uD800\"}]")]
    public void UndecodablePropertyStringsFailOnlyTheirProperty(string key, string member)
    {
        var json = """[{"@id":"e","@type":"PartUsage","elementId":"kept",""" + member + """},{"@id":"root","@type":"Namespace"}]""";
        foreach (var options in Options())
        {
            CheckSources(Encoding.UTF8.GetBytes(json), options, read =>
            {
                var graph = read();
                var element = (IPartUsage)graph["e"];
                Assert.Equal("kept", element.ElementId);
                Assert.Contains(graph["root"], graph.Roots);
                var property = element.MetaclassInfo.GetProperty(key);
                var typed = property.DeclaringMetaclass.InterfaceType.GetProperty(property.Name)!;
                var failure = Assert.Throws<MalformedValueException>(() => element.GetValue(key));
                Assert.Equal((property, "e"), (failure.Property, failure.ElementId));
                Assert.Contains("Unicode", failure.Message, StringComparison.Ordinal);
                Assert.Equal(failure.Message, Assert.Throws<MalformedValueException>(() => element.GetValue(key)).Message);
                var invocation = Assert.Throws<TargetInvocationException>(() => typed.GetValue(element));
                Assert.Equal(failure.Message, Assert.IsType<MalformedValueException>(invocation.InnerException).Message);
            });
        }
    }

    [Fact]
    public void ValidUnicodeAndRepeatedKeysInSeparateObjectsStillRead()
    {
        foreach (var text in new[] { @"\uD83D\uDE80", "\U0001F680" })
        {
            var json = """
                [{"@id":"!","@type":"PartDefinition","declaredName":"!","vendor:data":{"a":1}},
                 {"@id":"e","@type":"PartUsage","partDefinition":[{"@id":"!"}],"vendor:data":{"a":2}}]
                """.Replace("!", text, StringComparison.Ordinal);
            foreach (var options in Options())
            {
                CheckSources(Encoding.UTF8.GetBytes(json), options, read =>
                {
                    var graph = read();
                    var definition = (IPartDefinition)graph["\U0001F680"];
                    Assert.Equal("\U0001F680", definition.DeclaredName);
                    Assert.Same(definition, Assert.Single(((IPartUsage)graph["e"]).PartDefinition));
                    if (options.KeepExtensions)
                    {
                        Assert.Equal(1, definition.Extensions["vendor:data"].GetProperty("a").GetInt32());
                        Assert.Equal(2, graph["e"].Extensions["vendor:data"].GetProperty("a").GetInt32());
                    }
                });
            }
        }
    }

    [Fact]
    public void OpaqueExtensionStringsStayAsWritten()
    {
        const string json = """[{"@id":"e","@type":"Element","vendor:data":{"value":"\uD800"}}]""";
        foreach (var options in Options())
        {
            CheckSources(Encoding.UTF8.GetBytes(json), options, read =>
            {
                var graph = read();
                if (options.KeepExtensions)
                {
                    Assert.Equal("\"\\uD800\"", graph["e"].Extensions["vendor:data"].GetProperty("value").GetRawText());
                }
            });
        }
    }

    [Fact]
    public void KeptExtensionsFromLenientDocumentsRemainReadableAfterDisposal()
    {
        const string json = """
            [{"@id":"e","@type":"Element","vendor:data":{"value":"\uD800",/* metadata */"items":[1,],},}]
            """;
        MetamodelGraph fromDocument;
        MetamodelGraph fromElements;
        using (var document = JsonDocument.Parse(json, new JsonDocumentOptions { CommentHandling = JsonCommentHandling.Skip, AllowTrailingCommas = true }))
        {
            var options = new ReadOptions { KeepExtensions = true };
            fromDocument = MetamodelGraph.ReadJson(document, options);
            fromElements = MetamodelGraph.ReadJson(document.RootElement.EnumerateArray(), options);
        }

        foreach (var graph in new[] { fromDocument, fromElements })
        {
            var value = graph["e"].Extensions["vendor:data"];
            using var strict = JsonDocument.Parse(value.GetRawText());
            Assert.Equal("\"\\uD800\"", strict.RootElement.GetProperty("value").GetRawText());
            Assert.Equal(1, Assert.Single(strict.RootElement.GetProperty("items").EnumerateArray()).GetInt32());
        }
    }

    [Fact]
    public void DisposedSourcesAreNotReportedAsMalformedDocuments()
    {
        var document = JsonDocument.Parse("""[{"@id":"e","@type":"Element"}]""");
        var element = document.RootElement[0];
        document.Dispose();

        Assert.Throws<ObjectDisposedException>(() => MetamodelGraph.ReadJson(document));
        Assert.Throws<ObjectDisposedException>(() => MetamodelGraph.ReadJson([element]));
    }

    private static ReadOptions[] Options() =>
    [
        new(),
        new() { KeepExtensions = true },
        new() { CheckRanges = false },
        new() { CheckRanges = false, KeepExtensions = true },
    ];

    private void CheckSources(byte[] bytes, ReadOptions options, Action<Func<MetamodelGraph>> check)
    {
        var path = Path.Combine(directory, "input.json");
        File.WriteAllBytes(path, bytes);
        check(() => MetamodelGraph.ReadJson(path, options));
        using var stream = new MemoryStream(bytes);
        check(() => MetamodelGraph.ReadJson(stream, options));
        Assert.True(stream.CanRead);
        using var document = JsonDocument.Parse(bytes);
        check(() => MetamodelGraph.ReadJson(document, options));
        Assert.NotEqual(JsonValueKind.Undefined, document.RootElement.ValueKind);
        check(() => MetamodelGraph.ReadJson(
            document.RootElement.ValueKind == JsonValueKind.Array ? document.RootElement.EnumerateArray() : [document.RootElement],
            options));
    }
}
