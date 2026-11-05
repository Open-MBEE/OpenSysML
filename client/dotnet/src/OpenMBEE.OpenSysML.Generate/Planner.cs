using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Numerics;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>
/// Decides everything a generated file holds from a model's symbol trees, and refuses a model it cannot
/// generate soundly with every problem found, each naming the SysML elements behind it.
/// </summary>
/// <remarks>
/// <para>
/// Every definition but an enumeration's gets an interface, declaring a property for each feature it owns, and
/// a class implementing it. The interface extends those of the generated definitions it specializes, in the
/// order the model declares them; the class inherits the class of the first. Every property is implemented
/// explicitly, by each class whose value for it differs from what the class it inherits reads.
/// </para>
/// <para>
/// A feature's identity is its qualified name, never its property's name: a feature a definition inherits is
/// read, in that definition, from the most specific feature that redefines it there, whatever that one is
/// called. A definition inheriting two features that neither redefines the other, under one property name, or
/// two redefinitions of one feature that neither redefines the other, is refused, as is a redefinition a
/// redefined feature's property cannot soundly read.
/// </para>
/// </remarks>
internal static class Planner
{
    /// <summary>The class, in the root namespace, holding the stamp and the registry.</summary>
    internal const string RegistryName = "SysmlTypes";

    // The edges that make one element a kind of another; a typing edge names the element's type instead.
    private static readonly FrozenSet<string> GeneralizationKinds = FrozenSet.ToFrozenSet(["specializes", "subsets", "redefines"], StringComparer.Ordinal);

    // The usages that are an instance's feature values. Behavioral and connector usages are not, and get no property.
    private static readonly FrozenSet<string> FeatureKinds = FrozenSet.ToFrozenSet(
        ["attributeusage", "referenceusage", "partusage", "itemusage", "occurrenceusage", "individualusage", "portusage", "enumusage"],
        StringComparer.Ordinal);

    private static readonly FrozenSet<string> NamespaceKinds = FrozenSet.ToFrozenSet(
        ["package", "librarypackage", "namespace", "rootnamespace"], StringComparer.Ordinal);

    // What every generated interface already has, through ITypedObject or object; a property may not hide it.
    private static readonly FrozenSet<string> ReservedMembers = FrozenSet.ToFrozenSet(
        ["Instance", "Instantiation", "Equals", "GetHashCode", "GetType", "ToString", "MemberwiseClone", "Finalize", "ReferenceEquals"],
        StringComparer.Ordinal);

    // The members a generated class declares; a class may not share its own member's name.
    private static readonly FrozenSet<string> ClassMembers = FrozenSet.ToFrozenSet(["SysmlId", "FromInstance", "Unchecked"], StringComparer.Ordinal);

    private static readonly FrozenDictionary<string, ElementKind> Primitives = new Dictionary<string, ElementKind>(StringComparer.Ordinal)
    {
        ["Boolean"] = ElementKind.Boolean,
        ["String"] = ElementKind.String,
        ["Natural"] = ElementKind.Integer,
        ["Integer"] = ElementKind.Integer,
        ["Rational"] = ElementKind.Rational,
        ["Real"] = ElementKind.Real,
        ["Complex"] = ElementKind.Complex,
    }.ToFrozenDictionary(StringComparer.Ordinal);

    /// <summary>Whether a symbol kind declares a type, as <c>partDef</c> or <c>metaclass</c>.</summary>
    internal static bool IsDefinitionKind(string kind)
    {
        var lowered = kind.ToLowerInvariant();
        return lowered.EndsWith("def", StringComparison.Ordinal) || lowered == "metaclass";
    }

    /// <summary>Whether a symbol kind is a usage an instance holds a value for.</summary>
    internal static bool IsFeatureKind(string kind) => FeatureKinds.Contains(kind.ToLowerInvariant());

