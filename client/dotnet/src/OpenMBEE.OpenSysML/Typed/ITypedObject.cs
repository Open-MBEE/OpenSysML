namespace OpenMBEE.OpenSysML.Typed;

/// <summary>
/// What every generated typed interface extends: a statically typed view over an <see cref="OpenSysML.Instance"/>,
/// whose properties decode the instance's feature values.
/// </summary>
public interface ITypedObject
{
    /// <summary>The instance this object views.</summary>
    Instance Instance { get; }

    /// <summary>The instantiation the instance was built by, which nested objects are resolved through; null when there is none.</summary>
    Instantiation? Instantiation { get; }
}
