// Census: validateRenderingUsageType
// Expect: error: A rendering must be typed by one rendering definition.
package P { rendering def R1; rendering def R2; rendering r : R1, R2; }