    /// <summary>Plans the file generated from a model's symbol trees.</summary>
    /// <exception cref="GenerationException">The model cannot be generated soundly.</exception>
    internal static GenerationPlan Plan(IReadOnlyList<SymbolNode> roots, string rootNamespace, string modelHash, IReadOnlyList<string> documents) =>
        new Builder(rootNamespace).Build(roots, modelHash, documents);

    private static bool IsNamespaceKind(string kind) => NamespaceKinds.Contains(kind.ToLowerInvariant());

    private static bool IsEnumeration(string kind) => string.Equals(kind, "enumDef", StringComparison.OrdinalIgnoreCase);

    private static string Unescaped(string identifier) => identifier.TrimStart('@');

    private sealed class Builder(string rootNamespace)
    {
        private readonly Dictionary<string, Symbol> symbols = new(StringComparer.Ordinal);
        private readonly List<(SymbolNode Node, IReadOnlyList<Symbol> Owners)> definitionNodes = [];
        private readonly List<Symbol> usages = [];
        private readonly Dictionary<string, DefinitionPlan> definitions = new(StringComparer.Ordinal);
        private readonly Dictionary<DefinitionPlan, IReadOnlySet<DefinitionPlan>> ancestors = [];
        private readonly Dictionary<string, IReadOnlySet<string>> redefined = new(StringComparer.Ordinal);
        private readonly Dictionary<DefinitionPlan, Dictionary<string, FeaturePlan>> effective = [];
        private readonly List<string> problems = [];

        internal GenerationPlan Build(IReadOnlyList<SymbolNode> roots, string modelHash, IReadOnlyList<string> documents)
        {
            foreach (var root in roots)
            {
                Walk(root, []);
            }

            CheckResolved();
            Name();
            Specialize();
            ThrowIfProblems();
            Type();
            Inherit();
            ThrowIfProblems();
            var ordered = definitions.Values
                .OrderBy(definition => definition.Namespace, StringComparer.Ordinal)
                .ThenBy(definition => definition.Id, StringComparer.Ordinal)
                .ToList();
            return new GenerationPlan(rootNamespace, modelHash, [.. documents.Order(StringComparer.Ordinal)], ordered, UsageTypes());
        }

        private void ThrowIfProblems()
        {
            if (problems.Count > 0)
            {
                throw new GenerationException(problems);
            }
        }

        private void Walk(SymbolNode node, IReadOnlyList<Symbol> owners)
        {
            var symbol = node.Symbol;
            var isRoot = symbol.Id.Length == 0;
            if (!isRoot)
            {
                symbols.TryAdd(symbol.Id, symbol);
                if (IsDefinitionKind(symbol.Kind))
                {
                    if (!IsEnumeration(symbol.Kind))
                    {
                        definitionNodes.Add((node, owners));
                    }
                }
                else if (!IsNamespaceKind(symbol.Kind))
                {
                    usages.Add(symbol);
                }
            }

            IReadOnlyList<Symbol> childOwners = isRoot ? owners : [.. owners, symbol];
            foreach (var child in node.Children)
            {
                Walk(child, childOwners);
            }
        }

        // A reference the service could not resolve names something outside the documents, which the stamp would not cover.
        private void CheckResolved()
        {
            foreach (var symbol in symbols.Values)
            {
                foreach (var edge in symbol.Specializations.Where(edge => edge.Declared.Length > 0 && edge.TargetId.Length == 0))
                {
                    problems.Add(Unresolved(symbol, edge.Declared));
                }

                if (symbol.TypeFacts is { Declared.Length: > 0, ResolvedId.Length: 0 } type)
                {
                    problems.Add(Unresolved(symbol, type.Declared));
                }
            }
        }

        private static string Unresolved(Symbol symbol, string declared) =>
            $"{symbol.Id} refers to {declared}, which does not resolve: the documents generated from must declare everything the model refers to outside the standard library";

