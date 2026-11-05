using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The operating-system calls the shared cache needs and .NET does not offer: on POSIX a descriptor opened by
/// libc <c>open(2)</c>, never by a <see cref="FileStream"/>, whose implicit <c>flock</c> conflicts with the
/// other clients' <c>fcntl</c> locks on macOS; <c>lockf</c>, the lock Python, Java and Rust take; <c>flock</c>
/// for the guard; and hard links on every platform. On Windows <c>LockFileEx</c> over the whole file.
/// </summary>
internal static partial class CacheNative
{
    private const int ReadWrite = 2;
    private const int LockfUnlock = 0;
    private const int LockfTryLock = 2;
    private const int FlockExclusive = 2;
    private const int FlockNonBlocking = 4;
    private const int FlockUnlock = 8;
    private const int Interrupted = 4;
    private const int AccessDenied = 13;
    private const uint FailImmediately = 1;
    private const uint ExclusiveLock = 2;
    private const int WindowsLockViolation = 33;

    /// <summary>
    /// Opens an existing file for reading and writing with libc <c>open(path, O_RDWR | O_CLOEXEC)</c>, so no
    /// process this one starts inherits it. Only the two fixed arguments are passed, which the variadic
    /// calling convention of Apple arm64 allows.
    /// </summary>
    /// <param name="path">The file, which must exist.</param>
    /// <returns>The descriptor, owned by the handle.</returns>
    /// <exception cref="IOException">It could not be opened.</exception>
    internal static SafeFileHandle OpenDescriptor(string path)
    {
        var flags = ReadWrite | CloseOnExec;
        while (true)
        {
            var descriptor = Open(path, flags);
            if (descriptor >= 0)
            {
                return new SafeFileHandle(descriptor, ownsHandle: true);
            }

            var error = Marshal.GetLastPInvokeError();
            if (error != Interrupted)
            {
                throw Failure(error, $"open {path}");
            }
        }
    }

    /// <summary>Takes the <c>fcntl</c> lock over the whole of a file without waiting: <c>lockf(F_TLOCK, 0)</c> at offset 0.</summary>
    /// <param name="file">A descriptor from <see cref="OpenDescriptor"/>, at offset 0.</param>
    /// <param name="path">The file, for messages.</param>
    /// <returns>Whether it was taken; false when another process holds it.</returns>
    /// <exception cref="IOException">The lock could not be attempted.</exception>
    internal static bool TryLockf(SafeFileHandle file, string path) =>
        Attempt(file, path, "lockf", descriptor => Lockf(descriptor, LockfTryLock, 0));

    /// <summary>Releases the <c>fcntl</c> lock <see cref="TryLockf"/> took; closing the descriptor releases it too.</summary>
    /// <param name="file">The descriptor.</param>
    internal static void UnlockLockf(SafeFileHandle file) => _ = WithDescriptor(file, descriptor => Lockf(descriptor, LockfUnlock, 0));

    /// <summary>Takes an exclusive <c>flock</c> without waiting, which belongs to the open file rather than the process.</summary>
    /// <param name="file">A descriptor from <see cref="OpenDescriptor"/>.</param>
    /// <param name="path">The file, for messages.</param>
    /// <returns>Whether it was taken; false when another open of the file holds it.</returns>
    /// <exception cref="IOException">The lock could not be attempted.</exception>
    internal static bool TryFlock(SafeFileHandle file, string path) =>
        Attempt(file, path, "flock", descriptor => Flock(descriptor, FlockExclusive | FlockNonBlocking));

    /// <summary>Releases the <c>flock</c> <see cref="TryFlock"/> took; closing the descriptor releases it too.</summary>
    /// <param name="file">The descriptor.</param>
    internal static void UnlockFlock(SafeFileHandle file) => _ = WithDescriptor(file, descriptor => Flock(descriptor, FlockUnlock));

    /// <summary>Takes an exclusive <c>LockFileEx</c> lock over every byte of a file without waiting.</summary>
    /// <param name="file">The file's handle.</param>
    /// <param name="path">The file, for messages.</param>
    /// <returns>Whether it was taken; false when another handle holds an overlapping lock.</returns>
    /// <exception cref="IOException">The lock could not be attempted.</exception>
    internal static bool TryLockFileEx(SafeFileHandle file, string path)
    {
        var overlapped = default(Overlapped);
        if (LockFileEx(file, FailImmediately | ExclusiveLock, 0, uint.MaxValue, uint.MaxValue, ref overlapped))
        {
            return true;
        }

        var error = Marshal.GetLastPInvokeError();
        return error == WindowsLockViolation ? false : throw Failure(error, $"LockFileEx {path}");
    }

