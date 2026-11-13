using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace OpenMBEE.OpenSysML.CacheProbe;

/// <summary>
/// Holds or tries the cross-client lock alone, as another client would, with this probe's own system calls rather than
/// the client's, so that a change to how the client locks cannot also change what checks it: <c>lockf</c> on a
/// descriptor libc opened on POSIX, <c>LockFileEx</c> over the whole file on Windows.
/// </summary>
internal static partial class SharedLock
{
    private const int LockfUnlock = 0;
    private const int LockfLock = 1;
    private const int LockfTryLock = 2;
    private const int AccessDenied = 13;
    private const uint FailImmediately = 1;
    private const uint ExclusiveLock = 2;
    private const int WindowsLockViolation = 33;

    /// <summary>Whether no process holds the lock: it is taken and released at once.</summary>
    /// <param name="path">The lock file; one that is not there is free.</param>
    /// <returns>Whether it was free.</returns>
    internal static bool IsFree(string path)
    {
        if (!File.Exists(path))
        {
            return true;
        }

        return OperatingSystem.IsWindows() ? IsFreeOnWindows(path) : IsFreeOnPosix(path);
    }

    /// <summary>Takes the lock, waiting for it, and keeps it until <paramref name="held"/> returns; the file must exist.</summary>
    /// <param name="path">The lock file.</param>
    /// <param name="held">Runs while the lock is held.</param>
    internal static void Hold(string path, Action held)
    {
        if (OperatingSystem.IsWindows())
        {
            using var file = File.OpenHandle(path, FileMode.Open, FileAccess.ReadWrite, FileShare.ReadWrite | FileShare.Delete);
            var overlapped = default(Overlapped);
            if (!LockFileEx(file, ExclusiveLock, 0, uint.MaxValue, uint.MaxValue, ref overlapped))
            {
                throw new Win32Exception(Marshal.GetLastPInvokeError(), $"LockFileEx {path}");
            }

            held();
            _ = UnlockFileEx(file, 0, uint.MaxValue, uint.MaxValue, ref overlapped);
            return;
        }

        var descriptor = OpenOnPosix(path);
        try
        {
            if (Lockf(descriptor, LockfLock, 0) != 0)
            {
                throw new Win32Exception(Marshal.GetLastPInvokeError(), $"lockf {path}");
            }

            held();
            _ = Lockf(descriptor, LockfUnlock, 0);
        }
        finally
        {
            _ = Close(descriptor);
        }
    }

    private static int OpenOnPosix(string path)
    {
        var descriptor = Open(path, 2 | (OperatingSystem.IsMacOS() ? 0x1000000 : 0x80000));
        return descriptor >= 0 ? descriptor : throw new Win32Exception(Marshal.GetLastPInvokeError(), $"open {path}");
    }

    private static bool IsFreeOnPosix(string path)
    {
        var descriptor = OpenOnPosix(path);
        try
        {
            if (Lockf(descriptor, LockfTryLock, 0) == 0)
            {
                _ = Lockf(descriptor, LockfUnlock, 0);
                return true;
            }

            var error = Marshal.GetLastPInvokeError();
            return error == (OperatingSystem.IsMacOS() ? 35 : 11) || error == AccessDenied
                ? false
                : throw new Win32Exception(error, $"lockf {path}");
        }
        finally
        {
            _ = Close(descriptor);
        }
    }

    private static bool IsFreeOnWindows(string path)
    {
        using var file = File.OpenHandle(path, FileMode.Open, FileAccess.ReadWrite, FileShare.ReadWrite | FileShare.Delete);
        var overlapped = default(Overlapped);
        if (LockFileEx(file, FailImmediately | ExclusiveLock, 0, uint.MaxValue, uint.MaxValue, ref overlapped))
        {
            _ = UnlockFileEx(file, 0, uint.MaxValue, uint.MaxValue, ref overlapped);
            return true;
        }

        var error = Marshal.GetLastPInvokeError();
        return error == WindowsLockViolation ? false : throw new Win32Exception(error, $"LockFileEx {path}");
    }

    [LibraryImport("libc", EntryPoint = "open", StringMarshalling = StringMarshalling.Utf8, SetLastError = true)]
    private static partial int Open(string path, int flags);

    [LibraryImport("libc", EntryPoint = "lockf", SetLastError = true)]
    private static partial int Lockf(int descriptor, int command, nint length);

    [LibraryImport("libc", EntryPoint = "close")]
    private static partial int Close(int descriptor);

    [LibraryImport("kernel32.dll", EntryPoint = "LockFileEx", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool LockFileEx(SafeFileHandle file, uint flags, uint reserved, uint lengthLow, uint lengthHigh, ref Overlapped overlapped);

    [LibraryImport("kernel32.dll", EntryPoint = "UnlockFileEx", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool UnlockFileEx(SafeFileHandle file, uint reserved, uint lengthLow, uint lengthHigh, ref Overlapped overlapped);

    [StructLayout(LayoutKind.Sequential)]
    private struct Overlapped
    {
        public nint Internal;
        public nint InternalHigh;
        public uint Offset;
        public uint OffsetHigh;
        public nint Event;
    }
}
