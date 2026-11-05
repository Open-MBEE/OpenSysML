using System.Linq;
using Xunit;
using static OpenMBEE.OpenSysML.Generate.Tests.PlannerNamingTests;
using static OpenMBEE.OpenSysML.Generate.Tests.Symbols;

namespace OpenMBEE.OpenSysML.Generate.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class PlannerFactInheritanceTests
{
    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public void MultipleRedefinitionsUseTheNarrowestTypeAndIntersectTheirRanges(bool reverse)
    {
        var edges = new[] { Redefines("P::A::x"), Redefines("P::B::x") };
        var plan = Plan(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real"), Range("0", "*"))),
            Definition("P::B", "partDef", [Specializes("P::A")], Feature("P::B::x", Scalar("Integer"), Range("0", "1"), "attributeUsage", Redefines("P::A::x"))),
            Definition("P::C", "partDef", [Specializes("P::A"), Specializes("P::B")],
                Feature("P::C::x", new TypeFacts("", "", "", "Real", "declared", false, ""), null, "attributeUsage", reverse ? [.. edges.Reverse()] : edges)))));

        var shape = Assert.Single(Find(plan, "P::C").Features).Shape;
        Assert.Equal((ElementKind.Integer, Cardinality.Optional), (shape.Kind, shape.Cardinality));
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void IndependentInheritedRangesAreIntersectedRatherThanChosenByOrder()
    {
        var plan = Plan(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real"), Range("0", "*"))),
            Definition("P::B", Feature("P::B::x", Scalar("Real"), Range("1", "1"))),
            Definition("P::C", "partDef", [Specializes("P::A"), Specializes("P::B")],
                Feature("P::C::x", null, null, "attributeUsage", Redefines("P::A::x"), Redefines("P::B::x"))))));

        Assert.Equal(Cardinality.Required, Assert.Single(Find(plan, "P::C").Features).Shape.Cardinality);
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void IncompatibleInheritedTypesNameEveryProvider()
    {
        var problems = Problems(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real"))),
            Definition("P::B", Feature("P::B::x", Scalar("String"))),
            Definition("P::C", "partDef", [Specializes("P::A"), Specializes("P::B")],
                Feature("P::C::x", null, null, "attributeUsage", Redefines("P::A::x"), Redefines("P::B::x"))))));

        Assert.Contains("P::C::x inherits incompatible types from P::A::x and P::B::x", problems);
    }

    [Fact]
    public void DisjointInheritedRangesNameEveryProvider()
    {
        var problems = Problems(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real"), Range("0", "0"))),
            Definition("P::B", Feature("P::B::x", Scalar("Real"), Range("1", "1"))),
            Definition("P::C", "partDef", [Specializes("P::A"), Specializes("P::B")],
                Feature("P::C::x", null, null, "attributeUsage", Redefines("P::A::x"), Redefines("P::B::x"))))));

        Assert.Contains("P::C::x inherits disjoint multiplicities from P::A::x and P::B::x", problems);
    }

    [Fact]
    public void CyclicRedefinitionsFailWithoutRecursingOrIndexingAnEmptySlotSet()
    {
        var problems = Problems(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::x", null, null, "attributeUsage", Redefines("P::A::y")),
            Feature("P::A::y", null, null, "attributeUsage", Redefines("P::A::x"))))));

        Assert.Contains("P::A inherits a cyclic redefinition of P::A::x", problems);
    }

    [Theory]
    [InlineData("0", "1", "Optional")]
    [InlineData("0", "*", "Many")]
    public void DeepRedefinitionsKeepTheirInheritedTypeAndMultiplicity(string lower, string upper, string cardinality)
    {
        var chain = new SymbolNode[40];
        chain[0] = Definition("P::D0", Feature("P::D0::x", Scalar("Integer"), Range(lower, upper)));
        for (var i = 1; i < chain.Length; i++)
        {
            chain[i] = Definition($"P::D{i}", "partDef", [Specializes($"P::D{i - 1}")],
                Feature($"P::D{i}::x", null, null, "attributeUsage", Redefines($"P::D{i - 1}::x")));
        }

        var plan = Plan(Root(Package("P", chain)));

        Assert.All(plan.Definitions, definition =>
        {
            var shape = Assert.Single(definition.Features).Shape;
            Assert.Equal((ElementKind.Integer, System.Enum.Parse<Cardinality>(cardinality)), (shape.Kind, shape.Cardinality));
        });
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }
}
