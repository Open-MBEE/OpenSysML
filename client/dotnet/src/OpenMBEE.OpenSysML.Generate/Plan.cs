using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>How many values a generated property reads, from the feature's multiplicity.</summary>
internal enum Cardinality
{
    /// <summary>Exactly one: multiplicity <c>1</c>, or no multiplicity declared.</summary>
    Required,

    /// <summary>At most one: multiplicity <c>0..1</c>.</summary>
    Optional,

    /// <summary>Any number: an upper bound above one, unbounded or statically unknown.</summary>
    Many,
}

/// <summary>The C# type a single value of a feature is read as.</summary>
internal enum ElementKind
{
    /// <summary>The value as it arrived, for a type with no sound C# counterpart.</summary>
    Value,

    /// <summary><c>Boolean</c>.</summary>
    Boolean,

    /// <summary><c>String</c>.</summary>
    String,

    /// <summary><c>Integer</c> or <c>Natural</c>.</summary>
    Integer,

    /// <summary><c>Rational</c>.</summary>
    Rational,

    /// <summary><c>Real</c>.</summary>
    Real,

    /// <summary><c>Complex</c>.</summary>
    Complex,

    /// <summary>A value written with a unit.</summary>
    Quantity,

    /// <summary>A literal of an enumeration.</summary>
    EnumLiteral,

    /// <summary>An object of a generated definition.</summary>
    Generated,
}

/// <summary>What a property reads: the element type, how many, and why a value falls back to <see cref="ElementKind.Value"/>.</summary>
/// <param name="Kind">The element type.</param>
/// <param name="Definition">The generated definition, for <see cref="ElementKind.Generated"/>.</param>
/// <param name="Cardinality">How many values.</param>
/// <param name="Note">Why the element type was chosen, when that is not plain from the model.</param>
internal sealed record Shape(ElementKind Kind, DefinitionPlan? Definition, Cardinality Cardinality, string? Note);

/// <summary>One feature a definition owns, and the property its interface declares for it.</summary>
/// <param name="Symbol">The feature.</param>
/// <param name="Owner">The definition that owns it.</param>
/// <param name="Member">The property's name, as C# spells it.</param>
/// <param name="Shape">What the property reads.</param>
internal sealed record FeaturePlan(Symbol Symbol, DefinitionPlan Owner, string Member, Shape Shape)
{
    /// <summary>The feature's qualified name.</summary>
    internal string Id => Symbol.Id;

    /// <summary>Whether the interface's property hides one of the same name its base interfaces declare.</summary>
    internal bool Hides { get; set; }
}

/// <summary>One property a class implements: an interface's property, read from the feature that holds its value in this class.</summary>
/// <param name="Feature">The property's feature, and the interface that declares it.</param>
/// <param name="Slot">The feature whose value it reads: the feature itself, or what redefines it in this class.</param>
internal sealed record ImplementationPlan(FeaturePlan Feature, FeaturePlan Slot);

/// <summary>One SysML definition, and the interface and class generated for it.</summary>
internal sealed class DefinitionPlan(Symbol symbol, string @namespace, string typeName, string interfaceName)
{
    /// <summary>The definition.</summary>
    internal Symbol Symbol { get; } = symbol;

    /// <summary>The definition's qualified name.</summary>
    internal string Id => Symbol.Id;

    /// <summary>The C# namespace, as source spells it.</summary>
    internal string Namespace { get; } = @namespace;

    /// <summary>The class's name, as source spells it.</summary>
    internal string TypeName { get; } = typeName;

    /// <summary>The interface's name, as source spells it.</summary>
    internal string InterfaceName { get; } = interfaceName;

    /// <summary>The class, fully qualified.</summary>
    internal string ClassReference => $"global::{Namespace}.{TypeName}";

    /// <summary>The interface, fully qualified.</summary>
    internal string InterfaceReference => $"global::{Namespace}.{InterfaceName}";

    /// <summary>The generated definitions it specializes, in the order the model declares them.</summary>
    internal List<DefinitionPlan> Bases { get; } = [];

    /// <summary>The generalization edges to definitions without a generated class, in declared order.</summary>
    internal List<Specialization> UngeneratedBases { get; } = [];

    /// <summary>The features it owns, in ordinal order of their qualified names.</summary>
    internal List<FeaturePlan> Features { get; } = [];

    /// <summary>How many of the features it owns are unnamed, which no property can read by name.</summary>
    internal int UnnamedFeatures { get; set; }

    /// <summary>The properties its class implements itself, rather than inherits from its base class.</summary>
    internal List<ImplementationPlan> Implementations { get; } = [];

    /// <summary>The class it inherits: that of the first generated definition it specializes; null for none.</summary>
    internal DefinitionPlan? BaseClass => Bases.Count == 0 ? null : Bases[0];
}

/// <summary>Everything a generated file holds.</summary>
/// <param name="RootNamespace">The namespace the file's types are generated under.</param>
/// <param name="ModelHash">The stamp of the model source closure.</param>
/// <param name="Documents">The logical names of the closure's documents, in ordinal order.</param>
/// <param name="Definitions">Every generated definition, by C# namespace and then qualified name.</param>
/// <param name="UsageTypes">The generated definition each usage is typed by, in ordinal order of the usage.</param>
internal sealed record GenerationPlan(
    string RootNamespace,
    string ModelHash,
    IReadOnlyList<string> Documents,
    IReadOnlyList<DefinitionPlan> Definitions,
    IReadOnlyList<KeyValuePair<string, DefinitionPlan>> UsageTypes);
