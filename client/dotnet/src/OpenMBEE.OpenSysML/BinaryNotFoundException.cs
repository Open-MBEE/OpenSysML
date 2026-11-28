using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// No <c>sysml-grpc</c> binary could be found to start as a private service.
/// <see cref="SearchedLocations"/> lists every place looked at, in the order they were looked at.
/// </summary>
public class BinaryNotFoundException : ServiceStartException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">What was looked for and how to provide it.</param>
    /// <param name="searchedLocations">Every place looked at, in order, each saying what was found there.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A location is null.</exception>
    public BinaryNotFoundException(string message, IEnumerable<string> searchedLocations)
        : this(message, searchedLocations, null)
    {
    }

    /// <summary>Creates the exception with the failure that left nothing to start, such as a release that could not be downloaded.</summary>
    /// <param name="message">What was looked for and how to provide it.</param>
    /// <param name="searchedLocations">Every place looked at, in order, each saying what was found there.</param>
    /// <param name="innerException">The failure that left nothing to start; null when there was none.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A location is null.</exception>
    public BinaryNotFoundException(string message, IEnumerable<string> searchedLocations, Exception? innerException)
        : base(message, innerException)
    {
        SearchedLocations = new ValueList<string>(searchedLocations ?? throw new ArgumentNullException(nameof(searchedLocations)));
    }

    /// <summary>Every place looked at, in the order they were looked at, each saying what was found there.</summary>
    public ValueList<string> SearchedLocations { get; }
}
