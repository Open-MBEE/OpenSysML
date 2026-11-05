using System;
using System.Collections.Frozen;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Typed;

/// <summary>
/// The generated classes of one generated file, by the qualified name of the definition each was generated
/// from, and the usages whose instances they view. Generated code builds one, and resolves every typed view
/// through it, so a view of an instance is of the most specific generated class that describes it.
/// </summary>
public sealed class TypedRegistry
{
    private readonly FrozenDictionary<string, TypedFactory> classes;
    private readonly FrozenDictionary<string, string> usageTypes;

    /// <summary>Creates a registry.</summary>
    /// <param name="classes">Each generated class's factory, by the qualified name of the definition it was generated from.</param>
    /// <param name="usageTypes">
    /// The definition each usage is typed by, by the usage's qualified name: the service reports the instance
    /// of an instantiated usage as of the usage, which these relate to the definition's generated class.
    /// </param>
    /// <exception cref="ArgumentNullException">An argument, name or factory is null.</exception>
    /// <exception cref="ArgumentException">A name repeats, a usage is also a definition, or a usage is typed by a definition with no class.</exception>
    public TypedRegistry(IEnumerable<KeyValuePair<string, TypedFactory>> classes, IEnumerable<KeyValuePair<string, string>> usageTypes)
    {
        ArgumentNullException.ThrowIfNull(classes);
        ArgumentNullException.ThrowIfNull(usageTypes);
        var byDefinition = new Dictionary<string, TypedFactory>(StringComparer.Ordinal);
        foreach (var (definitionId, factory) in classes)
        {
            ArgumentNullException.ThrowIfNull(definitionId, nameof(classes));
            ArgumentNullException.ThrowIfNull(factory, nameof(classes));
            if (!byDefinition.TryAdd(definitionId, factory))
            {
                throw new ArgumentException($"{definitionId} has two generated classes.", nameof(classes));
            }
        }

        var byUsage = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var (usageId, definitionId) in usageTypes)
        {
            ArgumentNullException.ThrowIfNull(usageId, nameof(usageTypes));
            ArgumentNullException.ThrowIfNull(definitionId, nameof(usageTypes));
            if (byDefinition.ContainsKey(usageId))
            {
                throw new ArgumentException($"{usageId} is both a definition and a usage.", nameof(usageTypes));
            }

            if (!byDefinition.ContainsKey(definitionId))
            {
                throw new ArgumentException($"The usage {usageId} is typed by {definitionId}, which has no generated class.", nameof(usageTypes));
            }

            if (!byUsage.TryAdd(usageId, definitionId))
            {
                throw new ArgumentException($"The usage {usageId} is typed twice.", nameof(usageTypes));
            }
        }

        this.classes = byDefinition.ToFrozenDictionary(StringComparer.Ordinal);
        this.usageTypes = byUsage.ToFrozenDictionary(StringComparer.Ordinal);
    }

    /// <summary>The qualified names of the definitions that have a generated class.</summary>
    public IReadOnlyCollection<string> DefinitionIds => classes.Keys;

    /// <summary>
    /// The definition whose generated class views an instance of a type: the type itself when it is such a
    /// definition, the definition a usage is typed by, or null when no generated class describes it.
    /// </summary>
    /// <param name="typeSymbolId">The qualified name an instance reports as its type.</param>
    /// <returns>The definition's qualified name, or null.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="typeSymbolId"/> is null.</exception>
    public string? DefinitionOf(string typeSymbolId)
    {
        ArgumentNullException.ThrowIfNull(typeSymbolId);
        return classes.ContainsKey(typeSymbolId) ? typeSymbolId : usageTypes.GetValueOrDefault(typeSymbolId);
    }

    /// <summary>
    /// A typed view of an instance, as a generated interface: of the generated class of the instance's own
    /// type, which a specialization of the requested definition implements too; or, when no generated class
    /// describes the instance's type, of the requested definition's class.
    /// </summary>
    /// <typeparam name="T">The generated interface of the requested definition.</typeparam>
    /// <param name="instance">The instance to view.</param>
    /// <param name="instantiation">The instantiation the instance was built by; null when there is none.</param>
    /// <param name="definitionId">The qualified name of the requested definition.</param>
    /// <returns>The view.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="instance"/> or <paramref name="definitionId"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="definitionId"/> has no generated class here, or its class does not implement <typeparamref name="T"/>.</exception>
    /// <exception cref="InstanceTypeException">Another generated class, not a specialization of the requested one, describes the instance's type.</exception>
    public T FromInstance<T>(Instance instance, Instantiation? instantiation, string definitionId)
        where T : class, ITypedObject
    {
        ArgumentNullException.ThrowIfNull(instance);
        var requested = Factory(definitionId);
        var actual = DefinitionOf(instance.TypeSymbolId);
        var view = (actual is null ? requested : classes[actual])(instance, instantiation);
        if (view is T typed)
        {
            return typed;
        }

        return actual is null || actual == definitionId
            ? throw new ArgumentException($"The generated class of {definitionId} does not implement {typeof(T).Name}.", nameof(definitionId))
            : throw new InstanceTypeException(definitionId, instance.TypeSymbolId);
    }

    /// <summary>The decoder of a feature that holds objects of a generated definition: each a typed view, resolved through the instantiation.</summary>
    /// <typeparam name="T">The generated interface of the definition.</typeparam>
    /// <param name="definitionId">The definition's qualified name.</param>
    /// <returns>The decoder.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="definitionId"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="definitionId"/> has no generated class here.</exception>
    public FeatureDecoder<T> Decoder<T>(string definitionId)
        where T : class, ITypedObject
    {
        Factory(definitionId);
        return (featureName, value, instantiation) =>
        {
            if (value is Value.InstanceRefValue reference && instantiation?.FindInstance(reference.InstanceId) is { } nested)
            {
                return FromInstance<T>(nested, instantiation, definitionId);
            }

            throw new TypeMismatchException(featureName, $"an object of {definitionId} built by the instantiation", value);
        };
    }

    private TypedFactory Factory(string definitionId)
    {
        ArgumentNullException.ThrowIfNull(definitionId);
        return classes.TryGetValue(definitionId, out var factory)
            ? factory
            : throw new ArgumentException($"{definitionId} has no generated class in this registry.", nameof(definitionId));
    }
}
