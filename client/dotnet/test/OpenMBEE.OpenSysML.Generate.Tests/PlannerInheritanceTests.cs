using System.Linq;
using Xunit;
using static OpenMBEE.OpenSysML.Generate.Tests.PlannerNamingTests;
using static OpenMBEE.OpenSysML.Generate.Tests.Symbols;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// Inheritance follows the model's facts, never a coincidence of property names: interfaces extend every
/// generated base, classes inherit the first, diamond ancestry is shared, an inherited property reads the
/// feature that redefines it whatever that one is called, and what cannot be read soundly is refused naming
/// the features behind it.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class PlannerInheritanceTests
{
    [Fact]
    public void AnInterfaceExtendsEveryBaseInDeclaredOrderAndTheClassInheritsTheFirst()
    {
        var plan = Plan(Root(Package("P", Definition("P::B", Feature("P::B::b", Scalar("Real"))), Definition("P::A", Feature("P::A::a", Scalar("Real"))), Definition("P::D", "partDef", [Specializes("P::B"), Specializes("P::A")]))));

        var d = Find(plan, "P::D");
        Assert.Equal(["P::B", "P::A"], d.Bases.Select(other => other.Id));
        Assert.Equal("P::B", d.BaseClass!.Id);
        Assert.Equal(["P::A::a"], d.Implementations.Select(implementation => implementation.Feature.Id));
        var source = Emitter.Render(plan);
        Assert.Contains("public interface ID : global::Test.Model.P.IB, global::Test.Model.P.IA", source);
        Assert.Contains("public class D : global::Test.Model.P.B, global::Test.Model.P.ID", source);
        Assert.Empty(Compiler.Diagnostics(source));
    }

    [Fact]
    public void DiamondAncestryIsSharedAndImplementedOnce()
    {
        var plan = Plan(Root(Package(
            "P",
            Definition("P::Base", Feature("P::Base::weight", Scalar("Real"))),
            Definition("P::L", "partDef", [Specializes("P::Base")]),
            Definition("P::R", "partDef", [Specializes("P::Base")], Feature("P::R::r", Scalar("Real"))),
            Definition("P::Diamond", "partDef", [Specializes("P::L"), Specializes("P::R")], Feature("P::Diamond::extra", Scalar("Boolean"))))));

        Assert.Equal(["P::Diamond::extra", "P::R::r"], Find(plan, "P::Diamond").Implementations.Select(implementation => implementation.Feature.Id));
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void ARedefinitionOfTheSameNameHidesAndNarrowsTheInheritedProperty()
    {
        var plan = Plan(Vehicles(Feature("P::Car::engine", TypedBy("P::BigEngine"), null, "partUsage", Redefines("P::Vehicle::engine", "partUsage"))));

        var car = Find(plan, "P::Car");
        var engine = Assert.Single(car.Features);
        Assert.True(engine.Hides);
        Assert.Equal((ElementKind.Generated, Cardinality.Optional), (engine.Shape.Kind, engine.Shape.Cardinality));

        // The base class already reads the slot named engine with the inherited property's decoder.
        Assert.Equal(["P::Car::engine"], car.Implementations.Select(implementation => implementation.Feature.Id));
        var source = Emitter.Render(plan);
        Assert.Contains("new global::Test.Model.P.IBigEngine? Engine { get; }", source);
        Assert.Empty(Compiler.Diagnostics(source));
    }

    [Fact]
    public void AnInheritedPropertyReadsTheFeatureThatRedefinesItUnderAnotherName()
    {
        var plan = Plan(Vehicles(Feature("P::Car::motor", TypedBy("P::BigEngine"), Range("1", "1"), "partUsage", Redefines("P::Vehicle::engine", "partUsage"))));

        var implementations = Find(plan, "P::Car").Implementations.Select(implementation => (implementation.Feature.Id, implementation.Slot.Symbol.Name));
        Assert.Equal([("P::Car::motor", "motor"), ("P::Vehicle::engine", "motor")], implementations);
        var source = Emitter.Render(plan);
        Assert.Contains("global::Test.Model.P.IEngine? global::Test.Model.P.IVehicle.Engine =>\n            global::OpenMBEE.OpenSysML.Typed.TypedFeatures.Optional<global::Test.Model.P.IEngine>(this, \"motor\"", source);
        Assert.Empty(Compiler.Diagnostics(source));
    }

    [Fact]
    public void TwoUnrelatedFeaturesOfOneNameAreRefusedNamingBoth()
    {
        var problems = Problems(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::mass", Scalar("Real"))),
            Definition("P::B", Feature("P::B::mass", Scalar("Real"))),
            Definition("P::D", "partDef", [Specializes("P::A"), Specializes("P::B")]))));

        Assert.Equal(["P::D has the features P::A::mass and P::B::mass, different features that neither redefines the other, which would both be read as the property Mass"], problems);
    }

    [Fact]
    public void ARedefinitionInTheSpecializationJoinsThemAgain()
    {
        var plan = Plan(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::mass", Scalar("Real"))),
            Definition("P::B", Feature("P::B::mass", Scalar("Real"))),
            Definition("P::D", "partDef", [Specializes("P::A"), Specializes("P::B")], Feature("P::D::mass", Scalar("Real"), null, "attributeUsage", Redefines("P::A::mass"), Redefines("P::B::mass"))))));

        var d = Find(plan, "P::D");
        Assert.Equal([("P::D::mass", "mass"), ("P::B::mass", "mass")], d.Implementations.Select(implementation => (implementation.Feature.Id, implementation.Slot.Symbol.Name)));
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void TwoRedefinitionsOfOneFeatureThatMeetAreRefused()
    {
        var problems = Problems(Root(Package(
            "P",
            Definition("P::Base", Feature("P::Base::w", Scalar("Real"))),
            Definition("P::L", "partDef", [Specializes("P::Base")], Feature("P::L::w", Scalar("Real"), null, "attributeUsage", Redefines("P::Base::w"))),
            Definition("P::R", "partDef", [Specializes("P::Base")], Feature("P::R::w", Scalar("Real"), null, "attributeUsage", Redefines("P::Base::w"))),
            Definition("P::D", "partDef", [Specializes("P::L"), Specializes("P::R")]))));

        Assert.Contains("P::D inherits P::L::w and P::R::w, which all redefine P::Base::w and none of which redefines another", problems);
    }

    [Fact]
    public void ARedefinitionThePropertyCannotReadIsRefused()
    {
        var problems = Problems(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real")), Feature("P::A::y", Scalar("Real"), Range("0", "1"))),
            Definition(
                "P::B",
                "partDef",
                [Specializes("P::A")],
                Feature("P::B::x", Scalar("String"), null, "attributeUsage", Redefines("P::A::x")),
                Feature("P::B::y", Scalar("Real"), Range("0", "*"), "attributeUsage", Redefines("P::A::y"))))));

        Assert.Equal(
            [
                "P::B: P::B::x redefines P::A::x as one String, which the property IA.X, one Real, cannot soundly read",
                "P::B: P::B::y redefines P::A::y as many Real, which the property IA.Y, an optional Real, cannot soundly read",
            ],
            problems);
    }

    [Fact]
    public void ANarrowingRedefinitionIsRead()
    {
        var plan = Plan(Root(Package(
            "P",
            Definition("P::A", Feature("P::A::x", Scalar("Real")), Feature("P::A::xs", Scalar("Rational"), Range("0", "*")), Feature("P::A::z", Scalar("Complex"), Range("0", "1"))),
            Definition(
                "P::B",
                "partDef",
                [Specializes("P::A")],
                Feature("P::B::count", Scalar("Integer"), null, "attributeUsage", Redefines("P::A::x")),
                Feature("P::B::xs", Scalar("Integer"), Range("1", "1"), "attributeUsage", Redefines("P::A::xs")),
                Feature("P::B::z", Scalar("Real"), Range("1", "1"), "attributeUsage", Redefines("P::A::z"))))));

        Assert.Equal(["P::B::count", "P::B::xs", "P::B::z", "P::A::x"], Find(plan, "P::B").Implementations.Select(implementation => implementation.Feature.Id));
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void ADefinitionSpecializingItselfIsRefused()
    {
        var problems = Problems(Root(Package("P", Definition("P::A", "partDef", [Specializes("P::B")]), Definition("P::B", "partDef", [Specializes("P::A")]))));

        Assert.Equal(["P::A specializes itself, which no C# interface can", "P::B specializes itself, which no C# interface can"], problems);
    }

    [Fact]
    public void TheSameModelInAnotherOrderGivesTheSameFile()
    {
        var a = Definition("P::A", Feature("P::A::y", Scalar("Real")), Feature("P::A::x", Scalar("Real")));
        var b = Definition("P::B", "partDef", [Specializes("P::A")], Feature("P::B::z", Scalar("Boolean")));
        var other = Package("Q", Definition("Q::C"));

        var first = Emitter.Render(Plan(Root(Package("P", a, b), other)));
        var second = Emitter.Render(Plan(Root(other, Package("P", b with { }, a with { Children = [.. a.Children.Reverse()] }))));

        Assert.Equal(first, second);
        Assert.True(first.IndexOf("namespace Test.Model.P\n", System.StringComparison.Ordinal) < first.IndexOf("namespace Test.Model.Q\n", System.StringComparison.Ordinal));
    }

    [Fact]
    public void AnEmptyModelGivesAFileThatCompiles()
    {
        var source = Emitter.Render(Plan(Root()));

        Assert.Contains("public const string ModelHash = \"sha256:0\";", source);
        Assert.Contains($"public const string GeneratorVersion = \"{Emitter.GeneratorVersion}\";", source);
        Assert.Empty(Compiler.Diagnostics(source));
    }

    // Vehicle owns an optional engine; Car specializes it with the given feature.
    private static SymbolNode Vehicles(SymbolNode carFeature) => Root(Package(
        "P",
        Definition("P::Engine", Feature("P::Engine::power", Scalar("Real"))),
        Definition("P::BigEngine", "partDef", [Specializes("P::Engine")]),
        Definition("P::Vehicle", Feature("P::Vehicle::engine", TypedBy("P::Engine"), Range("0", "1"), "partUsage")),
        Definition("P::Car", "partDef", [Specializes("P::Vehicle")], carFeature)));
}
