// Census: validateAttributeUsageEnumerationType
// Expect: error: An enumeration must be typed by one enumeration definition.
package P { enum def E { e1; } enum def F { f1; } enum x : E, F; }
