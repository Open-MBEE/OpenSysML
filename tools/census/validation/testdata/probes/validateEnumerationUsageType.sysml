// Census: validateEnumerationUsageType
// Expect: error: An enumeration must be typed by one enumeration definition.
package P { enum def E { e1; } attribute def A; enum x : A; }
