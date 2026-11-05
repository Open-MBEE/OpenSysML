using System.Linq;
using Xunit;
using static OpenMBEE.OpenSysML.Generate.Tests.PlannerNamingTests;
using static OpenMBEE.OpenSysML.Generate.Tests.Symbols;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// The C# type each feature is read as: library scalars exactly, quantities, enumeration literals and generated
/// objects, with an honest <c>Value</c> for a type that has no sound counterpart; and how many values, from the
/// multiplicity the feature declares or inherits.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class PlannerTypingTests
{
    [Fact]
    public void LibraryScalarsMapToTheirExactCounterparts()
    {
        var plan = Plan(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::b", Scalar("Boolean")),
            Feature("P::A::c", Scalar("Complex")),
            Feature("P::A::i", Scalar("Integer")),
            Feature("P::A::n", Scalar("Natural")),
            Feature("P::A::q", Scalar("Rational")),
            Feature("P::A::r", Scalar("Real")),
            Feature("P::A::s", Scalar("String"))))));

        Assert.Equal(
            [ElementKind.Boolean, ElementKind.Complex, ElementKind.Integer, ElementKind.Integer, ElementKind.Rational, ElementKind.Real, ElementKind.String],
            Find(plan, "P::A").Features.Select(feature => feature.Shape.Kind));
        var source = Emitter.Render(plan);
        Assert.Contains("global::System.Numerics.BigInteger N { get; }", source);
        Assert.Contains("global::OpenMBEE.OpenSysML.Rational Q { get; }", source);
        Assert.Contains("global::System.Numerics.Complex C { get; }", source);
        Assert.Contains("FeatureDecoders.AsComplex", source);
    }

    [Fact]
    public void AQuantityIsAQuantityOnlyWhenItsValueNamesAUnit()
    {
        var plan = Plan(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::m", new TypeFacts("ISQ::MassValue", "ISQBase::MassValue", "attributeDef", "", "", true, "SI::kg")),
            Feature("P::A::v", new TypeFacts("ISQ::MassValue", "ISQBase::MassValue", "attributeDef", "", "", true, ""))))));

        var features = Find(plan, "P::A").Features;
        Assert.Equal(ElementKind.Quantity, features[0].Shape.Kind);
        Assert.Equal(ElementKind.Value, features[1].Shape.Kind);
        var source = Emitter.Render(plan);
        Assert.Contains("A quantity: its value is written in [SI::kg].", source);
        Assert.Contains("may hold a number or a quantity", source);
    }

    [Fact]
    public void ATypeWithNoSoundCounterpartIsReadAsAValueSayingWhy()
    {
        var plan = Plan(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::n", Scalar("Number")),
            Feature("P::A::p", TypedBy("Parts::Part")),
            Feature("P::A::u")))));

        var notes = Find(plan, "P::A").Features.Select(feature => (feature.Shape.Kind, feature.Shape.Note)).ToList();
        Assert.Equal(
            [
                (ElementKind.Value, "SysML Number has no C# counterpart"),
                (ElementKind.Value, "its type Parts::Part has no generated class"),
                (ElementKind.Value, "the feature is untyped"),
            ],
            notes);
        Assert.Contains("Read as a Value: SysML Number has no C# counterpart.", Emitter.Render(plan));
    }

    [Fact]
    public void AnEnumerationHasNoClassAndItsFeaturesHoldLiterals()
    {
        var plan = Plan(Root(Package("P", Definition("P::Color", "enumDef", [], Feature("P::Color::red", kind: "enumUsage")), Definition("P::Car", Feature("P::Car::c", TypedBy("P::Color", "enumDef"))))));

        Assert.DoesNotContain(plan.Definitions, definition => definition.Id == "P::Color");
        Assert.Equal(ElementKind.EnumLiteral, Assert.Single(Find(plan, "P::Car").Features).Shape.Kind);
        Assert.Contains("global::OpenMBEE.OpenSysML.Value.EnumLiteralValue C { get; }", Emitter.Render(plan));
    }

    [Fact]
    public void ADefinitionReducingToAScalarHoldsTheScalarAndAnotherHoldsItsObjects()
    {
        var watt = new TypeFacts("Watt", "P::Watt", "attributeDef", "Real", "declared", false, "");
        var plan = Plan(Root(Package("P", Definition("P::Watt", "attributeDef", [Specializes("ScalarValues::Real", "attributeDef")]), Definition("P::Engine", Feature("P::Engine::power", watt)), Definition("P::Car", Feature("P::Car::engine", TypedBy("P::Engine"), kind: "partUsage")))));

        Assert.Equal(ElementKind.Real, Assert.Single(Find(plan, "P::Engine").Features).Shape.Kind);
        var engine = Assert.Single(Find(plan, "P::Car").Features).Shape;
        Assert.Equal((ElementKind.Generated, "P::Engine"), (engine.Kind, engine.Definition!.Id));
        var source = Emitter.Render(plan);
        Assert.Contains("Registry.Decoder<global::Test.Model.P.IEngine>(global::Test.Model.P.Engine.SysmlId)", source);
        Assert.Contains("Also specializes ScalarValues::Real, which has no generated class.", source);
    }

    [Theory]
    [InlineData(null, null, "Required")]
    [InlineData("1", "1", "Required")]
    [InlineData("0", "1", "Optional")]
    [InlineData("0", "*", "Many")]
    [InlineData("1", "*", "Many")]
    [InlineData("4", "4", "Many")]
    [InlineData("", "", "Many")]
    [InlineData("0", "", "Many")]
    [InlineData("", "1", "Optional")]
    [InlineData("0", "18446744073709551616", "Many")]
    public void MultiplicityDecidesHowManyValuesAreRead(string? lower, string? upper, string expected)
    {
        var multiplicity = lower is null ? null : Range(lower, upper!);
        var plan = Plan(Root(Package("P", Definition("P::A", Feature("P::A::x", Scalar("Real"), multiplicity)))));

        Assert.Equal(System.Enum.Parse<Cardinality>(expected), Assert.Single(Find(plan, "P::A").Features).Shape.Cardinality);
    }

    [Fact]
    public void EachCardinalityReadsThroughItsAccessor()
    {
        var source = Emitter.Render(Plan(Root(Package("P", Definition(
            "P::A",
            Feature("P::A::one", Scalar("Real")),
            Feature("P::A::maybe", Scalar("Real"), Range("0", "1")),
            Feature("P::A::maybeText", Scalar("String"), Range("0", "1")),
            Feature("P::A::many", Scalar("Real"), Range("0", "*")))))));

        Assert.Contains("double global::Test.Model.P.IA.One =>\n            global::OpenMBEE.OpenSysML.Typed.TypedFeatures.Required<double>(this, \"one\"", source);
        Assert.Contains("double? global::Test.Model.P.IA.Maybe =>\n            global::OpenMBEE.OpenSysML.Typed.TypedFeatures.OptionalValue<double>(this, \"maybe\"", source);
        Assert.Contains("string? global::Test.Model.P.IA.MaybeText =>\n            global::OpenMBEE.OpenSysML.Typed.TypedFeatures.Optional<string>(this, \"maybeText\"", source);
        Assert.Contains("global::OpenMBEE.OpenSysML.ValueList<double> global::Test.Model.P.IA.Many =>\n            global::OpenMBEE.OpenSysML.Typed.TypedFeatures.List<double>(this, \"many\"", source);
    }

    [Fact]
    public void ARedefinitionStatingNeitherTypeNorMultiplicityInheritsBoth()
    {
        var plan = Plan(Root(Package("P", Definition("P::Engine", Feature("P::Engine::power", Scalar("Real"), Range("0", "1"))), Definition(
            "P::BigEngine",
            "partDef",
            [Specializes("P::Engine")],
            Feature("P::BigEngine::power", new TypeFacts("", "", "", "Real", "value", false, ""), null, "attributeUsage", Redefines("P::Engine::power"))))));

        var shape = Assert.Single(Find(plan, "P::BigEngine").Features).Shape;
        Assert.Equal((ElementKind.Real, Cardinality.Optional), (shape.Kind, shape.Cardinality));
    }

    [Fact]
    public void AReferenceOutsideTheDocumentsIsRefused()
    {
        var problems = Problems(Root(Package("P", Definition(
            "P::X",
            "partDef",
            [new Specialization("specializes", "Missing", "", "")],
            Feature("P::X::a", new TypeFacts("Nope", "", "", "", "", false, ""), null, "attributeUsage", new Specialization("typing", "Nope", "", ""))))));

        Assert.Equal(
            [
                "P::X refers to Missing, which does not resolve: the documents generated from must declare everything the model refers to outside the standard library",
                "P::X::a refers to Nope, which does not resolve: the documents generated from must declare everything the model refers to outside the standard library",
            ],
            problems);
    }

    [Fact]
    public void AUsageTypedByOneGeneratedDefinitionIsRegisteredAsIt()
    {
        var plan = Plan(Root(Package(
            "P",
            Definition("P::Car"),
            Definition("P::Boat"),
            Feature("P::car", TypedBy("P::Car"), null, "partUsage", Typing("P::Car")),
            Feature("P::sedan", new TypeFacts("", "", "", "", "", false, ""), null, "partUsage", new Specialization("subsets", "car", "P::car", "partUsage")),
            Feature("P::amphibian", TypedBy("P::Car"), null, "partUsage", Typing("P::Car"), Typing("P::Boat")),
            Feature("P::count", Scalar("Integer")))));

        Assert.Equal(
            [("P::car", "P::Car"), ("P::sedan", "P::Car")],
            plan.UsageTypes.Select(entry => (entry.Key, entry.Value.Id)));
        Assert.Contains("new(\"P::sedan\", \"P::Car\"),", Emitter.Render(plan));
    }
}
