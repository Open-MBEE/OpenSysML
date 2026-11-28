using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>Each metamodel property is one read-only interface property, mapped to and from its JSON key.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class PropertyTests
{
    /// <summary>Each interface declares exactly its metaclass's own properties, typed by range and multiplicity.</summary>
    [Fact]
    public void InterfacesDeclareTheirMetaclassesProperties()
    {
        var count = 0;
        foreach (var metaclass in Metaclasses.All)
        {
            var declared = metaclass.InterfaceType.GetProperties(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly);
            Assert.Equal(
                metaclass.DeclaredProperties.Select(property => property.Name).Order(StringComparer.Ordinal),
                declared.Select(property => property.Name).Order(StringComparer.Ordinal));
            foreach (var property in metaclass.DeclaredProperties)
            {
                count++;
                Assert.Same(metaclass, property.DeclaringMetaclass);
                Assert.Equal($"{metaclass.Name}::{property.JsonKey}", property.QualifiedName);
                Assert.Equal($"https://www.omg.org/spec/SysML#{metaclass.Name}_{property.JsonKey}", property.Iri);
                Assert.Equal(char.ToUpperInvariant(property.JsonKey[0]) + property.JsonKey[1..], property.Name);
                var member = declared.Single(candidate => candidate.Name == property.Name);
                Assert.True(member.CanRead, property.QualifiedName);
                Assert.False(member.CanWrite, property.QualifiedName);
                Assert.Equal(ExpectedType(property), member.PropertyType);
            }
        }

        Assert.Equal(415, count);
    }

    /// <summary>A metaclass's properties are its own and its ancestors', one per JSON key, found by key and by C# name.</summary>
    [Fact]
    public void EffectivePropertiesMapBothWays()
    {
        foreach (var metaclass in Metaclasses.All)
        {
            var expected = metaclass.Ancestors.Prepend(metaclass)
                .SelectMany(owner => owner.DeclaredProperties)
                .GroupBy(property => property.JsonKey)
                .Select(group => group.First())
                .OrderBy(property => property.JsonKey, StringComparer.Ordinal);
            Assert.Equal(expected, metaclass.Properties);
            Assert.Equal(metaclass.Properties.Count, metaclass.Properties.Select(property => property.Name).Distinct().Count());
            foreach (var property in metaclass.Properties)
            {
                Assert.True(metaclass.TryGetProperty(property.JsonKey, out var byKey));
                Assert.Same(property, byKey);
                Assert.True(metaclass.TryGetPropertyByName(property.Name, out var byName));
                Assert.Same(property, byName);
                Assert.True(metaclass.ConformsTo(property.DeclaringMetaclass));
            }

            Assert.False(metaclass.TryGetProperty("@id", out _));
            Assert.False(metaclass.TryGetProperty("bogus", out _));
        }

        Assert.True(Metaclasses.PartUsage.TryGetProperty("declaredName", out var declaredName));
        Assert.Same(Metaclasses.Element, declaredName.DeclaringMetaclass);
        Assert.False(Metaclasses.Element.TryGetProperty("partDefinition", out _));
        Assert.False(Metaclasses.Element.TryGetPropertyByName("declaredName", out _));
        Assert.Throws<ArgumentNullException>(() => Metaclasses.Element.TryGetProperty(null!, out _));
    }

    /// <summary>No property takes a name the runtime gives every element, so none hides it.</summary>
    [Fact]
    public void PropertiesDoNotCollideWithRuntimeMembers()
    {
        var runtime = typeof(IMetamodelElement).GetMembers()
            .Concat(typeof(object).GetMembers(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance | BindingFlags.Static))
            .Select(member => member.Name)
            .ToHashSet(StringComparer.Ordinal);
        Assert.Contains("MetaclassInfo", runtime);
        Assert.All(Metaclasses.All.SelectMany(metaclass => metaclass.DeclaredProperties), property => Assert.DoesNotContain(property.Name, runtime));
        Assert.True(Metaclasses.MetadataFeature.TryGetProperty("metaclass", out var metaclassProperty));
        Assert.Equal("Metaclass", metaclassProperty.Name);
    }

    /// <summary>Every property of every concrete metaclass reads, through the interface declaring it, the value supplied for it.</summary>
    [Fact]
    public void EveryElementClassReadsEveryPropertyThroughItsInterface()
    {
        var reads = 0;
        foreach (var metaclass in Fixtures.Concrete)
        {
            var element = metaclass.CreateElement("e", metaclass.Name);
            var supplied = new Dictionary<MetaProperty, object>();
            foreach (var (property, seed) in metaclass.Properties.Select((property, index) => (property, index)))
            {
                supplied[property] = Fixtures.SuppliedValue(property, seed);
                element.Supply(property, supplied[property]);
                Assert.True(element.IsSupplied(property));
            }

            foreach (var owner in metaclass.Ancestors.Prepend(metaclass))
            {
                foreach (var declared in owner.DeclaredProperties)
                {
                    var effective = metaclass.Properties.Single(property => property.JsonKey == declared.JsonKey);
                    var read = owner.InterfaceType.GetProperty(declared.Name)!.GetValue(element);
                    if (declared.IsMany)
                    {
                        Assert.Equal((IEnumerable<object>)((Array)supplied[effective]).Cast<object>(), ((System.Collections.IEnumerable)read!).Cast<object>());
                    }
                    else
                    {
                        Assert.Equal(supplied[effective], read);
                    }

                    reads++;
                }
            }
        }

        Assert.Equal(Fixtures.Concrete.Sum(metaclass => metaclass.Properties.Count), reads);
    }

    /// <summary>
    /// A property the document does not supply throws NotSuppliedException naming it, the element and whether it
    /// is derived; one supplied as null reads as null, or as empty when many-valued.
    /// </summary>
    [Fact]
    public void UnsuppliedAndNullPropertiesReadHonestly()
    {
        foreach (var metaclass in Fixtures.Concrete)
        {
            var element = metaclass.CreateElement("e-" + metaclass.Name, metaclass.Name);
            foreach (var owner in metaclass.Ancestors.Prepend(metaclass))
            {
                foreach (var declared in owner.DeclaredProperties)
                {
                    var effective = metaclass.Properties.Single(property => property.JsonKey == declared.JsonKey);
                    var getter = owner.InterfaceType.GetProperty(declared.Name)!;
                    var failure = Assert.Throws<TargetInvocationException>(() => getter.GetValue(element));
                    var notSupplied = Assert.IsType<NotSuppliedException>(failure.InnerException);
                    Assert.Same(effective, notSupplied.Property);
                    Assert.Equal(element.Id, notSupplied.ElementId);
                    Assert.Equal(effective.IsDerived, notSupplied.IsDerived);
                    Assert.IsAssignableFrom<MetamodelException>(notSupplied);
                }
            }

            foreach (var property in metaclass.Properties)
            {
                element.Supply(property, null);
            }

            foreach (var owner in metaclass.Ancestors.Prepend(metaclass))
            {
                foreach (var declared in owner.DeclaredProperties)
                {
                    var read = owner.InterfaceType.GetProperty(declared.Name)!.GetValue(element);
                    if (declared.IsMany)
                    {
                        Assert.Empty((System.Collections.IEnumerable)read!);
                    }
                    else
                    {
                        Assert.Null(read);
                    }
                }
            }
        }
    }

    /// <summary>The message says which key is missing from which element, and why a derived one usually is.</summary>
    [Fact]
    public void NotSuppliedMessagesNameTheKeyAndElement()
    {
        IElement element = (IElement)Metaclasses.Comment.CreateElement("c1", "Comment");
        var derived = Assert.Throws<NotSuppliedException>(() => element.QualifiedName);
        Assert.Equal("document carries no \"qualifiedName\" for element c1; owned-only exports omit derived properties", derived.Message);
        Assert.True(derived.IsDerived);
        var owned = Assert.Throws<NotSuppliedException>(() => element.DeclaredName);
        Assert.Equal("document carries no \"declaredName\" for element c1", owned.Message);
        Assert.False(owned.IsDerived);
        Assert.Throws<ArgumentNullException>(() => new NotSuppliedException(null!, "c1"));
        Assert.Throws<ArgumentNullException>(() => new NotSuppliedException(Metaclasses.Comment.Properties[0], null!));
    }

    /// <summary>Values reach an element only through the property and shape its metaclass declares, and only once.</summary>
    [Fact]
    public void SupplyRefusesWhatTheMetaclassCannotHold()
    {
        var part = Metaclasses.PartUsage.CreateElement("p", "PartUsage");
        Assert.True(Metaclasses.PartUsage.TryGetProperty("declaredName", out var declaredName));
        Assert.True(Metaclasses.PartUsage.TryGetProperty("ownedFeature", out var ownedFeature));
        Assert.True(Metaclasses.PartUsage.TryGetProperty("isAbstract", out var isAbstract));
        Assert.True(Metaclasses.Comment.TryGetProperty("body", out var body));

        Assert.Throws<ArgumentException>(() => part.Supply(body, "text"));
        Assert.Throws<ArgumentException>(() => part.Supply(declaredName, 3));
        Assert.Throws<ArgumentException>(() => part.Supply(isAbstract, "true"));
        Assert.Throws<ArgumentException>(() => part.Supply(ownedFeature, Metaclasses.Comment.CreateElement("c", "Comment")));
        Assert.Throws<ArgumentException>(() => part.Supply(ownedFeature, new IElement[] { (IElement)Metaclasses.Comment.CreateElement("c", "Comment") }));
        Assert.Throws<ArgumentException>(() => part.Supply(ownedFeature, new IFeature?[] { null }));
        Assert.False(part.IsSupplied(declaredName));

        part.Supply(declaredName, "wheel");
        Assert.Throws<InvalidOperationException>(() => part.Supply(declaredName, "again"));
        Assert.Equal("wheel", ((IPartUsage)part).DeclaredName);

        part.Supply(ownedFeature, new IPartUsage[] { (IPartUsage)Metaclasses.PartUsage.CreateElement("q", "PartUsage") });
        Assert.Equal(["q"], ((IType)part).OwnedFeature.Select(element => element.Id));
    }

    /// <summary>Property metadata carries the metamodel's range, multiplicity, derivation, defaults and associations.</summary>
    [Fact]
    public void PropertyMetadataFollowsTheMetamodel()
    {
        Assert.True(Metaclasses.Type.TryGetProperty("ownedFeature", out var ownedFeature));
        Assert.Equal("OwnedFeature", ownedFeature.Name);
        Assert.Equal(PropertyRangeKind.Metaclass, ownedFeature.RangeKind);
        Assert.Same(Metaclasses.Feature, ownedFeature.RangeMetaclass);
        Assert.Equal(typeof(IFeature), ownedFeature.ValueType);
        Assert.True(ownedFeature.IsMany && ownedFeature.IsOrdered && ownedFeature.IsUnique && ownedFeature.IsDerived);
        Assert.Equal("Type::ownedFeature", ownedFeature.ToString());
        Assert.Contains(ownedFeature.Subsets, subsetted => subsetted.QualifiedName == "Namespace::ownedMember");
        Assert.Equal("Feature::owningType", ownedFeature.Opposite?.QualifiedName);
        Assert.Same(ownedFeature, ownedFeature.Opposite!.Opposite);

        Assert.True(Metaclasses.Element.TryGetProperty("declaredName", out var declaredName));
        Assert.False(declaredName.IsDerived || declaredName.IsMany || declaredName.IsOrdered);
        Assert.Equal(typeof(string), declaredName.ValueType);
        Assert.Equal(PropertyRangeKind.Primitive, declaredName.RangeKind);
        Assert.Null(declaredName.DefaultLiteral);

        Assert.True(Metaclasses.Type.TryGetProperty("isAbstract", out var isAbstract));
        Assert.Equal(typeof(bool), isAbstract.ValueType);
        Assert.Equal("false", isAbstract.DefaultLiteral);

        Assert.True(Metaclasses.Membership.TryGetProperty("visibility", out var visibility));
        Assert.Equal(PropertyRangeKind.Enumeration, visibility.RangeKind);
        Assert.Equal(typeof(VisibilityKind), visibility.ValueType);
        Assert.Equal("public", visibility.DefaultLiteral);
        Assert.Equal(1, visibility.LowerBound);

        Assert.True(Metaclasses.PartUsage.TryGetProperty("partDefinition", out var partDefinition));
        Assert.Equal(["ItemUsage::itemDefinition"], partDefinition.Subsets.Select(subsetted => subsetted.QualifiedName));
        Assert.Empty(partDefinition.Redefines);
        Assert.True(Metaclasses.ActionUsage.TryGetProperty("actionDefinition", out var actionDefinition));
        Assert.Equal(["Step::behavior", "OccurrenceUsage::occurrenceDefinition"], actionDefinition.Redefines.Select(redefined => redefined.QualifiedName));
        Assert.True(Metaclasses.Relationship.TryGetProperty("relatedElement", out var relatedElement));
        Assert.False(relatedElement.IsUnique);

        var all = Metaclasses.All.SelectMany(metaclass => metaclass.DeclaredProperties).ToList();
        Assert.All(all, property => Assert.All(property.Redefines.Concat(property.Subsets), named =>
            Assert.True(property.DeclaringMetaclass.ConformsTo(named.DeclaringMetaclass), $"{property} names {named}")));
        Assert.All(all, property => Assert.True(property.IsMany || !property.IsOrdered, property.QualifiedName));
    }

    private static System.Type ExpectedType(MetaProperty property)
    {
        if (property.IsMany)
        {
            return typeof(IReadOnlyList<>).MakeGenericType(property.ValueType);
        }

        return property.ValueType.IsValueType ? typeof(Nullable<>).MakeGenericType(property.ValueType) : property.ValueType;
    }
}
