// Census: validateFeatureEndIsConstant
// Case: violating
// Ours: e[variable-feature-owner] Must be owned by an occurrence type
// Pilot: e: no viable alternative at input ';' | e: Must have at least two related elements
package P { class A; class B; assoc L { end var a : A; end b : B; } }
