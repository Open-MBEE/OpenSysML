// Census: validateActorMembershipOwningType
// Expect: error: 'actor' declares an actor of a requirement or case and is only allowed in a requirement or case body
package P { part def D; part def Q { actor a : D; } }
