// Census: validateStateDefinitionStateSubactionKind
// Case: violating
// Ours: e[state-entry-action] A state may have at most one entry action.
// Pilot: e: A state may have at most one entry action. | w: Duplicate of other owned member name | w: Duplicate of other owned member name
package P { state def S { entry; do; exit; entry; } }
