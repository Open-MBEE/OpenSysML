using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Text.Json.Nodes;
using Xunit;
using static OpenMBEE.OpenSysML.Metamodel.Tests.Documents;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>A read graph: lookup, order, roots, identity, cycles, selection by ancestry, and real exports.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class MetamodelGraphTests
{
    /// <summary>Elements keep document order and are found by <c>@id</c>; roots are the namespaces nothing owns.</summary>
    [Fact]
    public void LookupOrderAndRoots()
    {
        var graph = Read(
            Element("root", "Namespace"),
            Element("child", "Namespace"),
            Element("owner", "Element", ("ownedRelationship", Items(Ref("membership")))),
            Element("membership", "OwningMembership", ("ownedRelatedElement", Items(Ref("child")))));

        Assert.Equal(["root", "child", "owner", "membership"], graph.Elements.Select(element => element.Id));
        Assert.True(graph.Contains("root"));
        Assert.False(graph.Contains("absent"));
        Assert.True(graph.TryGet("child", out var child));
        Assert.Same(graph.Elements[1], child);
        Assert.False(graph.TryGet("absent", out var absent));
        Assert.Null(absent);
        Assert.Equal("the document holds no element with @id \"absent\"", Assert.Throws<KeyNotFoundException>(() => graph["absent"]).Message);
        Assert.Equal([graph["root"], graph["child"]], graph.All<INamespace>());
        Assert.Equal(graph.Elements, graph.All<IMetamodelElement>());
        Assert.Same(graph["root"], Assert.Single(graph.Roots));
        Assert.Same(graph.Roots, graph.Roots);
        Assert.Throws<NotSupportedException>(() => ((IList<IMetamodelElement>)graph.Elements).Clear());
        Assert.Throws<NotSupportedException>(() => ((IList<INamespace>)graph.Roots).Clear());
        Assert.Throws<NotSupportedException>(() => ((IList<INamespace>)graph.All<INamespace>()).Clear());
        Assert.Throws<ArgumentNullException>(() => graph.Contains(null!));
        Assert.Throws<ArgumentNullException>(() => graph.TryGet(null!, out _));

        var direct = Read(
            Element("root", "Namespace"),
            Element("child", "Namespace"),
            Element("owner", "Element", ("ownedRelationship", Items(Ref("child")))));
        Assert.Same(direct["root"], Assert.Single(direct.Roots));
        Assert.Throws<MalformedValueException>(() => ((IElement)direct["owner"]).OwnedRelationship);
    }

    /// <summary>A namespace with a non-null owner, owning namespace or owning relationship is no root, whether or not the owner is in the document.</summary>
    [Theory]
    [InlineData("owner", "root")]
    [InlineData("owner", "outside")]
    [InlineData("owningNamespace", "root")]
    [InlineData("owningNamespace", "outside")]
    [InlineData("owningRelationship", "outside")]
    public void RootsExcludeNamespacesWithOwnerMetadata(string key, string ownerId)
    {
        var graph = Read(
            Element("root", "Namespace"),
            Element("nested", "Namespace", (key, Ref(ownerId))),
            Element("unowned", "Package", (key, null)));

        Assert.Equal(["root", "unowned"], graph.Roots.Select(root => root.Id));
    }

    /// <summary>An element read twice is two elements; within a graph, an element is one object however reached.</summary>
    [Fact]
    public void ElementsBelongToOneGraph()
    {
        var first = Read(Element("e", "PartUsage", ("declaredName", "engine")));
        var second = Read(Element("e", "PartUsage", ("declaredName", "engine")));

        Assert.Same(first["e"], first.Elements[0]);
        Assert.NotEqual(first["e"], second["e"]);
        Assert.Equal(2, new HashSet<IMetamodelElement> { first["e"], first["e"], second["e"] }.Count);
    }

    /// <summary>References may form cycles, an element's reference to itself included.</summary>
    [Fact]
    public void CyclicReferencesResolve()
    {
        var graph = Read(
            Element("ns", "Namespace", ("owningRelationship", Ref("m"))),
            Element("m", "OwningMembership", ("ownedRelatedElement", Items(Ref("ns"))), ("owningRelatedElement", Ref("ns"))),
            Element("p", "PartUsage", ("ownedFeature", Items(Ref("p"))), ("type", Items(Ref("p")))));

        var ns = (INamespace)graph["ns"];
        var membership = (IOwningMembership)ns.OwningRelationship!;
        Assert.Same(ns, Assert.Single(membership.OwnedRelatedElement));
        Assert.Same(ns, membership.OwningRelatedElement);
        var part = (IPartUsage)graph["p"];
        Assert.Same(part, Assert.Single(part.OwnedFeature));
        Assert.Same(part, Assert.Single(part.Type));
        Assert.Same(part, Assert.Single(graph.Roots));
    }

    /// <summary><see cref="MetamodelGraph.All{T}"/> over every metaclass interface selects exactly the elements whose metaclass conforms to it.</summary>
    [Fact]
    public void SelectionByInterfaceFollowsMetamodelAncestry()
    {
        var graph = Read([.. Metaclasses.All.Select(metaclass => (JsonNode)Element(metaclass.Name, metaclass.Name))]);
        var all = typeof(MetamodelGraph).GetMethod(nameof(MetamodelGraph.All))!;

        Assert.Equal(Metaclasses.All.Count, graph.Elements.Count);
        foreach (var metaclass in Metaclasses.All)
        {
            var element = graph[metaclass.Name];
            Assert.Same(metaclass, element.MetaclassInfo);
            Assert.Equal(metaclass.IsAbstract, !element.GetType().IsVisible);
            var selected = ((IEnumerable<IMetamodelElement>)all.MakeGenericMethod(metaclass.InterfaceType).Invoke(graph, null)!).ToList();
            Assert.Equal(
                Metaclasses.All.Where(other => other.ConformsTo(metaclass)).Select(other => other.Name),
                selected.Select(each => each.Id));
        }

        Assert.Equal(graph.Elements.Where(element => element.MetaclassInfo.ConformsTo(Metaclasses.Feature)), graph.All<IFeature>());
        Assert.Contains(graph["PartUsage"], graph.All<IFeature>());
        Assert.DoesNotContain(graph["PartDefinition"], graph.All<IFeature>());
    }

    /// <summary>Every property of every metaclass reads what the document supplies for it, in every range kind and multiplicity.</summary>
    [Fact]
    public void EveryPropertyOfEveryMetaclassReadsItsValue()
    {
        var targets = new Dictionary<MetaclassInfo, string>();
        var elements = new List<JsonNode>();
        foreach (var metaclass in Metaclasses.All)
        {
            var element = Element(metaclass.Name, metaclass.Name);
            foreach (var property in metaclass.Properties)
            {
                element[property.JsonKey] = property.IsMany ? Items(Json(property, targets), Json(property, targets)) : Json(property, targets);
            }

            elements.Add(element);
        }

        elements.AddRange(targets.Select(target => (JsonNode)Element(target.Value, target.Key.Name)));
        var graph = Read([.. elements]);
        var count = 0;
        foreach (var metaclass in Metaclasses.All)
        {
            var element = graph[metaclass.Name];
            foreach (var property in metaclass.Properties)
            {
                var value = element.GetValue(property.JsonKey);
                var expected = Expected(graph, property, targets);
                if (property.IsMany)
                {
                    Assert.Equal(new[] { expected, expected }, ((System.Collections.IEnumerable)value!).Cast<object>());
                    Assert.True(typeof(IReadOnlyList<>).MakeGenericType(property.ValueType).IsInstanceOfType(value), property.QualifiedName);
                }
                else
                {
                    Assert.Equal(expected, value);
                }

                count++;
            }
        }

        Assert.Equal(Metaclasses.All.Sum(metaclass => metaclass.Properties.Count), count);
    }

    /// <summary>The sysml-toolkit <c>full-json</c> export reads whole, as the Python reader reads it.</summary>
    [Fact]
    public void ReadsTheToolkitFullJsonFixture()
    {
        using var raw = JsonDocument.Parse(File.ReadAllText(ToolkitFixture));
        var graph = MetamodelGraph.ReadJson(ToolkitFixture);
        Assert.Equal(87, graph.Elements.Count);
        Assert.Equal(raw.RootElement.EnumerateArray().Select(element => element.GetProperty("@id").GetString()), graph.Elements.Select(element => element.Id));

        var parts = graph.All<IPartUsage>();
        Assert.Equal(["wheel", "v"], parts.Select(part => part.DeclaredName));
        Assert.All(parts, part => Assert.IsAssignableFrom<IFeature>(part));

        var vehicle = graph.All<IPartDefinition>().Single(definition => definition.DeclaredName == "Vehicle");
        Assert.IsAssignableFrom<IType>(vehicle);
        var rawVehicle = raw.RootElement.EnumerateArray().Single(element => element.GetProperty("@id").GetString() == vehicle.Id);
        Assert.Equal(rawVehicle.GetProperty("ownedFeature").EnumerateArray().Select(reference => graph[reference.GetProperty("@id").GetString()!]), vehicle.OwnedFeature);
        Assert.Equal(["mass2", "wheel", "p", "cargo", "a", "c", "s"], vehicle.OwnedFeature.Select(feature => feature.DeclaredName));
        Assert.Equal(7, vehicle.Feature.Count);
        Assert.All(vehicle.Feature, feature => Assert.True(graph.Contains(feature.Id)));

        var bounded = graph.All<IMultiplicityRange>().First(range => RawHas(range.Id, "bound"));
        Assert.Equal(Raw(bounded.Id).GetProperty("bound").GetArrayLength(), bounded.Bound.Count);

        var v = parts.Single(part => part.DeclaredName == "v");
        Assert.Same(vehicle, Assert.Single(v.Type));
        Assert.IsAssignableFrom<INamespace>(Assert.Single(graph.Roots));

        foreach (var element in graph.Elements)
        {
            foreach (var property in element.MetaclassInfo.Properties.Where(property => RawHas(element.Id, property.JsonKey)))
            {
                element.GetValue(property.JsonKey);
            }
        }

        var missingName = JsonNode.Parse(rawVehicle.GetRawText())!.AsObject();
        missingName.Remove("qualifiedName");
        var incomplete = (IPartDefinition)Read(missingName)[vehicle.Id];
        Assert.True(Assert.Throws<NotSuppliedException>(() => incomplete.QualifiedName).IsDerived);
        Assert.Throws<UnresolvedReferenceException>(() => incomplete.OwnedFeature);

        JsonElement Raw(string id) => raw.RootElement.EnumerateArray().Single(element => element.GetProperty("@id").GetString() == id);

        bool RawHas(string id, string key) => Raw(id).TryGetProperty(key, out _);
    }

    private static JsonNode Json(MetaProperty property, Dictionary<MetaclassInfo, string> targets) => property.RangeKind switch
    {
        PropertyRangeKind.Metaclass => Ref(Target(property, targets)),
        PropertyRangeKind.Enumeration => JsonValue.Create(property.RangeEnumeration!.Literals[^1]),
        _ when property.ValueType == typeof(bool) => JsonValue.Create(true),
        _ when property.ValueType == typeof(string) => JsonValue.Create(property.QualifiedName),
        _ when property.ValueType == typeof(int) => JsonValue.Create(property.JsonKey.Length),
        _ => JsonValue.Create(property.JsonKey.Length + 0.25),
    };

    private static object Expected(MetamodelGraph graph, MetaProperty property, Dictionary<MetaclassInfo, string> targets) => property.RangeKind switch
    {
        PropertyRangeKind.Metaclass => graph[Target(property, targets)],
        PropertyRangeKind.Enumeration => Enum.ToObject(property.ValueType, property.RangeEnumeration!.Literals.Count - 1),
        _ when property.ValueType == typeof(bool) => true,
        _ when property.ValueType == typeof(string) => property.QualifiedName,
        _ when property.ValueType == typeof(int) => property.JsonKey.Length,
        _ => property.JsonKey.Length + 0.25,
    };

    /// <summary>The <c>@id</c> of a target of exactly the property's range metaclass, so an abstract range is targeted too.</summary>
    private static string Target(MetaProperty property, Dictionary<MetaclassInfo, string> targets)
    {
        var range = property.RangeMetaclass!;
        if (!targets.TryGetValue(range, out var id))
        {
            targets[range] = id = "target-" + range.Name;
        }

        return id;
    }
}
