using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Diagnostics.CodeAnalysis;
using System.IO;
using System.Linq;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// The elements of an exported JSON document, read as the generated metamodel types: SysML v2 API or
/// sysml-toolkit JSON, without a service. Its <see cref="Elements"/> are in document order and found by
/// <c>@id</c>; <see cref="All{T}"/> selects them by metamodel ancestry.
/// </summary>
/// <remarks>
/// A graph is complete when a <c>ReadJson</c> returns it, and immutable: every element is created, every value
/// converted and every reference among them resolved before it is returned, and it holds no JSON of the
/// source, which the caller may dispose. A document that is not an element graph fails the read with
/// <see cref="MalformedDocumentException"/>. A value that does not fit its property, or a reference the
/// document does not resolve, such as one to a standard-library element an export does not include, fails
/// only reads of that property, with <see cref="MalformedValueException"/> or
/// <see cref="UnresolvedReferenceException"/>, as an absent one does with <see cref="NotSuppliedException"/>.
/// Any number of threads may read it.
/// </remarks>
public sealed class MetamodelGraph
{
    private static readonly ReadOptions Defaults = new();

    private readonly ReadOnlyCollection<IMetamodelElement> elements;
    private readonly FrozenDictionary<string, IMetamodelElement> byId;

    internal MetamodelGraph(IMetamodelElement[] elements, IReadOnlyList<INamespace> roots, ReadOptions options)
    {
        this.elements = Array.AsReadOnly(elements);
        byId = elements.ToFrozenDictionary(element => element.Id, StringComparer.Ordinal);
        Roots = roots;
        CheckRanges = options.CheckRanges;
        KeepsExtensions = options.KeepExtensions;
    }

    /// <summary>Every element, in document order: a DataVersion's payload where the document holds one, none for a null payload.</summary>
    public IReadOnlyList<IMetamodelElement> Elements => elements;

    /// <summary>Whether the document was read with <see cref="ReadOptions.CheckRanges"/>.</summary>
    public bool CheckRanges { get; }

    /// <summary>Whether the document was read with <see cref="ReadOptions.KeepExtensions"/>, so every element's <see cref="IMetamodelElement.Extensions"/> reads.</summary>
    public bool KeepsExtensions { get; }

    /// <summary>
    /// The namespaces the document records no owner for, in document order: those with no non-null
    /// <c>owningRelationship</c>, <c>owner</c> or <c>owningNamespace</c> that no element's
    /// <c>ownedRelationship</c> or <c>ownedRelatedElement</c> refers to.
    /// </summary>
    public IReadOnlyList<INamespace> Roots { get; }

    /// <summary>The element with an <c>@id</c>.</summary>
    /// <param name="id">The <c>@id</c>.</param>
    /// <returns>The element.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="id"/> is null.</exception>
    /// <exception cref="KeyNotFoundException">The document holds no element with that <c>@id</c>.</exception>
    public IMetamodelElement this[string id] =>
        TryGet(id, out var element) ? element : throw new KeyNotFoundException($"the document holds no element with @id \"{id}\"");

    /// <summary>Reads a JSON document from a file, with the default <see cref="ReadOptions"/>.</summary>
    /// <param name="path">The file: a JSON document as <see cref="ReadJson(JsonDocument)"/> reads one, in UTF-8. A string is always a path, never JSON text.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="IOException">The file cannot be read.</exception>
    /// <exception cref="MalformedDocumentException">The file is not JSON, or not an element graph.</exception>
    public static MetamodelGraph ReadJson(string path) => ReadJson(path, Defaults);

    /// <summary>Reads a JSON document from a file.</summary>
    /// <param name="path">The file: a JSON document as <see cref="ReadJson(JsonDocument)"/> reads one, in UTF-8. A string is always a path, never JSON text.</param>
    /// <param name="options">How to read it.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><see cref="ReadOptions.Supertypes"/> maps one type to two supertypes.</exception>
    /// <exception cref="IOException">The file cannot be read.</exception>
    /// <exception cref="MalformedDocumentException">The file is not JSON, or not an element graph.</exception>
    public static MetamodelGraph ReadJson(string path, ReadOptions options)
    {
        ArgumentNullException.ThrowIfNull(path);
        ArgumentNullException.ThrowIfNull(options);
        using var stream = File.OpenRead(path);
        return Parse(stream, options, $"{path} is not a JSON document");
    }

    /// <summary>Reads a JSON document from a stream, to its end, with the default <see cref="ReadOptions"/>. The stream stays the caller's: it is not disposed.</summary>
    /// <param name="stream">A JSON document as <see cref="ReadJson(JsonDocument)"/> reads one, in UTF-8.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="stream"/> is null.</exception>
    /// <exception cref="MalformedDocumentException">The stream is not JSON, or not an element graph.</exception>
    public static MetamodelGraph ReadJson(Stream stream) => ReadJson(stream, Defaults);

