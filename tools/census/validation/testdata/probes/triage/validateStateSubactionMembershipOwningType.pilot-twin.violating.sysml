// Census: validateStateSubactionMembershipOwningType
// Case: violating
// Ours: e[state-subaction-membership-owning-type] 'entry' declares the entry action of a state and is only allowed in a st
// Pilot: e: mismatched input 'entry' expecting '}' | e: extraneous input '}' expecting EOF
package P { part def D; part def Q { entry action init; } }
