using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One element of a parsed model, as the service describes it: a snapshot, which a later edit to the
/// model does not change. Its facts are those the service reported; one it reported none of is
/// empty or null, never invented.
/// </summary>
public sealed record Symbol
{
    /// <summary>Creates a symbol, copying every collection.</summary>
    /// <param name="id">The fully qualified name, the identity the service reports.</param>
    /// <param name="name">The declared name; empty for an anonymous element.</param>
    /// <param name="kind">The element kind (<c>partDef</c>, <c>attributeUsage</c>, <c>RootNamespace</c>, …).</param>
    /// <param name="metadata">Further facts by name, as the service reports them.</param>
    /// <param name="childIds">The qualified names of the element's children.</param>
    /// <param name="attributes">The element's own attributes with their resolved values.</param>
    /// <param name="typeFacts">The static type facts; null when the element is not a definition or usage.</param>
    /// <param name="multiplicity">The declared multiplicity; null when none is declared.</param>
    /// <param name="specializations">Every generalization edge declared, in declaration order.</param>
    /// <param name="withheldLibraryAttributes">How many inherited standard-library attributes <paramref name="attributes"/> leaves out.</param>
    /// <exception cref="ArgumentNullException">A string or collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null, or a metadata name repeats.</exception>
    public Symbol(
        string id,
        string name,
        string kind,
        IEnumerable<KeyValuePair<string, string>> metadata,
        IEnumerable<string> childIds,
        IEnumerable<AttributeInfo> attributes,
        TypeFacts? typeFacts,
        Multiplicity? multiplicity,
        IEnumerable<Specialization> specializations,
        int withheldLibraryAttributes)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(kind);
        Id = id;
        Name = name;
        Kind = kind;
        Metadata = new ValueDictionary<string>(metadata);
        ChildIds = new ValueList<string>(childIds);
        Attributes = new ValueList<AttributeInfo>(attributes);
        TypeFacts = typeFacts;
        Multiplicity = multiplicity;
        Specializations = new ValueList<Specialization>(specializations);
        WithheldLibraryAttributes = withheldLibraryAttributes;
    }

    /// <summary>The fully qualified name, the identity the service reports.</summary>
    public string Id { get; }

    /// <summary>The declared name; empty for an anonymous element.</summary>
    public string Name { get; }

    /// <summary>The element kind (<c>partDef</c>, <c>attributeUsage</c>, <c>RootNamespace</c>, …).</summary>
    public string Kind { get; }

    /// <summary>
    /// Further facts by name, as the service reports them. <c>type</c> and <c>specializes</c> name only
    /// the first edge; <see cref="Specializations"/> has them all.
    /// </summary>
    public ValueDictionary<string> Metadata { get; }

    /// <summary>The qualified names of the element's children, in declaration order.</summary>
    public ValueList<string> ChildIds { get; }

    /// <summary>
    /// The element's own attributes with their resolved values, filled under the
    /// <see cref="Capabilities.SymbolAttributes"/> capability.
    /// </summary>
    public ValueList<AttributeInfo> Attributes { get; }

    /// <summary>
    /// The static type facts, filled under the <see cref="Capabilities.TypeFacts"/> capability; null when
    /// the element is not a definition or usage, or the service reported none.
    /// </summary>
    public TypeFacts? TypeFacts { get; }

    /// <summary>The declared multiplicity; null when none is declared or the service reported none.</summary>
    public Multiplicity? Multiplicity { get; }

    /// <summary>Every generalization edge declared, in declaration order.</summary>
    public ValueList<Specialization> Specializations { get; }

    /// <summary>
    /// How many attributes inherited from standard-library content <see cref="Attributes"/> leaves out,
    /// so that their absence is stated rather than silent.
    /// </summary>
    public int WithheldLibraryAttributes { get; }

    /// <summary>The symbol, as <c>Demo::Vehicle (partDef)</c>.</summary>
    /// <returns>The symbol's text.</returns>
    public override string ToString() => $"{(Id.Length == 0 ? Name : Id)} ({Kind})";
}
