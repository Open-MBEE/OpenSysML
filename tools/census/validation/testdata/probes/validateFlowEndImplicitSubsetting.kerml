// Census: validateFlowEndImplicitSubsetting
// Expect: error: Must be an accessible feature (use dot notation for nesting)
package P {
  class C {
    feature a { out feature o : ScalarValues::Integer; }
    feature b { in feature i : ScalarValues::Integer; }
    flow f from a::o to b::i;
  }
}