    /// <summary>Reads a JSON document from a stream, to its end. The stream stays the caller's: it is not disposed.</summary>
    /// <param name="stream">A JSON document as <see cref="ReadJson(JsonDocument)"/> reads one, in UTF-8.</param>
    /// <param name="options">How to read it.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><see cref="ReadOptions.Supertypes"/> maps one type to two supertypes.</exception>
    /// <exception cref="MalformedDocumentException">The stream is not JSON, or not an element graph.</exception>
    public static MetamodelGraph ReadJson(Stream stream, ReadOptions options)
    {
        ArgumentNullException.ThrowIfNull(stream);
        ArgumentNullException.ThrowIfNull(options);
        return Parse(stream, options, "the stream is not a JSON document");
    }

    /// <summary>Reads a parsed JSON document, as <see cref="ReadJson(JsonDocument, ReadOptions)"/> does, with the default <see cref="ReadOptions"/>.</summary>
    /// <param name="document">The document. It stays the caller's: it is not disposed, and the graph holds none of its JSON.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="document"/> is null.</exception>
    /// <exception cref="MalformedDocumentException">The document is not an element graph.</exception>
    public static MetamodelGraph ReadJson(JsonDocument document) => ReadJson(document, Defaults);

    /// <summary>
    /// Reads a parsed JSON document: an element object, an array of element objects, a SysML v2 API
    /// DataVersion (<c>identity</c> and <c>payload</c>, a null payload holding no element) or an array of
    /// them, or a Commit whose <c>change</c> array holds DataVersions. The document stays the caller's: it is
    /// not disposed, and the graph holds none of its JSON.
    /// </summary>
    /// <param name="document">The document.</param>
    /// <param name="options">How to read it.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><see cref="ReadOptions.Supertypes"/> maps one type to two supertypes.</exception>
    /// <exception cref="MalformedDocumentException">The document is not an element graph.</exception>
    public static MetamodelGraph ReadJson(JsonDocument document, ReadOptions options)
    {
        ArgumentNullException.ThrowIfNull(document);
        ArgumentNullException.ThrowIfNull(options);
        return GraphReader.Read(document.RootElement, options);
    }

    /// <summary>Reads elements, as <see cref="ReadJson(IEnumerable{JsonElement}, ReadOptions)"/> does, with the default <see cref="ReadOptions"/>.</summary>
    /// <param name="elements">The elements, in document order.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="elements"/> is null.</exception>
    /// <exception cref="MalformedDocumentException">The elements are not an element graph.</exception>
    public static MetamodelGraph ReadJson(IEnumerable<JsonElement> elements) => ReadJson(elements, Defaults);

    /// <summary>
    /// Reads elements: each an element object or a SysML v2 API DataVersion holding one, as an array of a
    /// document holds them. The graph holds none of their JSON.
    /// </summary>
    /// <param name="elements">The elements, in document order.</param>
    /// <param name="options">How to read them.</param>
    /// <returns>The graph.</returns>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException"><see cref="ReadOptions.Supertypes"/> maps one type to two supertypes.</exception>
    /// <exception cref="MalformedDocumentException">The elements are not an element graph.</exception>
    public static MetamodelGraph ReadJson(IEnumerable<JsonElement> elements, ReadOptions options)
    {
        ArgumentNullException.ThrowIfNull(elements);
        ArgumentNullException.ThrowIfNull(options);
        return GraphReader.Read(elements, options);
    }

    /// <summary>Finds the element with an <c>@id</c>.</summary>
    /// <param name="id">The <c>@id</c>.</param>
    /// <param name="element">The element, when the document holds one with that <c>@id</c>.</param>
    /// <returns>Whether it does.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="id"/> is null.</exception>
    public bool TryGet(string id, [NotNullWhen(true)] out IMetamodelElement? element)
    {
        ArgumentNullException.ThrowIfNull(id);
        return byId.TryGetValue(id, out element);
    }

    /// <summary>Whether the document holds an element with an <c>@id</c>.</summary>
    /// <param name="id">The <c>@id</c>.</param>
    /// <returns>Whether it does.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="id"/> is null.</exception>
    public bool Contains(string id)
    {
        ArgumentNullException.ThrowIfNull(id);
        return byId.ContainsKey(id);
    }

    /// <summary>The elements that are a <typeparamref name="T"/>, in document order: <c>All&lt;IFeature&gt;()</c> holds every part usage.</summary>
    /// <typeparam name="T">A generated metaclass interface, for example <c>IPartUsage</c>, or <see cref="IMetamodelElement"/> for every element.</typeparam>
    /// <returns>An immutable list of them.</returns>
    public IReadOnlyList<T> All<T>()
        where T : IMetamodelElement => Array.AsReadOnly(elements.OfType<T>().ToArray());

    private static MetamodelGraph Parse(Stream stream, ReadOptions options, string notJson)
    {
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(stream);
        }
        catch (JsonException e)
        {
            throw new MalformedDocumentException($"{notJson}: {e.Message}", e);
        }

        using (document)
        {
            return GraphReader.Read(document.RootElement, options);
        }
    }
}
