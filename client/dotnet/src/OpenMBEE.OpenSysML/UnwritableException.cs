using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A save or write refused its destination before writing anything: the file exists and overwriting was not asked
/// for, it is a directory or a symbolic link, its directory does not exist, it is the SysML v1 model a migration came
/// from, or a file a migration refers to is not a relative path below the model's directory, would land on another of
/// its files, or would pass through a symbolic link. Nothing on disk was changed.
/// </summary>
public class UnwritableException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">Why the destination was refused.</param>
    /// <param name="path">The destination refused, as a full path.</param>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    public UnwritableException(string message, string path)
        : base(message)
    {
        ArgumentNullException.ThrowIfNull(path);
        Path = path;
    }

    /// <summary>The destination refused, as a full path.</summary>
    public string Path { get; }
}
