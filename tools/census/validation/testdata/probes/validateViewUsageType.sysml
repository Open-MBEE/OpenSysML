// Census: validateViewUsageType
// Expect: error: A view must be typed by one view definition.
package P { view def V1; view def V2; view v : V1, V2; }
