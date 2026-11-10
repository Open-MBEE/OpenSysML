using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One element of a parsed model, as the service describes it: a snapshot, which a later edit to the
/// model does not change. Its facts are those the service reported; one it reported none of is
/// empty or null, never invented.
/// </summary>
/// <remarks>
/// Several facts summarise metamodel properties that <see cref="Model.ElementPropertiesAsync"/> derives in full; each
/// names the property it summarises, and that call is the surface whose answer the specification defines.
/// </remarks>
public sealed record Symbol
{
    /// <summary>Creates a symbol, copying every collection.</summary>
    /// <param name="id">The fully qualified name, the identity the service reports.</param>
    /// <param name="name">The declared name; empty for an anonymous element.</param>
    /// <param name="kind">The declaration keyword kind (<c>partDef</c>, <c>attributeUsage</c>, <c>RootNamespace</c>, …).</param>
    /// <param name="metadata">Further facts by name, as the service reports them.</param>
    /// <param name="childIds">The qualified names of the element's children.</param>
    /// <param name="attributes">The element's own attributes with their resolved values.</param>
    /// <param name="typeFacts">The static type facts; null when the element is not a definition or usage.</param>
    /// <param name="multiplicity">The declared multiplicity; null when none is declared.</param>
    /// <param name="specializations">Every generalization edge declared, in declaration order.</param>
    /// <param name="withheldLibraryAttributes">How many inherited standard-library attributes <paramref name="attributes"/> leaves out.</param>
    /// <param name="members">Every member, own then inherited; null or empty from a service that enumerates none.</param>
    /// <param name="withheldLibraryMembers">How many members inherited from standard-library content <paramref name="members"/> leaves out.</param>
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
        int withheldLibraryAttributes,
        IEnumerable<SymbolMember>? members = null,
        int withheldLibraryMembers = 0)
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
        Members = new ValueList<SymbolMember>(members ?? []);
        WithheldLibraryMembers = withheldLibraryMembers;
    }

    /// <summary>The fully qualified name, the identity the service reports.</summary>
    public string Id { get; }

    /// <summary>The declared name; empty for an anonymous element.</summary>
    public string Name { get; }

    /// <summary>
    /// The declaration keyword kind (<c>partDef</c>, <c>attributeUsage</c>, <c>RootNamespace</c>, …): the written
    /// syntax, not the element's metaclass, which <see cref="Model.ElementPropertiesAsync"/> reports with every
    /// property that classifies the element.
    /// </summary>
    public string Kind { get; }

    /// <summary>
    /// Further facts by name, as the service reports them, summarising what is written: <c>type</c> and
    /// <c>specializes</c> name only the first edge, which <see cref="Specializations"/> has all of, and
    /// <c>direction</c> the direction written on the feature, whose derived <c>Feature::direction</c> is
    /// <see cref="Model.ElementPropertiesAsync"/>' <c>direction</c>; beside them <c>multiplicity</c>, <c>abstract</c>, ….
    /// </summary>
    public ValueDictionary<string> Metadata { get; }

    /// <summary>
    /// The qualified names of the element's owned members, in declaration order; an unnamed member is listed under
    /// its positional id (<c>Demo::Pair::@2</c>). <see cref="Members"/> also lists the inherited ones.
    /// </summary>
    public ValueList<string> ChildIds { get; }

    /// <summary>
    /// The element's own attributes with their resolved values, filled under the
    /// <see cref="Capabilities.SymbolAttributes"/> capability.
    /// </summary>
    public ValueList<AttributeInfo> Attributes { get; }

    /// <summary>
    /// The static type facts, filled under the <see cref="Capabilities.TypeFacts"/> capability: the written typing and
    /// the derived <c>Feature::type</c>. Null when the element is not a definition or usage, or the service reported none.
    /// </summary>
    public TypeFacts? TypeFacts { get; }

    /// <summary>The declared multiplicity; null when none is declared or the service reported none.</summary>
    public Multiplicity? Multiplicity { get; }

    /// <summary>
    /// Every generalization edge written, in declaration order: what <see cref="Model.ElementPropertiesAsync"/>'
    /// <c>ownedSpecialization</c> reflects, without the implied ones the metamodel adds.
    /// </summary>
    public ValueList<Specialization> Specializations { get; }

    /// <summary>
    /// How many attributes inherited from standard-library content <see cref="Attributes"/> leaves out,
    /// so that their absence is stated rather than silent.
    /// </summary>
    public int WithheldLibraryAttributes { get; }

    /// <summary>
    /// Every member the element has (KerML <c>Type::member</c>), filled under the <see cref="Capabilities.SymbolMembers"/>
    /// capability: its owned members in declaration order, then the members it inherits in inheritance order, a
    /// redefinition masking what it redefines and a private member of a supertype not inherited. The members inherited
    /// from standard-library content are left out, and counted in <see cref="WithheldLibraryMembers"/>, unless
    /// <see cref="SymbolOptions.IncludeLibraryMembers"/> asked for them.
    /// </summary>
    public ValueList<SymbolMember> Members { get; }

    /// <summary>
    /// How many members inherited from standard-library content <see cref="Members"/> leaves out, so that their
    /// absence is stated rather than silent; 0 when <see cref="SymbolOptions.IncludeLibraryMembers"/> listed them.
    /// </summary>
    public int WithheldLibraryMembers { get; }

    /// <summary>The members the element declares itself: <see cref="Members"/> without the inherited ones, in declaration order.</summary>
    public ValueList<SymbolMember> OwnMembers => new(Members.Where(member => !member.IsInherited));

    /// <summary>The members the element inherits through specialization and typing, in inheritance order, each naming the type declaring it.</summary>
    public ValueList<SymbolMember> InheritedMembers => new(Members.Where(member => member.IsInherited));

    /// <summary>The symbol, as <c>Demo::Vehicle (partDef)</c>.</summary>
    /// <returns>The symbol's text.</returns>
    public override string ToString() => $"{(Id.Length == 0 ? Name : Id)} ({Kind})";
}
