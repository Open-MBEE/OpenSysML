using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One element a query selected: its identity, its metamodel type, and the properties it reported. An OSLC query selecting
/// held objects (<c>rdf:type=elmt:&lt;id&gt;</c>) answers one per object, whose <see cref="Id"/> and <see cref="Type"/> name
/// the element it is an instance of and whose <see cref="Path"/> names the object.
/// </summary>
public sealed record QueryElement
{
    /// <summary>Creates an element.</summary>
    /// <param name="id">The element's qualified name, the identity the service reports.</param>
    /// <param name="type">The element's metamodel type (<c>PartUsage</c>).</param>
    /// <param name="properties">The selected properties the element has, by name; copied.</param>
    /// <param name="path">The path the session holds an object under; empty for a model element.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A name repeats, or a name or a value is null.</exception>
    internal QueryElement(string id, string type, IEnumerable<KeyValuePair<string, string>> properties, string path = "")
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(type);
        ArgumentNullException.ThrowIfNull(path);
        Id = id;
        Type = type;
        Properties = new ValueDictionary<string>(properties);
        Path = path;
    }

    /// <summary>The element's qualified name, which is what a <see cref="Query.Scope"/> names it by.</summary>
    public string Id { get; }

    /// <summary>The element's metamodel type (<c>PartUsage</c>, <c>PartDefinition</c>).</summary>
    public string Type { get; }

    /// <summary>
    /// The properties <see cref="Query.Select"/> asked for, by name, as the service writes them; a property the element
    /// does not have is absent rather than empty.
    /// </summary>
    public ValueDictionary<string> Properties { get; }

    /// <summary>The path the session holds an object under (<c>car.wheels[2]</c>, <c>#7</c>); empty for a model element.</summary>
    public string Path { get; }

    /// <summary>The element as <c>Demo::vehicle (PartUsage)</c>.</summary>
    /// <returns>The element's text.</returns>
    public override string ToString() => $"{Id} ({Type})";
}
