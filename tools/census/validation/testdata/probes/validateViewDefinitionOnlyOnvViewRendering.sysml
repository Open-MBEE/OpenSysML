// Census: validateViewDefinitionOnlyOnvViewRendering
// Expect: error: A view definition may have at most one view rendering.
package P { rendering def R; view def V { render rendering r1 : R; render rendering r2 : R; } }
