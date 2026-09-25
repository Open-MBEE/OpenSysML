// Census: validateObjectiveMembershipOwningType
// Expect: error: 'objective' declares the objective of a case and is only allowed in a case body
package P { part def D; part def Q { objective o; } }
