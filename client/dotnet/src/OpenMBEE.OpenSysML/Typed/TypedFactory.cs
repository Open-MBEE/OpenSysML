namespace OpenMBEE.OpenSysML.Typed;

/// <summary>Creates the view a generated class gives of an instance.</summary>
/// <param name="instance">The instance to view.</param>
/// <param name="instantiation">The instantiation the instance was built by; null when there is none.</param>
/// <returns>The view.</returns>
public delegate TypedObject TypedFactory(Instance instance, Instantiation? instantiation);
