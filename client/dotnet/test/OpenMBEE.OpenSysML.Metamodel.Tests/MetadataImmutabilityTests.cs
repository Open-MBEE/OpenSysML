using System;
using System.Collections;
using System.Collections.Generic;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>Metadata collections are immutable snapshots, not writable views of the registry.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class MetadataImmutabilityTests
{
    /// <summary>Every public metadata collection refuses generic and non-generic list mutations.</summary>
    [Fact]
    public void PublicCollectionsCannotBeMutated()
    {
        AssertReadOnly(Metaclasses.All);
        AssertReadOnly(Metaclasses.Enumerations);
        foreach (var metaclass in Metaclasses.All)
        {
            AssertReadOnly(metaclass.Parents);
            AssertReadOnly(metaclass.Ancestors);
            AssertReadOnly(metaclass.DeclaredProperties);
            AssertReadOnly(metaclass.Properties);
            foreach (var property in metaclass.DeclaredProperties)
            {
                AssertReadOnly(property.Redefines);
                AssertReadOnly(property.Subsets);
            }
        }

        foreach (var enumeration in Metaclasses.Enumerations)
        {
            AssertReadOnly(enumeration.Literals);
            foreach (var literal in enumeration.Literals)
            {
                Assert.True(enumeration.TryParse(literal, out var value));
                Assert.Equal(literal, enumeration.GetLiteral(value));
            }
        }
    }

    /// <summary>Initialization and association linking do not retain caller-owned arrays.</summary>
    [Fact]
    public void InitializationCopiesInputCollections()
    {
        Assert.Throws<ArgumentNullException>(() => new MetaclassInfo("Root", true, typeof(IElement), null, null!));
        Assert.Throws<ArgumentException>(() => new MetaclassInfo("Root", false, typeof(IElement), null, NeverCreated));
        Assert.Throws<ArgumentException>(() => new MetaclassInfo("Root", true, typeof(IElement), typeof(Elements.Comment), NeverCreated));
        var root = new MetaclassInfo("Root", true, typeof(IElement), null, NeverCreated);
        var rootProperty = TextProperty(root, "text");
        root.Initialize([], [], [rootProperty]);
        var child = new MetaclassInfo("Child", true, typeof(IElement), null, NeverCreated);
        var childProperty = TextProperty(child, "name");
        MetaclassInfo[] parents = [root];
        MetaclassInfo[] ancestors = [root];
        MetaProperty[] declared = [childProperty];
        MetaProperty[] redefines = [rootProperty];
        MetaProperty[] subsets = [rootProperty];
        child.Initialize(parents, ancestors, declared);
        childProperty.Link(redefines, subsets, rootProperty);

        parents[0] = Metaclasses.Comment;
        ancestors[0] = Metaclasses.Comment;
        declared[0] = rootProperty;
        redefines[0] = childProperty;
        subsets[0] = childProperty;

        Assert.Same(root, Assert.Single(child.Parents));
        Assert.Same(root, Assert.Single(child.Ancestors));
        Assert.Same(childProperty, Assert.Single(child.DeclaredProperties));
        Assert.Equal([childProperty, rootProperty], child.Properties);
        Assert.True(child.ConformsTo(root));
        Assert.False(child.ConformsTo(Metaclasses.Comment));
        Assert.True(child.TryGetProperty("name", out var byKey));
        Assert.Same(childProperty, byKey);
        Assert.True(child.TryGetPropertyByName("Name", out var byName));
        Assert.Same(childProperty, byName);
        Assert.Same(rootProperty, Assert.Single(childProperty.Redefines));
        Assert.Same(rootProperty, Assert.Single(childProperty.Subsets));
        Assert.Same(rootProperty, childProperty.Opposite);
    }

    /// <summary>Changing an enumeration's source array cannot desynchronize literals and their lookup maps.</summary>
    [Fact]
    public void EnumerationCopiesLiteralsAndPreservesRoundTrips()
    {
        string[] literals = ["in", "inout", "out"];
        var enumeration = new EnumerationInfo("FeatureDirectionKind", typeof(FeatureDirectionKind), literals);
        literals[0] = "changed";

        Assert.Equal(["in", "inout", "out"], enumeration.Literals);
        Assert.Equal("in", enumeration.GetLiteral(FeatureDirectionKind.In));
        Assert.True(enumeration.TryParse("in", out var value));
        Assert.Equal(FeatureDirectionKind.In, value);
        Assert.False(enumeration.TryParse("changed", out _));
    }

    private static MetamodelElement NeverCreated(string id, string jsonType) =>
        throw new InvalidOperationException($"no element {id} of {jsonType} is created here");

    private static MetaProperty TextProperty(MetaclassInfo owner, string key) =>
        new(owner, key, char.ToUpperInvariant(key[0]) + key[1..], PropertyRangeKind.Primitive,
            null, null, typeof(string), isMany: false, isOrdered: false, isUnique: true,
            isDerived: false, lowerBound: 0, defaultLiteral: null);

    private static void AssertReadOnly<T>(IReadOnlyList<T> values)
    {
        Assert.False(values is T[], "A public metadata collection exposes its backing array.");
        if (values is IList<T> generic)
        {
            Assert.True(generic.IsReadOnly);
            Assert.Throws<NotSupportedException>(() => generic.Add(default!));
            Assert.Throws<NotSupportedException>(() => generic.Clear());
            if (values.Count > 0)
            {
                Assert.Throws<NotSupportedException>(() => generic[0] = values[0]);
            }
        }

        if (values is IList list)
        {
            Assert.True(list.IsReadOnly);
            Assert.Throws<NotSupportedException>(() => list.Clear());
            if (values.Count > 0)
            {
                Assert.Throws<NotSupportedException>(() => list[0] = values[0]);
            }
        }
    }
}
