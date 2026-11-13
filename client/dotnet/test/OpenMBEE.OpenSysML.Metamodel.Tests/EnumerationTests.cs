using System;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>Each metamodel enumeration is a C# enum whose members map to and from the literals a document writes.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class EnumerationTests
{
    /// <summary>Members are the literals PascalCased, in declaration order, valued by position, and map back to them.</summary>
    [Fact]
    public void EnumMembersMapToLiterals()
    {
        Assert.Equal(
            ["FeatureDirectionKind", "PortionKind", "RequirementConstraintKind", "StateSubactionKind", "TransitionFeatureKind", "TriggerKind", "VisibilityKind"],
            Metaclasses.Enumerations.Select(enumeration => enumeration.Name));
        foreach (var enumeration in Metaclasses.Enumerations)
        {
            Assert.True(enumeration.EnumType.IsEnum);
            Assert.Equal(enumeration.Name, enumeration.EnumType.Name);
            Assert.Equal(enumeration.Name, enumeration.ToString());
            Assert.Equal(
                enumeration.Literals.Select(literal => char.ToUpperInvariant(literal[0]) + literal[1..]),
                Enum.GetNames(enumeration.EnumType));
            for (var index = 0; index < enumeration.Literals.Count; index++)
            {
                var literal = enumeration.Literals[index];
                Assert.True(enumeration.TryParse(literal, out var value));
                Assert.Equal(index, Convert.ToInt32(value, System.Globalization.CultureInfo.InvariantCulture));
                Assert.Equal(enumeration.EnumType, value.GetType());
                Assert.Equal(literal, enumeration.GetLiteral(value));
                Assert.False(enumeration.TryParse(char.ToUpperInvariant(literal[0]) + literal[1..], out _));
            }

            Assert.True(Metaclasses.TryGetEnumeration(enumeration.Name, out var byName));
            Assert.Same(enumeration, byName);
        }

        Assert.Equal(["in", "inout", "out"], EnumerationInfo.Of<FeatureDirectionKind>().Literals);
        Assert.Equal("inout", EnumerationInfo.Of<FeatureDirectionKind>().GetLiteral(FeatureDirectionKind.Inout));
        Assert.True(EnumerationInfo.Of<VisibilityKind>().TryParse("protected", out var visibility));
        Assert.Equal(VisibilityKind.Protected, visibility);
    }

    /// <summary>Unknown literals and foreign enums are refused rather than coerced.</summary>
    [Fact]
    public void EnumerationsRefuseWhatIsNotTheirs()
    {
        var direction = EnumerationInfo.Of<FeatureDirectionKind>();
        Assert.False(direction.TryParse("sideways", out _));
        Assert.False(direction.TryParse(string.Empty, out _));
        Assert.Throws<ArgumentNullException>(() => direction.TryParse(null!, out _));
        Assert.Throws<ArgumentException>(() => direction.GetLiteral(VisibilityKind.Public));
        Assert.Throws<ArgumentException>(() => direction.GetLiteral((FeatureDirectionKind)7));
        Assert.Throws<ArgumentNullException>(() => direction.GetLiteral(null!));
        Assert.Throws<ArgumentException>(() => EnumerationInfo.Of<DayOfWeek>());
        Assert.False(Metaclasses.TryGetEnumeration("featureDirectionKind", out _));
        Assert.False(Metaclasses.TryGetEnumeration("PartUsage", out _));
    }
}
