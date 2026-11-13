using System;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>The command was asked for something it cannot do as asked: a bad argument, or an input it cannot read.</summary>
internal sealed class UsageException(string message) : Exception(message);
