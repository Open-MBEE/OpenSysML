//go:build race

package export

// raceBuild reports whether the tests run under the race detector, whose
// instrumentation allocates on its own.
const raceBuild = true
