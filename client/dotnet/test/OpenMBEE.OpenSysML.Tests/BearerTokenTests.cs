using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The bearer token a connection carries: where it comes from, how it is sent, what a refusal of it raises, and that it is never told.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class BearerTokenTests
{
    private const string Secret = "s3cr3t-T0ken.for~the+test/==";
    private const string Bearer = "Bearer " + Secret;

    /// <summary>Where a call's credentials can come from, for a theory over each.</summary>
    public static TheoryData<ClientWire, string> WiresAndSources => new()
    {
        { ClientWire.Grpc, "none" }, { ClientWire.Grpc, "token" }, { ClientWire.Grpc, "variable" }, { ClientWire.Grpc, "header" },
        { ClientWire.ConnectProto, "none" }, { ClientWire.ConnectProto, "token" }, { ClientWire.ConnectProto, "variable" }, { ClientWire.ConnectProto, "header" },
        { ClientWire.ConnectJson, "none" }, { ClientWire.ConnectJson, "token" }, { ClientWire.ConnectJson, "variable" }, { ClientWire.ConnectJson, "header" },
    };

    [Theory]
    [InlineData("abc")]
    [InlineData("A-Z_0.9~+/")]
    [InlineData("YWJj==")]
    [InlineData(Secret)]
    public void ATokenOfTheServicesFormIsAccepted(string token)
    {
        new ConnectionOptions { Token = token }.Validate();
    }

    [Theory]
    [InlineData("")]
    [InlineData("   ")]
    public void ABlankTokenIsRefused(string token)
    {
        var error = Assert.Throws<InvalidRequestException>(() => new ConnectionOptions { Token = token }.Validate());

        Assert.Equal($"ConnectionOptions.Token is blank; give the service's bearer token, or leave it null for the one ${ConnectionOptions.TokenVariable} names.", error.Message);
    }

    [Theory]
    [InlineData("two words")]
    [InlineData("Bearer abc")]
    [InlineData("line\nbreak")]
    [InlineData("tab\tbed")]
    [InlineData("caf\u00e9-token")]
    [InlineData("=leading-pad")]
    [InlineData("mid=pad")]
    [InlineData("====")]
    [InlineData(" padded ")]
    public void ATokenTheServiceCouldNeverAcceptIsRefusedWithoutBeingQuoted(string token)
    {
        var error = Assert.Throws<InvalidRequestException>(() => new ConnectionOptions { Token = token }.Validate());

        Assert.Equal($"ConnectionOptions.Token is not a bearer token: {BearerToken.Rule}.", error.Message);
    }

    [Theory]
    [InlineData("authorization")]
    [InlineData("Authorization")]
    [InlineData("AUTHORIZATION")]
    public void ATokenBesideAnAuthorizationHeaderIsRefused(string name)
    {
        var options = new ConnectionOptions { Token = Secret, Headers = new Dictionary<string, string> { [name] = "Basic abc" } };

        var error = Assert.Throws<InvalidRequestException>(options.Validate);

        Assert.Contains("set one or the other", error.Message, StringComparison.Ordinal);
        Assert.DoesNotContain(Secret, error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void TheTokenOptionIsCarriedLastAsABearerAndTheVariableIsNotRead()
    {
        var options = new ConnectionOptions { Token = Secret, Headers = new Dictionary<string, string> { ["X-Tenant"] = "t1" } };

        var headers = CallHeaders.Resolve(options, name => throw new InvalidOperationException($"{name} was read."));

        Assert.Equal([KeyValuePair.Create("x-tenant", "t1"), KeyValuePair.Create("authorization", Bearer)], headers.Entries);
        Assert.Equal(CredentialSource.Token, headers.Credentials);
    }

    [Fact]
    public void TheVariableNamesTheTokenWhenTheOptionDoesNotAndIsTrimmed()
    {
        var headers = CallHeaders.Resolve(new ConnectionOptions(), Variable($"  {Secret}\n"));

        Assert.Equal([KeyValuePair.Create("authorization", Bearer)], headers.Entries);
        Assert.Equal(CredentialSource.TokenVariable, headers.Credentials);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData(" \t")]
    public void AnUnsetOrBlankVariableIsNoToken(string? value)
    {
        Assert.Same(CallHeaders.None, CallHeaders.Resolve(new ConnectionOptions(), Variable(value)));
        Assert.Equal(CredentialSource.None, CallHeaders.None.Credentials);
        Assert.Empty(CallHeaders.None.Entries);
    }

    [Fact]
    public void AnAuthorizationHeaderTakesThePlaceOfTheVariable()
    {
        var options = new ConnectionOptions { Headers = new Dictionary<string, string> { ["Authorization"] = "Basic abc" } };

        var headers = CallHeaders.Resolve(options, Variable(Secret));

        Assert.Equal([KeyValuePair.Create("authorization", "Basic abc")], headers.Entries);
        Assert.Equal(CredentialSource.Header, headers.Credentials);
    }

    [Theory]
    [InlineData("not a token")]
    [InlineData("caf\u00e9")]
    public void AVariableThatIsNoTokenIsRefusedWithoutBeingQuoted(string value)
    {
        var error = Assert.Throws<InvalidRequestException>(() => CallHeaders.Resolve(new ConnectionOptions(), Variable(value)));

        Assert.Equal($"${ConnectionOptions.TokenVariable} does not name a bearer token: {BearerToken.Rule}. Correct it, or unset it if no service needs one.", error.Message);
    }

    [Theory]
    [InlineData(false, new[] { "opensysml_grpc_token", "Opensysml_Grpc_Token", "OPENSYSML_GRPC_TOKEN_FILE", "PATH" })]
    [InlineData(true, new[] { "OPENSYSML_GRPC_TOKEN_FILE", "PATH" })]
    public void APrivateServicesEnvironmentLosesTheTokenVariableUnderAnyCaseOnlyOnWindows(bool windows, string[] kept)
    {
        var environment = new Dictionary<string, string?>(StringComparer.Ordinal)
        {
            ["OPENSYSML_GRPC_TOKEN"] = Secret,
            ["opensysml_grpc_token"] = Secret,
            ["Opensysml_Grpc_Token"] = Secret,
            ["OPENSYSML_GRPC_TOKEN_FILE"] = "/run/token",
            ["PATH"] = "/usr/bin",
        };

        PrivateService.RemoveToken(environment, windows);

        Assert.Equal(kept.Order(StringComparer.Ordinal), environment.Keys.Order(StringComparer.Ordinal));
    }

    [Fact]
    public void AWindowsEnvironmentWhoseNamesIgnoreCaseLosesTheVariableToo()
    {
        var environment = new Dictionary<string, string?>(StringComparer.OrdinalIgnoreCase) { ["Opensysml_Grpc_Token"] = Secret, ["Path"] = "C:\\Windows" };

        PrivateService.RemoveToken(environment, windows: true);

        Assert.Equal(["Path"], environment.Keys);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task EveryCallCarriesTheTokenOption(ClientWire wire)
    {
        using var handler = Answering(wire);

        await using (var connection = await OpenAsync(handler, Options(handler, token: Secret), Variable("another-token")))
        {
            await connection.GetSymbolAsync();
        }

        Assert.Equal(2, handler.Requests.Count);
        Assert.All(handler.Requests, request => Assert.Equal(Bearer, request.Headers["authorization"]));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task EveryCallCarriesTheTokenTheVariableNamesWhenNoneIsGiven(ClientWire wire)
    {
        using var handler = Answering(wire);

        await using (var connection = await OpenAsync(handler, Options(handler), Variable(Secret)))
        {
            await connection.GetSymbolAsync();
        }

        Assert.All(handler.Requests, request => Assert.Equal(Bearer, request.Headers["authorization"]));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ABlankVariableSendsNoAuthorization(ClientWire wire)
    {
        using var handler = Answering(wire);

        await using (var connection = await OpenAsync(handler, Options(handler), Variable("  ")))
        {
            await connection.GetSymbolAsync();
        }

        Assert.All(handler.Requests, request => Assert.False(request.Headers.ContainsKey("authorization")));
    }

    [Theory]
    [MemberData(nameof(WiresAndSources))]
    public async Task UnauthenticatedFromTheServiceIsAnAuthenticationExceptionSayingWhatWasSent(ClientWire wire, string source)
    {
        const string Said = "Unauthorized: the bearer token is not this server's";
        using var handler = new FakeServiceHandler(wire) { ServerInfo = (_, _) => Task.FromResult(FakeServiceHandler.RefuseOn(wire, StatusCode.Unauthenticated, Said)) };
        var (options, environment) = source switch
        {
            "token" => (Options(handler, token: Secret), Variable(null)),
            "variable" => (Options(handler), Variable(Secret)),
            "header" => (Options(handler, headers: new Dictionary<string, string> { ["Authorization"] = Bearer }), Variable(null)),
            _ => (Options(handler), Variable(null)),
        };

        var error = await Assert.ThrowsAsync<AuthenticationException>(() => OpenAsync(handler, options, environment));

        Assert.Equal(StatusCode.Unauthenticated, error.StatusCode);
        Assert.Equal(Said, error.ServiceMessage);
        Assert.Equal("sysml.SysMLService/GetServerInfo", error.Method);
        Assert.Equal(source != "none", error.CredentialsSent);
        Assert.Equal(
            source switch
            {
                "token" => "the bearer token ConnectionOptions.Token gives was refused: give the token this service was started with",
                "variable" => "the bearer token $OPENSYSML_GRPC_TOKEN names was refused: give the token this service was started with, by ConnectionOptions.Token or $OPENSYSML_GRPC_TOKEN",
                "header" => "the authorization header ConnectionOptions.Headers gives was refused: correct it, or remove it and give the token this service was started with by ConnectionOptions.Token or $OPENSYSML_GRPC_TOKEN",
                _ => "the service requires a bearer token and none was sent: give the token it was started with, by ConnectionOptions.Token or $OPENSYSML_GRPC_TOKEN",
            },
            error.Remedy);
        Assert.Equal($"sysml.SysMLService/GetServerInfo was refused by the service with Unauthenticated: {Said}\n  fix: {error.Remedy}", error.Message);
        Assert.DoesNotContain(Secret, error.ToString(), StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnUnauthenticatedOnACallIsAnAuthenticationExceptionToo(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.RefuseOn(wire, StatusCode.Unauthenticated, "who are you")) };
        await using var connection = await OpenAsync(handler, Options(handler, token: Secret), Variable(null));

        var error = await Assert.ThrowsAsync<AuthenticationException>(connection.GetSymbolAsync);

        Assert.Equal("sysml.SysMLService/GetSymbol", error.Method);
        Assert.True(error.CredentialsSent);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnHttp401ThatIsNotTheServicesRefusalIsATransportException(ClientWire wire)
    {
        var page = "<html><body>401 Authorization Required</body></html>"u8.ToArray();
        using var handler = new FakeServiceHandler(wire)
        {
            ServerInfo = (_, _) => Task.FromResult(wire == ClientWire.Grpc
                ? GrpcResponses.Raw(page, status: null, contentType: "text/html", code: HttpStatusCode.Unauthorized)
                : ConnectResponses.Raw(HttpStatusCode.Unauthorized, "text/html", page)),
        };

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(handler, Options(handler, token: Secret), Variable(null)));

        Assert.Equal(StatusCode.Unauthenticated, error.StatusCode);
    }

    [Theory]
    [InlineData("http://fake.invalid:50051", "token", true)]
    [InlineData("http://fake.invalid:50051", "header", true)]
    [InlineData("http://fake.invalid:50051", "none", false)]
    [InlineData("https://fake.invalid:50051", "token", false)]
    [InlineData("127.0.0.1:50051", "token", false)]
    [InlineData("localhost:50051", "token", false)]
    [InlineData("[::1]:50051", "token", false)]
    public async Task CredentialsSentInTheClearToAnotherMachineAreWarnedOfOnceAndNeverLogged(string address, string source, bool warned)
    {
        using var handler = Answering(ClientWire.ConnectProto);
        var logger = new ListLogger();
        var options = Options(
            handler,
            token: source == "token" ? Secret : null,
            headers: source == "header" ? new Dictionary<string, string> { ["authorization"] = Bearer } : null,
            address: address,
            logger: logger);

        await using (var connection = await OpenAsync(handler, options, Variable(null)))
        {
            await connection.GetSymbolAsync();
            await connection.GetSymbolAsync();
        }

        var warnings = logger.Entries.Count(entry => entry.Level == LogLevel.Warning && entry.Message.Contains("cleartext HTTP to another machine", StringComparison.Ordinal));
        Assert.Equal(warned ? 1 : 0, warnings);
        Assert.NotEmpty(logger.Entries);
        Assert.DoesNotContain(logger.Entries, entry => entry.Message.Contains(Secret, StringComparison.Ordinal) || entry.Exception?.ToString().Contains(Secret, StringComparison.Ordinal) == true);
    }

    private static Func<string, string?> Variable(string? value) =>
        name => name == ConnectionOptions.TokenVariable ? value : null;

    private static FakeServiceHandler Answering(ClientWire wire) =>
        new(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(wire, new Wire.SymbolResponse())) };

    private static ConnectionOptions Options(
        FakeServiceHandler handler,
        string? token = null,
        IReadOnlyDictionary<string, string>? headers = null,
        string? address = null,
        ILogger? logger = null) => new()
        {
            Address = address ?? FakeServiceHandler.Address,
            Protocol = handler.Wire.Protocol(),
            Encoding = handler.Wire.Encoding(),
            HttpMessageHandler = handler,
            Headers = headers ?? new Dictionary<string, string>(),
            Token = token,
            Logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance,
        };

    private static Task<Connection> OpenAsync(FakeServiceHandler handler, ConnectionOptions options, Func<string, string?> environment)
    {
        Assert.Same(handler, options.HttpMessageHandler);
        return Connection.OpenAsync(options, environment, TestContext.Current.CancellationToken);
    }
}
