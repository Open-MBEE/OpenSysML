using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A lookup that required a symbol found none: <see cref="Model.SymbolAsync"/>, when the service
/// reports no symbol of that name, and <see cref="Model.LookupAsync"/>, which also names the declared
/// names close enough to be typos of it.
/// </summary>
public class SymbolNotFoundException : ModelException
{
    /// <summary>Creates the exception for a lookup that found no symbol, naming the declared names close to it.</summary>
    /// <param name="name">The name looked up.</param>
    /// <param name="suggestions">Declared names close enough to be typos of it, best first.</param>
    /// <param name="method">The method whose answer reported no symbol, as <c>sysml.SysMLService/GetSymbol</c>.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A suggestion is null.</exception>
    public SymbolNotFoundException(string name, IEnumerable<string> suggestions, string method)
        : this(name, new ValueList<string>(suggestions ?? throw new ArgumentNullException(nameof(suggestions))), method)
    {
    }

    /// <summary>Creates the exception for a symbol the service reported it could not find, in its own words.</summary>
    /// <param name="name">The name looked up.</param>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported no symbol, as <c>sysml.SysMLService/GetSymbol</c>.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public SymbolNotFoundException(string name, string message, string method)
        : base(message ?? throw new ArgumentNullException(nameof(message)), method, [])
    {
        ArgumentNullException.ThrowIfNull(name);
        Name = name;
        Suggestions = default;
    }

    private SymbolNotFoundException(string name, ValueList<string> suggestions, string method)
        : base(Describe(name, suggestions), method, [])
    {
        Name = name;
        Suggestions = suggestions;
    }

    /// <summary>The name looked up.</summary>
    public string Name { get; }

    /// <summary>Declared names close enough to be typos of <see cref="Name"/>, best first; at most three.</summary>
    public ValueList<string> Suggestions { get; }

    private static string Describe(string name, ValueList<string> suggestions)
    {
        ArgumentNullException.ThrowIfNull(name);
        var message = $"No symbol named '{name}' in this model";
        return suggestions.IsEmpty
            ? message + "."
            : $"{message}; did you mean {string.Join(", ", suggestions.Select(suggestion => $"'{suggestion}'"))}?";
    }
}
