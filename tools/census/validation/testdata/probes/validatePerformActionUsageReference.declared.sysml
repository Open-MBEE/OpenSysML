// Census: validatePerformActionUsageReference
// Expect: error: Must reference an action.
package P { part def PD; part def D { part p : PD; perform action f references p; } }
