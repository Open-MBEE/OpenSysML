using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A stored observable <see cref="Model.CompareResultsAsync"/> compares, with the feature of the run that answers it
/// (<c>sysml -observe stored=feature</c>).
/// </summary>
public sealed record ObservablePair
{
    private readonly string stored = string.Empty;

    /// <summary>Creates a pair.</summary>
    /// <param name="stored">The observable the tool stored, as its results name it.</param>
    /// <param name="feature">The feature of the run that answers it; null for the target's feature of the observable's own name.</param>
    /// <exception cref="ArgumentNullException"><paramref name="stored"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="stored"/> is empty or white space.</exception>
    public ObservablePair(string stored, string? feature = null)
    {
        Stored = stored;
        Feature = feature;
    }

    /// <summary>The observable the tool stored, as its results name it.</summary>
    /// <exception cref="ArgumentNullException">The name is null.</exception>
    /// <exception cref="ArgumentException">The name is empty or white space, which names no observable.</exception>
    public string Stored
    {
        get => stored;
        init
        {
            ArgumentException.ThrowIfNullOrWhiteSpace(value, nameof(Stored));
            stored = value;
        }
    }

    /// <summary>
    /// The feature of the run that answers the observable, as <c>target.total</c>; null for the target's feature of the
    /// observable's own name. Empty is refused before sending, since the service reads it as none.
    /// </summary>
    public string? Feature { get; init; }
}
