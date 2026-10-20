// Census: validateFeatureEndIsConstant
// Case: violating
// Ours: e[variable-feature-owner] Must be owned by an occurrence type
// Pilot: e: no viable alternative at input 'end'
package P { class A; class B; assoc L { var end a : A; end b : B; } }
