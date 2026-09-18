// Census: validateStakeholderMembershipOwningType
// Expect: error: 'stakeholder' declares a stakeholder of a requirement and is only allowed in a requirement body
package P { part def D; part def Q { stakeholder k : D; } }
