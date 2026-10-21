// Census: validateViewUsageOnlyOneViewRendering
// Case: violating
// Ours: e[only-one-view-rendering] A view may have at most one view rendering.
// Pilot: e: A view may have at most one view rendering.
package P { rendering def R; view def V; view v : V { render rendering r1 : R; render rendering r2 : R; } }
