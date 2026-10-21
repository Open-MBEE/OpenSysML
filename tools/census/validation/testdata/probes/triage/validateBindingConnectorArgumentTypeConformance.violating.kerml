// Census: validateBindingConnectorArgumentTypeConformance
// Case: violating
// Ours: w[bound-feature-types] Bound features should have conforming types
// Pilot: w: Bound features should have conforming types
package P { class A; class B; feature a : A; feature b : B; binding a = b; }
