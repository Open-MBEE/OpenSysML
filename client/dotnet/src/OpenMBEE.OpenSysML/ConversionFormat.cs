using System;
using System.IO;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The names of the formats the service reads and writes, as <c>sysml -from</c>, <c>-convert</c> and <c>-migrate</c> name
/// them. The service also takes the aliases <c>text</c> for notation, <c>turtle</c> and <c>rdf</c> for Turtle, and
/// <c>json</c> for the API's JSON element form, in any case, and answers each with its canonical name.
/// </summary>
public static class ConversionFormat
{
    /// <summary>SysML v2 textual notation, the form a model is parsed from.</summary>
    public const string SysML = "sysml";

    /// <summary>KerML textual notation, written by the same writer as <see cref="SysML"/>, which the service answers it as.</summary>
    public const string KerML = "kerml";

    /// <summary>RDF in Turtle syntax. The mapping is experimental.</summary>
    public const string Turtle = "ttl";

    /// <summary>The SysML v2 API's JSON element form, over the same experimental RDF mapping.</summary>
    public const string ApiJson = "api-json";

    /// <summary>SysML v1 as UML XMI 2.5.1 with the SysML profile applied: read by a migration, never written.</summary>
    public const string Xmi = "xmi";

    /// <summary>An Eclipse UML2 file, read as <see cref="Xmi"/>.</summary>
    public const string Uml = "uml";

    /// <summary>A Cameo/MagicDraw archive, read as <see cref="Xmi"/>.</summary>
    public const string Mdzip = "mdzip";

    /// <summary>
    /// The format a file is written in, as its extension names it, as <c>sysml -convert</c> and <c>%save</c> read one:
    /// <c>.sysml</c> or <c>.kerml</c> notation, <c>.ttl</c> or <c>.turtle</c> Turtle, <c>.json</c> the API's JSON
    /// element form, in any case.
    /// </summary>
    /// <param name="path">The file's name or path.</param>
    /// <returns>The format; null when the extension names none that is written.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    public static string? OfPath(string path)
    {
        ArgumentNullException.ThrowIfNull(path);
        return Path.GetExtension(path).ToUpperInvariant() switch
        {
            ".SYSML" => SysML,
            ".KERML" => KerML,
            ".TTL" or ".TURTLE" => Turtle,
            ".JSON" => ApiJson,
            _ => null,
        };
    }
}
