// Census: validateStateSubactionMembershioOwningType
// Expect: error: 'entry' declares the entry action of a state and is only allowed in a state body
package P { part def D; part def Q { entry action init; } }
