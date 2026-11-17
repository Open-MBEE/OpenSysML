using System;
using System.IO;
using System.Linq;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Finds an external tool as a shell would: a name with a directory is that file, a bare name the first on a search path.</summary>
internal static class ToolSearch
{
    /// <summary>The name of an executable file on this platform: <paramref name="name"/>, with .exe on Windows.</summary>
    internal static string Executable(string name) =>
        OperatingSystem.IsWindows() && !name.EndsWith(".exe", StringComparison.OrdinalIgnoreCase) ? name + ".exe" : name;

    /// <summary>The full path of <paramref name="path"/> when it is a file, else null.</summary>
    internal static string? Existing(string path) => File.Exists(path) ? Path.GetFullPath(path) : null;

    /// <summary>The full path of the tool <paramref name="name"/> names, or null when there is none.</summary>
    /// <param name="name">A path, or a bare name looked for in each directory of <paramref name="searchPath"/>.</param>
    /// <param name="searchPath">The value of PATH; null when it is unset.</param>
    internal static string? Find(string name, string? searchPath)
    {
        if (name.IndexOfAny([Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar]) >= 0)
        {
            return Existing(name) ?? Existing(Executable(name));
        }

        var directories = (searchPath ?? string.Empty).Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        return directories.Select(directory => Existing(Path.Combine(directory, Executable(name)))).FirstOrDefault(found => found is not null);
    }
}
