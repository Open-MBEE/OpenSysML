// Census: validateStateUsageParallelSubactions
// Expect: error: A parallel state cannot have successions or transitions.
package P { state def S { state x parallel { state a; state b; transition first a then b; } } }
