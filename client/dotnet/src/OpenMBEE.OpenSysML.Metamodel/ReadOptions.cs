using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>How <see cref="MetamodelGraph.ReadJson(System.IO.Stream, ReadOptions)"/> and its overloads read a document.</summary>
public sealed class ReadOptions
{
    /// <summary>
    /// Whether a reference to an element outside its property's metaclass is malformed, so that every read of
    /// the property throws <see cref="MalformedValueException"/>; true by default. Off, such references are
    /// kept: the property's typed read, which cannot hold them, still throws, and
    /// <see cref="IMetamodelElement.GetValue"/> returns the elements as written. Only range checks are off:
    /// every other value that does not fit its property is still malformed. Some exports write derived values
    /// outside their range, sysml-toolkit's <c>full-json</c> setting a requirement definition's <c>result</c>
    /// to the definition itself, and OpenSysML's own <c>api-json</c> a constructor expression's <c>function</c>
    /// to the attribute definition it constructs, for example.
    /// </summary>
    public bool CheckRanges { get; init; } = true;

    /// <summary>
    /// The supertype of each <c>@type</c> the metamodel lacks, for example <c>vendor:SpecialPart</c> to
    /// <c>PartUsage</c>: an element is read as the first metaclass along its <c>@type</c>'s chain of
    /// supertypes, and as <see cref="IElement"/> when the chain ends without one. Keys and values are matched as
    /// a document's types are, a <c>sysml:</c> or SysML IRI prefix removed. Null maps nothing.
    /// </summary>
    public IReadOnlyDictionary<string, string>? Supertypes { get; init; }

    /// <summary>
    /// Whether every element keeps, in <see cref="IMetamodelElement.Extensions"/>, the members of its JSON
    /// object that its metaclass has no property for, such as OpenSysML's <c>sysx:</c> annotations; false by
    /// default. Kept, they are copied as written, one JSON object per element that has any, which costs memory in
    /// proportion to their size: OpenSysML's <c>sysx:sourceText</c> holds an element's source text. Not kept,
    /// reading <see cref="IMetamodelElement.Extensions"/> throws.
    /// </summary>
    public bool KeepExtensions { get; init; }
}
