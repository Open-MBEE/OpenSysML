using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace OpenMBEE.OpenSysML.OrphanProbe;

/// <summary>
/// The leak the orphan detector must catch: an inheritable duplicate of the write end of a private
/// service's input, alive while a sibling starts, so the sibling holds the pipe open and the service
/// outlives the probe.
/// </summary>
internal static partial class Leak
{
    private const uint DuplicateSameAccess = 2;
    private static readonly IntPtr CurrentProcess = new(-1);

    /// <summary>Starts a process while an inheritable duplicate of <paramref name="input"/>'s handle exists, then closes the duplicate.</summary>
    /// <param name="input">The write end of the service's input.</param>
    /// <param name="start">Starts the process that inherits the duplicate.</param>
    /// <returns>That process.</returns>
    internal static Process WhileStarting(Stream input, Func<Process> start)
    {
        SafeHandle handle = input switch
        {
            PipeStream pipe => pipe.SafePipeHandle,
            FileStream file => file.SafeFileHandle,
            _ => throw new InvalidOperationException($"The service's input is a {input.GetType()}, whose handle the leak control cannot reach."),
        };
        var added = false;
        handle.DangerousAddRef(ref added);
        try
        {
            return OperatingSystem.IsWindows() ? WhileStartingOnWindows(handle.DangerousGetHandle(), start) : WhileStartingOnPosix(handle.DangerousGetHandle(), start);
        }
        finally
        {
            if (added)
            {
                handle.DangerousRelease();
            }
        }
    }

    // dup never sets close-on-exec, so the descriptor it returns passes to every process started while it is open.
    private static Process WhileStartingOnPosix(IntPtr descriptor, Func<Process> start)
    {
        var copy = Dup(checked((int)descriptor));
        if (copy < 0)
        {
            throw new Win32Exception(Marshal.GetLastPInvokeError());
        }

        try
        {
            return start();
        }
        finally
        {
            _ = Close(copy);
        }
    }

    private static Process WhileStartingOnWindows(IntPtr handle, Func<Process> start)
    {
        if (!DuplicateHandle(CurrentProcess, handle, CurrentProcess, out var copy, 0, inherit: true, DuplicateSameAccess))
        {
            throw new Win32Exception(Marshal.GetLastPInvokeError());
        }

        using var owned = new SafeFileHandle(copy, ownsHandle: true);
        return start();
    }

    [LibraryImport("libc", EntryPoint = "dup", SetLastError = true)]
    private static partial int Dup(int descriptor);

    [LibraryImport("libc", EntryPoint = "close")]
    private static partial int Close(int descriptor);

    [LibraryImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool DuplicateHandle(
        IntPtr sourceProcess,
        IntPtr source,
        IntPtr targetProcess,
        out IntPtr target,
        uint access,
        [MarshalAs(UnmanagedType.Bool)] bool inherit,
        uint options);
}
