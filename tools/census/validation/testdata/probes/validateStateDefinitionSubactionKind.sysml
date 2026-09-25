// Census: validateStateDefinitionSubactionKind
// Expect: error: A state may have at most one entry action.
package P { state def S { entry; do; exit; entry; } }