        private void Name()
        {
            var types = new Dictionary<string, List<string>>(StringComparer.Ordinal);
            var namespaces = new Dictionary<string, SortedSet<string>>(StringComparer.Ordinal);
            var rootSegments = rootNamespace.Split('.');
            for (var i = 1; i <= rootSegments.Length; i++)
            {
                Declare(namespaces, string.Join('.', rootSegments[..i]), "--namespace");
            }

            Declare(types, $"{rootNamespace}.{RegistryName}", $"the {RegistryName} class");
            var packageNamespaces = new Dictionary<string, SortedSet<string>>(StringComparer.Ordinal);
            foreach (var (node, owners) in definitionNodes)
            {
                var symbol = node.Symbol;
                var namespaceSegments = new List<string>(rootSegments);
                var typeSegments = new List<string>();
                var named = true;
                foreach (var owner in owners)
                {
                    named &= owner.Name.Length > 0;
                    if (typeSegments.Count == 0 && IsNamespaceKind(owner.Kind))
                    {
                        namespaceSegments.Add(Identifiers.Escaped(Identifiers.TypeName(owner.Name)));
                        var owned = string.Join('.', namespaceSegments.Select(Unescaped));
                        Declare(namespaces, owned, owner.Id);
                        Declare(packageNamespaces, owned, owner.Id);
                    }
                    else
                    {
                        typeSegments.Add(Identifiers.TypeName(owner.Name));
                    }
                }

                if (!named || symbol.Name.Length == 0)
                {
                    problems.Add($"{(symbol.Id.Length > 0 ? symbol.Id : "a definition")} has no name a C# type can carry: it or an element owning it is unnamed");
                    continue;
                }

                var typeName = string.Join('_', typeSegments.Append(Identifiers.TypeName(symbol.Name)));
                var @namespace = string.Join('.', namespaceSegments);
                var plan = new DefinitionPlan(symbol, @namespace, Identifiers.Escaped(typeName), Identifiers.Escaped("I" + typeName));
                definitions.Add(symbol.Id, plan);
                var qualified = string.Join('.', namespaceSegments.Select(Unescaped));
                Declare(types, $"{qualified}.{typeName}", $"{symbol.Id} (class)");
                Declare(types, $"{qualified}.I{typeName}", $"{symbol.Id} (interface)");
                if (ClassMembers.Contains(typeName))
                {
                    problems.Add($"{symbol.Id} would generate the class {typeName}, which may not share the name of the member {typeName} every generated class declares");
                }
            }

            foreach (var (name, sources) in types.Where(entry => entry.Value.Count > 1))
            {
                problems.Add($"{string.Join(" and ", sources.Order(StringComparer.Ordinal))} would all generate the C# type {name}");
            }

            foreach (var (name, sources) in types.Where(entry => namespaces.ContainsKey(entry.Key)))
            {
                problems.Add($"{string.Join(" and ", sources)} would generate the C# type {name}, which is also a namespace, of {string.Join(" and ", namespaces[name])}");
            }

            foreach (var (name, packages) in packageNamespaces.Where(entry => entry.Value.Count > 1))
            {
                problems.Add($"the packages {string.Join(" and ", packages)} would all generate the C# namespace {name}");
            }
        }

        private static void Declare(Dictionary<string, List<string>> names, string name, string source)
        {
            if (!names.TryGetValue(name, out var sources))
            {
                names[name] = sources = [];
            }

            sources.Add(source);
        }

        private static void Declare(Dictionary<string, SortedSet<string>> names, string name, string source)
        {
            if (!names.TryGetValue(name, out var sources))
            {
                names[name] = sources = new SortedSet<string>(StringComparer.Ordinal);
            }

            sources.Add(source);
        }

