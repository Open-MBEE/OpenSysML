using System.Collections.Generic;
using System.Linq;
using Xunit;
using static OpenMBEE.OpenSysML.Generate.Tests.Symbols;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// How definitions and features are named in C#: namespaces from packages, nested definitions joined by
/// <c>_</c>, keywords escaped, and every ambiguous name refused with the SysML elements behind it rather
/// than given an arbitrary suffix.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class PlannerNamingTests
{
    internal const string Namespace = "Test.Model";

    [Fact]
    public void PackagesBecomeNamespacesAndNestedDefinitionsJoinTheirOwnersNames()
    {
        var plan = Plan(Root(Package("Demo", Definition("Demo::Vehicle", Definition("Demo::Vehicle::Wheel")), Package("Demo::Parts", Definition("Demo::Parts::Bolt")))));

        var vehicle = Find(plan, "Demo::Vehicle");
        var wheel = Find(plan, "Demo::Vehicle::Wheel");
        var bolt = Find(plan, "Demo::Parts::Bolt");
        Assert.Equal(("Test.Model.Demo", "Vehicle", "IVehicle"), (vehicle.Namespace, vehicle.TypeName, vehicle.InterfaceName));
        Assert.Equal(("Test.Model.Demo", "Vehicle_Wheel", "IVehicle_Wheel"), (wheel.Namespace, wheel.TypeName, wheel.InterfaceName));
        Assert.Equal(("Test.Model.Demo.Parts", "Bolt", "IBolt"), (bolt.Namespace, bolt.TypeName, bolt.InterfaceName));
        Assert.Equal("global::Test.Model.Demo.IVehicle_Wheel", wheel.InterfaceReference);
    }

    [Fact]
    public void ADefinitionOwnedByAUsageJoinsTheUsagesName()
    {
        var plan = Plan(Root(Package("P", Feature("P::spare", kind: "partUsage") with { Children = [Definition("P::spare::Kit")] })));

        Assert.Equal("spare_Kit", Find(plan, "P::spare::Kit").TypeName);
    }

    [Fact]
    public void KeywordsAreEscapedAndNamesNoIdentifierHoldsAreSanitized()
    {
        var plan = Plan(Root(Package("event", Definition("event::string", Feature("event::string::class", Scalar("Real"))), Definition("event::2nd stage"))));

        var keyword = Find(plan, "event::string");
        Assert.Equal(("Test.Model.@event", "@string", "Istring"), (keyword.Namespace, keyword.TypeName, keyword.InterfaceName));
        Assert.Equal("Class", Assert.Single(keyword.Features).Member);
        Assert.Equal("_2nd_stage", Find(plan, "event::2nd stage").TypeName);
        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Theory]
    [InlineData("record")]
    [InlineData("file")]
    [InlineData("required")]
    [InlineData("scoped")]
    [InlineData("__arglist")]
    [InlineData("__makeref")]
    [InlineData("__reftype")]
    [InlineData("__refvalue")]
    [InlineData("rec\u200Dord")]
    [InlineData("__arg\u200Dlist")]
    public void RestrictedTypeNamesCompileInDeclarationsConstructorsAndReferences(string name)
    {
        var id = "P::" + name;
        var plan = Plan(Root(Package(
            "P",
            Definition(id),
            Definition("P::Derived", "partDef", [Specializes(id)]),
            Definition("P::Owner", Feature("P::Owner::item", TypedBy(id))))));
        var definition = Find(plan, id);
        var generated = Emitter.Render(plan);

        Assert.Equal("@" + Identifiers.TypeName(name), definition.TypeName);
        Assert.Contains("internal " + definition.TypeName + "(", generated, System.StringComparison.Ordinal);
        Assert.Empty(Compiler.Diagnostics(generated));
    }

    [Theory]
    [InlineData("車")]
    [InlineData("Cafe\u0301")]
    [InlineData("A\u200DB")]
    [InlineData("A\U0001F697B")]
    [InlineData("\U00010400")]
    public void UnicodeTypeNamesCompileAsConsumers(string name)
    {
        var plan = Plan(Root(Package("P", Definition("P::" + name))));

        Assert.Empty(Compiler.Diagnostics(Emitter.Render(plan)));
    }

    [Fact]
    public void FeatureNamesBecomePascalCaseProperties()
    {
        var plan = Plan(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::maxSpeed", Scalar("Real")),
            Named("P::A::top speed", "top speed", Scalar("Real")),
            Named("P::A::2d", "2d", Scalar("Real")),
            Feature("P::A::wheel_count", Scalar("Integer"))))));

        Assert.Equal(["_2d", "MaxSpeed", "TopSpeed", "WheelCount"], Find(plan, "P::A").Features.Select(feature => feature.Member).Order());
    }

    [Fact]
    public void TwoDefinitionsGeneratingOneTypeAreRefusedNamingBoth()
    {
        var problems = Problems(Root(Package("P", Definition("P::my car"), Definition("P::my_car"))));

        Assert.Equal(
            [
                "P::my car (class) and P::my_car (class) would all generate the C# type Test.Model.P.my_car",
                "P::my car (interface) and P::my_car (interface) would all generate the C# type Test.Model.P.Imy_car",
            ],
            problems);
    }

    [Fact]
    public void FormattingCharactersCannotHideTypeCollisions()
    {
        var problems = Problems(Root(Package("P", Definition("P::AB"), Definition("P::A\u200DB"))));

        Assert.Contains(problems, problem => problem.EndsWith("the C# type Test.Model.P.AB", System.StringComparison.Ordinal));
        Assert.Contains(problems, problem => problem.EndsWith("the C# type Test.Model.P.IAB", System.StringComparison.Ordinal));
        Assert.All(problems, problem =>
        {
            Assert.Contains("P::AB", problem, System.StringComparison.Ordinal);
            Assert.Contains("P::A\u200DB", problem, System.StringComparison.Ordinal);
        });
    }

    [Fact]
    public void FormattingCharactersCannotHideInterfaceOrRegistryCollisions()
    {
        var problems = Problems(Root(Package("P", Definition("P::A"), Definition("P::I\u200DA")), Definition("Sysml\u200DTypes")));

        Assert.Contains(problems, problem => problem.Contains("P::A (interface) and P::I\u200DA (class)", System.StringComparison.Ordinal));
        Assert.Contains(problems, problem => problem.Contains("Sysml\u200DTypes (class) and the SysmlTypes class", System.StringComparison.Ordinal));
    }

    [Fact]
    public void FormattingCharactersCannotHideNamespaceCollisions()
    {
        var problems = Problems(Root(Package("AB", Definition("AB::X")), Package("A\u200DB", Definition("A\u200DB::Y"))));

        Assert.Contains("the packages AB and A\u200DB would all generate the C# namespace Test.Model.AB", problems);
    }

    [Fact]
    public void FormattingCharactersCannotHideATypeThatIsAlsoANamespace()
    {
        var problem = Assert.Single(Problems(Root(Package("P", Definition("P::AB"), Package("P::A\u200DB", Definition("P::A\u200DB::X"))))));

        Assert.Contains("P::AB (class)", problem, System.StringComparison.Ordinal);
        Assert.Contains("also a namespace, of P::A\u200DB", problem, System.StringComparison.Ordinal);
    }

    [Fact]
    public void ADefinitionNamedAsAnotherOnesInterfaceIsRefused()
    {
        var problem = Assert.Single(Problems(Root(Package("P", Definition("P::Engine"), Definition("P::IEngine")))));

        Assert.Contains("P::Engine (interface) and P::IEngine (class)", problem);
    }

    [Fact]
    public void ATypeThatIsAlsoANamespaceIsRefused()
    {
        var problem = Assert.Single(Problems(Root(Package("P", Definition("P::Q"), Package("P::Q", Definition("P::Q::X"))))));

        Assert.Contains("P::Q (class)", problem);
        Assert.Contains("also a namespace, of P::Q", problem);
    }

    [Fact]
    public void TwoPackagesGeneratingOneNamespaceAreRefused()
    {
        var problems = Problems(Root(Package("my pkg", Definition("my pkg::A")), Package("my_pkg", Definition("my_pkg::B"))));

        Assert.Contains("the packages my pkg and my_pkg would all generate the C# namespace Test.Model.my_pkg", problems);
    }

    [Fact]
    public void ADefinitionNamedAsTheRegistryOrAClassMemberIsRefused()
    {
        var problems = Problems(Root(Definition("SysmlTypes"), Package("P", Definition("P::FromInstance"))));

        Assert.Contains(problems, problem => problem.Contains("SysmlTypes (class) and the SysmlTypes class", System.StringComparison.Ordinal));
        Assert.Contains(problems, problem => problem.StartsWith("P::FromInstance would generate the class FromInstance", System.StringComparison.Ordinal));
    }

    [Fact]
    public void FeaturesWhosePropertyNamesCollideAreRefusedNamingBoth()
    {
        var problems = Problems(Root(Package("P", Definition("P::A", Feature("P::A::mass", Scalar("Real")), Feature("P::A::Mass", Scalar("Real"))))));

        Assert.Contains("P::A::Mass and P::A::mass would both generate the property IA.Mass", problems);
    }

    [Fact]
    public void AFeatureNamedAsAMemberEveryInterfaceHasIsRefused()
    {
        var problems = Problems(Root(Package("P", Definition("P::A", Feature("P::A::instance"), Feature("P::A::toString"), Feature("P::A::iA")))));

        Assert.Contains("P::A::instance would generate the property Instance, which every generated interface already has", problems);
        Assert.Contains("P::A::toString would generate the property ToString, which every generated interface already has", problems);
        Assert.Contains("P::A::iA would generate the property IA, which may not share the name of its interface", problems);
    }

    [Fact]
    public void AFeatureWithNoLetterOrDigitIsRefusedAndAnUnnamedOneIsCounted()
    {
        Assert.Contains("P::A::+ has no letter or digit a C# property name can carry", Problems(Root(Package("P", Definition("P::A", Named("P::A::+", "+"))))));

        var plan = Plan(Root(Package("P", Definition("P::A", Named("P::A::", string.Empty, kind: "partUsage"), Feature("P::A::x", Scalar("Real"))))));
        var definition = Find(plan, "P::A");
        Assert.Equal(1, definition.UnnamedFeatures);
        Assert.Single(definition.Features);
        Assert.Contains("Its 1 unnamed feature has no property", Emitter.Render(plan));
    }

    [Fact]
    public void AnUnnamedDefinitionIsRefused()
    {
        var problem = Assert.Single(Problems(Root(Package("P", Definition("P::", "partDef", [])))));

        Assert.Contains("has no name a C# type can carry", problem);
    }

    internal static GenerationPlan Plan(params SymbolNode[] roots) => Planner.Plan(roots, Namespace, "sha256:0", ["test.sysml"]);

    internal static IReadOnlyList<string> Problems(params SymbolNode[] roots) => Assert.Throws<GenerationException>(() => Plan(roots)).Problems;

    internal static DefinitionPlan Find(GenerationPlan plan, string id) => plan.Definitions.Single(definition => definition.Id == id);
}
