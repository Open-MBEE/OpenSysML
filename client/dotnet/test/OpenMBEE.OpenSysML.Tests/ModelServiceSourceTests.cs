using System;
using System.IO;
using System.Text.RegularExpressions;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds the client's reading of a NotFound status to the service's own source. The client turns
/// NotFound into <see cref="ModelFileNotFoundException"/> for a parse and <see cref="ModelNotFoundException"/>
/// for a call on a model, never by the message's words, which is sound only while each such method's
/// handler, and the helpers it reads sources through, sends NotFound for that one cause.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class ModelServiceSourceTests
{
    private static readonly string Service = Repository.PathOf("internal", "frontend", "grpc", "service.go");
    private static readonly string Query = Repository.PathOf("internal", "frontend", "grpc", "query.go");

    public static TheoryData<string, string> ModelMethods => new()
    {
        { Service, "func (s *Service) GetSymbol(" },
        { Service, "func (s *Service) GetDiagnostics(" },
        { Service, "func (s *Service) Evaluate(" },
        { Service, "func (s *Service) Instantiate(" },
        { Query, "func (s *Service) Query(" },
    };

    [Theory]
    [MemberData(nameof(ModelMethods))]
    public void ACallOnAModelIsNotFoundOnlyForAModelTheServiceDoesNotHold(string path, string signature)
    {
        var body = GoFunction(path, signature);

        var notFound = Assert.Single(NotFound().Matches(body));
        Assert.Matches(@"cache\.Get\(req\.ModelHash\)\s*\n\s*if !ok \{\s*\n\s*return nil, statusErrorf\(connect\.CodeNotFound, (msgModelNotFound|""model not found: %s"")", body);
        Assert.Contains("ModelHash", notFound.Value, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("func (s *Service) ParseFile(")]
    [InlineData("func (s *Service) ParseSources(")]
    [InlineData("func (s *Service) documentInput(")]
    [InlineData("func inlineInput(")]
    public void AParseIsNotFoundOnlyForAFileItCannotRead(string signature)
    {
        Assert.Empty(NotFound().Matches(GoFunction(Service, signature)));
        Assert.Contains("fileInput(", GoFunction(Service, "func (s *Service) ParseFile(") + GoFunction(Service, "func (s *Service) documentInput("), StringComparison.Ordinal);
        var reading = Assert.Single(NotFound().Matches(GoFunction(Service, "func fileInput(")));
        Assert.Contains("file not found", reading.Value, StringComparison.Ordinal);
    }

    private static string GoFunction(string path, string signature)
    {
        var source = File.ReadAllText(path);
        var start = source.IndexOf(signature, StringComparison.Ordinal);
        Assert.True(start >= 0, $"{path} has no {signature}");
        var end = source.IndexOf("\n}\n", start, StringComparison.Ordinal);
        return source[start..end];
    }

    [GeneratedRegex(@"CodeNotFound,[^\n]*")]
    private static partial Regex NotFound();
}
