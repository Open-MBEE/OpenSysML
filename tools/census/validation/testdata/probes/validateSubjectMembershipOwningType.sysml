// Census: validateSubjectMembershipOwningType
// Expect: error: 'subject' declares the subject of a requirement or case and is only allowed in a requirement or case body
package P { part def D; part def Q { subject s : D; } }