        private void Specialize()
        {
            foreach (var definition in definitions.Values)
            {
                foreach (var edge in definition.Symbol.Specializations.Where(edge => GeneralizationKinds.Contains(edge.Kind) && edge.TargetId.Length > 0))
                {
                    if (definitions.TryGetValue(edge.TargetId, out var generated))
                    {
                        if (!definition.Bases.Contains(generated))
                        {
                            definition.Bases.Add(generated);
                        }
                    }
                    else if (!definition.UngeneratedBases.Any(listed => listed.TargetId == edge.TargetId))
                    {
                        definition.UngeneratedBases.Add(edge);
                    }
                }
            }

            foreach (var definition in definitions.Values)
            {
                if (Reaches(definition, definition))
                {
                    problems.Add($"{definition.Id} specializes itself, which no C# interface can");
                }
            }
        }

        private static bool Reaches(DefinitionPlan from, DefinitionPlan target)
        {
            var seen = new HashSet<DefinitionPlan>();
            var pending = new Stack<DefinitionPlan>(from.Bases);
            while (pending.TryPop(out var next))
            {
                if (next == target)
                {
                    return true;
                }

                if (seen.Add(next))
                {
                    foreach (var further in next.Bases)
                    {
                        pending.Push(further);
                    }
                }
            }

            return false;
        }

        private IReadOnlySet<DefinitionPlan> Ancestors(DefinitionPlan definition)
        {
            if (!ancestors.TryGetValue(definition, out var found))
            {
                var all = new HashSet<DefinitionPlan>();
                foreach (var direct in definition.Bases)
                {
                    all.Add(direct);
                    all.UnionWith(Ancestors(direct));
                }

                ancestors[definition] = found = all;
            }

            return found;
        }

        private void Type()
        {
            foreach (var (node, _) in definitionNodes)
            {
                if (!definitions.TryGetValue(node.Symbol.Id, out var definition))
                {
                    continue;
                }

                definition.UnnamedFeatures = node.Children.Count(child => IsFeatureKind(child.Symbol.Kind) && child.Symbol.Name.Length == 0);
                foreach (var child in node.Children.Where(child => IsFeatureKind(child.Symbol.Kind) && child.Symbol.Name.Length > 0).OrderBy(child => child.Symbol.Id, StringComparer.Ordinal))
                {
                    var member = Identifiers.MemberName(child.Symbol.Name);
                    if (member.Length == 0)
                    {
                        problems.Add($"{child.Symbol.Id} has no letter or digit a C# property name can carry");
                        continue;
                    }

                    var feature = new FeaturePlan(child.Symbol, definition, Identifiers.Escaped(member), ShapeOf(child.Symbol));
                    if (ReservedMembers.Contains(member))
                    {
                        problems.Add($"{feature.Id} would generate the property {member}, which every generated interface already has");
                    }

                    if (member == Unescaped(definition.InterfaceName))
                    {
                        problems.Add($"{feature.Id} would generate the property {member}, which may not share the name of its interface");
                    }

                    if (definition.Features.FirstOrDefault(other => Unescaped(other.Member) == member) is { } clash)
                    {
                        problems.Add($"{clash.Id} and {feature.Id} would both generate the property {Unescaped(definition.InterfaceName)}.{member}");
                    }

                    definition.Features.Add(feature);
                }
            }
        }

        private Shape ShapeOf(Symbol feature)
        {
            var (kind, definition, note) = ElementOf(ResolvedType(feature));
            var multiplicity = ResolvedMultiplicity(feature);
            return new Shape(kind, definition, CardinalityOf(multiplicity), note);
        }

