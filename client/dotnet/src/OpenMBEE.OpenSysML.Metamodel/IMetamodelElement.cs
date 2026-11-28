using System.Collections.Generic;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// What every element of a metamodel document carries besides its metaclass's properties: its
/// identity, the metaclass it is read as, an untyped read of its properties, and the keys its document
/// writes that no property has. Every generated root
/// metaclass interface, <see cref="IElement"/> among them, extends it, so no generated property may take
/// one of these names.
/// </summary>
public interface IMetamodelElement
{
    /// <summary>The element's <c>@id</c>.</summary>
    string Id { get; }

    /// <summary>The element's <c>@type</c>, as the document wrote it.</summary>
    string JsonType { get; }

    /// <summary>The metaclass the element is read as.</summary>
    MetaclassInfo MetaclassInfo { get; }

    /// <summary>
    /// Reads a property by its JSON key, untyped: null, a <see cref="bool"/>, <see cref="string"/>,
    /// <see cref="int"/>, <see cref="double"/>, a generated enum's member, an element, or for a many-valued
    /// property an <see cref="System.Collections.Generic.IReadOnlyList{T}"/> of them, empty when the document
    /// supplies null. It reads what the typed properties read, and also a reference outside the property's
    /// range that a document read with <see cref="ReadOptions.CheckRanges"/> off keeps, which no typed property
    /// can hold: as an <see cref="IMetamodelElement"/>, or a list of them.
    /// </summary>
    /// <param name="jsonKey">The key, for example <c>ownedFeature</c>.</param>
    /// <returns>The value.</returns>
    /// <exception cref="System.ArgumentNullException"><paramref name="jsonKey"/> is null.</exception>
    /// <exception cref="UnknownJsonKeyException">The element's metaclass has no property with that key.</exception>
    /// <exception cref="NotSuppliedException">The document does not supply the property.</exception>
    /// <exception cref="UnresolvedReferenceException">The document does not resolve a reference the property holds.</exception>
    /// <exception cref="MalformedValueException">The document's value does not fit the property, or, read with range checks on, refers outside its range.</exception>
    object? GetValue(string jsonKey);

    /// <summary>
    /// The members of the element's JSON object that its metaclass has no property for, <c>@id</c> and
    /// <c>@type</c> aside, by key and as written: an exporter's annotations, such as OpenSysML's
    /// <c>sysx:sourceText</c>, or the keys of a type the metamodel lacks beyond those of the metaclass it is
    /// read as. Empty when there are none. They are copies of the element's own, which no source document
    /// owns: the source may be disposed, and any number of threads may read them.
    /// </summary>
    /// <exception cref="System.InvalidOperationException">The document was read without <see cref="ReadOptions.KeepExtensions"/>.</exception>
    IReadOnlyDictionary<string, JsonElement> Extensions { get; }
}
