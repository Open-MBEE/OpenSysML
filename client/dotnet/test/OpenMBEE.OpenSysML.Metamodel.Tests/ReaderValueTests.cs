using System;
using System.Collections.Generic;
using System.Text.Json.Nodes;
using Xunit;
using static OpenMBEE.OpenSysML.Metamodel.Tests.Documents;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>How the reader converts values and resolves references, and what it refuses or defers.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReaderValueTests
{
    public static TheoryData<string, string, string, string> MalformedValues => new()
    {
        { "a reference list given a string", "partDefinition", Element("e", "PartUsage", ("partDefinition", "definition")).ToJsonString(), "\"partDefinition\" of element e: expected an array, got \"definition\"" },
        { "a list holding null", "partDefinition", Element("e", "PartUsage", ("partDefinition", Items((JsonNode?)null))).ToJsonString(), "the array holds null" },
        { "a list given one reference, by a type the metamodel lacks", "ownedRelationship", Element("e", "vendor:BooleanUsage", ("ownedRelationship", Ref("e"))).ToJsonString(), "\"ownedRelationship\" of element e: expected an array, got an object" },
        { "a reference given as a string", "partDefinition", Element("e", "PartUsage", ("partDefinition", Items("definition"))).ToJsonString(), "expected a reference to PartDefinition, got \"definition\"" },
        { "a single value given a list", "direction", Element("e", "PartUsage", ("direction", Items("in"))).ToJsonString(), "\"direction\" of element e: expected a single value, got an array" },
        { "an unknown literal", "direction", Element("e", "PartUsage", ("direction", "sideways")).ToJsonString(), "\"sideways\" is not a FeatureDirectionKind literal" },
        { "a literal given a number", "direction", Element("e", "PartUsage", ("direction", 1)).ToJsonString(), "expected a FeatureDirectionKind literal, got 1" },
        { "a Boolean given a string", "isAbstract", Element("t", "Type", ("isAbstract", "true")).ToJsonString(), "expected a Boolean, got \"true\"" },
        { "a string given a number", "declaredName", Element("e", "PartUsage", ("declaredName", 3)).ToJsonString(), "expected a string, got 3" },
        { "a string given a reference", "declaredName", Element("e", "PartUsage", ("declaredName", Ref("e"))).ToJsonString(), "expected a string, got a reference object" },
        { "a literal given a reference", "direction", Element("e", "PartUsage", ("direction", Ref("e"))).ToJsonString(), "expected a FeatureDirectionKind literal, got a reference object" },
        { "an integer given a Boolean", "value", Element("i", "LiteralInteger", ("value", true)).ToJsonString(), "expected a 32-bit integer, got true" },
        { "an integer given a fraction", "value", Element("i", "LiteralInteger", ("value", 3.5)).ToJsonString(), "expected a 32-bit integer, got 3.5" },
        { "an integer given an exponent", "value", Element("i", "LiteralInteger", ("value", JsonNode.Parse("3e0"))).ToJsonString(), "expected a 32-bit integer, got 3e0" },
        { "an integer beyond 32 bits", "value", Element("i", "LiteralInteger", ("value", 2147483648L)).ToJsonString(), "expected a 32-bit integer, got 2147483648" },
        { "a real given a string", "value", Element("r", "LiteralRational", ("value", "3")).ToJsonString(), "expected a finite real number, got \"3\"" },
        { "a real beyond double", "value", Element("r", "LiteralRational", ("value", JsonNode.Parse("1e400"))).ToJsonString(), "expected a finite real number, got 1e400" },
        { "a real given a Boolean", "value", Element("r", "LiteralRational", ("value", false)).ToJsonString(), "expected a finite real number, got false" },
        { "an @id that is no string", "partDefinition", Element("e", "PartUsage", ("partDefinition", Items(new JsonObject { ["@id"] = 3 }))).ToJsonString(), "@id must be a string, got 3" },
        { "an @ref that is no string", "partDefinition", Element("e", "PartUsage", ("partDefinition", Items(new JsonObject { ["@ref"] = false }))).ToJsonString(), "@ref must be a string, got false" },
        { "a reference with neither", "partDefinition", Element("e", "PartUsage", ("partDefinition", Items(new JsonObject { ["name"] = "Engine" }))).ToJsonString(), "expected a reference object with an @id or @ref" },
        { "a long scalar, shortened", "direction", Element("e", "PartUsage", ("direction", new string('x', 100))).ToJsonString(), $"\"{new string('x', 60)}... is not a FeatureDirectionKind literal" },
    };

    /// <summary>References resolve to the graph's own elements, typed by the property's range.</summary>
    [Fact]
    public void ReferencesResolveToTheGraphsElements()
    {
        var graph = Read(
            Element("usage", "PartUsage", ("partDefinition", Items(Ref("definition"))), ("owningRelationship", Ref("membership"))),
            Element("definition", "PartDefinition", ("declaredName", "Engine")),
            Element("membership", "OwningMembership", ("ownedRelatedElement", Items(Ref("usage")))));

        var usage = (IPartUsage)graph["usage"];
        var definition = Assert.Single(usage.PartDefinition);
        Assert.Same(graph["definition"], definition);
        Assert.Equal("Engine", definition.DeclaredName);
        Assert.Same(graph["membership"], usage.OwningRelationship);
        Assert.Same(usage, Assert.Single(((IRelationship)graph["membership"]).OwnedRelatedElement));
        Assert.Same(usage.PartDefinition, usage.PartDefinition);
    }

    /// <summary>
    /// A reference the document does not resolve, an absent <c>@id</c> or any <c>@ref</c>, fails only reads of its
    /// property, each time, typed or not; the read of the document and of every other property succeeds.
    /// </summary>
    [Fact]
    public void UnresolvedReferencesFailOnlyTheirProperty()
    {
        var graph = Read(
            Element("dangling", "PartUsage", ("declaredName", "engine"), ("partDefinition", Items(Ref("missing")))),
            Element("named", "PartUsage", ("partDefinition", Items(new JsonObject { ["@ref"] = "Engine", ["@id"] = "named" }))),
            Element("mixed", "PartUsage", ("partDefinition", Items(Ref("definition"), Ref("library-real")))),
            Element("single", "PartUsage", ("owningRelationship", Ref("gone"))),
            Element("definition", "PartDefinition"));

        var dangling = (IPartUsage)graph["dangling"];
        Assert.Equal("engine", dangling.DeclaredName);
        var failure = Assert.Throws<UnresolvedReferenceException>(() => dangling.PartDefinition);
        Assert.Equal(("missing", null, "partDefinition", "dangling"), (failure.TargetId, failure.TargetRef, failure.Property.JsonKey, failure.ElementId));
        Assert.Same(Metaclasses.PartUsage, failure.Property.DeclaringMetaclass);
        Assert.Equal("\"partDefinition\" of element dangling refers to unresolved @id \"missing\"", failure.Message);
        Assert.Throws<UnresolvedReferenceException>(() => dangling.PartDefinition);
        Assert.Throws<UnresolvedReferenceException>(() => dangling.GetValue("partDefinition"));

        var named = Assert.Throws<UnresolvedReferenceException>(() => ((IPartUsage)graph["named"]).PartDefinition);
        Assert.Equal((null, "Engine"), (named.TargetId, named.TargetRef));
        Assert.EndsWith("refers to unresolved @ref \"Engine\"", named.Message, StringComparison.Ordinal);

        Assert.Equal("library-real", Assert.Throws<UnresolvedReferenceException>(() => ((IPartUsage)graph["mixed"]).PartDefinition).TargetId);
        Assert.Equal("gone", Assert.Throws<UnresolvedReferenceException>(() => ((IPartUsage)graph["single"]).OwningRelationship).TargetId);
        Assert.IsType<MetamodelException>(failure, exactMatch: false);
    }

    /// <summary>The exception refuses a reference naming both or neither of an <c>@id</c> and an <c>@ref</c>.</summary>
    [Fact]
    public void UnresolvedReferencesNameExactlyOneTarget()
    {
        var property = Metaclasses.PartUsage.GetProperty("partDefinition");
        Assert.Throws<ArgumentException>(() => new UnresolvedReferenceException(property, "e", "a", "b"));
        Assert.Throws<ArgumentException>(() => new UnresolvedReferenceException(property, "e", null, null));
        Assert.Throws<ArgumentNullException>(() => new UnresolvedReferenceException(null!, "e", "a", null));
    }

    /// <summary>With range checks on, a reference to an element outside the property's metaclass is malformed, typed or not.</summary>
    [Fact]
    public void ReferenceRangesRequireTheDeclaredMetaclass()
    {
        var graph = Read(
            Element("usage", "PartUsage", ("partDefinition", Items(Ref("literal"))), ("declaredName", "engine")),
            Element("literal", "LiteralInteger", ("value", 1)));
        var usage = (IPartUsage)graph["usage"];
        Assert.Equal("engine", usage.DeclaredName);
        var literal = Assert.Throws<MalformedValueException>(() => usage.PartDefinition);
        Assert.Equal(literal.Message, Assert.Throws<MalformedValueException>(() => usage.GetValue("partDefinition")).Message);
        Assert.Equal(("partDefinition", "usage"), (literal.Property.JsonKey, literal.ElementId));
        Assert.Contains("reference target @id \"literal\" has @type \"LiteralInteger\", read as LiteralInteger; expected PartDefinition", literal.Message, StringComparison.Ordinal);
        Assert.DoesNotContain("Supertypes", literal.Message, StringComparison.Ordinal);

        var customUsage = Element("usage", "PartUsage", ("partDefinition", Items(Ref("custom"))));
        var custom = Element("custom", "vendor:SpecialPartDefinition");
        var unmapped = Assert.Throws<MalformedValueException>(() => ((IPartUsage)Read(customUsage, custom)["usage"]).PartDefinition);
        Assert.Contains("read as Element; expected PartDefinition; ReadOptions.Supertypes maps", unmapped.Message, StringComparison.Ordinal);

        var mapped = Read(new ReadOptions { Supertypes = new Dictionary<string, string> { ["vendor:SpecialPartDefinition"] = "PartDefinition" } }, customUsage, custom);
        Assert.Same(mapped["custom"], Assert.Single(((IPartUsage)mapped["usage"]).PartDefinition));
    }

    /// <summary>Each item of a many-valued reference is checked; the first that fails, in document order, is the one reported.</summary>
    [Fact]
    public void ManyReferenceRangesAreCheckedPerItem()
    {
        var graph = Read(
            Element("definition", "PartDefinition", ("ownedFeature", Items(Ref("feature"), Ref("namespace"), Ref("missing")))),
            Element("unresolvedFirst", "PartDefinition", ("ownedFeature", Items(Ref("missing"), Ref("namespace")))),
            Element("feature", "Feature"),
            Element("namespace", "Namespace"));
        var failure = Assert.Throws<MalformedValueException>(() => ((IPartDefinition)graph["definition"]).OwnedFeature);
        Assert.Equal("missing", Assert.Throws<UnresolvedReferenceException>(() => ((IPartDefinition)graph["unresolvedFirst"]).OwnedFeature).TargetId);

        Assert.Contains("@id \"namespace\" has @type \"Namespace\", read as Namespace; expected Feature", failure.Message, StringComparison.Ordinal);
    }

    /// <summary>
    /// With range checks on an out-of-range reference is malformed; off, the read keeps it: the typed property
    /// still reports it, and <see cref="IMetamodelElement.GetValue"/> returns it as written. Every other check still applies.
    /// </summary>
    [Fact]
    public void RangeChecksOffKeepOutOfRangeReferencesAndNothingElse()
    {
        JsonNode[] document =
        [
            Element("requirement", "RequirementDefinition", ("ownedInterface", Items(Ref("reference"))), ("result", Ref("requirement")), ("declaredName", "R")),
            Element("reference", "ReferenceUsage"),
        ];
        var unchecked_ = new ReadOptions { CheckRanges = false };

        Assert.True(new ReadOptions().CheckRanges);
        var checkedGraph = Read(document);
        Assert.True(checkedGraph.CheckRanges);
        Assert.Throws<MalformedValueException>(() => checkedGraph["requirement"].GetValue("ownedInterface"));
        Assert.Throws<MalformedValueException>(() => checkedGraph["requirement"].GetValue("result"));
        var graph = Read(unchecked_, document);
        Assert.False(graph.CheckRanges);
        var requirement = (IRequirementDefinition)graph["requirement"];
        Assert.Equal("R", requirement.DeclaredName);
        var typed = Assert.Throws<MalformedValueException>(() => requirement.OwnedInterface);
        Assert.Contains("expected InterfaceUsage; read with range checks off, GetValue(\"ownedInterface\") returns it", typed.Message, StringComparison.Ordinal);
        Assert.Throws<MalformedValueException>(() => requirement.Result);
        Assert.Equal([graph["reference"]], Assert.IsAssignableFrom<IReadOnlyList<IMetamodelElement>>(requirement.GetValue("ownedInterface")));
        Assert.Same(requirement, requirement.GetValue("result"));

        Assert.Throws<MalformedValueException>(() => ((IPartUsage)Read(unchecked_, Element("e", "PartUsage", ("direction", "sideways")))["e"]).Direction);
        Assert.Throws<MalformedValueException>(() => Read(unchecked_, Element("e", "PartUsage", ("partDefinition", Items((JsonNode?)null))))["e"].GetValue("partDefinition"));
        Assert.Throws<MalformedDocumentException>(() => Read(unchecked_, Element(), Element()));
        var dangling = (IPartUsage)Read(unchecked_, Element("e", "PartUsage", ("partDefinition", Items(Ref("missing")))))["e"];
        Assert.Throws<UnresolvedReferenceException>(() => dangling.PartDefinition);
    }

    /// <summary>An absent key is not supplied, derived or not; an explicit null reads as null, or as empty when many-valued.</summary>
    [Fact]
    public void MissingAndNullPropertiesPreserveCardinality()
    {
        var usage = (IPartUsage)Read(Element())["e"];
        var owned = Assert.Throws<NotSuppliedException>(() => usage.DeclaredName);
        Assert.Equal(("declaredName", "e", false), (owned.Property.JsonKey, owned.ElementId, owned.IsDerived));
        Assert.True(Assert.Throws<NotSuppliedException>(() => usage.Feature).IsDerived);
        Assert.True(Assert.Throws<NotSuppliedException>(() => usage.GetValue("feature")).IsDerived);

        var nulls = (IPartUsage)Read(Element("e", "PartUsage", ("direction", null), ("feature", null), ("owningRelationship", null)))["e"];
        Assert.Null(nulls.Direction);
        Assert.Null(nulls.OwningRelationship);
        Assert.Empty(nulls.Feature);
        Assert.Null(nulls.GetValue("direction"));
        Assert.Empty(Assert.IsAssignableFrom<IReadOnlyList<IFeature>>(nulls.GetValue("feature")));
        Assert.Empty(((IPartUsage)Read(Element("e", "PartUsage", ("feature", Items())))["e"]).Feature);
    }

    /// <summary>Enumeration literals, Booleans, strings, integers and reals read as their C# values.</summary>
    [Fact]
    public void EnumerationAndPrimitiveValuesRead()
    {
        var graph = Read(
            Element("e", "PartUsage", ("direction", "inout"), ("declaredName", "engine"), ("aliasIds", Items("x", "x"))),
            Element("t", "Type", ("isAbstract", true)),
            Element("i", "LiteralInteger", ("value", -3)),
            Element("r", "LiteralRational", ("value", 3)),
            Element("f", "LiteralRational", ("value", 2.5e-3)));

        var usage = (IPartUsage)graph["e"];
        Assert.Equal(FeatureDirectionKind.Inout, usage.Direction);
        Assert.Equal(FeatureDirectionKind.Inout, usage.GetValue("direction"));
        Assert.Equal(["x", "x"], usage.AliasIds);
        Assert.True(((IType)graph["t"]).IsAbstract);
        Assert.Equal(-3, ((ILiteralInteger)graph["i"]).Value);
        Assert.Equal(3.0, ((ILiteralRational)graph["r"]).Value);
        Assert.Equal(2.5e-3, ((ILiteralRational)graph["f"]).Value);
        Assert.Throws<NotSupportedException>(() => ((IList<string>)usage.AliasIds).Add("y"));
    }

    /// <summary>
    /// A value that does not fit its property fails every read of the property, typed or not, with
    /// <see cref="MalformedValueException"/>; the read of the document and of its other properties succeeds.
    /// </summary>
    [Theory]
    [MemberData(nameof(MalformedValues))]
    public void MalformedValuesFailTheirPropertysReads(string what, string key, string json, string reason)
    {
        var node = JsonNode.Parse(json)!.AsObject();
        node["elementId"] = "kept";
        var element = Read(node)[node["@id"]!.GetValue<string>()];
        var property = element.MetaclassInfo.GetProperty(key);
        var typed = property.DeclaringMetaclass.InterfaceType.GetProperty(property.Name)!;

        var failure = Assert.Throws<MalformedValueException>(() => element.GetValue(key));
        Assert.True(failure.Message.Contains(reason, StringComparison.Ordinal), $"{what}: {failure.Message}");
        Assert.Equal((element.Id, property), (failure.ElementId, failure.Property));
        var again = Assert.Throws<System.Reflection.TargetInvocationException>(() => typed.GetValue(element));
        Assert.Equal(failure.Message, Assert.IsType<MalformedValueException>(again.InnerException).Message);
        Assert.Equal("kept", ((IElement)element).ElementId);
    }

    /// <summary>
    /// Keys the element's metaclass has no property for, an exporter's annotations among them, are not read,
    /// whatever they hold; looking one up by key is <see cref="UnknownJsonKeyException"/>.
    /// </summary>
    [Fact]
    public void KeysOutsideTheMetaclassAreNotRead()
    {
        var graph = Read(Element(
            "e",
            "AttributeUsage",
            ("sysx:sourceText", "attribute power : Real = 300.0;"),
            ("value", Items((JsonNode?)null)),
            ("partDefinition", "not checked"),
            ("declaredName", "power")));

        var usage = (IAttributeUsage)graph["e"];
        Assert.Equal("power", usage.DeclaredName);
        var unknown = Assert.Throws<UnknownJsonKeyException>(() => usage.GetValue("sysx:sourceText"));
        Assert.Same(Metaclasses.AttributeUsage, unknown.Metaclass);
        Assert.Equal("sysx:sourceText", unknown.JsonKey);
        Assert.Equal("AttributeUsage has no property with JSON key \"sysx:sourceText\"", unknown.Message);
        Assert.Throws<UnknownJsonKeyException>(() => Metaclasses.AttributeUsage.GetProperty("partDefinition"));
        Assert.Same(Metaclasses.Element.GetProperty("declaredName"), Metaclasses.AttributeUsage.GetProperty("declaredName"));
        Assert.Throws<ArgumentNullException>(() => usage.GetValue(null!));
    }
}
