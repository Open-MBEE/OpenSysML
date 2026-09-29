//go:build !race

package grpc

// raceBuild reports whether the tests run under the race detector, which slows
// them too much for a wall-clock bound to mean anything.
const raceBuild = false
