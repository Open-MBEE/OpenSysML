namespace OpenMBEE.OpenSysML.Typed;

/// <summary>Decodes one value a feature holds into the type a generated property declares.</summary>
/// <typeparam name="T">The declared type.</typeparam>
/// <param name="featureName">The feature's name, which a failure names.</param>
/// <param name="value">One value the feature holds.</param>
/// <param name="instantiation">The instantiation an object reference is resolved through; null when there is none.</param>
/// <returns>The decoded value.</returns>
/// <exception cref="TypeMismatchException">The value is not of the declared type.</exception>
public delegate T FeatureDecoder<out T>(string featureName, Value value, Instantiation? instantiation);
