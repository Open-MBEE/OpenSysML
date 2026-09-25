// Census: validateViewUsageOnlyOneRendering
// Expect: error: A view may have at most one view rendering.
package P { rendering def R; view def V; view v : V { render rendering r1 : R; render rendering r2 : R; } }
