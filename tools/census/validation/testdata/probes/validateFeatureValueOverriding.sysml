// Census: validateFeatureValueOverriding
// Expect: error: cannot override the binding value of P::D::a
package P { private import ScalarValues::Integer; part def D { attribute a : Integer = 1; } part def E :> D { attribute :>> a = 2; } }
