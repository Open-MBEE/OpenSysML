// Census: validateFlowEndSubsetting
// Expect: error: Cannot identify flow end (use dot notation)
package P {
  class A { out feature o : ScalarValues::Integer; }
  class B { in feature i : ScalarValues::Integer; }
  class C { feature a : A; feature b : B; flow from A::o to B::i; }
}
