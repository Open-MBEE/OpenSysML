using System;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Reads the chain of causes an exception holds, as both transports classify failures by it.</summary>
internal static class Causes
{
    /// <summary>The first exception of a type in a chain of causes, starting with the exception itself.</summary>
    /// <typeparam name="T">The type sought.</typeparam>
    /// <param name="error">The exception, or null.</param>
    /// <returns>The exception found, or null.</returns>
    internal static T? Find<T>(Exception? error)
        where T : Exception
    {
        for (; error is not null; error = error.InnerException)
        {
            if (error is T found)
            {
                return found;
            }
        }

        return null;
    }

    /// <summary>The last cause of a chain: the failure everything else wraps.</summary>
    /// <param name="error">The exception.</param>
    /// <returns>Its innermost cause, or the exception itself when it has none.</returns>
    internal static Exception Innermost(Exception error)
    {
        while (error.InnerException is not null)
        {
            error = error.InnerException;
        }

        return error;
    }
}
