using System;
using System.Reflection;
using System.Text.RegularExpressions;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The release of <c>sysml-grpc</c> a private service is provided from when no binary is named: its repository,
/// from <see cref="ConnectionOptions.GithubRepo"/>, else <see cref="ConnectionOptions.GithubRepoVariable"/>, else
/// <see cref="ConnectionOptions.DefaultGithubRepo"/>; and its tag, from <see cref="ConnectionOptions.DownloadVersion"/>,
/// else <see cref="ConnectionOptions.VersionVariable"/>, else the release this client was built with, which alone is
/// not <see cref="Explicit"/>.
/// </summary>
/// <param name="Repo">The repository, <c>owner/repo</c>.</param>
/// <param name="Version">The release tag, or <see cref="Latest"/>.</param>
/// <param name="Explicit">Whether the tag was asked for, rather than being the release this client was built with.</param>
/// <param name="UnpinnedAllowed">Whether <see cref="ConnectionOptions.AllowUnpinnedDownloadVariable"/> grants same-origin trust to <paramref name="Repo"/>.</param>
internal sealed partial record ReleaseRequest(string Repo, string Version, bool Explicit, bool UnpinnedAllowed)
{
    /// <summary>The tag that asks for the repository's latest published release.</summary>
    internal const string Latest = "latest";

    /// <summary>This client's version, without build metadata.</summary>
    internal static string ClientVersion { get; } = ReadClientVersion();

    /// <summary>The release this client was built with: <c>v</c> and its version.</summary>
    internal static string BuiltAgainst { get; } = "v" + ClientVersion;

    /// <summary>Whether the tag asks for the latest release, which the releases API resolves.</summary>
    internal bool IsLatest => Version == Latest;

    /// <summary>The release options and the environment ask for.</summary>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <returns>The request.</returns>
    /// <exception cref="InvalidRequestException">A variable names no repository or tag.</exception>
    internal static ReleaseRequest From(ConnectionOptions options, Func<string, string?> environment)
    {
        var repo = options.GithubRepo ?? Variable(environment, ConnectionOptions.GithubRepoVariable) ?? ConnectionOptions.DefaultGithubRepo;
        if (!IsRepo(repo))
        {
            throw new InvalidRequestException($"${ConnectionOptions.GithubRepoVariable} is \"{repo}\", which is not an owner/repo of letters, digits, '.', '-' and '_'.");
        }

        var version = options.DownloadVersion ?? Variable(environment, ConnectionOptions.VersionVariable);
        if (version is not null && !IsVersion(version))
        {
            throw new InvalidRequestException(
                $"${ConnectionOptions.VersionVariable} is \"{version}\", which is neither \"{Latest}\" nor a release tag of letters, digits, '.', '-', '_' and '+'.");
        }

        return new ReleaseRequest(repo, version ?? BuiltAgainst, version is not null, AllowsUnpinned(environment(ConnectionOptions.AllowUnpinnedDownloadVariable), repo));
    }

    /// <summary>Whether a setting of <see cref="ConnectionOptions.AllowUnpinnedDownloadVariable"/> grants same-origin trust to a repository.</summary>
    /// <param name="setting">The variable's value; null when it is unset.</param>
    /// <param name="repo">The repository, <c>owner/repo</c>.</param>
    /// <returns>
    /// True for <c>1</c>, <c>true</c> or <c>yes</c>, or a comma-separated list naming the repository exactly; false
    /// when unset, empty, <c>0</c>, <c>false</c> or <c>no</c>, as every client reads it.
    /// </returns>
    internal static bool AllowsUnpinned(string? setting, string repo)
    {
        var value = setting?.Trim() ?? string.Empty;
        if (value.Length == 0 || value is "0" || value.Equals("false", StringComparison.OrdinalIgnoreCase) || value.Equals("no", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        if (value is "1" || value.Equals("true", StringComparison.OrdinalIgnoreCase) || value.Equals("yes", StringComparison.OrdinalIgnoreCase))
        {
            return true;
        }

        foreach (var named in value.Split(',', StringSplitOptions.TrimEntries))
        {
            if (named == repo)
            {
                return true;
            }
        }

        return false;
    }

    /// <summary>Whether text is an <c>owner/repo</c> whose names stay one URL path segment each.</summary>
    /// <param name="text">The text.</param>
    /// <returns>Whether it is.</returns>
    internal static bool IsRepo(string text)
    {
        var slash = text.IndexOf('/', StringComparison.Ordinal);
        return slash > 0 && IsRepoName(text[..slash]) && IsRepoName(text[(slash + 1)..]);
    }

    /// <summary>Whether text is <see cref="Latest"/> or a release tag that stays one URL path segment.</summary>
    /// <param name="text">The text.</param>
    /// <returns>Whether it is.</returns>
    internal static bool IsVersion(string text) => text == Latest || IsTag(text);

    /// <summary>Whether text is a release tag that stays one URL path segment: no delimiter, and nothing to climb with.</summary>
    /// <param name="text">The text.</param>
    /// <returns>Whether it is.</returns>
    internal static bool IsTag(string text) => TagPattern().IsMatch(text) && !text.Contains("..", StringComparison.Ordinal);

    private static bool IsRepoName(string name) => RepoNamePattern().IsMatch(name) && name.Trim('.').Length != 0;

    private static string? Variable(Func<string, string?> environment, string name) =>
        environment(name) is { } value && !string.IsNullOrWhiteSpace(value) ? value.Trim() : null;

    private static string ReadClientVersion()
    {
        var version = typeof(ReleaseRequest).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
            ?? throw new InvalidOperationException("The client assembly declares no informational version.");

        // SemVer build metadata names no release.
        var plus = version.IndexOf('+', StringComparison.Ordinal);
        return plus < 0 ? version : version[..plus];
    }

    [GeneratedRegex(@"^[A-Za-z0-9_+][A-Za-z0-9._+-]{0,127}\z", RegexOptions.CultureInvariant)]
    private static partial Regex TagPattern();

    [GeneratedRegex(@"^[A-Za-z0-9._-]{1,128}\z", RegexOptions.CultureInvariant)]
    private static partial Regex RepoNamePattern();
}
