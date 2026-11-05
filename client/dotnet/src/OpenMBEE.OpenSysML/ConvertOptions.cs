using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a model is converted, as <c>sysml -convert</c> converts one. Collections are copied when they are set. Which
/// directions an option applies to is the service's to judge: it refuses one given where it does not apply with
/// <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class ConvertOptions
{
    private readonly ValueList<string> documents;
    private readonly ValueList<string> keepDerived;

    /// <summary>The format inferred, notation written as it was parsed, every document, and the standard element array.</summary>
    public static ConvertOptions Default { get; } = new();

    /// <summary>
    /// The format the source is in, named as <see cref="ConversionFormat"/> names one; null infers it from a file's
    /// extension, and reads a parsed model as the notation it is. Inline content must name it. A SysML v1 format is
    /// refused by the service, pointing at <see cref="Connection.MigrateAsync"/>.
    /// </summary>
    public string? FromFormat { get; init; }

    /// <summary>
    /// Writes notation back out even when the parser could not read all of it, reporting its syntax errors as
    /// <see cref="Conversion.Diagnostics"/>. Notation to notation only.
    /// </summary>
    public bool TolerateSyntaxErrors { get; init; }

    /// <summary>
    /// How derived element ids are spelled when notation is written as <c>ttl</c> or <c>api-json</c>; null for the
    /// service's default, <see cref="OpenSysML.IdForm.Qualified"/>. Declared and normative ids are unchanged.
    /// </summary>
    public IdForm? IdForm { get; init; }

    /// <summary>
    /// For a parsed model, the documents whose elements are written, named as the parse named them; the model's other
    /// documents are read for the references into them. Empty writes every document. Naming one needs
    /// <see cref="Capabilities.ConvertDocuments"/>.
    /// </summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Documents
    {
        get => documents;
        init => documents = new ValueList<string>(value);
    }

    /// <summary>
    /// For <c>api-json</c> from notation, writes the compact document (<c>api-json-compact/1</c>) in place of the
    /// standard element array. Needs <see cref="Capabilities.ConvertCompact"/>.
    /// </summary>
    public bool Compact { get; init; }

    /// <summary>
    /// With <see cref="Compact"/>, leaves out every derived property of the metamodel except those in
    /// <see cref="KeepDerived"/>. Needs <see cref="Capabilities.ConvertCompact"/>.
    /// </summary>
    public bool OmitDerived { get; init; }

    /// <summary>
    /// With <see cref="OmitDerived"/>, the derived properties still written, named as in the element form
    /// (<c>owner</c>, <c>qualifiedName</c>). Naming one needs <see cref="Capabilities.ConvertCompact"/>.
    /// </summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> KeepDerived
    {
        get => keepDerived;
        init => keepDerived = new ValueList<string>(value);
    }

    /// <summary>
    /// Reports the semantic check's diagnostics alongside the output, each with its own severity and code, as a parse
    /// reports them; the conversion itself stands or fails as it would without it. Notation sources only. Needs
    /// <see cref="Capabilities.ConvertCheckDiagnostics"/>.
    /// </summary>
    public bool IncludeCheckDiagnostics { get; init; }

    /// <summary>
    /// For <c>api-json</c>, also writes the ownerless library stubs conversions wrote before 0.11.0 as roots of the
    /// element array, for a reader that still expects them; by default a library element is a bare reference, listed
    /// in <see cref="Conversion.LibraryReferences"/>. The service refuses it for any other target format. Needs
    /// <see cref="Capabilities.ConvertLibraryStubs"/>.
    /// </summary>
    public bool IncludeLibraryStubs { get; init; }

    /// <summary>Whether any of the compact form's options is asked for, which the service gates as one.</summary>
    internal bool AsksCompact => Compact || OmitDerived || !keepDerived.IsEmpty;
}
