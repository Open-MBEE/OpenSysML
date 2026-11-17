using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.InstantiateAsync"/> built: the object asked for, and every object reachable
/// from it, so a <see cref="Value.InstanceRefValue"/> in its feature values resolves without a further call.
/// </summary>
public sealed record Instantiation
{
    /// <summary>Creates an instantiation, copying its collections.</summary>
    /// <param name="root">The object of the symbol that was instantiated.</param>
    /// <param name="reachable">Every object built, the root among them, in the order the service reported.</param>
    /// <param name="diagnostics">What the service reported while building it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Instantiation(Instance root, IEnumerable<Instance> reachable, IEnumerable<Diagnostic> diagnostics)
    {
        ArgumentNullException.ThrowIfNull(root);
        Root = root;
        Reachable = new ValueList<Instance>(reachable);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>The object of the symbol that was instantiated.</summary>
    public Instance Root { get; }

    /// <summary>Every object built, the root among them, in the order the service reported.</summary>
    public ValueList<Instance> Reachable { get; }

    /// <summary>What the service reported while building it.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among the objects built.</returns>
    public Instance? FindInstance(long instanceId)
    {
        if (Root.Id == instanceId)
        {
            return Root;
        }

        foreach (var instance in Reachable)
        {
            if (instance.Id == instanceId)
            {
                return instance;
            }
        }

        return null;
    }

    /// <summary>
    /// The object holding another along the path the traversal reached it by: the <see cref="InstanceReach.InstanceId"/>
    /// of its <see cref="Instance.ReachedBy"/>, as an object.
    /// </summary>
    /// <param name="instance">An object of this instantiation.</param>
    /// <returns>The holder; null for the root, for an object the service did not say how it reached, and for a holder not among the objects built.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="instance"/> is null.</exception>
    public Instance? FindOwner(Instance instance)
    {
        ArgumentNullException.ThrowIfNull(instance);
        return instance.ReachedBy is { } reach ? FindInstance(reach.InstanceId) : null;
    }

    /// <summary>The object a reference refers to.</summary>
    /// <param name="reference">A reference among this instantiation's feature values.</param>
    /// <returns>The object; null when it is not among the objects built.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
    public Instance? Resolve(Value.InstanceRefValue reference)
    {
        ArgumentNullException.ThrowIfNull(reference);
        return FindInstance(reference.InstanceId);
    }
}
