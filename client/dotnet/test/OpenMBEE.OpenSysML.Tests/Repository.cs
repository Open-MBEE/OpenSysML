using System;
using System.IO;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Paths in the repository checkout the tests were built from.</summary>
internal static class Repository
{
    /// <summary>The repository root: the directory holding global.json and client/dotnet.</summary>
    internal static string Root { get; } = FindRoot();

    /// <summary>A path under the repository root.</summary>
    internal static string PathOf(params string[] parts) => Path.Combine([Root, .. parts]);

    private static string FindRoot()
    {
        for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
        {
            if (File.Exists(Path.Combine(directory.FullName, "global.json"))
                && File.Exists(Path.Combine(directory.FullName, "client", "dotnet", "OpenSysML.slnx")))
            {
                return directory.FullName;
            }
        }

        throw new InvalidOperationException(
            $"No directory above {AppContext.BaseDirectory} holds global.json and client/dotnet/OpenSysML.slnx; the tests run from a checkout.");
    }
}
