using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Every root-relative path that reaches one object the model's population holds, aliases through connector ends and
/// reference usages included, as <see cref="Model.InstancePathsAsync"/> answers them.
/// </summary>
public sealed record InstancePaths
{
    /// <summary>Creates the answer, copying the paths.</summary>
    /// <param name="paths">The paths, in traversal order.</param>
    /// <param name="isTruncated">Whether the service bounded its search short of every path.</param>
    /// <exception cref="ArgumentNullException"><paramref name="paths"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="paths"/> holds a null.</exception>
    public InstancePaths(IEnumerable<InstancePath> paths, bool isTruncated)
    {
        Paths = new ValueList<InstancePath>(paths);
        IsTruncated = isTruncated;
    }

    /// <summary>
    /// The paths in traversal order: the roots the population holds in id order and, from each, depth-first over the
    /// features of an object in name order and the values of a feature in their order. A path visits no object twice
    /// and steps through the features the model declares only; an object no root reaches has none.
    /// </summary>
    public ValueList<InstancePath> Paths { get; }

    /// <summary>Whether the service bounded its search short of every path: more reach the object than it lists, or one is deeper than it walks.</summary>
    public bool IsTruncated { get; }
}
