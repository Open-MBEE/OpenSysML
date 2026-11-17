using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using OpenMBEE.OpenSysML.Metamodel.Elements;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>The generated types generalize exactly as the metamodel's metaclasses do.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class HierarchyTests
{
    /// <summary>Every metaclass of the committed hierarchy, and no other, has a MetaclassInfo with its parents and abstractness.</summary>
    [Fact]
    public void MetaclassesMatchTheCommittedHierarchy()
    {
        Assert.Equal(175, CommittedOntology.Metaclasses.Count);
        Assert.Equal(
            CommittedOntology.Metaclasses.Keys.Order(StringComparer.Ordinal),
            Metaclasses.All.Select(metaclass => metaclass.Name));
        foreach (var metaclass in Metaclasses.All)
        {
            var committed = CommittedOntology.Metaclasses[metaclass.Name];
            Assert.Equal(committed.IsAbstract, metaclass.IsAbstract);
            Assert.Equal(committed.Parents.Order(StringComparer.Ordinal), metaclass.Parents.Select(parent => parent.Name).Order(StringComparer.Ordinal));
            Assert.Equal(
                CommittedOntology.AncestorsOrSelf(metaclass.Name).Where(name => name != metaclass.Name).Order(StringComparer.Ordinal),
                metaclass.Ancestors.Select(ancestor => ancestor.Name).Order(StringComparer.Ordinal));
        }
    }

    /// <summary>Each interface is assignable to exactly the interfaces of its metaclass's ancestors, and ConformsTo agrees.</summary>
    [Fact]
    public void InterfacesConformExactlyAsMetaclassesGeneralize()
    {
        foreach (var metaclass in Metaclasses.All)
        {
            var ancestors = CommittedOntology.AncestorsOrSelf(metaclass.Name);
            Assert.True(metaclass.InterfaceType.IsInterface, metaclass.Name);
            Assert.Equal("I" + metaclass.Name, metaclass.InterfaceType.Name);
            Assert.True(typeof(IMetamodelElement).IsAssignableFrom(metaclass.InterfaceType), metaclass.Name);
            foreach (var other in Metaclasses.All)
            {
                var expected = ancestors.Contains(other.Name);
                Assert.True(expected == other.InterfaceType.IsAssignableFrom(metaclass.InterfaceType), $"I{metaclass.Name} : I{other.Name} should be {expected}");
                Assert.True(expected == metaclass.ConformsTo(other), $"{metaclass.Name}.ConformsTo({other.Name}) should be {expected}");
                if (metaclass.ElementType is { } elementType)
                {
                    Assert.True(expected == other.InterfaceType.IsAssignableFrom(elementType), $"{metaclass.Name} : I{other.Name} should be {expected}");
                }
            }
        }
    }

    /// <summary>
    /// Selecting elements by a metaclass interface, as <c>OfType</c> or <c>is</c> does, selects every element whose
    /// metaclass specializes it and no other.
    /// </summary>
    [Fact]
    public void SelectingByInterfaceMatchesSemanticAncestry()
    {
        var elements = Fixtures.OneOfEach();
        var ofType = typeof(Enumerable).GetMethod(nameof(Enumerable.OfType))!;
        foreach (var metaclass in Metaclasses.All)
        {
            var expected = elements
                .Where(element => CommittedOntology.AncestorsOrSelf(element.MetaclassInfo.Name).Contains(metaclass.Name))
                .Select(element => element.Id);
            var selected = ((IEnumerable<object>)ofType.MakeGenericMethod(metaclass.InterfaceType).Invoke(null, [elements])!)
                .Cast<IMetamodelElement>()
                .Select(element => element.Id);
            Assert.Equal(expected, selected);
        }

        var parts = elements.OfType<IPartUsage>().Select(element => element.MetaclassInfo.Name).ToList();
        Assert.Contains("PartUsage", parts);
        Assert.Contains("ConnectionUsage", parts);
        Assert.DoesNotContain("ItemUsage", parts);
        Assert.All(parts, name => Assert.True(Metaclasses.All.Single(metaclass => metaclass.Name == name).ConformsTo(Metaclasses.PartUsage)));
        var actionUsage = elements.Single(element => element is ActionUsage);
        Assert.True(actionUsage is IStep and IOccurrenceUsage and IFeature and IElement);
        Assert.False(actionUsage is IPartUsage);
        Assert.False(elements.Single(element => element is Namespace) is IFeature);
    }

    /// <summary>
    /// Concrete metaclasses, and only they, have a public sealed element class that only a reader can construct;
    /// an abstract one's elements, those a document types as it, are of an internal sealed class read through its interface.
    /// </summary>
    [Fact]
    public void ConcreteMetaclassesHaveSealedElementClasses()
    {
        var elementClasses = typeof(MetamodelElement).Assembly.GetExportedTypes()
            .Where(type => type.Namespace == "OpenMBEE.OpenSysML.Metamodel.Elements")
            .ToHashSet();
        Assert.Equal(167, elementClasses.Count);
        foreach (var metaclass in Metaclasses.All)
        {
            if (metaclass.IsAbstract)
            {
                Assert.Null(metaclass.ElementType);
                var standIn = metaclass.CreateElement("x", "vendor:Thing");
                var standInType = standIn.GetType();
                Assert.False(standInType.IsVisible, metaclass.Name);
                Assert.True(standInType.IsSealed, metaclass.Name);
                Assert.Equal(typeof(MetamodelElement), standInType.BaseType);
                Assert.IsAssignableFrom(metaclass.InterfaceType, standIn);
                Assert.Same(metaclass, standIn.MetaclassInfo);
                Assert.Equal("vendor:Thing", standIn.JsonType);
                Assert.False(Metaclasses.TryGet(standInType, out _));
                Assert.All(
                    Metaclasses.All.Where(other => !metaclass.ConformsTo(other)),
                    other => Assert.False(other.InterfaceType.IsInstanceOfType(standIn), $"{metaclass.Name} is no {other.Name}"));
                continue;
            }

            var elementType = metaclass.ElementType!;
            Assert.Contains(elementType, elementClasses);
            Assert.Equal(metaclass.Name, elementType.Name);
            Assert.True(elementType.IsSealed, metaclass.Name);
            Assert.Equal(typeof(MetamodelElement), elementType.BaseType);
            Assert.Empty(elementType.GetConstructors(BindingFlags.Public | BindingFlags.Instance));
            Assert.True(Metaclasses.TryGet(elementType, out var byElementType));
            Assert.Same(metaclass, byElementType);
            Assert.True(Metaclasses.TryGet(metaclass.InterfaceType, out var byInterfaceType));
            Assert.Same(metaclass, byInterfaceType);

            var element = metaclass.CreateElement("e1", "sysml:" + metaclass.Name);
            Assert.IsType(elementType, element);
            Assert.Equal("e1", element.Id);
            Assert.Equal("sysml:" + metaclass.Name, element.JsonType);
            Assert.Same(metaclass, element.MetaclassInfo);
            Assert.Equal($"{metaclass.Name} 'e1'", element.ToString());
        }
    }

    /// <summary>Ancestors are each metaclass's linearization: every superclass once, each before its own superclasses.</summary>
    [Fact]
    public void AncestorsAreLinearizedNearestFirst()
    {
        foreach (var metaclass in Metaclasses.All)
        {
            var ancestors = metaclass.Ancestors;
            Assert.Equal(ancestors.Count, ancestors.Distinct().Count());
            Assert.All(metaclass.Parents, parent => Assert.Contains(parent, ancestors));
            for (var i = 0; i < ancestors.Count; i++)
            {
                for (var j = i + 1; j < ancestors.Count; j++)
                {
                    Assert.False(ancestors[j].ConformsTo(ancestors[i]), $"{metaclass.Name}: {ancestors[j]} follows its superclass {ancestors[i]}");
                }
            }
        }

        Assert.Equal(
            ["ItemUsage", "OccurrenceUsage", "Usage", "Feature", "Type", "Namespace", "Element"],
            Metaclasses.PartUsage.Ancestors.Select(ancestor => ancestor.Name));
    }

    /// <summary>The registry finds metaclasses by name and by generated type, and refuses anything else.</summary>
    [Fact]
    public void RegistryLooksUpByNameAndType()
    {
        Assert.True(Metaclasses.TryGet("PartUsage", out var byName));
        Assert.Same(Metaclasses.PartUsage, byName);
        Assert.False(Metaclasses.TryGet("partUsage", out _));
        Assert.False(Metaclasses.TryGet("sysml:PartUsage", out _));
        Assert.Same(Metaclasses.PartUsage, Metaclasses.Of<IPartUsage>());
        Assert.Same(Metaclasses.PartUsage, Metaclasses.Of<PartUsage>());
        Assert.Same(Metaclasses.Element, Metaclasses.Of<IElement>());
        Assert.False(Metaclasses.TryGet(typeof(string), out _));
        Assert.False(Metaclasses.TryGet(typeof(MetamodelElement), out _));
        Assert.Throws<ArgumentException>(() => Metaclasses.Of<IMetamodelElement>());
        Assert.Throws<ArgumentNullException>(() => Metaclasses.TryGet((string)null!, out _));
        Assert.Throws<ArgumentNullException>(() => Metaclasses.TryGet((System.Type)null!, out _));
        Assert.Equal(Metaclasses.All.Select(metaclass => metaclass.Name).Order(StringComparer.Ordinal), Metaclasses.All.Select(metaclass => metaclass.Name));
    }
}
