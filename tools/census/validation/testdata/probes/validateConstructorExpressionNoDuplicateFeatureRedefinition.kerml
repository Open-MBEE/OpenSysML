// Census: validateConstructorExpressionNoDuplicateFeatureRedefinition
// Expect: error: x of C is already bound by an earlier argument
package P { class T; class C { feature x : T; } feature t : T; feature c : C = new C(x = t, x = t); }
