using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class CapabilitiesTests
{
    private static readonly IReadOnlyDictionary<string, string> Constants = typeof(Capabilities)
        .GetFields(BindingFlags.Public | BindingFlags.Static)
        .Where(field => field.IsLiteral)
        .ToDictionary(field => field.Name, field => (string)field.GetRawConstantValue()!);

    [Fact]
    public void EveryCapabilityTheServiceDeclaresHasAConstantAndNoConstantIsInvented()
    {
        Assert.NotEmpty(ServiceCapabilities.Names);
        Assert.Equal(ServiceCapabilities.Names.Order(StringComparer.Ordinal), Constants.Values.Order(StringComparer.Ordinal));
    }

    [Fact]
    public void EachConstantIsNamedForItsValue()
    {
        Assert.All(Constants, constant => Assert.Equal(
            string.Concat(constant.Value.Split('_').Select(word => char.ToUpperInvariant(word[0]) + word[1..])),
            constant.Key));
    }

    [Fact]
    public void NamesAreCopiedDeduplicatedAndComparedOrdinally()
    {
        var names = new List<string> { "query", "convert", "query" };
        var capabilities = new Capabilities(names);
        names.Add("migrate");

        Assert.Equal(2, capabilities.Names.Count);
        Assert.True(capabilities.Has(Capabilities.Query));
        Assert.False(capabilities.Has("Query"));
        Assert.False(capabilities.Has(Capabilities.Migrate));
        Assert.Equal("[convert, query]", capabilities.ToString());
    }

    [Fact]
    public void AnUnknownNameFromANewerServiceIsAdvertisedLikeAnyOther()
    {
        var capabilities = new Capabilities(["query", "a_capability_from_the_future"]);

        Assert.True(capabilities.Has("a_capability_from_the_future"));
        capabilities.Require("a_capability_from_the_future");
    }

    [Fact]
    public void NullsAreRefused()
    {
        Assert.Throws<ArgumentNullException>(() => new Capabilities(null!));
        Assert.Throws<ArgumentException>(() => new Capabilities(["query", null!]));
        Assert.Throws<ArgumentNullException>(() => Capabilities.None.Has(null!));
        Assert.Throws<ArgumentNullException>(() => Capabilities.UpgradeRemedy(null!));
    }

    [Fact]
    public void RequireRefusesAMissingCapabilityNamingWhatIsAdvertisedAndTheRemedy()
    {
        var capabilities = new Capabilities(["query", "convert"]);

        var error = Assert.Throws<CapabilityException>(() => capabilities.Require(Capabilities.Migrate));

        Assert.Equal(Capabilities.Migrate, error.Capability);
        Assert.Equal(Capabilities.UpgradeRemedy(Capabilities.Migrate), error.Remedy);
        Assert.Contains("[convert, query]", error.Message, StringComparison.Ordinal);
        Assert.Contains("(version unknown)", error.Message, StringComparison.Ordinal);
        Assert.Contains(error.Remedy, error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<OpenSysMLException>(error);
    }

    [Fact]
    public void NoneAdvertisesNothing()
    {
        Assert.Empty(Capabilities.None.Names);
        Assert.Throws<CapabilityException>(() => Capabilities.None.Require(Capabilities.Query));
    }

    [Fact]
    public void EqualityIsSetEqualityIndependentOfOrder()
    {
        var one = new Capabilities(["query", "convert"]);
        var other = new Capabilities(["convert", "query", "query"]);

        Assert.True(one.Equals(other));
        Assert.True(one.Equals((object)other));
        Assert.Equal(one.GetHashCode(), other.GetHashCode());
        Assert.False(one.Equals(new Capabilities(["query"])));
        Assert.False(one.Equals(null));
    }

    [Fact]
    public void ServerInfoIsAValue()
    {
        var info = new ServerInfo("v1", new Capabilities(["query"]), answered: true);

        Assert.Equal(info, new ServerInfo("v1", new Capabilities(["query"]), answered: true));
        Assert.True(info.Equals((object)new ServerInfo("v1", new Capabilities(["query"]), answered: true)));
        Assert.Equal(info.GetHashCode(), new ServerInfo("v1", new Capabilities(["query"]), answered: true).GetHashCode());
        Assert.NotEqual(info, new ServerInfo("v2", new Capabilities(["query"]), answered: true));
        Assert.NotEqual(info, new ServerInfo("v1", Capabilities.None, answered: true));
        Assert.NotEqual(info, new ServerInfo("v1", new Capabilities(["query"]), answered: false));
        Assert.False(info.Equals(null));
        Assert.Equal("sysml-grpc v1, 1 capabilities", info.ToString());
        Assert.Equal("sysml-grpc (version unknown), 1 capabilities", new ServerInfo(string.Empty, new Capabilities(["query"]), answered: true).ToString());
        Assert.Equal("sysml-grpc (did not answer GetServerInfo)", new ServerInfo(string.Empty, Capabilities.None, answered: false).ToString());
        Assert.Throws<ArgumentNullException>(() => new ServerInfo(null!, Capabilities.None, answered: true));
        Assert.Throws<ArgumentNullException>(() => new ServerInfo("v1", null!, answered: true));
    }
}

[Trait(Category.Name, Category.Unit)]
public sealed class StatusCodeTests
{
    [Fact]
    public void StatusCodesAreGrpcsByNameAndNumber()
    {
        var ours = Enum.GetValues<StatusCode>().Select(code => (Name: code.ToString(), Number: (int)code));
        var grpc = Enum.GetValues<Grpc.Core.StatusCode>().Select(code => (Name: code == Grpc.Core.StatusCode.OK ? "Ok" : code.ToString(), Number: (int)code));

        Assert.Equal(grpc.OrderBy(code => code.Number), ours.OrderBy(code => code.Number));
    }

    [Fact]
    public void ExceptionsCarryTheirPayloads()
    {
        var cause = new InvalidOperationException("cause");
        var service = new ServiceException(StatusCode.NotFound, "model not found: m", "sysml.SysMLService/GetSymbol", cause);
        Assert.Equal(StatusCode.NotFound, service.StatusCode);
        Assert.Equal("model not found: m", service.ServiceMessage);
        Assert.Equal("sysml.SysMLService/GetSymbol", service.Method);
        Assert.Same(cause, service.InnerException);
        Assert.Equal("sysml.SysMLService/GetSymbol was refused by the service with NotFound: model not found: m", service.Message);
        Assert.Throws<ArgumentException>(() => new ServiceException(StatusCode.Ok, "fine", "m"));
        Assert.Throws<ArgumentNullException>(() => new ServiceException(StatusCode.Internal, null!, "m"));

        var transport = new TransportException(StatusCode.Unavailable, "down", "m", cause);
        Assert.Equal(StatusCode.Unavailable, transport.StatusCode);
        Assert.Equal("m", transport.Method);
        Assert.Same(cause, transport.InnerException);
        Assert.Throws<ArgumentNullException>(() => new TransportException(StatusCode.Internal, "x", null!));

        var stale = new StaleServiceException("http://h:1/", "it reports v1", "fix it", "v1");
        Assert.Equal(("http://h:1/", "it reports v1", "fix it", "v1"), (stale.Address, stale.Reason, stale.Remedy, stale.ServiceVersion));
        Assert.Contains("(version unknown)", new StaleServiceException("a", "b", "c", string.Empty).Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ServiceStartException>(stale);

        Assert.All(
            new OpenSysMLException[]
            {
                new(), new("m"), new("m", cause), new InvalidRequestException("m"), new InvalidRequestException("m", cause),
                new ServiceStartException("m"), new ServiceStartException("m", cause),
            },
            error => Assert.IsAssignableFrom<Exception>(error));
    }
}
