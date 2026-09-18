// Census: validateStateUsageSubactionKind
// Expect: error: A state may have at most one entry action.
package P { state def S { state x { entry; entry; } } }