    /// <summary>Releases the lock <see cref="TryLockFileEx"/> took; closing the handle releases it too.</summary>
    /// <param name="file">The file's handle.</param>
    internal static void UnlockFileEx(SafeFileHandle file)
    {
        var overlapped = default(Overlapped);
        _ = UnlockFileExNative(file, 0, uint.MaxValue, uint.MaxValue, ref overlapped);
    }

    /// <summary>Makes a new name for an existing file: <c>link(2)</c>, or <c>CreateHardLinkW</c> on Windows.</summary>
    /// <param name="existing">The file.</param>
    /// <param name="link">The new name, which must not exist.</param>
    /// <exception cref="IOException">The file system refused.</exception>
    internal static void HardLink(string existing, string link)
    {
        if (OperatingSystem.IsWindows())
        {
            if (!CreateHardLink(link, existing, IntPtr.Zero))
            {
                throw Failure(Marshal.GetLastPInvokeError(), $"link {existing} to {link}");
            }

            return;
        }

        while (Link(existing, link) != 0)
        {
            var error = Marshal.GetLastPInvokeError();
            if (error != Interrupted)
            {
                throw Failure(error, $"link {existing} to {link}");
            }
        }
    }

    // Linux and macOS number O_CLOEXEC differently; other systems are not claimed.
    private static int CloseOnExec =>
        OperatingSystem.IsLinux() ? 0x80000
        : OperatingSystem.IsMacOS() ? 0x1000000
        : throw new PlatformNotSupportedException("The shared cache is locked on Linux, macOS and Windows only.");

    // EWOULDBLOCK is EAGAIN on both; POSIX lets lockf report a held lock as either it or EACCES.
    private static int WouldBlock => OperatingSystem.IsMacOS() ? 35 : 11;

    private static bool Attempt(SafeFileHandle file, string path, string call, Func<int, int> attempt)
    {
        while (true)
        {
            var error = 0;
            if (WithDescriptor(file, descriptor => attempt(descriptor) == 0 ? 0 : error = Marshal.GetLastPInvokeError()) == 0)
            {
                return true;
            }

            if (error == WouldBlock || error == AccessDenied)
            {
                return false;
            }

            if (error != Interrupted)
            {
                throw Failure(error, $"{call} {path}");
            }
        }
    }

    private static int WithDescriptor(SafeFileHandle file, Func<int, int> call)
    {
        var added = false;
        file.DangerousAddRef(ref added);
        try
        {
            return call(checked((int)file.DangerousGetHandle()));
        }
        finally
        {
            if (added)
            {
                file.DangerousRelease();
            }
        }
    }

    private static IOException Failure(int error, string what) =>
        new($"{what}: {new Win32Exception(error).Message} (error {error})", new Win32Exception(error));

    [LibraryImport("libc", EntryPoint = "open", StringMarshalling = StringMarshalling.Utf8, SetLastError = true)]
    private static partial int Open(string path, int flags);

    [LibraryImport("libc", EntryPoint = "lockf", SetLastError = true)]
    private static partial int Lockf(int descriptor, int command, nint length);

    [LibraryImport("libc", EntryPoint = "flock", SetLastError = true)]
    private static partial int Flock(int descriptor, int operation);

    [LibraryImport("libc", EntryPoint = "link", StringMarshalling = StringMarshalling.Utf8, SetLastError = true)]
    private static partial int Link(string existing, string link);

    [LibraryImport("kernel32.dll", EntryPoint = "LockFileEx", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool LockFileEx(SafeFileHandle file, uint flags, uint reserved, uint lengthLow, uint lengthHigh, ref Overlapped overlapped);

    [LibraryImport("kernel32.dll", EntryPoint = "UnlockFileEx", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool UnlockFileExNative(SafeFileHandle file, uint reserved, uint lengthLow, uint lengthHigh, ref Overlapped overlapped);

    [LibraryImport("kernel32.dll", EntryPoint = "CreateHardLinkW", StringMarshalling = StringMarshalling.Utf16, SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static partial bool CreateHardLink(string link, string existing, IntPtr securityAttributes);

    // OVERLAPPED, whose offset of 0 is where the locked range starts.
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