        private (ElementKind Kind, DefinitionPlan? Definition, string? Note) ElementOf(TypeFacts? type)
        {
            if (type is null)
            {
                return (ElementKind.Value, null, "the feature is untyped");
            }

            // A value written with a unit is a magnitude and that unit, whatever scalar the magnitude reduces to.
            if (type.IsQuantity)
            {
                return type.Unit.Length > 0
                    ? (ElementKind.Quantity, null, $"its value is written in [{type.Unit}]")
                    : (ElementKind.Value, null, "a quantity value type whose value is written without a unit, so it may hold a number or a quantity");
            }

            // A definition reducing to a library scalar holds that scalar, even when it has a class of its own.
            if (Primitives.TryGetValue(type.Primitive, out var primitive))
            {
                return (primitive, null, null);
            }

            if (IsEnumeration(type.ResolvedKind))
            {
                return (ElementKind.EnumLiteral, null, null);
            }

            if (definitions.TryGetValue(type.ResolvedId, out var generated))
            {
                return (ElementKind.Generated, generated, null);
            }

            if (type.Primitive.Length > 0)
            {
                return (ElementKind.Value, null, $"SysML {type.Primitive} has no C# counterpart");
            }

            return type.ResolvedId.Length > 0
                ? (ElementKind.Value, null, $"its type {type.ResolvedId} has no generated class")
                : (ElementKind.Value, null, "the feature is untyped");
        }

        private static Cardinality CardinalityOf(Multiplicity? multiplicity)
        {
            if (multiplicity is null)
            {
                return Cardinality.Required;
            }

            if (!TryBound(multiplicity.Upper, out var upper) || upper > BigInteger.One)
            {
                return Cardinality.Many;
            }

            return TryBound(multiplicity.Lower, out var lower) && lower >= BigInteger.One
                ? Cardinality.Required
                : Cardinality.Optional;
        }

        private static bool TryBound(string text, out BigInteger bound) =>
            BigInteger.TryParse(text, NumberStyles.None, CultureInfo.InvariantCulture, out bound);

        private TypeFacts? ResolvedType(Symbol symbol)
        {
            var own = symbol.TypeFacts;
            if (NamesType(own))
            {
                return own;
            }

            var providers = Inherited(symbol, other => NamesType(other.TypeFacts));
            if (providers.Count == 0)
            {
                return own;
            }

            var choices = providers.Select(provider =>
            {
                var (kind, definition, note) = ElementOf(provider.TypeFacts);
                return (Type: provider.TypeFacts!, Shape: new Shape(kind, definition, Cardinality.Required, note));
            }).ToList();
            var inherited = choices.FirstOrDefault(candidate => choices.All(other => Reads(other.Shape, candidate.Shape))).Type;
            if (inherited is null)
            {
                problems.Add($"{symbol.Id} inherits incompatible types from {string.Join(" and ", providers.Select(provider => provider.Id))}");
                return own;
            }

            return own is null
                ? inherited
                : new TypeFacts(
                    inherited.Declared,
                    inherited.ResolvedId,
                    inherited.ResolvedKind,
                    inherited.Primitive,
                    inherited.PrimitiveSource,
                    own.IsQuantity || inherited.IsQuantity,
                    own.Unit.Length > 0 ? own.Unit : inherited.Unit);
        }

        private Multiplicity? ResolvedMultiplicity(Symbol symbol)
        {
            if (symbol.Multiplicity is { } own)
            {
                return own;
            }

            var providers = Inherited(symbol, other => other.Multiplicity is not null);
            if (providers.Count == 0)
            {
                return null;
            }

            BigInteger? lower = null;
            BigInteger? upper = null;
            foreach (var provider in providers)
            {
                var range = provider.Multiplicity!;
                if (TryBound(range.Lower, out var minimum))
                {
                    lower = lower is null ? minimum : BigInteger.Max(lower.Value, minimum);
                }

                if (TryBound(range.Upper, out var maximum))
                {
                    upper = upper is null ? maximum : BigInteger.Min(upper.Value, maximum);
                }
            }

            if (lower is not null && upper is not null && lower > upper)
            {
                problems.Add($"{symbol.Id} inherits disjoint multiplicities from {string.Join(" and ", providers.Select(provider => provider.Id))}");
            }

            var upperText = upper?.ToString(CultureInfo.InvariantCulture)
                ?? (providers.All(provider => provider.Multiplicity!.Upper == "*") ? "*" : string.Empty);
            return new Multiplicity(lower?.ToString(CultureInfo.InvariantCulture) ?? string.Empty, upperText);
        }

