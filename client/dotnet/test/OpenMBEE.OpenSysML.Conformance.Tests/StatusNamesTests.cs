using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class StatusNamesTests
{
    public static TheoryData<StatusCode, string> Canonical => new()
    {
        { StatusCode.Ok, "OK" },
        { StatusCode.Cancelled, "CANCELLED" },
        { StatusCode.Unknown, "UNKNOWN" },
        { StatusCode.InvalidArgument, "INVALID_ARGUMENT" },
        { StatusCode.DeadlineExceeded, "DEADLINE_EXCEEDED" },
        { StatusCode.NotFound, "NOT_FOUND" },
        { StatusCode.AlreadyExists, "ALREADY_EXISTS" },
        { StatusCode.PermissionDenied, "PERMISSION_DENIED" },
        { StatusCode.ResourceExhausted, "RESOURCE_EXHAUSTED" },
        { StatusCode.FailedPrecondition, "FAILED_PRECONDITION" },
        { StatusCode.Aborted, "ABORTED" },
        { StatusCode.OutOfRange, "OUT_OF_RANGE" },
        { StatusCode.Unimplemented, "UNIMPLEMENTED" },
        { StatusCode.Internal, "INTERNAL" },
        { StatusCode.Unavailable, "UNAVAILABLE" },
        { StatusCode.DataLoss, "DATA_LOSS" },
        { StatusCode.Unauthenticated, "UNAUTHENTICATED" },
    };

    [Theory]
    [MemberData(nameof(Canonical))]
    public void AStatusIsNamedCanonicallyAndMatchesItsName(StatusCode code, string name)
    {
        Assert.Equal(name, StatusNames.Name(code));
        Assert.True(StatusNames.Same(name, code));
        Assert.True(StatusNames.Same(name.ToLowerInvariant(), code));
    }

    [Fact]
    public void AStatusAlsoMatchesTheReferencesCamelSpelling()
    {
        Assert.True(StatusNames.Same("NotFound", StatusCode.NotFound));
        Assert.True(StatusNames.Same("invalidargument", StatusCode.InvalidArgument));
        Assert.True(StatusNames.Same("Canceled", StatusCode.Cancelled));
        Assert.True(StatusNames.Same("NOTFOUND", StatusCode.NotFound));
    }

    [Fact]
    public void AStatusDoesNotMatchAnother()
    {
        Assert.False(StatusNames.Same("NOT_FOUND", StatusCode.InvalidArgument));
        Assert.False(StatusNames.Same("OK", StatusCode.Unimplemented));
        Assert.False(StatusNames.Same("NOT FOUND", StatusCode.NotFound));
        Assert.False(StatusNames.Same("Cancelled", StatusCode.Unknown));
    }

    [Fact]
    public void AStatusTheProtocolDoesNotDefineIsNumbered()
    {
        Assert.Equal("Code(42)", StatusNames.Name((StatusCode)42));
        Assert.True(StatusNames.Same("Code(42)", (StatusCode)42));
        Assert.False(StatusNames.Same("UNKNOWN", (StatusCode)42));
    }
}
