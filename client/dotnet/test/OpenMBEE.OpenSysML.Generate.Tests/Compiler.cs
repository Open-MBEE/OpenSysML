using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// Compiles a generated file in memory against the client library, as a consumer that requires a documentation
/// comment on every public member would. A quick check of cases no committed golden holds; the goldens are
/// compiled for real by the consumer project's build.
/// </summary>
internal static class Compiler
{
    /// <summary>Every warning and error compiling the file reports, as <c>CS0108: message</c>.</summary>
    internal static List<string> Diagnostics(string source)
    {
        var library = typeof(Value).Assembly.Location;
        var references = ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!)
            .Split(Path.PathSeparator)
            .Where(path => !string.Equals(Path.GetFileName(path), Path.GetFileName(library), StringComparison.Ordinal))
            .Append(library)
            .Select(path => MetadataReference.CreateFromFile(path));
        var parse = new CSharpParseOptions(LanguageVersion.Latest, DocumentationMode.Diagnose);
        var compilation = CSharpCompilation.Create(
            "Consumer",
            [CSharpSyntaxTree.ParseText(source, parse)],
            references,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary).WithSpecificDiagnosticOptions(
                new Dictionary<string, ReportDiagnostic> { ["CS1591"] = ReportDiagnostic.Error }));

        return compilation.GetDiagnostics()
            .Where(diagnostic => diagnostic.Severity >= Microsoft.CodeAnalysis.DiagnosticSeverity.Warning)
            .Select(diagnostic => $"{diagnostic.Id}: {diagnostic.GetMessage(System.Globalization.CultureInfo.InvariantCulture)} at {diagnostic.Location.GetLineSpan().StartLinePosition}")
            .ToList();
    }
}
