using System;
using System.Reflection;
using System.Runtime.Loader;

namespace OpenMBEE.OpenSysML.LoadContextProbe;

/// <summary>
/// A collectible load context holding its own copy of the client and of every package it depends on,
/// resolved from this probe's dependency manifest; only the shared framework comes from the default
/// context.
/// </summary>
internal sealed class CopyContext(string name, string mainAssembly) : AssemblyLoadContext(name, isCollectible: true)
{
    private readonly AssemblyDependencyResolver resolver = new(mainAssembly);

    protected override Assembly? Load(AssemblyName assemblyName) =>
        resolver.ResolveAssemblyToPath(assemblyName) is { } path ? LoadFromAssemblyPath(path) : null;

    protected override IntPtr LoadUnmanagedDll(string unmanagedDllName) =>
        resolver.ResolveUnmanagedDllToPath(unmanagedDllName) is { } path ? LoadUnmanagedDllFromPath(path) : IntPtr.Zero;
}
