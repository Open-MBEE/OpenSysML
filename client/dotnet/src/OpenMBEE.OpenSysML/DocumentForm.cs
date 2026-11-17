namespace OpenMBEE.OpenSysML;

/// <summary>The form a document is rendered to, as the CLI's <c>-doc-form</c> names it. PDF needs the CLI's converter toolchain and is not offered by the service.</summary>
public enum DocumentForm
{
    /// <summary>Markdown, the default.</summary>
    Markdown,

    /// <summary>A standalone HTML page with the default stylesheet. Needs <see cref="Capabilities.RenderDocumentHtml"/>.</summary>
    Html,

    /// <summary>AsciiDoc. Needs <see cref="Capabilities.RenderDocumentAsciidoc"/>.</summary>
    AsciiDoc,
}
