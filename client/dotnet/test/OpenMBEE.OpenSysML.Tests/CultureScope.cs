using System;
using System.Globalization;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Runs the code it scopes under another current culture, and restores the one before.</summary>
internal sealed class CultureScope : IDisposable
{
    private readonly CultureInfo previous = CultureInfo.CurrentCulture;

    /// <summary>Makes a culture current.</summary>
    /// <param name="name">The culture's name, as <c>sv-SE</c>.</param>
    internal CultureScope(string name) => CultureInfo.CurrentCulture = new CultureInfo(name);

    public void Dispose() => CultureInfo.CurrentCulture = previous;
}