        private static bool NamesType(TypeFacts? type) => type is not null && (type.ResolvedId.Length > 0 || type.Declared.Length > 0);

        // Stop at the first declaration of a fact on each branch, keeping all branches' constraints.
        private List<Symbol> Inherited(Symbol symbol, Func<Symbol, bool> declares)
        {
            var found = new List<Symbol>();
            var visited = new HashSet<string>(StringComparer.Ordinal) { symbol.Id };
            var pending = new Stack<Specialization>(symbol.Specializations);
            while (pending.TryPop(out var edge))
            {
                if (!GeneralizationKinds.Contains(edge.Kind) || !symbols.TryGetValue(edge.TargetId, out var target) || !visited.Add(target.Id))
                {
                    continue;
                }

                if (declares(target))
                {
                    found.Add(target);
                    continue;
                }

                foreach (var further in target.Specializations)
                {
                    pending.Push(further);
                }
            }

            return [.. found.OrderBy(provider => provider.Id, StringComparer.Ordinal)];
        }

        // Every feature a feature redefines, directly or through what it redefines.
        private IReadOnlySet<string> Redefined(string featureId)
        {
            if (redefined.TryGetValue(featureId, out var found))
            {
                return found;
            }

            var all = new HashSet<string>(StringComparer.Ordinal);
            var pending = new Stack<string>([featureId]);
            while (pending.TryPop(out var next))
            {
                if (!symbols.TryGetValue(next, out var symbol))
                {
                    continue;
                }

                foreach (var edge in symbol.Specializations.Where(edge => edge.Kind == "redefines" && edge.TargetId.Length > 0))
                {
                    if (all.Add(edge.TargetId))
                    {
                        pending.Push(edge.TargetId);
                    }
                }
            }

            redefined[featureId] = all;
            return all;
        }

        private bool Redefines(FeaturePlan feature, FeaturePlan other) => Redefined(feature.Id).Contains(other.Id);

        private void Inherit()
        {
            foreach (var definition in definitions.Values)
            {
                var features = Inheritance(definition).SelectMany(owner => owner.Features).ToList();
                var slots = new Dictionary<string, FeaturePlan>(StringComparer.Ordinal);
                foreach (var feature in features)
                {
                    var candidates = features.Where(other => other == feature || Redefines(other, feature)).ToList();
                    var maximal = candidates.Where(candidate => !candidates.Any(other => other != candidate && Redefines(other, candidate))).ToList();
                    if (maximal.Count != 1)
                    {
                        problems.Add(maximal.Count == 0
                            ? $"{definition.Id} inherits a cyclic redefinition of {feature.Id}"
                            : $"{definition.Id} inherits {string.Join(" and ", maximal.Select(other => other.Id).Order(StringComparer.Ordinal))}, which all redefine {feature.Id} and none of which redefines another");
                        continue;
                    }

                    slots[feature.Id] = maximal[0];
                }

                foreach (var group in features.Where(feature => slots.GetValueOrDefault(feature.Id) == feature).GroupBy(feature => Unescaped(feature.Member), StringComparer.Ordinal).Where(group => group.Count() > 1))
                {
                    problems.Add($"{definition.Id} has the features {string.Join(" and ", group.Select(feature => feature.Id).Order(StringComparer.Ordinal))}, different features that neither redefines the other, which would both be read as the property {group.Key}");
                }

                foreach (var feature in definition.Features)
                {
                    feature.Hides = features.Any(other => other.Owner != definition && Unescaped(other.Member) == Unescaped(feature.Member));
                }

                effective[definition] = slots;
            }

            foreach (var definition in definitions.Values.Where(definition => effective[definition].Count == Inheritance(definition).Sum(owner => owner.Features.Count)))
            {
                Implement(definition);
            }
        }

