// Census: validateExhibitStateUsageReference
// Expect: error: Must reference a state.
package P { part def PD; part def D { part p : PD; exhibit state e references p; } }
