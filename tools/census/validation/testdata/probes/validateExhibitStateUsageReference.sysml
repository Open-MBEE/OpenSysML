// Census: validateExhibitStateUsageReference
// Expect: error: Must reference a state.
package P { action def Act; part def D { action a : Act; exhibit a; } }