        // The definition, then every generated definition it specializes, in ordinal order of their qualified names.
        private IEnumerable<DefinitionPlan> Inheritance(DefinitionPlan definition) =>
            Ancestors(definition).OrderBy(ancestor => ancestor.Id, StringComparer.Ordinal).Prepend(definition);

        private void Implement(DefinitionPlan definition)
        {
            var slots = effective[definition];
            var baseClass = definition.BaseClass;
            var inherited = baseClass is null ? new HashSet<DefinitionPlan>() : new HashSet<DefinitionPlan>(Ancestors(baseClass)) { baseClass };
            foreach (var owner in Inheritance(definition))
            {
                foreach (var feature in owner.Features)
                {
                    var slot = slots[feature.Id];
                    if (slot != feature && !Reads(feature.Shape, slot.Shape))
                    {
                        problems.Add($"{definition.Id}: {slot.Id} redefines {feature.Id} as {Describe(slot.Shape)}, which the property {Unescaped(owner.InterfaceName)}.{Unescaped(feature.Member)}, {Describe(feature.Shape)}, cannot soundly read");
                        continue;
                    }

                    // A property reads its slot by name with its own decoder, so an inherited implementation of the same slot name stands.
                    if (owner == definition || !inherited.Contains(owner) || effective[baseClass!].GetValueOrDefault(feature.Id)?.Symbol.Name != slot.Symbol.Name)
                    {
                        definition.Implementations.Add(new ImplementationPlan(feature, slot));
                    }
                }
            }
        }

        // Whether a property of one shape reads every value a redefining feature of another may hold.
        private bool Reads(Shape property, Shape slot)
        {
            var many = property.Cardinality switch
            {
                Cardinality.Many => true,
                Cardinality.Optional => slot.Cardinality != Cardinality.Many,
                _ => slot.Cardinality == Cardinality.Required,
            };
            var element = property.Kind == ElementKind.Value
                || (property.Kind == slot.Kind && (property.Kind != ElementKind.Generated || property.Definition == slot.Definition || Ancestors(slot.Definition!).Contains(property.Definition!)))
                || (property.Kind, slot.Kind) is (ElementKind.Real, ElementKind.Integer or ElementKind.Rational)
                    or (ElementKind.Rational, ElementKind.Integer)
                    or (ElementKind.Complex, ElementKind.Integer or ElementKind.Rational or ElementKind.Real);
            return many && element;
        }

        private static string Describe(Shape shape)
        {
            var element = shape.Kind == ElementKind.Generated ? shape.Definition!.Id : shape.Kind.ToString();
            return shape.Cardinality switch
            {
                Cardinality.Many => $"many {element}",
                Cardinality.Optional => $"an optional {element}",
                _ => $"one {element}",
            };
        }

        // The generated definition each usage is typed by: the service reports an instantiated usage's instance as of the usage.
        private List<KeyValuePair<string, DefinitionPlan>> UsageTypes()
        {
            var types = new List<KeyValuePair<string, DefinitionPlan>>();
            foreach (var usage in usages.Where(usage => !definitions.ContainsKey(usage.Id)).OrderBy(usage => usage.Id, StringComparer.Ordinal))
            {
                var typings = usage.Specializations.Where(edge => edge.Kind == "typing" && edge.TargetId.Length > 0).Select(edge => edge.TargetId).Distinct(StringComparer.Ordinal).ToList();
                var typeId = typings.Count switch
                {
                    0 => ResolvedType(usage)?.ResolvedId,
                    1 => typings[0],
                    _ => null,
                };
                if (typeId is not null && definitions.TryGetValue(typeId, out var definition))
                {
                    types.Add(new KeyValuePair<string, DefinitionPlan>(usage.Id, definition));
                }
            }

            return types;
        }
    }
}
